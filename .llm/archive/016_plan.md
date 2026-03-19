# Implementation Plan: myaccount-bff

## Overview

Two separate bodies of work:

1. **A — accounts service: emit `dsid` private claim** (prerequisite, small change)
2. **B — new service: `server/services/myaccount-bff/`** (main deliverable)

Both live in the same Go module (`github.com/barn0w1/hss-science/server`).

---

## Part A — accounts service: `dsid` private claim in access token JWT

### Motivation

`GetPrivateClaimsFromScopes` currently returns `{}`. The BFF needs the accounts-side
`device_session_id` (a ULID) to call `RevokeAllOtherSessions(current_session_id)` correctly.
The device session ID is assigned during login and stored in `refresh_tokens.device_session_id`.
After OIDC token exchange the BFF never sees it otherwise.

Decision from research.md §19.1: embed it as private JWT claim `dsid`.

### How `GetPrivateClaimsFromScopes` is called (zitadel/oidc v3 internals)

The zitadel/oidc token endpoint handler:
1. Calls `StorageAdapter.CreateAccessAndRefreshTokens` → writes both tokens to DB, returns `accessID`
2. Assembles JWT claims: uses `accessID` as `jti`, subject, scopes, etc.
3. Calls `StorageAdapter.GetPrivateClaimsFromScopes(ctx, userID, clientID, scopes)` → merges extra claims
4. Signs the JWT

So by the time step 3 runs the refresh token row **already exists** in the DB. Querying
`refresh_tokens` for the latest active record for `(user_id, client_id)` will reliably
return the token just created.

### A.1 — `internal/oidc/ports.go`

Add to both `TokenRepository` and `TokenService` interfaces:

```go
// GetLatestDeviceSessionID returns the device_session_id of the most recently
// created active refresh token for the given user and client. Returns ("", nil)
// if none exists (access-only token flow).
GetLatestDeviceSessionID(ctx context.Context, userID, clientID string) (string, error)
```

**Why not re-use existing methods**: `GetRefreshToken` needs the raw token hash (unknown here).
`GetRefreshInfo` is the same. `GetByID` returns an access token which doesn't store dsid.
This new purpose-built method is the cleanest fit.

**Trade-off**: expanding the interface. Considered adding a separate `PrivateClaimsSource`
interface to `StorageAdapter` instead, but that would require a bigger refactor of
`NewStorageAdapter`. Expanding the existing interface keeps the diff minimal.

### A.2 — `internal/oidc/postgres/token_repo.go`

Add method on `*TokenRepository`:

```go
func (r *TokenRepository) GetLatestDeviceSessionID(ctx context.Context, userID, clientID string) (string, error) {
    var dsid sql.NullString
    err := r.db.QueryRowxContext(ctx,
        `SELECT device_session_id
         FROM refresh_tokens
         WHERE user_id = $1 AND client_id = $2 AND expiration > now()
         ORDER BY created_at DESC
         LIMIT 1`,
        userID, clientID,
    ).Scan(&dsid)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return "", nil
        }
        return "", fmt.Errorf("get latest device session id: %w", err)
    }
    if dsid.Valid {
        return dsid.String, nil
    }
    return "", nil
}
```

**Why `sql.NullString`**: `device_session_id` is nullable — the column can be NULL when a
refresh token was issued without a device session (e.g. machine-to-machine via client credentials).

**Index**: The query filters on `(user_id, client_id)` and sorts by `created_at DESC`. Check
whether `refresh_tokens` has a composite index on these columns. If not, add a non-blocking
`CREATE INDEX CONCURRENTLY` in a future migration (the table is expected to be small for
myaccount-bff given 1 refresh token per session, so no immediate perf concern).

### A.3 — `internal/oidc/token_svc.go`

Add delegation method:

```go
func (s *tokenService) GetLatestDeviceSessionID(ctx context.Context, userID, clientID string) (string, error) {
    return s.repo.GetLatestDeviceSessionID(ctx, userID, clientID)
}
```

No error wrapping beyond what the repo provides; callers treat a blank string as "not available".

### A.4 — `internal/oidc/adapter/storage.go`

Replace:
```go
func (s *StorageAdapter) GetPrivateClaimsFromScopes(_ context.Context, _, _ string, _ []string) (map[string]any, error) {
    return map[string]any{}, nil
}
```

With:
```go
func (s *StorageAdapter) GetPrivateClaimsFromScopes(ctx context.Context, userID, clientID string, _ []string) (map[string]any, error) {
    dsid, err := s.tokens.GetLatestDeviceSessionID(ctx, userID, clientID)
    if err != nil {
        // Non-fatal: log and return empty rather than failing token issuance.
        slog.Warn("GetPrivateClaimsFromScopes: could not fetch device session id",
            "user_id", userID, "client_id", clientID, "error", err)
        return map[string]any{}, nil
    }
    if dsid == "" {
        return map[string]any{}, nil
    }
    return map[string]any{"dsid": dsid}, nil
}
```

**Error handling rationale**: Returning an error from `GetPrivateClaimsFromScopes` causes
zitadel/oidc to abort the entire token request. Since `dsid` is an enhancement (not required for
the token to be valid), a DB error here should NOT block token issuance. We log a warning and
return empty claims.

**Claim name `dsid`**: Short, not colliding with any standard OIDC claim. Since accounts and BFF
are the only consumers, an opaque shortname is fine. Full URI namespace (`hss-science.org/dsid`)
was considered but adds noise for no benefit in this closed system.

**Impact on other clients**: Any other registered client that does not need `dsid` simply gets a
claim it ignores. Does not break existing JWT consumers because all existing verifiers only check
`sub`, `iss`, `exp` (the gRPC interceptor). If the claim were ever sensitive, a scope-gate could
be added later, but device session IDs are not secret.

