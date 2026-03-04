# Accounts Service — Architecture Research

> **Scope:** Production code only in `server/services/accounts/`. Test files excluded.
> **Purpose:** Factual inventory of what exists and how it fits together, as a foundation for the `myaccount` gRPC domain.

---

## 1. Module & Runtime

| Item | Value |
|---|---|
| Go module | `github.com/barn0w1/hss-science/server` |
| Entrypoint | `services/accounts/main.go` |
| HTTP router | `github.com/go-chi/chi/v5` |
| Database driver | `github.com/jmoiron/sqlx` + `github.com/lib/pq` (PostgreSQL) |
| OIDC library | `github.com/zitadel/oidc/v3` |
| ID generation | `github.com/oklog/ulid/v2` |
| Crypto | `golang.org/x/crypto/bcrypt`, standard library AES-GCM |

The binary accepts a subcommand:
- `server` (default) — starts the HTTP server
- `cleanup` — runs one-shot token/auth-request cleanup and exits

---

## 2. Overall Architecture

The service follows **hexagonal architecture** (ports & adapters) with three layers:

```
┌─────────────────────────────────────────────────────────┐
│  HTTP / OIDC Library boundary                           │
│  main.go · authn/handler.go · oidc/adapter/storage.go  │
└────────────────────────┬────────────────────────────────┘
                         │ calls
┌────────────────────────▼────────────────────────────────┐
│  Application / Service layer                            │
│  identity/service.go · oidc/*_svc.go                   │
│  authn/login_usecase.go                                 │
└──────────┬──────────────────────┬───────────────────────┘
           │ Repository ports     │ Service ports
┌──────────▼──────────┐  ┌────────▼────────────────────────┐
│  identity/ports.go  │  │  oidc/ports.go                  │
│  (Repository iface) │  │  (Repository + Service ifaces)  │
└──────────┬──────────┘  └────────┬────────────────────────┘
           │ implements            │ implements
┌──────────▼──────────┐  ┌────────▼────────────────────────┐
│  identity/postgres  │  │  oidc/postgres                  │
│  user_repo.go       │  │  authrequest_repo.go            │
└─────────────────────┘  │  client_repo.go                 │
                         │  token_repo.go                  │
                         └─────────────────────────────────┘
```

**Dependency injection is manual**, wired entirely in `main.go`. No DI framework.

---

## 3. Package Inventory

### `config/`
`Config` struct loaded from environment variables via a testable `ConfigSource` interface.

Key fields relevant to `myaccount`:

```go
type Config struct {
    Port        string
    Issuer      string      // validated URL
    DatabaseURL string
    CryptoKey   [32]byte    // AES-256-GCM key

    AccessTokenLifetimeMinutes int
    RefreshTokenLifetimeDays   int
    AuthRequestTTLMinutes      int

    DBMaxOpenConns, DBMaxIdleConns int
    DBConnMaxLifetimeSecs, DBConnMaxIdleTimeSecs int

    RateLimitEnabled              bool
    RateLimitLoginRPM, RateLimitTokenRPM, RateLimitGlobalRPM int

    GoogleClientID, GoogleClientSecret string
    GitHubClientID, GitHubClientSecret string
}
```

`LoadFrom(src ConfigSource)` is the testable constructor.

---

### `internal/identity/`

**Domain objects** (`domain.go`):

```go
type User struct {
    ID, Email, Name, GivenName, FamilyName, Picture string
    EmailVerified bool
    CreatedAt, UpdatedAt time.Time
}

type FederatedIdentity struct {
    ID, UserID, Provider, ProviderSubject string
    ProviderEmail, ProviderDisplayName, ProviderGivenName, FamilyName, PictureURL string
    ProviderEmailVerified bool
    LastLoginAt, CreatedAt, UpdatedAt time.Time
}

type FederatedClaims struct { /* subset of User fields from the external provider */ }
```

**Repository port** (`ports.go`):

```go
type Repository interface {
    GetByID(ctx, id)                                              -> *User
    FindByFederatedIdentity(ctx, provider, providerSubject)       -> *User
    CreateWithFederatedIdentity(ctx, *User, *FederatedIdentity)   -> error
    UpdateUserFromClaims(ctx, userID, FederatedClaims, updatedAt) -> error
    UpdateFederatedIdentityClaims(ctx, provider, providerSubject, FederatedClaims, lastLoginAt) -> error
}
```

