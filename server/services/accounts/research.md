# Architecture Review & Reengineering Master Plan: `accounts` Microservice

> **Scope**: Based on deep analysis of `config`, `internal/authn`, `internal/identity`, `internal/oidc`, and `internal/pkg` packages.
> **Date**: 2026-03-02

---

## 1. Executive Summary

The `accounts` microservice is the **Identity Provider (IdP) and OIDC authorization server** for the HSS platform. It owns the complete user authentication lifecycle: brokering federated logins through external OAuth2 providers (Google, GitHub), managing persistent user identities, and issuing OIDC-compliant tokens to downstream clients.

### Structural Overview

```
config/         → Bootstrap: environment parsing, key loading, validation
internal/
  authn/        → Federated auth: OAuth2 code flow with Google/GitHub
  identity/     → Core domain: user + federated identity persistence
  oidc/         → OIDC protocol: auth requests, token issuance, adaptation to Zitadel
    adapter/    → Zitadel op.Storage bridge
    postgres/   → PostgreSQL repositories
  pkg/
    crypto/     → AES-256-GCM authenticated encryption
    domerr/     → Domain sentinel errors
```

### Current State Assessment

The service is in an **"early production" state**: the core protocol flows are functionally correct, backed by integration tests, and built on sound third-party libraries (`zitadel/oidc/v3`, `go-oidc/v3`). However, it exhibits **structural debt accumulated through iterative feature delivery** rather than upfront architecture planning. The most pressing concerns are security-class issues in the token lifecycle and tight coupling that will resist future evolution.

**Verdict**: The service works correctly for its primary use case today. It is not yet hardened for adversarial production conditions or multi-provider user account linking — both of which are likely requirements as the platform scales.

---

## 2. Architecture & Design Assessment

### 2.1 Layer Mapping Against Clean Architecture

| Layer | Expected Role | Actual Files | Compliance |
|---|---|---|---|
| Domain Entities | Core business rules, no external deps | `identity/domain.go`, `oidc/domain.go` | ✅ Good |
| Use Case / Application Service | Orchestrates domain, no HTTP/DB concerns | `identity/service.go`, `oidc/*_svc.go` | ⚠️ Partial |
| Interface Adapters | Translate between use cases and frameworks | `oidc/adapter/`, `authn/handler.go` | ⚠️ Partial |
| Frameworks & Drivers | HTTP, PostgreSQL, external libraries | `oidc/postgres/`, `config/`, `main.go` | ✅ Mostly correct |

### 2.2 What Works Well

**`identity` package — Strongest architectural example.** It cleanly demonstrates ports-and-adapters:
- `ports.go` defines `Repository` and `Service` interfaces (the ports)
- `service.go` contains pure domain logic
- `postgres/user_repo.go` is a concrete adapter, hidden behind the `Repository` interface
- Domain models (`User`, `FederatedIdentity`, `FederatedClaims`) contain zero infrastructure concerns

**`oidc` package — Correct intent, partial execution.** The service/repository separation is present and the `adapter/` subdirectory correctly isolates the Zitadel protocol concern from the domain. The three service interfaces (`AuthRequestService`, `ClientService`, `TokenService`) form a coherent set of ports. PostgreSQL implementations are properly hidden behind them.

**`pkg/domerr` — Correctly cross-cutting.** Sentinel errors are defined once and consumed across all layers, enabling consistent error propagation and `errors.Is()` matching.

### 2.3 Architectural Violations

#### 2.3.1 The `authn` Handler Violates Single Responsibility
`authn/handler.go`'s `FederatedCallback` method conflates three distinct concerns:
1. **OAuth2 protocol** — code exchange, token validation
2. **Domain logic** — identity find-or-create, AMR construction
3. **Integration signaling** — `CompleteLogin()` RPC to the OIDC layer

This makes the handler effectively an "application service" embedded in a framework adapter. The domain-level "complete a federated login" flow has no explicit service or use-case type representing it.

#### 2.3.2 `authn` Package Crosses a Dependency Boundary
```
authn → identity (domain types FederatedClaims, User)
authn → oidc domain (AuthRequestQuerier, defined locally in handler.go)
```
The `AuthRequestQuerier` interface is defined inside `authn/handler.go`, yet it represents a contract against the OIDC domain's `AuthRequestService`. This inverts dependency ownership: the `authn` package is reaching into OIDC territory. The interface should be defined in the `oidc` package and imported.

