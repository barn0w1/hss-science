# myaccount-bff: Research & Design Reference

**Purpose**: Everything needed to design `myaccount-bff`, a BFF (Backend-For-Frontend) service for
`myaccount.hss-science.org`. Acts as an OIDC Relying Party against the `accounts` service, manages
user sessions in Redis, exposes a REST API (OpenAPI) to the SPA, and calls the accounts service
gRPC API using the user's JWT.

---

## 1. Accounts Service Overview

### Runtime topology

```
Browser
  │  HTTPS
  ▼
accounts service  (HTTP :8080)
  ├── OIDC Provider  (zitadel/oidc v3.45.5 — Authorization Code + PKCE + Refresh Token)
  │     routes: /.well-known/openid-configuration, /oauth/v2/authorize, /oauth/v2/token,
  │             /oauth/v2/introspect, /oauth/v2/revoke, /keys, /userinfo
  │
  ├── Login UI  (/login, /login/select, /login/callback)
  │     HTML pages — provider selection + federated OAuth2 callback
  │
  ├── Health endpoints  (/healthz, /readyz)
  │
  └── gRPC Server  (:50051)
        AccountManagementService (all RPCs require Bearer JWT in `authorization` metadata)
```

### Module path

```
github.com/barn0w1/hss-science/server
```

---

## 2. OIDC Provider Configuration

### Provider config (`internal/oidc/adapter/provider.go`)

```go
op.Config{
    CryptoKey:                cryptoKey,      // [32]byte AES key (from CRYPTO_KEY env)
    DefaultLogoutRedirectURI: "/logged-out",
    CodeMethodS256:           true,           // PKCE S256 enforced
    AuthMethodPost:           true,
    AuthMethodPrivateKeyJWT:  false,
    GrantTypeRefreshToken:    true,
    RequestObjectSupported:   false,
    SupportedUILocales:       [en, ja],
}
```

### Key OIDC endpoints (zitadel/oidc v3 defaults)

| Endpoint | Path |
|----------|------|
| Discovery | `/.well-known/openid-configuration` |
| JWKS | `/keys` |
| Authorization | `/oauth/v2/authorize` |
| Token | `/oauth/v2/token` |
| Introspect | `/oauth/v2/introspect` |
| Revocation | `/oauth/v2/revoke` |
| UserInfo | `/userinfo` |
| End Session | `/oauth/v2/end_session` |

### Signing

- Algorithm: **RS256**
- Key: RSA ≥2048-bit private key (from `SIGNING_KEY_PEM` env)
- Key rotation: previous keys in `SIGNING_KEY_PREVIOUS_PEM` (separated by `---NEXT---`)
- Public keys served at `/keys` (JWKS)

### Token lifetimes (defaults, all configurable)

| Token | Default | Env var |
|-------|---------|---------|
| Access token | 15 min | `ACCESS_TOKEN_LIFETIME_MINUTES` (1–60) |
| Refresh token | 7 days | `REFRESH_TOKEN_LIFETIME_DAYS` (1–90) |
| Auth request | 30 min | `AUTH_REQUEST_TTL_MINUTES` (1–60) |
| ID token | 3600 s | per-client `id_token_lifetime_seconds` in DB |

---

## 3. Pre-registered Client for myaccount-bff

In `migrations/2_seed_clients.up.sql`:

```sql
INSERT INTO clients (
    id, secret_hash,
    redirect_uris, post_logout_redirect_uris,
    response_types, grant_types,
    access_token_type, allowed_scopes
) VALUES (
    'myaccount-bff',
    '$2y$10$...bcrypt...',
    '{"https://myaccount.hss-science.org/api/v1/auth/callback"}',
    '{"https://myaccount.hss-science.org/"}',
    '{"code"}',
    '{"authorization_code","refresh_token"}',
    'jwt',
    '{"openid","email","profile","offline_access"}'
);
```

Key facts:
- **Client ID**: `myaccount-bff`
- **Auth method**: `client_secret_basic` (default) — Basic auth on token endpoint
- **Application type**: `web` (default)
- **PKCE**: required (S256) because it is a web client — but secret-based auth is primary
- **Access token type**: `jwt` — Access tokens ARE JWTs (RS256-signed by accounts)
- **Scopes**: `openid email profile offline_access`
- **Callback**: `https://myaccount.hss-science.org/api/v1/auth/callback`
- **Post-logout redirect**: `https://myaccount.hss-science.org/`