**Service port** (`ports.go`):

```go
type Service interface {
    GetUser(ctx, userID)                                    -> *User
    FindOrCreateByFederatedLogin(ctx, provider, FederatedClaims) -> *User
}
```

**Service implementation** (`service.go`):
- ULID-based ID generation via `newID()`
- `FindOrCreateByFederatedLogin`: idempotent find-or-create; on existing user, updates both `users` and `federated_identities` records in the same call

**Postgres implementation** (`postgres/user_repo.go`):
- `GetByID`: `SELECT ... FROM users WHERE id = $1`
- `FindByFederatedIdentity`: JOIN across `users` and `federated_identities`
- `CreateWithFederatedIdentity`: explicit transaction, inserts both rows atomically
- All errors mapped to `domerr` sentinels; `sql.ErrNoRows` → `domerr.ErrNotFound`

**Database tables backing this package:**
- `users(id, email, email_verified, name, given_name, family_name, picture, created_at, updated_at)`
- `federated_identities(id, user_id, provider, provider_subject, provider_email, provider_email_verified, provider_display_name, provider_given_name, provider_family_name, provider_picture_url, last_login_at, created_at, updated_at)`

---

### `internal/oidc/`

**Domain objects** (`domain.go`):

| Type | Purpose |
|---|---|
| `AuthRequest` | Tracks one authorization flow (code + PKCE state) |
| `Client` | OIDC client configuration |
| `Token` | Issued access token record |
| `RefreshToken` | Issued refresh token record (raw token hashed on write) |

**Repository ports** (`ports.go`): `AuthRequestRepository`, `ClientRepository`, `TokenRepository`

**Service ports** (`ports.go`): `AuthRequestService`, `ClientService`, `TokenService`, `LoginCompleter`

**Service implementations:**

| File | Type | Key behaviour |
|---|---|---|
| `authrequest_svc.go` | `authRequestService` | Enforces TTL on `GetByID`/`GetByCode`; delegates mutations to repo |
| `client_svc.go` | `clientService` | `AuthorizeSecret` via bcrypt; `ClientCredentials` for m2m grant |
| `token_svc.go` | `tokenService` | ULID access IDs; 32-byte random refresh tokens; SHA-256 hashing; rotation on reuse |

**Postgres implementations** (`postgres/`):

| File | Tables |
|---|---|
| `authrequest_repo.go` | `auth_requests` |
| `client_repo.go` | `clients` (read-only) |
| `token_repo.go` | `tokens`, `refresh_tokens` |

PostgreSQL arrays used extensively (`pq.StringArray`) for scopes, URIs, grant types, AMR.

`token_repo.go` — `CreateAccessAndRefresh` is a 5-step transaction: lookup old access token ID via refresh token → delete old access token → conditionally delete old refresh token (guards against replay) → insert new access token → insert new refresh token.

**Database tables backing this package:**
- `clients(id, secret_hash, redirect_uris[], post_logout_redirect_uris[], application_type, auth_method, response_types[], grant_types[], access_token_type, allowed_scopes[], id_token_lifetime_seconds, clock_skew_seconds, id_token_userinfo_assertion, created_at, updated_at)`
- `auth_requests(id, client_id, redirect_uri, state, nonce, scopes[], response_type, response_mode, code_challenge, code_challenge_method, prompt[], max_age, login_hint, user_id, auth_time, amr[], is_done, code, created_at)`
- `tokens(id, client_id, subject, audience[], scopes[], expiration, refresh_token_id, created_at)`
- `refresh_tokens(id, token_hash, client_id, user_id, audience[], scopes[], auth_time, amr[], access_token_id, expiration, created_at)`

---

### `internal/authn/`

Implements the three-step federated login flow:

```
GET  /login          → handler.SelectProvider   — renders provider-selection HTML
POST /login/select   → handler.FederatedRedirect — encrypts state, redirects to upstream OAuth2
GET  /login/callback → handler.FederatedCallback — verifies state, fetches claims, runs use case
```

