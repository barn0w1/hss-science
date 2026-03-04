# OIDC Provider Architectural Evaluation

**Target:** `server/services/accounts`
**Library:** `zitadel/oidc` v3.45.5
**Date:** 2026-03-03
**Classification:** Security & Architectural Due Diligence

---

## Executive Summary

This report presents a deep architectural evaluation of the accounts service, which functions as an **identity broker** built on the `zitadel/oidc` v3 library. The service occupies a dual role: it acts as a standards-compliant OpenID Connect Provider to downstream Relying Parties while simultaneously acting as an OAuth 2.0 / OIDC Relying Party to upstream Identity Providers (Google via OIDC, GitHub via OAuth 2.0).

The implementation demonstrates strong architectural fundamentals. It follows clean hexagonal architecture with well-defined domain boundaries, implements the `op.Storage` interface with substantive rather than stub implementations, and handles security-critical operations — key management, client secret hashing, token lifecycle, and federation state encryption — with appropriate cryptographic primitives.

The system achieves **Early Production** maturity. It is architecturally sound and suitable for deployment in controlled environments with a known set of clients, but carries several gaps that should be resolved before broader production exposure: the absence of rate limiting, plaintext refresh token storage in the database, a potential race condition in token rotation, and the lack of expired token cleanup.

None of the findings indicate fundamental design flaws. The identified risks are addressable through incremental hardening without architectural redesign.

---

## 1. Architectural Overview

### System Positioning

The service is an **identity broker** (federation hub), not a pure OpenID Provider:

```
Downstream Relying Parties
        │
        ▼
 ┌──────────────────────────────┐
 │  accounts service (this OP)  │
 │                              │
 │  ┌────────────┐ ┌─────────┐ │
 │  │ zitadel/op │ │  authn  │ │
 │  │ (OP layer) │ │  (RP    │ │
 │  │            │ │  layer) │ │
 │  └────────────┘ └─────────┘ │
 │  ┌────────────────────────┐  │
 │  │   identity store (PG)  │  │
 │  └────────────────────────┘  │
 └──────────────────────────────┘
        │
        ▼
 Upstream IdPs (Google, GitHub)
```

Downstream RPs receive tokens signed by this service's own RSA key. Upstream provider subjects are never exposed; the service mints its own stable ULID-based subject identifiers. Claims from upstream providers are refreshed on every login and stored locally. This is architecturally clean identity brokering — the service owns the identity lifecycle rather than passing through upstream tokens.

### Component Architecture

The codebase follows hexagonal architecture with four distinct layers:

- **Domain layer** (`internal/oidc/domain.go`, `internal/identity/domain.go`): Pure data types with no dependencies.
- **Port layer** (`internal/oidc/ports.go`, `internal/identity/ports.go`): Interface definitions for repositories and services.
- **Service layer** (`internal/oidc/*_svc.go`, `internal/identity/service.go`): Business logic including TTL enforcement, secret verification, token ID generation, and identity find-or-create.
- **Adapter layer** (`internal/oidc/adapter/*`, `internal/oidc/postgres/*`, `internal/identity/postgres/*`): Infrastructure adapters implementing the port interfaces.

The `op.Storage` interface implementation sits in the adapter layer and translates between the `zitadel/oidc` library's expectations and the application's domain services. A compile-time interface check (`var _ op.Storage = (*StorageAdapter)(nil)`) enforces completeness.

### Library Delegation Model

The implementation correctly delegates complex OIDC protocol mechanics to the `zitadel/oidc` library: authorization code generation and encryption, PKCE verification, ID token construction (including `at_hash` and `c_hash`), discovery document generation, and JWKS endpoint serving. The application code implements only the `op.Storage` interface and the login UI, which is the intended and minimal-risk integration pattern.

---

## 2. Protocol & Specification Compliance

### Authorization Code Flow

The authorization code flow is structurally compliant with RFC 6749 Section 4.1 and OIDC Core Section 3.1. The lifecycle proceeds through well-defined states:

1. `CreateAuthRequest` persists all OAuth 2.0 and OIDC parameters with a ULID-based ID.
2. The user is redirected to `/login` where the `authn.Handler` manages federated authentication.
3. `CompleteLogin` sets `user_id`, `auth_time`, `amr`, and `is_done = true` on the auth request.
4. The library detects `Done() == true`, generates an encrypted authorization code via `SaveAuthCode`.
5. Code exchange via `AuthRequestByCode` retrieves the auth request; `DeleteAuthRequest` consumes it.