### Client auth method detail

The `client_secret_basic` auth method means the BFF must send:
```
Authorization: Basic base64(client_id:client_secret)
```
on all token endpoint calls. This is the **server-side** secret — never exposed to the browser.

---

## 4. Authorization Code Flow (BFF as RP)

```
SPA                  myaccount-bff               accounts service
 │                        │                            │
 │── GET /api/v1/auth/login ─▶│                        │
 │                        │── build authorize URL ──▶  │
 │◀── 302 to accounts /oauth/v2/authorize ─────────────│
 │                        │                            │
 │─────── browser follows redirect ───────────────────▶│ /oauth/v2/authorize
 │                        │                            │  (shows /login page)
 │◀──────── 302 to accounts /login ────────────────────│
 │── user picks Google/GitHub ──────────────────────── │ federated OAuth flow
 │── accounts /login/callback ──────────────────────── │ sets auth request done
 │                        │                            │
 │◀─────────────────── 302 to /api/v1/auth/callback?code=... ─────────────────
 │── GET /api/v1/auth/callback?code=&state= ──▶│         │
 │                        │── POST /oauth/v2/token ──▶  │
 │                        │   (code + code_verifier      │
 │                        │    + client_secret_basic)    │
 │                        │◀── {access_token, id_token,  │
 │                        │     refresh_token, ...}      │
 │                        │                             │
 │                        │ store session in Redis       │
 │◀─── Set-Cookie: session=<sid> ──────────────────────│
 │── API calls with cookie ──▶│                         │
 │                        │── gRPC call with Bearer JWT ▶│
```

### PKCE handling

The BFF generates `code_verifier` (random, 43–128 chars, base64url) and `code_challenge = S256(verifier)` at login initiation, stores verifier in session/state, sends challenge to accounts, then sends verifier on token exchange.

---

## 5. gRPC API — AccountManagementService

**Location**: `api/proto/accounts/v1/account_management.proto`
**Go package**: `github.com/barn0w1/hss-science/server/gen/accounts/v1`
**Address**: accounts service `:50051`

### Authentication

Every gRPC call requires:
```
metadata: authorization = "Bearer <access_token>"
```

The gRPC server verifies the JWT locally using the in-memory RSA public key set (same key as OIDC).
The BFF forwards the user's access token as-is.

JWT claims validated by interceptor (`internal/grpc/interceptor.go`):
- Valid RS256 signature against known JWKS keys (matched by `kid`)
- `iss` == configured issuer
- `exp` > now
- `sub` != ""

The `sub` claim becomes `userID` in gRPC handlers.

### RPC methods

#### `GetMyProfile` → `Profile`
- No request params
- Returns full profile derived from `sub` in JWT
- Profile fields: `user_id`, `email`, `email_verified`, `name`, `given_name`, `family_name`, `picture`, `name_is_local`, `picture_is_local`, `created_at`, `updated_at`

#### `UpdateMyProfile` → `Profile`
- Request: `optional string name`, `optional string picture`
- Both fields use proto3 `optional` — omit to leave unchanged
- Updates `local_name` / `local_picture` columns (BFF-managed overrides over federated data)
- `name_is_local` / `picture_is_local` flags indicate which values are locally set

#### `ListLinkedProviders` → `ListLinkedProvidersResponse`
- Returns list of `FederatedProviderInfo`:
  - `identity_id` (ULID)
  - `provider` (e.g. `"google"`, `"github"`)
  - `provider_email`
  - `last_login_at`

#### `UnlinkProvider(identity_id)` → `Empty`
- Deletes the specified federated identity
- Error if `identity_id` is empty → `codes.InvalidArgument`

#### `ListActiveSessions` → `ListActiveSessionsResponse`
- Returns list of `Session`:
  - `session_id` (ULID)
  - `device_name` (parsed from User-Agent)
  - `ip_address`
  - `created_at`, `last_used_at`

#### `RevokeSession(session_id)` → `Empty`
- Revokes a specific device session by ID
- Error if `session_id` is empty → `codes.InvalidArgument`