### A.5 — no migration needed

The change is read-only against existing `refresh_tokens`. No schema change.

### A.6 — test updates

- `storage_test.go`: update the `GetPrivateClaimsFromScopes` test stub expectation to allow a
  non-empty result (or inject a mock that returns a dsid and assert the claim is present).
- `token_repo_test.go`: add a `TestGetLatestDeviceSessionID` covering: found, not found, nullable.

---

## Part B — new service: `server/services/myaccount-bff/`

### B.0 — go.mod: add Redis client

One new direct dependency:

```
github.com/redis/go-redis/v9
```

All other dependencies are already in `go.mod`. Add with `go get github.com/redis/go-redis/v9`
from `server/`. This updates `go.mod` and `go.sum` in place.

**Why go-redis/v9**: The most widely-used Go Redis client, supports Redis 7, has an idiomatic
context-based API matching the rest of the codebase, and has a `redis.Nil` sentinel analogous to
`sql.ErrNoRows`.

---

### B.1 — File tree

The OpenAPI spec lives at `api/openapi/myaccount/v1/myaccount.yaml` in the repo root
(alongside `api/proto/`), not inside the service directory. The BFF references it but does not own it.

```
server/services/myaccount-bff/
├── main.go
├── Dockerfile
├── .env.example
├── config/
│   └── config.go
└── internal/
    ├── session/
    │   ├── model.go
    │   └── store.go
    ├── oidcrp/
    │   └── client.go
    ├── accounts/
    │   └── client.go
    ├── middleware/
    │   ├── auth.go
    │   └── csrf.go
    └── handler/
        ├── auth.go
        ├── profile.go
        ├── providers.go
        └── sessions.go
```

---

### B.2 — `config/config.go`

Pattern mirrors `services/identity-service/config/config.go` exactly: `ConfigSource` interface,
`OSEnvSource`, `MapSource`, `LoadFrom(src)`.

```go
type Config struct {
    Port           string
    OIDCIssuer     string     // OIDC_ISSUER — accounts service issuer URL
    ClientID       string     // CLIENT_ID — "myaccount-bff"
    ClientSecret   string     // CLIENT_SECRET — plaintext secret
    RedirectURL    string     // REDIRECT_URL
    AccountsGRPC   string     // ACCOUNTS_GRPC_ADDR — e.g. "accounts-service:50051"
    RedisURL       string     // REDIS_URL — e.g. "redis://redis:6379/0"
    SessionKey     [32]byte   // SESSION_KEY — 64 hex chars → 32 bytes (for HMAC or encryption)
    SessionIdleTTL time.Duration  // SESSION_IDLE_TTL_MINUTES (default 120)
    SessionHardTTL time.Duration  // SESSION_HARD_TTL_DAYS (default 7)
    CORSOrigins    []string   // CORS_ALLOWED_ORIGINS — comma-separated
}
```

Validation:
- `OIDCIssuer`, `ClientID`, `ClientSecret`, `RedirectURL`, `AccountsGRPC`, `RedisURL` required
- `SESSION_KEY`: must be exactly 64 hex chars → 32 bytes (same pattern as `CRYPTO_KEY` in accounts)
- `SessionIdleTTL`: 5–1440 min (default 120)
- `SessionHardTTL`: 1–90 days (default 7)
- `CORSOrigins`: at least one entry required

---

### B.3 — `internal/session/model.go`

```go
package session

import "time"

type Session struct {
    UserID          string    `json:"user_id"`
    AccessToken     string    `json:"access_token"`
    RefreshToken    string    `json:"refresh_token"`
    IDToken         string    `json:"id_token"`
    TokenExpiry     time.Time `json:"token_expiry"`
    DeviceSessionID string    `json:"device_session_id"` // from "dsid" JWT claim; "" if absent
    CreatedAt       time.Time `json:"created_at"`
    LastActiveAt    time.Time `json:"last_active_at"`
}
```

Stored as JSON in Redis. No encryption of token values — Redis is trusted internal infrastructure.
If encryption at rest is needed later, wrap the JSON with `crypto/AES-GCM` using `SessionKey`.

---

### B.4 — `internal/session/store.go`

```go
package session

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "time"

    "github.com/redis/go-redis/v9"
)

const (
    keyPrefix     = "session:"
    lockPrefix    = "refresh_lock:"
    statePrefix   = "oidc_state:"
    lockTTL       = 10 * time.Second
    stateTTL      = 10 * time.Minute
)

type Store struct {
    rdb     *redis.Client
    idleTTL time.Duration
    hardTTL time.Duration
}

func NewStore(rdb *redis.Client, idleTTL, hardTTL time.Duration) *Store { ... }

// Save serialises the session as JSON and sets it with idleTTL.
func (s *Store) Save(ctx context.Context, sid string, sess *Session) error { ... }

// Load retrieves and deserialises the session, sliding the TTL.
// Returns nil, ErrNotFound if the key does not exist.
func (s *Store) Load(ctx context.Context, sid string) (*Session, error) { ... }

// Delete removes the session key.
func (s *Store) Delete(ctx context.Context, sid string) error { ... }

// AcquireRefreshLock sets "refresh_lock:<sid>" with NX EX 10s.
// Returns true if the lock was acquired, false if already held.
func (s *Store) AcquireRefreshLock(ctx context.Context, sid string) (bool, error) { ... }

// ReleaseRefreshLock deletes the lock key.
func (s *Store) ReleaseRefreshLock(ctx context.Context, sid string) error { ... }

// SaveState stores {verifier} as JSON under "oidc_state:<state>", TTL 10 min.
func (s *Store) SaveState(ctx context.Context, state, verifier string) error { ... }

// LoadAndDeleteState atomically GET+DEL the state entry (one-time use).
// Returns ("", ErrNotFound) if missing or expired.
func (s *Store) LoadAndDeleteState(ctx context.Context, state string) (verifier string, err error) { ... }

var ErrNotFound = errors.New("session: not found")
```