#### 2.3.3 `oidc/adapter` Tightly Couples to `identity.Service`
The `StorageAdapter` in `oidc/adapter/storage.go` directly holds an `identity.Service` reference to resolve user claims. This creates a **cross-domain dependency at the adapter layer**: the OIDC adapter is aware of the identity domain's internal models. If `identity.Service` becomes unavailable, `SetUserinfoFromScopes`, `SetUserinfoFromToken`, and introspection all fail with no graceful degradation path.

#### 2.3.4 `config` Package Has No Port Abstraction
`config.Load()` reads directly from `os.Getenv()`. There is no `ConfigSource` interface. This means:
- Unit tests must mutate global process environment (`t.Setenv`)
- Alternative configuration sources (Vault, Kubernetes secrets, config files) require code changes
- It is a pure infrastructure adapter masquerading as a domain concern

#### 2.3.5 Provider Logic is a Function Pointer, Not a Port
`authn.Provider.FetchClaims` is a `func(ctx, *oauth2.Token) (*FederatedClaims, error)` field. This is not an interface. It cannot be mocked via interface substitution, cannot be composed with cross-cutting behaviors (logging, retry logic), and violates the Ports & Adapters pattern for provider-specific claim extraction.

### 2.4 Domain Model Concerns

**`identity`: One-to-one user/federated identity constraint is a design flaw.** The database has a `UNIQUE` constraint on `federated_identities.user_id`, enforcing a 1:1 `User` ↔ `FederatedIdentity` relationship. This prevents a user from linking both Google and GitHub accounts to the same identity — a near-universal expectation in modern IdP systems. This is the single most impactful domain model limitation.

**`identity`: `User.Email` does not sync on re-login.** The email is captured once at user creation from `FederatedClaims`. Subsequent logins only update `FederatedIdentity` fields. If a user changes their email with Google, `User.Email` becomes stale silently.

**`oidc`: `AuthRequest.Audience` is hardcoded to `[ClientID]`.** The `AuthRequest` adapter's `GetAudience()` always returns `[]string{a.domain.ClientID}`. Multi-audience tokens (e.g., resource servers that need to validate the audience claim) are not supported.

---

## 3. Security & OIDC Compliance Evaluation

### 3.1 Cryptographic Posture

| Primitive | Usage | Assessment |
|---|---|---|
| AES-256-GCM | OAuth state parameter encryption (`pkg/crypto`) | ✅ Correct — authenticated encryption, random nonce per call |
| RSA-2048 / RS256 | JWT signing (`oidc/adapter/keys.go`) | ⚠️ Functional but weak — 2048-bit minimum not enforced at config load; RS256 only |
| bcrypt | Client secret hashing (`oidc/client_svc.go`) | ✅ Correct — constant-time comparison |
| SHA-256 | Key ID derivation from public key (`adapter/keys.go`) | ✅ Correct — deterministic, collision resistant |
| UUID v4 | Nonce in OAuth state (`authn/handler.go`) | ✅ Correct — cryptographically random |
| ULID | Token and request IDs | ✅ Correct — monotonic, URL-safe, unpredictable |

**Critical Gap — RSA Key Size Not Enforced**: `config.parseRSAPrivateKey` performs no bit-length validation. A 512-bit RSA key would be accepted and used to sign production JWTs. The minimum of 2048 bits must be enforced at startup.

**Critical Gap — No Signing Key Rotation Strategy**: The RSA signing key is a single static key injected at startup. There is no versioning, no expiry, and no mechanism to add a successor key while allowing existing tokens (signed with the old key) to remain valid. The OIDC `/.well-known/jwks.json` endpoint can only expose one key. This is a single point of cryptographic failure.

**Concern — Nonce Birthday Paradox**: `pkg/crypto.Encrypt` generates a fresh 96-bit (12-byte) GCM nonce via `crypto/rand` per call. For AES-GCM, the probability of nonce collision becomes significant at approximately 2^32 encryptions under the same key (~4 billion calls). For an OAuth state parameter encrypted with a long-lived server key, this is a production risk if the service runs for years at high volume.

### 3.2 OIDC Protocol Compliance

