# Implementation Plan: Device Session Management + Production Hardening

> Written: 2026-03-16
> Based on: research.md + full codebase read
> Scope: Device/session layer, security hardening (CSP, RevokeToken fix)
> Out of scope: myaccount BFF HTTP API, migration runner, client secret replacement, P4 DX items

---

## 0. Goals and Design Decisions

### 0.1 The "Google Devices" Model

The goal mirrors `google.com/devices`: a **device session** represents a persistent browser/device identity. Multiple BFF clients (`chat-bff`, `drive-bff`, `myaccount-bff`) running on the same browser are all grouped under one device session. The OIDC security boundary remains per-client (each BFF has its own refresh token), but the device dimension is layered on top.

```
Device Session (accounts.hss-science.org cookie: "dsid")
  └── refresh_token (myaccount-bff, scopes: openid email)
  └── refresh_token (chat-bff, scopes: openid)
  └── refresh_token (drive-bff, scopes: openid)
```

Revoking the device session immediately invalidates all associated refresh tokens. Existing access tokens remain valid until expiry (they are short-lived, defaulting to 15 minutes).

### 0.2 Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Cookie value | ULID (the device session ID itself) | Simplest; no separate secret needed since the cookie is HttpOnly+Secure+accounts-domain-scoped |
| Cookie issuance point | `SelectProvider` (`GET /login`) | First HTTP response on the accounts domain in the flow |
| DB row creation point | `FederatedCallback` | Only point where `user_id` is known |
| device_session_id propagation | `auth_requests.device_session_id` column → `RefreshToken.DeviceSessionID` | Follows the natural data flow without context injection |
| Token refresh inheritance | Repository carries forward from old refresh token | Service layer stays clean; repo already opens a transaction for rotation |
| `last_used_at` update | Inside `CreateAccessAndRefresh` transaction | Atomic with token rotation; one round-trip fewer |
| Revoke device | Set `revoked_at` + DELETE all `refresh_tokens` WHERE device_session_id | Immediate enforcement; clean separation of soft-delete for BFF audit vs hard-delete of tokens |
| User-agent parsing | Minimal in-process heuristic, no new dependency | Avoids dependency on external UA database; "Chrome on macOS" level is sufficient |
| IP extraction | `CF-Connecting-IP` header (already implemented in `middleware.clientIP`) | Traffic always flows Cloudflare → reverse proxy → accounts |
| SameSite cookie policy | `Lax` | The `GET /login` request is a cross-origin redirect (RP → accounts), so `Strict` would drop the cookie on the initial page load |

### 0.3 Scope of Changes

Changes required:
1. **New SQL migration** (`3_device_sessions.up.sql`) — new table + FK columns
2. **New domain types** (`internal/oidc/domain.go`) — `DeviceSession`, update `RefreshToken`, `AuthRequest`
3. **New ports** (`internal/oidc/ports.go`) — `DeviceSessionRepository`, `DeviceSessionService`
4. **New repository** (`internal/oidc/postgres/device_session_repo.go`)
5. **New service** (`internal/oidc/device_session_svc.go`)
6. **`authn` package** — cookie issuance, device session creation, use-case update
7. **OIDC ports update** — `AuthRequestRepository.CompleteLogin` gains `deviceSessionID` param; `TokenRepository.CreateAccessAndRefresh` carries it forward
8. **OIDC adapter** — propagate `device_session_id` in `CreateAccessAndRefreshTokens`; fix `RevokeToken`
9. **Security middleware** — add CSP for login paths
10. **`main.go`** — wire new device session repo/service

