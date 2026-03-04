# Implementation Plan: Session Tracking (OP) + myaccount-bff (RP)

## Codebase Analysis Summary

### Current Architecture

The `accounts` service is an **OIDC Provider (OP)** built on:

- **OIDC Library**: `zitadel/oidc/v3` — implements `op.Storage` interface via `adapter.StorageAdapter`
- **Router**: `go-chi/chi/v5`
- **Database**: PostgreSQL via `jmoiron/sqlx`
- **Architecture**: Clean architecture — Domain → Ports (interfaces) → Adapters (postgres, HTTP)

**Key Packages:**

| Package | Responsibility |
|---|---|
| `internal/oidc/` | Domain types (`AuthRequest`, `Client`, `Token`, `RefreshToken`) and service interfaces |
| `internal/oidc/adapter/` | Bridges domain to `op.Storage`; translates between zitadel types and domain types |
| `internal/oidc/postgres/` | PostgreSQL repositories for auth requests, clients, tokens |
| `internal/identity/` | User identity domain (`User`, `FederatedIdentity`, `FederatedClaims`) |
| `internal/identity/postgres/` | PostgreSQL user repository |
| `internal/authn/` | Federated login flow — provider selection, OAuth2 redirect, callback, claim extraction |
| `internal/middleware/` | Rate limiting (`IPRateLimiter`), security headers |
| `internal/pkg/crypto/` | AES-256-GCM encryption for state parameters |
| `internal/pkg/domerr/` | Sentinel domain errors (`ErrNotFound`, `ErrUnauthorized`, etc.) |

### Current Login Flow (Step-by-Step)

```
1. Client (RP) → GET /authorize?client_id=myaccount-bff&...
   └─ zitadel/oidc calls StorageAdapter.CreateAuthRequest()
   └─ AuthRequest row inserted into DB
   └─ zitadel/oidc redirects to ClientAdapter.LoginURL() → "/login?authRequestID=<id>"

2. Browser → GET /login?authRequestID=<id>
   └─ Handler.SelectProvider() renders provider selection page

3. Browser → POST /login/select (authRequestID + provider)
   └─ Handler.FederatedRedirect() encrypts state, redirects to upstream IdP

4. Upstream IdP authenticates user, redirects back

5. Browser → GET /login/callback?code=<code>&state=<encrypted_state>
   └─ Handler.FederatedCallback():
       a. Decrypts state → extracts authRequestID + provider
       b. Exchanges code for upstream token
       c. Fetches claims from upstream IdP
       d. CompleteFederatedLogin.Execute():
          - identity.FindOrCreateByFederatedLogin() → upserts User + FederatedIdentity
          - authReqSvc.CompleteLogin() → sets user_id, auth_time, amr, is_done=true
       e. Redirects to OP callback URL → /authorize/callback?id=<authRequestID>

6. zitadel/oidc processes the completed AuthRequest:
   └─ StorageAdapter.SaveAuthCode() → stores authorization code
   └─ Redirects to client redirect_uri with code

7. Client (RP) → POST /oauth/v2/token (code + client_secret)
   └─ zitadel/oidc calls StorageAdapter.CreateAccessAndRefreshTokens()
   └─ Access token + refresh token + ID token returned to client
```

### What's Missing

1. **No session concept**: Tokens exist independently — there is no grouping of tokens into "sessions" with device metadata
2. **No device metadata capture**: User-Agent, IP address, etc. are not persisted during login
3. **No session listing/revocation**: Cannot enumerate or selectively revoke a user's active sessions
4. **No BFF implementation**: `server/services/myaccount-bff/` is empty

### Existing Client Registration (migration 2)

The `myaccount-bff` client is already seeded:
- **client_id**: `myaccount-bff`
- **auth_method**: `client_secret_basic` (default — confidential client)
- **redirect_uri**: `https://myaccount.hss-science.org/api/v1/auth/callback`
- **post_logout_redirect_uri**: `https://myaccount.hss-science.org/`
- **grant_types**: `authorization_code`, `refresh_token`
- **allowed_scopes**: `openid`, `email`, `profile`, `offline_access`

---

## Part 1: OP Session Tracking (accounts service)

### 1.1 Design Rationale

A **session** in this context represents: *one successful authentication event from one device*. Each time a user goes through the federated login flow, a new session is created and linked to the resulting refresh token. The session persists as long as at least one non-expired, non-revoked refresh token references it.

**Integration strategy**: Hook into `CompleteFederatedLogin.Execute()` at step 5d of the login flow — this is the only point where the HTTP request (containing device metadata) and the authenticated user identity converge.

### 1.2 Database Changes

**New migration**: `3_sessions.up.sql`