**Load implementation detail**: Use `redis.Client.GetEx` (GET + set new expiry atomically)
to slide the TTL in one round-trip. Then unmarshal JSON. After loading, check
`session.CreatedAt + hardTTL < now()` → return `ErrNotFound` (forces re-login) and `Delete`.

**LoadAndDeleteState**: Use a Lua script or a pipeline with `GET` + `DEL` to ensure one-time use.
Simplest: `GETDEL` command (Redis 6.2+). If Redis version is uncertain, use a Lua script:
```lua
local v = redis.call('GET', KEYS[1])
if v then redis.call('DEL', KEYS[1]) end
return v
```

---

### B.5 — `internal/oidcrp/client.go`

OIDC Relying Party setup using `coreos/go-oidc/v3` + `golang.org/x/oauth2`:

```go
package oidcrp

import (
    "context"
    "crypto/sha256"
    "encoding/base64"

    gooidc "github.com/coreos/go-oidc/v3/oidc"
    "golang.org/x/oauth2"
)

type Client struct {
    provider    *gooidc.Provider
    oauth2Cfg   oauth2.Config
    verifier    *gooidc.IDTokenVerifier
}

func New(ctx context.Context, issuer, clientID, clientSecret, redirectURL string) (*Client, error) {
    provider, err := gooidc.NewProvider(ctx, issuer)
    // OIDC discovery fetches /.well-known/openid-configuration
    // Caches internally; does not re-fetch on each request.
    ...
    verifer := provider.Verifier(&gooidc.Config{ClientID: clientID})
    return &Client{
        provider: provider,
        oauth2Cfg: oauth2.Config{
            ClientID:     clientID,
            ClientSecret: clientSecret,
            RedirectURL:  redirectURL,
            Endpoint:     provider.Endpoint(),
            Scopes:       []string{"openid", "email", "profile", "offline_access"},
        },
        verifier: verifier,
    }, nil
}

// AuthCodeURL builds the authorization redirect URL.
// Returns (url, state, verifier) — state and verifier are generated internally.
func (c *Client) AuthCodeURL() (url, state, verifier string) { ... }
// Uses oauth2.GenerateVerifier() and oauth2.S256ChallengeOption(verifier).

// Exchange trades the code for tokens.
// Returns the token set and the verified IDToken claims.
func (c *Client) Exchange(ctx context.Context, code, verifier string) (*oauth2.Token, *gooidc.IDToken, error) { ... }

// EndSessionURL returns the end_session endpoint URL with id_token_hint.
func (c *Client) EndSessionURL(idToken, postLogoutRedirectURI string) (string, error) { ... }
// Parses provider's end_session_endpoint from discovery document.
// gooidc.Provider.Endpoint() doesn't expose this; use provider's raw claims:
//   var raw struct{ EndSessionEndpoint string `json:"end_session_endpoint"` }
//   provider.Claims(&raw)
```

**S256 PKCE**: `oauth2.GenerateVerifier()` generates a 32-byte random base64url verifier.
`oauth2.S256ChallengeOption(verifier)` passes `code_challenge` + `code_challenge_method=S256`
to `AuthCodeURL`. `oauth2.VerifierOption(verifier)` passes `code_verifier` to `Exchange`.
These are stdlib `golang.org/x/oauth2` helpers — no manual SHA256 needed.

**ID token extraction**: After `Exchange`, get raw id_token string:
```go
rawIDToken, ok := token.Extra("id_token").(string)
```
Verify with `c.verifier.Verify(ctx, rawIDToken)` → `*gooidc.IDToken`.

**`dsid` claim extraction from access token**: The access token is a JWT. After exchange,
decode its payload (base64url-decode middle segment) without verification (accounts' gRPC
interceptor already verifies it on every gRPC call; the BFF trusts it for claim reading after
obtaining it directly from the accounts token endpoint):
```go
func extractDSID(rawAccessToken string) string {
    parts := strings.Split(rawAccessToken, ".")
    if len(parts) != 3 { return "" }
    payload, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil { return "" }
    var claims struct{ DSID string `json:"dsid"` }
    _ = json.Unmarshal(payload, &claims)
    return claims.DSID
}
```
**Why no signature verification here**: The BFF obtained this JWT directly from the accounts
token endpoint over a trusted internal connection; there is no MITM risk. Verifying the signature
would require a JWKS fetch which is unnecessary overhead for this internal use. The JWT's
signature IS verified by the accounts gRPC interceptor on every API call, which is the security
boundary.

---

### B.6 — `internal/accounts/client.go`

gRPC client wrapper:

```go
package accounts

import (
    "context"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
    "google.golang.org/grpc/metadata"

    pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

type Client struct {
    svc pb.AccountManagementServiceClient
}

func New(addr string) (*Client, error) {
    conn, err := grpc.NewClient(addr,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    ...
    return &Client{svc: pb.NewAccountManagementServiceClient(conn)}, nil
}

func (c *Client) withBearer(ctx context.Context, token string) context.Context {
    return metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token))
}

func (c *Client) GetMyProfile(ctx context.Context, token string) (*pb.Profile, error) {
    return c.svc.GetMyProfile(c.withBearer(ctx, token), &pb.GetMyProfileRequest{})
}

func (c *Client) UpdateMyProfile(ctx context.Context, token string, name, picture *string) (*pb.Profile, error) {
    return c.svc.UpdateMyProfile(c.withBearer(ctx, token), &pb.UpdateMyProfileRequest{
        Name:    name,
        Picture: picture,
    })
}

func (c *Client) ListLinkedProviders(ctx context.Context, token string) ([]*pb.FederatedProviderInfo, error) { ... }
func (c *Client) UnlinkProvider(ctx context.Context, token, identityID string) error { ... }
func (c *Client) ListActiveSessions(ctx context.Context, token string) ([]*pb.Session, error) { ... }
func (c *Client) RevokeSession(ctx context.Context, token, sessionID string) error { ... }
func (c *Client) RevokeAllOtherSessions(ctx context.Context, token, currentSessionID string) error { ... }
```

