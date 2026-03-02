# OIDC Package Research Report

## 1. Responsibilities & Core Logic

The `oidc` package is the **OpenID Connect authentication and authorization subsystem** of the accounts microservice. It serves as the core implementation of the OAuth2/OIDC protocol flow, handling:

1. **Authorization Code Flow**: Creating and managing authorization requests (OAuth2 authorization codes and PKCE challenges)
2. **Token Management**: Issuing access tokens and refresh tokens, with full lifecycle management (creation, validation, expiration, revocation)
3. **Client Management**: Validating client credentials and managing client authentication methods
4. **Session Management**: Storing authentication state, completing login flows, and terminating sessions
5. **User Information**: Exposing standardized user claims (profile, email, etc.) through OIDC endpoints

The package integrates with:
- **Zitadel OIDC library** (`zitadel/oidc/v3`) for the OIDC protocol machinery
- **PostgreSQL** for persistent storage of auth requests, tokens, and clients
- **Identity service** for user data retrieval and federated identity management
- **RSA cryptography** for JWT signing and verification

**Key Pattern**: The package follows **Ports & Adapters (Hexagonal Architecture)**, with clear separation between:
- Domain/business logic layer (service files: `*_svc.go`)
- Adapter layer bridging Zitadel protocol and domain models (`adapter/*`)
- Repository/persistence layer (`postgres/*`)

---

## 2. Domain Models

### Core Entities

#### **AuthRequest**
Represents a pending OAuth2 authorization request (one per login session).

```go
type AuthRequest struct {
	ID                  string    // Unique request ID (ULID)
	ClientID            string    // OAuth2 client requesting authorization
	RedirectURI         string    // Where to redirect after consent
	State               string    // Anti-CSRF token (client-provided)
	Nonce               string    // Replay attack prevention token
	Scopes              []string  // Requested scopes (openid, email, profile, etc.)
	ResponseType        string    // "code" (auth code flow)
	ResponseMode        string    // "query" or "form_post" 
	CodeChallenge       string    // PKCE challenge (S256 hashing)
	CodeChallengeMethod string    // "S256" or "plain"
	Prompt              []string  // UI hint (login, consent, etc.)
	MaxAge              *int64    // Max time since user last authenticated
	LoginHint           string    // Hint for which user/email to use
	UserID              string    // Set after user logs in
	AuthTime            time.Time // When user last authenticated
	AMR                 []string  // Auth Methods Used (federated, pwd, etc.)
	IsDone              bool      // Whether login flow is complete
	Code                string    // Generated authorization code
	CreatedAt           time.Time // Record creation timestamp
}
```

**State Transitions**: Empty → (user login) → IsDone=true + UserID set → Code generated → Consumed/Deleted

#### **Token**
Represents an access token with optional refresh token reference.

```go
type Token struct {
	ID             string        // Unique token ID (ULID)
	ClientID       string        // Which app issued this token
	Subject        string        // User ID this token represents
	Audience       []string      // Intended recipients (typically [ClientID])
	Scopes         []string      // Permissions granted
	Expiration     time.Time     // When token expires
	RefreshTokenID string        // Associated refresh token (if any)
	CreatedAt      time.Time
}
```

**Invariant**: Expiration is always checked; expired tokens are not returned from repository queries.

#### **RefreshToken**
Represents a refresh token used to obtain new access tokens.

```go
type RefreshToken struct {
	ID            string        // Unique refresh token ID (ULID)
	Token         string        // Opaque token string (ULID) sent to client
	ClientID      string        // Which app issued this token
	UserID        string        // User ID associated with token
	Audience      []string      // Intended recipients
	Scopes        []string      // Original scopes granted
	AuthTime      time.Time     // When user originally authenticated
	AMR           []string      // Original auth methods used
	AccessTokenID string        // Last access token ID (for rotation tracking)
	Expiration    time.Time     // Token expiration
	CreatedAt     time.Time
}
```

**Key Detail**: The `Token` field is the **opaque token string** sent to clients; the `ID` field is an internal identifier for DB tracking.

#### **Client**
Represents an OAuth2 client application.

