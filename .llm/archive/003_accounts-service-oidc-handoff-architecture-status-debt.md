# Accounts Service -- Handoff Document

> Written at the end of the initial implementation session.
> For the full design rationale, see `server/.agent/context/002_accounts_service_implementation_plan_and_design.md`.
> For `zitadel/oidc` library internals, see `server/.agent/context/001_zitadel_oidc_library_internal_architecture_and_api_research.md`.

---

## 1. Current State

The `accounts` service is a fully implemented OIDC Provider (OP) for the hss-science platform. It lives at `server/services/identity-service/` and compiles, lints, and passes all 102 tests.

### What exists

| Layer | Files | Purpose |
|---|---|---|
| Entrypoint | `main.go` | Config → DB → repos → Storage → Provider → chi router → ListenAndServe |
| Config | `config/config.go` | Env-var loading with validation. RSA PEM parsing (PKCS1 + PKCS8). |
| Domain models | `model/*.go` | Pure structs with `db:` tags: User, FederatedIdentity, Client, AuthRequest, Token, RefreshToken |
| Repositories | `repo/*.go` | sqlx + raw SQL. One file per aggregate. No ORM. |
| OIDC adapters | `oidcprovider/client.go`, `authrequest.go`, `refreshtoken.go` | Wrap domain models to satisfy `op.Client`, `op.AuthRequest`, `op.RefreshTokenRequest` |
| OIDC Storage | `oidcprovider/storage.go` | Implements `op.Storage` + `ClientCredentialsStorage` + `CanSetUserinfoFromRequest`. Delegates to repos. |
| OIDC Keys | `oidcprovider/keys.go` | RSA signing/public key types. Deterministic `kid` from SHA-256 of DER public key. |
| OIDC Provider | `oidcprovider/provider.go` | Constructs `op.Provider` via `op.NewProvider` + `op.StaticIssuer`. |
| Login flow | `login/handler.go` | SelectProvider, FederatedRedirect, FederatedCallback. AES-GCM state encryption. Inline HTML template. |
| Upstream IdPs | `login/upstream.go` | Google (OIDC Discovery + id_token verification) and GitHub (OAuth2 + `/user` API). |
| Migrations | `migrations/001_initial.sql`, `002_seed_clients.sql` | Full DDL + seed for `myaccount-bff` client. |
| Docker | `Dockerfile` | Multi-stage: golang:1.25-alpine → distroless nonroot. |
| Tests | `*_test.go` across all packages | 102 tests total. Unit tests + testcontainers-go PostgreSQL integration tests. |

### Verification status

```
go build ./services/identity-service/...     # clean
go vet ./services/identity-service/...       # clean
golangci-lint run ./services/identity-service/...  # 0 issues
go test ./services/identity-service/... -count=1   # 102 tests, all pass
```

### Key dependencies added to `go.mod`

| Module | Purpose |
|---|---|
| `github.com/zitadel/oidc/v3` | OIDC Provider core (op.Storage, op.Provider, etc.) |
| `github.com/go-chi/chi/v5` | HTTP router (transitive dep of zitadel, used explicitly) |
| `github.com/jmoiron/sqlx` | SQL extensions for database/sql |
| `github.com/lib/pq` | PostgreSQL driver |
| `github.com/google/uuid` | UUID generation |
| `golang.org/x/oauth2` | OAuth2 client for upstream IdPs |
| `github.com/coreos/go-oidc/v3` | OIDC Discovery + ID token verification for upstream IdPs |
| `golang.org/x/crypto` | bcrypt for client secret hashing |
| `golang.org/x/text` | language.Tag for supported UI locales |
| `github.com/testcontainers/testcontainers-go` | PostgreSQL containers for integration tests |

---

## 2. Technical Debt & Planned Refactoring

### 2.1 Things that work but could be improved

- **Migration schema is duplicated in tests.** Both `repo/repo_test.go` and `oidcprovider/storage_test.go` contain inline copies of the DDL for test setup. If the schema changes, three places need updating (the migration file + two test files). A shared test helper or running the actual migration files from tests would be better.

- **Repos are concrete types, not interfaces.** `Storage` holds `*repo.UserRepository` etc. directly. This makes the storage layer harder to unit-test in isolation without a real DB. If pure unit tests for storage logic become needed, extracting repo interfaces would help. The current integration tests (testcontainers) cover them well enough for now.

- **Login handler template is inline.** The HTML for the provider selection page is a `const` string in `handler.go`. This is fine for the minimal page but will need to be extracted to a proper template file if the UI becomes more complex.

- **`002_seed_clients.sql` has a placeholder bcrypt hash.** The seed file contains `$2a$10$PLACEHOLDER` which must be replaced with a real bcrypt hash before first deployment.

