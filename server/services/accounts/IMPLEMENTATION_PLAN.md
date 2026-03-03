# Implementation Plan: `accounts` Microservice Re-engineering

> **Status**: Pre-production. Breaking changes permitted. Zero backward compatibility required.
> **Based on**: `research.md` (2026-03-02 audit), full source review of all packages.
> **Scope**: OIDC Provider / IdP — `server/services/accounts/`

---

## 1. Architectural Blueprint (The "To-Be" State)

### 1.1 Target Dependency Graph

After the refactor, the dependency flow must be strictly unidirectional. No package may import a package at the same layer or above it (toward the framework boundary).

```
Layer 0 — Domain Entities (zero external deps)
├── identity/domain.go          (User, FederatedIdentity, FederatedClaims)
├── oidc/domain.go              (AuthRequest, Client, Token, RefreshToken)
└── pkg/domerr/errors.go        (sentinel errors)

Layer 1 — Ports (depend only on Layer 0)
├── identity/ports.go           (Repository, Service interfaces)
├── oidc/ports.go               (AuthRequestService, ClientService, TokenService, repos)
└── oidc/claims.go [NEW]        (UserClaimsSource interface — the bridge contract)

Layer 2 — Application Services (depend on Layer 0 + Layer 1 interfaces)
├── identity/service.go         (implements identity.Service)
├── oidc/authrequest_svc.go     (implements oidc.AuthRequestService)
├── oidc/client_svc.go          (implements oidc.ClientService)
├── oidc/token_svc.go           (implements oidc.TokenService)
└── authn/login_usecase.go [NEW](orchestrates federated login completion)

Layer 3 — Interface Adapters (depend on Layer 1 ports, never on Layer 2 concrete types)
├── identity/postgres/           → implements identity.Repository
├── oidc/postgres/               → implements oidc.*Repository
├── oidc/adapter/                → implements op.Storage via Layer 1 ports + UserClaimsSource
├── authn/handler.go             → thin HTTP adapter, delegates to login use-case
├── authn/providers/  [NEW dir]  → ClaimsProvider implementations (Google, GitHub)
└── pkg/crypto/                  → implements crypto.Cipher interface

Layer 4 — Composition Root (imports everything, owns wiring)
└── main.go
```

**Key change**: `oidc/adapter/storage.go` currently imports `identity.Service` directly. After the refactor it will import only `oidc.UserClaimsSource` (defined in `oidc/ports.go` or `oidc/claims.go`). The `main.go` composition root wires the `identity.Service` into an adapter that satisfies `UserClaimsSource`.

### 1.2 Resolved Circular / Cross-Layer Dependencies

| Current Violation | Resolution |
|---|---|
| `authn/handler.go` defines `AuthRequestQuerier` (an OIDC contract) | Move to `oidc/ports.go` as `LoginCompleter` interface |
| `oidc/adapter/storage.go` imports `identity.Service` | Depend on `UserClaimsSource` interface in `oidc/` package |
| `authn/handler.go` calls `crypto.Encrypt` directly | Depend on `crypto.Cipher` interface |
| `authn/handler.go` contains domain logic (find-or-create) | Extract to `authn/login_usecase.go` use-case type |

### 1.3 Interface Definitions

All new or relocated interfaces that form the architectural contracts:

```go
// ── oidc/ports.go (additions) ──────────────────────────────────────────

// UserClaimsSource provides user profile claims to the OIDC adapter.
// Implemented by an adapter wrapping identity.Service in main.go.
type UserClaimsSource interface {
    GetUserClaims(ctx context.Context, userID string) (*UserClaims, error)
}

// UserClaims is a projection of user profile data for OIDC token/userinfo responses.
type UserClaims struct {
    Subject       string
    Email         string
    EmailVerified bool
    Name          string
    GivenName     string
    FamilyName    string
    Picture       string
}

// LoginCompleter is consumed by the authn handler to finalize a login
// against the OIDC auth request lifecycle. Replaces the old AuthRequestQuerier.
type LoginCompleter interface {
    CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
}
```

```go
// ── pkg/crypto/cipher.go [NEW] ────────────────────────────────────────

// Cipher provides symmetric authenticated encryption.
type Cipher interface {
    Encrypt(plaintext []byte) (string, error)
    Decrypt(encoded string) ([]byte, error)
}
```

```go
// ── authn/claims_provider.go [NEW] ────────────────────────────────────

// ClaimsProvider fetches identity claims from an external OAuth2/OIDC provider.
type ClaimsProvider interface {
    FetchClaims(ctx context.Context, token *oauth2.Token) (*identity.FederatedClaims, error)
}
```

```go
// ── config/config.go (addition) ───────────────────────────────────────

// ConfigSource abstracts the origin of configuration key-value pairs.
type ConfigSource interface {
    Get(key string) string
}

// OSEnvSource reads from os.Getenv. Used in production.
type OSEnvSource struct{}
func (OSEnvSource) Get(key string) string { return os.Getenv(key) }

// MapSource reads from an in-memory map. Used in tests.
type MapSource map[string]string
func (m MapSource) Get(key string) string { return m[key] }
```

---

## 2. Domain Model Overhaul

### 2.1 Schema Change: 1:N User ↔ FederatedIdentity

The current `UNIQUE(user_id)` constraint on `federated_identities` enforces a 1:1 relationship. A user can only ever have one federated identity (e.g., Google OR GitHub, never both). This must become 1:N.

**Replacement migration** (`migrations/001_initial.sql` — full rewrite, since pre-production):

