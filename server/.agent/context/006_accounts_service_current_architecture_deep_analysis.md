# 006 - Accounts Service: Current Architecture Deep Analysis

**Date:** 2026-03-02
**Scope:** `server/services/accounts/` -- every file outside of `server/.agent/archive/` and `web/`
**Purpose:** Comprehensive analysis of the current codebase to identify architectural debts, coupling issues, mixed responsibilities, and domain boundary violations, in preparation for a modular-monolith refactoring.

---

## 1. High-Level Overview

The accounts service is a standalone OIDC Provider (OP) that delegates actual user authentication to upstream identity providers (Google OIDC, GitHub OAuth2). It is built on top of the **zitadel/oidc v3** library and exposes:

- Standard OIDC endpoints (authorization, token, userinfo, introspection, JWKS, end-session) via the `op.Provider` from zitadel/oidc, mounted at `/`
- A custom login flow (`/login/`, `/login/select`, `/login/callback`) that orchestrates the upstream federated authentication and then completes the internal auth request so the zitadel OIDC library can finalize the authorization code flow

### Current Package Layout

```
services/accounts/
  main.go              # Entrypoint: wiring, routing, HTTP server lifecycle
  config/              # Configuration loading from environment variables
  model/               # Data structs (AuthRequest, Client, User, FederatedIdentity, Token, RefreshToken)
  repo/                # PostgreSQL repositories (raw SQL via sqlx)
  oidcprovider/        # zitadel/oidc Storage implementation + adapter types
  login/               # HTTP handlers for federated login + upstream provider config
  migrations/          # SQL DDL + seed data (embedded via embed.FS)
  testhelper/          # Shared test utilities (migration runner, table cleaner)
```

### Data Flow Summary

1. **Authorization Request Initiation**: A relying party (RP) redirects the user to the OIDC authorization endpoint. The zitadel library receives this, calls `Storage.CreateAuthRequest()` to persist it, then retrieves the `Client.LoginURL()` to redirect the user to `/login?authRequestID=<id>`.

2. **Provider Selection**: `login.Handler.SelectProvider` renders an HTML page with buttons for each upstream provider.

3. **Federated Redirect**: `login.Handler.FederatedRedirect` encrypts the internal `authRequestID` + provider name into an AES-GCM state parameter, then redirects the user to the upstream provider's authorization endpoint.

4. **Federated Callback**: `login.Handler.FederatedCallback` exchanges the authorization code with the upstream provider, retrieves user info/claims, finds-or-creates a local user, calls `authReqRepo.CompleteLogin()` to mark the auth request as done, then redirects back to the zitadel library's authorize callback URL.

5. **Token Issuance**: The zitadel library calls `Storage.AuthRequestByCode()`, verifies the code challenge, calls `Storage.CreateAccessToken()` / `Storage.CreateAccessAndRefreshTokens()`, and returns tokens to the RP.

6. **Token Refresh / Revocation / Introspection / Userinfo**: All handled through the `Storage` methods, delegating to the appropriate repos.

---

## 2. Package-by-Package Analysis

### 2.1 `config/` Package

**Files:** `config.go`, `config_test.go`