**Supported (confirmed from code and tests):**
- Authorization Code Flow with PKCE (S256)
- Refresh Token Grant with token rotation
- Client Secret Basic and Post authentication
- UserInfo endpoint with `openid`, `email`, `profile` scopes
- Token introspection
- RS256 JWT access tokens

**Not Supported / Gaps:**
- **`private_key_jwt` client auth**: Disabled (`AuthMethodPrivateKeyJWT: false`) in `oidc/adapter/provider.go`. This limits support for confidential clients following modern best practices (RFC 7521).
- **Request Objects**: Disabled (`RequestObjectSupported: false`). Prevents JAR (JWT-Secured Authorization Request) support required by FAPI profiles.
- **Token Binding / DPoP**: No evidence of implementation.
- **Pushed Authorization Requests (PAR)**: Not present.
- **`acr_values`**: `GetACR()` on `AuthRequest` adapter always returns `""`.

These absences are **acceptable for the current use case** but are documented here as blockers for FAPI or high-assurance profiles.

### 3.3 Token Lifecycle Security Vulnerabilities

#### CRITICAL: Access Token Is Not Revoked on Refresh Token Rotation
**Location**: `oidc/postgres/token_repo.go`, `CreateAccessAndRefresh`

The refresh token rotation logic correctly deletes the old refresh token in a transaction. However, the **associated access token is not deleted**. The old access token remains valid in the database (subject to its expiration window — up to 60 minutes by default).

An attacker who intercepts a refresh token can:
1. Use the refresh token to rotate it (getting a new pair)
2. Continue using the old access token until it expires

This violates RFC 6819 §5.2.2.3 (Refresh Token Rotation) and the intent of rotation-based revocation. The fix is to delete the old access token (by `access_token_id` on the refresh token record) in the same transaction as rotation.

#### HIGH: Expired Auth Requests Accumulate Without Cleanup
**Location**: `oidc/authrequest_svc.go`

TTL enforcement for `AuthRequest` records is performed **in-memory after retrieval** from PostgreSQL:
```go
if time.Now().UTC().After(ar.CreatedAt.Add(s.authRequestTTL)) {
    return nil, fmt.Errorf("auth request expired: %w", domerr.ErrNotFound)
}
```
Expired records are fetched from the database and then discarded. They are never deleted. Over time, the `auth_requests` table will accumulate unbounded expired rows, degrading query performance and consuming storage. There is no vacuum/cleanup job.

#### MEDIUM: Scope Validation is Effectively Bypassed
**Location**: `oidc/adapter/client.go`

`ClientAdapter.IsScopeAllowed(scope)` always returns `false`. The comment in the research report flags this as potentially "dead code." The Zitadel framework calls this method to determine if a client is permitted to request a given scope. The current implementation means **scope restrictions configured on a client are not enforced by this adapter**. Scope validation is either handled entirely inside Zitadel's framework logic (which should be verified) or it is silently absent.

#### MEDIUM: Internal Errors Leak into OIDC Protocol Responses
**Location**: `oidc/adapter/storage.go`

```go
ar, err := s.authReqs.GetByID(ctx, id)
if err != nil {
    if domerr.Is(err, domerr.ErrNotFound) {
        return nil, oidc.ErrInvalidRequest().WithDescription("auth request not found")
    }
    return nil, err  // Raw internal error returned to protocol layer
}
```
Database errors, context cancellations, and panics bubble up as raw Go errors into Zitadel's protocol handler. Depending on how Zitadel surfaces these, they may appear in OIDC error responses, leaking internal implementation details to OAuth clients.

### 3.4 CSRF and State Parameter Security

The OAuth state parameter is AES-256-GCM encrypted and contains an embedded UUID nonce. This provides solid CSRF protection for the federated login flow. However:
- There is no secondary defense (no `SameSite` cookie, no referrer validation)
- The callback URL after login (`callbackURL func(ctx, string) string`) is a function pointer with no explicit redirect URI whitelist validation within `authn`. Validation must occur in the implementation passed at initialization.

### 3.5 Rate Limiting & Abuse Resistance