Authorization codes are single-use through the `DeleteAuthRequest` call during token exchange. The auth request TTL (configurable 1–60 minutes, default 30) provides an additional time-bound constraint: `authRequestService.GetByID` and `GetByCode` both reject requests whose `CreatedAt + TTL` has passed. A background goroutine periodically purges expired auth requests from the database.

Authorization code generation and encryption is fully delegated to the library using the configured 256-bit AES `CryptoKey`. The application stores the library's encrypted code opaquely and retrieves auth requests by matching on it.

### PKCE

PKCE with S256 is enabled in the provider configuration (`CodeMethodS256: true`). The `plain` method is not enabled — the `zitadel/oidc` library rejects plain challenges when only S256 is configured. The `CodeChallenge` and `CodeChallengeMethod` values are round-tripped through the auth request and exposed to the library via `GetCodeChallenge()`.

PKCE is not mandatory. If a client omits `code_challenge`, the flow proceeds without PKCE protection. The library supports `RequiredCodeChallengeMethod` for enforcement, but this is not configured. Per the OAuth 2.1 draft and the OAuth Security BCP, PKCE should be required for public clients. This is a compliance gap for scenarios involving `native` or `user_agent` application types.

### Client Credentials

Client credentials are validated via bcrypt comparison in `clientService.ClientCredentials`. The `StorageAdapter.ClientCredentials` method maps domain errors to OIDC error codes. The `clientCredentialsTokenRequest` struct correctly sets the client as both subject and audience, which aligns with RFC 6749 Section 4.4 semantics.

### ID Token Claims

Claims are populated by `setUserinfo` based on scope:

| Scope | Claims |
|-------|--------|
| `openid` | `sub` |
| `profile` | `name`, `given_name`, `family_name`, `picture`, `updated_at` |
| `email` | `email`, `email_verified` |

This aligns with OIDC Core Section 5.4. The `address` and `phone` scopes are not supported (the user model does not store these). The `sub` claim uses the local ULID, not the upstream provider subject — correct broker behavior.

`at_hash`, `c_hash`, `nonce`, `iss`, `aud`, `exp`, `iat`, and `auth_time` are handled by the library during ID token minting. `GetPrivateClaimsFromScopes` returns an empty map — no custom claims are added.

### Discovery and JWKS

The library generates the discovery document automatically. The signing algorithm is RS256 (the only advertised algorithm), which is the REQUIRED algorithm per OIDC Core Section 15.1. The JWKS endpoint exposes both current and previous public keys via `PublicKeySet.All()`, enabling graceful key rotation.

Key IDs are derived deterministically from the SHA-256 hash of the DER-encoded public key, truncated to 8 bytes (16 hex characters). This ensures signing key and JWKS key IDs match. The truncation is sufficient for key discrimination purposes.

### Introspection

`SetIntrospectionFromToken` correctly returns `Active: false` (without error) when a token is not found or expired, per RFC 7662 Section 2.2. For active tokens, it populates the full standard response fields including user claims based on the token's scopes.

### Error Handling

The `internalErr` helper logs real errors via `slog.Error` and returns sanitized `oidc.ErrServerError().WithDescription("internal error")` to clients. Domain errors are mapped to appropriate OIDC error codes:

- `domerr.ErrNotFound` for auth requests → `oidc.ErrInvalidRequest()`
- `domerr.ErrNotFound` for clients → `oidc.ErrInvalidClient()`
- `domerr.ErrUnauthorized` for secrets → `oidc.ErrInvalidClient()`
- `domerr.ErrNotFound` for refresh tokens → `op.ErrInvalidRefreshToken`

No internal error details leak to clients.

---

## 3. Interface Implementation Quality

### op.Storage Completeness

Of the 25 `op.Storage` methods, 23 are substantively implemented. Two methods — `GetKeyByIDAndClientID` and `ValidateJWTProfileScopes` — are intentional stubs returning "jwt profile grant not supported", which is consistent with the provider configuration (`AuthMethodPrivateKeyJWT: false`). No methods panic or silently swallow errors.

### Adapter Quality

