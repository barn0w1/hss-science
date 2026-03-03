# Implementation Plan: Elevate accounts Service to Production-Grade

**Source:** `OIDC_EVALUATION.md` findings
**Base path:** `services/accounts/`
**Go module:** `github.com/barn0w1/hss-science/server`

---

## Scope Summary

Eleven targeted changes, grouped into four batches ordered by dependency and
severity. Each change is self-contained within its batch. No architectural
redesign is required.

| Batch | Changes | Severity |
|-------|---------|----------|
| 1 – Token Security Core | Refresh token entropy, hashing, rotation race fix, revocation client-check | HIGH + MEDIUM |
| 2 – Operational Resilience | Expired token cleanup (CLI subcommand), DB connection pool | MEDIUM |
| 3 – HTTP Security Layer | Rate limiting, security headers | HIGH + MEDIUM |
| 4 – Protocol Hardening | PKCE enforcement, GitHub email, AMR normalization | MEDIUM + LOW |

---

## Batch 1 – Token Security Core

### 1.1 – Refresh Token: High-Entropy Generation + SHA-256 Storage

**Why:** `RefreshToken.Token` is currently a ULID (80 bits/ms entropy), stored
verbatim. A DB breach exposes all active refresh tokens as usable bearer
credentials.

**Two-part fix:** (a) generate raw tokens from `crypto/rand` (256-bit entropy),
(b) store SHA-256(rawToken) — never the raw value.

---

#### 1.1.1 – Database Migration

**New file:** `migrations/3_refresh_token_hash.up.sql`

```sql
-- Rename the token column to token_hash.
-- All existing raw ULID values are replaced in-place with their SHA-256 hashes.
-- Existing client sessions remain valid: when clients present their ULID strings,
-- the application hashes them before lookup, matching the stored hash.
ALTER TABLE refresh_tokens RENAME COLUMN token TO token_hash;

UPDATE refresh_tokens
SET token_hash = encode(sha256(token_hash::bytea), 'hex');
```

**New file:** `migrations/3_refresh_token_hash.down.sql`

```sql
-- WARNING: Rolling back invalidates all active refresh token sessions.
-- Stored hashes cannot be reversed to raw values.
ALTER TABLE refresh_tokens RENAME COLUMN token_hash TO token;
```

---

#### 1.1.2 – New helper functions in `internal/oidc/token_svc.go`

Add two new private functions **alongside** (not replacing) `newID()`:

```go
import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/base64"
    "encoding/hex"
)

// newRefreshTokenValue generates a cryptographically random opaque refresh
// token value (256-bit entropy). The raw value is returned to the caller;
// the caller must hash it with hashRefreshToken before persisting.
func newRefreshTokenValue() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("generate refresh token: %w", err)
    }
    return base64.RawURLEncoding.EncodeToString(b), nil
}

// hashRefreshToken returns the hex-encoded SHA-256 hash of a raw refresh token
// value. Always call this before persisting or querying by refresh token value.
func hashRefreshToken(raw string) string {
    h := sha256.Sum256([]byte(raw))
    return hex.EncodeToString(h[:])
}
```

---

#### 1.1.3 – Update `tokenService.CreateAccessAndRefresh` in `internal/oidc/token_svc.go`

Current code generates `refreshTokenValue := newID()` and stores it verbatim.
Replace the relevant block:

```go
// NEW:
rawRefreshToken, err := newRefreshTokenValue()
if err != nil {
    return "", "", fmt.Errorf("generate refresh token: %w", err)
}
refresh := &RefreshToken{
    Token: hashRefreshToken(rawRefreshToken), // store hash, not raw value
    ...
}
// currentRefreshToken parameter contains the raw value from the client;
// hash it before passing to the repo so the repo always works with hashes.
currentRefreshTokenHash := ""
if currentRefreshToken != "" {
    currentRefreshTokenHash = hashRefreshToken(currentRefreshToken)
}
if err := s.repo.CreateAccessAndRefresh(ctx, access, refresh, currentRefreshTokenHash); err != nil {
    return "", "", err
}
return accessID, rawRefreshToken, nil // client receives raw value; repo stores hash
```

---

#### 1.1.4 – Update lookup methods in `internal/oidc/token_svc.go`

Every method that accepts a client-presented raw token must hash it before
calling the repo:

```go
func (s *tokenService) GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error) {
    return s.repo.GetRefreshToken(ctx, hashRefreshToken(token))
}

func (s *tokenService) GetRefreshInfo(ctx context.Context, token string) (userID, tokenID string, err error) {
    return s.repo.GetRefreshInfo(ctx, hashRefreshToken(token))
}

func (s *tokenService) RevokeRefreshToken(ctx context.Context, token string) error {
    return s.repo.RevokeRefreshToken(ctx, hashRefreshToken(token))
}
```

`CreateAccess`, `GetByID`, `DeleteByUserAndClient`, and `Revoke` are unchanged.

---

#### 1.1.5 – Update SQL in `internal/oidc/postgres/token_repo.go`