**insecure.NewCredentials()**: Both services run in the same cluster; TLS is terminated at the
infrastructure layer (researh.md §19.6). Using insecure credentials for intra-cluster gRPC is
the established pattern already used in accounts' own test helpers.

**gRPC error mapping** (used in handlers, not here):

| gRPC code | HTTP status |
|-----------|-------------|
| `codes.NotFound` | 404 |
| `codes.InvalidArgument` | 400 |
| `codes.PermissionDenied` | 403 |
| `codes.Unauthenticated` | 401 |
| `codes.FailedPrecondition` | 409 |
| catchall | 500 |

Helper in a shared `internal/handler/errors.go`:
```go
func grpcToHTTP(err error) (int, string) { ... }
func writeError(w http.ResponseWriter, status int, code, message string) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    _ = json.NewEncoder(w).Encode(map[string]string{"error": code, "message": message})
}
```

---

### B.7 — `internal/middleware/csrf.go`

Simplified CSRF protection using `X-Requested-With` header check:

```go
func CSRF(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodPost || r.Method == http.MethodPatch ||
            r.Method == http.MethodDelete || r.Method == http.MethodPut {
            if r.Header.Get("X-Requested-With") != "XMLHttpRequest" {
                http.Error(w, `{"error":"csrf","message":"X-Requested-With header required"}`,
                    http.StatusForbidden)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}
```

**Why X-Requested-With**: SPA always sets this; simple browser form posts or redirects cannot.
Works correctly with `SameSite=Lax` cookie. No state/token needed — greatly reduces complexity.

**Trade-off vs. double-submit cookie**: Double-submit cookie requires the SPA to read a cookie
and echo it as a header. `X-Requested-With` is simpler for the SPA client and achieves the same
protection since `SameSite=Lax` already blocks cross-site POST.

---

### B.8 — `internal/middleware/auth.go`

Wraps all routes under `/api/v1/` except `/api/v1/auth/login` and `/api/v1/auth/callback`.

```go
type contextKey int
const ctxKeySession contextKey = iota

func SessionFromContext(ctx context.Context) *session.Session { ... }

func Auth(store *session.Store, oidcRP *oidcrp.Client) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // 1. Read cookie
            cookie, err := r.Cookie("__Host-sid")
            if err != nil {
                writeError(w, 401, "unauthorized", "not authenticated")
                return
            }
            sid := cookie.Value

            // 2. Load session (also slides TTL, enforces hard max)
            sess, err := store.Load(r.Context(), sid)
            if errors.Is(err, session.ErrNotFound) {
                clearCookie(w)
                writeError(w, 401, "unauthorized", "session expired")
                return
            }
            if err != nil {
                writeError(w, 500, "internal", "session store error")
                return
            }

            // 3. Refresh access token if needed
            if time.Until(sess.TokenExpiry) < 60*time.Second {
                sess, err = refreshTokens(r.Context(), store, oidcRP, sid, sess)
                if err != nil {
                    if errors.Is(err, ErrRefreshFailed) {
                        store.Delete(r.Context(), sid)
                        clearCookie(w)
                        writeError(w, 401, "unauthorized", "session expired")
                        return
                    }
                    writeError(w, 500, "internal", "token refresh error")
                    return
                }
            }

            ctx := context.WithValue(r.Context(), ctxKeySession, sess)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

**`refreshTokens` function** (internal helper):

```go
func refreshTokens(ctx context.Context, store *session.Store, rp *oidcrp.Client, sid string, sess *session.Session) (*session.Session, error) {
    // Acquire lock (NX EX 10s)
    acquired, err := store.AcquireRefreshLock(ctx, sid)
    if err != nil {
        return nil, err
    }
    if !acquired {
        // Another tab is refreshing. Wait and reload.
        time.Sleep(500 * time.Millisecond)
        reloaded, err := store.Load(ctx, sid)
        if errors.Is(err, session.ErrNotFound) {
            return nil, ErrRefreshFailed
        }
        return reloaded, err
    }
    defer store.ReleaseRefreshLock(ctx, sid)

    // Check again after acquiring lock (double-check)
    if time.Until(sess.TokenExpiry) >= 60*time.Second {
        return sess, nil  // Another goroutine refreshed it already
    }

    newToken, _, err := rp.RefreshToken(ctx, sess.RefreshToken)
    if err != nil {
        return nil, ErrRefreshFailed
    }

    sess.AccessToken = newToken.AccessToken
    sess.RefreshToken = newToken.RefreshToken
    sess.TokenExpiry = newToken.Expiry
    if newRaw, ok := newToken.Extra("id_token").(string); ok && newRaw != "" {
        sess.IDToken = newRaw
    }
    if newDSID := extractDSID(newToken.AccessToken); newDSID != "" {
        sess.DeviceSessionID = newDSID
    }
    sess.LastActiveAt = time.Now().UTC()

    if err := store.Save(ctx, sid, sess); err != nil {
        return nil, err
    }
    return sess, nil
}

