# Accounts Service — Deep Research Report

> Generated: 2026-03-16
> Scope: `hss-science/server/services/accounts` (entire codebase)
> Purpose: Pre-production elevation planning, with device/session management in mind for a future myaccount BFF

---

## 1. What This Service Is

The `accounts` service is a self-hosted OpenID Connect (OIDC) Authorization Server built on top of the [`zitadel/oidc` v3](https://github.com/zitadel/oidc) library. It is the single source of truth for user identity in the hss-science platform.

It does **not** handle username/password logins. Instead it is a pure **federated identity broker**: it accepts logins from upstream identity providers (Google, GitHub), upserts local user records, and issues OIDC tokens that downstream services can verify.

The anticipated first consumer is a `myaccount` BFF (Backend For Frontend), which will need richer session and device management than this service currently provides.

---

## 2. Architecture Overview

### 2.1 Structural Pattern

The service uses **hexagonal architecture (Ports & Adapters)**:

```
┌─────────────────────────────────────────────────────────────────┐
│  HTTP (chi router)                                              │
│    SecurityHeaders → GlobalRateLimit → chi.Mux                 │
│         ├── /login/*      authn.Handler (federated login)       │
│         ├── /healthz /readyz /logged-out  (ops)                 │
│         └── /*            op.Provider (zitadel OIDC)           │
└──────────────────────┬──────────────────────────────────────────┘
                       │ implements op.Storage
┌──────────────────────▼──────────────────────────────────────────┐
│  internal/oidc/adapter  (StorageAdapter)                        │
│  Bridge between zitadel library and domain services             │
└───┬──────────────┬─────────────────┬────────────────────────────┘
    │              │                 │
    ▼              ▼                 ▼
AuthReqSvc     ClientSvc         TokenSvc      ← domain services (ports.go)
    │              │                 │
    ▼              ▼                 ▼
internal/oidc/postgres/*          identity.Service
    │                                 │
    ▼                                 ▼
PostgreSQL                        internal/identity/postgres/
```

Each domain (identity, oidc) defines interface contracts in `ports.go`. Concrete implementations live in `postgres/` subdirectories. The adapter layer only depends on interfaces, never on concrete types. This is the correct pattern and makes the codebase highly testable.

### 2.2 Package Inventory

| Package | Role |
|---------|------|
| `main.go` | Wiring, HTTP server, background goroutines |
| `config/` | Environment-driven configuration with validation |
| `internal/authn/` | Federated login flow (Google, GitHub) |
| `internal/identity/` | User domain — find/create/update users |
| `internal/identity/postgres/` | PostgreSQL implementation of identity repository |
| `internal/oidc/` | OIDC domain — auth requests, clients, tokens |
| `internal/oidc/adapter/` | Bridges domain to zitadel `op.Storage` interface |
| `internal/oidc/postgres/` | PostgreSQL implementation of OIDC repositories |
| `internal/middleware/` | Rate limiting, security headers |
| `internal/pkg/crypto/` | AES-256-GCM cipher |
| `internal/pkg/domerr/` | Domain sentinel errors |
| `migrations/` | SQL schema (embedded into binary) |
| `testhelper/` | Shared integration test utilities |

---

## 3. Data Model

### 3.1 Schema (migration 1_initial)

```
users
  id TEXT PK (ULID)
  email, email_verified, name, given_name, family_name, picture
  created_at, updated_at

federated_identities
  id TEXT PK (ULID)
  user_id → users(id) ON DELETE CASCADE
  provider TEXT ('google' | 'github')
  provider_subject TEXT
  UNIQUE(provider, provider_subject)
  + mirrored provider claim fields (email, display_name, picture, etc.)
  last_login_at, created_at, updated_at

clients
  id TEXT PK
  secret_hash TEXT (bcrypt)
  redirect_uris TEXT[], post_logout_redirect_uris TEXT[]
  application_type, auth_method, response_types TEXT[], grant_types TEXT[]
  access_token_type, allowed_scopes TEXT[]
  id_token_lifetime_seconds INT, clock_skew_seconds INT
  id_token_userinfo_assertion BOOL

auth_requests
  id TEXT PK (ULID)
  + all OIDC request params (client_id, redirect_uri, scopes, nonce, etc.)
  code_challenge, code_challenge_method
  user_id TEXT (nullable, filled after login)
  auth_time TIMESTAMPTZ (nullable, filled after login)
  amr TEXT[] (nullable, filled after login)
  is_done BOOL
  code TEXT (nullable)
  created_at TIMESTAMPTZ
  INDEX(code) WHERE code IS NOT NULL
  INDEX(created_at)

tokens (access tokens)
  id TEXT PK (ULID)
  client_id, subject, audience TEXT[], scopes TEXT[]
  expiration TIMESTAMPTZ
  refresh_token_id TEXT (nullable, FK to refresh_tokens)
  created_at
  INDEX(subject, client_id)
  INDEX(expiration)

refresh_tokens
  id TEXT PK (ULID)
  token_hash TEXT UNIQUE (SHA-256 of raw refresh token)
  client_id, user_id → users(id)
  audience TEXT[], scopes TEXT[], amr TEXT[]
  auth_time TIMESTAMPTZ
  access_token_id TEXT (nullable)
  expiration TIMESTAMPTZ
  created_at
  INDEX(expiration)
```

### 3.2 Seeded Data (migration 2_seed_clients)

One pre-seeded client: `myaccount-bff` with:
- Redirect URI: `https://myaccount.hss-science.org/api/v1/auth/callback`
- Grant types: `authorization_code`, `refresh_token`
- Scopes: `openid`, `email`, `profile`, `offline_access`
- Access token type: `jwt`
- **Secret hash is a placeholder** — must be replaced before production.

---

## 4. Key Flows

### 4.1 OIDC Authorization Code Flow (PKCE)

```
1. Client → GET /authorize?client_id=...&code_challenge=...&code_challenge_method=S256
   → StorageAdapter.CreateAuthRequest
     - Validates PKCE is present for public clients (auth_method='none')
     - Persists AuthRequest domain object
     - Redirects to /login?authRequestID=...

2. User → GET /login?authRequestID=...
   → authn.Handler.SelectProvider → renders provider selection page

3. User → POST /login/select (provider=google|github, authRequestID=...)
   → authn.Handler.FederatedRedirect
     - Encrypts federatedState{AuthRequestID, Provider, Nonce} with AES-256-GCM
     - 302 to IdP authorization URL with encrypted state

4. IdP → GET /login/callback?code=...&state=...
   → authn.Handler.FederatedCallback
     - Decrypts state, exchanges code with IdP
     - Fetches user claims (OIDC id_token or GitHub API)
     - identity.Service.FindOrCreateByFederatedLogin (upsert user)
     - authReqSvc.CompleteLogin (sets user_id, auth_time, amr=["fed"], is_done=true)
     - 302 to op.AuthCallbackURL (zitadel internal continuation)

5. Zitadel → GET /callback (internal)
   → StorageAdapter.SaveAuthCode → stores short-lived code

6. Client → POST /oauth/v2/token (code + code_verifier)
   → StorageAdapter.CreateAccessAndRefreshTokens
     - Validates code_verifier against challenge (S256, done by zitadel)
     - Creates access token (ID = ULID, short-lived)
     - Creates refresh token (raw random bytes, stored as SHA-256 hash)
     - Returns access token ID + raw refresh token
```

### 4.2 Token Refresh Flow (with rotation)

```
Client → POST /oauth/v2/token (refresh_token=<raw>)
  → StorageAdapter.TokenRequestByRefreshToken
    - Hashes raw token, looks up refresh_token WHERE token_hash=... AND expiration > now()
  → StorageAdapter.CreateAccessAndRefreshTokens (currentRefreshToken = old raw value)
    - Transaction:
      1. Look up old access_token_id from old refresh token
      2. DELETE old access token
      3. DELETE old refresh token WHERE expiration > now() → ErrNotFound if already used (replay protection)
      4. INSERT new access token
      5. INSERT new refresh token
    - Returns new access token + new raw refresh token
```

### 4.3 User Upsert Logic

```
FindOrCreateByFederatedLogin(provider, claims):
  → FindByFederatedIdentity(provider, claims.Subject)
  If found:
    → UpdateFederatedIdentityClaims (update cached provider data)
    → UpdateUserFromClaims (sync email, name, picture etc.)
    → return updated User (claims applied in-memory)
  If not found:
    → CreateWithFederatedIdentity (transaction: INSERT users + INSERT federated_identities)
    → return new User
```

---

## 5. Security Properties

| Property | Status | Notes |
|----------|--------|-------|
| PKCE (S256) | ✅ Enforced | Mandatory for public clients; `CodeMethodS256: true` |
| Refresh token rotation | ✅ Implemented | Replay protection via `RowsAffected == 0` check |
| Refresh token storage | ✅ SHA-256 hashed | Raw values never stored |
| AES-256-GCM OAuth2 state | ✅ | Prevents CSRF and state tampering |
| RSA key rotation | ✅ | JWKS serves current + previous keys |
| Per-IP rate limiting | ✅ | Token bucket, bypassed for internal IPs |
| bcrypt client secrets | ✅ | Used for confidential clients |
| HTTPS enforcement | ⚠️ | HSTS header set but no redirect enforcement at app level |
| Content-Security-Policy | ❌ Missing | The login page has no CSP header |
| JWT profile grant | ✅ Blocked | Explicitly returns error |
| Request objects (JAR) | ✅ Blocked | `RequestObjectSupported: false` |
| ACR in ID token | ❌ Missing | `GetACR()` always returns `""` |
| AMR | ⚠️ Partial | Only `["fed"]` — no finer-grained method info |
| Client secret in seed | ❌ Placeholder | `2_seed_clients.up.sql` has a placeholder bcrypt hash |

---

## 6. What Is Missing or Not Production-Ready

### 6.1 Critical Gaps

**1. No device/session tracking**
The `refresh_tokens` table is the closest thing to a "session" but provides no device context. There is no concept of:
- Device name / user agent
- Device fingerprint / IP at login time
- Session creation timestamp (separate from token creation)
- Named sessions for the myaccount BFF to list and revoke
- `last_used_at` on sessions

This is the primary gap for the planned myaccount BFF. A user cannot currently answer "what devices am I logged in on?" or "revoke this laptop session."

**2. Seeded client secret must be replaced**
`2_seed_clients.up.sql` contains a hardcoded placeholder bcrypt hash. If shipped as-is to production, the secret for `myaccount-bff` is publicly known.

**3. No logout / session invalidation endpoint beyond TerminateSession**
`TerminateSession(ctx, userID, clientID)` deletes all tokens for a user+client pair. This covers the "log out everywhere for this app" scenario. But:
- There is no per-refresh-token (per-session) revocation at the OIDC layer except `RevokeToken`, which is more granular but not user-facing.
- There is no OIDC `end_session_endpoint` that a client can call with a hint to actually clear the user's context.
- The `POST /logged-out` is just a static page, not a real session termination flow.

**4. No admin or management API**
There is no way to programmatically:
- List clients
- Create/update/delete clients
- List users or federated identities
- Revoke all sessions for a user (admin action for account suspension)

All client management is done directly via SQL migrations.

**5. `RevokeToken` logic is ambiguous**
In `adapter/storage.go`, `RevokeToken(ctx, tokenOrTokenID, userID, clientID)` branches on `userID != ""` to decide whether to revoke an access token or a refresh token. The zitadel library passes `userID` from the refresh token lookup, so this happens to work — but the logic is fragile: if a public client passes a refresh token with an associated user, the wrong branch could be taken. The correct approach is to attempt access token revocation first and fall back to refresh token revocation (or inspect the token type, not the userID).

**6. No token introspection authentication**
`SetIntrospectionFromToken` in `storage.go` accepts any `tokenID` and returns token metadata. The zitadel library handles authentication of the caller (`Authorization: Bearer` or `client_credentials`), but there is no scope restriction on what resource servers can introspect what tokens. Any registered client with a valid credential can introspect any token.

### 6.2 Production-Quality Gaps

**7. No structured logging with request IDs**
`main.go` creates a `slog.Logger` and passes it to the OIDC provider but does not inject a request ID into the context, nor do the authn handlers log request IDs. Distributed tracing (trace/span propagation) is absent.

**8. No metrics or observability**
No Prometheus metrics, no OpenTelemetry spans. The only observability surface is structured logs. For production you'd want metrics on token issuance rate, auth request latency, DB query counts, error rates.

**9. Auth request cleanup is in-process only**
Auth requests are cleaned up by a background goroutine in the server process (every 5 minutes, hardcoded in `main.go`). The `cleanup` subcommand only handles token cleanup. If the server is killed mid-cleanup, orphaned auth requests accumulate. The cleanup goroutine interval is not configurable.

**10. No end-to-end test coverage of the login HTML flow**
The `select_provider.html` page:
- Has hardcoded `href="#"` placeholder links for Terms, Privacy, Help.
- Has a hardcoded `hss-science.org` brand name (not configurable).
- Has no CSP header protection.
- Has no CSRF protection on the `POST /login/select` form beyond the encrypted state (which lives in the redirect, not the form).
- Is not tested with browser automation.

**11. DB connection pool is not observable**
Pool settings are configurable but there is no `db.Stats()` exposure for monitoring.

**12. Cleanup goroutine timeouts**
The background cleanup goroutines in `main.go` use `context.Background()` — there is no timeout or deadline on cleanup operations, meaning a stalled DB call could block the goroutine forever.

**13. Migration runner is test-only**
`testhelper.RunMigrations` runs migrations by reading embedded SQL. But for production there is no migration management: no version table, no up/down tracking. Migrations are applied manually or via some external mechanism not visible in this codebase. A proper migration tool (e.g., `golang-migrate`) should be integrated.

**14. RSA key size is 2048-bit in tests**
`storage_test.go` generates a 2048-bit RSA key. The `config/config.go` validates `>= 2048` bits. For production, 4096-bit keys are recommended by current best practice.

**15. Key rotation has no automation**
Previous keys are provided via `SIGNING_KEY_PREVIOUS_PEM` environment variable (manual rotation). There is no automated key rotation, no key expiry, no JWKS `exp` claim.

**16. No email-based idempotency**
If the same user authenticates with Google and then GitHub (using the same email), they get **two separate user accounts** — the system only deduplicates by `(provider, provider_subject)`. There is no email-based account linking.

**17. Incomplete `GetPrivateClaimsFromScopes`**
Returns an empty map. If custom claims (e.g., roles, organization membership) are ever needed in access tokens, this is where they would go. Currently there is no claim extension point.

---

## 7. What Is Done Well

1. **Clean hexagonal architecture** — domain, ports, adapters, and infrastructure are correctly separated.
2. **Refresh token security** — SHA-256 hashing, rotation with replay protection, atomic transactions.
3. **PKCE enforcement** — S256 mandatory for public clients.
4. **AES-256-GCM state** — the federated login state is properly authenticated+encrypted, not just encoded.
5. **Rate limiting** — per-IP token buckets with sensible defaults, internal bypass, configurable via env.
6. **Test coverage** — all repository and storage adapter code is covered by integration tests using real Postgres via testcontainers. Pure unit tests cover all domain services.
7. **Config validation** — strict validation at startup with clear error messages. Fail-fast before serving traffic.
8. **Compile-time interface checks** — every `implements` relationship is asserted with `var _ Interface = (*Impl)(nil)`.
9. **Graceful shutdown** — 15s context on SIGTERM, in-flight requests allowed to drain.
10. **Key rotation support** — multiple previous keys supported in JWKS.

---

## 8. Session & Device Management — Gap Analysis for myaccount BFF

The myaccount BFF will need to present a "Devices" or "Sessions" view showing the user their active login sessions. Currently this is not possible because:

| Required Feature | Current State |
|-----------------|---------------|
| List sessions for a user | ❌ Not possible — refresh_tokens has no device context |
| Show device name / browser | ❌ No user-agent stored |
| Show login IP / location | ❌ No IP stored |
| Show session creation time | ⚠️ `refresh_tokens.created_at` approximates this |
| Show last active time | ❌ `refresh_tokens` has no `last_used_at` |
| Revoke individual session | ⚠️ `RevokeRefreshToken` exists but there's no API to call it |
| Revoke all sessions | ⚠️ `TerminateSession` is available but no user-facing API |
| Multi-device push notification | ❌ Not applicable yet |

### Recommended Schema Addition

To support the myaccount BFF, a `sessions` table should be added:

```sql
CREATE TABLE sessions (
  id           TEXT PRIMARY KEY,           -- ULID
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  client_id    TEXT NOT NULL,
  refresh_token_id TEXT REFERENCES refresh_tokens(id) ON DELETE SET NULL,

  -- device context (populated at login time)
  user_agent   TEXT,
  ip_address   TEXT,
  device_name  TEXT,                       -- parsed from user-agent

  -- lifecycle
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at   TIMESTAMPTZ,                -- null = active

  INDEX (user_id),
  INDEX (refresh_token_id)
);
```

This would require:
1. Creating a session record in `authn.Handler.FederatedCallback` at login completion (when `user_id` and the HTTP request context with headers are available).
2. Updating `last_used_at` on each token refresh (in `TokenRepository.CreateAccessAndRefresh`).
3. Linking session → refresh_token (1:1, update on rotation).
4. Exposing a `SessionRepository` with `ListByUserID`, `GetByID`, `Revoke`, `RevokeAll`.
5. A new BFF-facing HTTP API (or gRPC service) for the myaccount frontend to call.

---

## 9. Dependency Notes

| Dependency | Version | Notes |
|-----------|---------|-------|
| `zitadel/oidc/v3` | v3.45.5 | Core OIDC provider library |
| `coreos/go-oidc/v3` | v3.17.0 | Google upstream OIDC token verification |
| `go-chi/chi/v5` | v5.2.5 | HTTP router |
| `go-jose/go-jose/v4` | v4.1.3 | JWK/JWT signing |
| `jmoiron/sqlx` | v1.4.0 | SQL convenience |
| `lib/pq` | v1.11.2 | PostgreSQL driver |
| `oklog/ulid/v2` | v2.1.1 | Sortable unique IDs |
| `testcontainers-go` | v0.41.0 | Integration test containers |
| `golang.org/x/crypto` | v0.49.0 | bcrypt |
| `golang.org/x/oauth2` | v0.36.0 | OAuth2 client |
| `golang.org/x/time` | v0.15.0 | Rate limiter |

Go module version: `1.26`. All dependencies appear current.

---

## 10. File Index

```
services/accounts/
├── main.go                              # Entry point, wiring
├── Dockerfile                           # Two-stage distroless build
├── .env.example                         # Full env var documentation
├── config/
│   ├── config.go                        # Validated env → Config struct
│   └── config_test.go                   # 32 unit tests
├── internal/
│   ├── authn/
│   │   ├── config.go                    # authn.Config
│   │   ├── embed.go                     # Embed templates/
│   │   ├── handler.go                   # HTTP handlers for federated login
│   │   ├── handler_test.go              # Unit tests (10)
│   │   ├── login_usecase.go             # CompleteFederatedLogin use case
│   │   ├── provider.go                  # Provider abstraction
│   │   ├── provider_github.go           # GitHub claims provider
│   │   ├── provider_google.go           # Google OIDC claims provider
│   │   └── templates/
│   │       └── select_provider.html     # M3 provider selection page
│   ├── identity/
│   │   ├── domain.go                    # User, FederatedIdentity, FederatedClaims
│   │   ├── ports.go                     # Repository, Service interfaces
│   │   ├── service.go                   # identityService implementation
│   │   ├── service_test.go              # Unit tests (7)
│   │   └── postgres/
│   │       ├── user_repo.go             # PostgreSQL identity repository
│   │       └── user_repo_test.go        # Integration tests (5)
│   ├── middleware/
│   │   ├── ratelimit.go                 # Per-IP token bucket rate limiter
│   │   └── securityheaders.go           # X-Content-Type-Options, HSTS, etc.
│   ├── oidc/
│   │   ├── domain.go                    # AuthRequest, Client, Token, RefreshToken
│   │   ├── ports.go                     # Repository and service interfaces
│   │   ├── authrequest_svc.go           # TTL-checked auth request service
│   │   ├── authrequest_svc_test.go      # Unit tests (6)
│   │   ├── client_svc.go                # bcrypt client secret validation
│   │   ├── client_svc_test.go           # Unit tests (6)
│   │   ├── token_svc.go                 # ULID IDs, SHA-256 refresh tokens
│   │   ├── token_svc_test.go            # Unit tests (6)
│   │   ├── adapter/
│   │   │   ├── authrequest.go           # op.AuthRequest adapter
│   │   │   ├── authrequest_test.go      # 18 unit tests
│   │   │   ├── client.go                # op.Client adapter
│   │   │   ├── client_test.go           # 34 unit tests
│   │   │   ├── keys.go                  # RSA signing/public key wrappers
│   │   │   ├── keys_test.go             # Unit tests
│   │   │   ├── provider.go              # op.Provider configuration
│   │   │   ├── refreshtoken.go          # op.RefreshTokenRequest adapter
│   │   │   ├── storage.go               # StorageAdapter (op.Storage)
│   │   │   ├── storage_test.go          # 32 integration tests
│   │   │   └── userinfo.go              # UserClaims, UserClaimsSource
│   │   └── postgres/
│   │       ├── authrequest_repo.go      # PostgreSQL auth request repository
│   │       ├── client_repo.go           # PostgreSQL client repository
│   │       ├── repo_test.go             # Integration tests (11)
│   │       └── token_repo.go            # PostgreSQL token repository
│   └── pkg/
│       ├── crypto/
│       │   ├── aes.go                   # AES-256-GCM Cipher
│       │   └── aes_test.go              # Unit tests (4)
│       └── domerr/
│           ├── errors.go                # Sentinel domain errors
│           └── errors_test.go           # Unit tests
├── migrations/
│   ├── embed.go                         # Embedded FS for migrations
│   ├── 1_initial.up.sql                 # All tables
│   ├── 1_initial.down.sql               # Drop all
│   ├── 2_seed_clients.up.sql            # myaccount-bff client (placeholder secret)
│   └── 2_seed_clients.down.sql          # Remove seed
└── testhelper/
    └── testdb.go                        # RunMigrations, CleanTables
```

---

## 11. Production Elevation Checklist (Priority Order)

### P0 — Must Fix Before Production

- [ ] Replace placeholder bcrypt secret in `2_seed_clients.up.sql`
- [ ] Integrate a proper migration runner (golang-migrate or atlas) with version tracking
- [ ] Add CSP header to the login page (or via middleware for those paths)
- [ ] Upgrade RSA key size to 4096 bits and document minimum in `.env.example`

### P1 — Session/Device Management (for myaccount BFF)

- [ ] Design and implement `sessions` table (schema above)
- [ ] Populate session record at login completion in `authn.Handler.FederatedCallback`
- [ ] Update `last_used_at` on token refresh in `TokenRepository.CreateAccessAndRefresh`
- [ ] Implement `SessionRepository` with `ListByUserID`, `GetByID`, `Revoke`, `RevokeAll`
- [ ] Expose session management API (REST or gRPC) for myaccount BFF

### P2 — Operational Readiness

- [ ] Add Prometheus metrics (`/metrics` endpoint): token issuance, auth request counts, error rates, DB pool stats
- [ ] Inject request IDs into context; propagate through all log calls
- [ ] Add configurable timeout on background cleanup goroutine DB calls
- [ ] Make auth request cleanup interval configurable (currently hardcoded 5 minutes in main.go)
- [ ] Add `cleanup` subcommand support for auth requests (currently only tokens)
- [ ] Expose `db.Stats()` in health/metrics

### P3 — Security Hardening

- [ ] Fix `RevokeToken` branching logic (try access token first, fall back to refresh token)
- [ ] Consider token introspection scope restrictions (which clients can introspect which tokens)
- [ ] Implement ACR values if MFA is ever added (currently always `""`)
- [ ] Consider email-based account linking across upstream providers
- [ ] Document key rotation procedure; consider automated rotation support

### P4 — Developer Experience

- [ ] Replace hardcoded brand name (`hss-science.org`) in `select_provider.html` with config-driven variable
- [ ] Replace `href="#"` placeholder links in `select_provider.html`
- [ ] Add admin API for client management (CRUD)
- [ ] Add OpenTelemetry tracing
