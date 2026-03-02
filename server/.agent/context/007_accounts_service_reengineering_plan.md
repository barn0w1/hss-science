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
      domain.go                   (new -- reengineered domain types: User, FederatedIdentity)
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
- **The `users` and `federated_identities` tables are fully reengineered** with a new schema reflecting a strict 1:1 relationship and correct ownership of profile data (see Section 5).
- **Strict 1:1 User↔FederatedIdentity relationship**. The system does not support linking multiple providers to one user in this iteration. One user, one provider link.
- **`User` is the stable canonical identity** -- it holds all profile fields set at creation from IdP claims and is never overwritten. This makes it the single source of truth for profile data within our system.
- **`FederatedIdentity` is the live sync record** -- it mirrors the raw upstream provider claims and is updated on every successful login, acting as a real-time snapshot of what the IdP currently reports.

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

This is the core of Iteration 1. The reengineering insight is that there are two distinct concerns:

1. **Canonical system identity** (`User`) -- what *we* know about a person in our system. Stable,  authoritative, and simple. Profile data is recorded from the first login and kept as-is.
2. **Live upstream snapshot** (`FederatedIdentity`) -- what the IdP *currently* reports for this person. Refreshed on every login to track changes the user may have made upstream (email verification, avatar change, etc.).

These are complementary, not competing. The `User` is what downstream systems (OIDC tokens, auth requests) reference. The `FederatedIdentity` is an operational record for auditing and upstream sync. Keeping them separate, with a strict 1:1 relationship for simplicity, gives us the best of both: a stable identity we can rely on, and a fresh upstream snapshot we can inspect.

### 3.1 Domain Types -- `internal/identity/domain.go`

```go
// internal/identity/domain.go
package identity

import "time"

// User is the canonical system identity for a person who has authenticated
// at least once. Profile fields (Email, Name, etc.) are populated at first
// login from the upstream IdP claims and are NEVER overwritten afterward.
// This ensures our system has a stable, consistent identity that does not
// drift with every upstream change.
//
// A User has exactly one FederatedIdentity (strict 1:1 relationship).
// Multi-provider linking is not supported in this iteration.
type User struct {
    ID            string    // ULID, application-generated
    Email         string    // set at creation from IdP claims; stable
    EmailVerified bool      // set at creation; stable
    Name          string    // full display name; set at creation; stable
    GivenName     string    // set at creation; stable
    FamilyName    string    // set at creation; stable
    Picture       string    // set at creation; stable
    CreatedAt     time.Time
}

// FederatedIdentity links a User to a specific upstream identity provider
// and acts as a live mirror of the upstream claims. Its ProviderXxx fields
// are updated on EVERY successful login to reflect what the IdP currently
// reports. This allows the system to track upstream state (e.g., whether
// the user has verified their email with Google) without destabilising the
// canonical User record.
type FederatedIdentity struct {
    ID              string    // ULID, application-generated
    UserID          string    // UNIQUE FK -- enforces 1:1 with User
    Provider        string    // e.g. "google", "github"
    ProviderSubject string    // the upstream provider's own user ID

    // Raw upstream claims -- refreshed on every successful login.
    ProviderEmail         string
    ProviderEmailVerified bool
    ProviderDisplayName   string // maps to OIDC standard "name" claim
    ProviderGivenName     string
    ProviderFamilyName    string
    ProviderPictureURL    string

    LastLoginAt time.Time // updated on every successful federated login
    CreatedAt   time.Time
    UpdatedAt   time.Time // updated whenever provider claims are refreshed
}

// FederatedClaims is the set of claims obtained from an upstream identity
// provider during federated authentication. It is transient -- NOT persisted
// directly. It is used as input to the FindOrCreateByFederatedLogin use case,
// which maps the claims onto a User (first login) or a FederatedIdentity
// update (subsequent logins).
type FederatedClaims struct {
    Subject       string
    Email         string
    EmailVerified bool
    Name          string // full display name
    GivenName     string
    FamilyName    string
    Picture       string
}
```

**Why `User` holds profile fields (set once, never overwritten):**
- Our system needs a stable identity to reference in OIDC tokens, auth requests, and future features. A user's email/name in our system should not silently change just because they renamed their Google account.
- Profile data is established at the moment the user first authenticates with us -- at that point we have the most up-to-date, user-consented data. We record it once.
- If a user's profile needs to be updated in our system, that becomes an explicit system action (Iteration 3+), not a side-effect of login.

