# Accounts Service -- Implementation Plan

> **Status**: Draft -- pending review and inline annotations.
> This document describes the design for `server/services/accounts/`, the OIDC Provider (OP) / Identity service for the hss-science platform.

---

## 0. Constraints Derived from Existing Repository

These are non-negotiable -- they come from CI, release pipeline, or go.mod:

| Constraint | Source | Value |
|---|---|---|
| Go module path | `server/go.mod` | `github.com/barn0w1/hss-science/server` |
| Service source path | `ci2.yaml` matrix | `server/services/accounts/` |
| CI build target | `ci2.yaml` | `./services/accounts/...` |
| Dockerfile path | `release.yaml` matrix | `./server/services/accounts/Dockerfile` |
| Docker context | `release.yaml` | `.` (repo root) |
| Image name | `release.yaml` | `ghcr.io/barn0w1/hss-science/accounts` |
| Shared internal code | `ci2.yaml` change detection | `server/internal/**` |
| Generated proto code | `buf.gen.yaml` | `server/gen/` |
| Linter config | `.golangci.yml` | `govet, errcheck, staticcheck, gosec`, etc. |
| Go version | `go.mod` | 1.25.5 |

---

## 1. Design Decision: Library API Style

**Choice: Legacy `op.NewOpenIDProvider`**

Rationale:
- The Legacy API handles all OIDC protocol routing, request parsing, response serialization, discovery generation, CORS, PKCE validation, and error formatting for us.
- We only need to implement `op.Storage` + custom login routes. This is the correct level of abstraction for a first implementation.
- The experimental `op.Server` interface (pre-v4) is more powerful but requires us to reimplement every protocol operation. That's unnecessary complexity with no benefit for our use case.
- The Legacy path is what the reference implementation uses and is battle-tested.
- If we ever need finer control (e.g., custom token response fields), we can migrate to the `Server` API later -- the `NewLegacyServer` bridge makes this a non-breaking transition.

---

## 2. Design Decision: Access Token Format

**Choice: JWT access tokens (`AccessTokenTypeJWT`)**

Rationale:
- All our resource servers (chat, drive, myaccount gRPC services) need to verify access tokens. JWT tokens are self-verifiable using the public JWKS endpoint.
- Opaque tokens would require every resource server to call the introspection endpoint on every request, adding latency and coupling.
- The BFF layer can pass the JWT to gRPC services in metadata, and each service can independently verify it using the cached JWKS.
- Revocation is handled at the application layer (short-lived access tokens + refresh token rotation).

**Token lifetimes:**
- Access tokens: 15 minutes (short-lived, minimizes revocation window).
- ID tokens: 1 hour.
- Refresh tokens: 7 days (with rotation -- each use invalidates the old refresh token and issues a new one).

---

## 3. Design Decision: Signing Algorithm and Key Management