No changes needed to:
- `config/` (no new env vars required for the cookie; it's always Secure+HttpOnly+Lax)
- `internal/identity/` package
- `internal/pkg/` packages
- zitadel adapter interface files (keys.go, client.go, provider.go, userinfo.go)
- Existing test helpers (testdb.go gains the new table in CleanTables)

---

## 1. Database Migration

### File: `services/accounts/migrations/3_device_sessions.up.sql` (new)

```sql
-- Device sessions table: one row per browser/device identity
-- The row ID is also the value stored in the 'dsid' HttpOnly cookie.
CREATE TABLE device_sessions (
    id            TEXT        PRIMARY KEY,           -- ULID; also cookie value
    user_id       TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    user_agent    TEXT        NOT NULL DEFAULT '',
    ip_address    TEXT        NOT NULL DEFAULT '',
    device_name   TEXT        NOT NULL DEFAULT '',   -- parsed from user_agent at creation
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at    TIMESTAMPTZ                        -- NULL = active
);

CREATE INDEX device_sessions_user_id_idx  ON device_sessions (user_id);
CREATE INDEX device_sessions_revoked_idx  ON device_sessions (user_id) WHERE revoked_at IS NULL;

-- Link refresh tokens to their device session (nullable: pre-existing tokens have no device session)
ALTER TABLE refresh_tokens
    ADD COLUMN device_session_id TEXT REFERENCES device_sessions(id) ON DELETE SET NULL;

CREATE INDEX refresh_tokens_device_session_idx ON refresh_tokens (device_session_id)
    WHERE device_session_id IS NOT NULL;

-- Link auth requests to the device session cookie seen during the login flow
ALTER TABLE auth_requests
    ADD COLUMN device_session_id TEXT REFERENCES device_sessions(id) ON DELETE SET NULL;
```

**Rationale for `ON DELETE SET NULL` (not CASCADE) on `refresh_tokens`:**
Device session revocation is a logical operation: set `revoked_at` and DELETE the linked refresh tokens explicitly. We never hard-delete device session rows during normal operation — only during cleanup of old revoked sessions. At that point, the linked refresh tokens are already expired/deleted, so `SET NULL` is safe and prevents accidental token orphaning.

**Rationale for nullable `device_session_id` on `refresh_tokens`:**
Existing tokens in production will have no device session. Adding a NOT NULL constraint would break any rolling deployment without a backfill. NULL means "legacy token created before device sessions were introduced."

### File: `services/accounts/migrations/3_device_sessions.down.sql` (new)

```sql
ALTER TABLE auth_requests DROP COLUMN IF EXISTS device_session_id;
ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS device_session_id;
DROP TABLE IF EXISTS device_sessions;
```

### Update `testhelper/testdb.go`

`CleanTables` must delete `device_sessions` before `users` (FK dependency). Also delete in the right order: device_sessions goes after refresh_tokens (because refresh_tokens references device_sessions).

New cleanup order:
```
refresh_tokens → tokens → auth_requests → device_sessions → federated_identities → users → clients
```

Specifically, add `device_sessions` deletion between `auth_requests` and `federated_identities` in the existing `DELETE FROM ...` sequence.

---

## 2. Domain Layer Changes

### File: `internal/oidc/domain.go` — additions and modifications

**New type:**
```go
type DeviceSession struct {
    ID          string
    UserID      string
    UserAgent   string
    IPAddress   string
    DeviceName  string
    CreatedAt   time.Time
    LastUsedAt  time.Time
    RevokedAt   *time.Time // nil = active
}
```

**Modify `AuthRequest`** — add one field:
```go
type AuthRequest struct {
    // ... existing fields unchanged ...
    DeviceSessionID string // populated from the 'dsid' cookie during FederatedCallback
}
```

**Modify `RefreshToken`** — add one field:
```go
type RefreshToken struct {
    // ... existing fields unchanged ...
    DeviceSessionID string // empty for legacy tokens
}
```

---

## 3. Ports Layer Changes

### File: `internal/oidc/ports.go` — additions and signature changes

**New interface:**
```go
type DeviceSessionRepository interface {
    // FindOrCreate looks up an existing device session by (id, userID).
    // If found with matching userID, updates user_agent, ip_address, last_used_at and returns it.
    // If found with a DIFFERENT userID (cookie reuse across accounts), creates a new session.
    // If not found at all, creates a new session with the given id.
    // Returns the session that should be used (may have a different ID than requested).
    FindOrCreate(ctx context.Context, id, userID, userAgent, ipAddress, deviceName string) (*DeviceSession, error)

    // RevokeByID sets revoked_at on the device session and deletes all active refresh tokens
    // linked to it. Scoped to userID to prevent cross-user revocation.
    RevokeByID(ctx context.Context, id, userID string) error

    // ListActiveByUserID returns all non-revoked device sessions for a user, ordered by last_used_at DESC.
    ListActiveByUserID(ctx context.Context, userID string) ([]*DeviceSession, error)

    // DeleteRevokedBefore removes device sessions where revoked_at < before AND they have no
    // remaining linked refresh tokens. Returns the count deleted.
    DeleteRevokedBefore(ctx context.Context, before time.Time) (int64, error)
}

type DeviceSessionService interface {
    FindOrCreate(ctx context.Context, id, userID, userAgent, ipAddress, deviceName string) (*DeviceSession, error)
    RevokeByID(ctx context.Context, id, userID string) error
    ListActiveByUserID(ctx context.Context, userID string) ([]*DeviceSession, error)
    DeleteRevokedBefore(ctx context.Context, before time.Time) (int64, error)
}
```

**Modify `AuthRequestRepository`** — `CompleteLogin` gains `deviceSessionID string`:
```go
CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string, deviceSessionID string) error
```

This change propagates to: `authrequest_repo.go` (SQL), `authrequest_svc.go` (delegation), `authrequest_svc_test.go` (mock), `storage.go` (`s.authReqs.CompleteLogin` call), `login_usecase.go` (the call site in `authn`).

**Modify `AuthRequestService`** — same signature change on `CompleteLogin`.

**Modify `LoginCompleter`** — same signature change:
```go
type LoginCompleter interface {
    CompleteLogin(ctx context.Context, authRequestID, userID string, authTime time.Time, amr []string, deviceSessionID string) error
}
```

This interface is implemented by `authRequestService` and consumed by `authn.CompleteFederatedLogin`.

---

## 4. Repository Layer

### File: `internal/oidc/postgres/device_session_repo.go` (new)

```go
package postgres

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"

    "github.com/jmoiron/sqlx"
    "github.com/oklog/ulid/v2"

    oidcdom "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
    "github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

var _ oidcdom.DeviceSessionRepository = (*DeviceSessionRepository)(nil)

type DeviceSessionRepository struct{ db *sqlx.DB }

func NewDeviceSessionRepository(db *sqlx.DB) *DeviceSessionRepository {
    return &DeviceSessionRepository{db: db}
}
```

**`FindOrCreate` implementation — the core logic:**

```go
func (r *DeviceSessionRepository) FindOrCreate(
    ctx context.Context, id, userID, userAgent, ipAddress, deviceName string,
) (*oidcdom.DeviceSession, error) {
    // Attempt to find an existing session by ID
    var ds oidcdom.DeviceSession
    var revokedAt sql.NullTime
    err := r.db.QueryRowxContext(ctx,
        `SELECT id, user_id, user_agent, ip_address, device_name, created_at, last_used_at, revoked_at
         FROM device_sessions WHERE id = $1`, id,
    ).Scan(&ds.ID, &ds.UserID, &ds.UserAgent, &ds.IPAddress, &ds.DeviceName,
           &ds.CreatedAt, &ds.LastUsedAt, &revokedAt)

    if err == nil {
        // Row found — check user ownership and revocation
        if ds.UserID != userID || revokedAt.Valid {
            // Different user or already revoked: create a fresh session
            return r.create(ctx, ulid.Make().String(), userID, userAgent, ipAddress, deviceName)
        }
        // Matching user, active — update metadata
        _, err = r.db.ExecContext(ctx,
            `UPDATE device_sessions
             SET user_agent = $1, ip_address = $2, last_used_at = now()
             WHERE id = $3`,
            userAgent, ipAddress, ds.ID,
        )
        if err != nil {
            return nil, fmt.Errorf("update device session: %w", err)
        }
        ds.UserAgent = userAgent
        ds.IPAddress = ipAddress
        ds.LastUsedAt = time.Now().UTC()
        return &ds, nil
    }
    if !errors.Is(err, sql.ErrNoRows) {
        return nil, fmt.Errorf("lookup device session: %w", err)
    }
    // Not found — create with the requested ID (first login on this device)
    return r.create(ctx, id, userID, userAgent, ipAddress, deviceName)
}

func (r *DeviceSessionRepository) create(
    ctx context.Context, id, userID, userAgent, ipAddress, deviceName string,
) (*oidcdom.DeviceSession, error) {
    now := time.Now().UTC()
    _, err := r.db.ExecContext(ctx,
        `INSERT INTO device_sessions (id, user_id, user_agent, ip_address, device_name, created_at, last_used_at)
         VALUES ($1, $2, $3, $4, $5, $6, $6)`,
        id, userID, userAgent, ipAddress, deviceName, now,
    )
    if err != nil {
        return nil, fmt.Errorf("create device session: %w", err)
    }
    return &oidcdom.DeviceSession{
        ID: id, UserID: userID, UserAgent: userAgent,
        IPAddress: ipAddress, DeviceName: deviceName,
        CreatedAt: now, LastUsedAt: now,
    }, nil
}
```

**`RevokeByID` implementation — atomic revoke:**

```go
func (r *DeviceSessionRepository) RevokeByID(ctx context.Context, id, userID string) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer func() { _ = tx.Rollback() }()

    result, err := tx.ExecContext(ctx,
        `UPDATE device_sessions SET revoked_at = now()
         WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL`, id, userID)
    if err != nil {
        return fmt.Errorf("revoke device session: %w", err)
    }
    n, _ := result.RowsAffected()
    if n == 0 {
        return fmt.Errorf("device session %s: %w", id, domerr.ErrNotFound)
    }

    // Delete all active refresh tokens linked to this device session.
    // Their associated access tokens will naturally expire; this is intentional.
    if _, err = tx.ExecContext(ctx,
        `DELETE FROM refresh_tokens WHERE device_session_id = $1`, id,
    ); err != nil {
        return fmt.Errorf("delete refresh tokens for device session: %w", err)
    }

    return tx.Commit()
}
```

**`ListActiveByUserID`:**
```go
func (r *DeviceSessionRepository) ListActiveByUserID(
    ctx context.Context, userID string,
) ([]*oidcdom.DeviceSession, error) {
    rows, err := r.db.QueryxContext(ctx,
        `SELECT id, user_id, user_agent, ip_address, device_name, created_at, last_used_at, revoked_at
         FROM device_sessions
         WHERE user_id = $1 AND revoked_at IS NULL
         ORDER BY last_used_at DESC`, userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var sessions []*oidcdom.DeviceSession
    for rows.Next() {
        var ds oidcdom.DeviceSession
        var revokedAt sql.NullTime
        if err := rows.Scan(&ds.ID, &ds.UserID, &ds.UserAgent, &ds.IPAddress, &ds.DeviceName,
                            &ds.CreatedAt, &ds.LastUsedAt, &revokedAt); err != nil {
            return nil, err
        }
        sessions = append(sessions, &ds)
    }
    return sessions, rows.Err()
}
```

**`DeleteRevokedBefore`:**
```go
func (r *DeviceSessionRepository) DeleteRevokedBefore(ctx context.Context, before time.Time) (int64, error) {
    result, err := r.db.ExecContext(ctx,
        // Only delete if no active (unexpired) refresh tokens remain linked
        `DELETE FROM device_sessions
         WHERE revoked_at IS NOT NULL
           AND revoked_at < $1
           AND NOT EXISTS (
               SELECT 1 FROM refresh_tokens
               WHERE device_session_id = device_sessions.id
                 AND expiration > now()
           )`, before)
    if err != nil {
        return 0, fmt.Errorf("delete revoked device sessions: %w", err)
    }
    n, _ := result.RowsAffected()
    return n, nil
}
```

### Modify `internal/oidc/postgres/authrequest_repo.go`

Update `CompleteLogin` to accept and persist `deviceSessionID string`:

```go
func (r *AuthRequestRepository) CompleteLogin(
    ctx context.Context, id, userID string, authTime time.Time, amr []string, deviceSessionID string,
) error {
    _, err := r.db.ExecContext(ctx,
        `UPDATE auth_requests
         SET user_id = $1, auth_time = $2, amr = $3, is_done = true, device_session_id = $4
         WHERE id = $5`,
        userID, authTime, pq.Array(amr), nilIfEmptyStr(deviceSessionID), id,
    )
    return err
}
```

Where `nilIfEmptyStr` converts `""` to a `*string = nil` (reuse the same `nilIfEmpty` helper in the same file... actually it's in token_repo.go currently, not imported here. We should move it to a shared helper or duplicate it — given the tiny size, duplicate it in authrequest_repo.go).

### Modify `internal/oidc/postgres/token_repo.go`

**Update `CreateAccessAndRefresh`** to carry forward `device_session_id` during rotation, and persist `device_session_id` on new refresh tokens:

The `RefreshToken` domain struct now has `DeviceSessionID string`. The INSERT statement and scan must be updated.

For rotation (when `currentRefreshToken != ""`), the transaction additionally runs:
```sql
SELECT device_session_id FROM refresh_tokens WHERE token_hash = $1
```
And sets `refresh.DeviceSessionID` from the result before inserting the new refresh token.

For `last_used_at` update on the device session — done atomically in the token rotation transaction:
```sql
UPDATE device_sessions SET last_used_at = now() WHERE id = $device_session_id
```
This is added after the new refresh token is inserted, if `refresh.DeviceSessionID != ""`.

Full updated `CreateAccessAndRefresh` logic:
```go
func (r *TokenRepository) CreateAccessAndRefresh(
    ctx context.Context,
    access *oidc.Token,
    refresh *oidc.RefreshToken,
    currentRefreshToken string,  // hashed; empty for initial issuance
) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer func() { _ = tx.Rollback() }()

    if currentRefreshToken != "" {
        // --- Rotation path ---
        var oldAccessTokenID sql.NullString
        var oldDeviceSessionID sql.NullString  // NEW: carry forward device session
        err = tx.QueryRowContext(ctx,
            `SELECT access_token_id, device_session_id
             FROM refresh_tokens WHERE token_hash = $1`,
            currentRefreshToken,
        ).Scan(&oldAccessTokenID, &oldDeviceSessionID)
        if err != nil && !errors.Is(err, sql.ErrNoRows) {
            return fmt.Errorf("lookup old refresh token: %w", err)
        }

        // Inherit device session from old token if not already set
        if refresh.DeviceSessionID == "" && oldDeviceSessionID.Valid {
            refresh.DeviceSessionID = oldDeviceSessionID.String
        }

        if oldAccessTokenID.Valid && oldAccessTokenID.String != "" {
            if _, err = tx.ExecContext(ctx,
                `DELETE FROM tokens WHERE id = $1`, oldAccessTokenID.String,
            ); err != nil {
                return fmt.Errorf("revoke old access token: %w", err)
            }
        }

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
            return fmt.Errorf("refresh token already used or expired: %w", domerr.ErrNotFound)
        }
    }

    _, err = tx.ExecContext(ctx,
        `INSERT INTO tokens (id, client_id, subject, audience, scopes, expiration, refresh_token_id)
         VALUES ($1,$2,$3,$4,$5,$6,$7)`,
        access.ID, access.ClientID, access.Subject,
        pq.Array(access.Audience), pq.Array(access.Scopes),
        access.Expiration, access.RefreshTokenID,
    )
    if err != nil {
        return err
    }

    _, err = tx.ExecContext(ctx,
        `INSERT INTO refresh_tokens
         (id, token_hash, client_id, user_id, audience, scopes, auth_time, amr, access_token_id, expiration, device_session_id)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
        refresh.ID, refresh.Token, refresh.ClientID, refresh.UserID,
        pq.Array(refresh.Audience), pq.Array(refresh.Scopes),
        refresh.AuthTime, pq.Array(refresh.AMR),
        refresh.AccessTokenID, refresh.Expiration,
        nilIfEmpty(refresh.DeviceSessionID),  // NULL for initial tokens without device session (edge case)
    )
    if err != nil {
        return err
    }

    // Update device session last_used_at atomically if this token has one
    if refresh.DeviceSessionID != "" {
        if _, err = tx.ExecContext(ctx,
            `UPDATE device_sessions SET last_used_at = now() WHERE id = $1`,
            refresh.DeviceSessionID,
        ); err != nil {
            return fmt.Errorf("update device session last_used_at: %w", err)
        }
    }

    return tx.Commit()
}
```

Also update `scanRefreshToken` to scan the new `device_session_id` column:
```go
var deviceSessionID sql.NullString
err := row.Scan(&rt.ID, &rt.Token, &rt.ClientID, &rt.UserID, &audience, &scopes,
    &rt.AuthTime, &amr, &accessTokenID, &rt.Expiration, &rt.CreatedAt, &deviceSessionID)