**What it does:** Loads all configuration from environment variables. Parses RSA private keys (PKCS#1 and PKCS#8), hex-decodes the AES crypto key, validates required fields and range constraints for token lifetimes.

**Design observations:**
- **Flat, monolithic config struct**: `Config` holds *everything* -- OIDC provider settings, database URL, upstream IdP credentials, token lifetime policy, crypto keys. In a modular monolith, these would be scoped to their respective domains (identity config, OIDC config, authn config).
- **Direct `os.Getenv` coupling**: No abstraction over environment sources, making it hard to inject config from other sources (e.g., Vault, config files, test fixtures). However, `t.Setenv` works for tests, which is adequate for now.
- **Mixed concern: RSA key parsing**: The `parseRSAPrivateKey()` function here is a crypto utility that arguably belongs in a `pkg/crypto` or `internal/pkg` utility package.
- **Upstream-provider-aware**: Config knows about `GoogleClientID`, `GitHubClientID`, etc. by name. Adding a new upstream provider requires modifying this struct and `Load()` function.
- **Well-tested**: Good test coverage for all success and error paths.

### 2.2 `model/` Package

**Files:** `authrequest.go`, `client.go`, `federated_identity.go`, `token.go`, `user.go`

**What it does:** Pure data structs with `db:` struct tags for sqlx scanning.

**Design observations:**
- **Anemic models**: These are purely data transfer objects with no behavior. They serve double-duty as both database row representations and domain objects. There are no domain methods, validation rules, or invariant enforcement.
- **No domain boundaries**: All models live in a single flat package. `User` and `FederatedIdentity` are identity domain concepts, `Client` is an OIDC registration concept, `AuthRequest` is an OIDC protocol concept, and `Token`/`RefreshToken` are token domain concepts. They're all lumped together.
- **Implicit coupling via `db:` tags**: These structs are tightly coupled to the database schema. Any schema change necessitates changing these "domain" types. In Clean Architecture, domain entities should be independent of persistence concerns.
- **Array fields stored as Go slices**: `Scopes`, `Prompt`, `AMR`, `RedirectURIs`, etc. are `[]string` in Go but `TEXT[]` in PostgreSQL. The conversion between these is handled at the repo layer using `pq.StringArray`, but the model itself doesn't express this constraint.

### 2.3 `repo/` Package

**Files:** `authrequest.go`, `client.go`, `token.go`, `user.go`, `repo_test.go`

**What it does:** PostgreSQL data access via raw SQL and `sqlx`. Each repository struct wraps a `*sqlx.DB`.

**Design observations:**
- **No interfaces defined in repo package**: Repository types are concrete structs. The interfaces are defined *in the consumer* (`oidcprovider/storage.go` defines `UserReader`, `ClientReader`, `AuthRequestStore`, `TokenStore`; `login/handler.go` defines `userFinder`, `authRequestCompleter`). This is actually a reasonable Go pattern (consumer-defined interfaces), but it means the interface definitions are scattered across two different packages with overlapping method sets.
- **Duplicated interface definitions**: `oidcprovider.UserReader` requires `GetByID`, `FindByFederatedIdentity`, `CreateWithFederatedIdentity`. `login.userFinder` requires `FindByFederatedIdentity`, `CreateWithFederatedIdentity`. These overlap but are defined independently. `oidcprovider.AuthRequestStore` requires `Create`, `GetByID`, `GetByCode`, `SaveCode`, `CompleteLogin`, `Delete`. `login.authRequestCompleter` requires `GetByID`, `CompleteLogin`. Again, overlapping subsets.
- **Raw SQL everywhere**: No query builder, no ORM, no named queries. This is fine for the current size but makes refactoring harder -- SQL strings are not type-checked.
- **Manual row scanning**: `scanOne()` in `authrequest.go`, `scanToken()` and `scanRefreshToken()` in `token.go` manually scan into nullable intermediaries and then copy to the model struct. This is verbose but correct. However, `client.go` and `user.go` use `row.Scan()` and `row.StructScan()` respectively -- inconsistent patterns within the same package.
- **No transaction abstraction**: Transactions are created directly inside repos (`r.db.BeginTxx`). There's no unit-of-work pattern. If `login.Handler.findOrCreateUser` + `CompleteLogin` needed to be atomic, there's no way to wrap them in a single transaction from the handler layer.
- **Auth request TTL via SQL**: `activeFilter = "created_at > now() - interval '30 minutes'"` is a hardcoded constant in the repo. This TTL policy is a business rule embedded in the data access layer. It should be configurable and live in the domain/service layer.
- **`UserRepository` has `Create()` method not exposed via interface**: The `UserReader` interface in `oidcprovider` doesn't include `Create()`. `Create()` is only called internally by `CreateWithFederatedIdentity()`. This is fine but shows the interface isn't comprehensive -- it's tailored to consumers.
- **Testing uses testcontainers**: Integration tests spin up real PostgreSQL via Docker. This is heavyweight but high-fidelity. Both `repo/` and `oidcprovider/` have their own `TestMain` that starts a separate PostgreSQL container, meaning running all tests spins up two containers.

### 2.4 `oidcprovider/` Package

**Files:** `storage.go`, `provider.go`, `authrequest.go`, `client.go`, `keys.go`, `refreshtoken.go`, `storage_test.go`, `authrequest_test.go`, `client_test.go`, `keys_test.go`

**What it does:** Implements the `op.Storage` interface required by zitadel/oidc. This is the glue layer between the zitadel OIDC library and the internal repositories.

**Design observations:**

#### 2.4.1 The God-Object `Storage` Struct

`Storage` is the single most problematic type in the codebase from an architecture perspective:
- It implements **all** storage interfaces required by zitadel/oidc: `op.Storage` (which bundles `AuthStorage`, `OPStorage`, `ClientCredentialsStorage`), `KeyStorage`, etc.
- It holds references to 4 repos + 2 key types + 2 lifetimes + the raw `*sqlx.DB` -- that's 8 dependencies.
- It has ~30 methods spanning auth request management, token lifecycle, client lookup, user info population, key management, introspection, revocation, and health checking.
- These methods cross multiple domain boundaries: identity (user lookup), OIDC protocol (auth requests, clients), token management (access/refresh tokens), and key management (signing/verification keys).

This is a direct consequence of the zitadel/oidc library's design, which expects a single `Storage` interface implementation. But the current code doesn't internally decompose this; it's a flat file mixing all concerns.

#### 2.4.2 Adapter Types (AuthRequest, Client, RefreshTokenRequest)

These are thin wrappers that adapt `model.*` types to the interfaces required by `op.AuthRequest`, `op.Client`, and `op.RefreshTokenRequest`:
- `AuthRequest` wraps `model.AuthRequest` and maps fields to getter methods.
- `Client` wraps `model.Client` and converts string values to zitadel enum types.
- `RefreshTokenRequest` wraps `model.RefreshToken`.

These are correctly implementing the Adapter pattern. However:
- **They live in `oidcprovider/` alongside the Storage**: They should arguably be separate from the massive storage file or at least better organized.
- **String-to-enum mapping is hardcoded**: `Client.ApplicationType()`, `Client.AuthMethod()`, `Client.AccessTokenType()` all use switch statements with magic strings ("native", "user_agent", "client_secret_post", etc.). These string values come from the database and there's no validation that the database contains valid values.
- **`clientIDFromRequest()` uses type assertions**: The helper `clientIDFromRequest()` does `request.(*AuthRequest)` / `request.(*RefreshTokenRequest)` / `request.(*clientCredentialsTokenRequest)` type switches. This couples the implementation to knowing all possible concrete types.

#### 2.4.3 Key Management

`keys.go` implements `SigningKeyWithID` and `PublicKeyWithID` for the `op.SigningKey` and `op.Key` interfaces:
- Key ID is derived via SHA-256 of the DER-encoded public key, truncated to 8 bytes (16 hex chars). This is deterministic and stable across restarts as long as the key doesn't change.
- Uses `panic()` on marshaling failure in `deriveKeyID()`, which is acceptable as a coding error rather than a runtime error.
- Only supports RS256 (hardcoded). No EC key support or algorithm flexibility.
- Only supports a single signing key. No key rotation mechanism.

#### 2.4.4 Provider Configuration

`provider.go` is trivially small -- it just constructs an `op.Config` and calls `op.NewProvider()`. The config hardcodes:
- `CodeMethodS256: true` (good -- PKCE S256 required)
- `AuthMethodPost: true`
- `GrantTypeRefreshToken: true`
- Supported locales: English + Japanese

This file is clean but the config values are not externalized.

#### 2.4.5 Duplicated Userinfo Mapping

`storage.go` has both `setUserinfo()` and `setIntrospectionUserinfo()`. These contain nearly identical switch-case logic for mapping scopes to user attributes, but they target different types (`*oidc.UserInfo` vs `*oidc.IntrospectionResponse`). The duplication is forced by the zitadel library's type system (these are different structs that happen to have similar fields) but could be DRYed up with a more creative approach.

#### 2.4.6 JWT Profile Grant Stubs

`GetKeyByIDAndClientID()` and `ValidateJWTProfileScopes()` return hardcoded errors. These are stubs required by the interface but not supported. This is fine.

### 2.5 `login/` Package

**Files:** `handler.go`, `handler_test.go`, `upstream.go`, `templates/` (empty directory)

**What it does:** Handles the user-facing login flow: provider selection page, redirect to upstream IdP, callback processing, user find-or-create, and auth request completion.

**Design observations:**

#### 2.5.1 Mixed Responsibilities in `handler.go`

The `Handler` struct combines:
1. **HTTP request handling** (route handlers: `SelectProvider`, `FederatedRedirect`, `FederatedCallback`)
2. **Domain logic** (user find-or-create in `findOrCreateUser`)
3. **Cryptographic operations** (AES-GCM state encryption/decryption in `encryptState`/`decryptState`)
4. **HTML rendering** (inline template string `selectProviderHTML` as a Go const)

In Clean Architecture, these should be separate layers:
- HTTP handlers (presentation/adapter layer)
- Use cases / application services (finding/creating users, completing auth requests)
- Infrastructure (crypto, template rendering)

#### 2.5.2 Inline HTML Template

The `selectProviderHTML` is a raw HTML string constant embedded in `handler.go`. The `login/templates/` directory exists but is empty. This is arguably fine for a simple page but:
- It can't be overridden or themed without code changes.
- It mixes presentation with handler logic in the same file.
- No CSS, no accessibility attributes, no CSRF protection on the form.

#### 2.5.3 CSRF Vulnerability in Provider Selection

The `/login/select` POST form doesn't include a CSRF token. While the form is simple and the impact is limited (an attacker could only trigger a redirect to an OAuth2 provider, not complete a login), it's still a defense-in-depth gap. The encrypted state parameter on the callback protects the callback step, but the initial redirect is unprotected.

#### 2.5.4 Upstream Provider Configuration

`upstream.go` defines `UpstreamProvider` and `UpstreamClaims` structs, plus factory functions for Google and GitHub:
- **Google**: Uses `go-oidc` for OIDC discovery and ID token verification. Extracts claims from the ID token.
- **GitHub**: Uses raw HTTP calls to `https://api.github.com/user` since GitHub is OAuth2, not OIDC. Creates its own `http.Client` with a 10-second timeout.

Issues:
- **Provider-specific code in login package**: The Google/GitHub specifics (endpoint URLs, claim structures, API calls) are hardcoded in the login package. Adding a new provider means modifying `upstream.go` and `config.go`.
- **No shared `http.Client`**: The GitHub provider creates a new `http.Client` per request. Should use a shared client (or the one from the oauth2 token source).
- **GitHub email isn't necessarily the primary/verified email**: The `/user` endpoint returns the public profile email, which may be empty if the user hasn't made it public. The `user:email` scope grants access to `/user/emails`, but that endpoint isn't called. The `EmailVerified` field is never set for GitHub users.
- **`UserInfoFunc` signature couples providers to the login handler**: This is a function field, which is flexible but makes testing and extending harder than a well-defined interface.

#### 2.5.5 State Encryption

The encrypted state uses AES-256-GCM with the same `CryptoKey` used for the zitadel OIDC library's internal crypto. While AES-GCM is appropriate, sharing the key between two different crypto operations (zitadel's internal OIDC state/codes and the custom federated state) is a subtle key-reuse concern. They should ideally use derived subkeys.

