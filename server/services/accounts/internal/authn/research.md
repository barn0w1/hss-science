# Authn Package Research & Analysis

## 1. Responsibilities & Core Logic

The `authn` package is the federated authentication handler for the accounts microservice. Its primary responsibility is to orchestrate the OAuth 2.0 Authorization Code flow with support for multiple external identity providers (Google, GitHub).

**Core Flow:**
1. **Provider Selection** (`SelectProvider`): User requests login, receives HTML form with available providers
2. **OAuth Redirect** (`FederatedRedirect`): User selects a provider; handler encrypts state and redirects to provider's authorization endpoint
3. **OAuth Callback** (`FederatedCallback`): Provider redirects back with authorization code; handler:
   - Decrypts and validates state
   - Exchanges code for token via provider's token endpoint
   - Fetches user claims from provider (via `FetchClaims`)
   - Finds or creates user in identity service
   - Completes login by notifying the auth request repository
   - Redirects user to final callback URL

**Key Business Logic:**
- Implements the OAuth 2.0 Authorization Code grant type
- Maintains OIDC compatibility (Google provider uses ID tokens; GitHub uses REST API)
- Encrypts OAuth state to prevent CSRF attacks and preserve context across redirects
- Bridges external identity providers with internal user identity management
- Supports multiple concurrent auth requests via unique `authRequestID`

## 2. Domain Models

### `Provider` (provider.go)
```
Name         string                                              // e.g., "google", "github"
DisplayName  string                                              // e.g., "Sign in with Google"
OAuth2Config *oauth2.Config                                     // Preconfigured OAuth2 client config
FetchClaims  func(context.Context, *oauth2.Token) (*FederatedClaims, error)  // Provider-specific claim extraction
```
- Represents a single configured OAuth2 provider
- Encapsulates provider-specific logic for claim extraction
- Designed for easy provider addition (see `NewProviders`)

### `federatedState` (handler.go, JSON-serializable)
```
AuthRequestID string  // Links OAuth flow to the original auth request
Provider      string  // Which provider was selected
Nonce         string  // Cryptographic randomness (UUID)
```
- Encrypted and included in OAuth2 state parameter
- Survives the redirect to the provider and back
- Carries context needed to complete the login after callback
- Uses AES-256-GCM encryption (see `encryptState`/`decryptState`)

### `Handler` (handler.go)
```
providers    []*Provider                          // All configured providers
providerMap  map[string]*Provider                 // Name-based lookup
identitySvc  identity.Service                     // User creation/lookup
authReqs     AuthRequestQuerier                   // Auth request repository
cryptoKey    [32]byte                             // AES-256 key for state encryption
callbackURL  func(context.Context, string) string // Callback URL builder
tmpl         *template.Template                   // Provider selection HTML template
logger       *slog.Logger                         // Structured logging
```
- Central orchestrator for the entire federated auth flow
- Stateless HTTP handler (all context carried in encrypted state)
- Wires together providers, services, and request handlers

### Domain Models from `identity` Package
**FederatedClaims** (used by provider FetchClaims):
```
Subject       string  // Provider's unique user ID (e.g., GitHub user ID)
Email         string
EmailVerified bool
Name          string
GivenName     string
FamilyName    string
Picture       string  // Avatar URL
```
- Normalized claims format returned by all providers
- Provider-agnostic representation of user identity information

**User** (returned by identity.Service):
```
ID            string
Email         string
EmailVerified bool
Name          string
GivenName     string
FamilyName    string
Picture       string
CreatedAt     time.Time
```

## 3. Ports & Interfaces

### Outbound Ports (Consumed by Handler)

**`AuthRequestQuerier` interface (handler.go)**
```go
GetByID(ctx context.Context, id string) (AuthRequestInfo, error)
CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
```
- **GetByID**: Validates that an auth request exists (called in `SelectProvider`)
- **CompleteLogin**: Notifies the auth orchestrator that login succeeded (called in `FederatedCallback`)
- External implementation (not defined in this package) - likely managed by OIDC/OAuth2 server orchestration layer

