# Accounts Service -- Future Improvements Memo

> **Written after**: Refactoring session (003) completed all 5 items (R1-R5).
> **Prerequisite reading**: `002_accounts_handoff.md`, `003_accounts_refactor_plan.md`

---

## 1. Baseline Achieved

The accounts service is a fully implemented OIDC Provider with the following post-refactoring state:

- **113 tests passing** (102 original + 6 new config tests + 5 login handler tests).
- **All CI checks clean**: `go build`, `go vet`, `golangci-lint run` (0 issues, no `//nolint` suppressions).
- **Single source of truth for DDL**: Migration SQL files are embedded via `//go:embed` and executed by a shared `testhelper` package. No inline DDL remains in test files.
- **Consumer-defined repository interfaces**: `oidcprovider.Storage` and `login.Handler` depend on interfaces (`UserReader`, `ClientReader`, `AuthRequestStore`, `TokenStore`, `userFinder`, `authRequestCompleter`), not concrete `*repo.*` types. The concrete repos satisfy these via Go structural typing.
- **Graceful shutdown**: `main.go` uses `http.Server` with signal handling (`SIGINT`, `SIGTERM`), a 15-second shutdown timeout, and explicit HTTP timeouts (`ReadHeaderTimeout: 5s`, `ReadTimeout: 10s`, `WriteTimeout: 30s`, `IdleTimeout: 120s`).
- **Configurable token lifetimes**: `ACCESS_TOKEN_LIFETIME_MINUTES` (default 15, bounds 1-60) and `REFRESH_TOKEN_LIFETIME_DAYS` (default 7, bounds 1-90) are read from environment variables. `0` falls back to the default before bounds checking.
- **Seed client placeholder documented**: `002_seed_clients.sql` has a clear `-- WARNING` block about the placeholder bcrypt hash.

---

## 2. Known Technical Debt and Deferred Features

### 2.1 Technical Debt

| Item | Description | Effort |
|------|-------------|--------|
| **Placeholder bcrypt hash** | `002_seed_clients.sql` contains `$2a$10$PLACEHOLDER`. Must be replaced with a real hash before first deployment. This is an ops task, not a code change. | Trivial |
| **Inline login HTML template** | The provider selection page is a `const selectProviderHTML` in `login/handler.go`. Acceptable while the UI is minimal, but should be extracted to a template file or replaced by a proper frontend if the UI grows. | Low |
| **No mock-based unit tests for Storage** | The repository interfaces now make it possible to write pure unit tests for `oidcprovider.Storage` without PostgreSQL. This was intentionally deferred -- the existing 102 integration tests provide strong coverage. Add mock tests if Storage logic becomes more complex. | Medium |
| **`testcontainers-go` in two packages** | Both `repo/repo_test.go` and `oidcprovider/storage_test.go` independently start PostgreSQL containers. The shared `testhelper` package handles migrations and table cleanup but not container lifecycle. If a third package needs integration tests, consider a single shared container setup. | Low-medium |

### 2.2 Deferred Features (Explicitly Scoped Out)

These were deliberated in the original plan and handoff document. They are not oversights.

| Feature | Why Deferred | When to Add |
|---------|-------------|-------------|
| **Key rotation** | Single RSA signing key from env var. No multi-key JWKS. | When deploying a rotation strategy. Add retired keys to `KeySet()` while signing only with the active key. |
| **Refresh token reuse detection** | Old refresh tokens are deleted on rotation, but replaying a deleted token does not revoke the entire token family. | Detect reuse, then revoke all tokens for that user+client pair. |
| **OpenTelemetry tracing** | `zitadel/oidc` has built-in OTel spans that activate when an OTel SDK is configured. | Add `go.opentelemetry.io/otel` SDK init in `main.go`. |
| **Active expired row cleanup** | Auth requests and tokens use passive expiry filtering in queries. No cron job deletes old rows. | Add a periodic cleanup goroutine or `pg_cron` rule. |
| **Admin UI / dynamic client registration** | Clients are inserted via manual SQL seed files. | When the admin API is built. |
| **Account linking** | Each `(provider, provider_subject)` creates a new user. No email-based merging. The `federated_identities` table already supports many-to-one. | When an account management API is built, using a user-initiated linking flow. |
| **Profile editing / upstream profile refresh** | User profiles are set from upstream claims on first login and never updated. | When an account management API is built. Any "refresh from upstream" must be an explicit user action. |
| **Device flow, token exchange, JWT bearer grant** | Not needed for current use cases. | If/when those grant types are required. |
| **Dev mode / localhost HTTP issuer** | HTTPS only. No `op.WithAllowInsecure()`. | Not planned. Use a reverse proxy with TLS termination for local dev. |
| **Additional BFF clients** | Only `myaccount-bff` is seeded. `drive-bff`, `chat-bff`, S2S clients are not registered. | When those services are implemented. Add seed SQL or use the future admin API. |