#### `RevokeAllOtherSessions(current_session_id)` → `Empty`
- Revokes all active device sessions except the current one
- Best-effort: individual revoke errors are silently dropped

### gRPC error codes

| Domain error | gRPC code |
|-------------|-----------|
| `domerr.ErrNotFound` | `codes.NotFound` |
| `domerr.ErrUnauthorized` | `codes.PermissionDenied` |
| `domerr.ErrFailedPrecondition` | `codes.FailedPrecondition` |
| other | `codes.Internal` |

---

## 6. Domain Models

### User (`internal/identity/domain.go`)

```go
type User struct {
    ID            string       // ULID
    Email         string
    EmailVerified bool
    Name          string       // effective name (local override applied if set)
    GivenName     string
    FamilyName    string
    Picture       string       // effective picture URL
    CreatedAt     time.Time
    UpdatedAt     time.Time
    LocalName     *string      // nil = not overridden
    LocalPicture  *string      // nil = not overridden
}
```

### FederatedIdentity

```go
type FederatedIdentity struct {
    ID              string   // ULID
    UserID          string
    Provider        string   // "google" | "github"
    ProviderSubject string
    ProviderEmail         string
    ProviderEmailVerified bool
    ProviderDisplayName   string
    ProviderGivenName     string
    ProviderFamilyName    string
    ProviderPictureURL    string
    LastLoginAt   time.Time
    CreatedAt     time.Time
    UpdatedAt     time.Time
}
```

### DeviceSession (`internal/oidc/domain.go`)

```go
type DeviceSession struct {
    ID         string     // ULID (also stored in `dsid` HttpOnly cookie on the accounts service after login)
    UserID     string
    UserAgent  string
    IPAddress  string
    DeviceName string     // parsed from UA
    CreatedAt  time.Time
    LastUsedAt time.Time
    RevokedAt  *time.Time // nil = active
}
```

Note: `DeviceSession` is the accounts-side concept. Each refresh token is tied to a `device_session_id`. The BFF will have its own **Redis-based session** concept (separate from device sessions).

### Token (`internal/oidc/domain.go`)

```go
type Token struct {
    ID             string    // ULID = the JWT `jti` / subject for gRPC
    ClientID       string
    Subject        string    // user ULID
    Audience       []string
    Scopes         []string
    Expiration     time.Time
    RefreshTokenID string
    CreatedAt      time.Time
}
```

---

## 7. Database Schema (accounts service PostgreSQL)

### `users`
```sql
id TEXT PK | email | email_verified | name | given_name | family_name | picture
local_name TEXT NULL | local_picture TEXT NULL | created_at | updated_at
```

### `federated_identities`
```sql
id TEXT PK | user_id FK(users) | provider | provider_subject UNIQUE(provider,subject)
provider_email | provider_email_verified | provider_display_name | ...
last_login_at | created_at | updated_at
```

### `clients`
```sql
id TEXT PK | secret_hash (bcrypt) | redirect_uris TEXT[] | post_logout_redirect_uris TEXT[]
application_type | auth_method | response_types TEXT[] | grant_types TEXT[]
access_token_type | allowed_scopes TEXT[] | id_token_lifetime_seconds | clock_skew_seconds
id_token_userinfo_assertion | created_at | updated_at
```

### `auth_requests`
```sql
id TEXT PK | client_id | redirect_uri | state | nonce | scopes TEXT[]
response_type | response_mode | code_challenge | code_challenge_method
prompt TEXT[] | max_age | login_hint | user_id | auth_time | amr TEXT[]
is_done | code | device_session_id FK(device_sessions) | created_at
```

### `tokens`
```sql
id TEXT PK | client_id | subject | audience TEXT[] | scopes TEXT[]
expiration | refresh_token_id | created_at
```

### `refresh_tokens`
```sql
id TEXT PK | token_hash TEXT UNIQUE | client_id | user_id FK(users)
audience TEXT[] | scopes TEXT[] | auth_time | amr TEXT[] | access_token_id
device_session_id FK(device_sessions) NULL | expiration | created_at
```

### `device_sessions`
```sql
id TEXT PK | user_id FK(users) | user_agent | ip_address | device_name
created_at | last_used_at | revoked_at TIMESTAMPTZ NULL
```