### 2.6 `migrations/` Package

**Files:** `001_initial.sql`, `002_seed_clients.sql`, `embed.go`

**What it does:** Defines the database schema and seed data, embedded via `embed.FS`.

**Design observations:**
- **Ordered by filename**: `testhelper.RunMigrations()` reads files via `fs.ReadDir` which returns entries sorted alphabetically. This works by convention (prefixed numbers) but there's no migration tracking -- re-running is idempotent only if SQL statements are `CREATE TABLE IF NOT EXISTS` etc. (they're not -- `CREATE TABLE` without `IF NOT EXISTS`).
- **Seed data in migrations**: `002_seed_clients.sql` inserts a client with a placeholder secret hash. This conflates schema management with data seeding.
- **No down migrations**: No rollback capability.
- **No proper migration framework**: No tracking table, no version management.

### 2.7 `testhelper/` Package

**Files:** `testdb.go`

**What it does:** Provides `RunMigrations()` and `CleanTables()` for test setup.

**Design observations:**
- **Hardcoded table list**: `CleanTables` has a hardcoded list of table names in dependency order. This is fragile -- adding a table requires updating this list.
- **Coupled to migrations package**: Imports `migrations.FS` directly.
- **Used by both `repo/` and `oidcprovider/` test suites**: Each spins up its own PostgreSQL container.