No rate limiting, brute-force protection, or request frequency controls exist anywhere in the service. `SelectProvider`, `FederatedRedirect`, and `FederatedCallback` are all unconstrained. The token endpoint (managed by Zitadel) may have framework-level limits, but this is not confirmed. A circuit breaker or token bucket should be applied at the `authn` handler level and ideally at the infrastructure (reverse proxy) level.

---

## 4. Consolidated Technical Debt

### 4.1 Security-Class Debt (Must Fix)

| ID | Issue | Package | Severity |
|---|---|---|---|
| S-1 | Old access token not revoked during refresh token rotation | `oidc/postgres` | **Critical** |
| S-2 | RSA key size not validated at startup (accepts <2048-bit keys) | `config` | **High** |
| S-3 | No signing key rotation strategy (single static key) | `oidc/adapter`, `config` | **High** |
| S-4 | AES-256-GCM nonce birthday paradox with long-lived server key | `pkg/crypto` | **Medium** |
| S-5 | `IsScopeAllowed` always returns false — scope enforcement unclear | `oidc/adapter` | **Medium** |
| S-6 | Raw internal errors leak into OIDC protocol responses | `oidc/adapter` | **Medium** |
| S-7 | No rate limiting on authn HTTP handlers | `authn` | **Medium** |

### 4.2 Structural / Architectural Debt

| ID | Issue | Package | Impact |
|---|---|---|---|
| A-1 | 1:1 User ↔ FederatedIdentity constraint blocks multi-provider linking | `identity`, DB schema | Growth blocker |
| A-2 | `AuthRequestQuerier` interface defined in `authn`, not `oidc` | `authn` | Wrong ownership |
| A-3 | `StorageAdapter` tightly coupled to `identity.Service` with no fallback | `oidc/adapter` | Availability risk |
| A-4 | `authn.Handler.FetchClaims` is a function pointer, not a port interface | `authn` | Untestable |
| A-5 | `authn.FederatedCallback` mixes protocol, domain, and integration concerns | `authn` | SRP violation |
| A-6 | `config.Load()` directly reads `os.Getenv()` — no abstraction | `config` | Inflexibility |
| A-7 | `pkg/crypto` has no interface — all callers directly depend on concrete functions | `pkg/crypto` | Untestable |

### 4.3 Operational / Data Quality Debt

| ID | Issue | Package | Impact |
|---|---|---|---|
| O-1 | Expired `auth_requests` never deleted — unbounded accumulation | `oidc` | DB performance |
| O-2 | `User.Email` not synced on re-login — drift from provider | `identity` | Data integrity |
| O-3 | OIDC issuer URL not validated as a well-formed URL at startup | `config` | Latent runtime failure |
| O-4 | No database connection validation at config load | `config` | Deferred failure |
| O-5 | GitHub provider creates a new `http.Client` per claim fetch — no pooling | `authn` | Performance |
| O-6 | Google OIDC discovery fetched on every service startup (no cache) | `authn` | Startup reliability |

### 4.4 Code Quality Debt

| ID | Issue | Package | Impact |
|---|---|---|---|
| Q-1 | Token lifetime validation logic duplicated 3x in `Load()` | `config` | Maintainability |
| Q-2 | `domerr.Is()` is a redundant wrapper over `errors.Is()` | `pkg/domerr` | Misleading abstraction |
| Q-3 | Verbose nullable pointer scanning boilerplate in every repo method | `oidc/postgres`, `identity/postgres` | Maintainability |
| Q-4 | Type assertion chains in `clientIDFromRequest()` with silent zero-value fallback | `oidc/adapter` | Fragile |
| Q-5 | Silent template rendering failure in `SelectProvider` — logs error but continues | `authn` | UX / monitoring blind spot |
| Q-6 | `AuthRequestInfo` returned by `authn.AuthRequestQuerier.GetByID` is an empty struct | `authn` | Dead type |
| Q-7 | AMR value hardcoded as `[]string{"federated"}` — no provider specificity | `authn` | Limited audit data |
| Q-8 | Error wrapping format inconsistent across packages | all | Debuggability |

### 4.5 Missing Functionality Debt

| ID | Issue | Package | Impact |
|---|---|---|---|
| M-1 | No logout / federated session revocation handler | `authn` | Incomplete OIDC |
| M-2 | No distributed tracing or correlation IDs in any handler | all | Observability |
| M-3 | No metrics instrumentation (login success/failure rates, token issuance) | all | Monitoring |
| M-4 | No `doc.go` files documenting package contracts | `oidc`, `authn`, `identity` | Developer onboarding |
| M-5 | No provider discovery endpoint (clients must hardcode provider list) | `authn` | Tight coupling to UI |