---

## 8. JWT Access Token Structure

Access tokens are **JWTs** (`access_token_type = jwt`). The accounts OIDC provider signs them
RS256. The gRPC interceptor verifies them locally.

Standard JWT claims present:
- `sub` — user ULID
- `iss` — issuer URL (e.g. `https://accounts.hss-science.org`)
- `exp` — expiration Unix timestamp
- `aud` — audience (client ID)
- `jti` — token ID (ULID, stored in `tokens.id`)
- `scope` — space-separated scopes
- Profile claims (name, email, etc.) if `IDTokenUserinfoAssertion = true` for the client (currently false for myaccount-bff)

The BFF **must not re-validate the JWT itself** using a separate JWKS fetch on every request — it
should cache the public key set (or verify it offline). The gRPC interceptor in accounts already
validates the JWT on every gRPC call server-side, so the BFF only needs to:
1. Verify the JWT is not expired at the HTTP layer (for fast-fail before gRPC round-trip)
2. Extract `sub` for session binding (optional, can rely on gRPC)

---

## 9. Token Refresh Strategy for the BFF

The BFF holds the refresh token in the server-side Redis session. To maintain a fresh access token:

1. On every API call, check if the access token is within 60 s of expiry.
2. If so, call `POST /oauth/v2/token` with `grant_type=refresh_token`.
3. Replace the access token (and possibly refresh token — rotation is enabled) in Redis.
4. Forward the new access token to the gRPC call.

**Refresh token rotation**: `CreateAccessAndRefreshTokens` uses `currentRefreshToken` — the old
refresh token is invalidated and a new one is issued. The BFF must store the new refresh token
atomically.

---

## 10. Session Revocation / Logout

### BFF-triggered logout flow

1. BFF deletes its own Redis session.
2. BFF calls accounts OIDC end_session endpoint:
   ```
   GET /oauth/v2/end_session?id_token_hint=<id_token>&post_logout_redirect_uri=https://myaccount.hss-science.org/
   ```
   This triggers `TerminateSession` → `DeleteByUserAndClient` (revokes all tokens for this user+client).

### Session revocation via gRPC

The `RevokeSession(session_id)` and `RevokeAllOtherSessions(current_session_id)` RPCs operate on
**device sessions** in the accounts DB. Revoking a device session causes the corresponding refresh
tokens to have their `device_session_id` set to NULL (FK ON DELETE SET NULL), but the tokens
themselves are not immediately invalidated — they remain valid until expiry or explicit revocation.

For the BFF's own Redis session, the `session_id` concept is separate. If the BFF wants to expose
"revoke another session" from the UI, it calls `RevokeSession(device_session_id)` via gRPC. The
`current_session_id` for `RevokeAllOtherSessions` should be the device session ID tracked in the
BFF session (which the BFF learns from the device session stored after login — but the BFF does not
currently have direct access to the device session ID post-OIDC-token-exchange unless it is embedded
in the ID token or tracked by other means).

---

## 11. ID Token Claims

The accounts service sets userinfo from scopes (`internal/oidc/adapter/storage.go:setUserinfo`):

| Scope | Claims set |
|-------|-----------|
| `openid` | `sub` |
| `profile` | `name`, `given_name`, `family_name`, `picture`, `updated_at` |
| `email` | `email`, `email_verified` |

For `myaccount-bff` with `offline_access` scope, the token endpoint returns:
- `id_token` (JWT with profile claims since `profile email` are in allowed_scopes)
- `access_token` (JWT, type `jwt`)
- `refresh_token` (opaque)
- `token_type: Bearer`
- `expires_in`
- `scope`

---

## 12. Libraries Already in go.mod (relevant to BFF)

The following are already in the shared `go.mod` and can be used by the BFF without adding new deps:

| Library | Use |
|---------|-----|
| `github.com/coreos/go-oidc/v3 v3.17.0` | OIDC RP client (verify ID tokens, OIDC discovery) |
| `github.com/go-chi/chi/v5 v5.2.5` | HTTP router (same as accounts) |
| `github.com/go-jose/go-jose/v4 v4.1.3` | JWT parsing / verification |
| `github.com/google/uuid v1.6.0` | UUID for state/nonce |
| `golang.org/x/oauth2 v0.36.0` | OAuth2 code exchange |
| `google.golang.org/grpc v1.79.1` | gRPC client |
| `google.golang.org/protobuf v1.36.11` | Proto types |
| `github.com/oapi-codegen/oapi-codegen/v2 v2.6.0` | OpenAPI code generation (tool) |
| `github.com/getkin/kin-openapi v0.133.0` | OpenAPI middleware / validation |
| `github.com/gorilla/securecookie v1.1.2` | Signed/encrypted cookies |
| `github.com/oklog/ulid/v2 v2.1.1` | ULID session IDs |
| `github.com/rs/cors v1.11.1` | CORS middleware |

New deps the BFF will need:
- **Redis client**: `github.com/redis/go-redis/v9` — not yet in go.mod

---

## 13. BFF Architecture Design Principles

### Session model

The BFF holds a server-side session in Redis keyed by a random session ID (`sid`).

Session record contains:
```
{
  "user_id":        string,          // sub from ID token
  "access_token":   string,          // current JWT access token
  "refresh_token":  string,          // opaque refresh token
  "id_token":       string,          // ID token (for end_session hint)
  "token_expiry":   RFC3339,         // when access_token expires
  "device_session_id": string|"",    // accounts DeviceSession ID (if extractable)
  "created_at":     RFC3339,
  "last_active_at": RFC3339
}
```

Session cookie: `__Host-sid` (HttpOnly, Secure, SameSite=Lax, Path=/)

### OIDC RP flow implementation

The BFF uses `coreos/go-oidc/v3` with `golang.org/x/oauth2`:

```go
provider, _ := oidc.NewProvider(ctx, "https://accounts.hss-science.org")
oauth2Config := oauth2.Config{
    ClientID:     "myaccount-bff",
    ClientSecret: "...",
    RedirectURL:  "https://myaccount.hss-science.org/api/v1/auth/callback",
    Endpoint:     provider.Endpoint(),
    Scopes:       []string{"openid", "email", "profile", "offline_access"},
}
```

PKCE flow:
1. Generate `code_verifier` (43 random bytes, base64url)
2. Compute `S256(verifier)` → `code_challenge`
3. Store verifier in a short-lived signed cookie or Redis entry keyed by `state`
4. Pass `code_challenge` + `code_challenge_method=S256` to authorize URL

### gRPC client setup

```go
conn, _ := grpc.NewClient(
    "accounts-service:50051",
    grpc.WithTransportCredentials(insecure.NewCredentials()), // internal network
)
client := accountsv1.NewAccountManagementServiceClient(conn)
```

Each call injects the access token:
```go
md := metadata.Pairs("authorization", "Bearer "+session.AccessToken)
ctx = metadata.NewOutgoingContext(ctx, md)
```

### REST API surface (OpenAPI)

Suggested endpoint structure for `myaccount-spa`:

```
GET  /api/v1/auth/login          → redirect to OIDC authorize
GET  /api/v1/auth/callback       → exchange code, set session
POST /api/v1/auth/logout         → delete session, redirect to end_session
GET  /api/v1/auth/me             → { logged_in: bool, user_id?, ... }

GET  /api/v1/profile             → Profile (gRPC GetMyProfile)
PATCH /api/v1/profile            → Profile (gRPC UpdateMyProfile)

GET  /api/v1/providers           → []FederatedProviderInfo (gRPC ListLinkedProviders)
DELETE /api/v1/providers/:id     → (gRPC UnlinkProvider)

GET  /api/v1/sessions            → []Session (gRPC ListActiveSessions)
DELETE /api/v1/sessions/:id      → (gRPC RevokeSession)
DELETE /api/v1/sessions          → (gRPC RevokeAllOtherSessions, passing current sid)
```

### Token refresh middleware

A middleware wraps all authenticated routes:
1. Load session from Redis by cookie `sid`.
2. If session missing → 401.
3. If access_token expires within 60 s → call accounts token endpoint to refresh.
4. If refresh fails (expired/revoked) → delete session, return 401.
5. Update session in Redis with new tokens (and TTL reset).
6. Inject access_token into request context.

---

## 14. Security Considerations

### CSRF protection