// ... after scan:
if deviceSessionID.Valid {
    rt.DeviceSessionID = deviceSessionID.String
}
```

The SELECT statements in `GetRefreshToken` and `GetRefreshInfo` need `device_session_id` added to their column lists.

---

## 5. Service Layer

### File: `internal/oidc/device_session_svc.go` (new)

A thin service that delegates to the repository. Pattern matches `authrequest_svc.go`:

```go
package oidc

import "context"
import "time"

var _ DeviceSessionService = (*deviceSessionService)(nil)

type deviceSessionService struct {
    repo DeviceSessionRepository
}

func NewDeviceSessionService(repo DeviceSessionRepository) DeviceSessionService {
    return &deviceSessionService{repo: repo}
}

func (s *deviceSessionService) FindOrCreate(ctx context.Context, id, userID, userAgent, ipAddress, deviceName string) (*DeviceSession, error) {
    return s.repo.FindOrCreate(ctx, id, userID, userAgent, ipAddress, deviceName)
}

func (s *deviceSessionService) RevokeByID(ctx context.Context, id, userID string) error {
    return s.repo.RevokeByID(ctx, id, userID)
}

func (s *deviceSessionService) ListActiveByUserID(ctx context.Context, userID string) ([]*DeviceSession, error) {
    return s.repo.ListActiveByUserID(ctx, userID)
}

func (s *deviceSessionService) DeleteRevokedBefore(ctx context.Context, before time.Time) (int64, error) {
    return s.repo.DeleteRevokedBefore(ctx, before)
}
```

No additional logic at the service layer for now. Future: could add telemetry, caching.

### Modify `internal/oidc/authrequest_svc.go`

Update `CompleteLogin` delegation to pass through `deviceSessionID`:
```go
func (s *authRequestService) CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string, deviceSessionID string) error {
    return s.repo.CompleteLogin(ctx, id, userID, authTime, amr, deviceSessionID)
}
```

---

## 6. authn Package Changes

This package is the primary consumer of device sessions. It handles the HTTP layer where the cookie lives.

### New file: `internal/authn/devicename.go`

Holds a package-private `parseDeviceName(ua string) string` function. No new dependency — a 50-line pure heuristic covering iOS, Android, Windows, Mac, Linux, and the top 5 browsers.

```go
package authn

import "strings"

// parseDeviceName extracts a human-readable device label from a User-Agent string.
// Examples: "Chrome on macOS", "Safari on iPhone", "Firefox on Windows"
func parseDeviceName(ua string) string {
    os := parseOS(ua)
    browser := parseBrowser(ua)
    if browser == "" && os == "" {
        return "Unknown device"
    }
    if browser == "" {
        return os
    }
    if os == "" {
        return browser
    }
    return browser + " on " + os
}

func parseOS(ua string) string {
    switch {
    case strings.Contains(ua, "iPhone"):   return "iPhone"
    case strings.Contains(ua, "iPad"):     return "iPad"
    case strings.Contains(ua, "Android"):  return "Android"
    case strings.Contains(ua, "Windows"):  return "Windows"
    case strings.Contains(ua, "Macintosh"), strings.Contains(ua, "Mac OS X"):
        return "macOS"
    case strings.Contains(ua, "Linux"):    return "Linux"
    default:                               return ""
    }
}