The adapter layer is a clean translation layer. The `AuthRequest` adapter exposes all 15 interface methods with correct type mappings (string ↔ `oidc.ResponseType`, string ↔ `oidc.ResponseMode`). The `ClientAdapter` correctly maps all application types, auth methods, grant types, and response types. Scope filtering via `IsScopeAllowed` and `RestrictAdditionalAccessTokenScopes` operates against the client's `AllowedScopes` field from the database.

The `RefreshTokenRequest` adapter properly wraps the domain refresh token, implementing `SetCurrentScopes` for scope downscoping during refresh, and exposing `GetAuthTime()` and `GetAMR()` for propagation into new tokens.

### Service Layer Substance

The services are not mere pass-throughs. They add meaningful business logic:

- **AuthRequestService**: Enforces TTL-based expiration in `GetByID` and `GetByCode`.
- **ClientService**: Performs bcrypt-based secret verification in `ClientCredentials`.
- **TokenService**: Generates ULID-based identifiers for all token artifacts; constructs properly cross-referenced access/refresh token pairs.

### Lifecycle Enforcement

The auth request lifecycle (create → code save → login complete → token exchange → delete) is properly modeled. The `IsDone` flag gates the transition from login to code generation. The TTL provides a hard timeout. Code exchange via `DeleteAuthRequest` prevents reuse.

One subtlety: authorization code one-time-use is enforced by the library calling `DeleteAuthRequest`, not by the storage layer. The `GetByCode` query does not use `SELECT ... FOR UPDATE` or `DELETE ... RETURNING`. Under high concurrency, two token exchange requests with the same code could theoretically both succeed before the delete occurs. In practice, the library serializes this internally, but the database layer does not provide an independent guarantee.

---

## 4. Security Architecture

### Signing Key Management

RSA keys are loaded from PEM-encoded environment variables through `config.parseRSAPrivateKey`, which supports both PKCS#1 and PKCS#8 formats. A mandatory minimum key size of 2048 bits is enforced at load time. Key rotation is supported via `SigningKeySet.Previous`: previous keys are exposed in the JWKS endpoint for validation but are never used for signing. Key IDs are content-addressed via SHA-256 of the DER-encoded public key.

Keys are static for the process lifetime. There is no runtime key rotation or scheduled key refresh. Rotation requires redeployment with updated configuration. This is a pragmatic constraint, not a flaw — it is appropriate for the deployment model.

### Client Secret Security

Client secrets are stored as bcrypt hashes. Verification uses `bcrypt.CompareHashAndPassword` from `golang.org/x/crypto/bcrypt`, which is timing-safe by design. The seed migration uses `$2a$10$` cost factor, which is a reasonable production baseline. The application does not enforce a minimum bcrypt cost factor on stored hashes, but this is a configuration concern rather than a code vulnerability.

### Token Security

Access tokens are JWTs signed with the RSA key. Their IDs serve as the `jti` claim. The database stores only the token metadata (not the JWT itself), enabling both stateless validation (JWT signature check) and stateful introspection (database lookup).

Refresh tokens are opaque ULIDs stored server-side. They are rotated on use: `CreateAccessAndRefresh` accepts `currentRefreshToken` and the repository atomically deletes the old pair and creates the new one within a transaction.

**Finding: Refresh tokens stored in plaintext.** The `refresh_tokens.token` column stores the raw opaque value without hashing. A database breach would expose all active refresh tokens. Best practice is to store a SHA-256 hash and compare against it during presentation. This is the highest-severity data-at-rest concern.

**Finding: No refresh token reuse detection.** Old refresh tokens are hard-deleted on rotation. If an attacker uses a stolen refresh token before the legitimate client rotates it, the attacker obtains a valid new pair. The legitimate client's subsequent rotation fails, but there is no mechanism to revoke the entire token family. This limits the effectiveness of rotation as a theft detection strategy.

**Finding: Token rotation race condition.** The `CreateAccessAndRefresh` implementation does not check `RowsAffected()` on the old refresh token deletion. Two concurrent requests with the same refresh token could both succeed, minting two new token pairs. A `SELECT ... FOR UPDATE` or row-affected check would close this window.

### Federation State Security

The federated login state is encrypted with AES-256-GCM using a server-side 256-bit key. The state struct includes the auth request ID, provider name, and a random UUID nonce. GCM provides both confidentiality and integrity, preventing state forgery and tampering without the server's key.