```sql
CREATE TABLE sessions (
    id            TEXT        PRIMARY KEY,
    user_id       TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_agent    TEXT        NOT NULL DEFAULT '',
    ip_address    TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_active   TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at    TIMESTAMPTZ
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_user_id_active_idx ON sessions (user_id) WHERE revoked_at IS NULL;

ALTER TABLE auth_requests ADD COLUMN session_id TEXT;
ALTER TABLE refresh_tokens ADD COLUMN session_id TEXT REFERENCES sessions(id) ON DELETE SET NULL;
CREATE INDEX refresh_tokens_session_id_idx ON refresh_tokens (session_id) WHERE session_id IS NOT NULL;
```

**Corresponding down migration**: `3_sessions.down.sql`

```sql
DROP INDEX IF EXISTS refresh_tokens_session_id_idx;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS session_id;
ALTER TABLE auth_requests DROP COLUMN IF EXISTS session_id;
DROP INDEX IF EXISTS sessions_user_id_active_idx;
DROP INDEX IF EXISTS sessions_user_id_idx;
DROP TABLE IF EXISTS sessions;
```

### 1.3 Domain Layer Changes

**New file**: `internal/oidc/session_domain.go`

```go
// Session represents a single authenticated login event from a specific device.
type Session struct {
    ID         string
    UserID     string
    UserAgent  string
    IPAddress  string
    CreatedAt  time.Time
    LastActive time.Time
    RevokedAt  *time.Time
}

// DeviceInfo carries HTTP-derived metadata captured during login.
type DeviceInfo struct {
    UserAgent string
    IPAddress string
}
```

**Modifications to existing domain types** (`internal/oidc/domain.go`):

Add `SessionID` field to `AuthRequest`:

```go
type AuthRequest struct {
    // ... existing fields ...
    SessionID string  // NEW — links to sessions table
}
```

Add `SessionID` field to `RefreshToken`:

```go
type RefreshToken struct {
    // ... existing fields ...
    SessionID string  // NEW — links to sessions table
}
```

### 1.4 Ports Layer Changes

**New file**: `internal/oidc/session_ports.go`

```go
type SessionRepository interface {
    Create(ctx context.Context, session *Session) error
    GetByID(ctx context.Context, id string) (*Session, error)
    ListByUser(ctx context.Context, userID string) ([]*Session, error)
    TouchLastActive(ctx context.Context, id string, at time.Time) error
    Revoke(ctx context.Context, id string, at time.Time) error
}

type SessionService interface {
    CreateFromLogin(ctx context.Context, userID string, device DeviceInfo) (*Session, error)
    GetByID(ctx context.Context, id string) (*Session, error)
    ListByUser(ctx context.Context, userID string) ([]*Session, error)
    TouchLastActive(ctx context.Context, id string) error
    Revoke(ctx context.Context, id string) error
}
```

**Modifications to existing ports** (`internal/oidc/ports.go`):

`AuthRequestRepository` — update `CompleteLogin` signature:

```go
CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string, sessionID string) error
```

`AuthRequestService` and `LoginCompleter` — same signature change.

`TokenRepository.CreateAccessAndRefresh` — thread session_id through:

```go
CreateAccessAndRefresh(ctx context.Context, access *Token, refresh *RefreshToken, currentRefreshToken string) error
// No signature change needed — session_id travels on the RefreshToken struct
```

### 1.5 Session Service Implementation

**New file**: `internal/oidc/session_svc.go`

Implements `SessionService`:
- `CreateFromLogin`: generates ULID, inserts session row
- `GetByID`: retrieves session (returns `ErrNotFound` if revoked_at is set)
- `ListByUser`: returns all sessions (both active and revoked, let caller filter)
- `TouchLastActive`: updates `last_active` timestamp
- `Revoke`: sets `revoked_at`

### 1.6 Session Repository Implementation

**New file**: `internal/oidc/postgres/session_repo.go`

Implements `SessionRepository` with straightforward SQL CRUD operations.

### 1.7 Existing Code Modifications

#### 1.7.1 `internal/authn/login_usecase.go` — Thread DeviceInfo

Current `CompleteFederatedLogin.Execute` signature:

```go
func (uc *CompleteFederatedLogin) Execute(
    ctx context.Context, provider string, claims identity.FederatedClaims, authRequestID string,
) (string, error)
```

**New signature** — add `DeviceInfo` and `SessionService`:

```go
type CompleteFederatedLogin struct {
    identity   identity.Service
    loginComp  oidcdom.LoginCompleter
    sessions   oidcdom.SessionService  // NEW
}

func (uc *CompleteFederatedLogin) Execute(
    ctx context.Context,
    provider string,
    claims identity.FederatedClaims,
    authRequestID string,
    device oidcdom.DeviceInfo,        // NEW
) (string, error) {
    user, err := uc.identity.FindOrCreateByFederatedLogin(ctx, provider, claims)
    if err != nil {
        return "", fmt.Errorf("federated login: %w", err)
    }

    // NEW: Create a session record with device metadata
    session, err := uc.sessions.CreateFromLogin(ctx, user.ID, device)
    if err != nil {
        return "", fmt.Errorf("create session: %w", err)
    }

    authTime := time.Now().UTC()
    amr := []string{"fed"}
    // MODIFIED: Pass session.ID to CompleteLogin
    if err := uc.loginComp.CompleteLogin(ctx, authRequestID, user.ID, authTime, amr, session.ID); err != nil {
        return "", fmt.Errorf("complete login: %w", err)
    }

    return user.ID, nil
}
```