```sql
-- ============================================================
-- Table: users
-- ============================================================
CREATE TABLE users (
    id                TEXT        PRIMARY KEY,
    email             TEXT        NOT NULL DEFAULT '',
    email_verified    BOOLEAN     NOT NULL DEFAULT false,
    name              TEXT        NOT NULL DEFAULT '',
    given_name        TEXT        NOT NULL DEFAULT '',
    family_name       TEXT        NOT NULL DEFAULT '',
    picture           TEXT        NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- Table: federated_identities
-- Relationship: Many federated identities → one user (1:N)
-- ============================================================
CREATE TABLE federated_identities (
    id                      TEXT        PRIMARY KEY,
    user_id                 TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider                TEXT        NOT NULL,
    provider_subject        TEXT        NOT NULL,
    provider_email          TEXT        NOT NULL DEFAULT '',
    provider_email_verified BOOLEAN     NOT NULL DEFAULT false,
    provider_display_name   TEXT        NOT NULL DEFAULT '',
    provider_given_name     TEXT        NOT NULL DEFAULT '',
    provider_family_name    TEXT        NOT NULL DEFAULT '',
    provider_picture_url    TEXT        NOT NULL DEFAULT '',
    last_login_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One identity per (provider, subject) pair globally.
-- This prevents two users from claiming the same Google account.
CREATE UNIQUE INDEX federated_identities_provider_subject_idx
    ON federated_identities (provider, provider_subject);

-- Performance: find all identities for a user.
CREATE INDEX federated_identities_user_id_idx
    ON federated_identities (user_id);

-- NOTE: The old UNIQUE(user_id) constraint is intentionally removed.
-- A user may now link Google AND GitHub (and future providers).

-- ============================================================
-- Table: clients
-- ============================================================
CREATE TABLE clients (
    id                          TEXT        PRIMARY KEY,
    secret_hash                 TEXT        NOT NULL DEFAULT '',
    redirect_uris               TEXT[]      NOT NULL DEFAULT '{}',
    post_logout_redirect_uris   TEXT[]      NOT NULL DEFAULT '{}',
    application_type            TEXT        NOT NULL DEFAULT 'web',
    auth_method                 TEXT        NOT NULL DEFAULT 'client_secret_basic',
    response_types              TEXT[]      NOT NULL DEFAULT '{}',
    grant_types                 TEXT[]      NOT NULL DEFAULT '{}',
    access_token_type           TEXT        NOT NULL DEFAULT 'jwt',
    allowed_scopes              TEXT[]      NOT NULL DEFAULT '{}',
    id_token_lifetime_seconds   INTEGER     NOT NULL DEFAULT 3600,
    clock_skew_seconds          INTEGER     NOT NULL DEFAULT 0,
    id_token_userinfo_assertion BOOLEAN     NOT NULL DEFAULT false,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- ============================================================
-- Table: auth_requests
-- ============================================================
CREATE TABLE auth_requests (
    id                      TEXT        PRIMARY KEY,
    client_id               TEXT        NOT NULL,
    redirect_uri            TEXT        NOT NULL,
    state                   TEXT,
    nonce                   TEXT,
    scopes                  TEXT[]      NOT NULL DEFAULT '{}',
    response_type           TEXT        NOT NULL,
    response_mode           TEXT,
    code_challenge          TEXT,
    code_challenge_method   TEXT,
    prompt                  TEXT[],
    max_age                 INTEGER,
    login_hint              TEXT,
    user_id                 TEXT,
    auth_time               TIMESTAMPTZ,
    amr                     TEXT[],
    is_done                 BOOLEAN     NOT NULL DEFAULT false,
    code                    TEXT,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX auth_requests_code_idx
    ON auth_requests (code) WHERE code IS NOT NULL;

-- TTL index: enables efficient cleanup of expired auth requests.
CREATE INDEX auth_requests_created_at_idx
    ON auth_requests (created_at);

-- ============================================================
-- Table: tokens (access tokens)
-- ============================================================
CREATE TABLE tokens (
    id               TEXT        PRIMARY KEY,
    client_id        TEXT        NOT NULL,
    subject          TEXT        NOT NULL,
    audience         TEXT[]      NOT NULL DEFAULT '{}',
    scopes           TEXT[]      NOT NULL DEFAULT '{}',
    expiration       TIMESTAMPTZ NOT NULL,
    refresh_token_id TEXT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Performance: revoke all tokens for a user+client pair.
CREATE INDEX tokens_subject_client_idx
    ON tokens (subject, client_id);

-- ============================================================
-- Table: refresh_tokens
-- ============================================================
CREATE TABLE refresh_tokens (
    id               TEXT        PRIMARY KEY,
    token            TEXT        NOT NULL UNIQUE,
    client_id        TEXT        NOT NULL,
    user_id          TEXT        NOT NULL REFERENCES users(id),
    audience         TEXT[]      NOT NULL DEFAULT '{}',
    scopes           TEXT[]      NOT NULL DEFAULT '{}',
    amr              TEXT[]      NOT NULL DEFAULT '{}',
    auth_time        TIMESTAMPTZ NOT NULL,
    access_token_id  TEXT,
    expiration       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Changes from current schema:**

| Change | Rationale |
|---|---|
| Removed `UNIQUE(user_id)` on `federated_identities` | Enables 1:N user ↔ provider linking |
| Added `federated_identities_user_id_idx` index | Query performance for "list user's linked accounts" |
| Added `allowed_scopes TEXT[]` to `clients` | Enables per-client scope restriction (S-5) |
| Added `updated_at` to `users` | Track profile update timestamps |
| Added `auth_requests_created_at_idx` | Efficient expired row cleanup (O-1) |
| Added `tokens_subject_client_idx` | Efficient user+client token revocation |
| Added `NOT NULL DEFAULT` to all nullable text columns | Eliminates nullable pointer scanning boilerplate (Q-3) |

### 2.2 Updated Go Structs

```go
// ── identity/domain.go ────────────────────────────────────────────────