**`identity.Service` interface (from identity package, used in FederatedCallback)**
```go
FindOrCreateByFederatedLogin(ctx context.Context, provider string, claims FederatedClaims) (*User, error)
```
- Implements "update on login" pattern: finds existing user by federated identity or creates new one
- Deduplicates user creation across provider registrations
- Returns User object with ID for auth completion

### Inbound Ports (Exposed by Handler)

**HTTP Handler Methods:**
- `SelectProvider(w http.ResponseWriter, r *http.Request)`: GET /login?authRequestID=...
- `FederatedRedirect(w http.ResponseWriter, r *http.Request)`: POST /login/select (form submission)
- `FederatedCallback(w http.ResponseWriter, r *http.Request)`: GET /login/callback (OAuth redirect)

### Provider Extension Point

**`Provider.FetchClaims` function pointer**
- Allows provider-specific logic for extracting user claims from tokens
- **Google implementation** (`provider_google.go`):
  - Uses OpenID Connect: fetches and verifies ID token (`go-oidc/v3`)
  - Claims extracted from JWT claims (verified signature)
  - No additional HTTP calls
- **GitHub implementation** (`provider_github.go`):
  - Uses OAuth2 + REST API: exchanges code for access token, then calls GitHub API
  - HTTP GET to `https://api.github.com/user` with Bearer token
  - Claims extracted from REST response
  - Subject is GitHub user numeric ID formatted as string

### Configuration Dependencies
**`Config` struct (config.go)**
```go
IssuerURL          string  // Base URL of the issuer/accounts service
GoogleClientID     string
GoogleClientSecret string
GitHubClientID     string
GitHubClientSecret string
```
- Minimal, flat configuration
- Uses empty string to disable providers
- `IssuerURL` used to build OAuth redirect URL

## 4. Dependencies

### External Libraries
- **`golang.org/x/oauth2`**: Core OAuth2 client library
  - Handles code exchange for tokens
  - Manages token refresh logic (though `AccessTypeOffline` requested but not used in callback)
  - Scopes and endpoints management
- **`golang.org/x/oauth2/google`** & **`golang.org/x/oauth2/github`**: Provider-specific endpoints
- **`github.com/coreos/go-oidc/v3/oidc`**: OpenID Connect provider discovery and ID token verification (Google only)
- **`github.com/google/uuid`**: UUID generation for nonce in state
- **Standard Library**:
  - `crypto/aes`, `crypto/cipher`: AES-GCM encryption
  - `crypto/rand`: Cryptographically secure randomness
  - `encoding/base64`: State encoding
  - `encoding/json`: State serialization
  - `html/template`: Provider selection UI
  - `net/http`: HTTP handlers and client
  - `log/slog`: Structured logging
  - `time`: Timestamp operations

### Internal Packages
- **`identity` package**: User domain, service interface, federated claims models
  - Used for finding/creating users based on federated identity
- **`pkg/crypto`**: Custom cryptographic utilities
  - Wraps AES-256-GCM encryption/decryption with base64 encoding
  - Handles nonce generation and validation
  - Used exclusively for state parameter encryption

### Database/Storage
- Never directly (decoupled via `AuthRequestQuerier` and `identity.Service`)
- `AuthRequestQuerier` implementation (not in this package) presumably queries auth requests
- `identity.Service` implementation (in `identity/service.go`) accesses user repository

## 5. Specifications from Tests

### State Encryption/Decryption (`handler_test.go`)

**Specification: Symmetric Encryption Roundtrip**
```
- Encrypt(state) → encrypted string
- Decrypt(encrypted) → original state
- All fields preserved: AuthRequestID, Provider, Nonce
```
Test: `TestEncryptDecryptState_RoundTrip`
- Validates complete roundtrip with actual state values
- Ensures no data loss or corruption

**Specification: Encryption Robustness**
- **Invalid base64**: `TestDecryptState_InvalidBase64` - Decrypt must reject malformed base64
- **Ciphertext too short**: `TestDecryptState_TooShort` - Reject if shorter than nonce size (12 bytes for GCM)
- **Wrong decryption key**: `TestDecryptState_WrongKey` - Decrypt fails with different key (GCM tag verification fails)