**`CompleteFederatedLogin` use case** (`login_usecase.go`):
1. Calls `identity.Service.FindOrCreateByFederatedLogin` → returns `*User`
2. Calls `oidcdom.LoginCompleter.CompleteLogin` (sets `userID`, `authTime`, `amr=["fed"]` on `AuthRequest`)
3. Returns `userID`

**Provider abstraction** (`provider.go`, `provider_github.go`, `provider_google.go`):

```go
type ClaimsProvider interface {
    FetchClaims(ctx, *oauth2.Token) (*identity.FederatedClaims, error)
}

type Provider struct {
    Name, DisplayName  string
    OAuth2Config       *oauth2.Config
    Claims             ClaimsProvider
}
```

- Google: OIDC discovery + ID token verification (`coreos/go-oidc`)
- GitHub: REST API calls to `/user` and `/user/emails`

**OAuth2 state security**: `federatedState{AuthRequestID, Provider, Nonce}` is JSON-marshalled, then AES-256-GCM encrypted via `crypto.Cipher`.

---

### `internal/oidc/adapter/`

Bridges the domain services to the `zitadel/oidc` library.

| File | Type | Implements |
|---|---|---|
| `storage.go` | `StorageAdapter` | `op.Storage` (central integration point) |
| `authrequest.go` | `AuthRequest` | `op.AuthRequest` |
| `client.go` | `ClientAdapter` | `op.Client` |
| `refreshtoken.go` | `RefreshTokenRequest` | `op.RefreshTokenRequest` |
| `keys.go` | `SigningKeyWithID`, `PublicKeySet` | `op.SigningKey`, `op.Key` |
| `userinfo.go` | `UserClaims`, `UserClaimsSource` | interface (consumed by StorageAdapter) |
| `provider.go` | – | constructs `*op.Provider` with config |

**`StorageAdapter`** dependencies:

```go
type StorageAdapter struct {
    users      UserClaimsSource       // bridge to identity.Service (via userClaimsBridge in main.go)
    authReqs   oidcdom.AuthRequestService
    clients    oidcdom.ClientService
    tokens     oidcdom.TokenService
    signing    *SigningKeyWithID
    publicKeys *PublicKeySet
    accessTTL  time.Duration
    refreshTTL time.Duration
    healthCheck func(context.Context) error
}
```

**Scope → userinfo mapping** (in `setUserinfo`):
- `openid` → Subject
- `profile` → Name, GivenName, FamilyName, Picture, UpdatedAt
- `email` → Email, EmailVerified

**`UserClaimsSource`** interface (`userinfo.go`) is the seam between OIDC adapter and identity domain:

```go
type UserClaimsSource interface {
    UserClaims(ctx context.Context, userID string) (*UserClaims, error)
}
```

Implemented in `main.go` by `userClaimsBridge`, which calls `identity.Service.GetUser` and maps to `UserClaims`.

---

### `internal/middleware/`

| File | Type | Behaviour |
|---|---|---|
| `ratelimit.go` | `IPRateLimiter` | Token bucket per IP; skips loopback and RFC-1918 addresses; extracts IP from CF-Connecting-IP → X-Forwarded-For → RemoteAddr |
| `securityheaders.go` | – | Adds HSTS, CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy, Permissions-Policy |

---

### `internal/pkg/`

| Package | Type | Behaviour |
|---|---|---|
| `crypto` | `Cipher` / `AESCipher` | AES-256-GCM; nonce prepended to ciphertext; base64url output |
| `domerr` | sentinel errors | `ErrNotFound`, `ErrAlreadyExists`, `ErrUnauthorized`, `ErrInternal` |

---

### `migrations/`

SQL migration files embedded at compile time via `embed.FS` into `migrations.FS`. The service does **not** auto-migrate on startup; `testhelper.RunMigrations` applies them in tests only.

---

## 4. HTTP Server Structure