// User is the core identity entity. A user may have multiple
// federated identities linked to their account.
type User struct {
    ID            string
    Email         string
    EmailVerified bool
    Name          string
    GivenName     string
    FamilyName    string
    Picture       string
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

// FederatedIdentity represents a link between a User and an external
// identity provider (Google, GitHub, etc.). Multiple FederatedIdentity
// records may reference the same User.
type FederatedIdentity struct {
    ID                    string
    UserID                string
    Provider              string
    ProviderSubject       string
    ProviderEmail         string
    ProviderEmailVerified bool
    ProviderDisplayName   string
    ProviderGivenName     string
    ProviderFamilyName    string
    ProviderPictureURL    string
    LastLoginAt           time.Time
    CreatedAt             time.Time
    UpdatedAt             time.Time
}

// FederatedClaims holds the claims extracted from an external
// provider's token/API response. This is a value object, not persisted directly.
type FederatedClaims struct {
    Subject       string
    Email         string
    EmailVerified bool
    Name          string
    GivenName     string
    FamilyName    string
    Picture       string
}
```

```go
// ── identity/ports.go (updated) ───────────────────────────────────────

type Repository interface {
    GetByID(ctx context.Context, id string) (*User, error)
    FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*User, error)
    CreateWithFederatedIdentity(ctx context.Context, user *User, fi *FederatedIdentity) error
    LinkFederatedIdentity(ctx context.Context, fi *FederatedIdentity) error  // NEW: link to existing user
    UpdateFederatedIdentityClaims(ctx context.Context, provider, providerSubject string, claims FederatedClaims, lastLoginAt time.Time) error
    UpdateUserEmail(ctx context.Context, userID, email string, verified bool) error  // NEW: email sync
    ListFederatedIdentities(ctx context.Context, userID string) ([]*FederatedIdentity, error) // NEW
}
```

```go
// ── oidc/domain.go (addition to Client) ───────────────────────────────

type Client struct {
    ID                       string
    SecretHash               string
    RedirectURIs             []string
    PostLogoutRedirectURIs   []string
    ApplicationType          string
    AuthMethod               string
    ResponseTypes            []string
    GrantTypes               []string
    AccessTokenType          string
    AllowedScopes            []string   // NEW: per-client scope whitelist
    IDTokenLifetimeSeconds   int
    ClockSkewSeconds         int
    IDTokenUserinfoAssertion bool
    CreatedAt                time.Time
    UpdatedAt                time.Time
}
```

---

## 3. Security Hardening Checklist

### 3.1 CRITICAL: Access Token Revocation on Refresh Rotation (S-1)

The `CreateAccessAndRefresh` method in `oidc/postgres/token_repo.go` deletes the old refresh token but leaves the associated access token alive. An intercepted refresh token allows the attacker to rotate while the old access token remains valid for up to 60 minutes.

**Fix — within the existing transaction in `CreateAccessAndRefresh`:**

```go
func (r *TokenRepository) CreateAccessAndRefresh(
    ctx context.Context,
    access *oidc.Token,
    refresh *oidc.RefreshToken,
    currentRefreshToken string,
) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin tx: %w", err)
    }
    defer tx.Rollback()

    if currentRefreshToken != "" {
        // Step 1: Look up the old refresh token to find its access_token_id.
        var oldAccessTokenID string
        err := tx.QueryRowContext(ctx,
            `SELECT access_token_id FROM refresh_tokens WHERE token = $1`,
            currentRefreshToken,
        ).Scan(&oldAccessTokenID)
        if err != nil && !errors.Is(err, sql.ErrNoRows) {
            return fmt.Errorf("lookup old refresh token: %w", err)
        }

        // Step 2: Delete the old access token (if it exists).
        if oldAccessTokenID != "" {
            if _, err := tx.ExecContext(ctx,
                `DELETE FROM tokens WHERE id = $1`, oldAccessTokenID,
            ); err != nil {
                return fmt.Errorf("delete old access token: %w", err)
            }
        }

        // Step 3: Delete the old refresh token.
        if _, err := tx.ExecContext(ctx,
            `DELETE FROM refresh_tokens WHERE token = $1`, currentRefreshToken,
        ); err != nil {
            return fmt.Errorf("delete old refresh token: %w", err)
        }
    }

    // Step 4: Insert new access token.
    // Step 5: Insert new refresh token (with access_token_id referencing the new access token).
    // ... (existing insert logic)

    return tx.Commit()
}
```

- [ ] Add `access_token_id` lookup query before old refresh token deletion
- [ ] Add `DELETE FROM tokens WHERE id = $1` for old access token in same transaction
- [ ] Write unit test: rotate refresh token, assert old access token no longer retrievable
- [ ] Write unit test: rotate refresh token, assert old refresh token no longer retrievable
- [ ] Write unit test: first token issuance (no `currentRefreshToken`) still works

### 3.2 HIGH: RSA Key Size Enforcement (S-2)

**Fix — in `config/config.go`, `parseRSAPrivateKey`:**

- [ ] After `x509.ParsePKCS1PrivateKey` / `x509.ParsePKCS8PrivateKey`, validate:
  ```go
  if rsaKey.N.BitLen() < 2048 {
      return nil, fmt.Errorf("RSA signing key must be >= 2048 bits, got %d", rsaKey.N.BitLen())
  }
  ```
- [ ] Add unit test with 1024-bit key → expect error
- [ ] Add unit test with 2048-bit key → expect success
- [ ] Add unit test with 4096-bit key → expect success

### 3.3 HIGH: Signing Key Rotation Strategy (S-3)

Replace the single `*rsa.PrivateKey` in `config.Config` with a key set model:

```go
// ── config/config.go ──────────────────────────────────────────────────