The in-state nonce adds uniqueness to the ciphertext but is not validated server-side against a stored value. This theoretically permits replay of a captured encrypted state if the attacker also possesses a valid upstream authorization code. The practical exploitability is very low.

Google integration uses proper OIDC discovery with ID token verification (signature, audience, expiry, issuer) via the `go-oidc` library's verifier. GitHub integration uses plain OAuth 2.0 with a hardcoded API URL, preventing SSRF. GitHub's `email_verified` is conservatively not set (`false`).

### HTTP Security Posture

Server timeouts are fully configured: `ReadHeaderTimeout` (5s), `ReadTimeout` (10s), `WriteTimeout` (30s), `IdleTimeout` (120s). The `Recoverer` middleware catches panics.

**Gaps:**
- **No rate limiting** on any endpoint. The token endpoint, client credentials endpoint, and login flow are all unbounded. This is the most significant operational security gap.
- **No security response headers** (CSP, HSTS, X-Frame-Options, X-Content-Type-Options).
- **No CORS policy** defined. If SPAs call the token endpoint directly, CORS headers would be needed.
- **No TLS** at the application layer (assumed reverse proxy termination, but not documented).

### SQL Injection

All database queries use parameterized placeholders (`$1`, `$2`, ..., `$N`) with `database/sql`. PostgreSQL arrays are handled via `pq.Array()`. No string concatenation of user-supplied values exists in production queries. SQL injection risk is effectively mitigated.

---

## 5. Data & Persistence Layer

### Schema Design

The schema consists of five tables (`users`, `federated_identities`, `clients`, `auth_requests`, `tokens`, `refresh_tokens`). All primary keys are TEXT (ULID strings). Multi-valued attributes (scopes, URIs, grant types) use PostgreSQL `TEXT[]` arrays rather than join tables, which simplifies queries at the cost of per-element referential integrity.

Foreign key relationships are selectively applied:

| Relationship | Enforced |
|---|---|
| `federated_identities.user_id` → `users.id` | Yes, `ON DELETE CASCADE` |
| `refresh_tokens.user_id` → `users.id` | Yes, no cascade |
| `auth_requests.client_id` → `clients.id` | No |
| `tokens.client_id` → `clients.id` | No |
| `tokens.subject` → `users.id` | No |
| `tokens.refresh_token_id` → `refresh_tokens.id` | No |

The absence of foreign keys on token tables is a deliberate trade-off for independent lifecycle management (tokens can outlive client configuration changes), but it means the database cannot enforce referential consistency for these relationships.

### Indexing

The indexing strategy is targeted but minimal:

- `auth_requests_code_idx` — partial index on `code WHERE code IS NOT NULL` (efficient for code exchange).
- `auth_requests_created_at_idx` — supports cleanup queries.
- `tokens_subject_client_idx` — composite index for `DeleteByUserAndClient`.
- `federated_identities_user_id_idx` — supports reverse lookups.
- `refresh_tokens.token` — implicit unique index.

**Missing:** No indexes on `tokens.expiration` or `refresh_tokens.expiration` (would be needed for cleanup). No composite index on `refresh_tokens(user_id, client_id)` for the session termination query.

### Transaction Boundaries

Three operations use explicit transactions with the `defer tx.Rollback()` pattern:

1. **Token rotation** (`CreateAccessAndRefresh`): Atomically deletes old pair + creates new pair.
2. **Session termination** (`DeleteByUserAndClient`): Atomically deletes all tokens for a user-client pair.
3. **User creation** (`CreateWithFederatedIdentity`): Atomically creates user + initial federated identity.

The user update on returning login (`FindOrCreateByFederatedLogin`) performs two non-transactional writes (`UpdateUserFromClaims` + `UpdateFederatedIdentityClaims`). A failure between them creates a temporary inconsistency that self-heals on next login.

### Expired Data Accretion

Auth requests are cleaned up by a background goroutine. **Tokens and refresh tokens have no cleanup mechanism.** Expired tokens are logically invisible (queries include `AND expiration > now()`) but physically persist indefinitely. The `tokens` table is particularly susceptible to unbounded growth given short-lived access tokens (15-minute default) created on every authentication. Connection pool settings are left at Go's defaults (`MaxOpenConns = 0`, unbounded), which could exhaust database connections under load.