All queries that previously used `WHERE token = $1` must now use
`WHERE token_hash = $1`.

In `CreateAccessAndRefresh`, change the INSERT column name:

```sql
INSERT INTO refresh_tokens
  (id, token_hash, client_id, user_id, audience, scopes,
   auth_time, amr, access_token_id, expiration)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
```

In `GetRefreshToken`:
```sql
SELECT id, token_hash, client_id, user_id, audience, scopes,
       auth_time, amr, access_token_id, expiration, created_at
FROM refresh_tokens WHERE token_hash = $1 AND expiration > now()
```

In `GetRefreshInfo`:
```sql
SELECT user_id, id FROM refresh_tokens WHERE token_hash = $1 AND expiration > now()
```

In `RevokeRefreshToken`:
```sql
DELETE FROM refresh_tokens WHERE token_hash = $1
```

In `scanRefreshToken`: the second scan target was previously `&rt.Token`. It
remains `&rt.Token` because `domain.RefreshToken.Token` now holds the hash.

In the rotation-delete inside `CreateAccessAndRefresh`:
```sql
-- OLD: DELETE FROM refresh_tokens WHERE token = $1
-- NEW: DELETE FROM refresh_tokens WHERE token_hash = $1
```

---

#### 1.1.6 – Test updates

**`internal/oidc/token_svc_test.go`:** The mock repo's `CreateAccessAndRefresh`
now receives a hash for `currentRefreshToken`. Add assertions:

```go
// Verify the token passed to the repo is the hash of the raw value.
if repo.lastCurrentRefreshToken != hashRefreshToken("old-token") {
    t.Errorf("expected hashed currentRefreshToken, got %s", repo.lastCurrentRefreshToken)
}
// Verify the stored token is a hash, not the raw value.
if repo.lastRefresh.Token == refreshToken {
    t.Error("expected stored token to be a hash, not the raw value")
}
if repo.lastRefresh.Token != hashRefreshToken(refreshToken) {
    t.Error("stored token is not the expected SHA-256 hash")
}
```

**`internal/oidc/postgres/repo_test.go`** — `TestTokenRepository_CreateAccessAndRefresh`:
After rotation, verify the old token lookup fails using the raw value, and the
new token can be found using its raw value (which the test hashes before
querying directly). The test must exercise the full hash-based lookup path
end-to-end.

---

### 1.2 – Token Rotation Race Condition Fix

**Why:** Two concurrent token exchange requests with the same refresh token can
both succeed. The `DELETE FROM refresh_tokens WHERE token_hash = $1` does not
check whether a row was actually deleted.

**Fix:** Check `RowsAffected()` on the old refresh token deletion. Zero rows
means the token was already consumed — abort with `domerr.ErrNotFound`.

**File:** `internal/oidc/postgres/token_repo.go`

In `CreateAccessAndRefresh`, replace the refresh token deletion:

```go
// NEW:
result, err := tx.ExecContext(ctx,
    `DELETE FROM refresh_tokens WHERE token_hash = $1 AND expiration > now()`,
    currentRefreshToken,
)
if err != nil {
    return fmt.Errorf("delete old refresh token: %w", err)
}
n, err := result.RowsAffected()
if err != nil {
    return fmt.Errorf("rows affected: %w", err)
}
if n == 0 {
    // Token was already consumed or expired — reject to prevent double-use.
    return fmt.Errorf("refresh token already used or expired: %w", domerr.ErrNotFound)
}
```

The `AND expiration > now()` ensures that presenting an expired token (already
logically dead) also yields `RowsAffected=0` and a clean rejection rather than
a silent no-op.

The `domerr.ErrNotFound` propagates to `StorageAdapter.CreateAccessAndRefreshTokens`.
Update that method in `internal/oidc/adapter/storage.go` to distinguish this
case from a true internal error:

```go
accessID, refreshToken, err := s.tokens.CreateAccessAndRefresh(...)
if err != nil {
    if errors.Is(err, domerr.ErrNotFound) {
        // Refresh token was already consumed (race) or expired.
        return "", "", time.Time{}, op.ErrInvalidRefreshToken
    }
    return "", "", time.Time{}, internalErr("create access and refresh tokens", err)
}
```

**Test:** Add `TestTokenRepository_CreateAccessAndRefresh_DoubleRotation` in
`internal/oidc/postgres/repo_test.go`. Call `CreateAccessAndRefresh` twice with
the same `currentRefreshToken` value. Verify the second call returns an error
wrapping `domerr.ErrNotFound`.

---

### 1.3 – Client ID Enforcement in Token Revocation

**Why:** `RevokeToken` ignores its `clientID` parameter (`_`), allowing any
authenticated client to revoke another client's tokens by token ID.

**Scope of change:** `ports.go`, `token_svc.go`, `token_repo.go`,
`adapter/storage.go`.

---

#### 1.3.1 – Update interfaces in `internal/oidc/ports.go`

```go
// OLD:
Revoke(ctx context.Context, tokenID string) error
RevokeRefreshToken(ctx context.Context, token string) error

// NEW:
Revoke(ctx context.Context, tokenID, clientID string) error
RevokeRefreshToken(ctx context.Context, tokenHash, clientID string) error
```