```go
type Client struct {
	ID                       string        // Client ID (e.g., "mobile-app")
	SecretHash               string        // bcrypt hash of client secret
	RedirectURIs             []string      // Allowed callback URIs
	PostLogoutRedirectURIs   []string      // Allowed post-logout redirect URIs
	ApplicationType          string        // "web", "native", "user_agent"
	AuthMethod               string        // "client_secret_basic", "client_secret_post", "none", "private_key_jwt"
	ResponseTypes            []string      // ["code"], ["token"], etc.
	GrantTypes               []string      // ["authorization_code", "refresh_token"], ["client_credentials"], etc.
	AccessTokenType          string        // "jwt" or "bearer"
	IDTokenLifetimeSeconds   int           // How long ID tokens are valid
	ClockSkewSeconds         int           // Tolerance for time validation
	IDTokenUserinfoAssertion bool          // Include userinfo in ID token claims
	CreatedAt                time.Time
	UpdatedAt                time.Time
}
```

**Authentication**: Uses bcrypt comparison for secret validation (see `ClientService.ClientCredentials`).

---

## 3. Ports & Interfaces

### Domain Ports (Exported)

#### **Service Interfaces** (outbound contracts)

```go
// AuthRequestService: Manages auth requests with TTL-based expiration
type AuthRequestService interface {
	Create(ctx context.Context, ar *AuthRequest) error
	GetByID(ctx context.Context, id string) (*AuthRequest, error)
	GetByCode(ctx context.Context, code string) (*AuthRequest, error)
	SaveCode(ctx context.Context, id, code string) error
	CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
	Delete(ctx context.Context, id string) error
}

// ClientService: Client validation and authentication
type ClientService interface {
	GetByID(ctx context.Context, clientID string) (*Client, error)
	AuthorizeSecret(ctx context.Context, clientID, clientSecret string) error
	ClientCredentials(ctx context.Context, clientID, clientSecret string) (*Client, error)
}

// TokenService: Token issuance and management
type TokenService interface {
	CreateAccess(ctx context.Context, clientID, subject string, audience, scopes []string, expiration time.Time) (tokenID string, err error)
	CreateAccessAndRefresh(ctx context.Context, clientID, subject string, audience, scopes []string, accessExpiration, refreshExpiration, authTime time.Time, amr []string, currentRefreshToken string) (accessID, refreshToken string, err error)
	GetByID(ctx context.Context, tokenID string) (*Token, error)
	GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error)
	GetRefreshInfo(ctx context.Context, token string) (userID, tokenID string, err error)
	DeleteByUserAndClient(ctx context.Context, userID, clientID string) error
	Revoke(ctx context.Context, tokenID string) error
	RevokeRefreshToken(ctx context.Context, token string) error
}
```

#### **Repository Interfaces** (inbound contracts)

```go
// AuthRequestRepository: Persistence for AuthRequest
type AuthRequestRepository interface {
	Create(ctx context.Context, ar *AuthRequest) error
	GetByID(ctx context.Context, id string) (*AuthRequest, error)
	GetByCode(ctx context.Context, code string) (*AuthRequest, error)
	SaveCode(ctx context.Context, id, code string) error
	CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
	Delete(ctx context.Context, id string) error
}

// ClientRepository: Read-only client access
type ClientRepository interface {
	GetByID(ctx context.Context, clientID string) (*Client, error)
}

// TokenRepository: Persistence for access and refresh tokens
type TokenRepository interface {
	CreateAccess(ctx context.Context, access *Token) error
	CreateAccessAndRefresh(ctx context.Context, access *Token, refresh *RefreshToken, currentRefreshToken string) error
	GetByID(ctx context.Context, tokenID string) (*Token, error)
	GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error)
	GetRefreshInfo(ctx context.Context, token string) (userID, tokenID string, err error)
	DeleteByUserAndClient(ctx context.Context, userID, clientID string) error
	Revoke(ctx context.Context, tokenID string) error
	RevokeRefreshToken(ctx context.Context, token string) error
}
```

### Adapter Layer: Bridge to Zitadel

The `adapter/` package implements Zitadel's `op.Storage` interface, adapting domain models to protocol requirements.

#### **StorageAdapter** (Zitadel bridge)
Implements `zitadel/oidc/v3/pkg/op.Storage` interface, providing:

```go
type StorageAdapter struct {
	identity    identity.Service
	authReqs    oidcdom.AuthRequestService
	clients     oidcdom.ClientService
	tokens      oidcdom.TokenService
	signing     *SigningKeyWithID      // RSA private key for JWTs
	public      *PublicKeyWithID       // RSA public key for token validation
	accessTTL   time.Duration
	refreshTTL  time.Duration
	healthCheck func(context.Context) error
}
```