```
chi.Router
├── Recoverer                          (chi built-in)
├── SecurityHeaders()
├── GlobalIPRateLimiter.Middleware()
├── op.NewIssuerInterceptor(...)       (zitadel — rewrites issuer for discovery)
│
├── GET  /login                        → authn.Handler.SelectProvider
├── POST /login/select   + limiter     → authn.Handler.FederatedRedirect
├── GET  /login/callback + limiter     → authn.Handler.FederatedCallback
├── GET  /healthz                      → 200 OK
├── GET  /readyz                       → DB ping
├── GET  /logged-out                   → static message
└── /                                  → *op.Provider (zitadel OIDC router)
                                          /.well-known/openid-configuration
                                          /oauth/v2/authorize
                                          /oauth/v2/token          + tokenLimiter
                                          /oauth/v2/introspect
                                          /oauth/v2/revoke
                                          /oauth/v2/end_session
                                          /keys
                                          /userinfo
```

---

## 5. Background Goroutines

| Goroutine | Trigger | Purpose |
|---|---|---|
| `runAuthRequestCleanup` | every `AuthRequestTTL` | DELETE from `auth_requests` where `created_at < (now - TTL)` |
| `runTokenCleanupLoop` | every 1 hour | DELETE expired rows from `tokens` and `refresh_tokens` |
| `IPRateLimiter.Cleanup` | every 10 minutes | Remove idle IP entries from in-memory map |

All are cancellable via context; server shutdown (SIGINT/SIGTERM) cancels root context with a 15-second drain.

---

## 6. Dependency Wiring (main.go)

```
sqlx.DB
 ├─► identity/postgres.UserRepository          → identity.NewService()
 │                                               ↓ identity.Service
 ├─► oidc/postgres.AuthRequestRepository       → oidc.NewAuthRequestService()
 ├─► oidc/postgres.ClientRepository            → oidc.NewClientService()
 └─► oidc/postgres.TokenRepository             → oidc.NewTokenService()
                                                 ↓ oidc.TokenService

config.SigningKeys
 ├─► adapter.NewSigningKey(current)
 └─► adapter.NewPublicKeySet(current, previous[])

adapter.NewStorageAdapter(
    userClaimsBridge{identity.Service},         ← UserClaimsSource bridge
    oidc.AuthRequestService,
    oidc.ClientService,
    oidc.TokenService,
    signingKey, publicKeySet,
    accessTTL, refreshTTL,
    dbHealthCheck,
)
 ↓ op.Storage

adapter.NewProvider(issuer, cryptoKey, storage) → *op.Provider

authn.NewProviders(ctx, authnConfig)            → []*authn.Provider
authn.NewCompleteFederatedLogin(identitySvc, authReqSvc)
authn.NewHandler(providers, identitySvc, loginUC, cipher, callbackURL, logger)

chi.Router (assembled as above)
```

---

## 7. Error Handling Convention

```
Infrastructure layer  →  domerr sentinels  →  Service layer  →  protocol errors (OIDC / HTTP)
```

Mapping in `oidc/adapter/storage.go`:
- `domerr.ErrNotFound` → `oidc.ErrInvalidClient` / `oidc.ErrNotFound`
- `domerr.ErrUnauthorized` → `oidc.ErrInvalidClient`
- anything else → logged, then generic `internal_error` string returned to client

---

## 8. Security Properties

| Concern | Mechanism |
|---|---|
| OAuth2 state forgery | AES-256-GCM encryption (`crypto.AESCipher`) |
| Client secret storage | bcrypt hash in `clients.secret_hash` |
| Refresh token theft | SHA-256 hash stored, raw token returned once; rotation on reuse |
| Refresh token replay | Atomic transaction: old token must not be expired at rotation time |
| PKCE | `CodeMethodS256` enforced; public clients (authMethod="none") must supply challenge |
| RSA signing | RS256; key ID derived from SHA-256 of public key DER, first 8 bytes hex |
| Key rotation | `config.SigningKeySet{Current, Previous[]}` exposed via JWKS |
| Rate limiting | Token bucket per IP; internal IPs bypassed |
| Security headers | HSTS 2yr, strict CSP, no framing |

---

## 9. Integration Points for `myaccount` gRPC Domain

The following existing objects are directly reusable without modification:

### Repositories (read-only access patterns)