**Choice: RS256 (RSA PKCS#1 v1.5 with SHA-256)**

Rationale:
- Widest compatibility across all JWT libraries and platforms.
- All OIDC clients are required to support RS256 per spec (RFC 7518 Section 3.1).

**Key management approach:**
- A single RSA-2048 private key, loaded from an environment variable (PEM-encoded) at startup.
- Key ID (`kid`) derived deterministically from the key (e.g., hash of public key modulus), so it's stable across restarts.
- `SigningKey()` returns this key. `KeySet()` returns the corresponding public key.
- Key rotation is out of scope for the initial implementation. When needed, we'll add multi-key support (active signing key + retired verification-only keys in the JWKS).

---

## 4. Design Decision: Issuer Strategy

**Choice: Static issuer**

Value: `https://accounts.hss-science.org` (configurable via environment variable).

Rationale:
- We are not multi-tenant. One issuer, one OP instance.
- Simpler configuration, simpler token verification for resource servers.
- In local development, this will be overridden to `http://localhost:<port>` with `op.WithAllowInsecure()`.
> Author Annotation: No local E2E testing is planned. Implementation of local-specific logic (e.g., `localhost` overrides or `op.WithAllowInsecure()`) is strictly NOT required. Focus entirely on the production-grade HTTPS flow.

---

## 5. Design Decision: Federated Authentication Flow

Our OP does NOT authenticate users directly. Authentication is **federated to upstream IdPs** (Google, GitHub). Here is how the login flow works within the `zitadel/oidc` library's callback mechanism:

```
Relying Party (e.g., drive.hss-science.org)
    |
    | 1. GET /authorize?client_id=drive&redirect_uri=...&scope=openid+profile+email
    v
Our OP (accounts.hss-science.org)
    |
    | 2. Library creates AuthRequest, calls Storage.CreateAuthRequest()
    | 3. AuthRequest.Done() == false -> redirects to Client.LoginURL(authRequestID)
    v
Our Login Handler (/login/federated?authRequestID=xxx)
    |
    | 4. Builds upstream OAuth2 authorize URL for Google/GitHub
    | 5. Stores authRequestID in `state` parameter (encrypted/signed)
    | 6. Redirects user to Google
    v
Google / GitHub
    |
    | 7. User authenticates at Google
    | 8. Google redirects to our callback: /login/federated/callback?code=xxx&state=yyy
    v
Our Federated Callback Handler (/login/federated/callback)
    |
    | 9. Exchanges code for tokens with Google
    | 10. Extracts user identity from Google's ID token (sub, email, name)
    | 11. Upserts user in our database (find-or-create by provider+sub)
    | 12. Updates AuthRequest in storage: sets UserID, marks Done=true, records AuthTime
    | 13. Redirects to op.AuthCallbackURL: /authorize/callback?id=authRequestID
    v
Our OP (library-handled callback)
    |
    | 14. Library fetches AuthRequest by ID, sees Done()==true
    | 15. Issues authorization code, redirects to RP's redirect_uri
    v
Relying Party
    |
    | 16. Exchanges code for tokens at POST /oauth/token
    v
Our OP
    |
    | 17. Library issues access_token (JWT) + id_token + refresh_token
```

This is the critical insight: our "login UI" is not a username/password form -- it's an OAuth2 client that talks to upstream IdPs. The `zitadel/oidc` library only sees the result: a completed `AuthRequest` with a subject.

---

## 6. Design Decision: State Storage

### 6.1 Auth Requests -- PostgreSQL

Auth requests will be stored in PostgreSQL with a TTL (cleanup via a periodic job or `created_at` filter).

Rationale:
- Auth requests must survive server restarts (the user might be at Google for minutes).
- We already have PostgreSQL as our core database.
- The volume is low (one row per active login flow, short-lived).
- Redis is an alternative, but adding a Redis dependency just for auth requests when we already have PostgreSQL is unnecessary for now.

### 6.2 Authorization Codes -- PostgreSQL

Stored in the same table structure as auth requests (the code maps to an auth request ID). Codes are single-use and short-lived (< 60 seconds typically).

### 6.3 Tokens (Access/Refresh) -- PostgreSQL

Even though access tokens are JWTs (self-verifiable), we still need to track them for:
- Revocation (`RevokeToken`)
- Introspection (`SetIntrospectionFromToken`)
- Refresh token lifecycle (rotation, expiry)

Each token record stores: `id`, `subject`, `client_id`, `scopes`, `audience`, `expiration`, `refresh_token_id`.

### 6.4 Users -- PostgreSQL

Our user table is the canonical identity store. Fields include:
- `id` (UUID, our internal subject)
- `email`, `email_verified`
- `name`, `given_name`, `family_name`
- `picture`
- `created_at`, `updated_at`

### 6.5 Federated Identities -- PostgreSQL

Links upstream IdP identities to our users:
- `user_id` (FK -> users)
- `provider` (e.g., "google", "github")
- `provider_subject` (the `sub` claim from the upstream IdP)
- Unique constraint on `(provider, provider_subject)`

### 6.6 Clients -- Hardcoded in code (for now)

Our internal clients (drive, chat, myaccount-bff) are known at compile time. We'll register them in Go code, similar to the example's `RegisterClients()` pattern.

Rationale: Adding a dynamic client registration API is premature. We have a handful of first-party clients. If we need dynamic registration later, we can move the client store to the database.

### 6.7 Signing Keys -- Environment variable

Loaded at startup. Stored in-memory. Not persisted to DB.

---

## 7. Design Decision: Grant Types to Support

| Grant Type | Support | Notes |
|---|---|---|
| `authorization_code` | Yes | Primary flow for all RP/SPA clients via the BFF. |
| `refresh_token` | Yes | BFF needs long-lived sessions. Rotation is mandatory. |
| `client_credentials` | Yes | Service-to-service calls (e.g., chat service calling drive service). Implement `ClientCredentialsStorage`. |
| `urn:ietf:params:oauth:grant-type:jwt-bearer` | No | Not needed initially. No service accounts using JWT assertions. |
| `urn:ietf:params:oauth:grant-type:token-exchange` | No | Not needed initially. No delegation/impersonation use case yet. |
| `urn:ietf:params:oauth:grant-type:device_code` | No | No device flow use case. |

PKCE: **Required** for the authorization code flow. `code_challenge_method=S256` enabled in `op.Config`.

---

## 8. Proposed Directory Layout

```
server/services/accounts/
├── main.go                    # Entrypoint: config loading, wiring, HTTP server start
├── Dockerfile                 # Multi-stage Docker build
├── config/
│   └── config.go              # Env-var based configuration struct
├── oidcprovider/
│   ├── provider.go            # op.NewOpenIDProvider construction and router setup
│   ├── storage.go             # op.Storage implementation (orchestrator, delegates to repos)
│   ├── authrequest.go         # AuthRequest struct implementing op.AuthRequest interface
│   ├── client.go              # Client struct implementing op.Client interface + client registry
│   ├── refreshtoken.go        # RefreshTokenRequest struct implementing op.RefreshTokenRequest
│   └── keys.go                # SigningKey / PublicKey structs implementing op.SigningKey / op.Key
├── login/
│   ├── handler.go             # HTTP handlers: federated login initiation + callback
│   └── upstream.go            # Upstream IdP OAuth2 client configuration (Google, GitHub)
├── model/
│   ├── user.go                # User domain model
│   ├── federated_identity.go  # FederatedIdentity domain model
│   ├── token.go               # Token / RefreshToken domain models
│   └── authrequest.go         # AuthRequest domain/DB model
└── repo/
    ├── user.go                # UserRepository (PostgreSQL CRUD)
    ├── authrequest.go         # AuthRequestRepository (PostgreSQL CRUD)
    └── token.go               # TokenRepository (PostgreSQL CRUD)
```

### Why this layout

- **`oidcprovider/`**: Everything that directly satisfies the `zitadel/oidc` library contracts. `storage.go` is the central `op.Storage` impl. It delegates to `repo/` for persistence and uses domain models from `model/`.
- **`login/`**: The federated login flow handlers. These are HTTP routes mounted alongside the OP, wrapped with `op.IssuerInterceptor`. Completely separate from the OIDC protocol layer.
- **`model/`**: Domain types that are independent of the OIDC library. Pure data structures.
- **`repo/`**: Database access layer. One file per aggregate. Uses `sqlx` + raw SQL (no ORM).
- **`config/`**: Env-var loading. Single struct, populated at startup.

---

## 9. `op.Storage` Implementation Architecture

The `Storage` struct in `oidcprovider/storage.go` will hold references to the repositories and config:

```go
type Storage struct {
    userRepo       *repo.UserRepository
    authReqRepo    *repo.AuthRequestRepository
    tokenRepo      *repo.TokenRepository
    clients        map[string]*Client           // in-memory client registry
    signingKey     *SigningKey
    publicKey      *PublicKey
}
```

**Interface satisfaction summary:**

| Storage Method | Delegates To | Notes |
|---|---|---|
| `CreateAuthRequest` | `authReqRepo.Create()` | Converts `*oidc.AuthRequest` to domain model, persists, returns `*AuthRequest` |
| `AuthRequestByID` | `authReqRepo.GetByID()` | Returns `*AuthRequest` implementing `op.AuthRequest` |
| `AuthRequestByCode` | `authReqRepo.GetByCode()` | Looks up code -> auth request ID mapping |
| `SaveAuthCode` | `authReqRepo.SaveCode()` | Stores code -> auth request ID |
| `DeleteAuthRequest` | `authReqRepo.Delete()` | Cleanup after token issuance |
| `CreateAccessToken` | `tokenRepo.CreateAccess()` | Persists token metadata, returns `(tokenID, expiration)` |
| `CreateAccessAndRefreshTokens` | `tokenRepo.CreateAccessAndRefresh()` | Handles refresh token rotation |
| `TokenRequestByRefreshToken` | `tokenRepo.GetRefreshToken()` | Returns `*RefreshTokenRequest` |
| `TerminateSession` | `tokenRepo.DeleteByUserAndClient()` | Revokes all tokens for user+client |
| `RevokeToken` | `tokenRepo.Revoke()` | Revokes single token |
| `GetRefreshTokenInfo` | `tokenRepo.GetRefreshInfo()` | Returns user ID + token ID |
| `SigningKey` | returns `s.signingKey` | In-memory |
| `SignatureAlgorithms` | returns `[]jose.RS256` | Static |
| `KeySet` | returns `[]op.Key{s.publicKey}` | In-memory |
| `GetClientByClientID` | `s.clients[id]` | In-memory map lookup |
| `AuthorizeClientIDSecret` | `s.clients[id]` | Compares bcrypt hash |
| `SetUserinfoFromScopes` | `userRepo.GetByID()` | Maps user fields to `*oidc.UserInfo` based on scopes |
| `SetUserinfoFromToken` | `tokenRepo.Get()` + `userRepo.GetByID()` | Lookup token, then populate userinfo |
| `SetIntrospectionFromToken` | `tokenRepo.Get()` + `userRepo.GetByID()` | Lookup token, check active, populate introspection |
| `GetPrivateClaimsFromScopes` | (return empty map) | No custom JWT claims initially |
| `GetKeyByIDAndClientID` | (return error) | JWT Profile grant not supported |
| `ValidateJWTProfileScopes` | (return error) | JWT Profile grant not supported |
| `Health` | `db.PingContext()` | Database health check |

**Optional interfaces to implement:**

| Interface | Implement? | Notes |
|---|---|---|
| `CanSetUserinfoFromRequest` | **Yes** | Preferred by the library. Will become required in v4. Gives access to `GetClientID()`. |
| `ClientCredentialsStorage` | **Yes** | Needed for service-to-service auth. |
| `DeviceAuthorizationStorage` | No | No device flow. |
| `TokenExchangeStorage` | No | No token exchange. |
| `CanTerminateSessionFromRequest` | No | The basic `TerminateSession(userID, clientID)` is sufficient. |

---

## 10. `op.Client` Implementation

```go
type Client struct {
    id                   string
    secret               string              // bcrypt hash, empty for public clients
    redirectURIs         []string
    postLogoutURIs       []string
    applicationType      op.ApplicationType
    authMethod           oidc.AuthMethod
    responseTypes        []oidc.ResponseType
    grantTypes           []oidc.GrantType
    accessTokenType      op.AccessTokenType
    idTokenLifetime      time.Duration
    loginURL             func(string) string  // returns login initiation URL with authRequestID
    clockSkew            time.Duration
    devMode              bool
}
```

### Initial Client Registry

| Client ID | Type | Auth Method | Grant Types | Notes |
|---|---|---|---|---|
| `myaccount-bff` | Web | `client_secret_basic` | `authorization_code`, `refresh_token` | BFF for the MyAccount SPA |
| `drive-bff` | Web | `client_secret_basic` | `authorization_code`, `refresh_token` | BFF for the Drive SPA |
| `chat-bff` | Web | `client_secret_basic` | `authorization_code`, `refresh_token` | BFF for the Chat SPA |
| `chat-service` | Web | `client_secret_basic` | `client_credentials` | Chat gRPC service (S2S) |
| `drive-service` | Web | `client_secret_basic` | `client_credentials` | Drive gRPC service (S2S) |

All BFF clients will have `AccessTokenTypeJWT` and `LoginURL` pointing to `/login/federated?authRequestID=`.

---

## 11. `op.AuthRequest` Implementation

The `AuthRequest` struct serves dual purpose: it's the domain/DB model AND it satisfies the `op.AuthRequest` interface.

```go
type AuthRequest struct {
    ID            string           // UUID, primary key
    CreatedAt     time.Time
    ClientID      string           // The RP's client_id
    RedirectURI   string
    State         string
    Nonce         string
    Scopes        []string
    ResponseType  oidc.ResponseType
    ResponseMode  oidc.ResponseMode
    CodeChallenge string
    CodeChallengeMethod string
    Prompt        []string
    MaxAge        *time.Duration
    LoginHint     string
    UILocales     []string

    // Set during login completion:
    UserID        string           // Our internal user ID (the "subject")
    AuthTime      time.Time
    AMR           []string         // e.g. ["federated"]
    IsDone        bool

    // Authorization code (set after successful auth):
    Code          string
}

// Interface methods:
func (a *AuthRequest) GetID() string              { return a.ID }
func (a *AuthRequest) GetSubject() string          { return a.UserID }
func (a *AuthRequest) Done() bool                  { return a.IsDone }
// ... all other getters
```

---

## 12. Federated Login Handlers

### `/login/federated` -- Initiation

1. Read `authRequestID` from query params.
2. Build upstream OAuth2 authorization URL (e.g., Google):
   - `response_type=code`
   - `client_id=<our-google-client-id>`
   - `redirect_uri=https://accounts.hss-science.org/login/federated/callback`
   - `scope=openid email profile`
   - `state=<encrypt(authRequestID + provider + nonce)>`
3. Redirect user to Google.

### `/login/federated/callback` -- Completion

1. Read `code` and `state` from query params.
2. Decrypt/verify `state`, extract `authRequestID` and `provider`.
3. Exchange `code` for tokens with the upstream IdP (server-to-server).
4. Verify the upstream ID token. Extract claims: `sub`, `email`, `email_verified`, `name`, `given_name`, `family_name`, `picture`.
5. **Upsert user:**
   - Query `federated_identities` for `(provider, provider_subject)`.
   - If found: load the associated user.
   - If not found: create a new user from the upstream claims, create the federated identity link.
6. **Complete the auth request:**
   - `authReqRepo.CompleteLogin(ctx, authRequestID, user.ID, time.Now(), []string{"federated"})`
   - This sets `UserID`, `AuthTime`, `AMR`, `IsDone=true`.
7. Redirect to `{issuer}/authorize/callback?id={authRequestID}`.

The `IssuerInterceptor` is needed on the callback handler so we can construct the callback URL from `op.IssuerFromContext(ctx)`.

---

## 13. Configuration

All configuration via environment variables:

```go
type Config struct {
    // Server
    Port     string  // default: "8080"
    Issuer   string  // default: "https://accounts.hss-science.org"
    DevMode  bool    // default: false; enables http:// issuer

    // Database
    DatabaseURL string  // PostgreSQL connection string

    // Signing
    SigningKeyPEM string  // RSA private key in PEM format

    // OIDC
    CryptoKey string  // 32-byte hex-encoded AES key for opaque token encryption

    // Upstream IdPs
    GoogleClientID     string
    GoogleClientSecret string
    GitHubClientID     string
    GitHubClientSecret string

    // Client Secrets (for our RP clients)
    // Each BFF/service client secret is injected via env
    MyAccountBFFSecret string
    DriveBFFSecret     string
    ChatBFFSecret      string
    ChatServiceSecret  string
    DriveServiceSecret string
}
```

---

## 14. `op.Config` Construction

```go
opConfig := &op.Config{
    CryptoKey:                cryptoKey,                // [32]byte from env
    DefaultLogoutRedirectURI: "/logged-out",
    CodeMethodS256:           true,                     // PKCE mandatory
    AuthMethodPost:           true,                     // Allow form-post client auth
    AuthMethodPrivateKeyJWT:  false,                    // Not needed
    GrantTypeRefreshToken:    true,                     // BFF needs refresh tokens
    RequestObjectSupported:   false,                    // Not needed initially
    SupportedUILocales:       []language.Tag{language.English, language.Japanese},
}
```

Options:
```go
opts := []op.Option{
    op.WithLogger(logger.WithGroup("oidc")),
}
if config.DevMode {
    opts = append(opts, op.WithAllowInsecure())
}
```

---

## 15. HTTP Server Wiring

```go
func main() {
    // 1. Load config from env
    // 2. Connect to PostgreSQL
    // 3. Initialize repositories
    // 4. Initialize Storage
    // 5. Create OpenIDProvider

    provider, err := op.NewOpenIDProvider(config.Issuer, opConfig, storage, opts...)

    // 6. Build router
    router := chi.NewRouter()
    router.Use(middleware.Logger)
    router.Use(middleware.Recoverer)

    // 7. Mount login handlers (with IssuerInterceptor)
    interceptor := op.NewIssuerInterceptor(provider.IssuerFromRequest)
    loginHandler := login.NewHandler(storage, upstreamConfig, op.AuthCallbackURL(provider))
    router.Route("/login", func(r chi.Router) {
        r.Use(interceptor.Handler)
        r.Get("/federated", loginHandler.Initiate)
        r.Get("/federated/callback", loginHandler.Callback)
    })

    // 8. Mount static pages
    router.Get("/logged-out", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("You have been signed out."))
    })

    // 9. Mount the OP handler on root (so /.well-known/openid-configuration works)
    router.Mount("/", provider)

    // 10. Start server
    http.ListenAndServe(":"+config.Port, router)
}
```

---

## 16. Database Schema (Conceptual)

### `users`

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK, default `gen_random_uuid()` |
| `email` | `TEXT NOT NULL` | |
| `email_verified` | `BOOLEAN NOT NULL DEFAULT false` | |
| `name` | `TEXT` | Display name |
| `given_name` | `TEXT` | |
| `family_name` | `TEXT` | |
| `picture` | `TEXT` | URL |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |
| `updated_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

### `federated_identities`

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK |
| `user_id` | `UUID NOT NULL` | FK -> users |
| `provider` | `TEXT NOT NULL` | e.g., "google", "github" |
| `provider_subject` | `TEXT NOT NULL` | Upstream `sub` claim |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |
| | | UNIQUE(`provider`, `provider_subject`) |

### `auth_requests`

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK |
| `client_id` | `TEXT NOT NULL` | |
| `redirect_uri` | `TEXT NOT NULL` | |
| `state` | `TEXT` | |
| `nonce` | `TEXT` | |
| `scopes` | `TEXT[]` | PostgreSQL array |
| `response_type` | `TEXT NOT NULL` | |
| `response_mode` | `TEXT` | |
| `code_challenge` | `TEXT` | |
| `code_challenge_method` | `TEXT` | |
| `prompt` | `TEXT[]` | |
| `max_age` | `INTEGER` | seconds, nullable |
| `login_hint` | `TEXT` | |
| `user_id` | `UUID` | Set after login |
| `auth_time` | `TIMESTAMPTZ` | Set after login |
| `amr` | `TEXT[]` | Set after login |
| `is_done` | `BOOLEAN NOT NULL DEFAULT false` | |
| `code` | `TEXT` | Authorization code, set after auth |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

Index: `auth_requests_code_idx` on `code` (for `AuthRequestByCode` lookup).

### `tokens`

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK (this is the `jti` in JWT access tokens) |
| `client_id` | `TEXT NOT NULL` | |
| `subject` | `UUID NOT NULL` | FK -> users |
| `audience` | `TEXT[]` | |
| `scopes` | `TEXT[]` | |
| `expiration` | `TIMESTAMPTZ NOT NULL` | |
| `refresh_token_id` | `UUID` | FK -> refresh_tokens, nullable |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

### `refresh_tokens`

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK |
| `token` | `TEXT NOT NULL UNIQUE` | The actual refresh token string |
| `client_id` | `TEXT NOT NULL` | |
| `user_id` | `UUID NOT NULL` | FK -> users |
| `audience` | `TEXT[]` | |
| `scopes` | `TEXT[]` | |
| `auth_time` | `TIMESTAMPTZ NOT NULL` | |
| `amr` | `TEXT[]` | |
| `access_token_id` | `UUID` | FK -> tokens |
| `expiration` | `TIMESTAMPTZ NOT NULL` | |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

---

## 17. Dockerfile

```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -o /accounts ./services/accounts/

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /accounts /accounts
ENTRYPOINT ["/accounts"]
```

Context is repo root (per `release.yaml`), so COPY paths are relative to the repo root.

---

## 18. Scopes and UserInfo Mapping

When the library calls `SetUserinfoFromScopes` or `SetUserinfoFromRequest`, we map OIDC standard scopes to user fields:

| Scope | Claims Set |
|---|---|
| `openid` | `sub` (always the user's UUID) |
| `profile` | `name`, `given_name`, `family_name`, `picture`, `updated_at` |
| `email` | `email`, `email_verified` |
| `phone` | (not supported -- return nothing) |
| `address` | (not supported -- return nothing) |

Custom scopes: none initially.

---

## 19. Client Credentials Flow (Service-to-Service)

For S2S calls, services authenticate with their own `client_id` + `client_secret` and receive an access token without any user context.

The `ClientCredentialsStorage` interface methods:
- `ClientCredentials(ctx, clientID, clientSecret)`: Looks up client, verifies hashed secret.
- `ClientCredentialsTokenRequest(ctx, clientID, scopes)`: Returns a `TokenRequest` with the client ID as subject and the client ID in the audience.

The resulting JWT access token will have:
- `sub` = client ID (the service itself is the subject)
- `aud` = the target audience (or the client ID if none specified)
- `scope` = the requested scopes

Resource servers verify these tokens the same way as user tokens -- via JWKS.

---

## 20. Testing Strategy

Aligned with the README's "Unit Test-centric" policy:

- **`oidcprovider/storage_test.go`**: Test each `op.Storage` method with a real PostgreSQL database (using `testcontainers-go` or an in-memory mock). Focus: correct interface contract behavior, edge cases (expired tokens, missing auth requests, etc.).
- **`login/handler_test.go`**: Test federated callback logic with mocked upstream IdP responses (httptest server).
- **`repo/*_test.go`**: Integration tests for each repository against PostgreSQL.
- **`model/` and `config/`**: Standard unit tests.

No E2E tests in the repo (per README policy -- E2E is done in staging).

---

## 21. Observability

- **Logging**: `log/slog` (standard library), passed to the OP via `op.WithLogger()`.
- **Tracing**: The `zitadel/oidc` library has built-in OpenTelemetry spans. We'll add OTel instrumentation to our handlers using the standard `go.opentelemetry.io/otel` SDK. Exporter configuration via environment variables (OTLP endpoint).
- **Health**: `GET /healthz` (from the library's built-in health endpoint) + `GET /readyz` (checks DB connectivity via `Storage.Health()`).

---

## 22. Security Considerations

- **PKCE required**: `CodeMethodS256: true` in config. All public clients must use PKCE. Confidential clients should too.
- **State parameter**: The upstream federated login state is encrypted (AES-GCM) and includes a nonce to prevent replay.
- **Refresh token rotation**: Every refresh token use invalidates the old token and issues a new one. A reuse of an old refresh token indicates token theft and should invalidate the entire family (not implemented initially, but the architecture supports it).
- **Client secrets**: Stored as bcrypt hashes. Compared via `bcrypt.CompareHashAndPassword`.
- **No tokens in browser**: Access/refresh tokens are never exposed to SPAs. They exist only in the BFF layer (per the architecture docs).
- **Short-lived access tokens**: 15 minutes. Limits the damage window if a token is leaked.

---

## 23. Open Questions for Review

1. **Database migrations**: SQL schema definitions will live in this repo, but execution is out of scope (per README). Should migration files go in `server/services/accounts/migrations/` or a top-level `migrations/` directory?
> Author Annotation: Since each service should own and manage its own database schema (following the Bounded Context principle in microservices), it should be placed within the service’s directory — `server/services/accounts/migrations/` — rather than at the top level.

2. **Upstream IdP selection UI**: When multiple upstream IdPs are supported (Google + GitHub), do we present a provider selection page, or do we default to Google and add GitHub as a secondary option later?
> Author Annotation: We must present a provider selection page for the upstream IdP. Even if there is only one upstream IdP configured initially, always display this selection page so users explicitly choose their login method.

3. **User merging**: If a user signs in with Google first and later with GitHub using the same email, should we auto-link the accounts? This has security implications (email verification trust).
> Author Annotation: Do not auto-link accounts, even if they share the same email address. Treat them as entirely separate accounts. To reduce security risks, this IdP will not perform its own identity or email verification, so we cannot safely merge them based solely on email matching.

4. **Logout**: The `TerminateSession` implementation will revoke tokens, but we don't have a cookie-based SSO session. Should the OP set its own session cookie to enable true single sign-on (user isn't re-prompted for login if they already have a valid session)?
> Author Annotation: Keep it stateless. We do not need a custom SSO session cookie for the OP. We will rely entirely on the upstream IdPs for session state management.

5. **Admin client registration**: For now clients are hardcoded. When should we plan for database-backed dynamic client registration?
> Author Annotation: Hardcoding clients and their secrets in the source code is a major security risk, and using environment variables for multiple clients is unscalable. Therefore, we must use database-backed client storage (PostgreSQL) from Phase 1. I will leave the specific implementation details up to you in the plan. Note that we do not need to build an Admin UI for client registration right now; we will manually insert the client data into the DB using SQL.