- **No graceful shutdown.** `main.go` calls `http.ListenAndServe` without signal handling or `context.WithCancel`. Adding `os.Signal` handling and `server.Shutdown(ctx)` would be appropriate for production.

- **Token lifetimes are hardcoded constants.** `accessTokenLifetime = 15 * time.Minute` and `refreshTokenLifetime = 7 * 24 * time.Hour` are in `storage.go`. They could be made configurable via `Config` if per-deployment tuning is needed.

### 2.2 `.golangci.yml` was migrated to v2 format

The linter config was updated for golangci-lint v2.10.1:
- `linters.disable-all` → `linters.default: none`
- `issues.exclude-rules` → `linters.exclusions.rules`
- `issues.exclude-dirs` → `linters.exclusions.paths`
- `gosimple` removed (merged into `staticcheck` in v2)
- `goimports` moved to `formatters.enable`

Other services may need updating if they hit the same v2 migration issues.

---

## 3. What Was Explicitly Scoped Out

These are not oversights -- they were deliberately deferred per the plan:

| Item | Reason | When to add |
|---|---|---|
| **Key rotation** | Single signing key loaded from env. No multi-key JWKS support. | When deploying a key rotation strategy. Add retired keys to `KeySet()` while signing only with the active key. |
| **Refresh token reuse detection** | Old refresh tokens are deleted on rotation, but a replayed old token doesn't trigger family revocation. | Phase 2. Detect reuse → revoke all tokens for that user+client. |
| **OpenTelemetry tracing** | The `zitadel/oidc` library has built-in OTel spans; they'll activate when OTel is configured. | Phase 2. Add `go.opentelemetry.io/otel` SDK init in `main.go`. |
| **Active expired row cleanup** | Auth requests and tokens use passive expiry filtering (`WHERE created_at > now() - interval '30 minutes'`, `WHERE expiration > now()`). No cron job deletes old rows. | Add a periodic cleanup job or pg_cron rule. |
| **Admin UI / dynamic client registration** | Clients are inserted via manual SQL. | When the admin API is built. |
| **Account linking** | Each `(provider, provider_subject)` creates a new user. No email-based merging, no explicit linking UI. The schema already supports many-to-one via `federated_identities`. | When the account management API is built. |
| **Profile editing** | User profiles are populated from upstream claims on first login and never updated again. | When the account management API (future resource server in this same service) is built. |
| **Device flow, token exchange, JWT bearer grant** | Not needed for current use cases. | If/when those grant types are required. |
| **Dev mode / localhost** | No `op.WithAllowInsecure()`, no HTTP issuer support. HTTPS only. | Not planned. Use a reverse proxy with TLS termination for local dev. |
| **Other BFF clients** | Only `myaccount-bff` is seeded. `drive-bff`, `chat-bff`, S2S clients are not registered. | When those services are implemented. Add seed SQL or use the future admin API. |

---

## 4. Key Architectural Decisions

Future sessions must follow these patterns -- they are not arbitrary, they were deliberated in the plan.

### 4.1 Library API: Legacy `op.NewProvider`, NOT `op.Server`

We use the Legacy Provider API (`op.NewProvider` with `op.StaticIssuer`). The library handles all OIDC routing, discovery, PKCE validation, error formatting. We only implement `op.Storage` + custom login routes.

Do NOT migrate to the `op.Server` interface unless there is a concrete need for custom protocol-level behavior. The `NewLegacyServer` bridge exists if needed.

Note: the plan references `op.NewOpenIDProvider` (now deprecated). The actual code uses `op.NewProvider` + `op.StaticIssuer`, which is the non-deprecated replacement with the same behavior.

### 4.2 No profile overwrite on returning users

When a user logs in again via a federated IdP, their existing profile (name, email, picture) is NOT updated from upstream claims. Upstream claims are only used during initial user creation. This protects future user-edited profile data.

This is enforced in `login/handler.go:findOrCreateUser`. If you ever add a "refresh profile from upstream" feature, it must be an explicit user action, not automatic.

### 4.3 No email-based account linking

Each `(provider, provider_subject)` maps to exactly one user. Same email from two different providers = two different users. This is by design because we do not independently verify email addresses. Explicit account linking is a future feature that will use a user-initiated flow.

### 4.4 Federated state uses the OP CryptoKey

The `state` parameter in the upstream OAuth2 redirect is encrypted with AES-256-GCM using the same `[32]byte` CryptoKey from `op.Config`. No separate key management.

### 4.5 Stateless OP -- no session cookies

The OP does not set its own session cookie. Every authorization request triggers a full federated login flow. If the upstream IdP has an active session, it may auto-approve (giving the effect of SSO), but that is the upstream IdP's responsibility.

