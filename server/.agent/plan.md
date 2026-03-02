# Accounts Service Reengineering Plan -- Iteration 1

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
      domain.go                   (new -- reengineered domain types: User, FederatedIdentity, UserWithClaims)
      domain_test.go              (new)
      service.go                  (new -- application service / use cases)
      service_test.go             (new)
      ports.go                    (new -- repository interface + service interface)
      postgres/
        user_repo.go              (new -- reengineered from repo/user.go, targets new domain types)
        user_repo_test.go         (new -- moved from repo/repo_test.go user tests)

    authn/
      provider.go                 (new -- Provider struct, factory)
      provider_google.go          (new -- extracted from login/upstream.go)
      provider_github.go          (new -- extracted from login/upstream.go)
      handler.go                  (new -- HTTP handlers, depends on identity.Service interface)
      handler_test.go             (new -- adapted from login/handler_test.go)
      config.go                   (new -- authn-specific config subset)

  config/
    config.go                     (adapted -- upstream IdP fields remain, used to populate authn.Config)
    config_test.go                (unchanged)

  oidcprovider/                   (adapted -- storage.go bridges to identity.Service for userinfo)
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
    001_initial.sql               (reengineered -- see Section 5)
    002_seed_clients.sql          (unchanged)
    embed.go                      (unchanged)

  testhelper/
    testdb.go                     (adapted -- CleanTables ordering updated for new schema)
```

### Key Decisions

- **`internal/` is inside `services/accounts/`**, not at the server root. This keeps the modular-monolith boundaries within the service while allowing future services to have their own `internal/` trees.
- **The `model/` and `repo/` packages partially survive this iteration** (auth requests, clients, tokens). They will be dismantled in Iteration 2 when `internal/oidc` is created.
- **The `login/` package is deleted entirely**. Its contents are split between `internal/authn/` (HTTP handlers, upstream providers) and `internal/identity/` (FindOrCreate use case).
- **The `users` and `federated_identities` tables are fully reengineered** with a new schema reflecting the correct domain ownership of profile data (see Section 5).

---

## 2. internal/pkg -- Shared Utilities

### 2.1 internal/pkg/domerr -- Domain Error Types

A small, dependency-free package defining sentinel errors used across all domain modules.

```go
// internal/pkg/domerr/errors.go
package domerr

import "errors"

// Sentinel errors for domain-level failures.
// Infrastructure layers (repos, HTTP handlers) translate these
// into appropriate responses (sql.ErrNoRows -> ErrNotFound, etc.).
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
- Sentinel errors keep it simple. The `errors.Is` chain works naturally through wrapping.
- Domain code wraps them: `fmt.Errorf("user %s: %w", id, domerr.ErrNotFound)`.
- HTTP handlers and OIDC adapters translate these into HTTP status codes or OIDC error types at their respective boundaries.

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
- The caller (authn handler) handles JSON marshaling/unmarshaling of its state struct. This keeps the crypto package free of domain knowledge.
- Tests are direct ports of the existing `TestEncryptDecryptState_*` suite, targeting the raw functions.

---

## 3. internal/identity -- Identity Domain

This is the core of Iteration 1. The key reengineering insight is that profile fields (`Email`, `EmailVerified`, `Name`, `Picture`, etc.) are **claims provided by the upstream identity provider** -- they belong to `FederatedIdentity`, not to `User`. The `User` aggregate is merely a stable, internal anchor; the proof that an entity exists in our system. All profile data is derived from its associated federated identities.

### 3.1 Domain Types -- `internal/identity/domain.go`

```go
// internal/identity/domain.go
package identity

import "time"

// User is the root aggregate for the identity domain.
// It is a stable, minimal anchor representing a person who has
// authenticated at least once. It carries NO profile data --
// all profile claims are owned by the FederatedIdentity records
// associated with this user, because those claims originate from
// upstream identity providers, not from our system.
type User struct {
    ID        string    // ULID, application-generated
    CreatedAt time.Time
}

// FederatedIdentity links a User to a specific upstream identity provider
// and carries the profile claims as last reported by that provider.
// A User may accumulate multiple FederatedIdentity records (e.g., one for
// Google, one for GitHub).
type FederatedIdentity struct {
    ID              string    // ULID, application-generated
    UserID          string
    Provider        string    // e.g. "google", "github"
    ProviderSubject string    // the upstream provider's user ID for this subject

    // Profile claims sourced from the upstream provider.
    // These are refreshed on every successful login.
    Email         string
    EmailVerified bool
    DisplayName   string    // maps to OIDC standard "name" claim
    GivenName     string
    FamilyName    string
    PictureURL    string

    LastLoginAt time.Time // updated on every successful federated login
    CreatedAt   time.Time
    UpdatedAt   time.Time // updated whenever profile claims are refreshed
}

// FederatedClaims is the set of claims obtained from an upstream identity
// provider during federated authentication. It is transient -- NOT persisted
// directly. It is used as input to the FindOrCreateByFederatedLogin use case,
// which maps the claims onto a FederatedIdentity record.
type FederatedClaims struct {
    Subject       string
    Email         string
    EmailVerified bool
    Name          string // full display name
    GivenName     string
    FamilyName    string
    Picture       string
}

// UserWithClaims is a read model that aggregates a User with the profile
// claims from their most recently used federated identity.
// It is the view consumed by the OIDC layer to populate userinfo and
// introspection endpoints. It is NOT a domain entity -- it is derived
// on read and never persisted as a unit.
type UserWithClaims struct {
    UserID          string
    Email           string
    EmailVerified   bool
    Name            string
    GivenName       string
    FamilyName      string
    Picture         string
    ClaimsUpdatedAt time.Time // federated_identity.updated_at of the primary record
}
```