var ErrRefreshFailed = errors.New("refresh failed")
```

**`oidcrp.Client.RefreshToken`** method to add:
```go
func (c *Client) RefreshToken(ctx context.Context, rawRefreshToken string) (*oauth2.Token, *gooidc.IDToken, error) {
    tokenSrc := c.oauth2Cfg.TokenSource(ctx, &oauth2.Token{RefreshToken: rawRefreshToken})
    tok, err := tokenSrc.Token()
    ...
}
```

**Sliding TTL**: `store.Load` already calls `GETEX` to reset idle TTL on each request (B.4).

**Double-check after lock**: Necessary because two concurrent requests may both observe
`<60s remaining`, but only the first one actually needs to refresh. The second should use
the freshly `store.Save`d tokens from the first.

**Sleep simplicity**: `500ms` sleep before retry is a pragmatic choice. A more sophisticated
approach (polling with exponential backoff) is overkill for this use case where the refresh
call itself takes <200ms in normal conditions.

---

### B.9 — `internal/handler/auth.go`

#### `GET /api/v1/auth/login`

```
1. Generate state (UUID v4) and PKCE verifier (via rp.AuthCodeURL())
2. Store {verifier} in Redis: "oidc_state:<state>", TTL 10 min
3. Redirect 302 to authorize URL
```

State lives in Redis keyed by itself — no state cookie needed.

#### `GET /api/v1/auth/callback?code=...&state=...`

```
1. Read `code` and `state` from query
2. LoadAndDeleteState(state) → verifier — or 400 if missing/expired
3. Exchange(ctx, code, verifier) → token, idToken
4. Extract dsid from access token JWT payload (B.5)
5. Build Session{UserID: idToken.Subject, AccessToken, RefreshToken, IDToken, TokenExpiry,
                  DeviceSessionID: dsid, CreatedAt: now, LastActiveAt: now}
6. Generate sid = ulid.Make().String()
7. store.Save(ctx, sid, &sess)
8. Set cookie: __Host-sid=<sid>; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=604800
9. Redirect 302 to "/"
```

**Cookie details**:
- `__Host-` prefix: requires `Secure`, prohibits `Domain`, requires `Path=/`.
  Provides strongest same-site binding.
- `Max-Age`: 7 days (matches `SESSION_HARD_TTL_DAYS` default). Browser will clear it after
  7 days regardless of server-side session. Server-side still enforces hard max independently.
- `SameSite=Lax`: Allows cookie on top-level navigations (initial redirect back from accounts).

#### `POST /api/v1/auth/logout`

```
1. Auth middleware: load session (required)
2. Capture id_token from session
3. Delete Redis session
4. Clear cookie (Max-Age=0)
5. Build end_session URL: rp.EndSessionURL(idToken, "https://myaccount.hss-science.org/")
6. Write JSON: {"redirect_to": "<end_session_url>"}  — let SPA handle the redirect
```

**Why return redirect URL instead of server-side redirect**: The SPA's `POST /api/v1/auth/logout`
is an XHR/fetch. A 302 redirect from XHR goes to the redirect target transparently, which would
not navigate the browser. Return JSON with the URL and let the SPA do `window.location.href = ...`.

#### `GET /api/v1/auth/me`

```
1. Try to load session from cookie (no 401 on missing — unauthenticated users are valid)
2. Return JSON:
   If session: {"logged_in": true, "user_id": "<sub>"}
   If no session: {"logged_in": false}
```

No auth middleware on this endpoint — it must work for unauthenticated users.

---

### B.10 — `internal/handler/profile.go`

#### `GET /api/v1/profile`

```
1. Auth middleware provides session
2. Call accounts.GetMyProfile(ctx, session.AccessToken)
3. Map pb.Profile → JSON response
4. Map gRPC errors → HTTP errors
```

Response JSON (mirrors proto `Profile`):
```json
{
  "user_id": "01JXXX",
  "email": "user@example.com",
  "email_verified": true,
  "name": "Taro Yamada",
  "given_name": "Taro",
  "family_name": "Yamada",
  "picture": "https://...",
  "name_is_local": false,
  "picture_is_local": false,
  "created_at": "2025-01-01T00:00:00Z",
  "updated_at": "2025-01-15T12:00:00Z"
}
```

#### `PATCH /api/v1/profile`

Request body:
```json
{"name": "New Name", "picture": "https://..."}
```

Both fields optional (omitting them leaves the field unchanged — matches proto `optional`).
Parse with `json.Decoder`. Pass `*string` pointers to `accounts.UpdateMyProfile`.

---

### B.11 — `internal/handler/providers.go`

#### `GET /api/v1/providers`

Call `accounts.ListLinkedProviders` → return array of:
```json
[{
  "identity_id": "01JYYY",
  "provider": "google",
  "provider_email": "user@gmail.com",
  "last_login_at": "2026-03-15T10:00:00Z"
}]
```

#### `DELETE /api/v1/providers/{identityID}`

```
1. Read {identityID} from URL
2. Call accounts.UnlinkProvider(ctx, token, identityID)
3. 204 No Content on success
4. Handle codes.FailedPrecondition → 409 (would leave user with no login method)
```

---

### B.12 — `internal/handler/sessions.go`

#### `GET /api/v1/sessions`

Call `accounts.ListActiveSessions` → array of:
```json
[{
  "session_id": "01JZZZ",
  "device_name": "Chrome on macOS",
  "ip_address": "203.0.113.1",
  "created_at": "2026-03-01T00:00:00Z",
  "last_used_at": "2026-03-15T10:00:00Z",
  "is_current": true    // session_id == session.DeviceSessionID
}]
```

The `is_current` flag is added by the BFF by comparing each `session_id` against
`session.DeviceSessionID` from the Redis session. This lets the SPA highlight the current device
without needing extra context from accounts.

#### `DELETE /api/v1/sessions/{sessionID}`

Call `accounts.RevokeSession(ctx, token, sessionID)`. 204 on success.

**Self-revocation edge case**: If `sessionID == session.DeviceSessionID`, also delete the
BFF's own Redis session and clear the cookie (forces logout of current session).

#### `DELETE /api/v1/sessions` (revoke all others)

```
1. Call accounts.RevokeAllOtherSessions(ctx, token, session.DeviceSessionID)
2. session.DeviceSessionID may be "" if dsid was absent from JWT (older sessions pre-A.4 change)
   — in that case, the call goes through with empty string, which causes accounts to revoke
     ALL sessions (including current). This is an acceptable edge case for old sessions.
   — Could guard: if DeviceSessionID == "", return 409 with message "cannot identify current session"