**Why `FederatedIdentity` holds raw provider claims (updated every login):**
- We want to track the current upstream state for operational purposes: has Google now verified the email? Has the user changed their avatar? This data should stay current.
- Prefixing fields with `Provider` makes it unambiguous that these values come from the upstream IdP, not from our system.
- The 1:1 constraint keeps the model simple. Each `User` has exactly one `FederatedIdentity`, eliminating all "which provider wins?" questions.

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

    // CreateWithFederatedIdentity atomically creates a new User (with all
    // profile fields populated from claims) and their initial FederatedIdentity
    // in a single transaction.
    CreateWithFederatedIdentity(ctx context.Context, user *User, fi *FederatedIdentity) error

    // UpdateFederatedIdentityClaims refreshes the raw provider claims on an
    // existing FederatedIdentity record and records the login timestamp.
    // Called on every successful federated login where the user already exists.
    // The User record is NEVER touched by this method.
    UpdateFederatedIdentityClaims(
        ctx context.Context,
        provider, providerSubject string,
        claims FederatedClaims,
        lastLoginAt time.Time,
    ) error
}

// Service defines the application-level use cases for the identity domain.
// It is the "driving port" (primary port) -- the contract that external modules
// (authn, oidcprovider) depend on.
type Service interface {
    // GetUser retrieves a user's full identity (including stable profile fields) by ID.
    // Returns domerr.ErrNotFound if the user does not exist.
    GetUser(ctx context.Context, userID string) (*User, error)

    // FindOrCreateByFederatedLogin is the primary login use case.
    //
    // For a new user: a User is created with profile fields populated from
    // the provided claims, and their initial FederatedIdentity is created
    // atomically. The User's profile is fixed at this point.
    //
    // For an existing user: the FederatedIdentity is updated with the
    // fresh raw claims from the upstream provider. The User record itself
    // is NEVER modified -- it remains a stable anchor.
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
        // User exists. Refresh only the FederatedIdentity record with the latest
        // raw claims from the upstream provider. The User record itself is NEVER
        // modified -- it is the stable canonical identity we established at
        // first login.
        now := time.Now().UTC()
        if err := s.repo.UpdateFederatedIdentityClaims(ctx, provider, claims.Subject, claims, now); err != nil {
            return nil, fmt.Errorf("identity.FindOrCreate: update claims: %w", err)
        }
        return existing, nil
    }

    // New user: create User (with profile fields from claims) + FederatedIdentity
    // (with raw provider claims) atomically.
    now := time.Now().UTC()
    user := &User{
        ID:            newID(),
        Email:         claims.Email,
        EmailVerified: claims.EmailVerified,
        Name:          claims.Name,
        GivenName:     claims.GivenName,
        FamilyName:    claims.FamilyName,
        Picture:       claims.Picture,
        CreatedAt:     now,
    }
    fi := &FederatedIdentity{
        ID:                    newID(),
        UserID:                user.ID,
        Provider:              provider,
        ProviderSubject:       claims.Subject,
        ProviderEmail:         claims.Email,
        ProviderEmailVerified: claims.EmailVerified,
        ProviderDisplayName:   claims.Name,
        ProviderGivenName:     claims.GivenName,
        ProviderFamilyName:    claims.FamilyName,
        ProviderPictureURL:    claims.Picture,
        LastLoginAt:           now,
        CreatedAt:             now,
        UpdatedAt:             now,
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

**Key distinction from the simple "copy-fields" approach:** On first login, profile fields are written to `User` AND mirrored as raw provider claims on `FederatedIdentity`. On every subsequent login, ONLY the `FederatedIdentity` provider claims are updated; the `User` record is left completely untouched. This means `User.Email` is the profile as we recorded it, and `FederatedIdentity.ProviderEmail` is what the IdP says today -- two clearly separated concerns.

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
    ID            string    `db:"id"`
    Email         string    `db:"email"`
    EmailVerified bool      `db:"email_verified"`
    Name          string    `db:"name"`
    GivenName     string    `db:"given_name"`
    FamilyName    string    `db:"family_name"`
    Picture       string    `db:"picture"`
    CreatedAt     time.Time `db:"created_at"`
}

type federatedIdentityRow struct {
    ID                    string    `db:"id"`
    UserID                string    `db:"user_id"`
    Provider              string    `db:"provider"`
    ProviderSubject       string    `db:"provider_subject"`
    ProviderEmail         string    `db:"provider_email"`
    ProviderEmailVerified bool      `db:"provider_email_verified"`
    ProviderDisplayName   string    `db:"provider_display_name"`
    ProviderGivenName     string    `db:"provider_given_name"`
    ProviderFamilyName    string    `db:"provider_family_name"`
    ProviderPictureURL    string    `db:"provider_picture_url"`
    LastLoginAt           time.Time `db:"last_login_at"`
    CreatedAt             time.Time `db:"created_at"`
    UpdatedAt             time.Time `db:"updated_at"`
}