---

## 5. Reengineering & Refactoring Roadmap

The roadmap is organized into three phases, ordered by risk reduction and foundational impact. Items within each phase are sequenced to minimize dependencies between changes.

---

### Phase 1: Quick Wins & Critical Security Fixes

**Goal**: Eliminate security vulnerabilities and obvious code smells with low structural impact.

#### 1.1 Fix Access Token Revocation During Refresh Rotation [S-1]
**File**: `oidc/postgres/token_repo.go` — `CreateAccessAndRefresh`

In the same transaction that deletes the old refresh token, look up and delete its `access_token_id`:
```sql
-- Step 1: Get old access token ID
SELECT access_token_id FROM refresh_tokens WHERE token = $1

-- Step 2: Delete old access token
DELETE FROM tokens WHERE id = $1

-- Step 3: Delete old refresh token
DELETE FROM refresh_tokens WHERE token = $1

-- Step 4: Insert new access token + new refresh token
```
This is a single-transaction, additive SQL change. No interface changes required.

#### 1.2 Enforce RSA Key Size at Startup [S-2]
**File**: `config/config.go` — `parseRSAPrivateKey`

After parsing, add:
```go
if rsaKey.N.BitLen() < 2048 {
    return nil, fmt.Errorf("SIGNING_KEY_PEM must be at least 2048 bits, got %d", rsaKey.N.BitLen())
}
```

#### 1.3 Validate ISSUER URL Format at Startup [O-3]
**File**: `config/config.go` — `Load()`

Parse the issuer with `url.Parse()` and enforce HTTPS scheme in non-development environments.

#### 1.4 Fix Silent Template Rendering Failure [Q-5]
**File**: `authn/handler.go` — `SelectProvider`

After the template execute error is logged, write an HTTP 500 and `return`. Currently execution continues and the user receives a blank page with no status code indication of failure.

#### 1.5 Remove `domerr.Is()` Wrapper [Q-2]
**File**: `pkg/domerr/errors.go`

Delete the `Is()` wrapper function. Update all 3–5 call sites to use `errors.Is()` from the standard library directly. This is a mechanical global find-and-replace.

#### 1.6 Add Auth Request TTL Vacuum Job [O-1]
**File**: New: `oidc/postgres/authrequest_repo.go` (or a scheduler in `main.go`)

Add a `DeleteExpired(ctx context.Context, olderThan time.Duration) error` method to `AuthRequestRepository`. Schedule it in `main.go` with a `time.Ticker` (e.g., every 5 minutes):
```sql
DELETE FROM auth_requests WHERE created_at < now() - $1
```
Alternatively, push TTL enforcement into the SQL query itself:
```sql
SELECT ... FROM auth_requests WHERE id = $1 AND created_at > now() - $2
```

#### 1.7 Eliminate Token Lifetime Validation Duplication [Q-1]
**File**: `config/config.go`

Extract a `loadAndValidateIntParam(key string, defaultVal, minVal, maxVal int) (int, error)` helper. Replace the three duplicated blocks. This is a pure refactor with no behavior change.

#### 1.8 Fix Nullable Pointer Boilerplate in Repositories [Q-3]
**File**: `oidc/postgres/authrequest_repo.go`, `oidc/postgres/token_repo.go`

Add a package-level helper:
```go
func derefStr(p *string) string {
    if p == nil { return "" }
    return *p
}
func derefTime(p *time.Time) time.Time {
    if p == nil { return time.Time{} }
    return *p
}
```
Apply throughout all Scan calls. Pure cosmetic/maintainability improvement.

---

### Phase 2: Core Domain & Dependency Inversion Improvements

**Goal**: Correct the architectural violations that create tight coupling, obscure contracts, and impede testability. These changes require interface additions and moderate refactoring.

#### 2.1 Extract `ClaimsProvider` Interface in `authn` [A-4]
**Files**: `authn/provider.go`, `authn/provider_github.go`, `authn/provider_google.go`