type Config struct {
    // ... existing fields ...
    SigningKeys SigningKeySet  // Replaces: SigningKey *rsa.PrivateKey
}

type SigningKeySet struct {
    Current  *rsa.PrivateKey   // Used to sign new tokens
    Previous []*rsa.PrivateKey // Exposed in JWKS for verification only
}
```

**Environment variable design:**

| Variable | Required | Description |
|---|---|---|
| `SIGNING_KEY_PEM` | Yes | Current active signing key (PEM-encoded) |
| `SIGNING_KEY_PREVIOUS_PEM` | No | Comma-separated list of previous keys still valid for verification |

**Adapter changes (`oidc/adapter/keys.go`):**

- [ ] Rename `SigningKeyWithID` / `PublicKeyWithID` to support a set
- [ ] `KeySet()` returns all public keys (current + previous)
- [ ] `SigningKey()` returns only the current key
- [ ] `SignatureAlgorithms()` returns `[]jose.SignatureAlgorithm{jose.RS256}` (unchanged)

**Operational workflow:**
1. Generate new RSA key → set as `SIGNING_KEY_PEM`
2. Move old key to `SIGNING_KEY_PREVIOUS_PEM`
3. Restart service → new tokens signed with new key, old tokens still verifiable
4. After old token TTL expires, remove old key from `SIGNING_KEY_PREVIOUS_PEM`

- [ ] Update `config.Load` / `config.LoadFrom` to parse `SIGNING_KEY_PREVIOUS_PEM`
- [ ] Validate all keys are >= 2048 bits
- [ ] Update `StorageAdapter` constructor to accept `SigningKeySet`
- [ ] Implement `KeySet()` method returning all public keys
- [ ] Write integration test: sign with key A, rotate to key B, verify token signed with A using JWKS

---

## 4. Step-by-Step Implementation Phases

### Phase 1: Foundation & Security Core

**Goal**: Eliminate all Critical/High security issues. Lay the schema foundation. Establish interface contracts.

#### Step 1.1 — Rewrite database schema

Replace `migrations/001_initial.sql` with the schema from Section 2.1 above.

**Files modified:**
- `migrations/001_initial.sql` — full rewrite
- `migrations/002_seed_clients.sql` — add `allowed_scopes` column value

**Acceptance criteria:**
- `UNIQUE(user_id)` constraint removed from `federated_identities`
- `allowed_scopes` column present on `clients`
- `NOT NULL DEFAULT` on all text columns (eliminates null pointer scanning)
- New indexes for cleanup and revocation queries

#### Step 1.2 — Fix access token revocation during refresh rotation (S-1)

**Files modified:**
- `internal/oidc/postgres/token_repo.go` — `CreateAccessAndRefresh` method

**Implementation:** See Section 3.1 above.

**Acceptance criteria:**
- Old access token deleted in same transaction as old refresh token
- Unit test confirms old access token is not retrievable after rotation
- No regression on first-time token issuance (empty `currentRefreshToken`)

#### Step 1.3 — Enforce RSA key size at startup (S-2)

**Files modified:**
- `config/config.go` — `parseRSAPrivateKey` function

**Implementation:** See Section 3.2 above.

**Acceptance criteria:**
- Keys < 2048 bits rejected at startup with clear error message
- Unit tests cover 1024-bit (reject), 2048-bit (accept), 4096-bit (accept)

#### Step 1.4 — Validate ISSUER URL format at startup (O-3)

**Files modified:**
- `config/config.go` — `Load()` / `LoadFrom()`

**Implementation:**
```go
u, err := url.Parse(cfg.Issuer)
if err != nil || u.Scheme == "" || u.Host == "" {
    return nil, fmt.Errorf("ISSUER must be a valid URL, got %q", cfg.Issuer)
}
```

**Acceptance criteria:**
- Invalid URLs rejected at startup
- Empty string rejected
- `http://` and `https://` both accepted (to allow local dev)

#### Step 1.5 — Implement signing key rotation support (S-3)

**Files modified:**
- `config/config.go` — add `SigningKeySet`, parse `SIGNING_KEY_PREVIOUS_PEM`
- `internal/oidc/adapter/keys.go` — support multiple public keys in JWKS
- `internal/oidc/adapter/storage.go` — accept `SigningKeySet`

**Implementation:** See Section 3.3 above.

**Acceptance criteria:**
- JWKS endpoint exposes current + previous keys
- New tokens signed with current key only
- Tokens signed with previous key still validate against JWKS
- All keys validated >= 2048 bits

#### Step 1.6 — Fix internal error leakage into OIDC responses (S-6)

**Files modified:**
- `internal/oidc/adapter/storage.go` — all error return paths

**Implementation:** Add a `toOIDCError` helper:
```go
func toOIDCError(err error, notFoundDesc string) error {
    switch {
    case domerr.Is(err, domerr.ErrNotFound):
        return oidc.ErrInvalidRequest().WithDescription(notFoundDesc)
    case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
        return oidc.ErrServerError().WithDescription("request timeout")
    default:
        slog.Error("internal error in OIDC adapter", "error", err)
        return oidc.ErrServerError().WithDescription("internal error")
    }
}
```

Apply to every raw `return nil, err` path in `storage.go`.