**Why `User` has no profile fields:**
- Email, name, and picture come from the IdP -- they are that provider's assertion, not our own. A user's Google profile and GitHub profile may differ; neither is definitively "our" data.
- If we copied profile data onto `User`, we'd face the question of which provider's fields win, and when to re-sync them. Keeping them on `FederatedIdentity` sidesteps this entirely.
- The core `User.ID` is what all other parts of the system (OIDC tokens, auth requests) reference. Its immutability and stability are what matter.

**Why `UserWithClaims` as a read model:**
- The OIDC layer needs a unified, flat view of a user's claims to populate `userinfo` and introspection responses. Rather than the OIDC layer knowing how to navigate `FederatedIdentity` records, the identity service provides a pre-aggregated read model.
- When a user has multiple federated identities, the most recently logged-in one is used as the primary claim source (ordered by `last_login_at DESC`). This is a clear, stable policy.

### 3.2 Ports -- `internal/identity/ports.go`

```go
// internal/identity/ports.go
package identity

import (
    "context"
    "time"
)

// Repository defines the persistence contract for the identity domain.
// It is the "driven port" (secondary port) -- what the service calls internally.
// External modules MUST NOT depend on this interface; they use Service instead.
type Repository interface {
    // GetByID retrieves a user by their internal ID.
    // Returns domerr.ErrNotFound if the user does not exist.
    GetByID(ctx context.Context, id string) (*User, error)

    // FindByFederatedIdentity looks up a user by their upstream provider
    // and provider-specific subject identifier.
    // Returns (nil, nil) if no matching federated identity is found.
    // This (nil, nil) convention signals "not yet registered" as a normal
    // outcome -- not an error.
    FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*User, error)

    // CreateWithFederatedIdentity atomically creates a new User and their
    // initial FederatedIdentity in a single transaction.
    CreateWithFederatedIdentity(ctx context.Context, user *User, fi *FederatedIdentity) error

    // UpdateFederatedIdentityClaims refreshes the profile claims on an existing
    // FederatedIdentity record for the given provider and subject, and records
    // the login timestamp. Called on every successful federated login where
    // the user already exists.
    UpdateFederatedIdentityClaims(
        ctx context.Context,
        provider, providerSubject string,
        claims FederatedClaims,
        lastLoginAt time.Time,
    ) error

    // GetUserWithClaims returns the user's ID combined with their most recent
    // federated identity's profile claims (latest by last_login_at).
    // Returns domerr.ErrNotFound if the user does not exist.
    GetUserWithClaims(ctx context.Context, userID string) (*UserWithClaims, error)
}

// Service defines the application-level use cases for the identity domain.
// It is the "driving port" (primary port) -- the contract that external modules
// (authn, oidcprovider) depend on.
type Service interface {
    // GetUser retrieves a user's stable identity by ID.
    // Returns domerr.ErrNotFound if the user does not exist.
    GetUser(ctx context.Context, userID string) (*User, error)

    // GetUserWithClaims retrieves a user with their aggregated profile claims,
    // sourced from the most recently used federated identity.
    // Returns domerr.ErrNotFound if the user does not exist.
    // Used by the OIDC layer to populate userinfo/introspection responses.
    GetUserWithClaims(ctx context.Context, userID string) (*UserWithClaims, error)

    // FindOrCreateByFederatedLogin is the primary login use case.
    //
    // For an existing user: the FederatedIdentity is updated with the
    // fresh claims from the upstream provider (e.g., if the user's email or
    // picture changed). The User record itself is NOT modified.
    //
    // For a new user: a User and their initial FederatedIdentity are
    // created atomically from the provided claims.
    //
    // Returns the User (existing or newly created) in both cases.
    FindOrCreateByFederatedLogin(ctx context.Context, provider string, claims FederatedClaims) (*User, error)
}
```

### 3.3 Application Service -- `internal/identity/service.go`

```go
// internal/identity/service.go
package identity

import (
    "context"
    "fmt"
    "time"

    "github.com/oklog/ulid/v2"

    "github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

// Ensure identityService implements Service at compile time.
var _ Service = (*identityService)(nil)

type identityService struct {
    repo Repository
}

// NewService creates a new identity application service backed by the given repository.
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

func (s *identityService) GetUserWithClaims(ctx context.Context, userID string) (*UserWithClaims, error) {
    uwc, err := s.repo.GetUserWithClaims(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("identity.GetUserWithClaims(%s): %w", userID, err)
    }
    return uwc, nil
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
        // User exists. Refresh the FederatedIdentity record with the latest
        // claims from the upstream provider (email changes, picture updates, etc.).
        // We intentionally never modify the User record itself -- the User is a
        // stable internal anchor. Only IdP-sourced data on FederatedIdentity changes.
        now := time.Now().UTC()
        if err := s.repo.UpdateFederatedIdentityClaims(ctx, provider, claims.Subject, claims, now); err != nil {
            return nil, fmt.Errorf("identity.FindOrCreate: update claims: %w", err)
        }
        return existing, nil
    }

    // New user: create User + FederatedIdentity atomically.
    now := time.Now().UTC()
    user := &User{
        ID:        newID(),
        CreatedAt: now,
    }
    fi := &FederatedIdentity{
        ID:              newID(),
        UserID:          user.ID,
        Provider:        provider,
        ProviderSubject: claims.Subject,
        Email:           claims.Email,
        EmailVerified:   claims.EmailVerified,
        DisplayName:     claims.Name,
        GivenName:       claims.GivenName,
        FamilyName:      claims.FamilyName,
        PictureURL:      claims.Picture,
        LastLoginAt:     now,
        CreatedAt:       now,
        UpdatedAt:       now,
    }
    if err := s.repo.CreateWithFederatedIdentity(ctx, user, fi); err != nil {
        return nil, fmt.Errorf("identity.FindOrCreate: create: %w", err)
    }
    return user, nil
}

// newID generates a new ULID string. ULIDs are time-sortable, URL-safe,
// and application-generated, avoiding database-side ID generation.
func newID() string {
    return ulid.Make().String()
}
```