**Critical Methods**:
- `CreateAuthRequest(ctx, authReq, userID)`: Convert Zitadel's OIDC request to domain model
- `CreateAccessToken(ctx, request)`: Issue short-lived access tokens
- `CreateAccessAndRefreshTokens(ctx, request, currentRefreshToken)`: Issue token pair with rotation
- `TokenRequestByRefreshToken(ctx, token)`: Refresh token validation
- `AuthorizeClientIDSecret(ctx, clientID, secret)`: Client authentication
- `SetUserinfoFromScopes/Request/Token(ctx, userinfo, ...)`: Populate OIDC userinfo claims
- `GetPrivateClaimsFromScopes`: Returns empty (no custom private claims)
- `Health(ctx)`: Database health check

#### **Key Adapters**
- `AuthRequest`: Wraps domain `AuthRequest` as `op.AuthRequest`
- `ClientAdapter`: Wraps domain `Client` as `op.Client` with OIDC-specific methods
- `RefreshTokenRequest`: Wraps domain `RefreshToken` as `op.RefreshTokenRequest`
- `SigningKeyWithID` / `PublicKeyWithID`: RSA key wrappers for Zitadel JWT operations

#### **Provider Configuration**
```go
func NewProvider(issuer string, cryptoKey [32]byte, storage op.Storage, logger *slog.Logger) (*op.Provider, error)
```
Creates a Zitadel provider with:
- **S256 PKCE support** enabled (`CodeMethodS256: true`)
- **POST method authentication** enabled (`AuthMethodPost: true`)
- **Refresh token grant** enabled (`GrantTypeRefreshToken: true`)
- **Private Key JWT** disabled (`AuthMethodPrivateKeyJWT: false`)
- **Request Objects** disabled (`RequestObjectSupported: false`)
- **Supported UI Locales**: English, Japanese

---

## 4. Dependencies

### Internal Packages
- **`identity`** package: User data retrieval, federated identity management
  -Interface: `identity.Service.GetUser(ctx, userID) (*User, error)`
  - Used for populating OIDC userinfo claims from scope requests

- **`pkg/domerr`**: Custom error types (sentinel errors)
  - `ErrNotFound`, `ErrUnauthorized`, `ErrAlreadyExists`, `ErrInternal`

- **`authn`** package (sibling, not directly used in OIDC core)

### External Libraries
- **`github.com/zitadel/oidc/v3`**: Production-grade OIDC protocol server
  - Manages authentication flows, JWT issuance, protocol validation
  - Package provides: `op.Provider`, `op.Storage`, `oidc.AuthRequest`, `oidc.Error` types

- **`golang.org/x/crypto/bcrypt`**: Password hashing for client secret validation

- **`github.com/go-jose/go-jose/v4`**: JWT library for cryptographic operations
  - RSA key handling, signature algorithms (RS256)

- **`github.com/oklog/ulid/v2`**: Distributed ID generation
  - Used for: AuthRequest IDs, Token IDs, Refresh Token IDs

### Database
- **PostgreSQL**: Persistent storage via `sqlx`
  - Tables: `auth_requests`, `clients`, `tokens`, `refresh_tokens`, `users` (identity service)
  - Array data types: `auth_requests.scopes[]`, `auth_requests.prompt[]`, `auth_requests.amr[]`
  - Null handling for optional fields (`user_id`, `auth_time`, `code`, `refresh_token_id`)

### Cryptographic Dependencies
- **RSA Key Management**: 2048-bit RSA keys for signing
- **Key ID Derivation**: SHA256 hash of PKIX-encoded public key (first 8 bytes as hex)
- **Signature Algorithm**: RS256 only (no support for ES256, RS512, etc.)

---

## 5. Specifications from Tests

### AuthRequestService Behavior
(from `authrequest_svc_test.go`)

1. **TTL Enforcement**: Auth requests expire after a configurable TTL (default 30 minutes)
   - `GetByID` checks: `time.Now().UTC().After(ar.CreatedAt.Add(authRequestTTL))` → returns `domerr.ErrNotFound`
   - Same for `GetByCode`
   - **Edge case**: Expiration is checked at retrieval time, not in the database

