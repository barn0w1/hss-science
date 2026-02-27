# Accounts Service -- Implementation Plan

> **Status**: Revised -- all author annotations from Round 1 are incorporated.
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
- Key ID (`kid`) derived deterministically from the key (e.g., SHA-256 hash of the DER-encoded public key, truncated to 16 hex chars), so it's stable across restarts.
- `SigningKey()` returns this key. `KeySet()` returns the corresponding public key.
- Key rotation is out of scope for the initial implementation. When needed, we'll add multi-key support (active signing key + retired verification-only keys in the JWKS).

---

## 4. Design Decision: Issuer Strategy

**Choice: Static issuer, production-only HTTPS**

Value: `https://accounts.hss-science.org` (configurable via environment variable).

Rationale:
- We are not multi-tenant. One issuer, one OP instance.
- Simpler configuration, simpler token verification for resource servers.

No local development overrides are implemented. No `op.WithAllowInsecure()`, no `localhost` fallbacks, no `DevMode` toggle. The service is built exclusively for production-grade HTTPS deployment.

---

## 5. Design Decision: Federated Authentication Flow

Our OP does NOT authenticate users directly. Authentication is **federated to upstream IdPs** (Google, GitHub). Here is how the login flow works within the `zitadel/oidc` library's callback mechanism:

```
Relying Party (e.g., drive.hss-science.org)
    |
    | 1. GET /authorize?client_id=drive-bff&redirect_uri=...&scope=openid+profile+email
    v
Our OP (accounts.hss-science.org)
    |
    | 2. Library creates AuthRequest, calls Storage.CreateAuthRequest()
    | 3. AuthRequest.Done() == false -> redirects to Client.LoginURL(authRequestID)
    v
Provider Selection Page (GET /login?authRequestID=xxx)
    |
    | 4. Renders HTML page listing configured upstream IdPs (e.g., "Sign in with Google", "Sign in with GitHub")
    | 5. User clicks a provider
    | 6. POST /login/select?authRequestID=xxx&provider=google
    v
Federated Redirect Handler (POST /login/select)
    |
    | 7. Builds upstream OAuth2 authorize URL for the chosen provider
    | 8. Stores authRequestID + provider in `state` parameter (encrypted/signed)
    | 9. Redirects user to the upstream IdP (e.g., Google)
    v
Google / GitHub
    |
    | 10. User authenticates at the upstream IdP
    | 11. IdP redirects to our callback: GET /login/callback?code=xxx&state=yyy
    v
Our Federated Callback Handler (GET /login/callback)
    |
    | 12. Exchanges code for tokens with the upstream IdP (server-to-server)
    | 13. Extracts user identity from the upstream ID token (sub, email, name, picture)
    | 14. Finds or creates user in our database (keyed by provider+provider_subject, NO email-based linking)
    | 15. Updates AuthRequest in storage: sets UserID, marks Done=true, records AuthTime
    | 16. Redirects to op.AuthCallbackURL: /authorize/callback?id=authRequestID
    v
Our OP (library-handled callback)
    |
    | 17. Library fetches AuthRequest by ID, sees Done()==true
    | 18. Issues authorization code, redirects to RP's redirect_uri
    v
Relying Party
    |
    | 19. Exchanges code for tokens at POST /oauth/token
    v
Our OP
    |
    | 20. Library issues access_token (JWT) + id_token + refresh_token
```

### Key design points:

1. **Provider selection page is always shown.** Even if only one upstream IdP is configured, the user sees the selection page and explicitly chooses their login method. This ensures the UX is consistent and ready for multi-provider support.

2. **No account auto-linking.** Each `(provider, provider_subject)` pair maps to exactly one user in our database. If the same person signs in with Google and then with GitHub (even with the same email), they get two separate user accounts. We do not perform email-based identity merging because we do not independently verify email addresses.

3. **Stateless OP.** The OP does not set its own session cookie. Each authorization request triggers a full login flow through the upstream IdP. If the upstream IdP has an active session (e.g., the user is already signed into Google), the IdP may auto-approve without re-prompting, giving the effect of SSO. But this is the upstream IdP's responsibility, not ours.

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

Note: `email` is NOT unique. Multiple user accounts (from different upstream IdPs) can share the same email address. This is by design -- we do not perform email-based account linking.

### 6.5 Federated Identities -- PostgreSQL

