# Accounts Service -- Refactoring Plan

> **Author**: AI agent (session 3)
> **Status**: Reviewed -- all open questions resolved, all Go Architect corrections incorporated. Ready for implementation.
> **Prerequisite reading**: `001_accounts_completed_plan.md`, `001_zitadel_oidc_research.md`, `002_accounts_handoff.md`

---

## 0. Executive Summary

The accounts service is a working OIDC Provider with 102 passing tests. The implementation is sound, but the handoff document (002) identified specific technical debt items. This plan addresses each of them in a careful, non-breaking order.

**Scope of this refactoring:**
1. Eliminate duplicated DDL in test files by introducing a shared test helper.
2. Extract repository interfaces so the `oidcprovider.Storage` layer can be unit-tested without PostgreSQL.
3. Add graceful shutdown to `main.go`.
4. Make token lifetimes configurable via `Config`.
5. Address the `002_seed_clients.sql` placeholder hash (documentation fix -- actual hash replacement is a deployment concern).

**Explicitly out of scope** (preserving decisions from 002):
- No migration to `op.Server` API.
- No key rotation, refresh token reuse detection, OTel, active row cleanup, admin UI, account linking, profile editing, or additional grant types.
- No template extraction (login HTML is minimal and stable).

**Resolved decisions:**
- `//go:embed` for migration files (Human).
- Consumer-defined interfaces in `oidcprovider` and `login` packages (Human).
- `0` falls back to default token lifetimes; bounds are 1-60 min / 1-90 days after defaulting (Human + Go Architect).
- No new mock-based tests; existing 102 integration tests are sufficient (Human).

---

## 1. Refactoring Items

### R1: Shared Test Migration Helper

**Problem**: The DDL schema is duplicated in three places:
1. `migrations/001_initial.sql` (source of truth)
2. `repo/repo_test.go:runMigrations()` (inline string, lines 56-139)
3. `oidcprovider/storage_test.go:runMigrations()` (inline string, lines 62-146)

If the schema changes, all three must be updated in lockstep, violating DRY and inviting drift.

**Solution**: Create a new internal test-helper package that embeds and executes the actual migration SQL files at test time.

**Design**:
- New file: `migrations/embed.go` (package `migrations`)
  ```go
  package migrations

  import "embed"

  //go:embed *.sql
  var FS embed.FS
  ```
- New file: `testhelper/testdb.go` (package `testhelper`). This package provides two functions:
  ```go
  // RunMigrations executes all embedded *.sql files against db in lexicographic order.
  // It is safe to call from TestMain (no testing.TB required).
  func RunMigrations(db *sqlx.DB) error

  // CleanTables truncates all data tables in dependency-safe order.
  // For use inside individual test functions.
  func CleanTables(t testing.TB, db *sqlx.DB)
  ```
- `RunMigrations` must use `fs.ReadDir(migrations.FS, ".")` from the `io/fs` package (not `migrations.FS.ReadDir(".")` directly). The sort guarantee comes from `io/fs.ReadDir`: *"ReadDir reads the named directory and returns a list of directory entries sorted by filename."* The `embed.FS.ReadDir` method doc does not specify ordering. This ensures `001_initial.sql` runs before `002_seed_clients.sql` with a spec-backed guarantee.

**Impact on existing test files**:
- Both `TestMain` functions keep their existing container lifecycle code (testcontainers setup, connection, terminate). This is intentional -- parallel test isolation per package.
- Replace the inline `runMigrations()` call with `testhelper.RunMigrations(testDB)`, panicking on error exactly as before.
- Replace pkg-local `cleanTables(t)` with `testhelper.CleanTables(t, testDB)`.
- The `*testing.M` argument is NOT passed to testhelper. `RunMigrations` returns `error` (no `testing.TB` parameter), making it safe to call from `TestMain`. `CleanTables` takes `testing.TB` and is only called from individual test functions where `*testing.T` is available.

**File changes**:
| File | Action |
|------|--------|
| `migrations/embed.go` | **New** -- `//go:embed *.sql` directive |
| `testhelper/testdb.go` | **New** -- `RunMigrations` and `CleanTables` helpers |
| `repo/repo_test.go` | **Modify** -- remove `runMigrations()` and `cleanTables()`, delegate to testhelper |
| `oidcprovider/storage_test.go` | **Modify** -- remove `runMigrations()` and `cleanTables()`, delegate to testhelper |