#### 1.7.2 `internal/authn/handler.go` — Extract DeviceInfo

In `FederatedCallback`, extract device metadata from the HTTP request and pass it to `Execute`:

```go
func (h *Handler) FederatedCallback(w http.ResponseWriter, r *http.Request) {
    // ... existing code to decrypt state, exchange code, fetch claims ...

    device := oidcdom.DeviceInfo{
        UserAgent: r.UserAgent(),
        IPAddress: clientIP(r), // reuse the existing clientIP helper from middleware
    }

    if _, err := h.loginUC.Execute(r.Context(), state.Provider, *claims, state.AuthRequestID, device); err != nil {
        // ... existing error handling ...
    }

    // ... existing redirect ...
}
```

Note: The `clientIP` function currently lives in `internal/middleware/ratelimit.go`. It needs to be **extracted** into a shared utility (e.g., `internal/pkg/httputil/clientip.go`) so both the middleware and the authn handler can use it without import cycles.

#### 1.7.3 `internal/oidc/postgres/authrequest_repo.go` — Persist SessionID

**`CompleteLogin`** — add `session_id` parameter:

```go
func (r *AuthRequestRepository) CompleteLogin(
    ctx context.Context, id, userID string, authTime time.Time, amr []string, sessionID string,
) error {
    _, err := r.db.ExecContext(ctx,
        `UPDATE auth_requests
         SET user_id = $1, auth_time = $2, amr = $3, is_done = true, session_id = $4
         WHERE id = $5`,
        userID, authTime, pq.Array(amr), nilIfEmpty(sessionID), id)
    return err
}
```

**`scanOne`** — read `session_id`:

Add scanning of the `session_id` column and populate `ar.SessionID`.

#### 1.7.4 `internal/oidc/adapter/authrequest.go` — Expose SessionID

Add getter to the adapter `AuthRequest`:

```go
func (a *AuthRequest) GetSessionID() string { return a.domain.SessionID }
```

#### 1.7.5 `internal/oidc/adapter/storage.go` — Thread SessionID into RefreshToken

In `CreateAccessAndRefreshTokens`, the `request` parameter is an `op.TokenRequest`. When it originates from an auth code exchange, it's our `*AuthRequest` adapter. Extract the `session_id` and set it on the domain `RefreshToken`:

```go
func (s *StorageAdapter) CreateAccessAndRefreshTokens(
    ctx context.Context, request op.TokenRequest, currentRefreshToken string,
) (string, string, time.Time, error) {
    // ... existing setup ...

    // NEW: Extract session_id from the auth request
    sessionID := extractSessionID(request)

    // Pass session_id to token service
    accessID, refreshToken, err := s.tokens.CreateAccessAndRefresh(ctx,
        clientIDFromRequest(request), request.GetSubject(), request.GetAudience(), request.GetScopes(),
        accessExp, refreshExp, authTime, amr, currentRefreshToken, sessionID)
    // ...
}

func extractSessionID(request op.TokenRequest) string {
    if ar, ok := request.(*AuthRequest); ok {
        return ar.GetSessionID()
    }
    // For refresh token rotation, preserve the existing session_id
    if rtr, ok := request.(*RefreshTokenRequest); ok {
        return rtr.GetSessionID()
    }
    return ""
}
```

#### 1.7.6 `internal/oidc/token_svc.go` — Accept SessionID

Add `sessionID string` parameter to `CreateAccessAndRefresh`:

```go
func (s *tokenService) CreateAccessAndRefresh(
    ctx context.Context, clientID, subject string,
    audience, scopes []string,
    accessExpiration, refreshExpiration, authTime time.Time,
    amr []string, currentRefreshToken string,
    sessionID string,  // NEW
) (accessID, rawRefreshToken string, err error) {
    // ... existing code ...
    refresh := &RefreshToken{
        // ... existing fields ...
        SessionID: sessionID,  // NEW
    }
    // ...
}
```

Update the `TokenService` interface accordingly.

#### 1.7.7 `internal/oidc/postgres/token_repo.go` — Persist SessionID

In `CreateAccessAndRefresh`, include `session_id` in the refresh_tokens INSERT:

```sql
INSERT INTO refresh_tokens (id, token_hash, client_id, user_id, audience, scopes, auth_time, amr, access_token_id, expiration, session_id)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
```