---

#### 1.3.2 – Update `internal/oidc/postgres/token_repo.go`

```go
func (r *TokenRepository) Revoke(ctx context.Context, tokenID, clientID string) error {
    result, err := r.db.ExecContext(ctx,
        `DELETE FROM tokens WHERE id = $1 AND client_id = $2`, tokenID, clientID,
    )
    if err != nil {
        return err
    }
    n, _ := result.RowsAffected()
    if n == 0 {
        return fmt.Errorf("token %s: %w", tokenID, domerr.ErrNotFound)
    }
    return nil
}

func (r *TokenRepository) RevokeRefreshToken(ctx context.Context, tokenHash, clientID string) error {
    result, err := r.db.ExecContext(ctx,
        `DELETE FROM refresh_tokens WHERE token_hash = $1 AND client_id = $2`,
        tokenHash, clientID,
    )
    if err != nil {
        return err
    }
    n, _ := result.RowsAffected()
    if n == 0 {
        return fmt.Errorf("refresh token: %w", domerr.ErrNotFound)
    }
    return nil
}
```

---

#### 1.3.3 – Update `internal/oidc/token_svc.go`

```go
func (s *tokenService) Revoke(ctx context.Context, tokenID, clientID string) error {
    return s.repo.Revoke(ctx, tokenID, clientID)
}

func (s *tokenService) RevokeRefreshToken(ctx context.Context, token, clientID string) error {
    return s.repo.RevokeRefreshToken(ctx, hashRefreshToken(token), clientID)
}
```

---

#### 1.3.4 – Update `internal/oidc/adapter/storage.go`

The `RevokeToken` method currently discards `clientID` with `_`. Replace:

```go
func (s *StorageAdapter) RevokeToken(ctx context.Context, tokenOrTokenID, userID, clientID string) *oidc.Error {
    if userID != "" {
        if err := s.tokens.Revoke(ctx, tokenOrTokenID, clientID); err != nil {
            if errors.Is(err, domerr.ErrNotFound) {
                return oidc.ErrInvalidRequest().WithDescription("token not found")
            }
            return oidc.ErrServerError().WithParent(err)
        }
        return nil
    }
    if err := s.tokens.RevokeRefreshToken(ctx, tokenOrTokenID, clientID); err != nil {
        if errors.Is(err, domerr.ErrNotFound) {
            return oidc.ErrInvalidRequest().WithDescription("token not found")
        }
        return oidc.ErrServerError().WithParent(err)
    }
    return nil
}
```

---

#### 1.3.5 – Update mock in `internal/oidc/token_svc_test.go`

Update `mockTokenRepo` signatures to match the new interface:

```go
func (m *mockTokenRepo) Revoke(_ context.Context, _, _ string) error            { return m.err }
func (m *mockTokenRepo) RevokeRefreshToken(_ context.Context, _, _ string) error { return m.err }
```

---

## Batch 2 – Operational Resilience

### 2.1 – Expired Token Cleanup (CLI Subcommand Architecture)

**Why:** The `tokens` and `refresh_tokens` tables have no cleanup mechanism.
With 15-minute access token lifetimes, tables grow without bound, degrading
query performance over time.

**Design:** Implement a CLI subcommand dispatched from `main()` using `os.Args`.
The `cleanup` subcommand runs once and exits, making it directly suitable for
deployment as a Kubernetes CronJob. There are no background tickers in the
server process.

This also means the **existing** `runAuthRequestCleanup` background goroutine
in `main.go` is removed from the server path and absorbed into the `cleanup`
subcommand. The server process runs no periodic background tasks.

---

#### 2.1.1 – Database Migration

**New file:** `migrations/4_token_expiry_indexes.up.sql`

```sql
-- Support efficient expired-token deletion scans.
CREATE INDEX tokens_expiration_idx ON tokens (expiration);
CREATE INDEX refresh_tokens_expiration_idx ON refresh_tokens (expiration);
```

**New file:** `migrations/4_token_expiry_indexes.down.sql`

```sql
DROP INDEX IF EXISTS refresh_tokens_expiration_idx;
DROP INDEX IF EXISTS tokens_expiration_idx;
```

---

#### 2.1.2 – Update interfaces in `internal/oidc/ports.go`

Add to both `TokenRepository` and `TokenService`:

```go
// DeleteExpired removes access tokens and refresh tokens whose expiration is
// before the given cutoff. Returns (accessDeleted, refreshDeleted, error).
DeleteExpired(ctx context.Context, before time.Time) (int64, int64, error)
```

---

#### 2.1.3 – Implement in `internal/oidc/postgres/token_repo.go`