### 2.8 `main.go`

**What it does:** Application entrypoint. Creates all dependencies, wires them together, sets up routing, starts the HTTP server, handles graceful shutdown.

**Design observations:**
- **All wiring in `main()`**: This is essentially a composition root, which is the correct place for wiring. However, it has no abstraction -- everything is constructed inline.
- **No dependency injection framework**: Manual wiring is fine for this size.
- **OIDC provider mounted at root**: `router.Mount("/", provider)` gives the zitadel OIDC provider all unmatched routes. The custom login routes are mounted before it at `/login/`.
- **`op.AuthCallbackURL(provider)`** is used to generate the callback URL function -- this is a zitadel helper that constructs the internal authorize callback URL.
- **Health/readiness probes** are inline anonymous functions.

---

## 3. Dependency Graph

```
main.go
  imports: config, login, oidcprovider, repo
  depends on: chi, sqlx, pq (driver), op (zitadel)

config/
  imports: (stdlib only)
  no internal dependencies

model/
  imports: (stdlib only - just "time")
  no internal dependencies

repo/
  imports: model
  depends on: sqlx, pq, uuid

oidcprovider/
  imports: model
  depends on: sqlx, oidc (zitadel), op (zitadel), jose, uuid, bcrypt

login/
  imports: model, config
  depends on: go-oidc (coreos), oauth2, uuid

testhelper/
  imports: migrations
  depends on: sqlx

migrations/
  imports: (embed only)
  no internal dependencies
```