SPA calls are same-origin XHR/fetch with `SameSite=Lax` cookie. For `POST`/`PATCH`/`DELETE`
mutating endpoints, add double-submit cookie or `X-Requested-With` header check.

### State parameter (OIDC)

Generate a random `state` value (UUID), store it in a short-lived Redis key or encrypted cookie
(HMAC-signed), verify on callback before proceeding.

### Secret management

BFF needs:
- `CLIENT_SECRET` — the `myaccount-bff` secret (bcrypt hash is in DB; BFF sends the cleartext to the token endpoint)
- `SESSION_SECRET` — key for signing/encrypting session cookie or Redis keys
- `REDIS_URL` — Redis connection

### Token leakage

Access tokens are stored only in Redis (server-side). The browser never sees them. The session
cookie is HttpOnly. This is the core BFF security property.

### PKCE necessity

Even though `myaccount-bff` uses `client_secret_basic` (confidential client), adding PKCE is still
good practice and required by the accounts server for public clients. For a confidential client with
`auth_method != none`, the accounts server does NOT require PKCE (see `isPublicClient` check in
`storage.go:81`), so PKCE is optional but recommended.

---

## 15. DeviceSession ID Tracking

The accounts service assigns a `DeviceSession.ID` during login (stored in the `dsid` cookie on the
accounts service domain). This ID is linked to refresh tokens and is used for
`RevokeSession`/`RevokeAllOtherSessions` gRPC calls.

**Problem**: After the OIDC code exchange, the BFF does not receive the `device_session_id`
directly — it is an accounts-internal concept not currently embedded in the JWT or OIDC response.

**Options**:
1. **Use `ListActiveSessions`** to find the session matching the current device (by IP + UA
   correlation) — fragile.
2. **Embed `device_session_id` in the access token** as a private claim — requires accounts
   service change, `GetPrivateClaimsFromScopes` currently returns `{}`.
3. **Call `ListActiveSessions`** after login and cache the matching session ID — best feasible
   approach without accounts changes.
4. **Accept that the BFF's Redis session is not directly linked to a device session** — the BFF
   still manages its own session; device session management is a separate concern surfaced through
   `ListActiveSessions`.

For `RevokeAllOtherSessions`, the BFF needs to supply a `current_session_id`. If the BFF cannot
determine the current device session ID, it can skip this RPC and instead just list + revoke all
sessions individually (excluding none, or use the gRPC call with an empty string which will revoke
all).

---

## 16. Environment Variables for BFF

Suggested (mirroring accounts patterns):

```
PORT=8080
OIDC_ISSUER=https://accounts.hss-science.org   # accounts service issuer
CLIENT_ID=myaccount-bff
CLIENT_SECRET=...                               # plaintext secret
REDIRECT_URL=https://myaccount.hss-science.org/api/v1/auth/callback
ACCOUNTS_GRPC_ADDR=accounts-service:50051
REDIS_URL=redis://redis:6379/0
SESSION_KEY=<32 random hex bytes>              # for cookie signing
SESSION_TTL_MINUTES=60
CORS_ALLOWED_ORIGINS=https://myaccount.hss-science.org
```

---

## 17. Key Implementation Files in accounts to Watch

| File | Relevance |
|------|-----------|
| `migrations/2_seed_clients.up.sql` | Client registration — update callback/secret here |
| `internal/oidc/adapter/provider.go` | OIDC provider flags (PKCE, grants, etc.) |
| `internal/oidc/adapter/storage.go` | `GetPrivateClaimsFromScopes` — add device_session_id claim here if needed |
| `internal/grpc/interceptor.go` | JWT verification logic — what the BFF's gRPC must satisfy |
| `internal/grpc/errors.go` | gRPC error codes the BFF should handle |
| `api/proto/accounts/v1/account_management.proto` | Source of truth for all gRPC calls |
| `server/gen/accounts/v1/` | Generated Go client stubs — import directly in BFF |

---

## 18. Package Structure Recommendation for BFF

