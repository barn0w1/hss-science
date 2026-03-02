# Accounts Service Reengineering Plan -- Iteration 2+

**Date:** 2026-03-02
**Status:** Iteration 1 complete. Iteration 2 is the next body of work.

---

## Table of Contents

1. [Architectural Decisions (Settled)](#1-architectural-decisions-settled)
2. [Current State After Iteration 1](#2-current-state-after-iteration-1)
3. [Known Architectural Debts](#3-known-architectural-debts)
4. [Iteration 2: OIDC Domain Extraction](#4-iteration-2-oidc-domain-extraction)
5. [Iteration 3: Cleanup and Polish](#5-iteration-3-cleanup-and-polish)
6. [Target End-State Directory Tree](#6-target-end-state-directory-tree)

---

## 1. Architectural Decisions (Settled)

These decisions were made during Iteration 1 and are **not open for revisiting**. They form the foundation all future work builds on.

### 1.1 Identity Model

- **Strict 1:1 User<>FederatedIdentity.** One user, one provider link. Multi-provider linking is not supported. Enforced by `UNIQUE(user_id)` on `federated_identities`.
- **Immutable User profile.** Fields (`email`, `name`, `given_name`, `family_name`, `picture`, `email_verified`) are populated at first login from IdP claims and **never overwritten**. No `updated_at` column on `users`. If profile updates are ever needed, they become an explicit system action, not a login side-effect.
- **FederatedIdentity is the live sync record.** `provider_*` columns mirror raw upstream claims and are refreshed on every successful login. This gives us a current snapshot of what the IdP reports without destabilising the canonical User.
- **ULID primary keys** for `users` and `federated_identities` (application-generated, time-sortable, stored as `TEXT`). Legacy OIDC tables already use `TEXT` PKs.

### 1.2 Architecture Patterns

- **Hexagonal / Ports-and-Adapters** within each `internal/` module. Driving ports (`Service` interfaces) face outward; driven ports (`Repository` interfaces) face the persistence layer.
- **Consumer-defined interfaces.** Modules define the interfaces they depend on (e.g., `authn.AuthRequestQuerier`, `oidcprovider.ClientReader`). No module imports another module's concrete implementation directly.
- **Domain error translation.** `internal/pkg/domerr` provides sentinel errors (`ErrNotFound`, `ErrAlreadyExists`, etc.). Persistence adapters translate `sql.ErrNoRows` -> `domerr.ErrNotFound` at the boundary. HTTP/OIDC layers translate domain errors into protocol-specific responses.
- **No `db:` tags on domain types.** Scan targets (`*Row` structs) live in persistence adapters and carry all schema coupling. Domain types are pure.
- **`internal/` lives inside `services/accounts/`**, not at the server root. Each service owns its own internal tree.

### 1.3 Known Issues Carried Forward

| Issue | Impact | Fix Target |
|---|---|---|
| `oidcprovider.Storage` is a ~425-line god-object implementing all `op.Storage` methods | Hard to test, understand, or extend | Iteration 2 |
| `model/` package has anemic DTOs with `db:` tags shared between `repo/` and `oidcprovider/` | Tight coupling, schema in domain | Iteration 2 |
| `repo/` contains business rules in SQL (30-min auth request TTL via `activeFilter`) | Policy buried in persistence | Iteration 2 |
| Bcrypt client-secret verification lives in `oidcprovider.Storage.AuthorizeClientIDSecret` | Business logic in adapter | Iteration 2 |
| `authReqAdapter` bridge in `main.go` is temporary glue | Awkward wiring; delete when auth-request domain exists | Iteration 2 |
| Single RSA signing key, no rotation | Operational risk | Iteration 2/3 |
| Three separate testcontainers PostgreSQL instances (identity, oidcprovider, repo) | Slow CI, resource waste | Iteration 3 |
| GitHub provider may get empty email from `/user` endpoint | Silent auth failure for private-email users | Iteration 3 |
| No CSRF on provider selection POST | XSS-triggered IdP redirect possible (mitigated by encrypted callback state) | Iteration 3 |
| No observability (traces, metrics, request IDs) | Hard to debug in production | Iteration 3 |

---

## 2. Current State After Iteration 1

### 2.1 Directory Tree

```
services/accounts/
  main.go                               -- wired with identity.Service, authn.Handler, authReqAdapter bridge
  Dockerfile
  .env.example

  internal/
    pkg/
      domerr/
        errors.go                       -- ErrNotFound, ErrAlreadyExists, ErrUnauthorized, ErrInternal
        errors_test.go
      crypto/
        aes.go                          -- generic AES-256-GCM Encrypt/Decrypt
        aes_test.go

    identity/
      domain.go                         -- User, FederatedIdentity, FederatedClaims (no db tags)
      ports.go                          -- Repository (driven) + Service (driving) interfaces
      service.go                        -- FindOrCreateByFederatedLogin, GetUser
      service_test.go                   -- mock-based unit tests
      postgres/
        user_repo.go                    -- userRow/fiRow scan targets, domerr translation
        user_repo_test.go               -- testcontainers integration tests

    authn/
      config.go                         -- focused Config subset
      provider.go                       -- Provider struct, NewProviders factory
      provider_google.go                -- Google OIDC via coreos/go-oidc
      provider_github.go                -- GitHub OAuth2
      handler.go                        -- HTTP handlers, AuthRequestQuerier interface
      handler_test.go                   -- 10 test cases

  config/
    config.go                           -- monolithic Config struct (untouched)
    config_test.go

  oidcprovider/                         -- LEGACY: god-object Storage, zitadel type adapters
    storage.go                          -- ~425 lines; depends on identity.Service + model.* + repo.*
    storage_test.go
    provider.go                         -- NewProvider with hardcoded op.Config values
    authrequest.go                      -- wraps model.AuthRequest -> op.AuthRequest
    authrequest_test.go
    client.go                           -- wraps model.Client -> op.Client; string->enum mappers
    client_test.go
    keys.go                             -- single RSA key pair, deriveKeyID
    keys_test.go
    refreshtoken.go                     -- wraps model.RefreshToken -> op.RefreshTokenRequest

  model/                                -- LEGACY: anemic DTOs with db: tags
    authrequest.go                      -- model.AuthRequest
    client.go                           -- model.Client
    token.go                            -- model.Token, model.RefreshToken

  repo/                                 -- LEGACY: raw SQL repositories
    authrequest.go                      -- AuthRequestRepository (30-min activeFilter in SQL)
    client.go                           -- ClientRepository
    token.go                            -- TokenRepository (uuid.New() for IDs)
    repo_test.go                        -- integration tests for client, auth-request, token repos

  migrations/
    001_initial.sql                     -- reengineered schema (ULID users, 1:1 FI, TEXT PKs)
    002_seed_clients.sql
    embed.go

  testhelper/
    testdb.go                           -- RunMigrations, CleanTables
```

### 2.2 Dependency Graph (Current)

```
main.go
  |- config.Config
  |- identity.NewService(identitypg.NewUserRepository(db))
  |- repo.NewClientRepository(db)          <-- legacy
  |- repo.NewAuthRequestRepository(db)     <-- legacy
  |- repo.NewTokenRepository(db)           <-- legacy
  |- oidcprovider.NewStorage(db, identitySvc, clientRepo, authReqRepo, tokenRepo, ...)
  |- oidcprovider.NewProvider(issuer, cryptoKey, storage, logger)
  |- authn.NewProviders(ctx, authn.Config{...})
  |- authn.NewHandler(providers, identitySvc, &authReqAdapter{repo}, cryptoKey, callbackURL, logger)
  |- chi.Router -> /login/*, /healthz, /readyz, /logged-out, provider.Mount("/")

oidcprovider.Storage
  |- identity.Service           (clean)
  |- domerr                     (clean)
  |- model.AuthRequest          (legacy)
  |- model.Client               (legacy)
  |- model.Token                (legacy)
  |- model.RefreshToken         (legacy)
  |- repo interfaces (ClientReader, AuthRequestStore, TokenStore) satisfied by repo.*

authn.Handler
  |- identity.Service           (clean)
  |- pkg/crypto                 (clean)
  |- AuthRequestQuerier         (satisfied by authReqAdapter in main.go -- bridge to legacy repo)
```

### 2.3 What Iteration 1 Accomplished

- Extracted `internal/pkg/domerr` (shared sentinel errors) and `internal/pkg/crypto` (AES-256-GCM).
- Extracted `internal/identity` domain with clean domain types, service, repository interface, and PostgreSQL adapter.
- Extracted `internal/authn` with federated login handlers, upstream provider abstractions, and consumer-defined `AuthRequestQuerier`.
- Rewired `oidcprovider.Storage` to depend on `identity.Service` instead of the old `UserReader`/`repo.UserRepository`.
- Rewired `main.go` with new module constructors and a temporary `authReqAdapter` bridge.
- Reengineered DB schema: ULID PKs for identity tables, TEXT PKs throughout, immutable `users`, `provider_*` columns on `federated_identities`, `UNIQUE(user_id)`.
- Deleted: `login/` package, `model/user.go`, `model/federated_identity.go`, `repo/user.go`.
- All builds, tests (22 unit + integration), vet, and lint pass clean.

---

## 3. Known Architectural Debts

These are specific issues in the current code that Iteration 2 will address.

### 3.1 The `oidcprovider.Storage` God-Object

`storage.go` (~425 lines, ~30 methods) implements:
- `op.Storage` (auth-request lifecycle, token creation, token introspection, signing keys, health)
- `op.ClientCredentialsStorage` (client credentials grant)
- Helper methods (`setUserinfo`, `setIntrospectionUserinfo`, `clientIDFromRequest`)

It directly depends on three consumer-defined interfaces (`ClientReader`, `AuthRequestStore`, `TokenStore`) that are all satisfied by legacy `repo.*` types returning `model.*` DTOs.

### 3.2 `model/` Package -- Anemic DTOs

Three remaining files each export a single struct with `db:` tags:
- `model.AuthRequest` (25 fields) -- used by `repo/authrequest.go` and `oidcprovider/authrequest.go`
- `model.Client` (14 fields) -- used by `repo/client.go` and `oidcprovider/client.go`
- `model.Token` + `model.RefreshToken` -- used by `repo/token.go` and `oidcprovider/{storage,refreshtoken}.go`

These DTOs couple the persistence layer (`db:` tags) to the OIDC adapter layer (type wrappers). In clean architecture, domain types should have no infrastructure tags.

### 3.3 `repo/` Package -- Business Rules in SQL

- `repo/authrequest.go` line 35: `const activeFilter = "created_at > now() - interval '30 minutes'"` -- a policy decision (auth request TTL) hardcoded in a SQL fragment.
- `repo/token.go` line 90: `WHERE ... AND expiration > now()` -- token expiration check in SQL rather than domain.
- `repo/token.go` uses `uuid.New()` for IDs -- should migrate to ULID like identity tables.

### 3.4 Bcrypt in the Adapter Layer

`oidcprovider/storage.go:260-268`: `bcrypt.CompareHashAndPassword` is called directly in `AuthorizeClientIDSecret`. Client authentication is a domain concern, not an adapter concern.

### 3.5 Type Adapter Ceremony

`oidcprovider/authrequest.go`, `client.go`, `refreshtoken.go` each wrap a `model.*` struct and implement a zitadel interface (~15-20 getter methods each). The pattern is correct (adapting domain to framework), but currently the adapters wrap anemic DTOs rather than proper domain types.

### 3.6 String-to-Enum Conversion in `oidcprovider/client.go`

`ApplicationType()`, `AuthMethod()`, `ResponseTypes()`, `GrantTypes()`, `AccessTokenType()` all do string->zitadel-enum conversion. This mapping logic should live close to the domain or in a dedicated mapper, not scattered across a type adapter.

---

## 4. Iteration 2: OIDC Domain Extraction

**Goal:** Create `internal/oidc/` containing proper domain types, service interfaces, application services, and PostgreSQL adapters for auth requests, clients, and tokens. Dismantle the god-object `oidcprovider.Storage` into a thin compositor. Delete `model/` and `repo/`.

### 4.1 Design Principles

1. **Domain types in `internal/oidc/domain.go`** -- no `db:` tags, no framework imports. These are `AuthRequest`, `Client`, `Token`, `RefreshToken`.
2. **Repository interfaces in `internal/oidc/ports.go`** -- driven ports, one per aggregate root.
3. **Application services** -- `AuthRequestService`, `ClientService`, `TokenService` in separate files. Each owns its business logic (TTL policies, secret verification, token lifecycle).
4. **PostgreSQL adapters in `internal/oidc/postgres/`** -- scan targets with `db:` tags, `domerr` translation.
5. **Zitadel adapters in `internal/oidc/adapter/`** -- thin wrappers implementing `op.AuthRequest`, `op.Client`, `op.RefreshTokenRequest`, and a compositor implementing `op.Storage` by delegating to the three services + `identity.Service`.
6. **Key management** stays in `internal/oidc/adapter/keys.go` for now (moved from `oidcprovider/keys.go`).

### 4.2 Target Directory Tree for `internal/oidc/`

```
internal/oidc/
  domain.go                 -- AuthRequest, Client, Token, RefreshToken types
  ports.go                  -- AuthRequestRepository, ClientRepository, TokenRepository interfaces
                            -- AuthRequestService, ClientService, TokenService interfaces
  authrequest_svc.go        -- AuthRequestService implementation
  client_svc.go             -- ClientService implementation (bcrypt moves here)
  token_svc.go              -- TokenService implementation
  adapter/
    storage.go              -- thin op.Storage compositor delegating to domain services
    authrequest.go          -- op.AuthRequest wrapper around oidc.AuthRequest
    client.go               -- op.Client wrapper around oidc.Client (string->enum mappers)
    refreshtoken.go         -- op.RefreshTokenRequest wrapper
    keys.go                 -- SigningKeyWithID, PublicKeyWithID (from oidcprovider/keys.go)
    provider.go             -- NewProvider (from oidcprovider/provider.go)
  postgres/
    authrequest_repo.go     -- from repo/authrequest.go
    client_repo.go          -- from repo/client.go
    token_repo.go           -- from repo/token.go
    repo_test.go            -- from repo/repo_test.go (or split per-file)
```

### 4.3 Domain Types

```
AuthRequest:
  ID, ClientID, RedirectURI, State, Nonce, Scopes, ResponseType, ResponseMode
  CodeChallenge, CodeChallengeMethod, Prompt, MaxAge, LoginHint
  UserID, AuthTime, AMR, IsDone, Code, CreatedAt

Client:
  ID, SecretHash, RedirectURIs, PostLogoutRedirectURIs
  ApplicationType, AuthMethod, ResponseTypes, GrantTypes
  AccessTokenType, IDTokenLifetimeSeconds, ClockSkewSeconds, IDTokenUserinfoAssertion
  CreatedAt, UpdatedAt

Token:
  ID, ClientID, Subject, Audience, Scopes, Expiration, RefreshTokenID, CreatedAt

RefreshToken:
  ID, Token, ClientID, UserID, Audience, Scopes, AuthTime, AMR
  AccessTokenID, Expiration, CreatedAt
```

These mirror the current `model.*` types but without `db:` tags.

### 4.4 Service Responsibilities

**AuthRequestService:**
- `Create(ctx, *AuthRequest) error` -- generate ULID ID (replace uuid.New)
- `GetByID(ctx, id) (*AuthRequest, error)` -- enforce TTL in service, not SQL
- `GetByCode(ctx, code) (*AuthRequest, error)` -- enforce TTL in service
- `SaveCode(ctx, id, code) error`
- `CompleteLogin(ctx, id, userID, authTime, amr) error`
- `Delete(ctx, id) error`
- The 30-minute TTL becomes a configurable `time.Duration` on the service, passed from config.
- Also satisfies `authn.AuthRequestQuerier` directly, eliminating the `authReqAdapter` bridge in `main.go`.

**ClientService:**
- `GetByID(ctx, clientID) (*Client, error)`
- `AuthorizeSecret(ctx, clientID, clientSecret) error` -- bcrypt verification moves here
- `ClientCredentials(ctx, clientID, clientSecret) (*Client, error)` -- combines GetByID + AuthorizeSecret

**TokenService:**
- `CreateAccess(ctx, ...) (string, error)` -- generate ULID ID (replace uuid.New)
- `CreateAccessAndRefresh(ctx, ...) (accessID, refreshToken, error)` -- generate ULID IDs
- `GetByID(ctx, tokenID) (*Token, error)`
- `GetRefreshToken(ctx, token) (*RefreshToken, error)`
- `GetRefreshInfo(ctx, token) (userID, tokenID, error)`
- `DeleteByUserAndClient(ctx, userID, clientID) error`
- `Revoke(ctx, tokenID) error`
- `RevokeRefreshToken(ctx, token) error`

### 4.5 The Thin Storage Compositor

After extraction, `internal/oidc/adapter/storage.go` becomes:

```go
type StorageAdapter struct {
    identity    identity.Service
    authReqs    oidc.AuthRequestService
    clients     oidc.ClientService
    tokens      oidc.TokenService
    signing     *SigningKeyWithID
    public      *PublicKeyWithID
    accessTTL   time.Duration
    refreshTTL  time.Duration
}
```

Each `op.Storage` method becomes a 1-5 line delegation. ~120 lines total, down from ~425.

### 4.6 Implementation Steps

Each step must produce a compilable, test-passing codebase.

#### Step 1: Create `internal/oidc/domain.go`

- Define `AuthRequest`, `Client`, `Token`, `RefreshToken` types -- identical field sets to `model.*` but no `db:` tags.
- No code changes to existing files. Just the new file.

#### Step 2: Create `internal/oidc/ports.go`

- Define repository interfaces: `AuthRequestRepository`, `ClientRepository`, `TokenRepository`.
- Define service interfaces: `AuthRequestService`, `ClientService`, `TokenService`.
- `AuthRequestService` should embed or be compatible with `authn.AuthRequestQuerier` so it can satisfy that interface directly.

#### Step 3: Create `internal/oidc/postgres/` adapters

- Create `authrequest_repo.go` from `repo/authrequest.go` -- add scan targets with `db:` tags, map to domain types, translate `sql.ErrNoRows` -> `domerr.ErrNotFound`.
- Remove the hardcoded `activeFilter` from the repo. The repo does simple CRUD; TTL enforcement moves to the service.
- Create `client_repo.go` from `repo/client.go` -- same pattern.
- Create `token_repo.go` from `repo/token.go` -- same pattern, switch from `uuid.New()` to `ulid.Make()` for ID generation (or let the service generate IDs and pass them in).
- Create integration tests (move/adapt from `repo/repo_test.go`).

#### Step 4: Create application services

- Create `authrequest_svc.go` implementing `AuthRequestService`. TTL becomes `time.Duration` field. `GetByID`/`GetByCode` call repo then check `CreatedAt + TTL > now()` in Go.
- Create `client_svc.go` implementing `ClientService`. `AuthorizeSecret` does bcrypt comparison.
- Create `token_svc.go` implementing `TokenService`. Token IDs generated via `ulid.Make()`.
- Unit tests for each service with mock repositories.

#### Step 5: Create `internal/oidc/adapter/` -- zitadel type adapters

- Move `oidcprovider/authrequest.go` -> `internal/oidc/adapter/authrequest.go`. Change wrapped type from `model.AuthRequest` to `oidc.AuthRequest`.
- Move `oidcprovider/client.go` -> `internal/oidc/adapter/client.go`. Change wrapped type from `model.Client` to `oidc.Client`.
- Move `oidcprovider/refreshtoken.go` -> `internal/oidc/adapter/refreshtoken.go`. Change wrapped type.
- Move `oidcprovider/keys.go` -> `internal/oidc/adapter/keys.go`.
- Move `oidcprovider/provider.go` -> `internal/oidc/adapter/provider.go`.
- Move corresponding test files.

#### Step 6: Create `internal/oidc/adapter/storage.go` -- the thin compositor

- Implement all `op.Storage` methods by delegating to the three domain services + `identity.Service`.
- Each method is 1-5 lines of delegation + error mapping.
- Move `storage_test.go` from `oidcprovider/` and adapt.

#### Step 7: Rewire `main.go`

- Replace legacy `repo.*` constructors with `oidcpostgres.*` constructors.
- Replace `oidcprovider.NewStorage(...)` with `oidcadapter.NewStorageAdapter(...)`.
- Replace `oidcprovider.NewProvider(...)` with `oidcadapter.NewProvider(...)`.
- Delete the `authReqAdapter` bridge struct -- `AuthRequestService` satisfies `authn.AuthRequestQuerier` directly.
- Remove all `model` and `repo` imports.

#### Step 8: Delete legacy packages

- Delete `model/` directory entirely.
- Delete `repo/` directory entirely.
- Delete `oidcprovider/` directory entirely.

#### Step 9: Final verification

- `go build ./services/accounts/...`
- `go vet ./services/accounts/...`
- `go test ./services/accounts/...`
- `golangci-lint run ./services/accounts/...`
- Confirm `model/`, `repo/`, `oidcprovider/` are gone.
- Confirm no `db:` tags outside `internal/*/postgres/` packages.
- Confirm `main.go` imports only `internal/`, `config/`, and standard/third-party packages.

### 4.7 Implementation Checklist

- [ ] **Step 1:** Create `internal/oidc/domain.go` with AuthRequest, Client, Token, RefreshToken types
- [ ] **Step 2:** Create `internal/oidc/ports.go` with repository and service interfaces
- [ ] **Step 3:** Create `internal/oidc/postgres/` adapters (authrequest, client, token repos + tests)
- [ ] **Step 4:** Create application services (authrequest_svc, client_svc, token_svc + tests)
- [ ] **Step 5:** Move zitadel type adapters and keys to `internal/oidc/adapter/`
- [ ] **Step 6:** Create thin `StorageAdapter` compositor in `internal/oidc/adapter/storage.go` + tests
- [ ] **Step 7:** Rewire `main.go` (new constructors, delete authReqAdapter bridge, remove legacy imports)
- [ ] **Step 8:** Delete `model/`, `repo/`, `oidcprovider/`
- [ ] **Step 9:** Final verification (build, test, lint, directory check)

---

## 5. Iteration 3: Cleanup and Polish

These are improvements that become possible/easier after Iteration 2 is complete.

### 5.1 Testcontainers Unification

Currently three separate `TestMain` functions each spin up their own PostgreSQL testcontainer:
- `internal/identity/postgres/user_repo_test.go`
- `internal/oidc/postgres/repo_test.go` (will exist after Iteration 2; currently `repo/repo_test.go`)
- `oidcprovider/storage_test.go` (will be `internal/oidc/adapter/storage_test.go`)

Options:
1. **Shared test helper** that creates one container per `go test` invocation, passed via package-level variable.
2. **Build-tag gating** to skip integration tests in CI fast-path.
3. **Single integration test package** that imports all repos and tests them against one container.

### 5.2 Observability

- Add OpenTelemetry tracing spans to: identity service methods, OIDC service methods, repository calls, upstream provider HTTP calls.
- Add request-ID middleware to the chi router (generate UUID, inject in context, log with every request).
- Structured logging is already in place (`slog.JSONHandler`), but currently only `main.go` and `authn.Handler` use it. Propagate the logger to service layers.

### 5.3 Security Hardening

- **CSRF on provider selection POST:** The `/login/select` endpoint accepts a POST without CSRF protection. The encrypted state parameter protects the callback, but an attacker with XSS could trigger an IdP redirect. Consider adding a CSRF token or making the form use SameSite cookies.
- **GitHub primary email:** The GitHub `/user` endpoint may return an empty email for users with private emails. The provider should fall back to `/user/emails` to fetch the primary verified email.

### 5.4 Key Rotation

- Add a `signing_keys` table with `id`, `private_key_pem`, `created_at`, `expires_at`.
- `KeySet()` returns all non-expired keys (for verification). `SigningKey()` returns the most recently created key (for signing).
- New keys activated via config change or management endpoint.
- Old keys remain valid for verification until they expire.

### 5.5 Config Decomposition

The monolithic `config.Config` struct could be split so each module receives only its own config subset:
- `authn.Config` already exists (identity providers).
- Add `oidc.Config` (TTLs, signing key config).
- `config.Load()` assembles them all, but each module is decoupled from the full struct.

### 5.6 Auth Request TTL Cleanup

After Iteration 2 moves TTL enforcement to the service layer, consider adding a periodic cleanup job (goroutine or cron) that deletes expired auth requests from the database to prevent unbounded table growth.

### 5.7 Cleanup Checklist

- [ ] Unify testcontainers setup across integration test packages
- [ ] Add OpenTelemetry tracing spans
- [ ] Add request-ID middleware
- [ ] Add CSRF protection to provider selection form
- [ ] Harden GitHub provider to fetch primary email from `/user/emails`
- [ ] Implement signing key rotation
- [ ] Decompose config.Config into per-module subsets
- [ ] Add periodic cleanup job for expired auth requests
- [ ] Propagate structured logger to all service layers

---

## 6. Target End-State Directory Tree

After all iterations are complete:

```
services/accounts/
  main.go
  Dockerfile
  .env.example

  internal/
    pkg/
      domerr/
        errors.go
        errors_test.go
      crypto/
        aes.go
        aes_test.go

    identity/
      domain.go                 -- User, FederatedIdentity, FederatedClaims
      ports.go                  -- Repository, Service interfaces
      service.go
      service_test.go
      postgres/
        user_repo.go
        user_repo_test.go

    authn/
      config.go
      provider.go
      provider_google.go
      provider_github.go
      handler.go
      handler_test.go

    oidc/
      domain.go                 -- AuthRequest, Client, Token, RefreshToken
      ports.go                  -- Repository + Service interfaces per aggregate
      authrequest_svc.go
      authrequest_svc_test.go
      client_svc.go
      client_svc_test.go
      token_svc.go
      token_svc_test.go
      adapter/
        storage.go              -- thin op.Storage compositor
        storage_test.go
        authrequest.go          -- op.AuthRequest wrapper
        client.go               -- op.Client wrapper
        refreshtoken.go         -- op.RefreshTokenRequest wrapper
        keys.go                 -- signing/public key management
        keys_test.go
        provider.go             -- OIDC provider construction
      postgres/
        authrequest_repo.go
        client_repo.go
        token_repo.go
        repo_test.go

  config/
    config.go
    config_test.go

  migrations/
    001_initial.sql
    002_seed_clients.sql
    embed.go

  testhelper/
    testdb.go
```

The `model/`, `repo/`, `oidcprovider/`, and `login/` packages will all be gone. Every domain concept lives under `internal/` with clean boundaries, proper domain types, and infrastructure coupling isolated to `postgres/` and `adapter/` sub-packages.