### 4.6 Repositories: sqlx + raw SQL, no ORM

All database access uses `sqlx` with handwritten SQL. No GORM, no query builders. PostgreSQL arrays are scanned via `pq.StringArray`. Nullable columns use `*string` / `*time.Time` in scan targets.

### 4.7 Token format: JWT access tokens, 15-minute lifetime

Access tokens are JWTs (`AccessTokenTypeJWT`), verifiable by resource servers via the JWKS endpoint without calling introspection. Refresh tokens are opaque, rotated on every use, 7-day lifetime.

### 4.8 Supported grant types

Authorization Code (with PKCE), Refresh Token, Client Credentials. No others.

### 4.9 `IntrospectionResponse` embeds fields directly

The `oidc.IntrospectionResponse` type embeds `UserInfoProfile` and `UserInfoEmail` directly -- NOT as a `UserInfo` sub-struct. The `setIntrospectionUserinfo` helper in `storage.go` sets fields directly on the response (e.g., `resp.Name = user.Name`), unlike `setUserinfo` which works on `*oidc.UserInfo` (e.g., `info.Name = user.Name`). These are two separate helpers because of this structural difference in the oidc library.

### 4.10 File and package conventions

- All source under `server/services/identity-service/` -- bounded context.
- Package `oidcprovider` (not `oidc` or `provider`) to avoid name collisions with the library.
- Package `login` for federated login handlers -- separate from `oidcprovider` to keep OIDC protocol code separate from authentication UI.
- Exported constructors follow the `New*` pattern: `NewStorage`, `NewClient`, `NewAuthRequest`, `NewSigningKey`, `NewPublicKey`, `NewProvider`, `NewHandler`, `NewUpstreamProviders`.
- The login HTML template is an inline `const selectProviderHTML` in `handler.go`, not a separate file.

---

## 5. File Index

Quick reference for the next agent:

```
server/services/identity-service/
├── main.go                         # Entrypoint: wiring, router, health endpoints
├── Dockerfile                      # Multi-stage build
├── .env.example                    # Required env vars with comments
├── config/
│   ├── config.go                   # Config struct, Load(), parseRSAPrivateKey()
│   └── config_test.go              # 13 tests
├── model/
│   ├── user.go                     # User struct
│   ├── federated_identity.go       # FederatedIdentity struct
│   ├── client.go                   # Client struct (DB model with string arrays)
│   ├── authrequest.go              # AuthRequest struct
│   └── token.go                    # Token, RefreshToken structs
├── repo/
│   ├── user.go                     # UserRepository (Create, GetByID, FindByFederatedIdentity, CreateWithFederatedIdentity)
│   ├── client.go                   # ClientRepository (GetByID)
│   ├── authrequest.go              # AuthRequestRepository (Create, GetByID, GetByCode, SaveCode, CompleteLogin, Delete)
│   ├── token.go                    # TokenRepository (CreateAccess, CreateAccessAndRefresh, GetByID, GetRefreshToken, GetRefreshInfo, DeleteByUserAndClient, Revoke, RevokeRefreshToken)
│   └── repo_test.go               # 11 integration tests (testcontainers PostgreSQL)
├── oidcprovider/
│   ├── keys.go                     # signingKey, publicKey, deriveKeyID; NewSigningKey(), NewPublicKey()
│   ├── client.go                   # Client adapter (op.Client); NewClient()
│   ├── authrequest.go              # AuthRequest adapter (op.AuthRequest); NewAuthRequest()
│   ├── refreshtoken.go             # RefreshTokenRequest adapter (op.RefreshTokenRequest); NewRefreshTokenRequest()
│   ├── storage.go                  # Storage (op.Storage + ClientCredentialsStorage + CanSetUserinfoFromRequest); NewStorage()
│   ├── provider.go                 # NewProvider() → op.NewProvider + op.StaticIssuer
│   ├── keys_test.go                # 5 tests
│   ├── client_test.go              # 20 tests
│   ├── authrequest_test.go         # 17 tests
│   └── storage_test.go             # 25 integration tests (testcontainers PostgreSQL)
├── login/
│   ├── upstream.go                 # UpstreamProvider, UpstreamClaims, NewUpstreamProviders()
│   ├── handler.go                  # Handler (SelectProvider, FederatedRedirect, FederatedCallback, findOrCreateUser, encrypt/decryptState)
│   └── handler_test.go             # 11 tests
└── migrations/
    ├── 001_initial.sql             # Full DDL (users, federated_identities, clients, auth_requests, tokens, refresh_tokens)
    └── 002_seed_clients.sql        # Seed myaccount-bff (PLACEHOLDER secret hash)
```