2. **Lifecycle**: Create → SaveCode → CompleteLogin → Delete
   - `CompleteLogin` sets `UserID`, `AuthTime`, `AMR[]`, and `IsDone=true`
   - Does **not** require prior code generation

3. **Error Propagation**: Repository errors bubble up unchanged

### ClientService Behavior
(from `client_svc_test.go`)

1. **Secret Validation**: Uses bcrypt constant-time comparison
   - Wrong secret: returns `domerr.ErrUnauthorized`
   - Client not found: returns `domerr.ErrNotFound`

2. **Two Methods**:
   - `AuthorizeSecret`: Returns only error (pass/fail check)
   - `ClientCredentials`: Returns both client and error

3. **No Authorization Checks**: Service doesn't validate scope requests or redirect URIs
   - That's the adapter's responsibility

### TokenService Behavior
(from `token_svc_test.go`)

1. **ID Generation**: Uses ULID for all token IDs and refresh token values
   - Both `Access.ID` and `Refresh.Token` are ULIDs
   - Ensures they're non-empty and unique

2. **Access Token Invariants**:
   - `CreateAccess`: Accepts expiration time, stores as-is
   - `GetByID`: Returns token **only if not expired** (database query includes `AND expiration > now()`)

3. **Refresh Token Rotation**:
   - `CreateAccessAndRefresh` with non-empty `currentRefreshToken` → deletes old token in **same transaction**
   - Old token becomes unretrievable but not physically deleted until tested
   - Ensures only one refresh token is valid per user+client at a time

4. **Token Linking**:
   - `Access.RefreshTokenID` points to the Refresh Token's ID
   - `Refresh.AccessTokenID` points back to last Access Token ID
   - Used for tracking token families

5. **Deletion Semantics**:
   - `DeleteByUserAndClient`: Deletes **both** tokens and refresh tokens for that user+client combo
   - `Revoke/RevokeRefreshToken`: Individual revocation (hard delete)

### Repository Tests (PostgreSQL)
(from `postgres/repo_test.go`)

1. **Auth Request CRUD**: Full lifecycle tested
   - Scopes and prompt are stored as PostgreSQL arrays (`pq.StringArray`)
   - Nullable fields: `user_id`, `auth_time`, `code`, `max_age`
   - `GetByID`/`GetByCode` return `domerr.ErrNotFound` on no rows

2. **Client Retrieval**:
   - No insert test (clients are pre-populated / managed externally)
   - Arrays properly unmarshalled: `RedirectURIs`, `ResponseTypes`, `GrantTypes`

3. **Token Creation & Retrieval**:
   - Access tokens stored with `refresh_token_id` (nullable, used for linking)
   - Tokens are **NOT** returned if `expiration <= now()`
   - Refresh token value is the opaque token string; ID is internal

4. **Refresh Token Rotation**:
   - Creates new access + refresh pair
   - Passes old refresh token value to `CreateAccessAndRefresh`
   - **With transaction**: deletes old refresh token in same operation
   - Old token becomes inaccessible: `GetRefreshToken(ctx, oldToken)` returns `domerr.ErrNotFound`

### Adapter (StorageAdapter) Tests
(from `adapter/storage_test.go`)

1. **Protocol Adaptation**:
   - Converts Zitadel's `oidc.AuthRequest` → domain `AuthRequest`
   - Converts domain `AuthRequest` ← Zitadel's `op.AuthRequest` interface

2. **UserInfo Scope Handling**:
   - **Scope → Claim Mapping**:
     - `openid` → `Subject`
     - `profile` → `Name`, `GivenName`, `FamilyName`, `Picture`, `UpdatedAt`
     - `email` → `Email`, `EmailVerified`
   - Missing user → returns error (user not found)

3. **Introspection Response**:
   - Sets `Active = false` if token not found
   - Includes user claims if user exists and scopes allow
   - Always includes token metadata: `Subject`, `ClientID`, `Scope`, `Audience`, `Expiration`, `IssuedAt`, `TokenType`

4. **Client Authentication**:
   - Rejects with OIDC error: `oidc.ErrInvalidClient().WithDescription(...)`
   - Distinguishes between "not found" and "wrong secret"

5. **Unsupported Features**:
   - JWT Profile Grant: Returns error `"jwt profile grant not supported"`
   - Private claims: Returns empty map `map[string]any{}`