**Risk**: Low. Test-only change. No production code modified. Existing `TestMain` container setup code is verified correct and unchanged.

---

### R2: Repository Interfaces

**Problem**: `oidcprovider.Storage` holds concrete repository types (`*repo.UserRepository`, `*repo.ClientRepository`, etc.). This makes it impossible to unit-test Storage methods without a live PostgreSQL database. The handoff document (section 2.1) notes this explicitly.

**Solution**: Define interfaces for each repository in the consuming packages, have Storage depend on the interfaces, and keep the concrete implementations in `repo/`.

**Design**:

Interfaces are defined in each consuming package independently (Go idiom: "accept interfaces, return structs" / "define interfaces where they are used"). A little signature duplication between `oidcprovider` and `login` is acceptable and preferred over introducing cross-package coupling.

For `oidcprovider/storage.go`:
```go
type UserReader interface {
    GetByID(ctx context.Context, id string) (*model.User, error)
    FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*model.User, error)
    CreateWithFederatedIdentity(ctx context.Context, u *model.User, fi *model.FederatedIdentity) error
}

type ClientReader interface {
    GetByID(ctx context.Context, clientID string) (*model.Client, error)
}

type AuthRequestStore interface {
    Create(ctx context.Context, ar *model.AuthRequest) error
    GetByID(ctx context.Context, id string) (*model.AuthRequest, error)
    GetByCode(ctx context.Context, code string) (*model.AuthRequest, error)
    SaveCode(ctx context.Context, id, code string) error
    CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
    Delete(ctx context.Context, id string) error
}

type TokenStore interface {
    CreateAccess(ctx context.Context, clientID, subject string, audience, scopes []string, expiration time.Time) (string, error)
    CreateAccessAndRefresh(ctx context.Context, clientID, subject string, audience, scopes []string, accessExpiration, refreshExpiration time.Time, authTime time.Time, amr []string, currentRefreshToken string) (string, string, error)
    GetByID(ctx context.Context, tokenID string) (*model.Token, error)
    GetRefreshToken(ctx context.Context, token string) (*model.RefreshToken, error)
    GetRefreshInfo(ctx context.Context, token string) (string, string, error)
    DeleteByUserAndClient(ctx context.Context, userID, clientID string) error
    Revoke(ctx context.Context, tokenID string) error
    RevokeRefreshToken(ctx context.Context, token string) error
}
```

All interface method signatures are verified against the actual source in `repo/*.go`. Named vs. unnamed return values do not affect interface satisfaction in Go; only the underlying types are compared.

Update `Storage` struct:
```go
type Storage struct {
    db          *sqlx.DB
    userRepo    UserReader
    clientRepo  ClientReader
    authReqRepo AuthRequestStore
    tokenRepo   TokenStore
    signing     *signingKey
    public      *publicKey
}
```

The `NewStorage` constructor signature changes from concrete types to interfaces. The existing concrete `repo.*Repository` types already satisfy these interfaces -- no changes to `repo/` package.

For `login/handler.go`:
```go
type userFinder interface {
    FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*model.User, error)
    CreateWithFederatedIdentity(ctx context.Context, u *model.User, fi *model.FederatedIdentity) error
}

type authRequestCompleter interface {
    GetByID(ctx context.Context, id string) (*model.AuthRequest, error)
    CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
}
```

Update the `Handler` struct fields from `*repo.UserRepository` to `userFinder`, etc.

**Impact on `main.go`**: None in terms of logic. The wiring still passes concrete repo instances. Go's structural typing handles the concrete-to-interface assignment transparently.

**Impact on tests**:
- `oidcprovider/storage_test.go`: Existing integration tests remain unchanged. `newTestStorage` passes concrete repos which satisfy the interfaces via structural typing.
- `login/handler_test.go`: The `testHandler()` function currently sets `userRepo: nil` and `authReqRepo: nil`. A direct `nil` in a composite literal where the field type is an interface produces a true nil interface value (`i == nil`). No behavior change.