Define:
```go
type ClaimsProvider interface {
    FetchClaims(ctx context.Context, token *oauth2.Token) (*identity.FederatedClaims, error)
}
```
Replace the `FetchClaims func(...)` field on `Provider` with `Claims ClaimsProvider`. Implement `GoogleClaimsProvider` and `GitHubClaimsProvider` structs. This enables proper interface mocking in tests and composable pre/post processing (logging, retry, metrics).

#### 2.2 Move `AuthRequestQuerier` Ownership to `oidc` Package [A-2]
**Files**: `authn/handler.go`, `oidc/ports.go`

The `authn.AuthRequestQuerier` interface is defined in the wrong package. Move the definition (and its used types) to `oidc/ports.go` (or a new `oidc/authn_ports.go`). Have `authn/handler.go` import it from `oidc`. This correctly establishes that `oidc` owns the auth request contract.

#### 2.3 Extract a Login Orchestration Use-Case from `authn.Handler` [A-5]
**Files**: `authn/handler.go` → new `authn/login_usecase.go`

Extract the core business logic from `FederatedCallback` into a separate type:
```go
type CompleteFederatedLoginUseCase struct {
    identity  identity.Service
    authReqs  oidc.AuthRequestService // now correctly typed from oidc package
}

func (uc *CompleteFederatedLoginUseCase) Execute(
    ctx context.Context,
    provider string,
    claims identity.FederatedClaims,
    authRequestID string,
) (callbackURL string, err error) { ... }
```
`Handler.FederatedCallback` becomes a thin HTTP adapter that calls this use case. The use case can now be unit-tested in isolation without any HTTP concerns.

#### 2.4 Decouple `StorageAdapter` from `identity.Service` [A-3]
**Files**: `oidc/adapter/storage.go`

Introduce a lightweight `UserClaimsSource` interface scoped to what the OIDC adapter actually needs:
```go
// In oidc/adapter/
type UserClaimsSource interface {
    GetUserClaims(ctx context.Context, userID string) (*UserClaims, error)
}

type UserClaims struct {
    Subject       string
    Email         string
    EmailVerified bool
    Name          string
    GivenName     string
    FamilyName    string
    Picture       string
}
```
Provide an `IdentityServiceAdapter` that wraps `identity.Service` and implements `UserClaimsSource`. This breaks the direct cross-package dependency. It also enables graceful degradation: if `GetUserClaims` returns `domerr.ErrNotFound`, return a subject-only userinfo response instead of an error.

#### 2.5 Introduce `Cipher` Interface for `pkg/crypto` [A-7]
**File**: `pkg/crypto/aes.go`

```go
type Cipher interface {
    Encrypt(plaintext []byte) (string, error)
    Decrypt(encoded string) ([]byte, error)
}

type AESCipher struct { key [32]byte }
func NewAESCipher(key [32]byte) Cipher { return &AESCipher{key: key} }
```
Update `authn.Handler` and any other consumer to depend on `crypto.Cipher` rather than calling `crypto.Encrypt`/`crypto.Decrypt` directly. This allows unit tests to inject a no-op cipher and decouples the handler from the concrete encryption implementation.

#### 2.6 Introduce `ConfigSource` Interface [A-6]
**File**: `config/config.go`

```go
type ConfigSource interface {
    Get(key string) string
}

type OSEnvSource struct{}
func (s OSEnvSource) Get(key string) string { return os.Getenv(key) }
```
Refactor `Load()` to `LoadFrom(src ConfigSource) (*Config, error)`. Provide `Load()` as a convenience wrapper: `return LoadFrom(OSEnvSource{})`. Tests replace `t.Setenv` calls with a `map[string]string` implementation of `ConfigSource`. This is a pure additive change that does not break any existing callers.

#### 2.7 Implement Structured Error Wrapping in `StorageAdapter` [S-6]
**File**: `oidc/adapter/storage.go`

For every error path in the adapter, explicitly map domain errors to OIDC protocol errors:
```go
func toOIDCError(err error, defaultDesc string) error {
    if domerr.Is(err, domerr.ErrNotFound) {
        return oidc.ErrInvalidRequest().WithDescription(defaultDesc)
    }
    if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
        return oidc.ErrServerError().WithDescription("request timeout")
    }
    // Log the raw error internally for debugging
    return oidc.ErrServerError().WithDescription("internal error")
}
```
No raw Go errors should escape the adapter boundary into Zitadel's protocol machinery.