| Need | Existing port | Existing implementation |
|---|---|---|
| Look up a user by ID | `identity.Repository.GetByID` | `identity/postgres.UserRepository` |
| List a user's linked providers | *(not yet a port method — needs new query on `federated_identities`)* | add to `UserRepository` |
| Look up active tokens for a user | `oidc.TokenRepository.GetByID`, `DeleteByUserAndClient` | `oidc/postgres.TokenRepository` |
| Revoke a session | `oidc.TokenService.DeleteByUserAndClient` | `oidc.tokenService` |

### Services (directly callable)

| Need | Existing service | Method |
|---|---|---|
| Read user profile | `identity.Service` | `GetUser(ctx, userID)` |
| Revoke all sessions for user+client | `oidc.TokenService` | `DeleteByUserAndClient(ctx, userID, clientID)` |
| Revoke a specific refresh token | `oidc.TokenService` | `RevokeRefreshToken(ctx, rawToken, clientID)` |

### Infrastructure packages (reusable as-is)

| Package | Reuse |
|---|---|
| `internal/pkg/domerr` | Use same sentinel errors in new gRPC handler; map to `codes.NotFound`, `codes.Unauthenticated`, etc. |
| `internal/pkg/crypto` | `Cipher` interface available if any encrypted fields needed |
| `config` | `ConfigSource` abstraction makes adding new env vars straightforward |
| `migrations/embed.go` | New `.up.sql` / `.down.sql` files added here for any new tables |

### Pattern to follow for new gRPC handler

The existing pattern for adding a new transport endpoint is:

1. Define the domain port (interface) in `internal/<domain>/ports.go`
2. Implement in `internal/<domain>/postgres/` or reuse existing repos
3. Write a service in `internal/<domain>/service.go`
4. Write a gRPC handler that accepts service interfaces, maps proto ↔ domain types, and maps `domerr` sentinels to gRPC status codes
5. Wire in `main.go` alongside the existing HTTP server (gRPC listener on a separate port or via a shared multiplexer)

### Token-based caller authentication in gRPC

The `oidc.TokenRepository.GetByID` method returns an `*oidc.Token` which contains `Subject` (the user ID). A gRPC server-side interceptor can:
1. Extract the `Authorization: Bearer <token_id>` header from incoming metadata
2. Call `tokenService.GetByID(ctx, tokenID)` — returns `ErrNotFound` for expired/missing tokens
3. Inject the resolved `Subject` (user ID) into the request context for handler use

---

## 10. Database Schema Summary

```
users
  id PK · email · email_verified · name · given_name · family_name · picture · created_at · updated_at

federated_identities
  id PK · user_id FK(users) · provider · provider_subject
  provider_email · provider_email_verified · provider_display_name
  provider_given_name · provider_family_name · provider_picture_url
  last_login_at · created_at · updated_at

clients
  id PK · secret_hash · redirect_uris[] · post_logout_redirect_uris[]
  application_type · auth_method · response_types[] · grant_types[]
  access_token_type · allowed_scopes[] · id_token_lifetime_seconds
  clock_skew_seconds · id_token_userinfo_assertion · created_at · updated_at

auth_requests
  id PK · client_id · redirect_uri · state · nonce · scopes[]
  response_type · response_mode · code_challenge · code_challenge_method
  prompt[] · max_age · login_hint · user_id · auth_time · amr[]
  is_done · code · created_at

tokens
  id PK · client_id · subject(=user_id) · audience[] · scopes[]
  expiration · refresh_token_id · created_at

refresh_tokens
  id PK · token_hash(SHA-256) · client_id · user_id FK(users)
  audience[] · scopes[] · auth_time · amr[] · access_token_id FK(tokens)
  expiration · created_at
```

---

## 11. What Does Not Exist Yet

The following are absent from the current codebase and would need to be built for `myaccount`:

- A gRPC server listener and registration in `main.go`
- A proto definition for the `myaccount` service (lives in `api/proto/`)
- Generated gRPC stubs (`api/proto/` + `buf.gen.yaml` already present)
- A gRPC authentication interceptor (token → user ID)
- Read access to `federated_identities` beyond the `FindByFederatedIdentity` JOIN (e.g., listing all linked providers for a user)
- Any write operations on the user profile (e.g., change display name) — not currently in any port
- Session listing (no query exists to enumerate all active tokens for a user)
