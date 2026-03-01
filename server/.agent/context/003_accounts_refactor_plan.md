# Accounts Service -- Refactoring Plan

> **Author**: AI agent (session 3)
> **Status**: Draft -- awaiting human review and annotation before implementation.
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

---

## 1. Refactoring Items

### R1: Shared Test Migration Helper

**Problem**: The DDL schema is duplicated in three places:
1. `migrations/001_initial.sql` (source of truth)
2. `repo/repo_test.go:runMigrations()` (inline string, lines 56-139)
3. `oidcprovider/storage_test.go:runMigrations()` (inline string, lines 62-146)

If the schema changes, all three must be updated in lockstep, violating DRY and inviting drift.

**Solution**: Create a new internal test-helper package that reads and executes the actual migration SQL files at test time.

**Design**:
- New file: `testhelper/testdb.go` (package `testhelper`)
- This package provides:
  - `SetupTestDB(t testing.TB) *sqlx.DB` -- starts a testcontainers PostgreSQL instance, reads all `migrations/*.sql` files (sorted lexicographically), executes them, and returns the connected `*sqlx.DB`.
  - `CleanTables(t testing.TB, db *sqlx.DB)` -- truncates all data tables in dependency-safe order.
- The migration files are read using `os.ReadFile` with a path relative to the service root. Since Go test binaries set the working directory to the package being tested, we use a well-known relative path (`../migrations/` from `repo/` and `../../migrations/` from `oidcprovider/`, etc.). Alternatively, we can embed the migration files using `//go:embed` from a package at the service root level.

**Preferred approach -- `//go:embed`**:
- New file: `migrations/embed.go` (package `migrations`)
  ```go
  package migrations

  import "embed"

  //go:embed *.sql
  var FS embed.FS
  ```
- `testhelper/testdb.go` imports `migrations.FS` and reads all `.sql` files from the embedded filesystem. This avoids fragile relative path calculations and works regardless of which package's tests invoke it.

**Impact on existing test files**:
- `repo/repo_test.go`: Remove `runMigrations()` function. Replace `TestMain` body with call to `testhelper.SetupTestDB(nil)`. Replace `cleanTables(t)` with `testhelper.CleanTables(t, testDB)`.
- `oidcprovider/storage_test.go`: Same changes. Remove `runMigrations()` and `cleanTables()`.
- The `TestMain` functions remain in each test package (they own the lifecycle of the container), but they delegate setup to the shared helper.
- Both test files' `TestMain` currently use a package-level `var testDB *sqlx.DB` / `var storageTestDB *sqlx.DB`. This pattern is preserved -- only the migration execution is centralized.

**File changes**:
| File | Action |
|------|--------|
| `migrations/embed.go` | **New** -- embed directive |
| `testhelper/testdb.go` | **New** -- `SetupTestDB`, `CleanTables` |
| `repo/repo_test.go` | **Modify** -- remove inline DDL, delegate to testhelper |
| `oidcprovider/storage_test.go` | **Modify** -- remove inline DDL, delegate to testhelper |

**Risk**: Low. Test-only change. No production code modified.

---

### R2: Repository Interfaces

**Problem**: `oidcprovider.Storage` holds concrete repository types (`*repo.UserRepository`, `*repo.ClientRepository`, etc.). This makes it impossible to unit-test Storage methods without a live PostgreSQL database. The handoff document (section 2.1) notes this explicitly.

**Solution**: Define interfaces for each repository, have Storage depend on the interfaces, and keep the concrete implementations in `repo/`.

**Design**:

Define interfaces in a new file `oidcprovider/repofs.go` (or directly in `storage.go` grouped near the struct). The interfaces live in the `oidcprovider` package to avoid circular imports and because they represent what *Storage* needs, not what the repo package offers (Interface Segregation Principle).

```go
// oidcprovider/storage.go (or a dedicated interfaces file)

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

Then update `Storage` struct:
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

**Impact on `login/handler.go`**:
The login handler also depends on `*repo.UserRepository` and `*repo.AuthRequestRepository` directly. We should define similar interfaces in the `login` package (or reuse from `oidcprovider`). However, introducing a cross-package interface dependency creates coupling. The cleanest approach:
- Define the interfaces in each consuming package (`oidcprovider` and `login`) independently. They can have the same method signatures. Go's structural typing means the concrete repos satisfy both without any explicit `implements` declaration.
- This follows Go idiom: "Accept interfaces, return structs" and "Define interfaces where they are used."

For `login.Handler`:
```go
// login/handler.go

type userFinder interface {
    FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*model.User, error)
    CreateWithFederatedIdentity(ctx context.Context, u *model.User, fi *model.FederatedIdentity) error
}