**Acceptance criteria:**
- No raw Go errors escape the adapter boundary
- All non-domain errors logged with `slog.Error` before wrapping
- `domerr.ErrNotFound` → `invalid_request`
- Context errors → `server_error` with "request timeout"
- Unknown errors → `server_error` with "internal error"

#### Step 1.7 — Fix silent template rendering failure (Q-5)

**Files modified:**
- `internal/authn/handler.go` — `SelectProvider` method

**Implementation:** After the template execute error is logged, write HTTP 500 and return:
```go
if err := h.tmpl.Execute(w, data); err != nil {
    h.logger.Error("template render failed", "error", err)
    http.Error(w, "Internal Server Error", http.StatusInternalServerError)
    return
}
```

**Acceptance criteria:**
- Template failure returns HTTP 500 to the client
- Error is logged

#### Step 1.8 — Add expired auth request cleanup (O-1)

**Files modified:**
- `internal/oidc/ports.go` — add `DeleteExpiredBefore(ctx, time.Time) (int64, error)` to `AuthRequestRepository`
- `internal/oidc/postgres/authrequest_repo.go` — implement the method
- `main.go` — schedule periodic cleanup with `time.Ticker`

**SQL:**
```sql
DELETE FROM auth_requests WHERE created_at < $1
```

**Goroutine in main.go:**
```go
go func() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            cutoff := time.Now().UTC().Add(-cfg.AuthRequestTTL)
            deleted, err := authReqRepo.DeleteExpiredBefore(ctx, cutoff)
            if err != nil {
                logger.Error("auth request cleanup failed", "error", err)
            } else if deleted > 0 {
                logger.Info("cleaned up expired auth requests", "count", deleted)
            }
        }
    }
}()
```

**Acceptance criteria:**
- Expired auth requests are deleted periodically
- Cleanup respects graceful shutdown context
- Deletion count logged

---

### Phase 2: Architectural Realignment

**Goal**: Fix all dependency violations. Establish clean interface boundaries. Extract domain logic from handlers.

#### Step 2.1 — Introduce `ConfigSource` interface (A-6)

**Files modified:**
- `config/config.go` — add `ConfigSource` interface, `OSEnvSource`, `MapSource`; refactor `Load()` to `LoadFrom(ConfigSource)`
- `config/config_test.go` — replace `t.Setenv` calls with `MapSource`

**Implementation:** See Section 1.3 interfaces above.

**Acceptance criteria:**
- `Load()` is a thin wrapper around `LoadFrom(OSEnvSource{})`
- All config tests use `MapSource` instead of `t.Setenv`
- No behavior change for production callers

#### Step 2.2 — Introduce `Cipher` interface (A-7)

**Files created:**
- `internal/pkg/crypto/cipher.go` — interface definition

**Files modified:**
- `internal/pkg/crypto/aes.go` — `AESCipher` struct implements `Cipher`
- `internal/authn/handler.go` — depend on `crypto.Cipher` instead of `[32]byte` key + function calls

**Implementation:**
```go
// pkg/crypto/cipher.go
type Cipher interface {
    Encrypt(plaintext []byte) (string, error)
    Decrypt(encoded string) ([]byte, error)
}

type AESCipher struct{ key [32]byte }

func NewAESCipher(key [32]byte) *AESCipher { return &AESCipher{key: key} }
func (c *AESCipher) Encrypt(plaintext []byte) (string, error) { return Encrypt(c.key, plaintext) }
func (c *AESCipher) Decrypt(encoded string) ([]byte, error)   { return Decrypt(c.key, encoded) }
```

**Acceptance criteria:**
- `Handler` accepts `crypto.Cipher` in constructor, not a raw `[32]byte`
- Existing `Encrypt`/`Decrypt` package functions remain for backward compat during phase transition
- Handler tests can inject a no-op or deterministic cipher

#### Step 2.3 — Extract `ClaimsProvider` interface (A-4)

**Files created:**
- `internal/authn/claims_provider.go` — interface definition

**Files modified:**
- `internal/authn/provider.go` — replace `FetchClaims func(...)` field with `Claims ClaimsProvider`
- `internal/authn/provider_google.go` — define `googleClaimsProvider` struct implementing `ClaimsProvider`
- `internal/authn/provider_github.go` — define `githubClaimsProvider` struct implementing `ClaimsProvider`
- `internal/authn/handler.go` — call `provider.Claims.FetchClaims(...)` instead of `provider.FetchClaims(...)`

**Updated `Provider` struct:**
```go
type Provider struct {
    Name         string
    DisplayName  string
    OAuth2Config *oauth2.Config
    Claims       ClaimsProvider
}
```

**Acceptance criteria:**
- `ClaimsProvider` is a proper interface
- Google and GitHub each have a named struct implementing it
- Handler tests can inject a mock `ClaimsProvider`
- HTTP client in GitHub provider is pooled (reuse `http.Client` — fixes O-5)

#### Step 2.4 — Move `AuthRequestQuerier` to `oidc` package (A-2)

**Files modified:**
- `internal/oidc/ports.go` — add `LoginCompleter` interface (see Section 1.3)
- `internal/authn/handler.go` — remove `AuthRequestQuerier` type; import `oidc.LoginCompleter`
- `main.go` — adjust wiring (the `authReqBridge` already satisfies this contract)

The `AuthRequestInfo` struct (currently empty except for `ID`) and the `GetByID` query in the bridge are dead code — the handler never uses the returned `AuthRequestInfo`. Remove `GetByID` from the contract; the handler only needs `CompleteLogin`.