---

## 3. Architectural Decisions Future Agents Must Not Violate

These are firm decisions, not suggestions. Reversing any of these requires explicit human approval and a documented rationale.

### 3.1 Library API

Use the **Legacy Provider API** (`op.NewProvider` + `op.StaticIssuer`). Do NOT migrate to the `op.Server` interface unless there is a concrete need for custom protocol-level behavior. The library handles OIDC routing, discovery, PKCE validation, and error formatting.

### 3.2 No Profile Overwrite on Returning Logins

`login/handler.go:findOrCreateUser` returns the existing user unchanged if `FindByFederatedIdentity` finds a match. Upstream claims are only used during initial user creation. This protects future user-edited profile data.

### 3.3 No Email-Based Account Linking

Each `(provider, provider_subject)` maps to exactly one user. Same email from different providers = different users. We do not independently verify email addresses. Explicit account linking is a future feature requiring a user-initiated flow.

### 3.4 Federated State Uses the OP CryptoKey

The `state` parameter in upstream OAuth2 redirects is encrypted with AES-256-GCM using the same `[32]byte` CryptoKey from config. No separate key management.

### 3.5 Stateless OP -- No Session Cookies

The OP does not set its own session cookie. Every authorization request triggers a full federated login flow. If the upstream IdP has an active session, it may auto-approve (giving the effect of SSO), but that is the upstream IdP's responsibility.

### 3.6 sqlx + Raw SQL, No ORM

All database access uses `sqlx` with handwritten SQL. No GORM, no query builders. PostgreSQL arrays are scanned via `pq.StringArray`. Nullable columns use `*string` / `*time.Time` in scan targets.

### 3.7 JWT Access Tokens

Access tokens use `AccessTokenTypeJWT`, verifiable by resource servers via the JWKS endpoint without calling introspection. Refresh tokens are opaque and rotated on every use.

### 3.8 Supported Grant Types

Authorization Code (with PKCE), Refresh Token, Client Credentials. No others.

### 3.9 IntrospectionResponse Structure

`oidc.IntrospectionResponse` embeds `UserInfoProfile` and `UserInfoEmail` directly -- NOT as a `UserInfo` sub-struct. `setIntrospectionUserinfo` and `setUserinfo` are separate helpers because of this structural difference. Do not merge them.

### 3.10 Consumer-Defined Interfaces

Repository interfaces are defined in each consuming package (`oidcprovider`, `login`), not in the `repo` package. A little signature duplication between consumers is preferred over cross-package coupling. This follows the Go idiom: "accept interfaces, return structs."

### 3.11 Package Naming

- `oidcprovider` (not `oidc` or `provider`) to avoid name collisions with the library.
- `login` for federated login handlers -- separate from `oidcprovider` to keep OIDC protocol code separate from authentication UI.
- Exported constructors follow the `New*` pattern.

### 3.12 Test Infrastructure

- `migrations/embed.go` provides `//go:embed *.sql` for the migration files.
- `testhelper.RunMigrations(db)` uses `io/fs.ReadDir` (not `embed.FS.ReadDir`) for a spec-backed lexicographic sort guarantee.
- `testhelper.CleanTables(t, db)` truncates all data tables in dependency-safe order.
- `RunMigrations` takes no `testing.TB` parameter -- it is safe for `TestMain`. `CleanTables` takes `testing.TB`.
- Each test package independently manages its own testcontainers PostgreSQL container.

### 3.13 Token Lifetime Validation

Environment variable `0` is treated as "use default" (substitution happens before bounds checking). The bounds are 1-60 minutes for access tokens and 1-90 days for refresh tokens. These limits are enforced in `config.Load()`.