---

## 6. Identity & Federation

### Identity Lifecycle

Users are created via a **provider-subject-based find-or-create** pattern in `identity.Service.FindOrCreateByFederatedLogin`. The lookup key is `(provider, provider_subject)`, not email. This is a security-positive design: it prevents the classic email-based account takeover where an attacker registers at GitHub with a victim's email to hijack their Google-linked account.

The trade-off is the absence of cross-provider linking. A user who authenticates via Google and later via GitHub will create two separate local accounts even if they share an email address. The schema supports multi-provider linking (`federated_identities` has a foreign key to `users`, and multiple rows can reference the same `user_id`), but the service code provides no path to exercise this capability.

Claims from upstream providers are refreshed on every login: both the `users` and `federated_identities` tables are updated with the latest values. This ensures profile data stays current.

### Upstream Provider Design

The provider abstraction (`authn.ClaimsProvider` interface) is well-designed for OAuth 2.0 and OIDC upstream providers. Each provider implements `FetchClaims(ctx, token) → FederatedClaims`, decoupling protocol-specific claim extraction from the identity service. Adding a new OAuth 2.0 or OIDC provider requires a new implementation file and registration in `NewProviders` — approximately three files changed.

The abstraction would not directly accommodate SAML upstream providers, which use different protocol mechanics (assertions rather than token exchange). Supporting SAML would require refactoring the `Provider` struct's `*oauth2.Config` dependency into a more abstract interface.

### AMR (Authentication Methods References)

AMR values are set as `["federated:<provider>"]` (e.g., `["federated:google"]`). These do not follow RFC 8176 registered values and do not reflect the actual authentication method used at the upstream provider. The values are properly stored, persisted in auth requests, and propagated through to refresh tokens. `auth_time` is set to the callback processing time rather than the upstream provider's `auth_time` claim, which may not accurately represent when the user actually authenticated.

---

## 7. Risk Assessment

### High Severity

| Risk | Impact | Description |
|------|--------|-------------|
| No rate limiting | Availability, Security | All endpoints are unbounded. Token endpoint, client credentials, and login flow are susceptible to brute-force, credential stuffing, and resource exhaustion. Bcrypt's inherent slowness mitigates secret brute-force somewhat, but also amplifies CPU-based DoS potential. |
| Plaintext refresh token storage | Confidentiality | A database breach exposes all active refresh tokens, enabling session takeover for every active user. Best practice is to store a SHA-256 hash. |

### Medium Severity

| Risk | Impact | Description |
|------|--------|-------------|
| Token rotation race condition | Security | Concurrent use of the same refresh token can mint multiple new token pairs. The database layer does not enforce serialization or check rows-affected on old token deletion. |
| No expired token cleanup | Operational | The `tokens` and `refresh_tokens` tables grow indefinitely with expired rows, degrading query performance and increasing storage costs over time. |
| No security headers | Security | Missing CSP, HSTS, X-Frame-Options, and X-Content-Type-Options on the login HTML page. |
| PKCE not mandatory for public clients | Security | Public clients can complete the authorization code flow without PKCE, contrary to OAuth 2.1 and BCP recommendations. |
| Unbounded connection pool | Operational | Default `database/sql` pool settings allow unlimited connections to PostgreSQL, risking database connection exhaustion under load. |

### Low Severity

| Risk | Impact | Description |
|------|--------|-------------|
| No refresh token reuse detection | Security | Hard deletion of rotated tokens prevents detecting token family compromise. |
| Revocation ignores client ID | Spec compliance | `RevokeToken` discards the client ID parameter, allowing any authenticated client to revoke another client's tokens by ID (mitigated by token ID entropy). |
| Federation state nonce not validated | Security | The UUID nonce in the encrypted state is not checked server-side, creating a theoretical replay window (mitigated by AES-GCM integrity and upstream code expiry). |
| Non-standard AMR values | Spec compliance | `federated:*` values are not RFC 8176-registered. |
| GitHub email may be empty | Data quality | No `/user/emails` API call means GitHub users may lack email addresses. |

---

## 8. Maturity Assessment

### Classification: **Early Production**