### Adapter (AuthRequest) Tests
(from `adapter/authrequest_test.go`)

1. **OIDC Interface Compliance**:
   - `GetID()`, `GetClientID()`, `GetRedirectURI()`, `GetState()`, `GetNonce()`, `GetSubject()`, `GetScopes()`, `GetAudience()`
   - `GetAuthTime()`, `GetAMR()`, `GetACR()` (empty), `Done()`
   - `GetResponseType()`, `GetResponseMode()`, `GetCodeChallenge()`

2. **Code Challenge**:
   - Returns `nil` if domain's `CodeChallenge == ""`
   - Otherwise returns `&oidc.CodeChallenge{Challenge: ..., Method: S256}`

3. **Audience**:
   - Hardcoded to `[]string{ClientID}` (not client's audience claim)

### Adapter (ClientAdapter) Tests
(from `adapter/client_test.go`)

1. **Application Type Mapping**:
   - `"web"` → `op.ApplicationTypeWeb` (default)
   - `"native"` → `op.ApplicationTypeNative`
   - `"user_agent"` → `op.ApplicationTypeUserAgent`

2. **Auth Method Mapping**:
   - `"client_secret_basic"` → `oidc.AuthMethodBasic` (default)
   - `"client_secret_post"` → `oidc.AuthMethodPost`
   - `"none"` → `oidc.AuthMethodNone`
   - `"private_key_jwt"` → `oidc.AuthMethodPrivateKeyJWT`

3. **Token Type Mapping**:
   - `"jwt"` → `op.AccessTokenTypeJWT`
   - anything else → `op.AccessTokenTypeBearer` (default)

4. **Scope Restrictions**:
   - `IsScopeAllowed(scope)`: Always returns `false` (no scope restrictions enforced at this layer)
   - `RestrictAdditionalIdTokenScopes(scopes)`: Returns scopes unchanged (pass-through)
   - `RestrictAdditionalAccessTokenScopes(scopes)`: Returns scopes unchanged (pass-through)

5. **No Scope Validation**: ClientAdapter doesn't validate if requested scopes are allowed
   - This validation must happen in Zitadel provider

### Key-Related Tests
(from `adapter/keys_test.go`)

1. **Key ID Derivation**:
   - `deriveKeyID(publicKey)`: SHA256(PKIX-encoded public key) → first 8 bytes as hex → 16-char hex string
   - **Deterministic**: Same key → same ID
   - **Different keys**: Different IDs
   - **Consistency**: Signing and Public Keys derived from same RSA key have identical IDs

2. **Signing Key Interface**:
   - Implements `op.SigningKey`
   - Algorithm: RS256 only
   - ID: Derived via SHA256

3. **Public Key Interface**:
   - Implements `op.Key`
   - Algorithm: RS256
   - Use: `"sig"` (signature)

---

## 6. Tech Debt & Refactoring Candidates

### 6.1 Architectural Issues

#### **Issue 1: TTL Enforcement at Service Layer**
**Location**: `authrequest_svc.go` (GetByID, GetByCode)

**Problem**:
```go
if time.Now().UTC().After(ar.CreatedAt.Add(s.authRequestTTL)) {
    return nil, fmt.Errorf("auth request expired: %w", domerr.ErrNotFound)
}
```
- TTL is checked **in-memory after retrieval**, not at the database level
- This means:
  - Expired records accumulate in PostgreSQL until manually deleted
  - Repository doesn't enforce invariant (repository layer should be responsible for valid data)
  - Race condition possible: Record expires between query and validation

**Recommendation**:
- Move TTL check to `AuthRequestRepository.GetByID()` / `GetByCode()` at the SQL level
- Combine both into single database predicate: `WHERE ... AND created_at > now() - INTERVAL '30 minutes'`
- Service layer should trust repository to return only valid records
- Enables periodic cleanup: DELETE WHERE created_at < now() - INTERVAL '60 minutes'

---

#### **Issue 2: No Explicit Error Wrapping for Protocol Errors**
**Location**: `adapter/storage.go`

**Problem**:
```go
ar, err := s.authReqs.GetByID(ctx, id)
if err != nil {
	if domerr.Is(err, domerr.ErrNotFound) {
		return nil, oidc.ErrInvalidRequest().WithDescription("auth request not found")
	}
	return nil, err  // Exposes internal errors to protocol layer
}
```
- Inconsistent error handling: Some paths wrap in `oidc.Error`, others bubble up raw errors
- Internal database errors (connection timeouts, transaction failures) leak into protocol responses
- Makes debugging harder; unclear which errors are expected vs. bugs

**Recommendation**:
```go
// Create adapter error type
type StorageError struct {
	Code        string // oidc error code (e.g., "invalid_request")
	Description string
	Err         error  // wrapped internal error
}

// Convert systematically
if domerr.Is(err, domerr.ErrNotFound) {
    return nil, &StorageError{Code: "invalid_request", Description: "..."}
}
if errors.Is(err, context.DeadlineExceeded) {
    return nil, &StorageError{Code: "server_error", Description: "request timeout"}
}
```

---

#### **Issue 3: Identity Service as Hidden External Dependency**
**Location**: `adapter/storage.go` methods `setUserinfo()`, `setIntrospectionUserinfo()`

**Problem**:
```go
user, err := s.identity.GetUser(ctx, userID)
if err != nil {
	if domerr.Is(err, domerr.ErrNotFound) {
		return oidc.ErrInvalidRequest().WithDescription("user not found")
	}
	return err
}
```
- OIDC package directly depends on `identity.Service` (tight coupling)
- If identity service is unavailable:
  - Userinfo endpoint fails entirely
  - Token introspection fails
  - Cannot issue tokens with minimal claims
- No fallback behavior; no graceful degradation
- The dependency is injected but its failure modes are not documented

**Recommendation**:
- Make identity service optional in StorageAdapter
- Provide defaults for missing user data (empty strings, false booleans)
- Return partial userinfo rather than failing the endpoint
- Document: "If identity service unavailable, OIDC claims will be minimal (subject only)"

---

#### **Issue 4: No Constraints on Scope Claims Mapping**
**Location**: `adapter/storage.go` in `setUserinfo()`, `setIntrospectionUserinfo()`

**Problem**:
```go
case oidc.ScopeOpenID:
    userinfo.Subject = user.ID
case oidc.ScopeProfile:
    userinfo.Name = user.Name
    userinfo.GivenName = user.GivenName
    // ...
case oidc.ScopeEmail:
    userinfo.Email = user.Email
```
- **Hard-coded scope → claim mapping** with no configuration
- No validation that requested scopes are actually allowed for this client
- `ClientAdapter.IsScopeAllowed()` always returns `false` (dead code?)
- If a malicious client requests unauthorized scopes, they're granted anyway (Zitadel framework validates, but layering is unclear)

**Recommendation**:
```go
// Store scope constraints in Client model
type Client struct {
    AllowedScopes []string  // Add this
    // ...
}

// Validate in adapter/storage
func (s *StorageAdapter) SetUserinfoFromScopes(ctx context.Context, ..., scopes []string) error {
    client, _ := s.clients.GetByID(ctx, clientID)
    filteredScopes := filterScopes(scopes, client.AllowedScopes)
    s.setUserinfo(ctx, userinfo, userID, filteredScopes)
}
```

---

### 6.2 Code Quality Issues

#### **Issue 5: Inconsistent Nil Handling for Optional Fields**
**Location**: `postgres/token_repo.go`, `postgres/authrequest_repo.go`

**Problem**:
```go
// token_repo.go
var refreshTokenID *string
err := row.Scan(&t.ID, /*...*/, &refreshTokenID, &t.CreatedAt)
// ...
if refreshTokenID != nil {
    t.RefreshTokenID = *refreshTokenID  // Explicit pointer dereference
}

// authrequest_repo.go uses pq.StringArray for slices but pointers for scalars
var userID *string
var code *string
var authTime *time.Time
if userID != nil {
    ar.UserID = *userID
}
if code != nil {
    ar.Code = *code
}
```

**Issue**: 
- Verbose; pointer dereference in every repository scan
- Inconsistent with PostgreSQL NULL handling philosophy
- Repetitive boilerplate

**Recommendation**:
```go
// Create helper
func ptrToString(p *string) string {
    if p == nil {
        return ""
    }
    return *p
}

// Use
ar.UserID = ptrToString(userID)
ar.Code = ptrToString(code)
```

---

#### **Issue 6: Hard-coded Audience Derivation**
**Location**: `adapter/authrequest.go`

```go
func (a *AuthRequest) GetAudience() []string {
    return []string{a.domain.ClientID}  // Always [ClientID]
}
```

**Problem**:
- Audience claim is hard-coded to client ID
- OIDC spec allows audience to be configurable per client
- No way to express multi-audience tokens or custom audience claims
- Inconsistent with `adapter/storage.go` where audience defaults to `[]string{ClientID}` but could be different

**Recommendation**:
- Store `Audience` in domain `AuthRequest` explicitly (not derived)
- Or provide method: `GetAudience() []string { return a.domain.Audience }` 
- Default to `[ClientID]` only if not specified

---

#### **Issue 7: Type Assertion Pattern in Helper Functions**
**Location**: `adapter/storage.go` - `clientIDFromRequest()`, `extractAuthTimeAMR()`

**Problem**:
```go
func clientIDFromRequest(request op.TokenRequest) string {
    if ar, ok := request.(*AuthRequest); ok {
        return ar.GetClientID()
    }
    if rtr, ok := request.(*RefreshTokenRequest); ok {
        return rtr.GetClientID()
    }
    if ccr, ok := request.(*clientCredentialsTokenRequest); ok {
        return ccr.clientID
    }
    return ""  // Silent failure if type not recognized
}

func extractAuthTimeAMR(request op.TokenRequest) (time.Time, []string) {
    type authTimeGetter interface {
        GetAuthTime() time.Time
    }
    // ... similar type assertion pattern
    return authTime, amr  // Returns zero values if interfaces not satisfied
}
```

**Issues**:
- Silent fallback to zero values (empty string, zero time)
- No logging or error reporting when unexpected type is encountered
- Fragile: new token request types will silently return wrong values
- `clientCredentialsTokenRequest` uses dot-access (`ccr.clientID`) instead of interface method

**Recommendation**:
```go
func clientIDFromRequest(request op.TokenRequest) (string, error) {
    if ar, ok := request.(*AuthRequest); ok {
        return ar.GetClientID(), nil
    }
    // ... other cases
    return "", fmt.Errorf("unsupported token request type: %T", request)
}

// Or define interface for all token request types
type HasClientID interface {
    GetClientID() string
}
```

---

#### **Issue 8: Incomplete Token Revocation**
**Location**: `postgres/token_repo.go`

```go
func (r *TokenRepository) CreateAccessAndRefresh(..., currentRefreshToken string) error {
    tx, _ := r.db.BeginTxx(ctx, nil)
    
    if currentRefreshToken != "" {
        _, err = tx.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token = $1`, currentRefreshToken)
    }
    // Insert new access + refresh
    return tx.Commit()
}
```

**Problem**:
- Refresh token rotation **deletes old refresh token** immediately
- But **old access token is NOT deleted** (still valid in database with `expiration > now()`)
- Client can exploit this: Use old access token to call endpoints after rotation
- No transaction failure handling (defer Rollback is too permissive)

**Recommendation**:
```go
func (r *TokenRepository) CreateAccessAndRefresh(..., currentRefreshToken string) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin transaction: %w", err)
    }
    defer sql.Rollback() // Fail-safe rollback
    
    // Revoke old refresh token AND its associated access token
    if currentRefreshToken != "" {
        var oldAccessID string
        err = tx.QueryRowContext(ctx, 
            `SELECT access_token_id FROM refresh_tokens WHERE token = $1`, 
            currentRefreshToken).Scan(&oldAccessID)
        // ...
        tx.ExecContext(ctx, `DELETE FROM tokens WHERE id = $1`, oldAccessID)
        tx.ExecContext(ctx, `DELETE FROM refresh_tokens WHERE token = $1`, currentRefreshToken)
    }
    // ...
}
```

---

#### **Issue 9: No Secrets Rotation Strategy**
**Location**: `client_svc.go`, `adapter/keys.go`

**Problem**:
- Client secrets are **hashed at rest** (bcrypt) but **never rotated**
- Signing keys are **single static RSA key pair** injected at startup
- No key versioning strategy
- If a client secret or signing key is compromised, there's no safe way to rotate

**Recommendation**:
```go
// Client model
type Client struct {
    ID                 string
    SecretHashes       []string  // Multiple hashes (current + rotated)
    SecretRotatedAt    []time.Time
    // ...
}