**Business Rule**: State is cryptographically bound to the server's secret key; any tampering or key rotation invalidates all outstanding auth sessions.

### FederatedRedirect Validation (`handler_test.go`)

**Specification: Required Parameters**
```
- authRequestID: REQUIRED (non-empty)
- provider: REQUIRED (non-empty)
```
Test: `TestFederatedRedirect_MissingParams` - Must return 400 Bad Request for any missing param

**Specification: Provider Validation**
```
- provider must exist in providerMap
```
Test: `TestFederatedRedirect_UnknownProvider` - Unknown provider → 400 Bad Request

**Specification: State Parameter Inclusion**
```
- Redirect URL must include state= parameter
- State must be valid base64-encoded, encrypted JSON
```
Test: `TestFederatedRedirect_Success`
- Verifies redirect to IdP authorization endpoint
- Verifies state parameter is present in URL

**Business Rule**: FederatedRedirect must not change state format or encryption method; changes break existing sessions.

### FederatedCallback Validation (`handler_test.go`)

**Specification: Required Query Parameters**
```
- code: REQUIRED (non-empty)
- state: REQUIRED (non-empty)
```
Test: `TestFederatedCallback_MissingParams` - Must return 400 Bad Request for any missing param

**Specification: State Decryption**
```
- Invalid/corrupted state → 400 Bad Request
```
Test: `TestFederatedCallback_InvalidState` - Decryption failure treated as auth failure

**Implicit Specification** (from code, not tested):
- Code exchange with provider may fail → 500 Internal Server Error
- Fetching claims (HTTP call or token verification) may fail → 500 Internal Server Error
- User creation/lookup may fail → 500 Internal Server Error
- CompleteLogin RPC may fail → 500 Internal Server Error

### SelectProvider Validation (`handler_test.go`)

**Specification: Auth Request Validation**
```
- authRequestID from query string REQUIRED (non-empty)
- authRequestID must exist (via AuthRequestQuerier.GetByID)
```
Test: `TestSelectProvider_MissingAuthRequestID` - Missing → 400 Bad Request

**Business Rule**: SelectProvider acts as a checkpoint to validate the auth request exists before starting the OAuth flow. Auth requests are external resources managed outside this package.

### Implicit Specifications from Code

**State Nonce Generation**:
- A fresh `uuid.New().String()` is generated per OAuth attempt
- Protects against replay attacks (each authorization is unique)

**Authentication Time**:
- Set to `time.Now().UTC()` at callback completion
- Passed to `CompleteLogin()` as the auth time (when the user actually authenticated)

**Authentication Method Reference (AMR)**:
- Hardcoded as `[]string{"federated"}` indicating user authenticated via federated provider
- Could be extended to track provider-specific methods (e.g., `[]string{"federated:github"}`)
- Useful for risk assessment and compliance auditing

**Callback URL Construction**:
- Comes from dependency: `h.callbackURL(r.Context(), state.AuthRequestID)`
- Handler doesn't control destination; depends on injected function
- Prevents hardcoded redirects but obscures flow (see Tech Debt)

**Template Rendering Error Handling**:
- SelectProvider silently logs template errors but doesn't stop rendering
- `h.logger.Error` is called but execution continues
- User receives partial/blank response on template error (see Tech Debt)

## 6. Tech Debt & Refactoring Candidates

### Architecture Issues

#### 1. **Violation of Single Responsibility Principle**
**Issue**: `Handler` manages three concerns:
- HTTP request/response handling
- OAuth 2.0 protocol orchestration
- Application-level flow completion (identity service, auth requests)

**Evidence**:
- `FederatedCallback` contains: protocol handling (token exchange, claims fetch), domain logic (CreatedAt, AMR), and integration (CompleteLogin RPC)

**Impact**: Hard to test, reuse logic in different contexts (e.g., token refresh, re-authentication)

**Recommendation**:
```
Create separate layers:
- OAuthFlow (or similar): Handles OAuth protocol (code exchange, claims fetch)
- AuthenticationFlow / LoginFlow: Handles "complete login" business logic
- HTTPHandler: Orchestrates the above
```