3. 204 on success
```

---

### B.13 — `api/openapi/myaccount/v1/myaccount.yaml`

OpenAPI 3.0.3 spec at `api/openapi/myaccount/v1/myaccount.yaml` in the repo root
(alongside `api/proto/`). Use `oapi-codegen` (already in `go.mod` as a tool) to generate
request/response types only. Handler implementation follows the same hand-written chi
pattern as the accounts service.

Generator invocation (add to `Makefile`):
```makefile
generate-bff:
    go tool oapi-codegen -generate types \
        -package handler \
        -o server/services/myaccount-bff/internal/handler/api_gen.go \
        api/openapi/myaccount/v1/myaccount.yaml
```

Key spec structure:

```yaml
openapi: "3.0.3"
info:
  title: myaccount-bff
  version: "1.0"

paths:
  /api/v1/auth/login:
    get:
      operationId: AuthLogin
      summary: Initiate OIDC login
      responses:
        "302": { description: "Redirect to accounts authorize" }

  /api/v1/auth/callback:
    get:
      operationId: AuthCallback
      parameters:
        - in: query; name: code; required: true
        - in: query; name: state; required: true
      responses:
        "302": { description: "Redirect to SPA on success" }
        "400": { $ref: "#/components/responses/Error" }

  /api/v1/auth/logout:
    post:
      operationId: AuthLogout
      responses:
        "200": { content: {application/json: {schema: {$ref: "#/components/schemas/LogoutResponse"}}}}
        "401": { $ref: "#/components/responses/Unauthorized" }

  /api/v1/auth/me:
    get:
      operationId: AuthMe
      responses:
        "200": { content: {application/json: {schema: {$ref: "#/components/schemas/MeResponse"}}}}

  /api/v1/profile:
    get: ...
    patch: ...

  /api/v1/providers:
    get: ...

  /api/v1/providers/{identityId}:
    delete: ...

  /api/v1/sessions:
    get: ...
    delete: ...

  /api/v1/sessions/{sessionId}:
    delete: ...

components:
  schemas:
    Profile: { ... }
    Session: { properties: { ..., is_current: {type: boolean} } }
    FederatedProvider: { ... }
    Error: { properties: { error: {type: string}, message: {type: string} } }
    MeResponse:
      properties:
        logged_in: {type: boolean}
        user_id: {type: string}
    LogoutResponse:
      properties:
        redirect_to: {type: string}
  responses:
    Error:
      description: "Error response"
      content:
        application/json:
          schema: { $ref: "#/components/schemas/Error" }
    Unauthorized:
      description: "Not authenticated"
      content:
        application/json:
          schema: { $ref: "#/components/schemas/Error" }
```

---

### B.14 — `main.go`

Wire everything together:

```go
func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(logger)

    cfg, err := config.Load()
    // ...

    // Redis
    rdbOpts, _ := redis.ParseURL(cfg.RedisURL)
    rdb := redis.NewClient(rdbOpts)
    defer rdb.Close()
    if err := rdb.Ping(ctx).Err(); err != nil { logger.Error(...); os.Exit(1) }

    sessionStore := session.NewStore(rdb, cfg.SessionIdleTTL, cfg.SessionHardTTL)

    // OIDC RP
    oidcRP, err := oidcrp.New(ctx, cfg.OIDCIssuer, cfg.ClientID, cfg.ClientSecret, cfg.RedirectURL)
    // ...

    // gRPC client
    accountsClient, err := accounts.New(cfg.AccountsGRPC)
    // ...

    // Middlewares
    authMW   := middleware.Auth(sessionStore, oidcRP)
    csrfMW   := middleware.CSRF

    // Handlers
    authH      := handler.NewAuth(sessionStore, oidcRP)
    profileH   := handler.NewProfile(accountsClient)
    providersH := handler.NewProviders(accountsClient)
    sessionsH  := handler.NewSessions(accountsClient)

    // Router
    r := chi.NewRouter()
    r.Use(chimiddleware.Recoverer)
    r.Use(cors.New(cors.Options{
        AllowedOrigins:   cfg.CORSOrigins,
        AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
        AllowedHeaders:   []string{"Content-Type", "X-Requested-With"},
        AllowCredentials: true,
    }).Handler)
    r.Use(securityHeaders)

    r.Get("/api/v1/auth/login",    authH.Login)
    r.Get("/api/v1/auth/callback", authH.Callback)
    r.Get("/api/v1/auth/me",       authH.Me)

    r.Group(func(r chi.Router) {
        r.Use(authMW)
        r.Use(csrfMW)

        r.Post("/api/v1/auth/logout",    authH.Logout)
        r.Get("/api/v1/profile",         profileH.Get)
        r.Patch("/api/v1/profile",       profileH.Update)
        r.Get("/api/v1/providers",       providersH.List)
        r.Delete("/api/v1/providers/{identityID}", providersH.Unlink)
        r.Get("/api/v1/sessions",        sessionsH.List)
        r.Delete("/api/v1/sessions",     sessionsH.RevokeAllOthers)
        r.Delete("/api/v1/sessions/{sessionID}", sessionsH.Revoke)
    })

    r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
    r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
        if err := rdb.Ping(r.Context()).Err(); err != nil {
            http.Error(w, "redis not ready", 503); return
        }
        w.WriteHeader(200)
    })

    srv := &http.Server{
        Addr:              ":" + cfg.Port,
        Handler:           r,
        ReadHeaderTimeout: 5 * time.Second,
        ReadTimeout:       10 * time.Second,
        WriteTimeout:      30 * time.Second,
        IdleTimeout:       120 * time.Second,
    }
    // Graceful shutdown on SIGINT/SIGTERM (same pattern as accounts/main.go)
    ...
}
```

**Security headers middleware** (reuse pattern from `accounts/internal/middleware/securityheaders.go`):
```
X-Frame-Options: DENY
X-Content-Type-Options: nosniff
Referrer-Policy: strict-origin-when-cross-origin
Content-Security-Policy: default-src 'none'   ← BFF serves no HTML/JS
```

---

### B.15 — `Dockerfile`

Mirror `services/identity-service/Dockerfile` exactly:
- Multi-stage build: `golang:1.26-alpine` builder → `gcr.io/distroless/static-debian12` runtime
- Build args: `SERVICE=myaccount-bff`
- `go build -ldflags="-s -w" -o /app ./services/myaccount-bff`
- Non-root UID 10001
- `EXPOSE 8080`

---

### B.16 — `.env.example`

```env
PORT=8080