### Dependency Issues

1. **`oidcprovider` depends on `sqlx`**: The Storage struct holds a `*sqlx.DB` directly (used only for `Health()` ping). This leaks the database driver into the OIDC adapter layer.

2. **`oidcprovider` depends on `bcrypt`**: `AuthorizeClientIDSecret()` does bcrypt comparison directly. This is a crypto/security concern embedded in the OIDC adapter layer rather than the client repository.

3. **`login` depends on `config`**: `NewUpstreamProviders()` takes `*config.Config` as input, coupling the login package to the flat config struct. It only uses 5 fields from it (Issuer, Google*/GitHub*).

4. **Two separate OIDC libraries**: The service uses both `zitadel/oidc` (for acting as an OP) and `coreos/go-oidc` (for acting as an RP to Google). These are independent libraries with different type systems. This is correct (different roles) but adds complexity.

---

## 4. Specific Architectural Debts

### 4.1 God-Object Anti-Pattern: `oidcprovider.Storage`

**Severity: High**

The `Storage` struct implements ~30 methods across 5+ logical domains. It's forced by the zitadel `op.Storage` interface, but the internal implementation doesn't decompose it. Each method is a thin pass-through to a repo, but the struct itself is a dependency magnet.

**Impact on refactoring:** This is the primary obstacle to separating concerns. The zitadel library sees `Storage` as a single object, but internally it should delegate to domain-specific services.

### 4.2 No Service/Use-Case Layer

**Severity: High**