**Acceptance criteria:**
- `authn` package no longer defines any OIDC-domain interfaces
- `authn.Handler` depends on `oidc.LoginCompleter`
- `AuthRequestInfo` type deleted from `authn`

#### Step 2.5 — Extract login orchestration use-case (A-5)

**Files created:**
- `internal/authn/login_usecase.go`

**Files modified:**
- `internal/authn/handler.go` — `FederatedCallback` becomes a thin HTTP adapter

**New use-case type:**
```go
type CompleteFederatedLogin struct {
    identity    identity.Service
    loginComp   oidc.LoginCompleter
}

func NewCompleteFederatedLogin(
    identitySvc identity.Service,
    loginComp oidc.LoginCompleter,
) *CompleteFederatedLogin {
    return &CompleteFederatedLogin{identity: identitySvc, loginComp: loginComp}
}

// Execute processes a completed federated authentication.
// It finds or creates the user identity, syncs email if changed,
// and completes the OIDC login flow.
func (uc *CompleteFederatedLogin) Execute(
    ctx context.Context,
    provider string,
    claims identity.FederatedClaims,
    authRequestID string,
) (userID string, err error) {
    user, err := uc.identity.FindOrCreateByFederatedLogin(ctx, provider, claims)
    if err != nil {
        return "", fmt.Errorf("find or create user: %w", err)
    }

    authTime := time.Now().UTC()
    amr := []string{"federated"}

    if err := uc.loginComp.CompleteLogin(ctx, authRequestID, user.ID, authTime, amr); err != nil {
        return "", fmt.Errorf("complete login: %w", err)
    }

    return user.ID, nil
}
```

The `Handler.FederatedCallback` HTTP method is reduced to:
1. Read and validate HTTP parameters
2. Decrypt state
3. Exchange OAuth2 code for token
4. Call `provider.Claims.FetchClaims()`
5. Call `uc.Execute()`
6. Redirect to callback URL

**Acceptance criteria:**
- `CompleteFederatedLogin` is unit-testable with mocked `identity.Service` and `oidc.LoginCompleter`
- `FederatedCallback` HTTP handler contains zero domain logic
- All existing integration tests pass

#### Step 2.6 — Decouple `StorageAdapter` from `identity.Service` (A-3)

**Files modified:**
- `internal/oidc/ports.go` — add `UserClaimsSource` interface (see Section 1.3)
- `internal/oidc/adapter/storage.go` — replace `identity.Service` field with `UserClaimsSource`
- `main.go` — create an adapter that wraps `identity.Service` as `UserClaimsSource`

**Bridge adapter in main.go (or a dedicated file):**
```go
type identityClaimsAdapter struct {
    svc identity.Service
}

func (a *identityClaimsAdapter) GetUserClaims(ctx context.Context, userID string) (*oidcdom.UserClaims, error) {
    user, err := a.svc.GetUser(ctx, userID)
    if err != nil {
        return nil, err
    }
    return &oidcdom.UserClaims{
        Subject:       user.ID,
        Email:         user.Email,
        EmailVerified: user.EmailVerified,
        Name:          user.Name,
        GivenName:     user.GivenName,
        FamilyName:    user.FamilyName,
        Picture:       user.Picture,
    }, nil
}
```

**Acceptance criteria:**
- `oidc/adapter` package has zero imports from `identity`
- `StorageAdapter` depends only on `oidc.UserClaimsSource`
- `SetUserinfoFromScopes`, `SetUserinfoFromToken`, and introspection methods work unchanged
- `main.go` wires the bridge

#### Step 2.7 — Implement user email synchronization (O-2)

**Files modified:**
- `internal/identity/ports.go` — `UpdateUserEmail` already added in Section 2.2
- `internal/identity/postgres/user_repo.go` — implement `UpdateUserEmail`
- `internal/identity/service.go` — in `FindOrCreateByFederatedLogin`, sync email when provider says it changed

**Logic in `FindOrCreateByFederatedLogin`:**
```go
// After finding existing user:
if claims.EmailVerified && claims.Email != "" && claims.Email != user.Email {
    if err := s.repo.UpdateUserEmail(ctx, user.ID, claims.Email, claims.EmailVerified); err != nil {
        return nil, fmt.Errorf("sync user email: %w", err)
    }
    user.Email = claims.Email
    user.EmailVerified = claims.EmailVerified
}
```

**Acceptance criteria:**
- User email updated when provider provides a verified email that differs from stored email
- Unverified provider emails do not overwrite verified user emails
- Unit test covers email sync path

#### Step 2.8 — Eliminate config validation duplication (Q-1)

**Files modified:**
- `config/config.go`

**Implementation:** Extract a `loadIntParam(src ConfigSource, key string, defaultVal, min, max int) (int, error)` helper. Replace the three duplicated blocks for `ACCESS_TOKEN_LIFETIME_MINUTES`, `REFRESH_TOKEN_LIFETIME_DAYS`, and `AUTH_REQUEST_TTL_MINUTES`.

**Acceptance criteria:**
- Single validation function for bounded integer config values
- No behavior change
- All existing config tests pass

#### Step 2.9 — Remove `domerr.Is()` wrapper (Q-2)

**Files modified:**
- `internal/pkg/domerr/errors.go` — delete `Is()` function
- All call sites — replace `domerr.Is(err, ...)` with `errors.Is(err, domerr.Err...)`

This is a mechanical find-and-replace.

**Acceptance criteria:**
- `domerr.Is` no longer exists
- All callers use `errors.Is` from standard library
- All tests pass

#### Step 2.10 — Fix type assertion silent failures (Q-4)

**Files modified:**
- `internal/oidc/adapter/storage.go` — `clientIDFromRequest()`, `extractAuthTimeAMR()`