#### 2.8 Fix Type Assertion Silent Failures in `storage.go` [Q-4]
**File**: `oidc/adapter/storage.go` — `clientIDFromRequest()`, `extractAuthTimeAMR()`

Return errors from these helpers when an unrecognized type is encountered. Callers should propagate them. This eliminates the current behavior of silently returning zero values (`""`, `time.Time{}`) when an unexpected token request type is passed.

---

### Phase 3: Long-Term Architectural Shifts

**Goal**: Address constraints that require schema changes, introduce new infrastructure, or change the fundamental capability envelope of the service.

#### 3.1 Migrate to 1:N User ↔ FederatedIdentity Relationship [A-1]

This is the single highest-impact domain model change.

**Schema migration**:
```sql
-- Remove the UNIQUE constraint on user_id
ALTER TABLE federated_identities DROP CONSTRAINT <unique_user_id_constraint>;
-- The composite unique (provider, provider_subject) must remain
```

**Domain changes** (`identity/domain.go`, `identity/ports.go`):
- `Repository.FindByFederatedIdentity(ctx, provider, subject)` returns `*User` — no change needed
- `Repository.CreateWithFederatedIdentity(ctx, user, fi)` — may need to support "link to existing user"

**Service changes** (`identity/service.go`):
- `FindOrCreateByFederatedLogin` — logic unchanged; the constraint removal is the enabler

**New capability needed**: A "link accounts" use case that associates an existing user's ID with a new `FederatedIdentity`. This is a new service method, not a refactor.

#### 3.2 Implement Signing Key Rotation [S-3]

Replace the single `*rsa.PrivateKey` in `config.Config` and `oidc/adapter/storage.go` with a key set:

**Model**:
```go
type SigningKeySet struct {
    Current   *SigningKeyEntry  // Used to sign new tokens
    Previous  []*SigningKeyEntry // Still exposed in JWKS for verification
}

type SigningKeyEntry struct {
    KeyID     string
    Key       *rsa.PrivateKey
    CreatedAt time.Time
}
```

**Adapter changes**: `KeySet()` in `op.Storage` returns all public keys (current + previous). `SignatureAlgorithms()` and `SigningKey()` return the current key.

**Operational requirement**: Provide a mechanism (CLI command, admin API, or file reload) to rotate the current key to previous and introduce a new current key. Tokens signed with the previous key remain verifiable for their remaining TTL.

#### 3.3 Implement User Email Synchronization Policy [O-2]

Make the email synchronization behavior explicit and configurable rather than implicit and frozen.

**Option A (Recommended)**: On `FindOrCreateByFederatedLogin`, if `FederatedClaims.Email != User.Email` AND `FederatedClaims.EmailVerified == true`, update `User.Email`. This requires adding a `UpdateUserEmail` method to `Repository` and calling it in the service.

**Option B**: Accept staleness; query `FederatedIdentity.ProviderEmail` as the authoritative email for display. Update the `identity.Service` contract to expose the federated email where needed.

Document the chosen policy explicitly in `identity/domain.go`.

#### 3.4 Add Observability Infrastructure [M-2, M-3]

Introduce observability as a cross-cutting concern, not bolted onto individual handlers.

**Structured logging with Correlation IDs**: Inject a request-scoped logger via context middleware in `main.go`. All service and adapter methods receive context; extract the correlation ID from context for all log messages. This is a `slog.Logger` augmentation, not a new dependency.

**Metrics**: Add a `MetricsRecorder` interface in a new `internal/observability` package. Implement with a `prometheus` adapter. Key metrics:
- `accounts_federated_login_attempts_total{provider, result}` — `authn` package
- `accounts_token_issued_total{grant_type}` — `oidc` adapter
- `accounts_token_revoked_total` — `oidc` token service
- `accounts_auth_request_ttl_expired_total` — `oidc` auth request service

**Distributed Tracing**: Add OpenTelemetry span creation at service method boundaries. Pass span context through the existing `context.Context` parameters — no signature changes required.

#### 3.5 Add Logout and Token Revocation Endpoint [M-1]