There is no application service or use-case layer between the HTTP handlers / OIDC storage adapter and the repositories. Business logic is scattered:
- User find-or-create logic lives in `login.Handler.findOrCreateUser()`
- Auth request TTL policy lives in `repo.AuthRequestRepository` (hardcoded SQL)
- Token lifetime policy lives in `main.go` (config-to-duration conversion) and `oidcprovider.Storage` (passed at construction time)
- Client secret verification lives in `oidcprovider.Storage.AuthorizeClientIDSecret()`
- Userinfo claim mapping lives in `oidcprovider.Storage.setUserinfo()`

### 4.3 Model Package is Anemic and Undifferentiated

**Severity: Medium**

The `model` package is a catch-all for all data structures. There's no distinction between:
- Domain entities (User, FederatedIdentity)
- Protocol/framework types (AuthRequest, Client -- these are OIDC concepts)
- Infrastructure types (Token, RefreshToken -- these are persistence-coupled)

In Clean Architecture, `User` and `FederatedIdentity` would be in an `identity` domain, `AuthRequest` in an `oidc` domain, `Client` in an `oidc` or `registration` domain, and tokens in a `token` domain.

### 4.4 Repo Layer Embeds Business Rules

**Severity: Medium**

- The 30-minute auth request TTL (`activeFilter` constant) is a business rule hardcoded in SQL.
- Token expiration filtering (`expiration > now()` in `GetByID`, `GetRefreshToken`, `GetRefreshInfo`) is business logic in the data layer.
- No soft-delete or audit trail -- tokens are hard-deleted on revocation.

### 4.5 Tight Coupling Between Login Handler and Domain Logic

**Severity: Medium**

`login.Handler` directly performs:
1. User lookup/creation (domain operation)
2. Auth request completion (domain operation)
3. AES-GCM encryption/decryption (infrastructure operation)
4. HTML template rendering (presentation operation)

There's no separation between the HTTP adapter and the application logic. The `findOrCreateUser` method is a domain operation that should be in an application service.

### 4.6 No Error Domain Types

**Severity: Low-Medium**

Errors are ad-hoc throughout:
- Repos return raw `sql.ErrNoRows` or driver errors
- Storage methods translate `sql.ErrNoRows` to `oidc.ErrInvalidRequest()` or `op.ErrInvalidRefreshToken`
- Login handlers log errors and return generic HTTP error messages
- No custom error types for domain-specific failures (e.g., `UserNotFoundError`, `AuthRequestExpiredError`)

### 4.7 Interface Fragmentation

**Severity: Low-Medium**

The same repository is described by different interfaces in different packages:
- `oidcprovider.UserReader` (3 methods)
- `login.userFinder` (2 methods, subset of UserReader)
- `oidcprovider.AuthRequestStore` (6 methods)
- `login.authRequestCompleter` (2 methods, subset of AuthRequestStore)

While consumer-defined interfaces are idiomatic Go, the overlap creates a cognitive burden and makes it unclear what the canonical "shape" of these repositories is.

### 4.8 Duplicate Postgres Containers in Tests

**Severity: Low**

Both `repo/repo_test.go` and `oidcprovider/storage_test.go` have their own `TestMain` that starts a separate PostgreSQL container. This doubles test infrastructure cost and implies the packages are tested in isolation when they share the same schema.

### 4.9 No Observability

**Severity: Low**

- No metrics (no Prometheus, no OpenTelemetry metrics) despite having otel dependencies.
- Logging is basic `slog` with no request-ID correlation.
- No tracing spans around repo calls or external HTTP calls to upstream providers.

### 4.10 Single Signing Key, No Rotation

**Severity: Low** (for current scale)

The service loads a single RSA key at startup. There's no key rotation support, no JWK set versioning. The `KeySet()` method always returns exactly one key. For a modular monolith, key management should be an explicit, well-bounded concern.

---

## 5. Mapping Current Structure to Target Modules

Based on the analysis, here is how the current code maps to potential modular-monolith boundaries:

| Current Location | Current Responsibility | Target Module |
|---|---|---|
| `model/user.go`, `model/federated_identity.go` | User + federated identity data | `internal/identity` |
| `repo/user.go` | User persistence | `internal/identity` (infra layer) |
| `login/handler.go` (findOrCreateUser) | User find-or-create logic | `internal/identity` (use case) |
| `login/upstream.go` | Upstream IdP configuration | `internal/authn` |
| `login/handler.go` (SelectProvider, FederatedRedirect, FederatedCallback) | Login HTTP handlers | `internal/authn` (HTTP adapter) |
| `login/handler.go` (encryptState/decryptState) | Crypto utility | `internal/pkg/crypto` |
| `model/client.go` | OIDC client data | `internal/oidc` |
| `model/authrequest.go` | OIDC auth request data | `internal/oidc` |
| `model/token.go` | Token data | `internal/oidc` |
| `repo/client.go` | Client persistence | `internal/oidc` (infra layer) |
| `repo/authrequest.go` | Auth request persistence | `internal/oidc` (infra layer) |
| `repo/token.go` | Token persistence | `internal/oidc` (infra layer) |
| `oidcprovider/storage.go` | zitadel Storage adapter | `internal/oidc` (adapter layer) |
| `oidcprovider/authrequest.go`, `client.go`, `refreshtoken.go` | Type adapters | `internal/oidc` (adapter layer) |
| `oidcprovider/keys.go` | Key management | `internal/oidc` (infra) or `internal/pkg/crypto` |
| `oidcprovider/provider.go` | OIDC provider config | `internal/oidc` (setup) |
| `config/config.go` | Configuration | Split across modules + `internal/pkg/config` |
| `migrations/` | Schema management | `internal/pkg/migrations` or remain at service root |
| `testhelper/` | Test utilities | `internal/pkg/testhelper` |

---

## 6. Key Challenges for Refactoring

### 6.1 The zitadel `op.Storage` Interface Constraint

The zitadel library expects a **single** object implementing `op.Storage`. This interface is large (~25+ methods). In a clean modular monolith, these methods belong to different modules. The solution is to keep a thin **compositor** type in the OIDC module that implements `op.Storage` by delegating to module-specific services. The internal modules expose their own clean interfaces.

### 6.2 Shared `model` Types

Currently, `model.User` is used by `repo/`, `oidcprovider/`, and `login/`. In a modular monolith, each module should own its domain types. Cross-module references should go through well-defined ports (interfaces). The identity module should own `User` and expose a service interface; the OIDC module should only know about `UserID` and user claims via an interface, not the full `model.User`.

### 6.3 Shared Database

All repos currently share the same `*sqlx.DB` and the same schema. In a modular monolith with clean boundaries, each module should ideally own its tables and access them through its own repository layer. Cross-module data access should go through service interfaces, not direct DB queries. However, since this is a monolith (not microservices), sharing the database is acceptable as long as table ownership is clear.

### 6.4 Test Infrastructure

The two-container test setup needs to be unified. A shared test database setup (perhaps via a build tag or a shared `TestMain` at a higher level) would reduce overhead.

---

## 7. Summary of Findings

### What Works Well
- **Clean adapter pattern** for zitadel types (AuthRequest, Client, RefreshTokenRequest wrappers)
- **Consumer-defined interfaces** in Go-idiomatic style
- **Good test coverage** with real database integration tests
- **Correct OIDC flow** implementation with PKCE, state encryption, proper token lifecycle
- **Reasonable dependency choices** (chi, sqlx, zitadel/oidc, testcontainers)
- **Secure defaults** (AES-GCM state encryption, bcrypt client secrets, RS256 JWT signing)

### What Needs Refactoring
1. **God-object `Storage`** needs decomposition into domain-specific services behind the `op.Storage` facade
2. **Missing service/use-case layer** -- business logic is scattered across handlers, storage, and repos
3. **Flat `model` package** needs to be split into domain-owned entities
4. **Business rules in repo layer** (TTL, expiration filtering) need to move up
5. **Login handler** mixes HTTP, domain logic, crypto, and presentation
6. **Config** is a monolithic struct that should be split per-module
7. **Interface definitions** are fragmented across consuming packages
8. **No error types** for domain-specific failures
9. **Upstream provider code** is hardcoded for Google/GitHub with no extension mechanism
10. **Shared crypto key** between zitadel internals and custom state encryption
