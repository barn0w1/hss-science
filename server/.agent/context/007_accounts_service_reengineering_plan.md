# Accounts Service Refactoring Plan -- Iteration 1

**Scope:** Extract `internal/pkg`, `internal/identity`, and `internal/authn` from the current flat structure.
**Out of Scope:** `internal/oidc` domain, dismantling `oidcprovider.Storage`, token management. See [Future Roadmap](#9-future-roadmap).

---

## Table of Contents

1. [Target Directory Tree](#1-target-directory-tree)
2. [internal/pkg -- Shared Utilities](#2-internalpkg----shared-utilities)
3. [internal/identity -- Identity Domain](#3-internalidentity----identity-domain)
4. [internal/authn -- Authentication Adapter](#4-internalauthn----authentication-adapter)
5. [Database Schema Changes](#5-database-schema-changes)
6. [Bridging the Legacy oidcprovider](#6-bridging-the-legacy-oidcprovider)
7. [Migration of main.go Wiring](#7-migration-of-maingo-wiring)
8. [Testing Strategy](#8-testing-strategy)
9. [Future Roadmap](#9-future-roadmap)
10. [Implementation Order](#10-implementation-order)

---

## 1. Target Directory Tree

After this iteration, the accounts service will look like this. Files marked `(unchanged)` are NOT modified; files marked `(adapted)` receive minimal edits to point at new import paths; files marked `(new)` are created from scratch; files marked `(deleted)` are removed entirely.

```
services/accounts/
  main.go                         (adapted -- rewired to use new packages)
  Dockerfile                      (unchanged)
  .env.example                    (unchanged)

  internal/
    pkg/
      domerr/
        errors.go                 (new)
        errors_test.go            (new)
      crypto/
        aes.go                    (new -- extracted from login/handler.go)
        aes_test.go               (new -- extracted from login/handler_test.go)

    identity/
      domain.go                   (new -- pure domain types: User, FederatedIdentity)
      domain_test.go              (new)
      service.go                  (new -- application service / use cases)
      service_test.go             (new)
      ports.go                    (new -- repository interface + service interface)
      postgres/
        user_repo.go              (new -- moved from repo/user.go, targets identity domain types)
        user_repo_test.go         (new -- moved from repo/repo_test.go user tests)

    authn/
      provider.go                 (new -- UpstreamProvider, UpstreamClaims, factory)
      provider_google.go          (new -- extracted from login/upstream.go)
      provider_github.go          (new -- extracted from login/upstream.go)
      handler.go                  (new -- HTTP handlers, depends on identity.Service interface)
      handler_test.go             (new -- adapted from login/handler_test.go)
      config.go                   (new -- authn-specific config subset)

  config/
    config.go                     (adapted -- remove upstream IdP fields, import authn config)
    config_test.go                (adapted)

  oidcprovider/                   (adapted -- storage.go UserReader interface replaced by identity.Service)
    storage.go                    (adapted)
    storage_test.go               (adapted)
    provider.go                   (unchanged)
    authrequest.go                (unchanged)
    authrequest_test.go           (unchanged)
    client.go                     (unchanged)
    client_test.go                (unchanged)
    keys.go                       (unchanged)
    keys_test.go                  (unchanged)
    refreshtoken.go               (unchanged)

  repo/
    authrequest.go                (unchanged -- stays until Iteration 2)
    client.go                     (unchanged -- stays until Iteration 2)
    token.go                      (unchanged -- stays until Iteration 2)
    repo_test.go                  (adapted -- user tests removed, moved to identity/postgres/)

  model/
    authrequest.go                (unchanged -- stays until Iteration 2)
    client.go                     (unchanged -- stays until Iteration 2)
    token.go                      (unchanged -- stays until Iteration 2)
    user.go                       (deleted -- replaced by internal/identity/domain.go)
    federated_identity.go         (deleted -- replaced by internal/identity/domain.go)

  login/                          (deleted entirely -- replaced by internal/authn/)

  migrations/
    001_initial.sql               (adapted -- see Section 5)
    002_seed_clients.sql          (unchanged)
    embed.go                      (unchanged)

  testhelper/
    testdb.go                     (unchanged)
```

### Key Decisions

- **`internal/` is inside `services/accounts/`**, not at the server root. This keeps the modular-monolith boundaries within the service while allowing future services to have their own `internal/` trees.
- **The `model/` and `repo/` packages survive this iteration** for auth requests, clients, and tokens. They will be dismantled in Iteration 2 when `internal/oidc` is created.
- **The `login/` package is deleted entirely**. Its contents are split between `internal/authn/` (HTTP handlers, upstream providers) and `internal/identity/` (findOrCreateUser use case).

---

## 2. internal/pkg -- Shared Utilities

### 2.1 internal/pkg/domerr -- Domain Error Types

A small, dependency-free package defining sentinel errors and error constructors used across all domain modules.

```go
// internal/pkg/domerr/errors.go
package domerr

import "errors"

// Sentinel errors for domain-level failures.
// Infrastructure layers (repos, HTTP handlers) translate these
// into appropriate responses (sql.ErrNoRows -> NotFound, etc.).
var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrUnauthorized  = errors.New("unauthorized")
    ErrInternal      = errors.New("internal error")
)

// Is is a convenience re-export so consumers don't need to import "errors" separately.
func Is(err, target error) bool { return errors.Is(err, target) }
```

**Design rationale:**
- Sentinel errors (not struct types) keep it simple. The `errors.Is` chain works naturally.
- Domain code returns `fmt.Errorf("user %s: %w", id, domerr.ErrNotFound)` -- the caller can `errors.Is(err, domerr.ErrNotFound)` to detect the category.
- HTTP handlers and OIDC adapters translate these into HTTP status codes or OIDC error types at the boundary.

### 2.2 internal/pkg/crypto -- AES-GCM Utilities

Extracted from `login/handler.go`'s `encryptState`/`decryptState`.

```go
// internal/pkg/crypto/aes.go
package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/base64"
    "fmt"
)

// Encrypt encrypts plaintext using AES-256-GCM with the given key,
// returning a base64url-encoded ciphertext string.
func Encrypt(key [32]byte, plaintext []byte) (string, error) {
    block, err := aes.NewCipher(key[:])
    if err != nil {
        return "", fmt.Errorf("aes cipher: %w", err)
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", fmt.Errorf("gcm: %w", err)
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := rand.Read(nonce); err != nil {
        return "", fmt.Errorf("nonce: %w", err)
    }
    ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
    return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decodes a base64url-encoded ciphertext and decrypts it
// using AES-256-GCM with the given key.
func Decrypt(key [32]byte, encoded string) ([]byte, error) {
    ciphertext, err := base64.URLEncoding.DecodeString(encoded)
    if err != nil {
        return nil, fmt.Errorf("base64 decode: %w", err)
    }
    block, err := aes.NewCipher(key[:])
    if err != nil {
        return nil, err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return nil, fmt.Errorf("ciphertext too short")
    }
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return nil, fmt.Errorf("decrypt: %w", err)
    }
    return plaintext, nil
}
```

**Design rationale:**
- Generic `Encrypt(key, plaintext) -> string` / `Decrypt(key, encoded) -> plaintext` -- no knowledge of what is being encrypted.
- The caller (authn handler) marshals/unmarshals the state struct itself. This keeps the crypto package free of domain knowledge.
- Tests are a direct port of the existing `TestEncryptDecryptState_RoundTrip` etc., targeting the raw functions.

---

## 3. internal/identity -- Identity Domain

This is the core of Iteration 1. It defines the User aggregate, the FederatedIdentity value object, the repository port, the application service, and the PostgreSQL adapter.

### 3.1 Domain Types -- `internal/identity/domain.go`

```go
package identity

import "time"

// User is the root aggregate for the identity domain.
// It represents a person who has authenticated at least once.
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

// FederatedIdentity links a User to an external identity provider.
// A User may have multiple federated identities (e.g., both Google and GitHub).
type FederatedIdentity struct {
    ID              string
    UserID          string
    Provider        string
    ProviderSubject string
    CreatedAt       time.Time
}

// FederatedClaims is the set of claims obtained from an upstream identity
// provider during federated authentication. It is NOT persisted directly;
// it is used as input to the FindOrCreate use case.
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

**Key differences from current `model.User` and `model.FederatedIdentity`:**
- No `db:` struct tags. These are pure domain types, decoupled from the persistence mechanism.
- `FederatedClaims` is a new type that replaces `login.UpstreamClaims` as the input to the identity use case. The authn layer maps upstream-provider-specific responses into this generic type before calling the identity service.

### 3.2 Ports -- `internal/identity/ports.go`

```go
package identity

import "context"

// Repository defines the persistence contract for the identity domain.
// It is the "driven port" (secondary port) in hexagonal architecture.
type Repository interface {
    // GetByID retrieves a user by their internal ID.
    // Returns domerr.ErrNotFound if the user does not exist.
    GetByID(ctx context.Context, id string) (*User, error)

    // FindByFederatedIdentity looks up a user by their upstream provider
    // and provider-specific subject identifier.
    // Returns (nil, nil) if no matching federated identity is found.
    FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*User, error)

    // CreateWithFederatedIdentity atomically creates a new User and their
    // initial FederatedIdentity in a single transaction.
    CreateWithFederatedIdentity(ctx context.Context, user *User, fi *FederatedIdentity) error
}

// Service defines the application-level use cases for the identity domain.
// It is the "driving port" (primary port) -- what other modules call.
type Service interface {
    // GetUser retrieves a user by ID.
    // Returns domerr.ErrNotFound if the user does not exist.
    GetUser(ctx context.Context, userID string) (*User, error)

    // FindOrCreateByFederatedLogin looks up a user by their federated identity.
    // If no user exists, it creates one using the provided claims.
    // Returns the user (existing or newly created).
    FindOrCreateByFederatedLogin(ctx context.Context, provider string, claims FederatedClaims) (*User, error)
}
```

**Design rationale:**
- **Two separate interfaces** (Repository and Service) create a clean layered architecture. External modules (`authn`, `oidcprovider`) depend on `identity.Service`, never on `identity.Repository` directly.
- **`FindByFederatedIdentity` returns `(nil, nil)` for not-found** rather than an error. This preserves the current behavior from `repo/user.go:52` and is appropriate because "no matching identity" is a normal outcome, not an error.
- **`CreateWithFederatedIdentity` takes domain types**, not model types with `db:` tags.
- **`GetUser`** is the method the OIDC layer will call for userinfo population, replacing the current `oidcprovider.UserReader.GetByID`.

### 3.3 Application Service -- `internal/identity/service.go`

```go
package identity

import (
    "context"
    "fmt"

    "github.com/google/uuid"

    "github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

// Ensure identityService implements Service at compile time.
var _ Service = (*identityService)(nil)

type identityService struct {
    repo Repository
}

// NewService creates a new identity application service.
func NewService(repo Repository) Service {
    return &identityService{repo: repo}
}

func (s *identityService) GetUser(ctx context.Context, userID string) (*User, error) {
    user, err := s.repo.GetByID(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("identity.GetUser(%s): %w", userID, err)
    }
    return user, nil
}

func (s *identityService) FindOrCreateByFederatedLogin(
    ctx context.Context,
    provider string,
    claims FederatedClaims,
) (*User, error) {
    existing, err := s.repo.FindByFederatedIdentity(ctx, provider, claims.Subject)
    if err != nil {
        return nil, fmt.Errorf("identity.FindOrCreate: lookup: %w", err)
    }
    if existing != nil {
        return existing, nil
    }

    userID := uuid.New().String()
    user := &User{
        ID:            userID,
        Email:         claims.Email,
        EmailVerified: claims.EmailVerified,
        Name:          claims.Name,
        GivenName:     claims.GivenName,
        FamilyName:    claims.FamilyName,
        Picture:       claims.Picture,
    }
    fi := &FederatedIdentity{
        ID:              uuid.New().String(),
        UserID:          userID,
        Provider:        provider,
        ProviderSubject: claims.Subject,
    }
    if err := s.repo.CreateWithFederatedIdentity(ctx, user, fi); err != nil {
        return nil, fmt.Errorf("identity.FindOrCreate: create: %w", err)
    }
    return user, nil
}
```

**What moved here:** The `findOrCreateUser` logic from `login/handler.go:193-222` is now a proper application service method. The handler no longer contains any domain logic.

### 3.4 PostgreSQL Adapter -- `internal/identity/postgres/user_repo.go`

```go
package postgres

import (
    "context"
    "database/sql"
    "errors"
    "time"

    "github.com/jmoiron/sqlx"

    "github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
    "github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

// Ensure UserRepository implements identity.Repository at compile time.
var _ identity.Repository = (*UserRepository)(nil)

type UserRepository struct {
    db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
    return &UserRepository{db: db}
}

// userRow is the database-specific representation. Domain types are never
// annotated with `db:` tags; the mapping happens here at the boundary.
type userRow struct {
    ID            string    `db:"id"`
    Email         string    `db:"email"`
    EmailVerified bool      `db:"email_verified"`
    Name          string    `db:"name"`
    GivenName     string    `db:"given_name"`
    FamilyName    string    `db:"family_name"`
    Picture       string    `db:"picture"`
    CreatedAt     time.Time `db:"created_at"`
    UpdatedAt     time.Time `db:"updated_at"`
}

func (row *userRow) toDomain() *identity.User {
    return &identity.User{
        ID:            row.ID,
        Email:         row.Email,
        EmailVerified: row.EmailVerified,
        Name:          row.Name,
        GivenName:     row.GivenName,
        FamilyName:    row.FamilyName,
        Picture:       row.Picture,
        CreatedAt:     row.CreatedAt,
        UpdatedAt:     row.UpdatedAt,
    }
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*identity.User, error) {
    var row userRow
    err := r.db.QueryRowxContext(ctx,
        `SELECT id, email, email_verified, name, given_name, family_name,
                picture, created_at, updated_at
         FROM users WHERE id = $1`, id,
    ).StructScan(&row)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, domerr.ErrNotFound
    }
    if err != nil {
        return nil, err
    }
    return row.toDomain(), nil
}

func (r *UserRepository) FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*identity.User, error) {
    var row userRow
    err := r.db.QueryRowxContext(ctx,
        `SELECT u.id, u.email, u.email_verified, u.name, u.given_name,
                u.family_name, u.picture, u.created_at, u.updated_at
         FROM users u
         JOIN federated_identities fi ON fi.user_id = u.id
         WHERE fi.provider = $1 AND fi.provider_subject = $2`,
        provider, providerSubject,
    ).StructScan(&row)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, nil
    }
    if err != nil {
        return nil, err
    }
    return row.toDomain(), nil
}

func (r *UserRepository) CreateWithFederatedIdentity(ctx context.Context, user *identity.User, fi *identity.FederatedIdentity) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer func() { _ = tx.Rollback() }()

    _, err = tx.ExecContext(ctx,
        `INSERT INTO users (id, email, email_verified, name, given_name, family_name, picture)
         VALUES ($1, $2, $3, $4, $5, $6, $7)`,
        user.ID, user.Email, user.EmailVerified, user.Name, user.GivenName, user.FamilyName, user.Picture,
    )
    if err != nil {
        return err
    }

    _, err = tx.ExecContext(ctx,
        `INSERT INTO federated_identities (id, user_id, provider, provider_subject)
         VALUES ($1, $2, $3, $4)`,
        fi.ID, fi.UserID, fi.Provider, fi.ProviderSubject,
    )
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

**Key difference from current `repo/user.go`:**
- Returns `domerr.ErrNotFound` instead of raw `sql.ErrNoRows`. The domain error translation happens at the persistence boundary.
- Uses an internal `userRow` struct with `db:` tags for scanning, then converts to the pure domain `identity.User` via `toDomain()`.
- `FindByFederatedIdentity` preserves the `(nil, nil)` convention for not-found.

---

## 4. internal/authn -- Authentication Adapter

This module owns the HTTP login flow and upstream IdP integration. It depends on `identity.Service` (not `identity.Repository`) and `pkg/crypto`.

### 4.1 Config -- `internal/authn/config.go`

```go
package authn

// Config holds the configuration for upstream identity providers.
// It is a focused subset, not the monolithic top-level Config.
type Config struct {
    IssuerURL          string
    GoogleClientID     string
    GoogleClientSecret string
    GitHubClientID     string
    GitHubClientSecret string
}
```

The top-level `config.Config` will populate this subset and pass it to `authn.NewProviders()`. This decouples the authn module from the monolithic config.

### 4.2 Upstream Providers -- extracted from `login/upstream.go`

The `UpstreamProvider` struct and `UpstreamClaims` type move here. The key change: the claims type now maps to `identity.FederatedClaims` at a defined boundary rather than being its own standalone type consumed by both the handler and the identity layer.

```go
// internal/authn/provider.go
package authn

import (
    "context"

    "golang.org/x/oauth2"

    "github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
)

// Provider represents an upstream identity provider (e.g., Google, GitHub).
type Provider struct {
    Name         string
    DisplayName  string
    OAuth2Config *oauth2.Config
    // FetchClaims exchanges an OAuth2 token for identity claims.
    FetchClaims func(ctx context.Context, token *oauth2.Token) (*identity.FederatedClaims, error)
}

// NewProviders builds the list of configured upstream providers.
func NewProviders(ctx context.Context, cfg Config) ([]*Provider, error) {
    var providers []*Provider

    callbackURL := cfg.IssuerURL + "/login/callback"

    if cfg.GoogleClientID != "" {
        p, err := newGoogleProvider(ctx, cfg.GoogleClientID, cfg.GoogleClientSecret, callbackURL)
        if err != nil {
            return nil, fmt.Errorf("google provider: %w", err)
        }
        providers = append(providers, p)
    }

    if cfg.GitHubClientID != "" {
        providers = append(providers, newGitHubProvider(cfg.GitHubClientID, cfg.GitHubClientSecret, callbackURL))
    }

    return providers, nil
}
```

`provider_google.go` and `provider_github.go` contain the extracted Google and GitHub factory functions from `login/upstream.go`, each returning `*Provider`. The `FetchClaims` functions return `*identity.FederatedClaims` directly (instead of the old `*UpstreamClaims`), eliminating a mapping step.

### 4.3 HTTP Handler -- `internal/authn/handler.go`

```go
package authn

import (
    "context"
    "encoding/json"
    "html/template"
    "log/slog"
    "net/http"
    "time"

    "github.com/google/uuid"

    "github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
    "github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/crypto"
)

// AuthRequestQuerier is the narrow interface needed by the handler to validate
// auth request existence and complete the login. This is temporarily satisfied
// by the existing repo.AuthRequestRepository until Iteration 2 extracts it
// into internal/oidc.
type AuthRequestQuerier interface {
    GetByID(ctx context.Context, id string) (AuthRequestInfo, error)
    CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
}

// AuthRequestInfo is a minimal read-only view of an auth request.
// This avoids importing model.AuthRequest directly into authn.
type AuthRequestInfo struct {
    ID string
}

type Handler struct {
    providers   []*Provider
    providerMap map[string]*Provider
    identitySvc identity.Service
    authReqs    AuthRequestQuerier
    cryptoKey   [32]byte
    callbackURL func(context.Context, string) string
    tmpl        *template.Template
    logger      *slog.Logger
}

func NewHandler(
    providers []*Provider,
    identitySvc identity.Service,
    authReqs AuthRequestQuerier,
    cryptoKey [32]byte,
    callbackURL func(context.Context, string) string,
    logger *slog.Logger,
) *Handler {
    pm := make(map[string]*Provider, len(providers))
    for _, p := range providers {
        pm[p.Name] = p
    }
    tmpl := template.Must(template.New("select_provider").Parse(selectProviderHTML))
    return &Handler{
        providers:   providers,
        providerMap: pm,
        identitySvc: identitySvc,
        authReqs:    authReqs,
        cryptoKey:   cryptoKey,
        callbackURL: callbackURL,
        tmpl:        tmpl,
        logger:      logger,
    }
}
```

**Key changes from current `login.Handler`:**
1. **`userRepo` replaced by `identity.Service`**: The handler calls `identitySvc.FindOrCreateByFederatedLogin()` instead of doing the lookup/create itself.
2. **`authReqRepo` replaced by `AuthRequestQuerier` interface**: A narrow, authn-local interface that only requires `GetByID` and `CompleteLogin`. The existing `repo.AuthRequestRepository` satisfies this via a thin adapter (see Section 6).
3. **Crypto delegated to `pkg/crypto`**: `encryptState`/`decryptState` call `crypto.Encrypt`/`crypto.Decrypt` and handle JSON marshaling locally.
4. **No domain logic**: `findOrCreateUser` is gone. The handler is a pure HTTP adapter.

The handler methods (`SelectProvider`, `FederatedRedirect`, `FederatedCallback`) remain structurally identical but with the above dependency changes. The `FederatedCallback` method body becomes:

```go
func (h *Handler) FederatedCallback(w http.ResponseWriter, r *http.Request) {
    code := r.URL.Query().Get("code")
    stateParam := r.URL.Query().Get("state")
    if code == "" || stateParam == "" {
        http.Error(w, "missing code or state", http.StatusBadRequest)
        return
    }

    state, err := h.decryptState(stateParam)
    if err != nil {
        h.logger.Error("failed to decrypt state", "error", err)
        http.Error(w, "invalid state", http.StatusBadRequest)
        return
    }

    provider, ok := h.providerMap[state.Provider]
    if !ok {
        http.Error(w, "unknown provider in state", http.StatusBadRequest)
        return
    }

    token, err := provider.OAuth2Config.Exchange(r.Context(), code)
    if err != nil {
        h.logger.Error("code exchange failed", "provider", state.Provider, "error", err)
        http.Error(w, "authentication failed", http.StatusInternalServerError)
        return
    }

    // Provider returns identity.FederatedClaims directly.
    claims, err := provider.FetchClaims(r.Context(), token)
    if err != nil {
        h.logger.Error("user info retrieval failed", "provider", state.Provider, "error", err)
        http.Error(w, "authentication failed", http.StatusInternalServerError)
        return
    }

    // Delegate to identity service -- no domain logic in the handler.
    user, err := h.identitySvc.FindOrCreateByFederatedLogin(r.Context(), state.Provider, *claims)
    if err != nil {
        h.logger.Error("find or create user failed", "error", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    now := time.Now().UTC()
    if err := h.authReqs.CompleteLogin(r.Context(), state.AuthRequestID, user.ID, now, []string{"federated"}); err != nil {
        h.logger.Error("complete login failed", "error", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    callbackURL := h.callbackURL(r.Context(), state.AuthRequestID)
    http.Redirect(w, r, callbackURL, http.StatusFound)
}
```

### 4.4 State Encryption -- simplified

```go
type federatedState struct {
    AuthRequestID string `json:"a"`
    Provider      string `json:"p"`
    Nonce         string `json:"n"`
}

func (h *Handler) encryptState(state federatedState) (string, error) {
    plaintext, err := json.Marshal(state)
    if err != nil {
        return "", err
    }
    return crypto.Encrypt(h.cryptoKey, plaintext)
}

func (h *Handler) decryptState(encoded string) (federatedState, error) {
    var state federatedState
    plaintext, err := crypto.Decrypt(h.cryptoKey, encoded)
    if err != nil {
        return state, err
    }
    if err := json.Unmarshal(plaintext, &state); err != nil {
        return state, fmt.Errorf("unmarshal state: %w", err)
    }
    return state, nil
}
```

---

## 5. Database Schema Changes

**No schema changes for the `users` and `federated_identities` tables.** The database schema is fine; the problem was that Go domain types had `db:` tags. The new architecture uses internal `Row` structs in the postgres adapter layer for scanning, and pure domain types above that. The SQL in `001_initial.sql` stays as-is.

The `model/user.go` and `model/federated_identity.go` files are deleted because their types are replaced by `identity.User` and `identity.FederatedIdentity` in the domain layer, and `postgres.userRow` in the persistence layer.

---

## 6. Bridging the Legacy oidcprovider

The `oidcprovider.Storage` god-object currently depends on a `UserReader` interface that returns `*model.User`. After Iteration 1, the identity domain owns the `User` type. We need a bridge.

### 6.1 Replace `oidcprovider.UserReader` with `identity.Service`

The current `UserReader` interface in `oidcprovider/storage.go`:

```go
// CURRENT
type UserReader interface {
    GetByID(ctx context.Context, id string) (*model.User, error)
    FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*model.User, error)
    CreateWithFederatedIdentity(ctx context.Context, u *model.User, fi *model.FederatedIdentity) error
}
```

Is replaced by depending on `identity.Service` directly:

```go
// NEW -- in oidcprovider/storage.go
import "github.com/barn0w1/hss-science/server/services/accounts/internal/identity"

type Storage struct {
    db                   *sqlx.DB
    identitySvc          identity.Service   // replaces userRepo UserReader
    clientRepo           ClientReader
    authReqRepo          AuthRequestStore
    tokenRepo            TokenStore
    signing              *SigningKeyWithID
    public               *PublicKeyWithID
    accessTokenLifetime  time.Duration
    refreshTokenLifetime time.Duration
}
```

The methods that used `s.userRepo.GetByID` now call `s.identitySvc.GetUser`:

```go
func (s *Storage) setUserinfo(ctx context.Context, userinfo *oidc.UserInfo, userID string, scopes []string) error {
    user, err := s.identitySvc.GetUser(ctx, userID)
    if err != nil {
        if domerr.Is(err, domerr.ErrNotFound) {
            return oidc.ErrInvalidRequest().WithDescription("user not found")
        }
        return err
    }

    for _, scope := range scopes {
        switch scope {
        case oidc.ScopeOpenID:
            userinfo.Subject = user.ID
        case oidc.ScopeProfile:
            userinfo.Name = user.Name
            userinfo.GivenName = user.GivenName
            userinfo.FamilyName = user.FamilyName
            userinfo.Picture = user.Picture
            userinfo.UpdatedAt = oidc.FromTime(user.UpdatedAt)
        case oidc.ScopeEmail:
            userinfo.Email = user.Email
            userinfo.EmailVerified = oidc.Bool(user.EmailVerified)
        }
    }
    return nil
}
```

**This is a clean cut:** The `oidcprovider` package no longer imports `model.User` or `model.FederatedIdentity`. It imports `identity.User` from the domain layer. The only `model.*` types it still uses are `model.AuthRequest`, `model.Client`, `model.Token`, `model.RefreshToken` -- all of which stay until Iteration 2.

### 6.2 AuthRequestQuerier Adapter for authn

The `authn.AuthRequestQuerier` interface needs to be satisfied by the existing `repo.AuthRequestRepository`. Since `authn` must not import `model.AuthRequest`, we create a thin adapter in `main.go` wiring:

```go
// In main.go, a small adapter struct that bridges repo.AuthRequestRepository
// to the authn.AuthRequestQuerier interface.
type authReqAdapter struct {
    repo *repo.AuthRequestRepository
}

func (a *authReqAdapter) GetByID(ctx context.Context, id string) (authn.AuthRequestInfo, error) {
    ar, err := a.repo.GetByID(ctx, id)
    if err != nil {
        return authn.AuthRequestInfo{}, err
    }
    return authn.AuthRequestInfo{ID: ar.ID}, nil
}

func (a *authReqAdapter) CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error {
    return a.repo.CompleteLogin(ctx, id, userID, authTime, amr)
}
```

This adapter lives in `main.go` because it's pure wiring glue -- it bridges two modules that shouldn't know about each other. It will be deleted in Iteration 2 when `internal/oidc` absorbs auth request management.

---

## 7. Migration of main.go Wiring

The new `main.go` wiring replaces the old dependency graph:

```go
func main() {
    logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    slog.SetDefault(logger)

    cfg, err := config.Load()
    if err != nil {
        logger.Error("failed to load config", "error", err)
        os.Exit(1)
    }

    db, err := sqlx.Connect("postgres", cfg.DatabaseURL)
    if err != nil {
        logger.Error("failed to connect to database", "error", err)
        os.Exit(1)
    }
    defer func() { _ = db.Close() }()

    // --- Identity module ---
    identityRepo := identitypg.NewUserRepository(db)
    identitySvc  := identity.NewService(identityRepo)

    // --- Legacy repos (stay until Iteration 2) ---
    clientRepo  := repo.NewClientRepository(db)
    authReqRepo := repo.NewAuthRequestRepository(db)
    tokenRepo   := repo.NewTokenRepository(db)

    // --- OIDC provider (adapted to use identity.Service) ---
    signingKey := oidcprovider.NewSigningKey(cfg.SigningKey)
    publicKey  := oidcprovider.NewPublicKey(cfg.SigningKey)

    storage := oidcprovider.NewStorage(
        db, identitySvc, clientRepo, authReqRepo, tokenRepo,
        signingKey, publicKey,
        time.Duration(cfg.AccessTokenLifetimeMinutes)*time.Minute,
        time.Duration(cfg.RefreshTokenLifetimeDays)*24*time.Hour,
    )

    provider, err := oidcprovider.NewProvider(cfg.Issuer, cfg.CryptoKey, storage, logger)
    if err != nil {
        logger.Error("failed to create OIDC provider", "error", err)
        os.Exit(1)
    }

    // --- Authn module ---
    authnCfg := authn.Config{
        IssuerURL:          cfg.Issuer,
        GoogleClientID:     cfg.GoogleClientID,
        GoogleClientSecret: cfg.GoogleClientSecret,
        GitHubClientID:     cfg.GitHubClientID,
        GitHubClientSecret: cfg.GitHubClientSecret,
    }
    upstreamProviders, err := authn.NewProviders(context.Background(), authnCfg)
    if err != nil {
        logger.Error("failed to initialize upstream providers", "error", err)
        os.Exit(1)
    }

    authReqBridge := &authReqAdapter{repo: authReqRepo} // temporary bridge

    loginHandler := authn.NewHandler(
        upstreamProviders,
        identitySvc,
        authReqBridge,
        cfg.CryptoKey,
        op.AuthCallbackURL(provider),
        logger,
    )

    // --- Routing (unchanged structure) ---
    router := chi.NewRouter()
    router.Use(middleware.Recoverer)

    interceptor := op.NewIssuerInterceptor(provider.IssuerFromRequest)
    router.Route("/login", func(r chi.Router) {
        r.Use(interceptor.Handler)
        r.Get("/", loginHandler.SelectProvider)
        r.Post("/select", loginHandler.FederatedRedirect)
        r.Get("/callback", loginHandler.FederatedCallback)
    })

    // ... healthz, readyz, logged-out, provider mount -- identical to current ...
}
```

### What's imported from where

| Package alias | Import path | New/Changed? |
|---|---|---|
| `identity` | `.../internal/identity` | New |
| `identitypg` | `.../internal/identity/postgres` | New |
| `authn` | `.../internal/authn` | New |
| `repo` | `.../repo` | Unchanged (minus user repo) |
| `oidcprovider` | `.../oidcprovider` | Adapted (new Storage constructor signature) |
| `config` | `.../config` | Adapted |

---

## 8. Testing Strategy

### 8.1 Unit Tests (no database)

| Package | Test File | What's tested |
|---|---|---|
| `internal/pkg/domerr` | `errors_test.go` | `errors.Is` chaining with wrapped domain errors |
| `internal/pkg/crypto` | `aes_test.go` | Encrypt/Decrypt round-trip, wrong key, short ciphertext, invalid base64 |
| `internal/identity` | `service_test.go` | `FindOrCreateByFederatedLogin` with a mock `Repository`: existing user path, new user path, repo error propagation |
| `internal/identity` | `domain_test.go` | Any domain validation if added later |
| `internal/authn` | `handler_test.go` | All existing handler tests from `login/handler_test.go`, adapted to use mock `identity.Service` and mock `AuthRequestQuerier` |

### 8.2 Integration Tests (testcontainers PostgreSQL)

| Package | Test File | What's tested |
|---|---|---|
| `internal/identity/postgres` | `user_repo_test.go` | All user repo operations against real PostgreSQL: Create+GetByID, FindByFederatedIdentity, CreateWithFederatedIdentity, not-found returns `domerr.ErrNotFound` |
| `oidcprovider` | `storage_test.go` | Adapted to construct `identity.NewService(identitypg.NewUserRepository(db))` instead of raw `repo.NewUserRepository(db)` |

### 8.3 Deleted / Moved Tests

- `repo/repo_test.go`: User-related tests (TestUserRepository_*) move to `internal/identity/postgres/user_repo_test.go`. Client, AuthRequest, and Token tests stay.
- `login/handler_test.go`: Moves entirely to `internal/authn/handler_test.go`. Tests are adapted to mock `identity.Service` instead of using nil `userRepo`.

### 8.4 Test Container Consolidation

Both `repo/repo_test.go` and `oidcprovider/storage_test.go` currently spin up separate PostgreSQL containers. After this iteration, `internal/identity/postgres/` also needs one. We should consolidate by having each test suite use `testhelper.RunMigrations` with its own testcontainers setup, which is unchanged. The two-container problem is deferred to a future infrastructure improvement (shared test binary or build-tag-gated shared setup).

---

## 9. Future Roadmap

### Iteration 2: internal/oidc Domain

**Scope:** Extract everything the zitadel `op.Storage` interface needs into `internal/oidc/`.

#### 9.1 AuthRequest Management

- Move `model.AuthRequest` into `internal/oidc/domain.go` (or a sub-domain like `authflow`).
- Move `repo/authrequest.go` into `internal/oidc/postgres/authrequest_repo.go`.
- Extract the 30-minute TTL from the SQL `activeFilter` into a configurable domain constant.
- Create an `oidc.AuthRequestService` that owns creation, code-saving, completion, and deletion.
- The `authn` module's `AuthRequestQuerier` interface will be satisfied by `oidc.AuthRequestService` instead of the `authReqAdapter` bridge.
- Delete the `authReqAdapter` from `main.go`.

#### 9.2 Client Management

- Move `model.Client` into `internal/oidc/domain.go`.
- Move `repo/client.go` into `internal/oidc/postgres/client_repo.go`.
- Move bcrypt secret verification from `oidcprovider.Storage.AuthorizeClientIDSecret` into an `oidc.ClientService`.
- Move the string-to-enum mapping (ApplicationType, AuthMethod, etc.) from `oidcprovider/client.go` adapter into the domain or a dedicated mapper.

#### 9.3 Token Management

- Move `model.Token` and `model.RefreshToken` into `internal/oidc/domain.go`.
- Move `repo/token.go` into `internal/oidc/postgres/token_repo.go`.
- Create an `oidc.TokenService` that owns creation, refresh, revocation, and introspection.

#### 9.4 Dismantling the Storage God-Object

Once all domain services exist, `oidcprovider/storage.go` becomes a pure **compositor** -- a thin struct that holds references to `identity.Service`, `oidc.AuthRequestService`, `oidc.ClientService`, `oidc.TokenService`, and delegates each `op.Storage` method to the appropriate service. Each method becomes a 1-3 line delegation. The `NewStorage` constructor takes service interfaces instead of repos.

#### 9.5 Key Management

- Move `oidcprovider/keys.go` to `internal/oidc/keys/` or `internal/pkg/crypto/`.
- Add key rotation support: store multiple keys in the database, return all active keys in `KeySet()`, sign with the newest key.

### Iteration 3: Cleanup and Polish

- Delete the empty `model/`, `repo/`, `login/` packages entirely.
- Delete the `old` config fields that were absorbed into module-specific configs.
- Unify test infrastructure into a single shared testcontainers setup.
- Add OpenTelemetry tracing spans to key operations.
- Add structured request-ID logging middleware.
- Consider CSRF protection on the provider selection form.

### Target End-State Directory Tree (after all iterations)

```
services/accounts/
  main.go
  Dockerfile
  .env.example
  internal/
    pkg/
      domerr/
      crypto/
    identity/
      domain.go
      ports.go
      service.go
      postgres/
        user_repo.go
    authn/
      config.go
      provider.go
      provider_google.go
      provider_github.go
      handler.go
    oidc/
      domain.go         (AuthRequest, Client, Token, RefreshToken)
      ports.go          (service + repo interfaces)
      authrequest_svc.go
      client_svc.go
      token_svc.go
      adapter/
        storage.go      (thin op.Storage compositor)
        authrequest.go  (op.AuthRequest adapter)
        client.go       (op.Client adapter)
        refreshtoken.go (op.RefreshTokenRequest adapter)
        keys.go
        provider.go
      postgres/
        authrequest_repo.go
        client_repo.go
        token_repo.go
  config/
    config.go
  migrations/
    001_initial.sql
    002_seed_clients.sql
    embed.go
  testhelper/
    testdb.go
```

---

## 10. Implementation Order

The implementation should proceed in this exact order, with each step producing a compilable, test-passing codebase:

### Step 1: Create `internal/pkg/domerr`
- Create `internal/pkg/domerr/errors.go` with sentinel errors.
- Create `internal/pkg/domerr/errors_test.go`.
- No existing code changes. Just adding new files.

### Step 2: Create `internal/pkg/crypto`
- Create `internal/pkg/crypto/aes.go` and `aes_test.go`.
- These are standalone functions. No existing code changes yet.
- Verify tests pass in isolation.

### Step 3: Create `internal/identity` domain + service
- Create `internal/identity/domain.go` (User, FederatedIdentity, FederatedClaims).
- Create `internal/identity/ports.go` (Repository interface, Service interface).
- Create `internal/identity/service.go` (identityService implementing Service).
- Create `internal/identity/service_test.go` with mock Repository.

### Step 4: Create `internal/identity/postgres` adapter
- Create `internal/identity/postgres/user_repo.go` (port of `repo/user.go` targeting domain types).
- Create `internal/identity/postgres/user_repo_test.go` (port of user-related tests from `repo/repo_test.go`).
- Verify integration tests pass.

### Step 5: Create `internal/authn`
- Create `internal/authn/config.go`, `provider.go`, `provider_google.go`, `provider_github.go`.
- Create `internal/authn/handler.go` with the new Handler depending on `identity.Service`.
- Create `internal/authn/handler_test.go`.
- This does NOT yet touch any existing file.

### Step 6: Adapt `oidcprovider/storage.go` to use `identity.Service`
- Replace the `UserReader` interface with `identity.Service` in Storage struct.
- Update `NewStorage` constructor signature.
- Update all methods that called `s.userRepo.GetByID` to call `s.identitySvc.GetUser`.
- Update `setUserinfo`, `setIntrospectionUserinfo`, `SetIntrospectionFromToken` to work with `identity.User`.
- Update `oidcprovider/storage_test.go` to construct the service chain.
- Remove the `UserReader` interface declaration.

### Step 7: Rewire `main.go`
- Change imports to use `internal/identity`, `internal/identity/postgres`, `internal/authn`.
- Add the `authReqAdapter` bridge struct.
- Remove the old `repo.NewUserRepository` call and `login.NewHandler`/`login.NewUpstreamProviders` calls.
- Remove imports of `login` and the user-repo portion of `repo`.

### Step 8: Delete old code
- Delete `login/` directory entirely.
- Delete `model/user.go` and `model/federated_identity.go`.
- Remove user-related tests from `repo/repo_test.go`.
- Remove user-related code from `repo/user.go` (delete the file entirely since it only has user code).

### Step 9: Adapt `config/config.go`
- Keep the upstream IdP fields in config.Config for now (they're still needed to populate `authn.Config`). Removing them is a cosmetic change that can happen in Iteration 3 if desired.
- No functional change required; the top-level config populates `authn.Config` in main.go.

### Step 10: Final verification
- Run full test suite: `go test ./services/accounts/...`
- Run linter: `golangci-lint run ./services/accounts/...`
- Verify no import cycles.
- Verify `model/` only contains `authrequest.go`, `client.go`, `token.go`.
- Verify `repo/` only contains `authrequest.go`, `client.go`, `token.go`, `repo_test.go`.
- Verify `login/` is gone.