**`authn` package**:
- Add `Logout(w http.ResponseWriter, r *http.Request)` to `Handler`
- For providers that support RP-Initiated Logout (Google supports this via the `end_session_endpoint` in OIDC discovery), redirect to the provider's logout endpoint
- Revoke the application-side session (call `TokenService.DeleteByUserAndClient`)

**`oidc` package**:
- Zitadel's `op.Provider` supports token revocation endpoint (`/revoke`) natively if the `Storage` adapter implements `op.RevokeStorage`. Verify and implement this interface.

#### 3.6 Enforce Scope Constraints Per Client [S-5]

**Schema change**: Add an `allowed_scopes TEXT[]` column to the `clients` table.

**Domain change** (`oidc/domain.go`):
```go
type Client struct {
    // ...
    AllowedScopes []string
}
```

**Adapter change** (`oidc/adapter/client.go`):
```go
func (c *ClientAdapter) IsScopeAllowed(scope string) bool {
    for _, s := range c.domain.AllowedScopes {
        if s == scope { return true }
    }
    return false
}
```
Zitadel's framework calls `IsScopeAllowed` before processing scope requests, so this is a plug-in fix once the data is in place.

#### 3.7 Periodic Auth Request Vacuum (Scheduled Cleanup) [O-1]

Beyond the SQL-level TTL in Phase 1, implement a production-grade cleanup:

**Option A (Simple)**: Background goroutine in `main.go` with a `time.Ticker` calling `authReqRepo.DeleteExpired(ctx, 24*time.Hour)`. Use a graceful shutdown pattern tied to the server's `context.Context`.

**Option B (Robust)**: Delegate cleanup to a PostgreSQL `pg_cron` job inside the database. This survives application restarts and scales across multiple instances without coordination. Add the job to `migrations/`.

---

## Appendix: Cross-Package Dependency Map

```
config
  └─ (no internal imports)

main.go
  ├─ config
  ├─ identity/service + postgres
  ├─ oidc/authrequest_svc + client_svc + token_svc
  ├─ oidc/adapter/storage + provider
  └─ authn/handler + providers

authn
  ├─ identity (FederatedClaims, User, Service)
  └─ pkg/crypto (Encrypt/Decrypt)

identity
  ├─ pkg/domerr (ErrNotFound)
  └─ (postgres adapter: sqlx, ulid)

oidc (domain services)
  ├─ pkg/domerr
  └─ (postgres adapter: sqlx, ulid, pq)

oidc/adapter
  ├─ oidc (domain services)
  ├─ identity (Service — coupling to fix in Phase 2)
  └─ pkg/domerr
```

**Desired post-Phase-2 state**: `oidc/adapter` should depend only on `oidc` domain ports and a new `UserClaimsSource` interface, not directly on `identity.Service`.

---

## Appendix: Priority Matrix

| Issue | Phase | Security | Effort | Priority Score |
|---|---|---|---|---|
| Access token not revoked on rotation (S-1) | 1 | Critical | Low | 🔴 P0 |
| RSA key size not enforced (S-2) | 1 | High | Trivial | 🔴 P0 |
| Auth request accumulation / no vacuum (O-1) | 1 | Medium | Low | 🟠 P1 |
| Silent template failure (Q-5) | 1 | Low | Trivial | 🟠 P1 |
| Token lifetime DRY violation (Q-1) | 1 | None | Low | 🟡 P2 |
| ClaimsProvider interface (A-4) | 2 | Low | Medium | 🟠 P1 |
| AuthRequestQuerier ownership (A-2) | 2 | Low | Low | 🟡 P2 |
| Decouple StorageAdapter from identity (A-3) | 2 | Medium | Medium | 🟠 P1 |
| Structured OIDC error mapping (S-6) | 2 | Medium | Low | 🟠 P1 |
| ConfigSource abstraction (A-6) | 2 | None | Medium | 🟡 P2 |
| 1:N federated identities (A-1) | 3 | None | High | 🟠 P1 (growth blocker) |
| Signing key rotation (S-3) | 3 | High | High | 🟠 P1 |
| Observability infrastructure (M-2, M-3) | 3 | None | High | 🟡 P2 |
| Scope enforcement per client (S-5) | 3 | Medium | Medium | 🟡 P2 |
| Logout / revocation endpoint (M-1) | 3 | Medium | Medium | 🟡 P2 |