When doing refresh token rotation: read the `session_id` from the old refresh token and propagate it to the new one. Also call `SessionService.TouchLastActive` to update the session's `last_active` timestamp.

#### 1.7.8 `internal/oidc/adapter/refreshtoken.go` — Expose SessionID

```go
func (r *RefreshTokenRequest) GetSessionID() string { return r.domain.SessionID }
```

### 1.8 Session Management API (OP)

**New file**: `internal/oidc/adapter/session_handler.go`

Expose REST endpoints for session management, mounted under `/api/v1/sessions` in `main.go`. These endpoints are protected by access token validation (bearer token in Authorization header, validated via introspection or JWT verification).

```
GET  /api/v1/sessions       → List active sessions for the authenticated user
DELETE /api/v1/sessions/:id  → Revoke a specific session (deletes associated refresh tokens)
```

**Handler implementation**:

```go
type SessionHandler struct {
    sessions oidcdom.SessionService
    tokens   oidcdom.TokenService
    verifier AccessTokenVerifier  // validates bearer tokens
    logger   *slog.Logger
}
```

**`GET /api/v1/sessions`**:
1. Extract and validate bearer token → get `sub` claim (user_id)
2. Call `sessions.ListByUser(ctx, userID)` filtering active only
3. For each session, join with refresh_tokens to determine if still active (has non-expired refresh token)
4. Return JSON array of session objects

**Response shape**:
```json
[
  {
    "id": "01JA...",
    "user_agent": "Mozilla/5.0 ...",
    "ip_address": "203.0.113.42",
    "created_at": "2026-03-01T10:00:00Z",
    "last_active": "2026-03-05T14:30:00Z",
    "is_current": true
  }
]
```

**`DELETE /api/v1/sessions/:id`**:
1. Extract and validate bearer token → get `sub` claim
2. Call `sessions.GetByID(ctx, id)` → verify it belongs to the authenticated user
3. Call `tokens.DeleteBySession(ctx, sessionID)` → delete all tokens linked to this session
4. Call `sessions.Revoke(ctx, id)` → set `revoked_at`
5. Return 204 No Content

**Token deletion by session** — new method needed:

Add to `TokenRepository` and `TokenService`:
```go
DeleteBySession(ctx context.Context, sessionID string) error
```

Implementation: `DELETE FROM refresh_tokens WHERE session_id = $1` + `DELETE FROM tokens WHERE refresh_token_id IN (SELECT id FROM refresh_tokens WHERE session_id = $1)` (in a transaction, delete tokens first, then refresh tokens).

### 1.9 Access Token Verification for Session API

The session management endpoints need to validate the caller's access token. Two approaches:

**Option A (Recommended): JWT verification**
Since `access_token_type` is `jwt`, the OP can verify its own JWTs locally using the signing key. Create a lightweight middleware:

```go
type JWTAuthMiddleware struct {
    keySet   *PublicKeySet
    issuer   string
}
```

This parses the `Authorization: Bearer <jwt>` header, verifies the signature with the OP's public keys, checks `exp`, `iss`, and extracts the `sub` claim.

**Option B: Self-introspection**
Call the OP's own introspection endpoint. Adds unnecessary overhead since the OP can verify its own tokens directly.

### 1.10 Wire-up in `main.go`

```go
// Create session service
sessionRepo := oidcpg.NewSessionRepository(db)
sessionSvc := oidcdom.NewSessionService(sessionRepo)

// Pass sessionSvc to authn handler
loginHandler := authn.NewHandler(
    upstreamProviders, identitySvc, authReqSvc,
    crypto.NewAESCipher(cfg.CryptoKey),
    op.AuthCallbackURL(provider),
    sessionSvc,  // NEW
    logger,
)

// Mount session API
sessionHandler := oidcadapter.NewSessionHandler(sessionSvc, tokenSvc, publicKeys, cfg.Issuer, logger)
router.Route("/api/v1/sessions", func(r chi.Router) {
    r.Use(sessionHandler.AuthMiddleware)
    r.Get("/", sessionHandler.List)
    r.Delete("/{id}", sessionHandler.Revoke)
})
```

### 1.11 Test Plan (OP)

| Test Target | Test Type | Description |
|---|---|---|
| `session_svc.go` | Unit | `CreateFromLogin` generates ULID, returns correct struct; `Revoke` sets timestamp |
| `session_repo.go` | Integration (testcontainers) | CRUD operations against real Postgres |
| `CompleteFederatedLogin` | Unit (mocked deps) | Verify session is created and session_id is passed to `CompleteLogin` |
| `StorageAdapter.CreateAccessAndRefreshTokens` | Integration | Verify session_id propagates to refresh_tokens table |
| `SessionHandler.List` | Unit (mocked deps) | Verify correct JSON response shape, auth enforcement |
| `SessionHandler.Revoke` | Unit (mocked deps) | Verify ownership check, token deletion, session revocation |