func parseBrowser(ua string) string {
    switch {
    case strings.Contains(ua, "Edg/"):     return "Edge"
    case strings.Contains(ua, "OPR/") || strings.Contains(ua, "Opera"):
        return "Opera"
    case strings.Contains(ua, "Chrome"):   return "Chrome"
    case strings.Contains(ua, "Firefox"):  return "Firefox"
    case strings.Contains(ua, "Safari") && !strings.Contains(ua, "Chrome"):
        return "Safari"
    default:                               return ""
    }
}
```

This covers >95% of real browsers. Order matters: Edge contains "Chrome", Opera contains "OPR", so they must be checked first.

### Modify `internal/authn/handler.go`

**New Handler fields:**
```go
type Handler struct {
    // ... existing fields ...
    deviceSessions oidcdom.DeviceSessionService
}
```

**Update `NewHandler` signature** to accept `deviceSessions oidcdom.DeviceSessionService`.

**Cookie constants (package-level):**
```go
const (
    deviceCookieName = "dsid"
    deviceCookieMaxAge = 63072000 // 2 years, matching HSTS
)
```

**`SelectProvider` change** — read or generate the device session cookie (no DB write yet):

```go
func (h *Handler) SelectProvider(w http.ResponseWriter, r *http.Request) {
    authRequestID := r.URL.Query().Get("authRequestID")
    if authRequestID == "" {
        http.Error(w, "missing authRequestID", http.StatusBadRequest)
        return
    }

    // Ensure the device session cookie exists.
    // If missing, set a pre-generated ID now so it's present on the callback.
    if _, err := r.Cookie(deviceCookieName); err != nil {
        http.SetCookie(w, &http.Cookie{
            Name:     deviceCookieName,
            Value:    ulid.Make().String(),
            MaxAge:   deviceCookieMaxAge,
            HttpOnly: true,
            Secure:   true,
            SameSite: http.SameSiteLaxMode,
            Path:     "/",
        })
    }

    // ... existing template rendering unchanged ...
}
```

**Why `Lax` not `Strict`?**
The `GET /login?authRequestID=...` request is triggered by a redirect from a different origin (the RP redirecting the user's browser to the accounts service). With `SameSite=Strict`, the cookie would not be sent on that initial navigation, so the existing cookie would appear missing even though it was set before. `Lax` allows the cookie to be sent on top-level GET navigations from other origins, which is exactly our flow.

**`FederatedCallback` change** — create/find device session after user is identified:

```go
func (h *Handler) FederatedCallback(w http.ResponseWriter, r *http.Request) {
    // ... existing: decrypt state, exchange code, fetch claims ...

    user, err := h.loginUC.identitySvc.FindOrCreateByFederatedLogin(
        r.Context(), state.Provider, *claims)
    // NOTE: loginUC.Execute now needs to be split or extended to pass deviceSessionID.
    // See login_usecase.go changes below.

    // Determine device session ID from cookie
    dsID := ""
    if cookie, err := r.Cookie(deviceCookieName); err == nil {
        dsID = cookie.Value
    }
    if dsID == "" {
        dsID = ulid.Make().String() // fallback: no cookie (e.g. direct API call)
    }

    deviceName := parseDeviceName(r.UserAgent())
    ipAddress := clientIP(r)  // reuse the IP extraction logic

    ds, err := h.deviceSessions.FindOrCreate(
        r.Context(), dsID, user.ID,
        r.UserAgent(), ipAddress, deviceName,
    )
    if err != nil {
        h.logger.Error("device session find-or-create failed", "error", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    // If FindOrCreate returned a different ID (cross-user cookie reuse),
    // update the cookie with the new ID.
    if ds.ID != dsID {
        http.SetCookie(w, &http.Cookie{
            Name:     deviceCookieName,
            Value:    ds.ID,
            MaxAge:   deviceCookieMaxAge,
            HttpOnly: true,
            Secure:   true,
            SameSite: http.SameSiteLaxMode,
            Path:     "/",
        })
    }

    // Complete the login, passing the device session ID to be stored on the auth request
    if err := h.loginUC.CompleteLogin(
        r.Context(), state.AuthRequestID, user.ID, ds.ID,
    ); err != nil {
        h.logger.Error("login completion failed", "error", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    callbackURL := h.callbackURL(r.Context(), state.AuthRequestID)
    http.Redirect(w, r, callbackURL, http.StatusFound)
}
```

We need the IP extraction from the middleware. Rather than importing the middleware package (which would create an import cycle since both are internal), extract `clientIP` into a shared helper or duplicate the 5-line function. Best approach: a tiny helper in the `authn` package (`internal/authn/ip.go`) since the logic is just reading `CF-Connecting-IP` or `RemoteAddr`:

```go
// internal/authn/ip.go
package authn

import (
    "net"
    "net/http"
    "strings"
)

func clientIP(r *http.Request) string {
    if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
        return cf
    }
    host, _, err := net.SplitHostPort(r.RemoteAddr)
    if err != nil {
        return r.RemoteAddr
    }
    return host
}
```

This duplicates 5 lines from `middleware/ratelimit.go`. The alternative is extracting to a shared `internal/pkg/netutil` package, but that's over-engineering for now — two copies of 5 lines is fine.

### Modify `internal/authn/login_usecase.go`

**Current structure:** `CompleteFederatedLogin.Execute` does identity lookup + `CompleteLogin`.

**Problem:** `FederatedCallback` now needs to call identity lookup first (to get user.ID before device session creation), then `CompleteLogin`. This means the use case's `Execute` monolith must be split or the handler needs to call the identity service directly.

**Approach: keep the use case but restructure it.** The use case becomes a struct with two methods instead of one:

```go
type CompleteFederatedLogin struct {
    identity  identity.Service
    loginComp oidcdom.LoginCompleter
}

// FindOrCreateUser resolves the identity for the federated claims.
// Called first, before device session creation.
func (uc *CompleteFederatedLogin) FindOrCreateUser(
    ctx context.Context, provider string, claims identity.FederatedClaims,
) (*identity.User, error) {
    user, err := uc.identity.FindOrCreateByFederatedLogin(ctx, provider, claims)
    if err != nil {
        return nil, fmt.Errorf("federated login: %w", err)
    }
    return user, nil
}

// CompleteLogin marks the auth request as done, recording auth time, AMR, and device session.
func (uc *CompleteFederatedLogin) CompleteLogin(
    ctx context.Context, authRequestID, userID, deviceSessionID string,
) error {
    authTime := time.Now().UTC()
    amr := []string{"fed"}
    if err := uc.loginComp.CompleteLogin(
        ctx, authRequestID, userID, authTime, amr, deviceSessionID,
    ); err != nil {
        return fmt.Errorf("complete login: %w", err)
    }
    return nil
}
```

The handler then calls these two steps explicitly, with device session creation in between (shown above).

---

## 7. OIDC Adapter Changes

### Modify `internal/oidc/adapter/storage.go`

**`CreateAccessAndRefreshTokens` — propagate `device_session_id`:**

When creating initial tokens (auth code exchange), `request` is `*AuthRequest`. The `*AuthRequest` adapter wraps `*oidcdom.AuthRequest` which now has `DeviceSessionID`. We pass it to the token service:

```go
func (s *StorageAdapter) CreateAccessAndRefreshTokens(
    ctx context.Context, request op.TokenRequest, currentRefreshToken string,
) (string, string, time.Time, error) {
    // ... existing expiration/authTime/amr extraction ...

    // Extract device session ID if available from the auth request
    deviceSessionID := extractDeviceSessionID(request)

    accessID, refreshToken, err := s.tokens.CreateAccessAndRefresh(ctx,
        clientIDFromRequest(request), request.GetSubject(), request.GetAudience(), request.GetScopes(),
        accessExp, refreshExp, authTime, amr, currentRefreshToken, deviceSessionID,
    )
    // ... rest unchanged ...
}

func extractDeviceSessionID(request op.TokenRequest) string {
    type deviceSessionGetter interface {
        GetDeviceSessionID() string
    }
    if g, ok := request.(deviceSessionGetter); ok {
        return g.GetDeviceSessionID()
    }
    return ""
}
```

For token refresh (`*RefreshTokenRequest`), `GetDeviceSessionID()` returns `""` and the device session is inherited from the old token in the repository layer (already handled in step 4 above). This means no change is needed to `RefreshTokenRequest`.

**Update `TokenService.CreateAccessAndRefresh` signature** to accept `deviceSessionID string` as the last parameter (before or after `currentRefreshToken`):
```go
CreateAccessAndRefresh(ctx context.Context, clientID, subject string,
    audience, scopes []string,
    accessExpiration, refreshExpiration, authTime time.Time,
    amr []string, currentRefreshToken, deviceSessionID string,
) (accessID, refreshToken string, err error)
```

**`AuthRequest` adapter** (`internal/oidc/adapter/authrequest.go`) — add `GetDeviceSessionID()`:
```go
func (a *AuthRequest) GetDeviceSessionID() string { return a.ar.DeviceSessionID }
```

**Fix `RevokeToken` — swap the branches:**

The current logic has access/refresh branches INVERTED. The fix:

```go
func (s *StorageAdapter) RevokeToken(ctx context.Context, tokenOrTokenID, userID, clientID string) *oidc.Error {
    if userID != "" {
        // GetRefreshTokenInfo succeeded → this IS a refresh token
        // tokenOrTokenID is the raw refresh token value
        if err := s.tokens.RevokeRefreshToken(ctx, tokenOrTokenID, clientID); err != nil {
            if errors.Is(err, domerr.ErrNotFound) {
                return oidc.ErrInvalidRequest().WithDescription("token not found")
            }
            return oidc.ErrServerError().WithParent(err)
        }
        return nil
    }
    // GetRefreshTokenInfo failed → this is an access token
    // tokenOrTokenID is the access token ID
    if err := s.tokens.Revoke(ctx, tokenOrTokenID, clientID); err != nil {
        if errors.Is(err, domerr.ErrNotFound) {
            return oidc.ErrInvalidRequest().WithDescription("token not found")
        }
        return oidc.ErrServerError().WithParent(err)
    }
    return nil
}
```

**Why this is the correct fix:** Zitadel calls `GetRefreshTokenInfo(ctx, clientID, token)` in its revocation handler. If that returns a non-empty `userID`, it passes `userID` into `RevokeToken`, indicating the token was identified as a refresh token. The old code then called `Revoke` (access token path) — wrong. The fix routes refresh tokens to `RevokeRefreshToken` and access tokens to `Revoke`.

---

## 8. Security Headers — Add CSP for Login Routes

### Modify `internal/middleware/securityheaders.go`

Add a second middleware function `LoginPageSecurityHeaders()` for the `/login/*` routes:

```go
// LoginPageSecurityHeaders sets security headers appropriate for the HTML login page.
// In addition to the standard headers, it adds a Content-Security-Policy
// tailored to the inline-styled M3 template.
func LoginPageSecurityHeaders() func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            h := w.Header()
            // Standard headers (same as SecurityHeaders)
            h.Set("X-Content-Type-Options", "nosniff")
            h.Set("X-Frame-Options", "DENY")
            h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
            h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
            // CSP: the M3 template uses inline styles and scripts.
            // form-action 'self' is the critical constraint — prevents form hijacking.
            // frame-ancestors 'none' redundantly enforces X-Frame-Options: DENY.
            h.Set("Content-Security-Policy",
                "default-src 'none'; "+
                    "style-src 'unsafe-inline'; "+
                    "script-src 'unsafe-inline'; "+
                    "img-src 'self' data: https:; "+       // provider logos may be data URIs
                    "connect-src 'none'; "+
                    "form-action 'self'; "+
                    "frame-ancestors 'none'; "+
                    "base-uri 'self'",
            )
            next.ServeHTTP(w, r)
        })
    }
}
```

**Note on `unsafe-inline`:** The `select_provider.html` template uses inline `<style>` and inline `<script>` for ripple effects. Adding nonces would require template-level changes; that's a P4 improvement. The critical CSP gains here are `form-action 'self'` (prevents the POST from being targeted at a different origin) and `frame-ancestors 'none'` (clickjacking protection).

### Modify `main.go`

In `runServer`, replace `r.Use(interceptor.Handler)` in the `/login` route group with:
```go
router.Route("/login", func(r chi.Router) {
    if cfg.RateLimitEnabled {
        r.Use(loginLimiter.Middleware())
    }
    r.Use(appmiddleware.LoginPageSecurityHeaders())  // replaces generic SecurityHeaders for this sub-router
    r.Use(interceptor.Handler)
    r.Get("/", loginHandler.SelectProvider)
    r.Post("/select", loginHandler.FederatedRedirect)
    r.Get("/callback", loginHandler.FederatedCallback)
})
```

The outer router keeps `appmiddleware.SecurityHeaders()` for OIDC endpoints. The `/login` sub-router overrides with `LoginPageSecurityHeaders()` which adds CSP.

---

## 9. main.go Wiring Changes

```go
func runServer(cfg *config.Config, db *sqlx.DB, tokenSvc oidcdom.TokenService, logger *slog.Logger) {
    // ... existing repos/services unchanged ...

    // NEW: device session layer
    deviceSessionRepo := oidcpg.NewDeviceSessionRepository(db)
    deviceSessionSvc  := oidcdom.NewDeviceSessionService(deviceSessionRepo)

    // ... storage, provider, upstream providers unchanged ...

    loginHandler := authn.NewHandler(
        upstreamProviders,
        identitySvc,
        authReqSvc,
        deviceSessionSvc,           // NEW parameter
        crypto.NewAESCipher(cfg.CryptoKey),
        op.AuthCallbackURL(provider),
        logger,
    )

    // ... rate limiters, router unchanged ...

    // NEW: background cleanup for revoked device sessions (hourly, alongside token cleanup)
    go runDeviceSessionCleanup(cleanupCtx, deviceSessionSvc, 30*24*time.Hour, logger)
}

func runDeviceSessionCleanup(
    ctx context.Context,
    svc oidcdom.DeviceSessionService,
    maxAge time.Duration,
    logger *slog.Logger,
) {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            cutoff := time.Now().UTC().Add(-maxAge)
            n, err := svc.DeleteRevokedBefore(ctx, cutoff)
            if err != nil {
                logger.Error("device session cleanup failed", "error", err)
                continue
            }
            if n > 0 {
                logger.Info("cleaned up revoked device sessions", "count", n)
            }
        }
    }
}
```

The `cleanup` subcommand does not need changes for now (device sessions are cleaned by the server process; a future improvement could add it to the CronJob too).

---

## 10. Test Strategy

### Integration tests for DeviceSessionRepository

New file: `internal/oidc/postgres/device_session_repo_test.go`

Required test cases:
- `TestDeviceSessionRepository_Create` — new session created on first call with unknown ID
- `TestDeviceSessionRepository_FindExisting` — existing session updated on re-use (user_agent, last_used_at)
- `TestDeviceSessionRepository_CrossUser` — cookie with different user_id triggers creation of new session
- `TestDeviceSessionRepository_RevokedSession` — revoked session triggers creation of new session
- `TestDeviceSessionRepository_RevokeByID` — sets revoked_at, deletes linked refresh tokens
- `TestDeviceSessionRepository_RevokeByID_WrongUser` — returns ErrNotFound
- `TestDeviceSessionRepository_ListActiveByUserID` — excludes revoked sessions
- `TestDeviceSessionRepository_DeleteRevokedBefore` — only deletes when no active refresh tokens remain

### Integration test additions to existing files

`internal/oidc/postgres/repo_test.go`:
- `TestTokenRepository_DeviceSessionIDCarriedForward` — rotation carries device_session_id from old to new refresh token
- `TestTokenRepository_LastUsedAtUpdated` — device_sessions.last_used_at is updated on token refresh

`internal/oidc/adapter/storage_test.go`:
- `TestStorage_CreateAccessAndRefreshTokens_WithDeviceSession` — device_session_id propagated to refresh token
- `TestStorage_RevokeToken_RefreshToken` — verify refresh token revocation works (was previously broken)
- `TestStorage_RevokeToken_AccessToken` — verify access token revocation works

### Unit tests

`internal/authn/handler_test.go`:
- `TestSelectProvider_SetsCookieWhenMissing` — cookie is set if none present
- `TestSelectProvider_KeepsExistingCookie` — no new cookie if one already exists
- `TestFederatedCallback_CreatesDeviceSession` — mock device session service called
- `TestFederatedCallback_UpdatesCookieOnUserMismatch` — new cookie set when FindOrCreate returns different ID

`internal/authn/devicename_test.go`:
- Table-driven tests for `parseDeviceName` covering iOS, Android, macOS, Windows, Linux, Edge, Chrome, Firefox, Safari, empty UA

---

## 11. Complete File Change Summary

| File | Change Type | Summary |
|------|-------------|---------|
| `migrations/3_device_sessions.up.sql` | **NEW** | device_sessions table; FK columns on refresh_tokens, auth_requests |
| `migrations/3_device_sessions.down.sql` | **NEW** | Rollback |
| `testhelper/testdb.go` | MODIFY | Add `device_sessions` to `CleanTables` |
| `internal/oidc/domain.go` | MODIFY | Add `DeviceSession` type; add `DeviceSessionID` field to `AuthRequest` and `RefreshToken` |
| `internal/oidc/ports.go` | MODIFY | Add `DeviceSessionRepository`, `DeviceSessionService`; update `CompleteLogin` signature on `AuthRequestRepository`, `AuthRequestService`, `LoginCompleter`; update `TokenService.CreateAccessAndRefresh`; update `TokenRepository.CreateAccessAndRefresh` |
| `internal/oidc/device_session_svc.go` | **NEW** | Thin service delegating to repository |
| `internal/oidc/authrequest_svc.go` | MODIFY | `CompleteLogin` passes `deviceSessionID` through |
| `internal/oidc/authrequest_svc_test.go` | MODIFY | Update mock signatures |
| `internal/oidc/token_svc.go` | MODIFY | `CreateAccessAndRefresh` accepts `deviceSessionID`; passes to repo |
| `internal/oidc/token_svc_test.go` | MODIFY | Add `deviceSessionID` to calls |
| `internal/oidc/postgres/authrequest_repo.go` | MODIFY | `CompleteLogin` SQL includes `device_session_id = $4` |
| `internal/oidc/postgres/token_repo.go` | MODIFY | INSERT includes `device_session_id`; rotation carries it forward; UPDATE `last_used_at`; scan updated |
| `internal/oidc/postgres/device_session_repo.go` | **NEW** | Full repository implementation |
| `internal/oidc/postgres/device_session_repo_test.go` | **NEW** | Integration tests |
| `internal/oidc/postgres/repo_test.go` | MODIFY | New token tests for device session propagation |
| `internal/oidc/adapter/authrequest.go` | MODIFY | Add `GetDeviceSessionID()` method |
| `internal/oidc/adapter/authrequest_test.go` | MODIFY | Test new method |
| `internal/oidc/adapter/storage.go` | MODIFY | `CreateAccessAndRefreshTokens` propagates device session; fix `RevokeToken` |
| `internal/oidc/adapter/storage_test.go` | MODIFY | New tests for device session + RevokeToken fix |
| `internal/authn/handler.go` | MODIFY | Cookie issuance in SelectProvider; device session creation in FederatedCallback; new Handler field |
| `internal/authn/handler_test.go` | MODIFY | Cookie and device session tests |
| `internal/authn/login_usecase.go` | MODIFY | Split into `FindOrCreateUser` + `CompleteLogin` |
| `internal/authn/ip.go` | **NEW** | `clientIP` helper |
| `internal/authn/devicename.go` | **NEW** | `parseDeviceName`, `parseOS`, `parseBrowser` |
| `internal/authn/devicename_test.go` | **NEW** | UA parsing unit tests |
| `internal/middleware/securityheaders.go` | MODIFY | Add `LoginPageSecurityHeaders()` |
| `main.go` | MODIFY | Wire DeviceSessionRepository/Service; pass to NewHandler; add cleanup goroutine; use LoginPageSecurityHeaders |

---

## 12. Trade-offs and Rejected Alternatives

### T1: Cookie value = ULID vs. separate opaque random value

**Chosen:** ULID (same as device session DB primary key)
**Alternative:** Separate 32-byte random token (stored as SHA-256 hash in DB, like refresh tokens)

ULID as cookie is acceptable here because:
- The cookie is never used as a credential — it's just a session identifier
- Its lookup requires the user_id to match (so knowing a ULID doesn't let you access another user's data)
- Simplifies the implementation (no separate hash/verify step)

If we were storing the device session ID in a server-rendered page or URL param, we'd use the separate-hash approach. Being HttpOnly cookie makes it sufficient.

### T2: When to create the device session DB row

**Chosen:** FederatedCallback (after user is known)
**Alternative:** SelectProvider (pre-create with unknown user, update later)

Pre-creating at SelectProvider would produce orphaned rows for users who navigate to the login page but don't complete login. Creating only after successful authentication keeps the data clean and the row count bounded by real logins.

### T3: `last_used_at` update strategy — in-transaction vs. async

**Chosen:** In the `CreateAccessAndRefresh` transaction
**Alternative:** Async background update (fire-and-forget)

In-transaction is simpler, correct, and the overhead is one `UPDATE` per token refresh (which is already a transaction). The async approach risks dirty reads in `ListActiveByUserID`. For the expected refresh token frequency (every 15 minutes per logged-in client), the in-transaction overhead is negligible.

### T4: `RevokeByID` — soft delete vs. hard delete of device session

**Chosen:** Set `revoked_at` on device session (soft), DELETE the refresh tokens (hard)
**Alternative:** CASCADE DELETE device session → refresh tokens

Soft delete of the device session row preserves the audit trail for the myaccount BFF ("you revoked this device on Date X"). Hard deleting the refresh tokens ensures immediate enforcement. Cascade would enforce immediately too, but would lose the revocation record.

### T5: `deviceSessionID` in `TokenService.CreateAccessAndRefresh` vs. passed only through repo

Adding it to the service interface means the adapter knows about device sessions. An alternative is to have the service look it up from the auth request context (passing it through Go context). Context-passing of domain state is an anti-pattern — explicit parameter is cleaner.

For token refresh, `deviceSessionID = ""` is passed from the adapter (because `RefreshTokenRequest` doesn't know it), and the repo inherits it from the old token. This is a slight coupling between service and repo behavior, but it's documented by convention and tested.

### T6: `LoginCompleter` signature change — impact on interface consumers

The only consumer of `LoginCompleter` is `authn.CompleteFederatedLogin`. The only implementor is `authRequestService`. Both are in the same service module. The signature change is contained and safe.

### T7: Duplicate `clientIP` logic vs. shared package

The `clientIP` function is 5 lines and duplicated in `internal/authn/ip.go` vs. `internal/middleware/ratelimit.go`. Alternatives:
- Extract to `internal/pkg/netutil` (adds a new package for 5 lines — over-engineering)
- Import middleware from authn (import cycle: authn would import middleware, middleware imports nothing from authn today — actually this is fine, no cycle exists)
- Duplicate (chosen)

Duplication is acceptable here because the function is trivial and the two use cases are independent (rate limiting vs. device fingerprinting). If a third use case appears, extract to `internal/pkg/netutil`.

---

## 13. Data Flow Diagram (After Changes)

```
Browser (accounts domain cookie: dsid=<ULID>)
    │
    │  GET /login?authRequestID=X
    ▼
SelectProvider handler
  - reads dsid cookie
  - if missing: generate new ULID, set dsid cookie (MaxAge=2yr, HttpOnly, Secure, SameSite=Lax)
  - renders provider selection page
    │
    │  POST /login/select
    ▼
FederatedRedirect handler
  - reads dsid cookie (for future use; not DB operation)
  - encrypts federatedState{AuthRequestID, Provider, Nonce}
  - 302 → upstream IdP
    │
    ◄─── IdP callback
    │  GET /login/callback?code=...&state=...
    ▼
FederatedCallback handler
  1. decrypt state
  2. exchange code with IdP
  3. fetch claims from IdP
  4. loginUC.FindOrCreateUser(provider, claims)  → identity.Service → users table
  5. read dsid cookie value
  6. deviceSessions.FindOrCreate(dsid, user.ID, ua, ip, device_name)
       - creates/updates device_sessions row
       - returns ds (may have new ID if cookie was cross-user)
  7. if ds.ID != cookie: update dsid cookie
  8. loginUC.CompleteLogin(authRequestID, user.ID, ds.ID)
       - authRequestService.CompleteLogin(id, userID, authTime, amr, deviceSessionID)
       - SQL: UPDATE auth_requests SET user_id, auth_time, amr, is_done=true, device_session_id
  9. 302 → op.AuthCallbackURL (zitadel continues)
    │
    ▼ (zitadel internal)
StorageAdapter.SaveAuthCode(id, code)
    │
    │  POST /oauth/v2/token (code + code_verifier)
    ▼
StorageAdapter.AuthRequestByCode → oidcdom.AuthRequest (has DeviceSessionID)
StorageAdapter.CreateAccessAndRefreshTokens
  - request is *AuthRequest → GetDeviceSessionID() = ar.DeviceSessionID
  - TokenService.CreateAccessAndRefresh(..., deviceSessionID)
  - TokenRepository.CreateAccessAndRefresh:
      INSERT tokens (access token)
      INSERT refresh_tokens (... device_session_id = DeviceSessionID ...)
      UPDATE device_sessions SET last_used_at = now() WHERE id = DeviceSessionID
    │
    ▼
Client receives access_token + refresh_token

    │
    │  POST /oauth/v2/token (refresh_token=<raw>)  [future refreshes]
    ▼
StorageAdapter.TokenRequestByRefreshToken
StorageAdapter.CreateAccessAndRefreshTokens (currentRefreshToken = old raw value, deviceSessionID = "")
  - TokenRepository.CreateAccessAndRefresh:
      SELECT device_session_id FROM refresh_tokens WHERE token_hash = old_hash  [carry forward]
      DELETE old access token
      DELETE old refresh token (replay protection)
      INSERT new access token
      INSERT new refresh token (device_session_id = inherited from old)
      UPDATE device_sessions SET last_used_at = now()

    │
    │  Future: BFF calls device session API
    ▼
DeviceSessionService.ListActiveByUserID(userID)
  → SELECT FROM device_sessions WHERE user_id AND revoked_at IS NULL ORDER BY last_used_at DESC

DeviceSessionService.RevokeByID(id, userID)
  → UPDATE device_sessions SET revoked_at = now()
  → DELETE FROM refresh_tokens WHERE device_session_id = id
  (all access tokens for this device expire within accessTTL = 15min)
```

---

## 14. Implementation Order

Implement in this sequence to ensure each step compiles and tests pass before proceeding:

1. **Migration** (`3_device_sessions.up/down.sql`, update `testhelper/testdb.go`)
2. **Domain types** (`internal/oidc/domain.go`)
3. **Ports** (`internal/oidc/ports.go`) — all interface changes at once
4. **`authrequest_svc.go`** — `CompleteLogin` delegation update (compiles against updated ports)
5. **`authrequest_repo.go`** — SQL update
6. **`token_svc.go`** — accept `deviceSessionID`, pass to repo
7. **`token_repo.go`** — full update (INSERT, carry-forward, last_used_at, scan)
8. **`device_session_repo.go`** + tests
9. **`device_session_svc.go`**
10. **OIDC adapter** (`authrequest.go` + `storage.go` — device session propagation + RevokeToken fix)
11. **`authn` package** (`devicename.go`, `ip.go`, `login_usecase.go`, `handler.go`) + tests
12. **`securityheaders.go`** — add `LoginPageSecurityHeaders()`
13. **`main.go`** wiring
14. **Test sweep** — run all integration tests

---

## 15. Todo List

### Phase 1 — Database Migration ✅

- [x] Create `migrations/3_device_sessions.up.sql`
  - [x] `CREATE TABLE device_sessions` with all columns (`id`, `user_id`, `user_agent`, `ip_address`, `device_name`, `created_at`, `last_used_at`, `revoked_at`)
  - [x] `CREATE INDEX device_sessions_user_id_idx`
  - [x] `CREATE INDEX device_sessions_revoked_idx` (partial: `WHERE revoked_at IS NULL`)
  - [x] `ALTER TABLE refresh_tokens ADD COLUMN device_session_id TEXT REFERENCES device_sessions(id) ON DELETE SET NULL`
  - [x] `CREATE INDEX refresh_tokens_device_session_idx` (partial: `WHERE device_session_id IS NOT NULL`)
  - [x] `ALTER TABLE auth_requests ADD COLUMN device_session_id TEXT REFERENCES device_sessions(id) ON DELETE SET NULL`
- [x] Create `migrations/3_device_sessions.down.sql`
  - [x] `ALTER TABLE auth_requests DROP COLUMN IF EXISTS device_session_id`
  - [x] `ALTER TABLE refresh_tokens DROP COLUMN IF EXISTS device_session_id`
  - [x] `DROP TABLE IF EXISTS device_sessions`
- [x] Update `testhelper/testdb.go` — add `device_sessions` to `CleanTables` between `auth_requests` and `federated_identities` deletions

### Phase 2 — Domain Layer

- [ ] `internal/oidc/domain.go` — add `DeviceSession` struct (`ID`, `UserID`, `UserAgent`, `IPAddress`, `DeviceName`, `CreatedAt`, `LastUsedAt`, `RevokedAt *time.Time`)
- [ ] `internal/oidc/domain.go` — add `DeviceSessionID string` field to `AuthRequest`
- [ ] `internal/oidc/domain.go` — add `DeviceSessionID string` field to `RefreshToken`

### Phase 3 — Ports Layer

- [ ] `internal/oidc/ports.go` — add `DeviceSessionRepository` interface (`FindOrCreate`, `RevokeByID`, `ListActiveByUserID`, `DeleteRevokedBefore`)
- [ ] `internal/oidc/ports.go` — add `DeviceSessionService` interface (same four methods)
- [ ] `internal/oidc/ports.go` — update `AuthRequestRepository.CompleteLogin` signature: add `deviceSessionID string` parameter
- [ ] `internal/oidc/ports.go` — update `AuthRequestService.CompleteLogin` signature: add `deviceSessionID string` parameter
- [ ] `internal/oidc/ports.go` — update `LoginCompleter.CompleteLogin` signature: add `deviceSessionID string` parameter
- [ ] `internal/oidc/ports.go` — update `TokenService.CreateAccessAndRefresh` signature: add `deviceSessionID string` as last parameter
- [ ] `internal/oidc/ports.go` — update `TokenRepository.CreateAccessAndRefresh` signature: add `deviceSessionID string` (via updated `RefreshToken` struct field — already covered by domain change; verify repo interface matches)

### Phase 4 — Auth Request Service

- [ ] `internal/oidc/authrequest_svc.go` — update `CompleteLogin` to accept and forward `deviceSessionID string` to `s.repo.CompleteLogin`
- [ ] `internal/oidc/authrequest_svc_test.go` — update mock/stub `CompleteLogin` call signatures to include `deviceSessionID` argument

### Phase 5 — Auth Request Repository

- [ ] `internal/oidc/postgres/authrequest_repo.go` — update `CompleteLogin` SQL: add `device_session_id = $4` to the `UPDATE auth_requests SET ...` statement; add `nilIfEmptyStr` helper (converts `""` to `*string = nil`)

### Phase 6 — Token Service

- [ ] `internal/oidc/token_svc.go` — update `CreateAccessAndRefresh` to accept `deviceSessionID string` as last parameter
- [ ] `internal/oidc/token_svc.go` — assign `DeviceSessionID: deviceSessionID` on the `refresh` struct before calling `s.repo.CreateAccessAndRefresh`
- [ ] `internal/oidc/token_svc_test.go` — update all `CreateAccessAndRefresh` call sites to pass `deviceSessionID` argument (use `""` for existing tests)

### Phase 7 — Token Repository

- [ ] `internal/oidc/postgres/token_repo.go` — update `CreateAccessAndRefresh` rotation path: add `SELECT device_session_id FROM refresh_tokens WHERE token_hash = $1` and store in `oldDeviceSessionID sql.NullString`
- [ ] `internal/oidc/postgres/token_repo.go` — carry-forward logic: if `refresh.DeviceSessionID == ""` and `oldDeviceSessionID.Valid`, set `refresh.DeviceSessionID = oldDeviceSessionID.String`
- [ ] `internal/oidc/postgres/token_repo.go` — update `INSERT INTO refresh_tokens` to include `device_session_id` column (`$11`) with `nilIfEmpty(refresh.DeviceSessionID)`
- [ ] `internal/oidc/postgres/token_repo.go` — add `UPDATE device_sessions SET last_used_at = now() WHERE id = $1` after refresh token INSERT (only if `refresh.DeviceSessionID != ""`)
- [ ] `internal/oidc/postgres/token_repo.go` — update `scanRefreshToken`: add `deviceSessionID sql.NullString` to `Scan`; assign `rt.DeviceSessionID` if valid
- [ ] `internal/oidc/postgres/token_repo.go` — update `GetRefreshToken` SELECT to include `device_session_id` column
- [ ] `internal/oidc/postgres/token_repo.go` — update `GetRefreshInfo` SELECT to include `device_session_id` column (or confirm it is not needed there)
- [ ] `internal/oidc/postgres/repo_test.go` — add `TestTokenRepository_DeviceSessionIDCarriedForward`: rotation carries `device_session_id` from old to new refresh token
- [ ] `internal/oidc/postgres/repo_test.go` — add `TestTokenRepository_LastUsedAtUpdated`: `device_sessions.last_used_at` is updated atomically on token refresh

### Phase 8 — Device Session Repository

- [ ] Create `internal/oidc/postgres/device_session_repo.go`
  - [ ] `var _ oidcdom.DeviceSessionRepository = (*DeviceSessionRepository)(nil)` compile-time check
  - [ ] `NewDeviceSessionRepository(db *sqlx.DB) *DeviceSessionRepository`
  - [ ] `FindOrCreate`: SELECT by ID; if found and same user+active → UPDATE metadata; if found but different user or revoked → call `create` with new ULID; if not found → call `create` with requested ID
  - [ ] private `create` helper: INSERT single row, return populated `*DeviceSession`
  - [ ] `RevokeByID`: open transaction → `UPDATE device_sessions SET revoked_at = now() WHERE id AND user_id AND revoked_at IS NULL` (check `RowsAffected == 0` → `ErrNotFound`) → `DELETE FROM refresh_tokens WHERE device_session_id = $1` → commit
  - [ ] `ListActiveByUserID`: `SELECT ... WHERE user_id = $1 AND revoked_at IS NULL ORDER BY last_used_at DESC`
  - [ ] `DeleteRevokedBefore`: `DELETE FROM device_sessions WHERE revoked_at IS NOT NULL AND revoked_at < $1 AND NOT EXISTS (SELECT 1 FROM refresh_tokens WHERE device_session_id = device_sessions.id AND expiration > now())`
- [ ] Create `internal/oidc/postgres/device_session_repo_test.go`
  - [ ] `TestDeviceSessionRepository_Create` — unknown ID → new row created
  - [ ] `TestDeviceSessionRepository_FindExisting` — known ID + same user → `user_agent`, `last_used_at` updated; same `ID` returned
  - [ ] `TestDeviceSessionRepository_CrossUser` — known ID but different `user_id` → new row with fresh ULID returned
  - [ ] `TestDeviceSessionRepository_RevokedSession` — known ID but `revoked_at` set → new row with fresh ULID returned
  - [ ] `TestDeviceSessionRepository_RevokeByID` — sets `revoked_at` on session; linked `refresh_tokens` deleted
  - [ ] `TestDeviceSessionRepository_RevokeByID_WrongUser` — returns `domerr.ErrNotFound`; no mutation
  - [ ] `TestDeviceSessionRepository_ListActiveByUserID` — returns only non-revoked sessions, ordered `last_used_at DESC`
  - [ ] `TestDeviceSessionRepository_DeleteRevokedBefore` — does not delete when active refresh tokens remain; deletes when all tokens expired/gone

### Phase 9 — Device Session Service

- [ ] Create `internal/oidc/device_session_svc.go`
  - [ ] `var _ DeviceSessionService = (*deviceSessionService)(nil)` compile-time check
  - [ ] `NewDeviceSessionService(repo DeviceSessionRepository) DeviceSessionService`
  - [ ] Delegate all four methods to `s.repo`

### Phase 10 — OIDC Adapter

- [ ] `internal/oidc/adapter/authrequest.go` — add `GetDeviceSessionID() string` method returning `a.ar.DeviceSessionID`
- [ ] `internal/oidc/adapter/authrequest_test.go` — add test for `GetDeviceSessionID()` (returns empty string by default; returns value when set)
- [ ] `internal/oidc/adapter/storage.go` — add `extractDeviceSessionID(request op.TokenRequest) string` helper using duck-type interface `GetDeviceSessionID() string`
- [ ] `internal/oidc/adapter/storage.go` — update `CreateAccessAndRefreshTokens`: call `extractDeviceSessionID(request)`; pass result as last arg to `s.tokens.CreateAccessAndRefresh`
- [ ] `internal/oidc/adapter/storage.go` — fix `RevokeToken`: swap branches so `userID != ""` → `RevokeRefreshToken`; `userID == ""` → `Revoke`
- [ ] `internal/oidc/adapter/storage_test.go` — add `TestStorageAdapter_CreateAccessAndRefreshTokens_WithDeviceSession`: device session ID from auth request propagated to refresh token
- [ ] `internal/oidc/adapter/storage_test.go` — add `TestStorageAdapter_RevokeToken_RefreshToken`: refresh token revocation calls correct repo method
- [ ] `internal/oidc/adapter/storage_test.go` — add `TestStorageAdapter_RevokeToken_AccessToken`: access token revocation calls correct repo method

### Phase 11 — authn Package

- [ ] Create `internal/authn/ip.go` — `clientIP(r *http.Request) string` using `CF-Connecting-IP` header, falling back to `r.RemoteAddr`
- [ ] Create `internal/authn/devicename.go` — `parseDeviceName(ua string) string`, `parseOS(ua string) string`, `parseBrowser(ua string) string`
- [ ] Create `internal/authn/devicename_test.go` — table-driven tests covering: iPhone/Safari, Android/Chrome, macOS/Chrome, macOS/Firefox, macOS/Safari, macOS/Edge, Windows/Chrome, Windows/Firefox, Linux/Firefox, empty UA, and unknown UA
- [ ] `internal/authn/login_usecase.go` — split `Execute` into two methods on `CompleteFederatedLogin`:
  - [ ] `FindOrCreateUser(ctx, provider, claims) (*identity.User, error)` — identity lookup only
  - [ ] `CompleteLogin(ctx, authRequestID, userID, deviceSessionID string) error` — marks auth request done (sets `authTime = now()`, `amr = ["fed"]`)
  - [ ] Remove old `Execute` method (or keep as internal if still needed for tests — likely remove)
- [ ] `internal/authn/handler.go` — add `deviceSessions oidcdom.DeviceSessionService` field to `Handler`
- [ ] `internal/authn/handler.go` — update `NewHandler` signature to accept `deviceSessions oidcdom.DeviceSessionService` parameter
- [ ] `internal/authn/handler.go` — add package-level constants `deviceCookieName = "dsid"` and `deviceCookieMaxAge = 63072000`
- [ ] `internal/authn/handler.go` — update `SelectProvider`: if `dsid` cookie missing, generate ULID and call `http.SetCookie` with `HttpOnly: true`, `Secure: true`, `SameSite: http.SameSiteLaxMode`, `MaxAge: deviceCookieMaxAge`
- [ ] `internal/authn/handler.go` — update `FederatedCallback`: call `loginUC.FindOrCreateUser` first; read `dsid` cookie (fallback: new ULID); call `h.deviceSessions.FindOrCreate`; if `ds.ID != dsID` update cookie; call `loginUC.CompleteLogin` with `ds.ID`
- [ ] `internal/authn/handler_test.go` — add `TestSelectProvider_SetsCookieWhenMissing`
- [ ] `internal/authn/handler_test.go` — add `TestSelectProvider_KeepsExistingCookie`
- [ ] `internal/authn/handler_test.go` — add `TestFederatedCallback_CreatesDeviceSession`
- [ ] `internal/authn/handler_test.go` — add `TestFederatedCallback_UpdatesCookieOnUserMismatch`

### Phase 12 — Security Headers

- [ ] `internal/middleware/securityheaders.go` — add `LoginPageSecurityHeaders() func(http.Handler) http.Handler` with CSP: `default-src 'none'`, `style-src 'unsafe-inline'`, `script-src 'unsafe-inline'`, `img-src 'self' data: https:`, `connect-src 'none'`, `form-action 'self'`, `frame-ancestors 'none'`, `base-uri 'self'`; plus all standard headers (`X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`, `Strict-Transport-Security`)

### Phase 13 — main.go Wiring

- [ ] `main.go` — instantiate `deviceSessionRepo := oidcpg.NewDeviceSessionRepository(db)` in `runServer`
- [ ] `main.go` — instantiate `deviceSessionSvc := oidcdom.NewDeviceSessionService(deviceSessionRepo)` in `runServer`
- [ ] `main.go` — pass `deviceSessionSvc` as new parameter to `authn.NewHandler`
- [ ] `main.go` — add `go runDeviceSessionCleanup(cleanupCtx, deviceSessionSvc, 30*24*time.Hour, logger)` goroutine
- [ ] `main.go` — add `runDeviceSessionCleanup` function (24-hour ticker; calls `svc.DeleteRevokedBefore(ctx, now-maxAge)`)
- [ ] `main.go` — in `/login` route group: replace generic `appmiddleware.SecurityHeaders()` (if applied there) with `appmiddleware.LoginPageSecurityHeaders()` (note: current code applies SecurityHeaders globally via `router.Use`; the `/login` sub-router should use `LoginPageSecurityHeaders()` via its own `r.Use` call)

### Phase 14 — Test Sweep

- [ ] Run `go build ./...` from `services/accounts/` — resolve any compile errors
- [ ] Run `go vet ./...` — resolve any vet warnings
- [ ] Run `go test ./...` — all unit and integration tests pass
- [ ] Verify `testhelper/testdb.go` `CleanTables` order is correct: `refresh_tokens → tokens → auth_requests → device_sessions → federated_identities → users → clients`
- [ ] Verify no test relies on the old `Execute` method of `CompleteFederatedLogin` (update any such tests)