```go
func (r *TokenRepository) DeleteExpired(ctx context.Context, before time.Time) (int64, int64, error) {
    res1, err := r.db.ExecContext(ctx,
        `DELETE FROM tokens WHERE expiration < $1`, before,
    )
    if err != nil {
        return 0, 0, fmt.Errorf("delete expired access tokens: %w", err)
    }
    accessDeleted, _ := res1.RowsAffected()

    res2, err := r.db.ExecContext(ctx,
        `DELETE FROM refresh_tokens WHERE expiration < $1`, before,
    )
    if err != nil {
        return accessDeleted, 0, fmt.Errorf("delete expired refresh tokens: %w", err)
    }
    refreshDeleted, _ := res2.RowsAffected()

    return accessDeleted, refreshDeleted, nil
}
```

---

#### 2.1.4 – Implement in `internal/oidc/token_svc.go`

```go
func (s *tokenService) DeleteExpired(ctx context.Context, before time.Time) (int64, int64, error) {
    return s.repo.DeleteExpired(ctx, before)
}
```

---

#### 2.1.5 – Refactor `main.go` to a CLI subcommand architecture

`main()` becomes a thin dispatcher. All logic moves into two top-level
functions: `runServer` and `runCleanup`. Both share a common `buildServices`
helper that sets up DB and services, keeping the code DRY.

**Dispatch logic in `main()`:**

```go
func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(logger)

    cfg, err := config.Load()
    if err != nil {
        logger.Error("failed to load config", "error", err)
        os.Exit(1)
    }

    cmd := "server" // default
    if len(os.Args) > 1 {
        cmd = os.Args[1]
    }

    switch cmd {
    case "server":
        runServer(cfg, logger)
    case "cleanup":
        runCleanup(cfg, logger)
    default:
        fmt.Fprintf(os.Stderr, "unknown command %q\nusage: accounts [server|cleanup]\n", cmd)
        os.Exit(1)
    }
}
```

**`runServer` function:**

Extract the current `main()` body into `runServer`. Key change: **remove** the
`runAuthRequestCleanup` goroutine entirely. The server process has no background
cleanup tickers.

```go
func runServer(cfg *config.Config, logger *slog.Logger) {
    db := mustConnectDB(cfg, logger)
    defer func() { _ = db.Close() }()

    // Apply connection pool settings (see 2.2).
    db.SetMaxOpenConns(cfg.DBMaxOpenConns)
    db.SetMaxIdleConns(cfg.DBMaxIdleConns)
    db.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifetimeSecs) * time.Second)
    db.SetConnMaxIdleTime(time.Duration(cfg.DBConnMaxIdleTimeSecs) * time.Second)

    authReqSvc, clientSvc, tokenSvc, identitySvc := buildServices(db, cfg)

    // ... build provider, upstream providers, login handler ...
    // ... build router with rate limiter and security headers (see Batch 3) ...
    // ... start HTTP server with graceful shutdown ...
    // NOTE: no runAuthRequestCleanup goroutine — cleanup is handled by the
    //       `cleanup` subcommand run as a CronJob.
}
```

**`runCleanup` function:**

One-shot: connects to DB, runs both cleanup operations, logs results, exits.
No HTTP server. No signal handling.

```go
func runCleanup(cfg *config.Config, logger *slog.Logger) {
    db := mustConnectDB(cfg, logger)
    defer func() { _ = db.Close() }()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    authReqSvc, _, tokenSvc, _ := buildServices(db, cfg)

    // Clean up expired auth requests.
    authCutoff := time.Now().UTC().Add(-time.Duration(cfg.AuthRequestTTLMinutes) * time.Minute)
    authDeleted, err := authReqSvc.DeleteExpiredBefore(ctx, authCutoff)
    if err != nil {
        logger.Error("auth request cleanup failed", "error", err)
        os.Exit(1)
    }
    logger.Info("cleaned up expired auth requests", "count", authDeleted)

    // Clean up expired tokens.
    accessDeleted, refreshDeleted, err := tokenSvc.DeleteExpired(ctx, time.Now().UTC())
    if err != nil {
        logger.Error("token cleanup failed", "error", err)
        os.Exit(1)
    }
    logger.Info("cleaned up expired tokens",
        "access_tokens", accessDeleted,
        "refresh_tokens", refreshDeleted,
    )
}
```

**`mustConnectDB` helper:**

```go
func mustConnectDB(cfg *config.Config, logger *slog.Logger) *sqlx.DB {
    db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
    if err != nil {
        logger.Error("failed to connect to database", "error", err)
        os.Exit(1)
    }
    return db
}
```

**`buildServices` helper:**

```go
func buildServices(db *sqlx.DB, cfg *config.Config) (
    oidcdom.AuthRequestService,
    oidcdom.ClientService,
    oidcdom.TokenService,
    identity.Service,
) {
    authReqSvc := oidcdom.NewAuthRequestService(
        oidcpg.NewAuthRequestRepository(db),
        time.Duration(cfg.AuthRequestTTLMinutes)*time.Minute,
    )
    clientSvc := oidcdom.NewClientService(oidcpg.NewClientRepository(db))
    tokenSvc  := oidcdom.NewTokenService(oidcpg.NewTokenRepository(db))
    identitySvc := identity.NewService(identitypg.NewUserRepository(db))
    return authReqSvc, clientSvc, tokenSvc, identitySvc
}
```