**File changes**:
| File | Action |
|------|--------|
| `oidcprovider/storage.go` | **Modify** -- add interface types, change struct fields to interfaces, update `NewStorage` signature |
| `login/handler.go` | **Modify** -- add interface types, change struct fields to interfaces, update `NewHandler` signature |
| `main.go` | **No change** (concrete types satisfy interfaces) |
| `repo/*.go` | **No change** |
| `oidcprovider/storage_test.go` | **Minor** -- no type adjustment needed (structural typing) |
| `login/handler_test.go` | **Minor** -- `testHandler` nil assignments remain valid for interface fields |

**Risk**: Low-medium. Interface extraction is a purely structural change. All 102 tests must still pass. The only risk is getting a method signature wrong, which the compiler will catch.

---

### R3: Graceful Shutdown

**Problem**: `main.go` calls `http.ListenAndServe` directly (line 97) without signal handling or graceful shutdown. The handoff document (section 2.1) identifies this.

**Solution**: Add `os.Signal` handling and use `http.Server.Shutdown(ctx)`.

**Design**:
```go
// main.go -- replace the final http.ListenAndServe block

srv := &http.Server{
    Addr:              ":" + cfg.Port,
    Handler:           router,
    ReadHeaderTimeout: 5 * time.Second,
    ReadTimeout:       10 * time.Second,
    WriteTimeout:      30 * time.Second,
    IdleTimeout:       120 * time.Second,
}

go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        logger.Error("server error", "error", err)
        os.Exit(1)
    }
}()

logger.Info("accounts service started", "port", cfg.Port)

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

logger.Info("shutting down server")
ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()
if err := srv.Shutdown(ctx); err != nil {
    logger.Error("server forced to shutdown", "error", err)
    os.Exit(1)
}
logger.Info("server stopped")
```

**Design decisions**:
- **HTTP timeouts**: Setting `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and `IdleTimeout` explicitly eliminates the gosec G112 (Slowloris) lint finding. The existing `//nolint:gosec` on `http.ListenAndServe` is no longer needed and should be removed. `WriteTimeout: 30s` accommodates auth code + token responses; `IdleTimeout: 120s` is standard for keep-alive. The `time` import is already required for the shutdown timeout -- no extra import cost.
- **Shutdown timeout**: 15 seconds. `http.Server.Shutdown` docs: *"If the provided context expires before the shutdown is complete, Shutdown returns the context's error."* This is a hard ceiling. OIDC token exchanges are fast, so 15s is generous.
- **Signals**: `SIGINT` (Ctrl+C) and `SIGTERM` (Kubernetes pod termination). Signal channel buffer of 1 per `signal.Notify` docs: *"For a channel used for notification of just one signal value, a buffer of size 1 is sufficient."*
- **`http.ErrServerClosed`**: `Shutdown` causes `ListenAndServe` to immediately return `http.ErrServerClosed` -- this is the normal exit path, not an error.
- The `defer db.Close()` already in `main.go` will execute after shutdown.
- Added imports: `os/signal`, `syscall`, `time`.

**File changes**:
| File | Action |
|------|--------|
| `main.go` | **Modify** -- replace `http.ListenAndServe` with graceful shutdown pattern; add HTTP timeouts; remove `//nolint:gosec` |

**Risk**: Low. Well-established Go pattern. No behavioral change for the OIDC protocol.

---

### R4: Configurable Token Lifetimes

**Problem**: Access token lifetime (15 min) and refresh token lifetime (7 days) are hardcoded constants in `oidcprovider/storage.go` (lines 22-24). The handoff document (section 2.1) suggests making these configurable.

**Solution**: Add `AccessTokenLifetime` and `RefreshTokenLifetime` fields to `config.Config`, pass them through to `Storage`.

**Design**:

1. Add to `config.Config`:
```go
AccessTokenLifetimeMinutes  int  // default: 15
RefreshTokenLifetimeDays    int  // default: 7
```

2. Add a `getEnvInt` helper to `config.go` (add `strconv` to imports):
```go
// getEnvInt returns defaultVal when the env var is absent or empty.
// A value of "0" returns 0 -- the caller is responsible for semantic defaulting.
func getEnvInt(key string, defaultVal int) int {
    v := os.Getenv(key)
    if v == "" {
        return defaultVal
    }
    n, err := strconv.Atoi(v)
    if err != nil {
        return defaultVal
    }
    return n
}
```