```
server/services/myaccount-bff/
├── main.go
├── Dockerfile
├── .env.example
├── config/
│   └── config.go              # env-based config
├── internal/
│   ├── session/
│   │   ├── store.go           # Redis session store
│   │   └── model.go           # Session struct
│   ├── oidcrp/
│   │   └── client.go          # OIDC RP (coreos/go-oidc + oauth2)
│   ├── middleware/
│   │   ├── auth.go            # session load + token refresh
│   │   └── csrf.go
│   ├── handler/
│   │   ├── auth.go            # login, callback, logout, /me
│   │   ├── profile.go         # GET/PATCH profile
│   │   ├── providers.go       # list/unlink federated providers
│   │   └── sessions.go        # list/revoke device sessions
│   └── accounts/
│       └── client.go          # gRPC client wrapper
└── api/
    └── openapi.yaml           # OpenAPI 3.x spec
```

---
## 19. Open Questions / Design Decisions

1. **Device session ID in JWT**: Should `accounts` embed `device_session_id` as a private JWT claim
   so BFF can pass it to `RevokeAllOtherSessions`? This requires a change to `GetPrivateClaimsFromScopes`.
   > **Decision: YES — embed `device_session_id` as a private claim in the access token.**
   > Modify `GetPrivateClaimsFromScopes` in the accounts service to include the device session ID.
   > The BFF extracts it from the JWT after token exchange and stores it in the Redis session.
   > This is the only natural point where the BFF can learn the device session ID without fragile
   > heuristics (IP/UA correlation). Required for correct `RevokeAllOtherSessions` behavior.

2. **Proactive token refresh vs. on-demand**: Should the BFF refresh the access token proactively
   (background goroutine per session) or lazily on each request? Lazy (per-request) is simpler and
   correct.
   > **Decision: Lazy (on-demand) refresh only.**
   > The auth middleware checks token expiry on every request and refreshes if within 60s of expiry.
   > Background goroutines would need to track all active sessions and make unnecessary calls for
   > idle sessions. Lazy refresh is simpler, correct, and scales naturally.

3. **Session TTL extension**: Should the Redis session TTL reset on every request (sliding window)?
   Or fixed lifetime from login? Recommend sliding window with hard max.
   > **Decision: Sliding window TTL (e.g. 2h) with a hard maximum (e.g. 7 days from login).**
   > Active users should not be unexpectedly logged out. A sliding window resets TTL on each
   > request. The hard max prevents sessions from living indefinitely. Store `created_at` in the
   > Redis session to enforce the hard max. This mirrors Google's session behavior.

4. **Multiple tabs / concurrent refresh**: Need a Redis lock or compare-and-swap during token
   refresh to avoid issuing multiple refresh calls simultaneously (refresh token rotation means
   second caller will get invalid refresh token error).
   > **Decision: Use a Redis NX lock during token refresh.**
   > Acquire `SET refresh_lock:<session_id> 1 NX EX 10` before calling the token endpoint.
   > If the lock is already held (another tab is refreshing), wait briefly and retry — the other
   > tab will have updated the session with fresh tokens. This serializes refresh calls per session
   > and prevents the second caller from receiving an invalid refresh token error.

5. **ID token caching**: Store the ID token in Redis for use as `id_token_hint` on logout? Yes —
   the accounts OIDC `end_session` endpoint accepts and validates it.
   > **Decision: YES — store the ID token in the Redis session.**
   > Required for clean logout via `end_session?id_token_hint=<id_token>`. Without it, the
   > accounts service cannot reliably identify the user/session to terminate on logout.

6. **gRPC TLS**: Currently main.go wires gRPC server without TLS (trusting internal network).
   BFF should use `insecure.NewCredentials()` if on same cluster network, or TLS if across networks.
   > **Decision: Use `insecure.NewCredentials()` — same cluster, no in-process TLS.**
   > TLS is terminated by Cloudflare Tunnel + reverse proxy at the infrastructure layer.
   > Adding application-level TLS for intra-cluster gRPC would be over-engineering for this setup.

7. **Error message consistency**: Decide on error response format for the REST API (JSON with
   `{"error": "...", "message": "..."}` pattern vs. standard problem+json).
   > **Decision: Simple JSON — `{"error": "<code>", "message": "<human-readable>"}`.**
   > Problem+JSON (RFC 7807) adds complexity with little benefit for a SPA consumer.
   > The `error` field holds a machine-readable code; `message` holds a human-readable description.
   > Example: `{"error": "unauthorized", "message": "session expired"}`.