**Kubernetes CronJob example** (for reference in operator documentation):

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: accounts-cleanup
spec:
  schedule: "0 * * * *"   # hourly
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: cleanup
              image: accounts:latest
              args: ["cleanup"]
              envFrom:
                - secretRef:
                    name: accounts-env
          restartPolicy: OnFailure
```

---

### 2.2 – Database Connection Pool Configuration

**Why:** `sqlx.Connect` leaves `MaxOpenConns=0` (unlimited), risking PostgreSQL
connection exhaustion under load.

---

#### 2.2.1 – Update `config/config.go`

Add to `Config`:

```go
DBMaxOpenConns        int
DBMaxIdleConns        int
DBConnMaxLifetimeSecs int
DBConnMaxIdleTimeSecs int
```

In `LoadFrom`, use `loadBoundedInt` with sensible defaults:

```go
cfg.DBMaxOpenConns, err = loadBoundedInt(src, "DB_MAX_OPEN_CONNS", 25, 1, 500)
if err != nil { return nil, err }
cfg.DBMaxIdleConns, err = loadBoundedInt(src, "DB_MAX_IDLE_CONNS", 10, 1, 200)
if err != nil { return nil, err }
cfg.DBConnMaxLifetimeSecs, err = loadBoundedInt(src, "DB_CONN_MAX_LIFETIME_SECONDS", 300, 10, 3600)
if err != nil { return nil, err }
cfg.DBConnMaxIdleTimeSecs, err = loadBoundedInt(src, "DB_CONN_MAX_IDLE_TIME_SECONDS", 180, 10, 1800)
if err != nil { return nil, err }
```

---

#### 2.2.2 – Apply in `runServer` in `main.go`

After `mustConnectDB`, before building services:

```go
db.SetMaxOpenConns(cfg.DBMaxOpenConns)
db.SetMaxIdleConns(cfg.DBMaxIdleConns)
db.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifetimeSecs) * time.Second)
db.SetConnMaxIdleTime(time.Duration(cfg.DBConnMaxIdleTimeSecs) * time.Second)
```

The `cleanup` subcommand does not need pool tuning (it is short-lived and runs
only a few queries), so pool settings are applied in `runServer` only.

---

#### 2.2.3 – Update `.env.example`

```
# Database connection pool (optional; defaults shown)
# DB_MAX_OPEN_CONNS=25
# DB_MAX_IDLE_CONNS=10
# DB_CONN_MAX_LIFETIME_SECONDS=300
# DB_CONN_MAX_IDLE_TIME_SECONDS=180
```

---

## Batch 3 – HTTP Security Layer

### 3.1 – Per-IP Rate Limiting

**Why:** No rate limiting exists on any endpoint. The token endpoint, client
credentials endpoint, and login flow are all unbounded.

**Approach:** Per-IP token bucket using `golang.org/x/time/rate` applied only
to the public login/authorize flow. Internal BFF calls bypass the limiter
automatically based on the TCP source address (RFC-1918 / loopback). The
real client IP is read from the `CF-Connecting-IP` header set by Cloudflare
Tunnel, not from `X-Forwarded-For`.

---

#### 3.1.1 – Add dependency

```
go get golang.org/x/time/rate
```

Verify `golang.org/x/time` appears as a direct dependency in `go.mod`.

---

#### 3.1.2 – New file: `internal/middleware/ratelimit.go`

```go
package middleware

import (
    "net"
    "net/http"
    "strings"
    "sync"
    "time"

    "golang.org/x/time/rate"
)

// IPRateLimiter implements per-IP token-bucket rate limiting with automatic
// eviction of inactive IP entries to prevent unbounded memory growth.
// Internal requests (from private RFC-1918 / loopback addresses) are always
// bypassed — they originate from Caddy or internal BFF services.
type IPRateLimiter struct {
    mu      sync.Mutex
    entries map[string]*ipEntry
    limit   rate.Limit
    burst   int
}

type ipEntry struct {
    limiter  *rate.Limiter
    lastSeen time.Time
}

// NewIPRateLimiter creates a limiter: `rps` sustained requests per second,
// `burst` maximum burst per IP.
func NewIPRateLimiter(rps float64, burst int) *IPRateLimiter {
    return &IPRateLimiter{
        entries: make(map[string]*ipEntry),
        limit:   rate.Limit(rps),
        burst:   burst,
    }
}

func (l *IPRateLimiter) allow(r *http.Request) bool {
    ip := clientIP(r)
    l.mu.Lock()
    e, ok := l.entries[ip]
    if !ok {
        e = &ipEntry{limiter: rate.NewLimiter(l.limit, l.burst)}
        l.entries[ip] = e
    }
    e.lastSeen = time.Now()
    limiter := e.limiter
    l.mu.Unlock()
    return limiter.Allow()
}