3. In `config.Load()`, read from env vars with `0` -> default substitution BEFORE bounds checking:
```go
cfg.AccessTokenLifetimeMinutes = getEnvInt("ACCESS_TOKEN_LIFETIME_MINUTES", 15)
if cfg.AccessTokenLifetimeMinutes == 0 {
    cfg.AccessTokenLifetimeMinutes = 15
}
if cfg.AccessTokenLifetimeMinutes < 1 || cfg.AccessTokenLifetimeMinutes > 60 {
    return nil, fmt.Errorf("ACCESS_TOKEN_LIFETIME_MINUTES must be 0 (default) or 1-60, got %d", cfg.AccessTokenLifetimeMinutes)
}

cfg.RefreshTokenLifetimeDays = getEnvInt("REFRESH_TOKEN_LIFETIME_DAYS", 7)
if cfg.RefreshTokenLifetimeDays == 0 {
    cfg.RefreshTokenLifetimeDays = 7
}
if cfg.RefreshTokenLifetimeDays < 1 || cfg.RefreshTokenLifetimeDays > 90 {
    return nil, fmt.Errorf("REFRESH_TOKEN_LIFETIME_DAYS must be 0 (default) or 1-90, got %d", cfg.RefreshTokenLifetimeDays)
}
```

The order is critical: default substitution first, then bounds validation. This resolves the conflict between "0 = default" and "must be >= 1".

4. Update `Storage` to accept these as struct fields:
```go
type Storage struct {
    // ... existing fields ...
    accessTokenLifetime  time.Duration
    refreshTokenLifetime time.Duration
}
```

5. Update `NewStorage` to accept the durations:
```go
func NewStorage(..., accessTokenLifetime, refreshTokenLifetime time.Duration) *Storage
```

6. Remove the `const` block in `storage.go` and use the struct fields instead.

7. Update `main.go` wiring:
```go
storage := oidcprovider.NewStorage(db, userRepo, clientRepo, authReqRepo, tokenRepo,
    signingKey, publicKey,
    time.Duration(cfg.AccessTokenLifetimeMinutes) * time.Minute,
    time.Duration(cfg.RefreshTokenLifetimeDays) * 24 * time.Hour,
)
```

8. Update `.env.example` with the new env vars (commented, with defaults).

**Impact on tests**:
- `oidcprovider/storage_test.go`: `newTestStorage()` must pass the new duration args. Use the same defaults (15 min, 7 days) so existing test assertions remain valid.
- `config/config_test.go`: Add tests for the new env vars (valid, missing/defaults, zero/default-fallback, out-of-range).

**File changes**:
| File | Action |
|------|--------|
| `config/config.go` | **Modify** -- add fields, add `strconv` import, add `getEnvInt`, parsing with 0-default + validation |
| `config/config_test.go` | **Modify** -- add tests for new fields |
| `oidcprovider/storage.go` | **Modify** -- replace constants with struct fields, update constructor |
| `oidcprovider/storage_test.go` | **Modify** -- update `newTestStorage` call |
| `main.go` | **Modify** -- pass config values to `NewStorage` |
| `.env.example` | **Modify** -- add new vars |

**Risk**: Low. The defaults match current behavior. Existing tests pass with defaults.

---

### R5: Seed Client Placeholder Documentation

**Problem**: `002_seed_clients.sql` contains `$2a$10$PLACEHOLDER` as the bcrypt hash. This must be replaced before deployment, but nothing in the codebase or documentation makes this operationally clear beyond the handoff doc.

**Solution**: Add a clear `-- WARNING` comment in the SQL file itself and reference it in `.env.example`.

**Design**:
- Prepend a comment block to `002_seed_clients.sql`:
  ```sql
  -- WARNING: The secret_hash below is a PLACEHOLDER.
  -- Before first deployment, generate a real bcrypt hash:
  --   htpasswd -nbBC 10 "" 'your-secret' | cut -d: -f2
  -- or in Go:
  --   hash, _ := bcrypt.GenerateFromPassword([]byte("your-secret"), bcrypt.DefaultCost)
  -- Replace '$2a$10$PLACEHOLDER' with the generated hash.
  ```