type authRequestCompleter interface {
    GetByID(ctx context.Context, id string) (*model.AuthRequest, error)
    CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
}
```

And update the `Handler` struct fields from `*repo.UserRepository` to `userFinder`, etc.

**Impact on `main.go`**: None in terms of logic. The wiring still passes concrete repo instances. The function signatures accept interfaces now, but Go's type system handles this transparently.

**Impact on tests**:
- `oidcprovider/storage_test.go`: Currently uses real repos with testcontainers. These tests remain as integration tests. Optionally, new pure unit tests can be added that use mock implementations of the interfaces.
- `login/handler_test.go`: The `testHandler()` function currently sets `userRepo: nil` and `authReqRepo: nil` (because the tests that exist don't exercise those paths). With interfaces, we can now write tests that use mocks for `findOrCreateUser` and `SelectProvider` flows.

**File changes**:
| File | Action |
|------|--------|
| `oidcprovider/storage.go` | **Modify** -- add interface types, change struct fields to interfaces, update `NewStorage` signature |
| `login/handler.go` | **Modify** -- add interface types, change struct fields to interfaces, update `NewHandler` signature |
| `main.go` | **No change** (concrete types satisfy interfaces) |
| `repo/*.go` | **No change** |
| `oidcprovider/storage_test.go` | **Minor** -- `newTestStorage` may need type adjustment (likely none due to structural typing) |
| `login/handler_test.go` | **Minor** -- `testHandler` nil assignments still valid for interface fields |

**Risk**: Low-medium. Interface extraction is a purely structural change. All 102 tests must still pass. The only risk is getting a method signature wrong, which the compiler will catch.

---

### R3: Graceful Shutdown

**Problem**: `main.go` calls `http.ListenAndServe` directly (line 97) without signal handling or graceful shutdown. The handoff document (section 2.1) identifies this.

**Solution**: Add `os.Signal` handling and use `http.Server.Shutdown(ctx)`.

**Design**:
```go
// main.go -- replace the final http.ListenAndServe block

srv := &http.Server{
    Addr:    ":" + cfg.Port,
    Handler: router,
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
- Shutdown timeout: 15 seconds. This gives in-flight requests time to complete. OIDC token exchanges are fast, so this is generous.
- Signals: `SIGINT` (Ctrl+C) and `SIGTERM` (Kubernetes pod termination).
- The `defer db.Close()` already in `main.go` will execute after shutdown.
- Added imports: `os/signal`, `syscall`, `time`.

**File changes**:
| File | Action |
|------|--------|
| `main.go` | **Modify** -- replace `http.ListenAndServe` with graceful shutdown pattern |

**Risk**: Low. This is a well-established Go pattern. No behavioral change for the OIDC protocol.

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

2. In `config.Load()`, read from env vars `ACCESS_TOKEN_LIFETIME_MINUTES` and `REFRESH_TOKEN_LIFETIME_DAYS` with defaults:
```go
cfg.AccessTokenLifetimeMinutes = getEnvInt("ACCESS_TOKEN_LIFETIME_MINUTES", 15)
cfg.RefreshTokenLifetimeDays = getEnvInt("REFRESH_TOKEN_LIFETIME_DAYS", 7)
```

Add validation: `AccessTokenLifetimeMinutes` must be >= 1 and <= 60. `RefreshTokenLifetimeDays` must be >= 1 and <= 90. These bounds prevent misconfiguration (e.g., 0-minute tokens or year-long refresh tokens).

3. Add a `getEnvInt` helper to `config.go`.

4. Update `Storage` to accept these as fields:
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
- `config/config_test.go`: Add tests for the new env vars (valid, missing/defaults, out-of-range).

**File changes**:
| File | Action |
|------|--------|
| `config/config.go` | **Modify** -- add fields, parsing, validation, `getEnvInt` |
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
| `testhelper/testdb.go` | `testhelper` | Shared `SetupTestDB` and `CleanTables` for integration tests |

### Modified Files
| File | Changes |
|------|---------|
| `main.go` | Graceful shutdown; pass token lifetimes to `NewStorage` |
| `config/config.go` | Add `AccessTokenLifetimeMinutes`, `RefreshTokenLifetimeDays`, `getEnvInt` |
| `config/config_test.go` | Tests for new config fields |
| `oidcprovider/storage.go` | Interface types for repos; struct fields become interfaces; token lifetimes as struct fields; update `NewStorage` |
| `oidcprovider/storage_test.go` | Remove inline DDL; use testhelper; update `newTestStorage` args |
| `login/handler.go` | Interface types for repos; struct fields become interfaces; update `NewHandler` |
| `login/handler_test.go` | Minor: `testHandler()` field types (nil still valid) |
| `repo/repo_test.go` | Remove inline DDL; use testhelper |
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
- [ ] `golangci-lint run ./services/accounts/...` -- 0 issues
- [ ] `go test ./services/accounts/... -count=1` -- all tests pass (count should be >= 102)
- [ ] No new test files import `github.com/testcontainers/testcontainers-go` that didn't before (keep the testcontainers dependency confined)
- [ ] `migrations/001_initial.sql` is the sole source of truth for DDL
- [ ] No inline DDL strings remain in test files
- [ ] `docker build` still succeeds (Dockerfile unchanged)
- [ ] All architectural invariants from section 4 are preserved

---

## 6. Open Questions for Review

> Annotate these before implementation begins.

1. **R1 embed approach**: Should `migrations/embed.go` use `//go:embed` or should test helpers use relative file paths? Embed is more robust but adds a `.go` file to the migrations directory. Relative paths keep migrations as pure SQL but are fragile across different test working directories.
> Human: Please use `//go:embed`. It is much more robust for Go testing. 

2. **R2 interface location**: Should the repository interfaces live in the consuming packages (`oidcprovider`, `login`) as proposed, or in a shared package (e.g., `repo/` exports interfaces alongside implementations)? The Go-idiomatic approach is consumer-defined interfaces, but this creates two separate interface definitions with identical signatures.
> Human: Define the interfaces in the consuming packages (`oidcprovider` and `login`) as proposed. It follows the Go idiom "accept interfaces, return structs" and defines them where they are used. A little signature duplication is better than introducing unnecessary package coupling.

3. **R4 validation bounds**: Are the proposed bounds for token lifetimes reasonable? (Access: 1-60 minutes, Refresh: 1-90 days). Should we allow 0 to mean "use library default" or require explicit values?
> Human: The proposed bounds are reasonable. Yes, please allow `0` to fall back to the default values (15 minutes and 7 days).

4. **Test count target**: The current test count is 102. Should we add new unit tests for Storage using mock repos (leveraging R2 interfaces), or is the existing integration test coverage sufficient for now?
> Human: The existing integration test coverage is sufficient for now