// Cleanup removes IP entries idle for longer than ttl. Call from a background
// goroutine at regular intervals.
func (l *IPRateLimiter) Cleanup(ttl time.Duration) {
    l.mu.Lock()
    defer l.mu.Unlock()
    threshold := time.Now().Add(-ttl)
    for ip, e := range l.entries {
        if e.lastSeen.Before(threshold) {
            delete(l.entries, ip)
        }
    }
}

// Middleware returns an http.Handler middleware that enforces the rate limit.
// Requests whose direct TCP connection originates from a private (RFC-1918)
// or loopback address are unconditionally allowed — these are internal
// BFF/service calls (arriving via Caddy on the private network) that must
// never be throttled regardless of the endpoint being accessed.
func (l *IPRateLimiter) Middleware() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if isInternalRequest(r) {
                next.ServeHTTP(w, r)
                return
            }
            if !l.allow(r) {
                http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}

// clientIP returns the originating end-user IP address.
// Traffic arrives via Cloudflare Tunnel → Caddy → App. Cloudflare sets the
// CF-Connecting-IP header to the real end-user IP before the request enters
// the tunnel; this header is the authoritative source for public traffic.
// Falls back to the direct connection address if the header is absent (e.g.
// in local development without Cloudflare).
func clientIP(r *http.Request) string {
    if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
        return cf
    }
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        return r.RemoteAddr
    }
    return host
}

// isInternalRequest returns true when the TCP connection's direct source
// address is loopback or a private RFC-1918 range. Caddy runs on the same
// host or internal network and forwards requests from such addresses.
// Because the bypass is based on the physical TCP source (r.RemoteAddr),
// not on a spoofable header, it cannot be forged by an external caller.
func isInternalRequest(r *http.Request) bool {
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        host = r.RemoteAddr
    }
    ip := net.ParseIP(host)
    if ip == nil {
        return false
    }
    if ip.IsLoopback() {
        return true
    }
    for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"} {
        _, network, _ := net.ParseCIDR(cidr)
        if network.Contains(ip) {
            return true
        }
    }
    return false
}
```

---

#### 3.1.3 – Update `config/config.go`

Add to `Config`:

```go
RateLimitEnabled   bool
RateLimitPublicRPM int // /authorize and /login/* (public-facing only), per IP
                       // Internal endpoints (/token, /userinfo, /introspect, etc.)
                       // are never rate-limited — they are bypassed at the middleware
                       // level based on the TCP source address (RFC-1918 / loopback).
```

In `LoadFrom`:

```go
cfg.RateLimitEnabled = src.Get("RATE_LIMIT_ENABLED") != "false" // default on
cfg.RateLimitPublicRPM, err = loadBoundedInt(src, "RATE_LIMIT_PUBLIC_RPM", 20, 1, 600)
if err != nil { return nil, err }
```

---

#### 3.1.4 – Apply in `runServer` in `main.go`

Instantiate a single rate limiter for the public-facing login/authorize flow.
Token, userinfo, introspect, and all other OIDC endpoints served by the
provider mount are **not** wrapped — they are exclusively called by the
internal BFF and must never be throttled. The `isInternalRequest()` check
inside `Middleware()` provides a defense-in-depth bypass for any request
arriving from a private source address regardless of path.

```go
var publicLimiter *appmiddleware.IPRateLimiter
if cfg.RateLimitEnabled {
    publicLimiter = appmiddleware.NewIPRateLimiter(
        float64(cfg.RateLimitPublicRPM)/60.0, 5)

    // Evict stale IP state every 10 minutes to bound memory usage.
    go func() {
        t := time.NewTicker(10 * time.Minute)
        defer t.Stop()
        for range t.C {
            publicLimiter.Cleanup(15 * time.Minute)
        }
    }()
}
```

Apply per-route limiters in the route definitions:

```go
router.Route("/login", func(r chi.Router) {
    if cfg.RateLimitEnabled {
        r.Use(publicLimiter.Middleware())
    }
    r.Use(interceptor.Handler)
    r.Get("/", loginHandler.SelectProvider)
    r.Post("/select", loginHandler.FederatedRedirect)
    r.Get("/callback", loginHandler.FederatedCallback)
})

// /authorize begins the login flow and must be rate-limited for public traffic.
// All other provider paths (/token, /userinfo, /introspect, /.well-known/*,
// /oauth/v2/*, etc.) are BFF-accessed internal endpoints — they are mounted
// without any rate limiting middleware.
if cfg.RateLimitEnabled {
    router.With(authorizePathLimiter(publicLimiter)).Mount("/", provider)
} else {
    router.Mount("/", provider)
}
```

Add the path-matching helper in `main.go`:

```go
// authorizePathLimiter applies the given limiter only to the /authorize
// endpoint path. Every other provider path passes through unrestricted.
func authorizePathLimiter(limiter *appmiddleware.IPRateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        lm := limiter.Middleware()(next)
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            if r.URL.Path == "/authorize" {
                lm.ServeHTTP(w, r)
            } else {
                next.ServeHTTP(w, r)
            }
        })
    }
}
```

---

#### 3.1.5 – Update `.env.example`

```
# Rate limiting — applies only to public user-facing endpoints (/authorize, /login/*)
# Internal/BFF endpoints (/token, /userinfo, /introspect, etc.) are never rate-limited;
# requests from private IP ranges (RFC-1918 / loopback) bypass the limiter automatically.
# Set RATE_LIMIT_ENABLED=false to disable entirely (e.g. local dev without Cloudflare).
# RATE_LIMIT_ENABLED=true
# RATE_LIMIT_PUBLIC_RPM=20
```

---

### 3.2 – Security Response Headers

**Why:** All responses lack standard defensive headers. The login HTML page is
exposed without CSP or framing protection.

---

#### 3.2.1 – New file: `internal/middleware/securityheaders.go`

```go
package middleware