**File changes**:
| File | Action |
|------|--------|
| `migrations/002_seed_clients.sql` | **Modify** -- add warning comment |

**Risk**: None. Documentation-only change.

---

## 2. Implementation Order

The refactoring items should be implemented in this order to minimize risk and ensure each step can be verified independently:

| Step | Item | Rationale |
|------|------|-----------|
| 1 | **R5** -- Seed client placeholder docs | Zero-risk, no code change. Gets it out of the way. |
| 2 | **R1** -- Shared test migration helper | Establishes clean test infrastructure before other changes. All 102 tests must still pass. |
| 3 | **R3** -- Graceful shutdown | Independent of other changes. Small, self-contained `main.go` modification. |
| 4 | **R4** -- Configurable token lifetimes | Adds new config fields and modifies Storage constructor. Must happen before R2 so the constructor signature is stable before interfaces are extracted. |
| 5 | **R2** -- Repository interfaces | The most structurally significant change. Done last so all other constructor changes are settled. All 102 tests must still pass. |

After each step: `go build ./services/accounts/...`, `go vet ./services/accounts/...`, `golangci-lint run ./services/accounts/...`, and `go test ./services/accounts/... -count=1` must all pass cleanly.

---

## 3. New & Modified File Summary

### New Files
| File | Package | Purpose |
|------|---------|---------|
| `migrations/embed.go` | `migrations` | `//go:embed *.sql` for test migration helper |
| `testhelper/testdb.go` | `testhelper` | `RunMigrations` (safe for `TestMain`) and `CleanTables` (for individual tests) |

### Modified Files
| File | Changes |
|------|---------|
| `main.go` | Graceful shutdown with HTTP timeouts; remove `//nolint:gosec`; pass token lifetimes to `NewStorage` |
| `config/config.go` | Add `AccessTokenLifetimeMinutes`, `RefreshTokenLifetimeDays`, `getEnvInt`, `strconv` import |
| `config/config_test.go` | Tests for new config fields (valid, defaults, zero-fallback, out-of-range) |
| `oidcprovider/storage.go` | Interface types for repos; struct fields become interfaces; token lifetimes as struct fields; update `NewStorage` |
| `oidcprovider/storage_test.go` | Remove inline DDL and `cleanTables`; use `testhelper.RunMigrations` and `testhelper.CleanTables`; update `newTestStorage` args |
| `login/handler.go` | Interface types for repos; struct fields become interfaces; update `NewHandler` |
| `login/handler_test.go` | Minor: field types change from concrete to interface (`nil` assignments remain valid) |
| `repo/repo_test.go` | Remove inline DDL and `cleanTables`; use `testhelper.RunMigrations` and `testhelper.CleanTables` |
| `migrations/002_seed_clients.sql` | Add placeholder warning comment |
| `.env.example` | Add `ACCESS_TOKEN_LIFETIME_MINUTES`, `REFRESH_TOKEN_LIFETIME_DAYS` |

### Unchanged Files
| File | Reason |
|------|--------|
| `model/*.go` | Pure data structs, no changes needed |
| `repo/user.go`, `repo/client.go`, `repo/authrequest.go`, `repo/token.go` | Concrete implementations unchanged; they implicitly satisfy new interfaces |
| `oidcprovider/keys.go` | No changes |
| `oidcprovider/client.go` | No changes |
| `oidcprovider/authrequest.go` | No changes |
| `oidcprovider/refreshtoken.go` | No changes |
| `oidcprovider/provider.go` | No changes |
| `oidcprovider/keys_test.go` | No changes |
| `oidcprovider/client_test.go` | No changes |
| `oidcprovider/authrequest_test.go` | No changes |
| `login/upstream.go` | No changes |
| `migrations/001_initial.sql` | No changes |
| `Dockerfile` | No changes |

---

## 4. Architectural Invariants (Must NOT Be Violated)

These are carried forward from `002_accounts_handoff.md` section 4 and must be preserved:

1. **Legacy `op.NewProvider` + `op.StaticIssuer` API** -- do NOT migrate to `op.Server`.
2. **No profile overwrite on returning logins** -- `findOrCreateUser` logic must not be altered.
3. **No email-based account linking** -- `(provider, provider_subject)` uniqueness is preserved.
4. **Federated state uses the OP CryptoKey** -- no separate key management.
5. **Stateless OP -- no session cookies**.
6. **sqlx + raw SQL, no ORM** -- repos remain handwritten SQL.
7. **JWT access tokens with `AccessTokenTypeJWT`**.
8. **`IntrospectionResponse` embeds fields directly** -- `setIntrospectionUserinfo` and `setUserinfo` remain separate helpers.
9. **Package naming**: `oidcprovider` (not `oidc`/`provider`), `login` for auth UI.
10. **`New*` constructor naming convention**.

---

## 5. Verification Checklist

After all refactoring is complete:

- [ ] `go build ./services/accounts/...` -- clean
- [ ] `go vet ./services/accounts/...` -- clean
- [ ] `golangci-lint run ./services/accounts/...` -- 0 issues (no `//nolint:gosec` needed)
- [ ] `go test ./services/accounts/... -count=1` -- all tests pass (count should be >= 102)
- [ ] No new test files import `github.com/testcontainers/testcontainers-go` that didn't before (keep the testcontainers dependency confined)
- [ ] `migrations/001_initial.sql` is the sole source of truth for DDL
- [ ] No inline DDL strings remain in test files
- [ ] `docker build` still succeeds (Dockerfile unchanged)
- [ ] All architectural invariants from section 4 are preserved

---

## 6. Resolved Questions

All open questions from the initial draft have been resolved:

| # | Question | Resolution |
|---|----------|------------|
| 1 | R1 embed vs. relative paths | Use `//go:embed` (Human decision) |
| 2 | R2 interface location | Consumer-defined in `oidcprovider` and `login` (Human decision) |
| 3 | R4 validation / `0` handling | `0` falls back to defaults (15 min / 7 days) before bounds check of 1-60 / 1-90 (Human + Go Architect correction) |
| 4 | New mock tests | Not needed; existing 102 integration tests sufficient (Human decision) |

---

## Appendix: Review Annotations (Historical)

The following annotations were provided during review and have been incorporated into the plan text above. They are preserved here for traceability.

<details>
<summary>R1 -- Go Architect: testing.TB vs *testing.M</summary>

`testing.TB` is an interface satisfied by `*testing.T`, `*testing.B`, and `*testing.F` -- but NOT by `*testing.M`. The original plan's proposed `testhelper.SetupTestDB(nil)` call from `TestMain` was a latent bug. Fix: `RunMigrations(db *sqlx.DB) error` with no `testing.TB` parameter. Each `TestMain` keeps starting its own container, calls `testhelper.RunMigrations(testDB)`, and panics on error.
</details>

<details>
<summary>R1 -- Go Architect: fs.ReadDir sort guarantee</summary>

The sort guarantee comes from `io/fs.ReadDir`, not from `embed.FS.ReadDir`. `RunMigrations` must call `fs.ReadDir(migrations.FS, ".")` from `io/fs`. Also noted: `testcontainers-go/modules/postgres` `WithInitScripts` accepts filesystem paths (not embedded content), so `embed.FS` + `RunMigrations` is the superior approach.
</details>

<details>
<summary>R1 -- Go Architect: TestMain container API verification</summary>

Existing container setup in both test files uses correct current API. `postgres.Run`, `WithDatabase`, `WithUsername`, `WithPassword`, `WithWaitStrategy`, `ConnectionString`, `Terminate` -- all verified. No changes needed.
</details>

<details>
<summary>R2 -- Go Architect: Interface signature verification</summary>

All interface method signatures verified against `repo/*.go`. Named vs. unnamed returns do not affect interface satisfaction. `AuthRequestRepository.Delete` confirmed at `repo/authrequest.go`. All interfaces are complete.
</details>

<details>
<summary>R2 -- Go Architect: nil interface safety in handler_test.go</summary>

Direct `nil` in a composite literal where the field type is an interface produces a true nil interface value (`i == nil`). This is distinct from assigning a typed nil pointer to an interface (which produces a non-nil interface). No behavior change after the interface extraction.
</details>