**Implementation:** Return explicit errors when an unrecognized type is encountered:
```go
func clientIDFromRequest(req op.TokenRequest) (string, error) {
    switch r := req.(type) {
    case *AuthRequest:
        return r.domain.ClientID, nil
    case *RefreshTokenRequest:
        return r.domain.ClientID, nil
    case *clientCredentialsTokenRequest:
        return r.clientID, nil
    default:
        return "", fmt.Errorf("unsupported token request type: %T", req)
    }
}
```

**Acceptance criteria:**
- Unknown types return errors, not zero values
- Callers propagate errors (wrapped as OIDC server errors via `toOIDCError`)

---

### Phase 3: Feature Completion

**Goal**: Scope enforcement, observability, logout, and operational hardening.

#### Step 3.1 — Enforce per-client scope restrictions (S-5)

**Files modified:**
- `internal/oidc/domain.go` — `AllowedScopes` already added in Section 2.2
- `internal/oidc/postgres/client_repo.go` — include `allowed_scopes` in SELECT/scan
- `internal/oidc/adapter/client.go` — implement `IsScopeAllowed` correctly

**Implementation:**
```go
func (c *ClientAdapter) IsScopeAllowed(scope string) bool {
    // If no scopes are configured, allow all (open policy).
    if len(c.domain.AllowedScopes) == 0 {
        return true
    }
    for _, s := range c.domain.AllowedScopes {
        if s == scope {
            return true
        }
    }
    return false
}
```

**Acceptance criteria:**
- Client with empty `allowed_scopes` permits all scopes (backward compatible)
- Client with `["openid", "email"]` rejects `profile` scope
- Zitadel framework calls `IsScopeAllowed` and respects the return value (verify with integration test)

#### Step 3.2 — Add structured logging with correlation IDs (M-2)

**Files created:**
- `internal/pkg/middleware/requestid.go` — HTTP middleware that injects a request ID into `context` and `slog`

**Files modified:**
- `main.go` — add middleware to router

**Implementation:**
```go
func RequestID(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get("X-Request-ID")
        if id == "" {
            id = ulid.Make().String()
        }
        ctx := context.WithValue(r.Context(), requestIDKey, id)
        w.Header().Set("X-Request-ID", id)
        logger := slog.Default().With("request_id", id)
        ctx = withLogger(ctx, logger)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

**Acceptance criteria:**
- Every request has a unique correlation ID
- ID propagated in context and response header
- All `slog` calls include the correlation ID

#### Step 3.3 — Add Prometheus metrics instrumentation (M-3)

**Files created:**
- `internal/pkg/metrics/metrics.go` — metric definitions and HTTP handler

**Files modified:**
- `main.go` — register metrics handler at `/metrics`
- `internal/authn/handler.go` — increment login counters
- `internal/oidc/adapter/storage.go` — increment token issuance counters

**Key metrics:**
```go
var (
    FederatedLoginAttempts = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "accounts_federated_login_attempts_total"},
        []string{"provider", "result"},
    )
    TokensIssued = prometheus.NewCounterVec(
        prometheus.CounterOpts{Name: "accounts_tokens_issued_total"},
        []string{"grant_type"},
    )
    AuthRequestsExpired = prometheus.NewCounter(
        prometheus.CounterOpts{Name: "accounts_auth_requests_expired_total"},
    )
)
```

**Acceptance criteria:**
- `/metrics` endpoint exposes Prometheus-format metrics
- Login success/failure rates tracked by provider
- Token issuance rates tracked by grant type

#### Step 3.4 — Implement logout / session revocation (M-1)

**Files modified:**
- `internal/authn/handler.go` — add `Logout` method
- `main.go` — register logout route

**Implementation:** Zitadel's `op.Provider` supports the revocation endpoint (`/revoke`) natively if the `Storage` adapter implements `op.RevokeStorage`. The `StorageAdapter` already has `Revoke` and `RevokeRefreshToken` methods in the token service. Verify alignment with `op.RevokeStorage` interface and register.

For RP-Initiated Logout, add a handler that:
1. Validates the `id_token_hint` parameter
2. Calls `TokenService.DeleteByUserAndClient`
3. Redirects to Post-Logout Redirect URI (if in client's whitelist)

**Acceptance criteria:**
- `/revoke` endpoint functional for access and refresh tokens
- RP-Initiated Logout redirects correctly
- All user tokens for the client are deleted on logout

#### Step 3.5 — GitHub HTTP client pooling (O-5)

**Files modified:**
- `internal/authn/provider_github.go`

**Implementation:** Create the `http.Client` once in the `githubClaimsProvider` struct constructor (Step 2.3), not per `FetchClaims` call.

**Acceptance criteria:**
- Single `http.Client` instance reused across all GitHub claim fetches
- 10-second timeout preserved

#### Step 3.6 — Consistent error wrapping format (Q-8)

**Convention to adopt across all packages:**
```go
// Pattern: "verb noun: %w"
return fmt.Errorf("create user: %w", err)
return fmt.Errorf("fetch claims from %s: %w", provider, err)
return fmt.Errorf("parse RSA key: %w", err)
```

Audit all `fmt.Errorf` calls for consistency. No mixed formats like `"failed to X"` vs `"could not X"` vs `"X failed"`.

**Acceptance criteria:**
- All error wrapping follows `"verb noun: %w"` pattern
- No information-free wrappers like `"error: %w"`

---

## 5. Coding Standards & Guidelines

### 5.1 Error Handling

**Domain → Adapter boundary rule**: No raw Go error (database, network, context) may cross from the domain/infrastructure layer into the OIDC protocol layer. Every error path in `oidc/adapter/storage.go` must map to a Zitadel OIDC error type.

```go
// WRONG — leaks internal error
return nil, err