import "net/http"

// SecurityHeaders adds standard security response headers to all responses.
// The CSP is restrictive since the current login page has no external scripts
// or styles. Expand it if a full frontend is introduced.
func SecurityHeaders() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            h := w.Header()
            h.Set("X-Content-Type-Options", "nosniff")
            h.Set("X-Frame-Options", "DENY")
            h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
            h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
            h.Set("Content-Security-Policy",
                "default-src 'none'; form-action 'self'; frame-ancestors 'none'")
            h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
            next.ServeHTTP(w, r)
        })
    }
}
```

**Note on CSP:** `form-action 'self'` allows the login page `<form>` to POST to
`/login/select`. `default-src 'none'` is safe because the page has no external
resources. If a favicon or stylesheet is added later, extend `img-src` and
`style-src` accordingly.

---

#### 3.2.2 – Apply in `runServer` in `main.go`

```go
router.Use(middleware.Recoverer)
router.Use(appmiddleware.SecurityHeaders()) // global — applies to all routes
```

---

## Batch 4 – Protocol Hardening

### 4.1 – PKCE Enforcement for Public Clients

**Why:** Authorization code flow can be completed without PKCE. Per OAuth 2.1
and RFC 9700, PKCE is required for public clients (`auth_method = 'none'`).

**Approach:** Enforce in `StorageAdapter.CreateAuthRequest`, early in the flow,
before the user sees the login page.

**File:** `internal/oidc/adapter/storage.go`

In `CreateAuthRequest`, after building `ar` and before calling
`s.authReqs.Create`, add:

```go
// Enforce PKCE for public clients (auth_method = 'none').
// Confidential clients using client_secret_basic / client_secret_post are
// exempt, consistent with RFC 9700 and OAuth 2.1 §7.6.
if client, err := s.clients.GetByID(ctx, ar.ClientID); err == nil {
    if client.AuthMethod == "none" && ar.CodeChallenge == "" {
        return nil, oidc.ErrInvalidRequest().
            WithDescription("code_challenge is required for public clients (S256)")
    }
}
// Note: if GetByID fails here, we proceed. The library performs its own full
// client validation; this check fires only when the client can be resolved.
```

**Downstream impact:** The seeded client in `migrations/2_seed_clients.up.sql`
uses `auth_method = 'client_secret_basic'`, so it is unaffected. This change
only impacts future `auth_method = 'none'` clients.

**Test:** Add a unit test in `internal/oidc/adapter/storage_test.go` that
verifies `CreateAuthRequest` returns `ErrInvalidRequest` when the mock client
returns `auth_method = 'none'` and `code_challenge` is empty.

---

### 4.2 – GitHub Email Completeness

**Why:** GitHub's `/user` endpoint returns a null email when the user has no
public email set, resulting in an empty email in the identity store.

**File:** `internal/authn/provider_github.go`

In `FetchClaims`, after decoding `ghUser`, add a fallback call when email is
absent. The `user:email` scope is already requested:

```go
email := ghUser.Email
emailVerified := false
if email == "" {
    // Fall back to the private emails endpoint, which requires user:email scope.
    email, emailVerified = g.fetchPrimaryEmail(ctx, token)
}