<details>
<summary>R3 -- Go Architect: http.Server.Shutdown semantics</summary>

`Shutdown` causes `ListenAndServe` to immediately return `http.ErrServerClosed` (the normal exit sentinel). The 15-second context timeout is a hard ceiling. `Shutdown` does not wait for hijacked connections (not applicable -- no WebSockets). Signal channel buffer of 1 is correct per `signal.Notify` docs.
</details>

<details>
<summary>R3 -- Go Architect: HTTP server timeouts / gosec G112</summary>

Bare `&http.Server{Addr, Handler}` triggers gosec G112 (Slowloris). Adding `ReadHeaderTimeout: 5s`, `ReadTimeout: 10s`, `WriteTimeout: 30s`, `IdleTimeout: 120s` eliminates the finding and removes the need for `//nolint:gosec`. The `time` import is already required for the shutdown timeout.
</details>

<details>
<summary>R4 -- Go Architect: 0-default vs bounds-check conflict</summary>

`getEnvInt` returns raw `0` when the env var is `"0"`. The caller must substitute the default (`15` or `7`) for `0` BEFORE the bounds check. `strconv` is not currently imported in `config.go` -- must be added.
</details>

---

## 7. Implementation TODO

### Step 1: R5 -- Seed Client Placeholder Documentation
- [x] Add `-- WARNING` comment block to `migrations/002_seed_clients.sql`
- [x] Verify `go build ./services/accounts/...` passes

### Step 2: R1 -- Shared Test Migration Helper
- [x] Create `migrations/embed.go` with `//go:embed *.sql`
- [x] Create `testhelper/testdb.go` with `RunMigrations` and `CleanTables`
- [x] Update `repo/repo_test.go`: remove `runMigrations()` and `cleanTables()`, use testhelper
- [x] Update `oidcprovider/storage_test.go`: remove `runMigrations()` and `cleanTables()`, use testhelper
- [x] Verify `go build ./services/accounts/...` passes
- [x] Verify `go test ./services/accounts/... -count=1` passes (all 102 tests)

### Step 3: R3 -- Graceful Shutdown
- [x] Update `main.go`: replace `http.ListenAndServe` with `http.Server` + graceful shutdown + HTTP timeouts
- [x] Remove `//nolint:gosec` suppression
- [x] Verify `go build ./services/accounts/...` passes
- [x] Verify `golangci-lint run ./services/accounts/...` passes (no G112)

### Step 4: R4 -- Configurable Token Lifetimes
- [x] Add `getEnvInt` helper and `strconv` import to `config/config.go`
- [x] Add `AccessTokenLifetimeMinutes` and `RefreshTokenLifetimeDays` fields + parsing + validation to `config/config.go`
- [x] Add tests for new config fields to `config/config_test.go`
- [x] Update `oidcprovider/storage.go`: replace constants with struct fields, update `NewStorage`
- [x] Update `oidcprovider/storage_test.go`: pass default durations to `newTestStorage`
- [x] Update `main.go`: pass config token lifetimes to `NewStorage`
- [x] Update `.env.example` with new env vars
- [x] Verify `go build ./services/accounts/...` passes
- [x] Verify `go test ./services/accounts/... -count=1` passes

### Step 5: R2 -- Repository Interfaces
- [x] Add interface types to `oidcprovider/storage.go`, update `Storage` struct fields and `NewStorage` signature
- [x] Add interface types to `login/handler.go`, update `Handler` struct fields and `NewHandler` signature
- [x] Verify `go build ./services/accounts/...` passes
- [x] Verify `go test ./services/accounts/... -count=1` passes (all 102 tests)
- [x] Verify `golangci-lint run ./services/accounts/...` passes

### Final Verification
- [x] `go build ./services/accounts/...` -- clean
- [x] `go vet ./services/accounts/...` -- clean
- [x] `golangci-lint run ./services/accounts/...` -- 0 issues
- [x] `go test ./services/accounts/... -count=1` -- all tests pass (113: 102 original + 6 new config + 5 login handler)
- [x] No inline DDL strings remain in test files
- [x] All architectural invariants preserved