// CORRECT — maps to protocol error
return nil, toOIDCError(err, "token not found")
```

**Service → Handler boundary rule**: Services return domain errors (`domerr.ErrNotFound`, etc.) or wrapped errors. HTTP handlers are responsible for mapping these to HTTP status codes. Use `errors.Is()` for matching.

**Error wrapping format**: `fmt.Errorf("verb noun: %w", err)`. Lowercase. No period. Always use `%w` to preserve error chain.

### 5.2 Configuration Management

- All configuration loaded through `ConfigSource` interface
- `Load()` is the production convenience wrapper; tests use `MapSource`
- Validation happens at load time — fail fast on startup, not at first request
- Secrets (RSA keys, crypto keys, client secrets) never logged at any level
- All duration/lifetime config values have explicit min/max bounds enforced at load

### 5.3 Testing Strategy

**Unit tests (primary focus — per README policy):**
- Every domain service method has unit tests
- Every repository method tested against a real PostgreSQL instance via `testhelper/testdb.go`
- Every adapter method tested with mocked dependencies (via the new interfaces)
- Use-case types (`CompleteFederatedLogin`) tested with mocked services
- HTTP handlers tested with `httptest.NewRecorder` and mocked use-cases
- Config loading tested with `MapSource`, never `t.Setenv`
- Cipher-dependent code tested with a deterministic `Cipher` implementation

**What NOT to test locally:**
- Full OIDC authorization code flow (E2E — staging only, per README)
- Cross-service integration (IdP ↔ downstream clients)
- Performance/load testing

**Test file naming**: `*_test.go` in the same package for whitebox tests. Separate `_test` package for blackbox interface compliance tests.

### 5.4 Package Documentation

Each package should have a short `doc.go` file specifying:
- What the package does (one sentence)
- What layer it belongs to (domain, service, adapter, infrastructure)
- What it is allowed to import

Example:
```go
// Package identity defines the core user identity domain model and
// service interfaces. It belongs to the domain layer and must not
// import any infrastructure, adapter, or framework packages.
package identity
```

### 5.5 SQL Conventions

- All queries use parameterized placeholders (`$1`, `$2`, etc.) — never string interpolation
- All multi-step mutations wrapped in explicit transactions
- All `SELECT` queries specify exact column list — never `SELECT *`
- All `TEXT` columns use `NOT NULL DEFAULT ''` — eliminates nullable pointer scanning
- Indexes named as `{table}_{columns}_idx`
- Constraints named as `{table}_{columns}_{type}` (e.g., `federated_identities_provider_subject_unique`)

---

## Appendix: File Change Summary

| File | Phase | Action |
|---|---|---|
| `migrations/001_initial.sql` | 1 | Rewrite |
| `migrations/002_seed_clients.sql` | 1 | Update (add `allowed_scopes`) |
| `config/config.go` | 1+2 | Modify (RSA validation, URL validation, ConfigSource, SigningKeySet, DRY) |
| `config/config_test.go` | 2 | Modify (use MapSource) |
| `internal/oidc/postgres/token_repo.go` | 1 | Modify (access token revocation) |
| `internal/oidc/adapter/storage.go` | 1+2 | Modify (error wrapping, type assertions, decouple identity) |
| `internal/oidc/adapter/keys.go` | 1 | Modify (key rotation support) |
| `internal/oidc/adapter/client.go` | 3 | Modify (scope enforcement) |
| `internal/oidc/ports.go` | 1+2 | Modify (add LoginCompleter, UserClaimsSource, DeleteExpiredBefore) |
| `internal/oidc/domain.go` | 2 | Modify (AllowedScopes on Client) |
| `internal/oidc/postgres/authrequest_repo.go` | 1 | Modify (add DeleteExpiredBefore) |
| `internal/oidc/postgres/client_repo.go` | 3 | Modify (scan allowed_scopes) |
| `internal/authn/handler.go` | 1+2 | Modify (template fix, extract use-case, depend on interfaces) |
| `internal/authn/provider.go` | 2 | Modify (ClaimsProvider interface on Provider struct) |
| `internal/authn/provider_github.go` | 2+3 | Modify (implement ClaimsProvider, pool HTTP client) |
| `internal/authn/provider_google.go` | 2 | Modify (implement ClaimsProvider) |
| `internal/authn/claims_provider.go` | 2 | **Create** (ClaimsProvider interface) |
| `internal/authn/login_usecase.go` | 2 | **Create** (CompleteFederatedLogin) |
| `internal/identity/domain.go` | 1 | Modify (add UpdatedAt) |
| `internal/identity/ports.go` | 1 | Modify (add LinkFederatedIdentity, UpdateUserEmail, ListFederatedIdentities) |
| `internal/identity/service.go` | 2 | Modify (email sync logic) |
| `internal/identity/postgres/user_repo.go` | 2 | Modify (implement new repo methods) |
| `internal/pkg/crypto/aes.go` | 2 | Modify (AESCipher struct) |
| `internal/pkg/crypto/cipher.go` | 2 | **Create** (Cipher interface) |
| `internal/pkg/domerr/errors.go` | 2 | Modify (remove Is wrapper) |
| `internal/pkg/middleware/requestid.go` | 3 | **Create** (request ID middleware) |
| `internal/pkg/metrics/metrics.go` | 3 | **Create** (Prometheus metrics) |
| `main.go` | 1+2+3 | Modify (cleanup goroutine, wiring changes, middleware, metrics) |