| Maturity Level | Description | Assessment |
|---|---|---|
| Experimental | Proof of concept, not for production | — |
| MVP | Minimal viable, significant gaps | — |
| **Early Production** | **Sound architecture, deployable with known limitations** | **This system** |
| Production-Grade | Hardened, operationally mature | Not yet |
| Enterprise-Ready | Multi-tenant, compliant, highly available | Not yet |

### Justification

**What earns Early Production:**

1. **Architecturally sound**: Clean hexagonal architecture with proper separation of concerns. No fundamental design flaws that would require rearchitecting.
2. **Correct protocol implementation**: The authorization code flow, PKCE, client credentials, token lifecycle, introspection, and key management all function correctly. The library delegation model minimizes custom protocol code.
3. **Strong cryptographic foundations**: RSA-2048+ signing keys, bcrypt for secrets, AES-256-GCM for state encryption, ULID for identifiers. No homebrew cryptography.
4. **Transactional integrity for critical operations**: Token rotation, session termination, and user creation use proper database transactions.
5. **Test infrastructure**: Integration tests against real PostgreSQL via testcontainers, unit tests with mocks for service layer. This is above-average for a service at this stage.
6. **Federation security**: Encrypted state parameters, proper ID token verification for Google, hardcoded URLs preventing SSRF.

**What prevents Production-Grade:**

1. **No rate limiting**: This is a blocking gap for any internet-facing deployment.
2. **Plaintext refresh token storage**: A standard security expectation for token storage that is not met.
3. **No expired data cleanup**: Would cause operational degradation over time.
4. **Connection pool not configured**: Would cause issues under load.
5. **Race condition in token rotation**: Could be exploited under specific conditions.
6. **No security headers**: Expected for any web-facing service.

**What would be needed for Enterprise-Ready:**

- Multi-tenant data isolation
- Dynamic client registration (RFC 7591)
- Account linking across providers
- Custom claims enrichment via `GetPrivateClaimsFromScopes`
- HSM or KMS integration for signing keys
- Runtime key rotation without redeployment
- Audit logging
- Back-channel logout (OIDC Back-Channel Logout 1.0)
- Token family revocation for refresh token compromise detection
- Observability instrumentation (metrics, distributed tracing)

---

## 9. Architectural Strengths

**Library delegation model.** By implementing only the `op.Storage` interface and deferring all protocol mechanics to `zitadel/oidc`, the service minimizes the surface area of custom OIDC code. This is the highest-leverage architectural decision in the codebase — it means the service benefits from the library's protocol correctness without having to reimplement authorization endpoint logic, token signing, PKCE verification, or discovery document generation.

**Hexagonal architecture.** The domain/port/adapter separation is rigorous. Domain types have no external dependencies. Services operate against port interfaces. The adapter layer handles translation. This makes the system testable (mocks for unit tests, real database for integration tests) and extensible (new storage backends or upstream providers can be added without touching domain logic).

**Identity broker design.** The decision to mint local subject identifiers rather than pass through upstream subjects is architecturally significant. It decouples downstream RPs from upstream provider identity schemas, enables stable subject identifiers across provider changes, and positions the service as the authoritative identity source.

**Scope-gated claims.** The `setUserinfo` method correctly gates claim population on the requested scope, ensuring that tokens never contain more information than the client is authorized to access. The client-level `AllowedScopes` field provides an additional restriction layer.

**Defensive configuration validation.** The `config.Load` function validates all critical parameters — issuer URL format, key sizes, TTL bounds, required environment variables — and fails hard at startup rather than silently using insecure defaults.

---

## 10. Conclusion

This OIDC provider implementation is a well-architected system built on sound engineering principles. The library delegation model, hexagonal architecture, and security-conscious design decisions (bcrypt, AES-256-GCM, RSA-2048+, PKCE S256, encrypted federation state) indicate a mature understanding of OIDC protocol requirements and threat landscape.

The gaps identified — rate limiting, plaintext refresh token storage, expired token cleanup, token rotation race condition — are operationally significant but architecturally minor. They can be addressed incrementally without redesigning the system's core architecture or reworking its integration with the `zitadel/oidc` library.

For the current deployment context (a controlled set of known clients, internet-facing with reverse proxy), the system is deployable with appropriate operational mitigations (WAF-level rate limiting, database maintenance schedules). Resolving the high and medium severity findings would bring it to Production-Grade maturity.