**What moved here from `login/handler.go`:** The `findOrCreateUser` logic (lines 193-222) is now a proper application service method. The handler has zero domain logic.

**Key change from the original plan:** The existing-user path now calls `UpdateFederatedIdentityClaims` before returning. This ensures every successful login refreshes the profile data from the upstream provider -- email, verified status, display name, picture -- while the `User` record remains untouched.

### 3.4 PostgreSQL Adapter -- `internal/identity/postgres/user_repo.go`

```go
// internal/identity/postgres/user_repo.go
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

// userRow and federatedIdentityRow are database-specific scan targets.
// Domain types carry NO `db:` tags; all schema coupling lives here at the boundary.
type userRow struct {
    ID        string    `db:"id"`
    CreatedAt time.Time `db:"created_at"`
}

type federatedIdentityRow struct {
    ID              string    `db:"id"`
    UserID          string    `db:"user_id"`
    Provider        string    `db:"provider"`
    ProviderSubject string    `db:"provider_subject"`
    Email           string    `db:"email"`
    EmailVerified   bool      `db:"email_verified"`
    DisplayName     string    `db:"display_name"`
    GivenName       string    `db:"given_name"`
    FamilyName      string    `db:"family_name"`
    PictureURL      string    `db:"picture_url"`
    LastLoginAt     time.Time `db:"last_login_at"`
    CreatedAt       time.Time `db:"created_at"`
    UpdatedAt       time.Time `db:"updated_at"`
}

type userWithClaimsRow struct {
    UserID          string    `db:"user_id"`
    Email           string    `db:"email"`
    EmailVerified   bool      `db:"email_verified"`
    DisplayName     string    `db:"display_name"`
    GivenName       string    `db:"given_name"`
    FamilyName      string    `db:"family_name"`
    PictureURL      string    `db:"picture_url"`
    ClaimsUpdatedAt time.Time `db:"claims_updated_at"`
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*identity.User, error) {
    var row userRow
    err := r.db.QueryRowxContext(ctx,
        `SELECT id, created_at FROM users WHERE id = $1`, id,
    ).StructScan(&row)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, domerr.ErrNotFound
    }
    if err != nil {
        return nil, err
    }
    return &identity.User{ID: row.ID, CreatedAt: row.CreatedAt}, nil
}

func (r *UserRepository) FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*identity.User, error) {
    var row userRow
    err := r.db.QueryRowxContext(ctx,
        `SELECT u.id, u.created_at
         FROM users u
         JOIN federated_identities fi ON fi.user_id = u.id
         WHERE fi.provider = $1 AND fi.provider_subject = $2`,
        provider, providerSubject,
    ).StructScan(&row)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, nil // (nil, nil) signals "not yet registered" -- a normal outcome
    }
    if err != nil {
        return nil, err
    }
    return &identity.User{ID: row.ID, CreatedAt: row.CreatedAt}, nil
}

func (r *UserRepository) CreateWithFederatedIdentity(
    ctx context.Context,
    user *identity.User,
    fi *identity.FederatedIdentity,
) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer func() { _ = tx.Rollback() }()

    _, err = tx.ExecContext(ctx,
        `INSERT INTO users (id, created_at) VALUES ($1, $2)`,
        user.ID, user.CreatedAt,
    )
    if err != nil {
        return err
    }

    _, err = tx.ExecContext(ctx,
        `INSERT INTO federated_identities
            (id, user_id, provider, provider_subject,
             email, email_verified, display_name, given_name, family_name, picture_url,
             last_login_at, created_at, updated_at)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
        fi.ID, fi.UserID, fi.Provider, fi.ProviderSubject,
        fi.Email, fi.EmailVerified, fi.DisplayName, fi.GivenName, fi.FamilyName, fi.PictureURL,
        fi.LastLoginAt, fi.CreatedAt, fi.UpdatedAt,
    )
    if err != nil {
        return err
    }

    return tx.Commit()
}

func (r *UserRepository) UpdateFederatedIdentityClaims(
    ctx context.Context,
    provider, providerSubject string,
    claims identity.FederatedClaims,
    lastLoginAt time.Time,
) error {
    _, err := r.db.ExecContext(ctx,
        `UPDATE federated_identities
         SET email           = $1,
             email_verified  = $2,
             display_name    = $3,
             given_name      = $4,
             family_name     = $5,
             picture_url     = $6,
             last_login_at   = $7,
             updated_at      = now()
         WHERE provider = $8 AND provider_subject = $9`,
        claims.Email, claims.EmailVerified, claims.Name,
        claims.GivenName, claims.FamilyName, claims.Picture,
        lastLoginAt, provider, providerSubject,
    )
    return err
}