### 1.12 Files Changed / Created Summary (OP)

| Action | File |
|---|---|
| **Create** | `migrations/3_sessions.up.sql` |
| **Create** | `migrations/3_sessions.down.sql` |
| **Create** | `internal/oidc/session_domain.go` |
| **Create** | `internal/oidc/session_ports.go` |
| **Create** | `internal/oidc/session_svc.go` |
| **Create** | `internal/oidc/session_svc_test.go` |
| **Create** | `internal/oidc/postgres/session_repo.go` |
| **Create** | `internal/oidc/adapter/session_handler.go` |
| **Create** | `internal/pkg/httputil/clientip.go` |
| **Modify** | `internal/oidc/domain.go` — add `SessionID` to `AuthRequest` and `RefreshToken` |
| **Modify** | `internal/oidc/ports.go` — update `CompleteLogin` signatures, add `DeleteBySession` |
| **Modify** | `internal/oidc/authrequest_svc.go` — update `CompleteLogin` to pass `sessionID` |
| **Modify** | `internal/oidc/token_svc.go` — accept `sessionID` param, add `DeleteBySession` |
| **Modify** | `internal/oidc/postgres/authrequest_repo.go` — persist + scan `session_id` |
| **Modify** | `internal/oidc/postgres/token_repo.go` — persist + scan `session_id`, add `DeleteBySession` |
| **Modify** | `internal/oidc/adapter/authrequest.go` — add `GetSessionID()` |
| **Modify** | `internal/oidc/adapter/refreshtoken.go` — add `GetSessionID()` |
| **Modify** | `internal/oidc/adapter/storage.go` — thread `sessionID` through token creation |
| **Modify** | `internal/authn/login_usecase.go` — accept `SessionService` + `DeviceInfo`, create session |
| **Modify** | `internal/authn/handler.go` — extract `DeviceInfo` from HTTP request, pass to `Execute` |
| **Modify** | `internal/middleware/ratelimit.go` — extract `clientIP` to shared package |
| **Modify** | `main.go` — wire session service, mount session API |
| **Modify** | `testhelper/testdb.go` — add `sessions` to `CleanTables` |

---

## Part 2: myaccount-bff (Relying Party)

### 2.1 Architecture Overview

```
Browser ←→ myaccount-bff (BFF) ←→ accounts (OP)
  │              │                      │
  │  opaque      │  access_token        │
  │  cookie      │  refresh_token       │
  │              │  id_token             │
  │              ▼                      │
  │           Redis                     │
  │         (sessions)                  │
```