return &identity.FederatedClaims{
    Subject:       strconv.FormatInt(ghUser.ID, 10),
    Email:         email,
    EmailVerified: emailVerified,
    Name:          ghUser.Name,
    Picture:       ghUser.AvatarURL,
}, nil
```

Add helper method on `githubClaimsProvider`:

```go
// fetchPrimaryEmail queries the /user/emails endpoint and returns the primary
// verified email address. Returns ("", false) if none is found or the call
// fails — the caller treats a missing email as a non-fatal condition.
func (g *githubClaimsProvider) fetchPrimaryEmail(ctx context.Context, token *oauth2.Token) (string, bool) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet,
        "https://api.github.com/user/emails", nil)
    if err != nil {
        return "", false
    }
    token.SetAuthHeader(req)
    resp, err := g.httpClient.Do(req) //nolint:gosec // URL is a hardcoded constant
    if err != nil {
        return "", false
    }
    defer func() { _ = resp.Body.Close() }()
    if resp.StatusCode != http.StatusOK {
        return "", false
    }
    var emails []struct {
        Email    string `json:"email"`
        Primary  bool   `json:"primary"`
        Verified bool   `json:"verified"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
        return "", false
    }
    for _, e := range emails {
        if e.Primary && e.Verified {
            return e.Email, true
        }
    }
    return "", false
}
```

---

### 4.3 – AMR Value Normalization

**Why:** `federated:google` / `federated:github` are non-standard AMR values.

**File:** `internal/authn/login_usecase.go`

```go
// OLD:
amr := []string{"federated:" + provider}

// NEW:
// "fed" signals federation-based authentication without encoding
// provider-specific information in the AMR string.
// The upstream provider is already recorded in federated_identities.provider.
amr := []string{"fed"}
```

---

## Migration Sequence

```
migrations/3_refresh_token_hash.up.sql    ← Batch 1
migrations/3_refresh_token_hash.down.sql
migrations/4_token_expiry_indexes.up.sql  ← Batch 2
migrations/4_token_expiry_indexes.down.sql
```

---

## Files Changed

| File | Change |
|------|--------|
| `migrations/3_refresh_token_hash.up.sql` | **NEW** — rename `token` → `token_hash`, backfill SHA-256 |
| `migrations/3_refresh_token_hash.down.sql` | **NEW** |
| `migrations/4_token_expiry_indexes.up.sql` | **NEW** — indexes on `expiration` columns |
| `migrations/4_token_expiry_indexes.down.sql` | **NEW** |
| `internal/oidc/ports.go` | Add `DeleteExpired`; update `Revoke`/`RevokeRefreshToken` signatures |
| `internal/oidc/token_svc.go` | Add `newRefreshTokenValue`, `hashRefreshToken`; update 5 methods; add `DeleteExpired` |
| `internal/oidc/token_svc_test.go` | Update mock signatures; add hash assertions |
| `internal/oidc/postgres/token_repo.go` | Column rename in SQL, `RowsAffected` checks, `DeleteExpired` |
| `internal/oidc/postgres/repo_test.go` | Add double-rotation test |
| `internal/oidc/adapter/storage.go` | Fix `RevokeToken` (pass clientID); fix `CreateAccessAndRefreshTokens` (ErrNotFound path); add PKCE check in `CreateAuthRequest` |
| `internal/oidc/adapter/storage_test.go` | Add PKCE enforcement test |
| `internal/middleware/ratelimit.go` | **NEW** |
| `internal/middleware/securityheaders.go` | **NEW** |
| `config/config.go` | Add DB pool + rate limit config fields |
| `config/config_test.go` | Add tests for new config fields |
| `main.go` | Refactor to `server`/`cleanup` subcommand dispatch; extract `runServer`, `runCleanup`, `buildServices`, `mustConnectDB`; remove `runAuthRequestCleanup` goroutine; apply DB pool, single public rate limiter (`authorizePathLimiter` + `/login/*`), security headers |
| `internal/authn/provider_github.go` | Add `fetchPrimaryEmail` fallback |
| `internal/authn/login_usecase.go` | Change AMR to `["fed"]` |
| `.env.example` | Document new environment variables |

---

## Verification

### Unit and integration tests

```bash
cd services/accounts

# Unit tests
go test ./internal/oidc/...
go test ./internal/oidc/adapter/...
go test ./internal/authn/...
go test ./config/...
go test ./internal/middleware/...

# Integration tests (Docker required for testcontainers)
go test ./internal/oidc/postgres/...
go test ./internal/identity/postgres/...
```

**Key assertions to cover:**

- Refresh token created, fetched by raw value → found (hash lookup transparent to caller)
- Old raw token after rotation → fetch returns `domerr.ErrNotFound`
- `CreateAccessAndRefresh` called twice with same `currentRefreshToken` → second call returns `domerr.ErrNotFound`
- `Revoke` with wrong `clientID` → returns `domerr.ErrNotFound`, token untouched in DB
- `DeleteExpired` removes rows with `expiration < now()`, leaves future-dated rows intact
- Public client (auth_method = 'none') without `code_challenge` → `CreateAuthRequest` returns `ErrInvalidRequest`

### Manual smoke tests

1. Apply migrations 3 and 4 against the development database.
2. Start server: `./accounts server` → confirm `/.well-known/openid-configuration` responds.
3. Complete a full login flow → confirm ID token, access token, and refresh token are issued.
4. Use refresh token → confirm new access token issued, old refresh token rejected on re-use.
5. Hit `/login` and `/authorize` rapidly with a spoofed `CF-Connecting-IP: 1.2.3.4` header → confirm `429 Too Many Requests` after burst is exhausted. Then repeat the same requests from a private IP (e.g. `curl --interface 127.0.0.1`) → confirm all pass without 429 (internal bypass active). Also confirm that rapid calls to `/token` never trigger 429 regardless of source IP.
6. Inspect any response header → confirm `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, etc.
7. Run cleanup command: `./accounts cleanup` → confirm clean exit with logged deletion counts.
8. Run `EXPLAIN (ANALYZE, BUFFERS) DELETE FROM tokens WHERE expiration < now()` → confirm index scan on `tokens_expiration_idx`.

### Static analysis

```bash
go vet ./...
golangci-lint run
```