func toUser(row userRow) *identity.User {
    return &identity.User{
        ID:            row.ID,
        Email:         row.Email,
        EmailVerified: row.EmailVerified,
        Name:          row.Name,
        GivenName:     row.GivenName,
        FamilyName:    row.FamilyName,
        Picture:       row.Picture,
        CreatedAt:     row.CreatedAt,
    }
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*identity.User, error) {
    var row userRow
    err := r.db.QueryRowxContext(ctx,
        `SELECT id, email, email_verified, name, given_name, family_name, picture, created_at
         FROM users WHERE id = $1`, id,
    ).StructScan(&row)
    if errors.Is(err, sql.ErrNoRows) {
        return nil, domerr.ErrNotFound
    }
    if err != nil {
        return nil, err
    }
    return toUser(row), nil
}

func (r *UserRepository) FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*identity.User, error) {
    var row userRow
    err := r.db.QueryRowxContext(ctx,
        `SELECT u.id, u.email, u.email_verified, u.name, u.given_name, u.family_name, u.picture, u.created_at
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
    return toUser(row), nil
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
        `INSERT INTO users (id, email, email_verified, name, given_name, family_name, picture, created_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
        user.ID, user.Email, user.EmailVerified, user.Name,
        user.GivenName, user.FamilyName, user.Picture, user.CreatedAt,
    )
    if err != nil {
        return err
    }

    _, err = tx.ExecContext(ctx,
        `INSERT INTO federated_identities
            (id, user_id, provider, provider_subject,
             provider_email, provider_email_verified,
             provider_display_name, provider_given_name, provider_family_name, provider_picture_url,
             last_login_at, created_at, updated_at)
         VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
        fi.ID, fi.UserID, fi.Provider, fi.ProviderSubject,
        fi.ProviderEmail, fi.ProviderEmailVerified,
        fi.ProviderDisplayName, fi.ProviderGivenName, fi.ProviderFamilyName, fi.ProviderPictureURL,
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
         SET provider_email           = $1,
             provider_email_verified  = $2,
             provider_display_name    = $3,
             provider_given_name      = $4,
             provider_family_name     = $5,
             provider_picture_url     = $6,
             last_login_at            = $7,
             updated_at               = now()
         WHERE provider = $8 AND provider_subject = $9`,
        claims.Email, claims.EmailVerified, claims.Name,
        claims.GivenName, claims.FamilyName, claims.Picture,
        lastLoginAt, provider, providerSubject,
    )
    return err
}
```

**Key differences from the old `repo/user.go`:**
- `userRow` now carries all profile fields (`email`, `email_verified`, `name`, etc.) -- these are stable system-owned values.
- `federatedIdentityRow` uses `provider_` prefixed column names, making it unambiguous that these are raw upstream values.
- `UpdateFederatedIdentityClaims` updates ONLY the `federated_identities` table -- the `users` table is never touched after creation.
- No `GetUserWithClaims` method -- it is not needed because all profile data lives directly on `User`.
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

The schema for `users` and `federated_identities` is reengineered to match the new domain model:

- **`users`** carries all canonical profile fields alongside `id` and `created_at`. These are populated at creation and never updated. No `updated_at` column -- the profile is immutable by design.
- **`federated_identities`** stores the raw upstream provider claims using `provider_`-prefixed column names, making it unambiguous that these values come from the IdP, not our system. A `UNIQUE(user_id)` constraint enforces the strict 1:1 relationship at the database level.

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
-- Identity domain: the canonical user identity.
-- Profile fields are set at first login from IdP claims and never updated.
-- No updated_at column -- the record is immutable after INSERT.
CREATE TABLE users (
    id             TEXT PRIMARY KEY,                   -- application-generated ULID
    email          TEXT        NOT NULL DEFAULT '',
    email_verified BOOLEAN     NOT NULL DEFAULT false,
    name           TEXT        NOT NULL DEFAULT '',
    given_name     TEXT        NOT NULL DEFAULT '',
    family_name    TEXT        NOT NULL DEFAULT '',
    picture        TEXT        NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Identity domain: live upstream snapshot, one record per user (1:1).
-- provider_* columns mirror the raw claims as last reported by the IdP.
-- Refreshed on every successful federated login.
-- UNIQUE(user_id) enforces the strict 1:1 relationship with users.
CREATE TABLE federated_identities (
    id               TEXT PRIMARY KEY,              -- application-generated ULID
    user_id          TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider         TEXT NOT NULL,                 -- e.g. "google", "github"
    provider_subject TEXT NOT NULL,                 -- provider's own user ID

    -- Raw upstream claims sourced from this provider. Refreshed on every login.
    provider_email           TEXT    NOT NULL DEFAULT '',
    provider_email_verified  BOOLEAN NOT NULL DEFAULT false,
    provider_display_name    TEXT    NOT NULL DEFAULT '',
    provider_given_name      TEXT    NOT NULL DEFAULT '',
    provider_family_name     TEXT    NOT NULL DEFAULT '',
    provider_picture_url     TEXT    NOT NULL DEFAULT '',

    last_login_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE(provider, provider_subject),
    UNIQUE(user_id)  -- enforces strict 1:1 relationship with users
);

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
| `users` | `email`, `email_verified`, `name`, `given_name`, `family_name`, `picture` | Present (mutable) | Present (immutable at creation) | Stable canonical profile owned by our system |
| `users` | `updated_at` | Present | **Removed** | Profile never changes; no update tracking needed |
| `federated_identities` | `id` | `UUID` | `TEXT` (ULID) | Application-generated ULID |
| `federated_identities` | `user_id` | `UUID` | `TEXT` | Matches new `users.id` type |
| `federated_identities` | (no profile cols) | Not present | `provider_email`, `provider_email_verified`, `provider_display_name`, `provider_given_name`, `provider_family_name`, `provider_picture_url` | Raw upstream claims stored with clear `provider_` prefix |
| `federated_identities` | `last_login_at`, `updated_at` | Not present | **Added** | Login and refresh tracking |
| `federated_identities` | UNIQUE(user_id) | Not present | **Added** | Enforces 1:1 relationship at DB level |
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

Because `User` now carries all profile fields directly, the OIDC bridge simply calls `GetUser` and reads from the returned struct. No join, no read model, no aggregation -- just a direct lookup.

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
            // Profile is immutable after creation; updated_at == created_at is
            // semantically accurate and avoids a misleading "last changed" claim.
            userinfo.UpdatedAt = oidc.FromTime(user.CreatedAt)
        case oidc.ScopeEmail:
            userinfo.Email = user.Email
            userinfo.EmailVerified = oidc.Bool(user.EmailVerified)
        }
    }
    return nil
}

func (s *Storage) setIntrospectionUserinfo(introspection *oidc.IntrospectionResponse, user *identity.User, scopes []string) {
    for _, scope := range scopes {
        switch scope {
        case oidc.ScopeOpenID:
            introspection.Subject = user.ID
        case oidc.ScopeProfile:
            introspection.Name = user.Name
            introspection.GivenName = user.GivenName
            introspection.FamilyName = user.FamilyName
            introspection.Picture = user.Picture
            introspection.UpdatedAt = oidc.FromTime(user.CreatedAt)
        case oidc.ScopeEmail:
            introspection.Email = user.Email
            introspection.EmailVerified = oidc.Bool(user.EmailVerified)
        }
    }
}
```

`SetIntrospectionFromToken` is updated to call `GetUser` first, then pass the result to `setIntrospectionUserinfo`:

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

    user, err := s.identitySvc.GetUser(ctx, token.Subject)
    if err != nil && !domerr.Is(err, domerr.ErrNotFound) {
        return err
    }
    if user != nil {
        s.setIntrospectionUserinfo(introspection, user, token.Scopes)
    }
    return nil
}
```

**This is a clean cut:** The `oidcprovider` package no longer imports `model.User`, `model.FederatedIdentity`, or any user repo. Its only user-related import is `identity.Service` and `identity.User`. The OIDC bridge is noticeably simpler than before -- one `GetUser` call, read fields directly.

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
| `internal/identity` | `service_test.go` | All service methods with a mock `Repository`: (1) existing user path -- verifies `UpdateFederatedIdentityClaims` IS called AND `User` record is NOT modified; (2) new user path -- verifies `User` is populated with claims fields and FI gets `Provider`-prefixed fields; (3) `GetUser` delegation; (4) error propagation |
| `internal/authn` | `handler_test.go` | All handler tests from `login/handler_test.go`, adapted to inject mock `identity.Service` and mock `AuthRequestQuerier` |

### 8.2 Integration Tests (testcontainers PostgreSQL)

| Package | Test File | What's tested |
|---|---|---|
| `internal/identity/postgres` | `user_repo_test.go` | Full repo contract: `GetByID` (found with all profile fields / not-found → `domerr.ErrNotFound`), `FindByFederatedIdentity` (found / not-found → nil,nil), `CreateWithFederatedIdentity` (verifies User profile cols AND FI `provider_*` cols are written), `UpdateFederatedIdentityClaims` (verifies `provider_*` cols change AND `users` row is unchanged), UNIQUE(user_id) constraint prevents a second FI for the same user |
| `oidcprovider` | `storage_test.go` | Adapted to construct the service chain: `identity.NewService(identitypg.NewUserRepository(db))` injected as `identitySvc` |

### 8.3 Deleted / Moved Tests

- `repo/repo_test.go`: All `TestUserRepository_*` tests move to `internal/identity/postgres/user_repo_test.go`.
- `login/handler_test.go`: Moves entirely to `internal/authn/handler_test.go`, adapted to mock `identity.Service`.

### 8.4 testhelper/testdb.go Update

`CleanTables` has a hardcoded table list. The new column layout doesn't change the table names, but the FK `UNIQUE(user_id)` on `federated_identities` means deletes must still clear `federated_identities` before `users` (which is already the case). No functional change required. The `updated_at` trigger on `users` is removed since the table no longer has that column; no trigger update needed.

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
- Consider whether to add a user profile update flow (explicit user action) that updates `User` profile fields and mirrors them back to `FederatedIdentity.ProviderXxx` -- the separation of concerns established in Iteration 1 makes this a clean, explicit feature.

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
      domain.go         (User, FederatedIdentity, FederatedClaims)
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
- Create `internal/identity/domain.go` (User with profile fields, FederatedIdentity with `Provider`-prefixed fields, FederatedClaims).
- Create `internal/identity/ports.go` (Repository and Service interfaces -- no `GetUserWithClaims`).
- Create `internal/identity/service.go` (identityService with FindOrCreate logic: new user populates User from claims; existing user only updates FI).
- Create `internal/identity/service_test.go` with a mock Repository, covering: (a) existing user path -- `UpdateFederatedIdentityClaims` called, `GetByID`/`CreateWithFederatedIdentity` NOT called; (b) new user path -- `User` populated with claims fields; (c) `GetUser` delegation; (d) error propagation.

### Step 6: Create `internal/identity/postgres` adapter
- Create `internal/identity/postgres/user_repo.go` implementing the new `Repository` interface against the reengineered schema.
- Create `internal/identity/postgres/user_repo_test.go` covering all four repository methods against a real PostgreSQL testcontainer, including a test that `UpdateFederatedIdentityClaims` does NOT change the `users` row and a test that the `UNIQUE(user_id)` constraint prevents duplicate FI records.
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
- Rewrite `setUserinfo` and `setIntrospectionUserinfo` to call `GetUser` and read fields directly from `*identity.User`.
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
- Confirm `oidcprovider/storage.go` calls `identitySvc.GetUser()`, not any `GetUserWithClaims` variant.

---

## 11. Implementation Checklist

- [x] **Step 1:** `go get github.com/oklog/ulid/v2`
- [x] **Step 2:** Rewrite `migrations/001_initial.sql` to new schema
- [x] **Step 3:** Create `internal/pkg/domerr/errors.go` + `errors_test.go`
- [x] **Step 4:** Create `internal/pkg/crypto/aes.go` + `aes_test.go`
- [x] **Step 5:** Create `internal/identity/domain.go`, `ports.go`, `service.go`, `service_test.go`
- [x] **Step 6:** Create `internal/identity/postgres/user_repo.go` + `user_repo_test.go`
- [x] **Step 7:** Create `internal/authn/config.go`, `provider.go`, `provider_google.go`, `provider_github.go`, `handler.go`, `handler_test.go`
- [x] **Step 8:** Adapt `oidcprovider/storage.go` + `storage_test.go` (replace `UserReader` with `identity.Service`)
- [x] **Step 9:** Rewire `main.go` (new imports, `authReqAdapter` bridge, remove old `login`/`repo.UserRepository` usage)
- [x] **Step 10:** Delete `login/`, `model/user.go`, `model/federated_identity.go`, `repo/user.go`; remove user tests from `repo/repo_test.go`
- [x] **Step 11:** Final verification (`go build`, `go test`, `golangci-lint run`)