Links upstream IdP identities to our users:
- `user_id` (FK -> users)
- `provider` (e.g., "google", "github")
- `provider_subject` (the `sub` claim from the upstream IdP)
- Unique constraint on `(provider, provider_subject)`

This is a strict one-to-one mapping. One federated identity record maps to exactly one user. There is no mechanism to merge identities.

### 6.6 Clients -- PostgreSQL (database-backed)

OAuth/OIDC client registrations are stored in PostgreSQL. This avoids hardcoding secrets in source code and supports adding/modifying clients without redeploying.

For the initial deployment, clients will be inserted manually via SQL. No admin UI or dynamic registration API is needed yet. The `GetClientByClientID` and `AuthorizeClientIDSecret` storage methods will query the `clients` table.

Client secrets are stored as bcrypt hashes in the database.

### 6.7 Signing Keys -- Environment variable

Loaded at startup. Stored in-memory. Not persisted to DB.

### 6.8 Database Migrations

Migration files will live in `server/services/accounts/migrations/`, following the bounded context principle -- each service owns its own schema. Migration execution is out of scope for this codebase (handled by infrastructure tooling).

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
├── migrations/
│   ├── 001_initial.sql        # Schema: users, federated_identities, auth_requests, tokens, refresh_tokens, clients
│   └── 002_seed_clients.sql   # Initial client registrations (placeholder secrets)
├── config/
│   └── config.go              # Env-var based configuration struct
├── oidcprovider/
│   ├── provider.go            # op.NewOpenIDProvider construction and router setup
│   ├── storage.go             # op.Storage implementation (orchestrator, delegates to repos)
│   ├── authrequest.go         # AuthRequest struct implementing op.AuthRequest interface
│   ├── client.go              # Client struct implementing op.Client interface (loaded from DB)
│   ├── refreshtoken.go        # RefreshTokenRequest struct implementing op.RefreshTokenRequest
│   └── keys.go                # SigningKey / PublicKey structs implementing op.SigningKey / op.Key
├── login/
│   ├── handler.go             # HTTP handlers: provider selection page, federated redirect, callback
│   ├── upstream.go            # Upstream IdP OAuth2 client configuration (Google, GitHub)
│   └── templates/
│       └── select_provider.html  # Provider selection page template
├── model/
│   ├── user.go                # User domain model
│   ├── federated_identity.go  # FederatedIdentity domain model
│   ├── client.go              # Client domain model
│   ├── token.go               # Token / RefreshToken domain models
│   └── authrequest.go         # AuthRequest domain/DB model
└── repo/
    ├── user.go                # UserRepository (PostgreSQL CRUD)
    ├── client.go              # ClientRepository (PostgreSQL CRUD)
    ├── authrequest.go         # AuthRequestRepository (PostgreSQL CRUD)
    └── token.go               # TokenRepository (PostgreSQL CRUD)