#### 2. **Function Pointer for Provider-Specific Logic**
**Issue**: `Provider.FetchClaims` is a function pointer (`func(context.Context, *oauth2.Token) (*FederatedClaims, error)`)

**Evidence**:
```go
FetchClaims: func(ctx context.Context, token *oauth2.Token) (*identity.FederatedClaims, error)
```

**Impact**:
- Hard to test (can't mock at interface level)
- Makes it difficult to add pre/post-processing logic (logging, validation, enrichment)
- Logic spread across multiple files (`provider_github.go`, `provider_google.go`)
- Can't compose or pipeline transformations

**Recommendation**:
```go
// Define interface
type ClaimsProvider interface {
    FetchClaims(ctx context.Context, token *oauth2.Token) (*FederatedClaims, error)
}

// Implement per provider
type GoogleClaimsProvider struct { ... }
type GitHubClaimsProvider struct { ... }

// Use in Provider
type Provider struct {
    ClaimsProvider ClaimsProvider
}
```

#### 3. **Google OIDC Discovery on Every Initialization**
**Issue**: `newGoogleProvider` calls `gooidc.NewProvider(ctx, "https://accounts.google.com")` every time

**Evidence** (provider_google.go, line ~15):
```go
oidcProvider, err := gooidc.NewProvider(ctx, "https://accounts.google.com")
```

**Impact**:
- HTTP request to Google's OIDC discovery endpoint on every service startup
- Discovery result is immutable; caching would improve startup time
- Discovery can fail, blocking service initialization

**Recommendation**:
```go
// Cache the OIDC provider
var googleOIDCProvider *oidc.Provider
func init() {
    // Or use sync.Once pattern
}

func newGoogleProvider(...) {
    verifier := googleOIDCProvider.Verifier(...)
}
```

#### 4. **GitHub Provider Makes Uncontrolled HTTP Calls**
**Issue**: GitHub provider's `FetchClaims` makes direct HTTP call via `httpClient.Do(req)`

**Evidence** (provider_github.go, line ~32):
```go
httpClient := &http.Client{Timeout: 10 * time.Second}
resp, err := httpClient.Do(req)
```

**Impact**:
- No connection pooling (new client per fetch)
- No retry logic for transient failures
- Hard timeout of 10 seconds (could be problematic for slow networks)
- No observability (no metrics, tracing, circuit breaker)

**Recommendation**:
```go
// Use injected HTTP client with pooling
func newGitHubProvider(clientID, clientSecret, callbackURL string, httpClient *http.Client) *Provider

// Or implement with proper instrumentation
type InstrumentedHTTPClient struct {
    client *http.Client
    metrics MetricsRecorder
}
```

#### 5. **Implicit Routing and Callback Logic**
**Issue**: Callback URL generation is hidden behind a function pointer

**Evidence** (handler.go):
```go
type Handler struct {
    callbackURL func(context.Context, string) string
}

// In FederatedCallback:
callbackURL := h.callbackURL(r.Context(), state.AuthRequestID)
```

**Impact**:
- Handler doesn't control where user is redirected after login
- Makes it hard to understand flow without seeing handler initialization
- Could be exploited if callbackURL function has bugs

**Recommendation**:
- Document expected callback URL format
- Validate callback URL against whitelist before redirecting
- Consider moving callback URL construction to auth request repository (which created this request)

### Error Handling Issues

#### 6. **No Structured Error Types**
**Issue**: All errors are formatted strings wrapped with `fmt.Errorf`

**Evidence**:
```go
return "", fmt.Errorf("github user API: %w", err)
return nil, fmt.Errorf("oidc discovery: %w", err)
```

**Impact**:
- Can't distinguish between different error types programmatically
- Difficult to implement retry logic or error recovery
- Can't map errors to appropriate HTTP status codes consistently

**Recommendation**:
```go
// Define error types
var (
    ErrProviderNotFound = errors.New("provider not found")
    ErrTokenExchange = errors.New("token exchange failed")
    ErrClaimsFetch = errors.New("claims fetch failed")
)

// Use custom error types
type ProviderError struct {
    Provider string
    Err      error
}
```

#### 7. **Inconsistent Error Recovery**
**Issue**: No mechanism to handle transient failures gracefully

**Evidence**:
- GitHub API call failure → 500 Internal Server Error (no retry)
- Token exchange failure → 500 Internal Server Error (no retry)
- User creation failure → 500 Internal Server Error (no retry)

**Impact**:
- Transient network issues fail the entire login attempt
- No circuit breaker to fail fast if OAuth provider is down
- No metrics to detect systematic issues

**Recommendation**:
```go
// Implement exponential backoff for Google OIDC discovery
// Implement circuit breaker for GitHub API calls
// Log and track error frequency per provider
```

#### 8. **Silent Template Rendering Failures**
**Issue**: Template rendering errors are logged but rendering continues

**Evidence** (handler.go):
```go
if err := h.tmpl.Execute(w, selectProviderData{...}); err != nil {
    h.logger.Error("template execution failed", "error", err)
}
// No response returned; user gets blank page
```

**Impact**:
- User sees blank page instead of error message
- Error log is easily missed in production
- No HTTP status code indicates error

**Recommendation**:
```go
if err := h.tmpl.Execute(w, selectProviderData{...}); err != nil {
    h.logger.Error("template execution failed", "error", err)
    http.Error(w, "internal server error", http.StatusInternalServerError)
    return
}
```

### Security Issues

#### 9. **No Rate Limiting or Brute Force Protection**
**Issue**: HTTP handlers have no guards against repeated attempts

**Evidence**: No middleware or logic checking request frequency

**Impact**:
- FederatedRedirect could be called repeatedly with invalid providers (resource exhaustion)
- SelectProvider could be probed with random authRequestIDs
- OAuth provider callbacks could be flooded

**Recommendation**:
```go
// Implement rate limiting middleware
// Track failed attempts per IP/user
// Implement exponential backoff for repeated failures
```

#### 10. **State Parameter is Only Protection Against CSRF**
**Issue**: No additional anti-CSRF measures (e.g., SameSite cookies)

**Evidence**:
- State is encrypted but origin not verified independently
- Relies on OAuth2 library to validate state before callback

**Impact**:
- If state encryption breaks, CSRF protection is lost
- No defense-in-depth

**Recommendation**:
```go
// Check SameSite cookie attribute
// Implement double-submit cookie pattern as backup
// Validate referrer header on callback
w.Header().Set("Set-Cookie", "...")  // with SameSite=Strict
```

#### 11. **No Validation of OAuth2 Config**
**Issue**: `NewProviders` doesn't validate OAuth2 configuration

**Evidence** (provider.go):
```go
if cfg.GoogleClientID != "" {
    p, err := newGoogleProvider(ctx, cfg.GoogleClientID, ...)
    // But what if ClientSecret is empty? RedirectURL wrong?
}
```

**Impact**:
- Invalid configs fail at runtime (not at initialization)
- May cause cascading failures in callback handler

**Recommendation**:
```go
func (cfg Config) Validate() error {
    if cfg.GoogleClientID != "" && cfg.GoogleClientSecret == "" {
        return errors.New("google client secret required if client ID set")
    }
    // Validate URLs are well-formed
}
func NewProviders(...) error {
    if err := cfg.Validate(); err != nil {
        return err
    }
}
```

### Testing & Observability Issues

#### 12. **Limited Observable Behavior**
**Issue**: No metrics, tracing, or request correlation

**Evidence**:
- Logging uses structured `slog` but no request ID/correlation ID
- No counters for success/failure rates per provider
- No visibility into latency of OAuth endpoint calls

**Impact**:
- Hard to debug issues in production
- No alerting on high failure rates
- Can't trace a user's login journey across services

**Recommendation**:
```go
// Add request context with correlation ID
// Log all state transitions
// Emit metrics: login_attempts_total, login_duration_seconds per provider
// Implement distributed tracing
```

#### 13. **Template Not Testable**
**Issue**: HTML template is embedded as a constant; hard to test or modify

**Evidence** (handler.go):
```go
const selectProviderHTML = `<!DOCTYPE html>...`
tmpl := template.Must(template.New("select_provider").Parse(selectProviderHTML))
```

**Impact**:
- Can't test HTML rendering separately
- Can't modify UI without code change
- Template syntax errors panic at initialization

**Recommendation**:
```go
// Load template from file
// Implement template versioning/i18n
// Separate HTML generation from handler
type ProviderSelector interface {
    RenderProviders(ctx context.Context, providers []*Provider, authRequestID string) (string, error)
}
```

### Performance Issues

#### 14. **No Connection Pooling for External APIs**
**Issue**: Github provider creates new HTTP client per request

**Impact**:
- TCP connection overhead for every claim fetch
- No keep-alive reuse
- Limits throughput

**Recommendation**: Inject shared HTTP client with pooling.

#### 15. **Provider Map Lookup on Every Request**
**Minimal Issue**: `providerMap` is rebuilt per handler instance
- Not a bottleneck, but could be optimized during initialization
- Current approach is fine for typical provider counts

### Missing Functionality

#### 16. **No Provider Metadata Exposure**
**Issue**: Handler exposes providers internally but no public API to discover available providers

**Impact**:
- Client can't learn available providers dynamically
- UI must hardcode provider list
- New provider additions require frontend and backend coordination

**Recommendation**:
```go
// Add public endpoint
func (h *Handler) ListProviders(w http.ResponseWriter, r *http.Request) {
    // Return JSON list of {name, displayName}
}
```

#### 17. **No Logout/Session Revocation**
**Issue**: Package only handles login; no logout mechanism

**Impact**:
- User sessions have no explicit revocation point
- Federated logout (if provider supports) not implemented

**Recommendation**:
```go
// Add logout handler
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
    // Revoke token with provider (if supported)
    // Clear session
}
```

### Coupling & Boundary Issues

#### 18. **Tight Coupling to Identity Service**
**Issue**: Handler directly calls `identity.Service.FindOrCreateByFederatedLogin`

**Impact**:
- Can't reuse OAuth logic without identity service
- Hard to test with different identity implementations
- User creation logic is opaque to handler

**Recommendation**:
```go
// Extract user creation into a separate port
type UserRepository interface {
    FindOrCreateByFederatedLogin(ctx context.Context, provider string, claims FederatedClaims) (*User, error)
}
// And a separate service for user syncing claims
```

#### 19. **AuthRequestQuerier is Too Generic**
**Issue**: Interface is minimal; implementation details hidden

```go
type AuthRequestQuerier interface {
    GetByID(ctx context.Context, id string) (AuthRequestInfo, error)
    CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
}
```

**Impact**:
- Can't debug auth request state without seeing implementation
- Error handling strategy is unclear
- `AuthRequestInfo` is empty struct (unused)

**Recommendation**:
```go
type AuthRequest struct {
    ID    string
    State string  // "pending", "completed", etc.
    Error string  // if failed
}
```

### Code Quality Issues

#### 20. **Repeated Code in Provider Implementations**
**Issue**: Both GitHub and Google providers manually create `Provider` struct

**Evidence**:
```go
// In both provider_github.go and provider_google.go
return &Provider{
    Name:         "...",
    DisplayName:  "...",
    OAuth2Config: oauth2Cfg,
    FetchClaims:  func(...) { ... },
}
```

**Recommendation**: Extract common boilerplate or use factory pattern.

---

## Summary

The `authn` package successfully implements OAuth 2.0 federated authentication but exhibits several architectural and operational gaps typical of a "working but not yet battle-hardened" system. Key refactoring priorities should be:

1. **High Priority**:
   - Separate OAuth protocol logic from business logic
   - Use interfaces instead of function pointers for provider logic
   - Implement structured error types and proper error recovery
   - Add rate limiting and observability

2. **Medium Priority**:
   - Cache OIDC discovery and HTTP clients
   - Improve error handling in FederatedCallback
   - Add provider validation at initialization
   - Implement distributed tracing with correlation IDs

3. **Low Priority**:
   - Extract template to external file
   - Add provider discovery endpoint
   - Implement logout mechanism
   - Reduce code duplication in provider factories