The BFF is a **confidential OIDC Relying Party** that:
- Receives an opaque session cookie from the browser
- Looks up the real tokens in Redis
- Proxies authenticated requests to backend APIs (or the OP's session management API)
- Handles token refresh transparently

### 2.2 Service Directory Structure

```
server/services/myaccount-bff/
├── main.go
├── Dockerfile
├── .env.example
├── config/
│   └── config.go
└── internal/
    ├── auth/
    │   ├── handler.go          # Login, Callback, Logout, SessionInfo endpoints
    │   ├── oidc_client.go      # OIDC/OAuth2 client configuration + helpers
    │   └── middleware.go       # Session-required middleware
    ├── session/
    │   ├── domain.go           # SessionData struct
    │   └── store.go            # Redis session store
    └── middleware/
        └── securityheaders.go  # Reuse pattern from accounts
```

### 2.3 Dependencies (additions to `go.mod`)

```
github.com/redis/go-redis/v9    # Redis client
```

All other dependencies (`go-chi`, `coreos/go-oidc`, `golang.org/x/oauth2`, `google/uuid`, `golang.org/x/time`) are already in `go.mod`.

### 2.4 Configuration

**`config/config.go`**:

```go
type Config struct {
    Port      string   // default: 8081

    // OIDC Provider
    OIDCIssuer       string   // e.g., "https://accounts.hss-science.org"
    OIDCClientID     string   // "myaccount-bff"
    OIDCClientSecret string
    OIDCRedirectURL  string   // "https://myaccount.hss-science.org/api/v1/auth/callback"
    OIDCScopes       []string // default: ["openid","email","profile","offline_access"]

    // Redis
    RedisAddr     string   // e.g., "redis:6379"
    RedisPassword string
    RedisDB       int

    // Session
    SessionTTLHours        int  // default: 168 (7 days, matching refresh token TTL)
    SessionCookieName      string // default: "__Host-session"
    SessionCookieDomain    string // empty for __Host- prefix cookies

    // Security
    CryptoKey              [32]byte // for CSRF token encryption
    PostLoginRedirectURL   string   // "https://myaccount.hss-science.org/"
    PostLogoutRedirectURL  string   // "https://myaccount.hss-science.org/"
    AllowedOrigins         []string // CORS origins for the SPA

    // Rate Limiting
    RateLimitEnabled   bool
    RateLimitLoginRPM  int
    RateLimitGlobalRPM int
}
```

**Environment variables** (`.env.example`):

```env
PORT=8081
OIDC_ISSUER=https://accounts.hss-science.org
OIDC_CLIENT_ID=myaccount-bff
OIDC_CLIENT_SECRET=
OIDC_REDIRECT_URL=https://myaccount.hss-science.org/api/v1/auth/callback
REDIS_ADDR=redis:6379
REDIS_PASSWORD=
REDIS_DB=0
SESSION_TTL_HOURS=168
CRYPTO_KEY=
POST_LOGIN_REDIRECT_URL=https://myaccount.hss-science.org/
POST_LOGOUT_REDIRECT_URL=https://myaccount.hss-science.org/
```

### 2.5 Redis Session Store

**`internal/session/domain.go`**:

```go
type SessionData struct {
    UserID       string    `json:"user_id"`
    IDToken      string    `json:"id_token"`
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    TokenExpiry  time.Time `json:"token_expiry"`
    CreatedAt    time.Time `json:"created_at"`
}
```

**`internal/session/store.go`**:

```go
type Store interface {
    Create(ctx context.Context, data *SessionData) (sessionID string, err error)
    Get(ctx context.Context, sessionID string) (*SessionData, error)
    Update(ctx context.Context, sessionID string, data *SessionData) error
    Delete(ctx context.Context, sessionID string) error
}

type RedisStore struct {
    client *redis.Client
    ttl    time.Duration
    prefix string  // "bff:session:"
}
```

**Key format**: `bff:session:<opaque-uuid>`

**Implementation details**:
- `Create`: Generate UUID v4, JSON-marshal `SessionData`, `SET bff:session:<uuid> <json> EX <ttl>`
- `Get`: `GET bff:session:<uuid>`, unmarshal, return `ErrNotFound` if key doesn't exist
- `Update`: `SET bff:session:<uuid> <json> KEEPTTL` (preserve original TTL)
- `Delete`: `DEL bff:session:<uuid>`

Tokens are stored encrypted at rest in Redis using AES-256-GCM (same pattern as `internal/pkg/crypto/`). The `Store` implementation encrypts before writing and decrypts after reading.

### 2.6 OIDC Client Setup

**`internal/auth/oidc_client.go`**:

```go
type OIDCClient struct {
    provider    *gooidc.Provider     // coreos/go-oidc — for discovery + ID token verification
    oauth2Cfg   *oauth2.Config       // golang.org/x/oauth2 — for auth code flow
    verifier    *gooidc.IDTokenVerifier
}

func NewOIDCClient(ctx context.Context, issuer, clientID, clientSecret, redirectURL string, scopes []string) (*OIDCClient, error) {
    provider, err := gooidc.NewProvider(ctx, issuer)
    // ...
    oauth2Cfg := &oauth2.Config{
        ClientID:     clientID,
        ClientSecret: clientSecret,
        RedirectURL:  redirectURL,
        Endpoint:     provider.Endpoint(),
        Scopes:       scopes,
    }
    verifier := provider.Verifier(&gooidc.Config{ClientID: clientID})
    // ...
}
```

### 2.7 Auth Handler Endpoints

**`internal/auth/handler.go`**:

```go
type Handler struct {
    oidcClient *OIDCClient
    sessions   session.Store
    cipher     crypto.Cipher   // for CSRF state encryption
    config     *HandlerConfig
    logger     *slog.Logger
}

type HandlerConfig struct {
    CookieName           string
    PostLoginRedirectURL string
    PostLogoutRedirectURL string
    SessionTTL           time.Duration
}
```

#### `GET /api/v1/auth/login`

1. Generate PKCE code verifier + code challenge (S256)
2. Generate random `state` parameter, encrypt with AES (includes PKCE verifier)
3. Set encrypted state in a short-lived (`__Host-oidc-state`) cookie (5 min TTL, HttpOnly, Secure, SameSite=Lax)
4. Redirect to OP's authorize endpoint with:
   - `response_type=code`
   - `client_id=myaccount-bff`
   - `redirect_uri=<callback_url>`
   - `scope=openid email profile offline_access`
   - `state=<random>`
   - `code_challenge=<challenge>`
   - `code_challenge_method=S256`

#### `GET /api/v1/auth/callback`

1. Read the `state` query param + `__Host-oidc-state` cookie
2. Decrypt cookie value, verify `state` matches
3. Extract PKCE code verifier from decrypted state
4. Exchange authorization code for tokens at OP's token endpoint (with PKCE verifier + client_secret)
5. Verify ID token signature and claims using `gooidc.IDTokenVerifier`
6. Extract `sub` claim from ID token
7. Create session in Redis:
   ```go
   sessionData := &session.SessionData{
       UserID:       idToken.Subject,
       IDToken:      rawIDToken,
       AccessToken:  oauth2Token.AccessToken,
       RefreshToken: oauth2Token.RefreshToken,
       TokenExpiry:  oauth2Token.Expiry,
       CreatedAt:    time.Now().UTC(),
   }
   sessionID, err := h.sessions.Create(ctx, sessionData)
   ```
8. Set session cookie:
   ```go
   http.SetCookie(w, &http.Cookie{
       Name:     "__Host-session",
       Value:    sessionID,
       Path:     "/",
       MaxAge:   int(h.config.SessionTTL.Seconds()),
       HttpOnly: true,
       Secure:   true,
       SameSite: http.SameSiteLaxMode,
   })
   ```
9. Clear the `__Host-oidc-state` cookie
10. Redirect to `PostLoginRedirectURL`

#### `POST /api/v1/auth/logout`

1. Read session cookie → get session from Redis
2. Build OP's end_session_endpoint URL with:
   - `id_token_hint=<stored_id_token>`
   - `post_logout_redirect_uri=<post_logout_redirect_url>`
3. Delete Redis session
4. Clear session cookie (MaxAge=-1)
5. Redirect to OP's end_session_endpoint

The OP's `TerminateSession` (called by zitadel/oidc during end_session) will delete the access/refresh tokens for this user+client.

#### `GET /api/v1/auth/session`

1. Session middleware has already validated the session
2. Return session info (user_id, created_at) — **never expose tokens**

**Response**:
```json
{
  "user_id": "01JA...",
  "email": "user@example.com",
  "name": "John Doe",
  "picture": "https://...",
  "created_at": "2026-03-01T10:00:00Z"
}
```

This endpoint fetches user info from the OP's userinfo endpoint using the stored access token (refreshing first if expired).

### 2.8 Session Middleware

**`internal/auth/middleware.go`**:

```go
func (h *Handler) RequireSession(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie(h.config.CookieName)
        if err != nil {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        sess, err := h.sessions.Get(r.Context(), cookie.Value)
        if err != nil {
            // Session expired or not found
            clearSessionCookie(w, h.config.CookieName)
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        // Check if access token needs refresh
        if time.Now().After(sess.TokenExpiry.Add(-30 * time.Second)) {
            refreshed, err := h.refreshTokens(r.Context(), cookie.Value, sess)
            if err != nil {
                // Refresh failed — session is dead
                h.sessions.Delete(r.Context(), cookie.Value)
                clearSessionCookie(w, h.config.CookieName)
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }
            sess = refreshed
        }

        // Store session in request context
        ctx := context.WithValue(r.Context(), sessionContextKey, sess)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 2.9 Silent Token Refresh

**`internal/auth/handler.go`**:

```go
func (h *Handler) refreshTokens(ctx context.Context, sessionID string, sess *session.SessionData) (*session.SessionData, error) {
    // Use oauth2 token source for refresh
    oldToken := &oauth2.Token{
        AccessToken:  sess.AccessToken,
        RefreshToken: sess.RefreshToken,
        Expiry:       sess.TokenExpiry,
        TokenType:    "Bearer",
    }

    // oauth2.Config.TokenSource handles the refresh_token grant automatically
    src := h.oidcClient.oauth2Cfg.TokenSource(ctx, oldToken)
    newToken, err := src.Token()
    if err != nil {
        return nil, fmt.Errorf("refresh token: %w", err)
    }

    // Update session in Redis
    updated := &session.SessionData{
        UserID:       sess.UserID,
        IDToken:      sess.IDToken, // ID token doesn't change on refresh
        AccessToken:  newToken.AccessToken,
        RefreshToken: newToken.RefreshToken, // May be rotated
        TokenExpiry:  newToken.Expiry,
        CreatedAt:    sess.CreatedAt,
    }

    if err := h.sessions.Update(ctx, sessionID, updated); err != nil {
        return nil, fmt.Errorf("update session: %w", err)
    }

    return updated, nil
}
```

### 2.10 Routing Setup (`main.go`)

```go
router := chi.NewRouter()
router.Use(chimiddleware.Recoverer)
router.Use(middleware.SecurityHeaders())
// Rate limiting (same pattern as accounts)

// Public endpoints (no session required)
router.Get("/healthz", healthHandler)
router.Get("/readyz", readyHandler)

// Auth endpoints
router.Route("/api/v1/auth", func(r chi.Router) {
    r.Get("/login", authHandler.Login)
    r.Get("/callback", authHandler.Callback)
    r.Post("/logout", authHandler.Logout)  // POST for logout (CSRF safe)
})

// Protected endpoints (session required)
router.Route("/api/v1", func(r chi.Router) {
    r.Use(authHandler.RequireSession)
    r.Get("/auth/session", authHandler.SessionInfo)
    // Future: proxy to OP's session management API
    // r.Get("/sessions", authHandler.ListSessions)
    // r.Delete("/sessions/{id}", authHandler.RevokeSession)
})
```

### 2.11 Dockerfile

```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY server/go.mod server/go.sum ./
RUN go mod download
COPY server/ ./
RUN CGO_ENABLED=0 go build -o /myaccount-bff ./services/myaccount-bff/

FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=builder /myaccount-bff /myaccount-bff
ENTRYPOINT ["/myaccount-bff"]
```

### 2.12 Security Considerations

| Concern | Mitigation |
|---|---|
| Cookie theft (XSS) | `HttpOnly`, `Secure`, `__Host-` prefix, CSP headers |
| CSRF | `SameSite=Lax` on session cookie; `POST` for state-changing ops; encrypted OIDC state parameter |
| Token leakage to browser | Tokens live only in Redis; browser only sees opaque session ID |
| Session fixation | New session ID generated on every login |
| Open redirect | `post_logout_redirect_uri` is registered at the OP; `PostLoginRedirectURL` is configured server-side |
| PKCE | Code verifier generated per-login, bound to state cookie |
| Token at rest | Encrypted in Redis using AES-256-GCM |
| Replay attacks on state | State cookie is single-use (cleared after callback) |

### 2.13 Test Plan (BFF)

| Test Target | Test Type | Description |
|---|---|---|
| `session/store.go` | Unit (miniredis) | Create/Get/Update/Delete with in-memory Redis |
| `auth/handler.go` Login | Unit | Verify PKCE generation, state cookie, redirect URL construction |
| `auth/handler.go` Callback | Unit (mocked OIDC) | Verify code exchange, session creation, cookie setting |
| `auth/handler.go` Logout | Unit | Verify Redis cleanup, cookie clearing, OP redirect |
| `auth/middleware.go` | Unit | Verify session validation, token refresh trigger, 401 on missing/expired |
| `config/config.go` | Unit | Validate required fields, defaults, bounds |

### 2.14 Files Created Summary (BFF)

| File | Purpose |
|---|---|
| `main.go` | Entry point, wire-up |
| `Dockerfile` | Container build |
| `.env.example` | Environment variable documentation |
| `config/config.go` | Configuration loading + validation |
| `config/config_test.go` | Config tests |
| `internal/session/domain.go` | `SessionData` struct |
| `internal/session/store.go` | Redis session store interface + implementation |
| `internal/session/store_test.go` | Redis store tests |
| `internal/auth/oidc_client.go` | OIDC client setup |
| `internal/auth/handler.go` | Login, Callback, Logout, SessionInfo |
| `internal/auth/handler_test.go` | Handler unit tests |
| `internal/auth/middleware.go` | Session-required middleware |
| `internal/auth/middleware_test.go` | Middleware tests |
| `internal/middleware/securityheaders.go` | Security headers middleware |

---

## Implementation Order

### Phase 1: OP Session Infrastructure
1. Create migration `3_sessions.up.sql` / `3_sessions.down.sql`
2. Create `session_domain.go`, `session_ports.go`, `session_svc.go`
3. Create `postgres/session_repo.go`
4. Extract `clientIP` to shared package `internal/pkg/httputil/`
5. Write unit + integration tests for session service/repo

### Phase 2: OP Flow Integration
6. Modify `AuthRequest` and `RefreshToken` domain types (add `SessionID`)
7. Update `AuthRequestRepository` — persist/scan `session_id`
8. Update `authn/login_usecase.go` — accept `SessionService` + `DeviceInfo`
9. Update `authn/handler.go` — extract `DeviceInfo`, pass to `Execute`
10. Update adapter layer — thread `session_id` through token creation
11. Update `token_svc.go` and `token_repo.go` — persist `session_id`
12. Update `main.go` wire-up
13. Update all affected tests

### Phase 3: OP Session Management API
14. Implement JWT auth middleware for OP's API endpoints
15. Create `SessionHandler` with `List` and `Revoke`
16. Add `DeleteBySession` to token repo/service
17. Mount in `main.go`
18. Write tests

### Phase 4: BFF Foundation
19. Create BFF config, main.go, Dockerfile
20. Implement Redis session store
21. Implement OIDC client setup
22. Write tests for session store

### Phase 5: BFF Auth Lifecycle
23. Implement Login endpoint (PKCE + state)
24. Implement Callback endpoint (code exchange + session creation)
25. Implement session middleware (validation + silent refresh)
26. Implement Logout endpoint (OP + Redis cleanup)
27. Implement SessionInfo endpoint
28. Write handler + middleware tests

### Phase 6: Integration Validation
29. Verify end-to-end flow with both services running
30. Verify session appears in OP's session list after BFF login
31. Verify session revocation from OP propagates correctly