# accounts service OIDC issuer
OIDC_ISSUER=https://accounts.hss-science.org

# OIDC client credentials (myaccount-bff client registered in accounts DB)
CLIENT_ID=myaccount-bff
CLIENT_SECRET=changeme

# Must match the redirect_uris registered for myaccount-bff in accounts DB
REDIRECT_URL=https://myaccount.hss-science.org/api/v1/auth/callback

# gRPC address of the accounts service (intra-cluster, no TLS)
ACCOUNTS_GRPC_ADDR=accounts-service:50051

# Redis URL
REDIS_URL=redis://redis:6379/0

# 64 hex chars = 32 bytes, used for session signing
# Generate: openssl rand -hex 32
SESSION_KEY=0000000000000000000000000000000000000000000000000000000000000000

# Session lifetime settings
SESSION_IDLE_TTL_MINUTES=120    # reset on each request (sliding window)
SESSION_HARD_TTL_DAYS=7        # absolute max from login

# CORS — comma-separated allowed origins
CORS_ALLOWED_ORIGINS=https://myaccount.hss-science.org
```

---

## Dependency Graph (implementation order)

```
A.1 ports.go  ──►  A.2 token_repo.go  ──►  A.3 token_svc.go  ──►  A.4 storage.go
                                                                         (accounts service change complete)

B.0 go.mod  ──►  B.1 file tree  ──►  B.2 config  ──►  B.3 session/model
                                                    ──►  B.4 session/store  ──►  B.8 middleware/auth
                                                    ──►  B.5 oidcrp/client  ──►  B.8 middleware/auth
                                                    ──►  B.6 accounts/client  ──►  B.10-12 handlers
                                                    ──►  B.7 middleware/csrf  ──►  B.14 main.go
                                   B.9 handler/auth      ──►  B.14 main.go
                                   B.10 handler/profile  ──►  B.14 main.go
                                   B.11 handler/providers ──► B.14 main.go
                                   B.12 handler/sessions  ──► B.14 main.go
                                   B.13 openapi.yaml  (can be written any time)
                                   B.15 Dockerfile        ──►  after B.14
                                   B.16 .env.example      ──►  after B.2