```

### Why this layout

- **`oidcprovider/`**: Everything that directly satisfies the `zitadel/oidc` library contracts. `storage.go` is the central `op.Storage` impl. It delegates to `repo/` for persistence and uses domain models from `model/`.
- **`login/`**: The federated login flow handlers. These are HTTP routes mounted alongside the OP, wrapped with `op.IssuerInterceptor`. Completely separate from the OIDC protocol layer. Includes the provider selection page template.
- **`model/`**: Domain types that are independent of the OIDC library. Pure data structures.
- **`repo/`**: Database access layer. One file per aggregate. Uses `sqlx` + raw SQL (no ORM).
- **`config/`**: Env-var loading. Single struct, populated at startup.
- **`migrations/`**: SQL schema files. Owned by this service (bounded context). Execution is handled externally.

---

## 9. `op.Storage` Implementation Architecture

The `Storage` struct in `oidcprovider/storage.go` will hold references to the repositories and config:

```go
type Storage struct {
    userRepo       *repo.UserRepository
    clientRepo     *repo.ClientRepository
    authReqRepo    *repo.AuthRequestRepository
    tokenRepo      *repo.TokenRepository
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
| `GetClientByClientID` | `clientRepo.GetByID()` | Database query, returns `*Client` implementing `op.Client` |
| `AuthorizeClientIDSecret` | `clientRepo.GetByID()` | Load client from DB, compare bcrypt hash |
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

The `Client` struct in `oidcprovider/client.go` is loaded from the `clients` database table and satisfies the `op.Client` interface:

```go
type Client struct {
    id                             string
    secretHash                     string              // bcrypt hash from DB
    redirectURIs                   []string
    postLogoutRedirectURIs         []string
    applicationType                op.ApplicationType
    authMethod                     oidc.AuthMethod
    responseTypes                  []oidc.ResponseType
    grantTypes                     []oidc.GrantType
    accessTokenType                op.AccessTokenType
    idTokenLifetime                time.Duration
    idTokenUserinfoClaimsAssertion bool
    clockSkew                      time.Duration
}
```

The `LoginURL(id string) string` method is not stored in the database -- it's implemented directly on the struct and always returns `/login?authRequestID=<id>`, pointing to the provider selection page. This is constant across all clients.

`DevMode() bool` always returns `false` -- there is no dev mode.

`IsScopeAllowed(scope string) bool` returns `false` for all custom scopes (we have none). Standard OIDC scopes are handled by the library.

`RestrictAdditionalIdTokenScopes()` and `RestrictAdditionalAccessTokenScopes()` pass through all scopes (no filtering).

### Initial Client Registrations (via SQL)

These clients will be manually inserted into the `clients` table:

| Client ID | Type | Auth Method | Grant Types | Notes |
|---|---|---|---|---|
| `myaccount-bff` | Web | `client_secret_basic` | `authorization_code`, `refresh_token` | BFF for the MyAccount SPA |
| `drive-bff` | Web | `client_secret_basic` | `authorization_code`, `refresh_token` | BFF for the Drive SPA |
| `chat-bff` | Web | `client_secret_basic` | `authorization_code`, `refresh_token` | BFF for the Chat SPA |
| `chat-service` | Web | `client_secret_basic` | `client_credentials` | Chat gRPC service (S2S) |
| `drive-service` | Web | `client_secret_basic` | `client_credentials` | Drive gRPC service (S2S) |

All BFF clients will have `AccessTokenTypeJWT`, `response_type=code`, and `id_token_lifetime=1h`.

A seed SQL file (`migrations/002_seed_clients.sql`) will contain the initial INSERT statements with bcrypt-hashed placeholder secrets that must be replaced with real values before deployment.

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

## 12. Login Handlers

The login flow has three handlers, all mounted under `/login` with the `IssuerInterceptor` middleware:

### `GET /login` -- Provider Selection Page

1. Read `authRequestID` from query params.
2. Validate the auth request exists (fetch from DB to confirm it's pending).
3. Render the `select_provider.html` template, listing all configured upstream IdPs.
   - Each provider is rendered as a form button that POSTs to `/login/select`.
   - The `authRequestID` is included as a hidden form field.
4. The page always lists all configured providers, even if only one exists.

### `POST /login/select` -- Federated Redirect

1. Read `authRequestID` and `provider` from form body.
2. Validate the `provider` value against the configured upstream IdP list.
3. Build upstream OAuth2 authorization URL for the chosen provider:
   - `response_type=code`
   - `client_id=<our-upstream-client-id-for-this-provider>`
   - `redirect_uri=https://accounts.hss-science.org/login/callback`
   - `scope=openid email profile`
   - `state=<encrypt(authRequestID + provider + nonce)>`
4. Redirect user to the upstream IdP.

### `GET /login/callback` -- Federated Completion

1. Read `code` and `state` from query params.
2. Decrypt/verify `state`, extract `authRequestID` and `provider`.
3. Exchange `code` for tokens with the upstream IdP (server-to-server HTTP call).
4. Verify the upstream ID token. Extract claims: `sub`, `email`, `email_verified`, `name`, `given_name`, `family_name`, `picture`.
5. **Find or create user** (strict identity isolation -- no email-based linking):
   - Query `federated_identities` for `(provider, provider_subject)`.
   - If found: load the associated user. Update user profile fields from upstream claims (name, picture, etc.).
   - If not found: create a new user from the upstream claims, create the federated identity link.
   - No email-based matching. No cross-provider account merging.
6. **Complete the auth request:**
   - `authReqRepo.CompleteLogin(ctx, authRequestID, user.ID, time.Now(), []string{"federated"})`
   - This sets `UserID`, `AuthTime`, `AMR`, `IsDone=true`.
7. Redirect to `{issuer}/authorize/callback?id={authRequestID}`.

The `IssuerInterceptor` is applied to all three handlers so we can construct the callback URL from `op.IssuerFromContext(ctx)`.

---

## 13. Configuration

All configuration via environment variables:

```go
type Config struct {
    // Server
    Port     string  // default: "8080"
    Issuer   string  // required, e.g. "https://accounts.hss-science.org"

    // Database
    DatabaseURL string  // required, PostgreSQL connection string

    // Signing
    SigningKeyPEM string  // required, RSA private key in PEM format

    // OIDC
    CryptoKey string  // required, 32-byte hex-encoded AES key for opaque token encryption

    // Upstream IdPs
    GoogleClientID     string  // required if Google is enabled
    GoogleClientSecret string  // required if Google is enabled
    GitHubClientID     string  // optional, omit to disable GitHub login
    GitHubClientSecret string  // optional, omit to disable GitHub login
}
```

Notes:
- No `DevMode` flag. The service is production-only.
- No per-client secret environment variables. Client secrets are stored in the database.
- Upstream IdP configuration is minimal. The callback URL is always `{Issuer}/login/callback` (derived from the issuer).

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

opts := []op.Option{
    op.WithLogger(logger.WithGroup("oidc")),
}

provider, err := op.NewOpenIDProvider(config.Issuer, opConfig, storage, opts...)
```

No `WithAllowInsecure()`. The issuer must be a valid HTTPS URL.

---

## 15. HTTP Server Wiring

```go
func main() {
    // 1. Load config from env
    // 2. Connect to PostgreSQL
    // 3. Initialize repositories (user, client, authrequest, token)
    // 4. Initialize Storage (with repos + signing key)
    // 5. Create OpenIDProvider

    provider, err := op.NewOpenIDProvider(config.Issuer, opConfig, storage, opts...)

    // 6. Build router
    router := chi.NewRouter()
    router.Use(middleware.Logger)
    router.Use(middleware.Recoverer)

    // 7. Mount login handlers (with IssuerInterceptor)
    interceptor := op.NewIssuerInterceptor(provider.IssuerFromRequest)
    loginHandler := login.NewHandler(authReqRepo, userRepo, upstreamProviders, op.AuthCallbackURL(provider))
    router.Route("/login", func(r chi.Router) {
        r.Use(interceptor.Handler)
        r.Get("/", loginHandler.SelectProvider)           // Provider selection page
        r.Post("/select", loginHandler.FederatedRedirect)  // Redirect to upstream IdP
        r.Get("/callback", loginHandler.FederatedCallback) // Callback from upstream IdP
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

## 16. Database Schema

Migration files in `server/services/accounts/migrations/`.

### `001_initial.sql`

#### `users`

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK, default `gen_random_uuid()` |
| `email` | `TEXT NOT NULL` | NOT unique (multiple accounts can share email) |
| `email_verified` | `BOOLEAN NOT NULL DEFAULT false` | |
| `name` | `TEXT` | Display name |
| `given_name` | `TEXT` | |
| `family_name` | `TEXT` | |
| `picture` | `TEXT` | URL |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |
| `updated_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

#### `federated_identities`

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK, default `gen_random_uuid()` |
| `user_id` | `UUID NOT NULL` | FK -> users(id) ON DELETE CASCADE |
| `provider` | `TEXT NOT NULL` | e.g., "google", "github" |
| `provider_subject` | `TEXT NOT NULL` | Upstream `sub` claim |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |
| | | UNIQUE(`provider`, `provider_subject`) |

#### `clients`

| Column | Type | Notes |
|---|---|---|
| `id` | `TEXT` | PK (e.g., "drive-bff", "chat-service") |
| `secret_hash` | `TEXT` | bcrypt hash; empty string for public clients |
| `redirect_uris` | `TEXT[] NOT NULL` | |
| `post_logout_redirect_uris` | `TEXT[] NOT NULL DEFAULT '{}'` | |
| `application_type` | `TEXT NOT NULL DEFAULT 'web'` | "web", "native", "user_agent" |
| `auth_method` | `TEXT NOT NULL DEFAULT 'client_secret_basic'` | "client_secret_basic", "client_secret_post", "none" |
| `response_types` | `TEXT[] NOT NULL` | e.g., `{"code"}` |
| `grant_types` | `TEXT[] NOT NULL` | e.g., `{"authorization_code","refresh_token"}` |
| `access_token_type` | `TEXT NOT NULL DEFAULT 'jwt'` | "jwt" or "bearer" |
| `id_token_lifetime_seconds` | `INTEGER NOT NULL DEFAULT 3600` | |
| `clock_skew_seconds` | `INTEGER NOT NULL DEFAULT 0` | |
| `id_token_userinfo_assertion` | `BOOLEAN NOT NULL DEFAULT false` | |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |
| `updated_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

#### `auth_requests`

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

Index: `auth_requests_code_idx` on `code` WHERE `code IS NOT NULL` (partial index for `AuthRequestByCode` lookup).

#### `tokens`

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK (this is the `jti` in JWT access tokens) |
| `client_id` | `TEXT NOT NULL` | |
| `subject` | `TEXT NOT NULL` | User UUID for user tokens, client ID for client_credentials tokens |
| `audience` | `TEXT[]` | |
| `scopes` | `TEXT[]` | |
| `expiration` | `TIMESTAMPTZ NOT NULL` | |
| `refresh_token_id` | `UUID` | FK -> refresh_tokens, nullable |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

#### `refresh_tokens`

| Column | Type | Notes |
|---|---|---|
| `id` | `UUID` | PK |
| `token` | `TEXT NOT NULL UNIQUE` | The actual refresh token string |
| `client_id` | `TEXT NOT NULL` | |
| `user_id` | `UUID NOT NULL` | FK -> users(id) |
| `audience` | `TEXT[]` | |
| `scopes` | `TEXT[]` | |
| `auth_time` | `TIMESTAMPTZ NOT NULL` | |
| `amr` | `TEXT[]` | |
| `access_token_id` | `UUID` | FK -> tokens(id) |
| `expiration` | `TIMESTAMPTZ NOT NULL` | |
| `created_at` | `TIMESTAMPTZ NOT NULL DEFAULT now()` | |

### `002_seed_clients.sql`

Contains INSERT statements for the initial client registrations. Secret hashes are bcrypt values that must be generated and replaced before deployment:

```sql
INSERT INTO clients (id, secret_hash, redirect_uris, response_types, grant_types, access_token_type) VALUES
  ('myaccount-bff', '$2a$10$PLACEHOLDER', '{"https://myaccount.hss-science.org/auth/callback"}', '{"code"}', '{"authorization_code","refresh_token"}', 'jwt'),
  ('drive-bff',     '$2a$10$PLACEHOLDER', '{"https://drive.hss-science.org/auth/callback"}',     '{"code"}', '{"authorization_code","refresh_token"}', 'jwt'),
  ('chat-bff',      '$2a$10$PLACEHOLDER', '{"https://chat.hss-science.org/auth/callback"}',      '{"code"}', '{"authorization_code","refresh_token"}', 'jwt'),
  ('chat-service',  '$2a$10$PLACEHOLDER', '{}',                                                   '{"code"}', '{"client_credentials"}',                 'jwt'),
  ('drive-service', '$2a$10$PLACEHOLDER', '{}',                                                   '{"code"}', '{"client_credentials"}',                 'jwt');
```

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
- `ClientCredentials(ctx, clientID, clientSecret)`: Loads client from DB, verifies bcrypt-hashed secret.
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
- **`login/handler_test.go`**: Test federated callback logic with mocked upstream IdP responses (httptest server). Test provider selection page rendering.
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
- **Client secrets**: Stored as bcrypt hashes in the `clients` database table. Compared via `bcrypt.CompareHashAndPassword`.
- **No tokens in browser**: Access/refresh tokens are never exposed to SPAs. They exist only in the BFF layer (per the architecture docs).
- **Short-lived access tokens**: 15 minutes. Limits the damage window if a token is leaked.
- **No account linking**: Each federated identity is an isolated user. No cross-provider merging.
- **HTTPS only**: No dev mode, no insecure overrides. The issuer must be HTTPS.

---

## 23. Resolved Questions

All questions from the previous draft have been resolved per author annotations:

1. **Database migrations** -> `server/services/accounts/migrations/` (bounded context).
2. **Upstream IdP selection UI** -> Always show a provider selection page, even with one provider.
3. **User merging** -> No auto-linking. Separate accounts per `(provider, provider_subject)`.
4. **Logout / SSO sessions** -> Stateless OP. No session cookie. Rely on upstream IdP sessions.
5. **Client registration** -> Database-backed from Phase 1. Manual SQL inserts. No admin UI.