// Keys model
type SigningKey struct {
    KeyID     string
    Key       *rsa.PrivateKey
    CreatedAt time.Time
    ExpiresAt time.Time
}

// Keep multiple keys with expiration
var keys = map[string]*SigningKey{}
```

---

### 6.3 Testing Gaps

#### **Issue 10: No Concurrent Access Tests**
**Problem**:
- No tests for concurrent refresh token rotation
- No tests for concurrent auth request completion
- Database connection pool behavior untested

**Recommendation**:
- Add parallel tests using Go's `t.Parallel()`
- Simulate race conditions in refresh token issuance
- Verify transaction isolation levels

---

#### **Issue 11: Missing Integration Tests for Error Paths**
**Problem**:
- Tests verify happy path thoroughly
- Missing: What happens when client is deleted while token is being issued?
- Missing: What if identity service times out during userinfo claim resolution?

**Recommendation**:
- Add fault injection tests
- Mock identity service timeouts
- Test cascading failures

---

### 6.4 Performance Concerns

#### **Issue 12: N+1 Query in Userinfo Endpoints**
**Location**: `adapter/storage.go` - `SetUserinfoFrom*()` methods

**Problem**:
```go
func (s *StorageAdapter) SetUserinfoFromToken(...) error {
    token, err := s.tokens.GetByID(ctx, tokenID)  // Query 1
    // ...
    user, err := s.identity.GetUser(ctx, token.Subject)  // Query 2 (separate service)
    // 2 round-trips for single userinfo request
}
```

**Recommendation**:
- Cache user data per request (context.WithValue)
- Or batch load users when multiple tokens present
- Consider adding `GetUsers(ctx, []userID) []User` to identity.Service

---

#### **Issue 13: Auth Request Accumulation**
**Location**: postgres/authrequest_repo.go

**Problem**:
- Expired auth requests accumulate in database (not cleaned up)
- Service-layer TTL check doesn't delete; repository's GetByID still returns them
- After 1 month, table could have millions of expired records

**Recommendation**:
- Add periodic vacuum job: DELETE FROM auth_requests WHERE created_at < now() - '1 day'
- Or soft-delete with `deleted_at` timestamp
- Or move to Redis cache with automatic expiration

---

### 6.5 Documentation & Maintainability

#### **Issue 14: Missing Boundary Documentation**
**Problem**:
- Unclear which validations Zitadel performs vs. this adapter
- Unclear error contract between domain and adapter layers
- No documentation of supported OIDC response types, grant types, methods only what's enabled in NewProvider

**Recommendation**:
- Add `doc.go` explaining:
  ```
  // Package oidc provides the OAuth2/OIDC protocol implementation.
  //
  // Responsibilities:
  // - Token lifecycle (issuance, refresh, revocation)
  // - Auth request state management
  // - Client authentication (secret validation)
  // - User claims resolution
  //
  // Delegated to Zitadel (zitadel/oidc/v3):
  // - Protocol endpoint routing (/authorize, /token, /userinfo)
  // - Request parameter validation
  // - Scope consent validation (currently empty)
  // - OIDC error formatting
  //
  // Assumptions:
  // - Auth decisions (whether to allow login) are made outside this package
  // - AuthRequest.UserID is populated by external login handler
  // - Client data is pre-populated and immutable
  ```

---

## Summary: Risk Ranking

| Issue | Severity | Impact | Effort |
|-------|----------|--------|--------|
| TTL Enforcement at Service Layer | **HIGH** | Data accumulation, inefficient queries | Medium |
| Identity Service Tight Coupling | **HIGH** | Availability/performance degradation | Medium |
| Token Revocation Incomplete | **HIGH** | Security risk (access token reuse) | Medium |
| Inconsistent Error Wrapping | **MEDIUM** | Debugging difficulty | Small |
| No Secrets Rotation Strategy | **MEDIUM** | Operational risk | Large |
| Type Assertion Silent Failures | **MEDIUM** | Difficult bugs to diagnose | Small |
| Scope Claims Constraints Missing | **MEDIUM** | Scope validation unclear | Medium |
| N+1 Query on Userinfo | **LOW** | Performance (non-critical path) | Small |
| Nil Handling Boilerplate | **LOW** | Code maintainability | Small |
| Concurrent Access Testing | **MEDIUM** | Race conditions undetected | Medium |