func (r *UserRepository) GetUserWithClaims(ctx context.Context, userID string) (*identity.UserWithClaims, error) {
    // Selects the user's stable ID combined with the profile claims from the
    // federated identity most recently used to log in. When a user has multiple
    // federated identities (e.g., Google + GitHub), the most recently active
    // one is treated as the primary claims source.
    var row userWithClaimsRow
    err := r.db.QueryRowxContext(ctx,
        `SELECT
             u.id              AS user_id,
             fi.email          AS email,
             fi.email_verified AS email_verified,
             fi.display_name   AS display_name,
             fi.given_name     AS given_name,
             fi.family_name    AS family_name,
             fi.picture_url    AS picture_url,
             fi.updated_at     AS claims_updated_at
         FROM users u
         JOIN federated_identities fi ON fi.user_id = u.id
         WHERE u.id = $1
         ORDER BY fi.last_login_at DESC
         LIMIT 1`,
        userID,
    ).StructScan(&row)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, domerr.ErrNotFound
    }
    if err != nil {
        return nil, err
    }
    return &identity.UserWithClaims{
        UserID:          row.UserID,
        Email:           row.Email,
        EmailVerified:   row.EmailVerified,
        Name:            row.DisplayName,
        GivenName:       row.GivenName,
        FamilyName:      row.FamilyName,
        Picture:         row.PictureURL,
        ClaimsUpdatedAt: row.ClaimsUpdatedAt,
    }, nil
}
```

**Key differences from the old `repo/user.go`:**
- `userRow` only maps `id` and `created_at` -- no profile fields.
- `federatedIdentityRow` carries all the profile columns that used to live on `userRow`.
- `UpdateFederatedIdentityClaims` is a new method that refreshes profile data on every login.
- `GetUserWithClaims` joins `users` + latest `federated_identities` record by `last_login_at`.
- All `sql.ErrNoRows` responses are translated to `domerr.ErrNotFound` at this boundary.

---

## 4. internal/authn -- Authentication Adapter

This module owns the HTTP login flow and upstream IdP integration. It depends on `identity.Service` (not `identity.Repository`) and `pkg/crypto`.

### 4.1 Config -- `internal/authn/config.go`

```go
// internal/authn/config.go
package authn

// Config holds the configuration for upstream identity providers.
// It is a focused subset of the top-level service config,
// decoupling the authn module from the monolithic config.Config struct.
type Config struct {
    IssuerURL          string
    GoogleClientID     string
    GoogleClientSecret string
    GitHubClientID     string
    GitHubClientSecret string
}
```

The top-level `config.Config` populates this struct in `main.go` and passes it to `authn.NewProviders()`.

### 4.2 Upstream Providers -- `internal/authn/provider.go`

The `UpstreamProvider` struct and its factory functions move here. The key change: `FetchClaims` returns `*identity.FederatedClaims` directly -- no intermediate `UpstreamClaims` type, no redundant mapping step.

```go
// internal/authn/provider.go
package authn

import (
    "context"
    "fmt"

    "golang.org/x/oauth2"

    "github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
)

// Provider represents an upstream identity provider (e.g., Google, GitHub).
type Provider struct {
    Name         string
    DisplayName  string
    OAuth2Config *oauth2.Config
    // FetchClaims exchanges a completed OAuth2 token for the user's
    // identity claims. It is provider-specific and returns the claims
    // in the domain's canonical type directly.
    FetchClaims func(ctx context.Context, token *oauth2.Token) (*identity.FederatedClaims, error)
}

// NewProviders builds the list of configured upstream providers from config.
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

`provider_google.go` and `provider_github.go` contain the extracted factory functions from `login/upstream.go`. Each returns `*Provider` with a `FetchClaims` function that maps provider-specific responses directly to `*identity.FederatedClaims`.

### 4.3 HTTP Handler -- `internal/authn/handler.go`