```

---

## Key decisions summary (from research.md §19)

| # | Decision | Implemented in |
|---|----------|----------------|
| 1 | `dsid` private claim in access JWT | A.1–A.4 |
| 2 | Lazy token refresh | B.8 auth middleware |
| 3 | Sliding TTL (2h) + hard max (7d) | B.4 store.Load + B.8 |
| 4 | Redis NX lock for concurrent refresh | B.4 AcquireRefreshLock + B.8 refreshTokens |
| 5 | Store ID token in Redis session | B.3 model, B.9 callback handler |
| 6 | insecure gRPC (same cluster) | B.6 accounts/client.go |
| 7 | Simple JSON errors `{error, message}` | B.6 grpcToHTTP + all handlers |

---

## Trade-offs not yet resolved

1. **Access token size**: Once `dsid` is added as a private claim, the JWT grows by ~32 chars.
   Negligible for Bearer token in gRPC metadata.

2. **Redis availability**: BFF is completely unavailable if Redis is down. No fallback or
   degraded mode. Acceptable for this service — all user sessions require Redis.

3. **Session store encryption**: Currently, access token is stored in plaintext in Redis.
   If Redis is treated as a shared/untrusted database, add AES-GCM encryption using `SessionKey`.
   For now, Redis is internal-only; plaintext is acceptable.

---

## Todo

### Phase A — accounts service (`dsid` private claim)

- [x] **A.1** `internal/oidc/ports.go` — add `GetLatestDeviceSessionID` to `TokenRepository` interface
- [x] **A.1** `internal/oidc/ports.go` — add `GetLatestDeviceSessionID` to `TokenService` interface
- [x] **A.2** `internal/oidc/postgres/token_repo.go` — implement `GetLatestDeviceSessionID` (SQL SELECT on `refresh_tokens`, handle `sql.NullString`)
- [x] **A.3** `internal/oidc/token_svc.go` — add delegation method on `tokenService`
- [x] **A.4** `internal/oidc/adapter/storage.go` — replace `GetPrivateClaimsFromScopes` body: call `GetLatestDeviceSessionID`, return `{"dsid": ...}` or `{}`, log+swallow DB errors
- [x] **A.6** `storage_test.go` — update `GetPrivateClaimsFromScopes` test: inject mock returning a dsid, assert claim present
- [x] **A.6** `token_repo_test.go` — add `TestGetLatestDeviceSessionID`: found, not found, nullable column

### Phase B — new service setup

- [x] **B.0** `server/go.mod` — `go get github.com/redis/go-redis/v9`
- [x] **B.1** Create directory skeleton at `server/services/myaccount-bff/` (all dirs, empty `.gitkeep` or stub files)

### Phase B — foundation

- [x] **B.2** `config/config.go` — `Config` struct, `ConfigSource` / `OSEnvSource` / `MapSource`, `Load()`, all field validations (required strings, 64-hex SESSION_KEY, bounded int TTLs, CORS list)
- [x] **B.3** `internal/session/model.go` — `Session` struct with all seven fields and JSON tags
- [x] **B.4** `internal/session/store.go` — `Store` struct and `NewStore`; implement `Save`, `Load` (GETEX + hard-max eviction), `Delete`, `AcquireRefreshLock` (SET NX EX), `ReleaseRefreshLock`, `SaveState`, `LoadAndDeleteState` (GETDEL or Lua), sentinel `ErrNotFound`

### Phase B — external clients

- [x] **B.5** `internal/oidcrp/client.go` — `Client`, `New` (OIDC discovery via `gooidc.NewProvider`), `AuthCodeURL` (S256 PKCE: `GenerateVerifier` + `S256ChallengeOption`), `Exchange` (run token exchange + verify ID token), `EndSessionURL` (parse `end_session_endpoint` from raw provider claims), `RefreshToken` (use `oauth2.TokenSource`)
- [x] **B.5** `internal/oidcrp/client.go` — package-level `extractDSID` (base64url-decode JWT middle segment, unmarshal `dsid` field)
- [x] **B.6** `internal/accounts/client.go` — `Client`, `New` (insecure gRPC dial), `withBearer` context helper; all seven RPC wrappers: `GetMyProfile`, `UpdateMyProfile`, `ListLinkedProviders`, `UnlinkProvider`, `ListActiveSessions`, `RevokeSession`, `RevokeAllOtherSessions`
- [x] **B.6** `internal/handler/errors.go` — `grpcToHTTP` (gRPC code → HTTP status + error string), `writeError` (JSON `{error, message}` response)

### Phase B — middleware

- [x] **B.7** `internal/middleware/csrf.go` — `CSRF` middleware: reject POST/PATCH/DELETE/PUT missing `X-Requested-With: XMLHttpRequest`
- [x] **B.8** `internal/middleware/auth.go` — `Auth` middleware factory: read `__Host-sid` cookie, `store.Load` (slide TTL), lazy refresh gate (`<60s` remaining); `refreshTokens` helper: NX lock acquire, 500ms wait-and-reload if not acquired, double-check after lock, call `rp.RefreshToken`, update session fields, `store.Save`; `SessionFromContext`, `clearCookie`, `ErrRefreshFailed`

### Phase B — handlers

- [x] **B.9** `internal/handler/auth.go` — `AuthHandler` struct, `NewAuth`; `Login`: `AuthCodeURL`, `SaveState`, redirect 302; `Callback`: read params, `LoadAndDeleteState`, `Exchange`, `extractDSID`, build `Session`, generate ULID sid, `store.Save`, set `__Host-sid` cookie, redirect to `/`; `Logout`: delete session, clear cookie, return `{"redirect_to": end_session_url}`; `Me`: optional session load, return `{logged_in, user_id}`
- [x] **B.10** `internal/handler/profile.go` — `ProfileHandler`, `NewProfile`; `Get`: call `GetMyProfile`, map `pb.Profile` → JSON; `Update`: decode optional-field PATCH body, call `UpdateMyProfile`, return updated profile
- [x] **B.11** `internal/handler/providers.go` — `ProvidersHandler`, `NewProviders`; `List`: call `ListLinkedProviders`, return JSON array; `Unlink`: extract path param, call `UnlinkProvider`, 204, map `FailedPrecondition` → 409
- [x] **B.12** `internal/handler/sessions.go` — `SessionsHandler`, `NewSessions`; `List`: call `ListActiveSessions`, decorate each entry with `is_current` (compare against `sess.DeviceSessionID`); `Revoke`: call `RevokeSession`, 204, additionally delete BFF Redis session + clear cookie on self-revocation; `RevokeAllOthers`: guard empty `DeviceSessionID` → 409, call `RevokeAllOtherSessions`, 204

### Phase B — OpenAPI spec and codegen

- [x] **B.13** `api/openapi/myaccount/v1/myaccount.yaml` — write full spec: all 10 path operations, `Profile` / `Session` (with `is_current`) / `FederatedProvider` / `Error` / `MeResponse` / `LogoutResponse` schemas, `Error` and `Unauthorized` response components
- [x] **B.13** `Makefile` — add `generate-bff` target: `go tool oapi-codegen -generate types -package handler -o ... myaccount.yaml`
- [x] **B.13** Run codegen to produce `internal/handler/api_gen.go`; verify it compiles cleanly

### Phase B — wiring and deployment

- [x] **B.14** `main.go` — wire all components: parse Redis URL + ping, `session.NewStore`, `oidcrp.New`, `accounts.New`; build chi router with global middlewares (Recoverer, CORS, security headers), public routes (`/login`, `/callback`, `/me`), authenticated group (`authMW` + `csrfMW`) for all other routes, `/healthz` + `/readyz`; graceful shutdown on SIGINT/SIGTERM
- [x] **B.15** `Dockerfile` — multi-stage: `golang:1.26-alpine` builder (`go build -ldflags="-s -w"`), `gcr.io/distroless/static-debian12` runtime, non-root UID 10001, `EXPOSE 8080`
- [x] **B.16** `.env.example` — all env vars with inline comments and safe placeholder values