```go
// internal/authn/handler.go
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

// AuthRequestQuerier is the narrow interface the handler needs to interact
// with auth requests. It intentionally exposes only GetByID (for validation)
// and CompleteLogin (to finalize the flow). The full auth request lifecycle
// is owned by the oidcprovider (temporarily) and will move to internal/oidc
// in Iteration 2.
type AuthRequestQuerier interface {
    GetByID(ctx context.Context, id string) (AuthRequestInfo, error)
    CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
}

// AuthRequestInfo is a minimal read-only view of an auth request exposed to authn.
// It contains only what the handler needs, avoiding direct knowledge of model.AuthRequest.
type AuthRequestInfo struct {
    ID string
}

// Handler is the HTTP adapter for the federated login flow.
// It contains zero domain logic -- all identity decisions are delegated
// to identity.Service.
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
1. `userRepo userFinder` replaced by `identitySvc identity.Service` -- handler calls the service, never the repo.
2. `authReqRepo authRequestCompleter` replaced by `AuthRequestQuerier` -- a purposefully narrow interface owned by the authn module.
3. `encryptState`/`decryptState` delegate to `crypto.Encrypt`/`Decrypt`; the handler handles only JSON marshaling.
4. `findOrCreateUser` is gone. `FederatedCallback` calls `identitySvc.FindOrCreateByFederatedLogin` directly.

The `FederatedCallback` method:

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

    // FetchClaims returns *identity.FederatedClaims directly. No mapping step.
    claims, err := provider.FetchClaims(r.Context(), token)
    if err != nil {
        h.logger.Error("user info retrieval failed", "provider", state.Provider, "error", err)
        http.Error(w, "authentication failed", http.StatusInternalServerError)
        return
    }

    // Delegate entirely to identity service. The handler has no knowledge of
    // user creation, claims storage, or what "finding" a user means.
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

### 4.4 State Encryption

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

The schema for `users` and `federated_identities` is reengineered to match the new domain model. The change is substantive:

- **`users`** is stripped to a minimal anchor: just `id` and `created_at`. All profile data moves out.
- **`federated_identities`** grows to carry the full set of IdP-provided profile claims, `last_login_at`, and `updated_at` for tracking claim freshness.

### 5.1 Decision: ULID Primary Keys

New tables use application-generated **ULIDs** (Universally Unique Lexicographically Sortable Identifiers) instead of database-generated UUIDs. ULIDs are:
- **Time-sortable:** The 48-bit timestamp prefix means rows are naturally ordered by insertion time in index scans.
- **Application-generated:** No round-trip to the database is needed to obtain an ID before insertion.
- **URL-safe:** 26-character Crockford Base32, no hyphens.
- **Stored as `TEXT`** in PostgreSQL (no native ULID type needed; the text representation is 26 chars).

The legacy tables (`auth_requests`, `clients`, `tokens`, `refresh_tokens`) keep their existing UUID PK types for now and are untouched until Iteration 2. However, their foreign-key columns that reference `users.id` must change type from `UUID` to `TEXT` to match the new PK type. The existing UUID string values are valid `TEXT` values, so this is purely mechanical.

The Go dependency to add: `github.com/oklog/ulid/v2`.

### 5.2 New `001_initial.sql`

```sql
-- Identity domain: the stable user anchor.
-- No profile data lives here -- all IdP-provided claims are on federated_identities.
CREATE TABLE users (
    id          TEXT PRIMARY KEY,                   -- application-generated ULID
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Identity domain: one record per (user, upstream provider) pair.
-- Carries all profile claims as last reported by that provider.
-- Refreshed on every successful federated login.
CREATE TABLE federated_identities (
    id               TEXT PRIMARY KEY,              -- application-generated ULID
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL,                 -- e.g. "google", "github"
    provider_subject TEXT NOT NULL,                 -- provider's own user ID

    -- Profile claims sourced from this provider. Refreshed on every login.
    email            TEXT        NOT NULL DEFAULT '',
    email_verified   BOOLEAN     NOT NULL DEFAULT false,
    display_name     TEXT        NOT NULL DEFAULT '',
    given_name       TEXT        NOT NULL DEFAULT '',
    family_name      TEXT        NOT NULL DEFAULT '',
    picture_url      TEXT        NOT NULL DEFAULT '',

    last_login_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(provider, provider_subject)
);

CREATE INDEX federated_identities_user_id_last_login_idx
    ON federated_identities (user_id, last_login_at DESC);

-- OIDC domain: registered relying-party clients.
-- Unchanged from previous schema.
CREATE TABLE clients (
    id                          TEXT PRIMARY KEY,
    secret_hash                 TEXT        NOT NULL DEFAULT '',
    redirect_uris               TEXT[]      NOT NULL,
    post_logout_redirect_uris   TEXT[]      NOT NULL DEFAULT '{}',
    application_type            TEXT        NOT NULL DEFAULT 'web',
    auth_method                 TEXT        NOT NULL DEFAULT 'client_secret_basic',
    response_types              TEXT[]      NOT NULL,
    grant_types                 TEXT[]      NOT NULL,
    access_token_type           TEXT        NOT NULL DEFAULT 'jwt',
    id_token_lifetime_seconds   INTEGER     NOT NULL DEFAULT 3600,
    clock_skew_seconds          INTEGER     NOT NULL DEFAULT 0,
    id_token_userinfo_assertion BOOLEAN     NOT NULL DEFAULT false,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- OIDC domain: in-flight authorization requests.
-- user_id changed from UUID to TEXT to match the new users.id type.
CREATE TABLE auth_requests (
    id                    TEXT        PRIMARY KEY,  -- kept as text (was UUID)
    client_id             TEXT        NOT NULL,
    redirect_uri          TEXT        NOT NULL,
    state                 TEXT,
    nonce                 TEXT,
    scopes                TEXT[],
    response_type         TEXT        NOT NULL,
    response_mode         TEXT,
    code_challenge        TEXT,
    code_challenge_method TEXT,
    prompt                TEXT[],
    max_age               INTEGER,
    login_hint            TEXT,
    user_id               TEXT,                     -- FK type changed UUID -> TEXT
    auth_time             TIMESTAMPTZ,
    amr                   TEXT[],
    is_done               BOOLEAN     NOT NULL DEFAULT false,
    code                  TEXT,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX auth_requests_code_idx ON auth_requests (code) WHERE code IS NOT NULL;

-- OIDC domain: issued access tokens.
-- Unchanged from previous schema (subject is a string, not a FK).
CREATE TABLE tokens (
    id               TEXT        PRIMARY KEY,       -- kept as text (was UUID)
    client_id        TEXT        NOT NULL,
    subject          TEXT        NOT NULL,
    audience         TEXT[],
    scopes           TEXT[],
    expiration       TIMESTAMPTZ NOT NULL,
    refresh_token_id TEXT,                          -- FK type changed UUID -> TEXT
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- OIDC domain: refresh tokens.
-- user_id changed from UUID to TEXT to match the new users.id type.
CREATE TABLE refresh_tokens (
    id               TEXT        PRIMARY KEY,       -- kept as text (was UUID)
    token            TEXT        NOT NULL UNIQUE,
    client_id        TEXT        NOT NULL,
    user_id          TEXT        NOT NULL REFERENCES users(id),  -- type changed UUID -> TEXT
    audience         TEXT[],
    scopes           TEXT[],
    auth_time        TIMESTAMPTZ NOT NULL,
    amr              TEXT[],
    access_token_id  TEXT,                          -- type changed UUID -> TEXT
    expiration       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### 5.3 Schema Change Summary

| Table | Column | Old type | New type | Reason |
|---|---|---|---|---|
| `users` | `id` | `UUID DEFAULT gen_random_uuid()` | `TEXT` (ULID) | Application-generated ULID |
| `users` | `email` … `updated_at` | All present | **Removed** | Profile data moved to `federated_identities` |
| `federated_identities` | `id` | `UUID` | `TEXT` (ULID) | Application-generated ULID |
| `federated_identities` | `user_id` | `UUID` | `TEXT` | Matches new `users.id` type |
| `federated_identities` | `email`, `email_verified`, etc. | Not present | **Added** | Profile claims now owned here |
| `federated_identities` | `last_login_at`, `updated_at` | Not present | **Added** | Refresh tracking |
| `auth_requests` | `id` | `UUID` | `TEXT` | Consistency (stored as text anyway) |
| `auth_requests` | `user_id` | `UUID` | `TEXT` | Matches new `users.id` type |
| `tokens` | `id`, `refresh_token_id` | `UUID` | `TEXT` | Consistency |
| `refresh_tokens` | `id`, `access_token_id`, `user_id` | `UUID` | `TEXT` | Matches new `users.id` type |

### 5.4 Impact on Legacy Repos (Iteration 1, in-scope)

`repo/authrequest.go` and `repo/token.go` treat `user_id` as a `*string` / `string` in their scan targets and bind parameters, so the `UUID -> TEXT` type change requires **no code changes** in those files. The schema changes are purely at the DDL level.

---

## 6. Bridging the Legacy oidcprovider

The `oidcprovider.Storage` god-object is left structurally intact in this iteration, but its user-access dependency is replaced with `identity.Service`.

### 6.1 Replace `oidcprovider.UserReader` with `identity.Service`

The current `UserReader` interface in `oidcprovider/storage.go` is deleted. `Storage` instead depends on `identity.Service`:

```go
// NEW -- in oidcprovider/storage.go
import (
    "github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
    "github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

type Storage struct {
    db                   *sqlx.DB
    identitySvc          identity.Service   // replaces the old UserReader interface
    clientRepo           ClientReader
    authReqRepo          AuthRequestStore
    tokenRepo            TokenStore
    signing              *SigningKeyWithID
    public               *PublicKeyWithID
    accessTokenLifetime  time.Duration
    refreshTokenLifetime time.Duration
}
```

### 6.2 Updated `setUserinfo` and `setIntrospectionUserinfo`

Because profile data no longer lives on `identity.User`, the OIDC bridge must call `GetUserWithClaims` to obtain a view with the profile fields. The `UserWithClaims` read model was designed precisely for this use case.

```go
func (s *Storage) setUserinfo(ctx context.Context, userinfo *oidc.UserInfo, userID string, scopes []string) error {
    userClaims, err := s.identitySvc.GetUserWithClaims(ctx, userID)
    if err != nil {
        if domerr.Is(err, domerr.ErrNotFound) {
            return oidc.ErrInvalidRequest().WithDescription("user not found")
        }
        return err
    }

    for _, scope := range scopes {
        switch scope {
        case oidc.ScopeOpenID:
            userinfo.Subject = userClaims.UserID
        case oidc.ScopeProfile:
            userinfo.Name = userClaims.Name
            userinfo.GivenName = userClaims.GivenName
            userinfo.FamilyName = userClaims.FamilyName
            userinfo.Picture = userClaims.Picture
            // UpdatedAt reflects when the IdP last reported a change to
            // the user's profile -- semantically correct for this claim.
            userinfo.UpdatedAt = oidc.FromTime(userClaims.ClaimsUpdatedAt)
        case oidc.ScopeEmail:
            userinfo.Email = userClaims.Email
            userinfo.EmailVerified = oidc.Bool(userClaims.EmailVerified)
        }
    }
    return nil
}

func (s *Storage) setIntrospectionUserinfo(introspection *oidc.IntrospectionResponse, userClaims *identity.UserWithClaims, scopes []string) {
    for _, scope := range scopes {
        switch scope {
        case oidc.ScopeOpenID:
            introspection.Subject = userClaims.UserID
        case oidc.ScopeProfile:
            introspection.Name = userClaims.Name
            introspection.GivenName = userClaims.GivenName
            introspection.FamilyName = userClaims.FamilyName
            introspection.Picture = userClaims.Picture
            introspection.UpdatedAt = oidc.FromTime(userClaims.ClaimsUpdatedAt)
        case oidc.ScopeEmail:
            introspection.Email = userClaims.Email
            introspection.EmailVerified = oidc.Bool(userClaims.EmailVerified)
        }
    }
}
```

`SetIntrospectionFromToken` is updated to call `GetUserWithClaims` first, then pass the result to `setIntrospectionUserinfo`:

```go
func (s *Storage) SetIntrospectionFromToken(ctx context.Context, introspection *oidc.IntrospectionResponse, tokenID, subject, clientID string) error {
    token, err := s.tokenRepo.GetByID(ctx, tokenID)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            introspection.Active = false
            return nil
        }
        return err
    }

    introspection.Active = true
    introspection.Subject = token.Subject
    introspection.ClientID = token.ClientID
    introspection.Scope = oidc.SpaceDelimitedArray(token.Scopes)
    introspection.Audience = token.Audience
    introspection.Expiration = oidc.FromTime(token.Expiration)
    introspection.IssuedAt = oidc.FromTime(token.CreatedAt)
    introspection.TokenType = oidc.BearerToken

    userClaims, err := s.identitySvc.GetUserWithClaims(ctx, token.Subject)
    if err != nil && !domerr.Is(err, domerr.ErrNotFound) {
        return err
    }
    if userClaims != nil {
        s.setIntrospectionUserinfo(introspection, userClaims, token.Scopes)
    }
    return nil
}
```

**This is a clean cut:** The `oidcprovider` package no longer imports `model.User`, `model.FederatedIdentity`, or any user repo. Its only user-related import is `identity.Service` and `identity.UserWithClaims`.

### 6.3 AuthRequestQuerier Adapter for authn

`authn.Handler` needs `GetByID` and `CompleteLogin` without importing `model.AuthRequest`. A thin adapter in `main.go` bridges `repo.AuthRequestRepository` to `authn.AuthRequestQuerier`:

```go
// In main.go -- purely wiring glue; deleted in Iteration 2.
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

---

## 7. Migration of main.go Wiring

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

    // --- OIDC provider (storage adapted to use identity.Service) ---
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

    authReqBridge := &authReqAdapter{repo: authReqRepo} // temporary bridge, removed in Iteration 2

    loginHandler := authn.NewHandler(
        upstreamProviders,
        identitySvc,
        authReqBridge,
        cfg.CryptoKey,
        op.AuthCallbackURL(provider),
        logger,
    )

    // Routing (structure unchanged from current main.go)
    router := chi.NewRouter()
    router.Use(middleware.Recoverer)

    interceptor := op.NewIssuerInterceptor(provider.IssuerFromRequest)
    router.Route("/login", func(r chi.Router) {
        r.Use(interceptor.Handler)
        r.Get("/", loginHandler.SelectProvider)
        r.Post("/select", loginHandler.FederatedRedirect)
        r.Get("/callback", loginHandler.FederatedCallback)
    })

    router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
    router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
        if err := db.PingContext(r.Context()); err != nil {
            http.Error(w, "not ready", http.StatusServiceUnavailable)
            return
        }
        w.WriteHeader(http.StatusOK)
    })
    router.Get("/logged-out", func(w http.ResponseWriter, _ *http.Request) {
        _, _ = w.Write([]byte("You have been signed out."))
    })

    router.Mount("/", provider)

    srv := &http.Server{ /* same timeouts as current */ }
    // ... graceful shutdown identical to current main.go ...
}
```

### Import table

| Package alias | Import path | Status |
|---|---|---|
| `identity` | `.../internal/identity` | New |
| `identitypg` | `.../internal/identity/postgres` | New |
| `authn` | `.../internal/authn` | New |
| `repo` | `.../repo` | Unchanged (auth, client, token only) |
| `oidcprovider` | `.../oidcprovider` | Adapted (new Storage constructor) |
| `config` | `.../config` | Unchanged |

---

## 8. Testing Strategy

### 8.1 Unit Tests (no database)

| Package | Test File | What's tested |
|---|---|---|
| `internal/pkg/domerr` | `errors_test.go` | `errors.Is` chaining with wrapped domain errors |
| `internal/pkg/crypto` | `aes_test.go` | Encrypt/Decrypt round-trip, wrong key, short ciphertext, invalid base64 |
| `internal/identity` | `service_test.go` | All three service methods with a mock `Repository`: existing user path (confirm claims update is called), new user path, error propagation, `GetUserWithClaims` delegation |
| `internal/authn` | `handler_test.go` | All handler tests from `login/handler_test.go`, adapted to inject mock `identity.Service` and mock `AuthRequestQuerier` |

### 8.2 Integration Tests (testcontainers PostgreSQL)

| Package | Test File | What's tested |
|---|---|---|
| `internal/identity/postgres` | `user_repo_test.go` | Full repo contract: `GetByID` (found / not-found → `domerr.ErrNotFound`), `FindByFederatedIdentity` (found / not-found → nil,nil), `CreateWithFederatedIdentity`, `UpdateFederatedIdentityClaims` (claims refresh verified by subsequent `GetUserWithClaims`), `GetUserWithClaims` (most-recent FI selected when multiple exist) |
| `oidcprovider` | `storage_test.go` | Adapted to construct the service chain: `identity.NewService(identitypg.NewUserRepository(db))` injected as `identitySvc` |

### 8.3 Deleted / Moved Tests

- `repo/repo_test.go`: All `TestUserRepository_*` tests move to `internal/identity/postgres/user_repo_test.go`.
- `login/handler_test.go`: Moves entirely to `internal/authn/handler_test.go`, adapted to mock `identity.Service`.

### 8.4 testhelper/testdb.go Update

`CleanTables` has a hardcoded table list. The new column layout doesn't change the table names, but the order must respect the new FK relationships (no change needed -- `federated_identities` already comes before `users` in the delete order). No functional change required. The `updated_at` trigger on `users` is removed since the table no longer has that column; no trigger update needed.

---

## 9. Future Roadmap

### Iteration 2: internal/oidc Domain

**Scope:** Extract everything the zitadel `op.Storage` interface needs into `internal/oidc/`, completing the dismantling of the god-object.

#### 9.1 AuthRequest Domain

- Create `internal/oidc/domain.go` with `AuthRequest` as a proper domain type (no `db:` tags).
- Move `repo/authrequest.go` into `internal/oidc/postgres/authrequest_repo.go`.
- Create `internal/oidc/authrequest_svc.go` implementing an `AuthRequestService`.
- Extract the hardcoded 30-minute TTL from `repo/authrequest.go`'s `activeFilter` SQL into a configurable domain constant on the service, not in SQL.
- The `authn.AuthRequestQuerier` interface will be satisfied by `oidc.AuthRequestService` directly. The `authReqAdapter` bridge in `main.go` is deleted.
- Move `oidcprovider/authrequest.go` type adapter into `internal/oidc/adapter/authrequest.go`.

#### 9.2 Client Domain

- Create `model.Client` equivalent in `internal/oidc/domain.go`.
- Move `repo/client.go` into `internal/oidc/postgres/client_repo.go`.
- Create `internal/oidc/client_svc.go`. Bcrypt client-secret verification moves from `oidcprovider.Storage.AuthorizeClientIDSecret` into `ClientService.AuthorizeSecret`.
- Move the string-to-enum conversion logic (ApplicationType, AuthMethod, etc.) from `oidcprovider/client.go` into a mapper in the domain layer.
- Move `oidcprovider/client.go` adapter into `internal/oidc/adapter/client.go`.

#### 9.3 Token Domain

- Create `Token` and `RefreshToken` types in `internal/oidc/domain.go`.
- Move `repo/token.go` into `internal/oidc/postgres/token_repo.go`.
- Create `internal/oidc/token_svc.go` owning creation, refresh, revocation, introspection.
- Move `oidcprovider/refreshtoken.go` adapter into `internal/oidc/adapter/refreshtoken.go`.

#### 9.4 Dismantling the Storage God-Object

Once all three domain services exist, `oidcprovider/storage.go` becomes a pure **compositor**:

```go
// internal/oidc/adapter/storage.go
type StorageAdapter struct {
    identity identity.Service
    authreq  AuthRequestService
    client   ClientService
    token    TokenService
    keys     KeyProvider
}
```

Every `op.Storage` method becomes a 1-3 line delegation to the appropriate service. The 400-line `storage.go` becomes ~120 lines of pure plumbing.

#### 9.5 Key Management

- Move `oidcprovider/keys.go` to `internal/oidc/adapter/keys.go` or `internal/pkg/crypto/`.
- Add database-backed key rotation: a `signing_keys` table, `KeySet()` returns all non-expired keys, new key activated by config or a management endpoint.

### Iteration 3: Cleanup and Polish

- Delete the now-empty `model/` and `repo/` packages.
- Remove the last legacy imports from `main.go`.
- Unify the testcontainers setup across packages (single shared `TestMain` or build-tag gating).
- Add OpenTelemetry tracing spans to key operations (identity service methods, repo calls, upstream provider calls).
- Add a request-ID logging middleware.
- Consider CSRF token protection on the provider selection form (issue: the encrypted state parameter on the callback protects the authentication completion, but the initial provider selection POST is unguarded).
- Harden the GitHub provider to fetch primary/verified email from `/user/emails` endpoint when `/user` returns an empty email.

### Target End-State Directory Tree (after all iterations)

```
services/accounts/
  main.go
  Dockerfile
  .env.example
  internal/
    pkg/
      domerr/
        errors.go
      crypto/
        aes.go
    identity/
      domain.go         (User, FederatedIdentity, FederatedClaims, UserWithClaims)
      ports.go          (Repository, Service interfaces)
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
      ports.go          (AuthRequestService, ClientService, TokenService + their repo interfaces)
      authrequest_svc.go
      client_svc.go
      token_svc.go
      adapter/
        storage.go      (thin op.Storage compositor delegating to domain services)
        authrequest.go  (op.AuthRequest adapter wrapping oidc.AuthRequest)
        client.go       (op.Client adapter wrapping oidc.Client)
        refreshtoken.go (op.RefreshTokenRequest adapter)
        keys.go         (key management)
        provider.go     (OIDC provider construction)
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

Each step must produce a compilable, test-passing codebase before the next begins.

### Step 1: Add ULID dependency
- Add `github.com/oklog/ulid/v2` to `go.mod`: `go get github.com/oklog/ulid/v2`.
- No code changes yet.

### Step 2: Reengineer the database schema
- Rewrite `migrations/001_initial.sql` to the new schema from Section 5.
- Update `testhelper/testdb.go` `CleanTables` if table names changed (they haven't, but verify).
- Verify the migration runs cleanly against a fresh PostgreSQL instance.

### Step 3: Create `internal/pkg/domerr`
- Create `internal/pkg/domerr/errors.go` with sentinel errors.
- Create `internal/pkg/domerr/errors_test.go`.
- No existing code changes.

### Step 4: Create `internal/pkg/crypto`
- Create `internal/pkg/crypto/aes.go` and `aes_test.go`.
- Standalone package, no existing code changes.
- Verify tests pass.

### Step 5: Create `internal/identity` domain + service
- Create `internal/identity/domain.go` (User, FederatedIdentity, FederatedClaims, UserWithClaims).
- Create `internal/identity/ports.go` (Repository and Service interfaces).
- Create `internal/identity/service.go` (identityService with updated FindOrCreate logic including claims refresh).
- Create `internal/identity/service_test.go` with a mock Repository, covering: existing user path verifies `UpdateFederatedIdentityClaims` is called; new user path; `GetUserWithClaims` delegation; error propagation.

### Step 6: Create `internal/identity/postgres` adapter
- Create `internal/identity/postgres/user_repo.go` implementing the new `Repository` interface against the reengineered schema.
- Create `internal/identity/postgres/user_repo_test.go` covering all five repository methods against a real PostgreSQL testcontainer.
- Verify integration tests pass.

### Step 7: Create `internal/authn`
- Create `internal/authn/config.go`, `provider.go`, `provider_google.go`, `provider_github.go`.
- Create `internal/authn/handler.go` with the reengineered Handler.
- Create `internal/authn/handler_test.go` (ported and adapted from `login/handler_test.go`).
- No existing files are touched in this step.

### Step 8: Adapt `oidcprovider/storage.go` to use `identity.Service`
- Delete the `UserReader` interface declaration.
- Replace the `userRepo UserReader` field with `identitySvc identity.Service` on `Storage`.
- Update `NewStorage` constructor signature.
- Rewrite `setUserinfo` and `setIntrospectionUserinfo` to call `GetUserWithClaims`.
- Rewrite `SetIntrospectionFromToken` to use the new call pattern.
- Update `oidcprovider/storage_test.go` to construct the full service chain.

### Step 9: Rewire `main.go`
- Update imports: add `internal/identity`, `internal/identity/postgres`, `internal/authn`.
- Add `authReqAdapter` bridge struct.
- Replace `repo.NewUserRepository` + `login.NewHandler`/`login.NewUpstreamProviders` with new module constructors.
- Remove imports of `login` and the user-repo parts of `repo`.

### Step 10: Delete obsolete code
- Delete `login/` directory.
- Delete `model/user.go` and `model/federated_identity.go`.
- Remove all `TestUserRepository_*` tests from `repo/repo_test.go`.
- Delete `repo/user.go`.
- Verify `model/` contains only `authrequest.go`, `client.go`, `token.go`.
- Verify `repo/` contains only `authrequest.go`, `client.go`, `token.go`, `repo_test.go`.

### Step 11: Final verification
- Run full test suite: `go test ./services/accounts/...`
- Run linter: `golangci-lint run ./services/accounts/...`
- Verify no import cycles: `go build ./services/accounts/...`
- Confirm `login/` is gone and `model/user.go`, `model/federated_identity.go` are gone.
- Confirm `oidcprovider/storage.go` has no import of `model.User` or `model.FederatedIdentity`.
