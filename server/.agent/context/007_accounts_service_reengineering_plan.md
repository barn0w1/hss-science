# Accounts Service Reengineering Plan -- Iteration 2+

**Date:** 2026-03-02
**Status:** Iteration 1 complete. Iteration 2 is the next body of work.

---

## Table of Contents

1. [Architectural Decisions (Settled)](#1-architectural-decisions-settled)
2. [Current State After Iteration 1](#2-current-state-after-iteration-1)
3. [Known Architectural Debts](#3-known-architectural-debts)
4. [Iteration 2: Deep-Dive Implementation Guide](#4-iteration-2-deep-dive-implementation-guide)
   - 4.1 `internal/oidc/domain.go` — exact field sets
   - 4.2 `internal/oidc/ports.go` — exact Go signatures
   - 4.3 PostgreSQL Adapter Contract — null handling & scan targets
   - 4.4 Application Service Contract — ID generation & TTL policy
   - 4.5 Transaction Boundaries
   - 4.6 `internal/oidc/adapter/` — zitadel type wrappers
   - 4.7 `internal/oidc/adapter/storage.go` — op.Storage method map
   - 4.8 Error Translation Layer Map
   - 4.9 `main.go` No-Break Rewire Sequence
   - 4.10 Test Migration Guide
5. [Iteration 2 Implementation Checklist](#5-iteration-2-implementation-checklist)
6. [Iteration 3: Cleanup and Polish](#6-iteration-3-cleanup-and-polish)
7. [Target End-State Directory Tree](#7-target-end-state-directory-tree)

---

## 1. Architectural Decisions (Settled)

These decisions were made during Iteration 1 and are **not open for revisiting**.

### 1.1 Identity Model

- **Strict 1:1 User↔FederatedIdentity.** Enforced by `UNIQUE(user_id)` on `federated_identities`.
- **Immutable User profile.** Fields (`email`, `name`, `given_name`, `family_name`, `picture`, `email_verified`) populated at first login from IdP claims and never overwritten. No `updated_at` column on `users`.
- **FederatedIdentity is the live sync record.** `provider_*` columns mirror raw upstream claims, refreshed on every login.
- **ULID primary keys** for `users` and `federated_identities` (application-generated, time-sortable, stored as `TEXT` in PostgreSQL).

### 1.2 Architecture Patterns

- **Hexagonal / Ports-and-Adapters** within each `internal/` module.
- **Consumer-defined interfaces.** Modules define the interfaces they need; no module imports concrete implementations of another module.
- **Domain error translation.** `domerr` sentinels (`ErrNotFound`, `ErrAlreadyExists`, `ErrUnauthorized`, `ErrInternal`) are translated at persistence boundaries. Infrastructure layers never leak `sql.ErrNoRows` into domain code. Protocol layers (OIDC, HTTP) translate domain errors into protocol-specific responses.
- **No `db:` tags on domain types.** Scan targets live in `postgres/` sub-packages.
- **Service layer owns ID generation.** Services call `ulid.Make().String()` to generate IDs before writing to repos. Repos never generate IDs.

### 1.3 Known Issues in Current Code (Iteration 2 targets)

| Issue | Location | Fix |
|---|---|---|
| `oidcprovider.Storage` is ~425 lines, ~30 methods, implements `op.Storage` + `op.ClientCredentialsStorage` | `oidcprovider/storage.go` | Dismantle into `internal/oidc/adapter/storage.go` thin compositor |
| `model.*` DTOs have `db:` tags, shared by repo and oidcprovider adapter layers | `model/` | Replace with clean domain types in `internal/oidc/domain.go` |
| Auth request 30-min TTL hardcoded in SQL `activeFilter` string fragment | `repo/authrequest.go:35` | Move to `AuthRequestService` as configurable `time.Duration` |
| Token expiry enforced in SQL `WHERE expiration > now()` | `repo/token.go:89,97,103` | **Keep in SQL** — this is a data property, not configurable policy |
| `bcrypt.CompareHashAndPassword` called in `oidcprovider/storage.go:264` | `oidcprovider/storage.go` | Move to `ClientService.AuthorizeSecret` |
| `uuid.New().String()` used for all ID generation in `repo/token.go:23,58,59,60` | `repo/token.go` | Replace with `ulid.Make().String()` in service layer |
| `authReqAdapter` bridge in `main.go` wraps legacy `repo.AuthRequestRepository` | `main.go:145` | Replace with bridge wrapping `oidc.AuthRequestService` |

---

## 2. Current State After Iteration 1

### 2.1 Directory Tree

```
services/accounts/
  main.go                               -- identity.Service + authn.Handler + authReqAdapter bridge
  internal/
    pkg/domerr/errors.go                -- ErrNotFound, ErrAlreadyExists, ErrUnauthorized, ErrInternal
    pkg/crypto/aes.go                   -- AES-256-GCM Encrypt/Decrypt
    identity/domain.go                  -- User, FederatedIdentity, FederatedClaims (no db tags)
    identity/ports.go                   -- Repository + Service interfaces
    identity/service.go                 -- FindOrCreateByFederatedLogin, GetUser
    identity/postgres/user_repo.go      -- scan targets, domerr translation
    authn/handler.go                    -- HTTP handlers; AuthRequestQuerier interface defined here
    authn/provider*.go                  -- Google + GitHub upstream providers
  config/config.go                      -- monolithic Config struct
  oidcprovider/storage.go               -- LEGACY: god-object, still wired in main.go
  oidcprovider/authrequest.go           -- wraps model.AuthRequest -> op.AuthRequest
  oidcprovider/client.go                -- wraps model.Client -> op.Client
  oidcprovider/refreshtoken.go          -- wraps model.RefreshToken -> op.RefreshTokenRequest
  oidcprovider/keys.go                  -- SigningKeyWithID, PublicKeyWithID
  oidcprovider/provider.go              -- NewProvider(*Storage, ...) hardcoded op.Config
  model/authrequest.go                  -- LEGACY: model.AuthRequest (db: tags)
  model/client.go                       -- LEGACY: model.Client (db: tags)
  model/token.go                        -- LEGACY: model.Token, model.RefreshToken (db: tags)
  repo/authrequest.go                   -- LEGACY: SQL with activeFilter TTL
  repo/client.go                        -- LEGACY: SQL GetByID
  repo/token.go                         -- LEGACY: SQL with uuid.New() ID generation
  repo/repo_test.go                     -- integration tests (testcontainers)
  migrations/001_initial.sql            -- reengineered schema
  testhelper/testdb.go                  -- RunMigrations, CleanTables
```

### 2.2 Dependency Graph (Current)

```
main.go
  ├── identity.NewService(identitypg.NewUserRepository(db))
  ├── repo.NewClientRepository(db)          ← legacy
  ├── repo.NewAuthRequestRepository(db)     ← legacy
  ├── repo.NewTokenRepository(db)           ← legacy
  ├── oidcprovider.NewStorage(db, identitySvc, clientRepo, authReqRepo, tokenRepo, ...)
  ├── oidcprovider.NewProvider(issuer, cryptoKey, storage, logger)
  ├── authn.NewHandler(..., &authReqAdapter{repo: authReqRepo}, ...)
  └── chi.Router

oidcprovider.Storage
  ├── identity.Service  (clean — from Iteration 1)
  ├── domerr            (clean — from Iteration 1)
  ├── model.AuthRequest (legacy)
  ├── model.Client      (legacy)
  ├── model.Token       (legacy)
  └── model.RefreshToken (legacy)
```

---

## 3. Known Architectural Debts

### 3.1 Current `op.Storage` method inventory

Complete trace of all 30 methods and their data flows:

| Method | Calls | Returns | Notes |
|---|---|---|---|
| `CreateAuthRequest` | `authReqRepo.Create()` | `op.AuthRequest` | `uuid.New()` ID; `*uint` MaxAge clamped to `*int64` |
| `AuthRequestByID` | `authReqRepo.GetByID()` | `op.AuthRequest` | TTL in SQL `activeFilter`; `sql.ErrNoRows→oidc.Err` |
| `AuthRequestByCode` | `authReqRepo.GetByCode()` | `op.AuthRequest` | Same TTL issue |
| `SaveAuthCode` | `authReqRepo.SaveCode()` | `error` | Pure delegation |
| `DeleteAuthRequest` | `authReqRepo.Delete()` | `error` | Pure delegation |
| `CreateAccessToken` | `tokenRepo.CreateAccess()` | `string, time.Time, error` | Computes exp from `accessTokenLifetime` |
| `CreateAccessAndRefreshTokens` | `tokenRepo.CreateAccessAndRefresh()` | `string, string, time.Time, error` | Extracts authTime/amr via interface assertions; transaction in repo |
| `TokenRequestByRefreshToken` | `tokenRepo.GetRefreshToken()` | `op.RefreshTokenRequest` | `sql.ErrNoRows→op.ErrInvalidRefreshToken` |
| `TerminateSession` | `tokenRepo.DeleteByUserAndClient()` | `error` | Transaction in repo |
| `RevokeToken` | `tokenRepo.Revoke()` or `RevokeRefreshToken()` | `*oidc.Error` | Conditional on `userID != ""` |
| `GetRefreshTokenInfo` | `tokenRepo.GetRefreshInfo()` | `string, string, error` | `sql.ErrNoRows→op.ErrInvalidRefreshToken` |
| `GetClientByClientID` | `clientRepo.GetByID()` | `op.Client` | `sql.ErrNoRows→oidc.ErrInvalidClient` |
| `AuthorizeClientIDSecret` | `clientRepo.GetByID()` + bcrypt | `error` | bcrypt in adapter layer — wrong place |
| `SetUserinfoFromScopes` | `identity.GetUser()` | `error` | Clean already |
| `SetUserinfoFromRequest` | `identity.GetUser()` | `error` | Clean already |
| `SetUserinfoFromToken` | `tokenRepo.GetByID()` + `identity.GetUser()` | `error` | Two lookups |
| `SetIntrospectionFromToken` | `tokenRepo.GetByID()` + `identity.GetUser()` | `error` | Clean already |
| `GetPrivateClaimsFromScopes` | — | `map[string]any{}, nil` | Not implemented, trivial |
| `GetKeyByIDAndClientID` | — | error | JWT Profile grant not supported |
| `ValidateJWTProfileScopes` | — | error | JWT Profile grant not supported |
| `Health` | `db.PingContext()` | `error` | Direct DB dep |
| `SigningKey` | — | `op.SigningKey` | Returns field directly |
| `SignatureAlgorithms` | — | `[]jose.SignatureAlgorithm` | Hardcoded RS256 |
| `KeySet` | — | `[]op.Key` | Returns field directly |
| `ClientCredentials` | `AuthorizeClientIDSecret()` + `GetClientByClientID()` | `op.Client, error` | Two serial repo lookups; optimize to one in `ClientService.ClientCredentials` |
| `ClientCredentialsTokenRequest` | — | `op.TokenRequest, error` | Returns private `clientCredentialsTokenRequest` struct |

Helper methods (not part of `op.Storage`):
- `setUserinfo(ctx, *oidc.UserInfo, userID, scopes)` — scope switch, reads from `identity.User`
- `setIntrospectionUserinfo(introspection, *identity.User, scopes)` — same
- `clientIDFromRequest(op.TokenRequest) string` — type switch over 3 concrete types
- `promptToStrings([]string) []string` — nil if empty

---

## 4. Iteration 2: Deep-Dive Implementation Guide

### 4.1 `internal/oidc/domain.go` — Exact Field Sets

These are exact mirrors of `model.*` structs with `db:` tags removed. Field names and types are identical to ensure zero-friction migration.

```go
package oidc

import "time"

// AuthRequest represents an in-flight OIDC authorization request.
// ID is a ULID (replacing uuid.New()). Nullable DB columns map to
// zero-value fields (not pointers) in the domain type; the scan target
// in the postgres adapter handles the pointer-dereference.
type AuthRequest struct {
    ID                  string
    ClientID            string
    RedirectURI         string
    State               string
    Nonce               string
    Scopes              []string
    ResponseType        string
    ResponseMode        string
    CodeChallenge       string
    CodeChallengeMethod string
    Prompt              []string
    MaxAge              *int64    // nil means "no max_age constraint"
    LoginHint           string
    UserID              string    // empty string until CompleteLogin is called
    AuthTime            time.Time // zero value until CompleteLogin is called
    AMR                 []string
    IsDone              bool
    Code                string    // empty until SaveCode is called
    CreatedAt           time.Time
}

// Client represents a registered OIDC relying-party client.
// ApplicationType, AuthMethod, ResponseTypes, GrantTypes, AccessTokenType
// are stored as strings in the DB; the adapter layer converts them to
// zitadel enum types (op.ApplicationType, oidc.AuthMethod, etc.).
type Client struct {
    ID                       string
    SecretHash               string
    RedirectURIs             []string
    PostLogoutRedirectURIs   []string
    ApplicationType          string // "web" | "native" | "user_agent"
    AuthMethod               string // "client_secret_basic" | "client_secret_post" | "none" | "private_key_jwt"
    ResponseTypes            []string
    GrantTypes               []string
    AccessTokenType          string // "jwt" | "bearer"
    IDTokenLifetimeSeconds   int
    ClockSkewSeconds         int
    IDTokenUserinfoAssertion bool
    CreatedAt                time.Time
    UpdatedAt                time.Time
}

// Token represents an issued access token.
// RefreshTokenID is empty for tokens without an associated refresh token.
type Token struct {
    ID             string
    ClientID       string
    Subject        string
    Audience       []string
    Scopes         []string
    Expiration     time.Time
    RefreshTokenID string    // empty for pure access tokens
    CreatedAt      time.Time
}

// RefreshToken represents an issued refresh token.
// Token is the opaque token string (ULID, URL-safe); ID is the row PK.
// AccessTokenID may be empty (set when the paired access token exists).
type RefreshToken struct {
    ID            string
    Token         string    // opaque token value, UNIQUE in DB
    ClientID      string
    UserID        string
    Audience      []string
    Scopes        []string
    AuthTime      time.Time
    AMR           []string
    AccessTokenID string    // empty when paired access token has been revoked
    Expiration    time.Time
    CreatedAt     time.Time
}
```

### 4.2 `internal/oidc/ports.go` — Exact Go Signatures

```go
package oidc

import (
    "context"
    "time"
)

// --- Driven ports (Repository interfaces) ---

// AuthRequestRepository defines persistence operations for auth requests.
// No TTL filter in any query — TTL enforcement is the service's responsibility.
type AuthRequestRepository interface {
    Create(ctx context.Context, ar *AuthRequest) error
    GetByID(ctx context.Context, id string) (*AuthRequest, error)
    GetByCode(ctx context.Context, code string) (*AuthRequest, error)
    SaveCode(ctx context.Context, id, code string) error
    CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
    Delete(ctx context.Context, id string) error
}

// ClientRepository defines persistence operations for OIDC clients.
type ClientRepository interface {
    GetByID(ctx context.Context, clientID string) (*Client, error)
}

// TokenRepository defines persistence operations for tokens.
// Token and RefreshToken IDs are generated by the service, not the repo.
// Token expiration filtering (expiration > now()) stays in SQL because it
// is a data validity property, not a configurable business policy.
type TokenRepository interface {
    CreateAccess(ctx context.Context, access *Token) error
    CreateAccessAndRefresh(ctx context.Context, access *Token, refresh *RefreshToken, currentRefreshToken string) error
    GetByID(ctx context.Context, tokenID string) (*Token, error)
    GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error)
    GetRefreshInfo(ctx context.Context, token string) (userID, tokenID string, err error)
    DeleteByUserAndClient(ctx context.Context, userID, clientID string) error
    Revoke(ctx context.Context, tokenID string) error
    RevokeRefreshToken(ctx context.Context, token string) error
}

// --- Driving ports (Service interfaces) ---

// AuthRequestService defines use cases for the auth request lifecycle.
// It also satisfies authn.AuthRequestQuerier for CompleteLogin (same
// signature). GetByID returns *AuthRequest which does NOT satisfy
// authn.AuthRequestQuerier.GetByID directly; main.go uses a thin bridge
// struct (see Section 4.9).
type AuthRequestService interface {
    Create(ctx context.Context, ar *AuthRequest) error
    GetByID(ctx context.Context, id string) (*AuthRequest, error)
    GetByCode(ctx context.Context, code string) (*AuthRequest, error)
    SaveCode(ctx context.Context, id, code string) error
    CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error
    Delete(ctx context.Context, id string) error
}

// ClientService defines use cases for client authentication.
// ClientCredentials is a convenience method that combines GetByID and
// AuthorizeSecret in a single repo lookup (optimization over current two-
// lookup pattern in oidcprovider/storage.go:336-341).
type ClientService interface {
    GetByID(ctx context.Context, clientID string) (*Client, error)
    AuthorizeSecret(ctx context.Context, clientID, clientSecret string) error
    ClientCredentials(ctx context.Context, clientID, clientSecret string) (*Client, error)
}

// TokenService defines use cases for the token lifecycle.
// Services generate ULID IDs; repos receive fully-formed domain objects.
// Expirations are passed from the adapter layer (which owns the TTL config).
type TokenService interface {
    CreateAccess(ctx context.Context, clientID, subject string, audience, scopes []string, expiration time.Time) (tokenID string, err error)
    CreateAccessAndRefresh(ctx context.Context, clientID, subject string, audience, scopes []string, accessExpiration, refreshExpiration, authTime time.Time, amr []string, currentRefreshToken string) (accessID, refreshToken string, err error)
    GetByID(ctx context.Context, tokenID string) (*Token, error)
    GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error)
    GetRefreshInfo(ctx context.Context, token string) (userID, tokenID string, err error)
    DeleteByUserAndClient(ctx context.Context, userID, clientID string) error
    Revoke(ctx context.Context, tokenID string) error
    RevokeRefreshToken(ctx context.Context, token string) error
}
```

### 4.3 PostgreSQL Adapter Contract — Null Handling & Scan Targets

Each adapter file uses a private `*Row` struct with `db:` tags as the scan target. The domain type has no tags.

**`internal/oidc/postgres/authrequest_repo.go`**

Nullable DB columns in `auth_requests`:
- `user_id TEXT` — NULL until `CompleteLogin` called
- `auth_time TIMESTAMPTZ` — NULL until `CompleteLogin` called
- `code TEXT` — NULL until `SaveCode` called
- `max_age INTEGER` — NULL if not provided in auth request

Scan target:
```go
type authRequestRow struct {
    ID                  string         `db:"id"`
    ClientID            string         `db:"client_id"`
    RedirectURI         string         `db:"redirect_uri"`
    State               string         `db:"state"`
    Nonce               string         `db:"nonce"`
    Scopes              pq.StringArray `db:"scopes"`
    ResponseType        string         `db:"response_type"`
    ResponseMode        string         `db:"response_mode"`
    CodeChallenge       string         `db:"code_challenge"`
    CodeChallengeMethod string         `db:"code_challenge_method"`
    Prompt              pq.StringArray `db:"prompt"`
    MaxAge              *int64         `db:"max_age"`
    LoginHint           string         `db:"login_hint"`
    UserID              *string        `db:"user_id"`      // nullable
    AuthTime            *time.Time     `db:"auth_time"`    // nullable
    AMR                 pq.StringArray `db:"amr"`
    IsDone              bool           `db:"is_done"`
    Code                *string        `db:"code"`         // nullable
    CreatedAt           time.Time      `db:"created_at"`
}
```

Note: This scan target uses `pq.StringArray` for slices (same as current `repo/authrequest.go`'s manual scan approach). The struct-scan style is simpler and matches how `identity/postgres` works. Alternatively, the current repo uses `row.Scan(...)` with explicit `pq.StringArray` locals — either approach works; struct-scan is preferred for consistency.

Mapper function:
```go
func toAuthRequest(row authRequestRow) *oidc.AuthRequest {
    ar := &oidc.AuthRequest{
        ID: row.ID, ClientID: row.ClientID, ...
        MaxAge: row.MaxAge,
        // Dereference nullable fields with zero-value fallback:
    }
    if row.UserID != nil { ar.UserID = *row.UserID }
    if row.AuthTime != nil { ar.AuthTime = *row.AuthTime }
    if row.Code != nil { ar.Code = *row.Code }
    return ar
}
```

Error translation: `sql.ErrNoRows` → `domerr.ErrNotFound` (same pattern as `identity/postgres`).

**`internal/oidc/postgres/client_repo.go`**

All `clients` columns are NOT NULL. Use `pq.StringArray` for `redirect_uris`, `post_logout_redirect_uris`, `response_types`, `grant_types`.

```go
type clientRow struct {
    ID                       string         `db:"id"`
    SecretHash               string         `db:"secret_hash"`
    RedirectURIs             pq.StringArray `db:"redirect_uris"`
    PostLogoutRedirectURIs   pq.StringArray `db:"post_logout_redirect_uris"`
    ApplicationType          string         `db:"application_type"`
    AuthMethod               string         `db:"auth_method"`
    ResponseTypes            pq.StringArray `db:"response_types"`
    GrantTypes               pq.StringArray `db:"grant_types"`
    AccessTokenType          string         `db:"access_token_type"`
    IDTokenLifetimeSeconds   int            `db:"id_token_lifetime_seconds"`
    ClockSkewSeconds         int            `db:"clock_skew_seconds"`
    IDTokenUserinfoAssertion bool           `db:"id_token_userinfo_assertion"`
    CreatedAt                time.Time      `db:"created_at"`
    UpdatedAt                time.Time      `db:"updated_at"`
}
```

Error translation: `sql.ErrNoRows` → `domerr.ErrNotFound`.

**`internal/oidc/postgres/token_repo.go`**

Nullable columns:
- `tokens.refresh_token_id TEXT` — NULL for pure access tokens (no associated refresh token)
- `refresh_tokens.access_token_id TEXT` — NULL when paired access token has been revoked

Scan targets:
```go
type tokenRow struct {
    ID             string         `db:"id"`
    ClientID       string         `db:"client_id"`
    Subject        string         `db:"subject"`
    Audience       pq.StringArray `db:"audience"`
    Scopes         pq.StringArray `db:"scopes"`
    Expiration     time.Time      `db:"expiration"`
    RefreshTokenID *string        `db:"refresh_token_id"` // nullable
    CreatedAt      time.Time      `db:"created_at"`
}

type refreshTokenRow struct {
    ID            string         `db:"id"`
    Token         string         `db:"token"`
    ClientID      string         `db:"client_id"`
    UserID        string         `db:"user_id"`
    Audience      pq.StringArray `db:"audience"`
    Scopes        pq.StringArray `db:"scopes"`
    AuthTime      time.Time      `db:"auth_time"`
    AMR           pq.StringArray `db:"amr"`
    AccessTokenID *string        `db:"access_token_id"` // nullable
    Expiration    time.Time      `db:"expiration"`
    CreatedAt     time.Time      `db:"created_at"`
}
```

**Token expiration filter stays in SQL.** The queries:
```sql
-- GetByID
SELECT ... FROM tokens WHERE id = $1 AND expiration > now()

-- GetRefreshToken
SELECT ... FROM refresh_tokens WHERE token = $1 AND expiration > now()

-- GetRefreshInfo
SELECT user_id, id FROM refresh_tokens WHERE token = $1 AND expiration > now()
```

This is intentional. Token expiration is set at creation time, embedded in the record, and is a data validity property. It is not a configurable policy (unlike auth request TTL). `GetRefreshInfo` returns `(userID, tokenID string)` where `tokenID` is `refresh_tokens.id` (the PK, not the access token ID).

### 4.4 Application Service Contract — ID Generation & TTL Policy

**`internal/oidc/authrequest_svc.go`**

```go
type authRequestService struct {
    repo            AuthRequestRepository
    authRequestTTL  time.Duration // default: 30 * time.Minute, from Config
}

func NewAuthRequestService(repo AuthRequestRepository, authRequestTTL time.Duration) AuthRequestService {
    return &authRequestService{repo: repo, authRequestTTL: authRequestTTL}
}
```

TTL enforcement in `GetByID` and `GetByCode`:
```go
func (s *authRequestService) GetByID(ctx context.Context, id string) (*AuthRequest, error) {
    ar, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return nil, err // domerr.ErrNotFound already set by repo
    }
    if time.Now().UTC().After(ar.CreatedAt.Add(s.authRequestTTL)) {
        return nil, domerr.ErrNotFound // expired = not found from caller's perspective
    }
    return ar, nil
}
```

`Create` in the service does NOT generate an ID — the `StorageAdapter` generates the ULID before calling `Create`, so the `*AuthRequest` passed to `Create` already has its ID set.

**`internal/oidc/client_svc.go`**

```go
func (s *clientService) ClientCredentials(ctx context.Context, clientID, clientSecret string) (*Client, error) {
    c, err := s.repo.GetByID(ctx, clientID)
    if err != nil {
        return nil, err // domerr.ErrNotFound already set
    }
    if err := bcrypt.CompareHashAndPassword([]byte(c.SecretHash), []byte(clientSecret)); err != nil {
        return nil, fmt.Errorf("client %s: %w", clientID, domerr.ErrUnauthorized)
    }
    return c, nil
}

func (s *clientService) AuthorizeSecret(ctx context.Context, clientID, clientSecret string) error {
    _, err := s.ClientCredentials(ctx, clientID, clientSecret)
    return err
}
```

**`internal/oidc/token_svc.go`** — ID generation pattern

```go
func (s *tokenService) CreateAccess(ctx context.Context, clientID, subject string, audience, scopes []string, expiration time.Time) (string, error) {
    id := newID() // ulid.Make().String()
    access := &Token{
        ID: id, ClientID: clientID, Subject: subject,
        Audience: audience, Scopes: scopes,
        Expiration: expiration, CreatedAt: time.Now().UTC(),
    }
    if err := s.repo.CreateAccess(ctx, access); err != nil {
        return "", err
    }
    return id, nil
}

func (s *tokenService) CreateAccessAndRefresh(
    ctx context.Context, clientID, subject string,
    audience, scopes []string,
    accessExpiration, refreshExpiration, authTime time.Time,
    amr []string, currentRefreshToken string,
) (accessID, refreshToken string, err error) {
    now := time.Now().UTC()
    accessID = newID()
    refreshID = newID()
    refreshTokenValue := newID() // the opaque token value clients present

    access := &Token{
        ID: accessID, ClientID: clientID, Subject: subject,
        Audience: audience, Scopes: scopes,
        Expiration: accessExpiration,
        RefreshTokenID: refreshID,
        CreatedAt: now,
    }
    refresh := &RefreshToken{
        ID: refreshID, Token: refreshTokenValue,
        ClientID: clientID, UserID: subject,
        Audience: audience, Scopes: scopes,
        AuthTime: authTime, AMR: amr,
        AccessTokenID: accessID,
        Expiration: refreshExpiration, CreatedAt: now,
    }
    if err := s.repo.CreateAccessAndRefresh(ctx, access, refresh, currentRefreshToken); err != nil {
        return "", "", err
    }
    return accessID, refreshTokenValue, nil
}
```

### 4.5 Transaction Boundaries

**Transaction 1: `TokenRepository.CreateAccessAndRefresh`**

```
BEGIN
  IF currentRefreshToken != "":
    DELETE FROM refresh_tokens WHERE token = currentRefreshToken
  INSERT INTO tokens (id, client_id, subject, audience, scopes, expiration, refresh_token_id, created_at)
    VALUES (access.ID, access.ClientID, access.Subject, ..., access.RefreshTokenID, access.CreatedAt)
  INSERT INTO refresh_tokens (id, token, client_id, user_id, audience, scopes, auth_time, amr, access_token_id, expiration, created_at)
    VALUES (refresh.ID, refresh.Token, ..., refresh.AccessTokenID, ...)
COMMIT
```

Cross-references:
- `tokens.refresh_token_id = refresh.ID` (the PK, not the opaque value)
- `refresh_tokens.access_token_id = access.ID` (the access token PK)
- `refresh_tokens.token = refreshTokenValue` (the opaque value clients use)

**Transaction 2: `TokenRepository.DeleteByUserAndClient`**

```
BEGIN
  DELETE FROM refresh_tokens WHERE user_id = $userID AND client_id = $clientID
  DELETE FROM tokens WHERE subject = $userID AND client_id = $clientID
COMMIT
```

Order: refresh_tokens first (no FK issues). Both tables have no FK relationship to each other.

**No transaction needed for:**
- `AuthRequestRepository.Create` — single INSERT
- `AuthRequestRepository.CompleteLogin` — single UPDATE
- `TokenRepository.CreateAccess` — single INSERT

### 4.6 `internal/oidc/adapter/` — Zitadel Type Wrappers

These files move from `oidcprovider/` with the only change being the wrapped type changes from `model.*` to `oidc.*` (domain types).

**`internal/oidc/adapter/authrequest.go`** (from `oidcprovider/authrequest.go`):

```go
// Wraps *oidc.AuthRequest (domain type) instead of *model.AuthRequest.
// All getter methods delegate to the domain struct directly.
type AuthRequest struct {
    domain *oidc.AuthRequest
}

func NewAuthRequest(ar *oidc.AuthRequest) *AuthRequest { return &AuthRequest{domain: ar} }

func (a *AuthRequest) GetID() string          { return a.domain.ID }
func (a *AuthRequest) GetClientID() string    { return a.domain.ClientID }
func (a *AuthRequest) GetRedirectURI() string { return a.domain.RedirectURI }
func (a *AuthRequest) GetState() string       { return a.domain.State }
func (a *AuthRequest) GetNonce() string       { return a.domain.Nonce }
func (a *AuthRequest) GetScopes() []string    { return a.domain.Scopes }
func (a *AuthRequest) GetSubject() string     { return a.domain.UserID }
func (a *AuthRequest) GetACR() string         { return "" }
func (a *AuthRequest) GetAMR() []string       { return a.domain.AMR }
func (a *AuthRequest) GetAuthTime() time.Time { return a.domain.AuthTime }
func (a *AuthRequest) Done() bool             { return a.domain.IsDone }
func (a *AuthRequest) GetAudience() []string  { return []string{a.domain.ClientID} }

func (a *AuthRequest) GetResponseType() oidcpkg.ResponseType {
    return oidcpkg.ResponseType(a.domain.ResponseType)
}
func (a *AuthRequest) GetResponseMode() oidcpkg.ResponseMode {
    return oidcpkg.ResponseMode(a.domain.ResponseMode)
}
func (a *AuthRequest) GetCodeChallenge() *oidcpkg.CodeChallenge {
    if a.domain.CodeChallenge == "" { return nil }
    return &oidcpkg.CodeChallenge{
        Challenge: a.domain.CodeChallenge,
        Method:    oidcpkg.CodeChallengeMethod(a.domain.CodeChallengeMethod),
    }
}
```

**`internal/oidc/adapter/client.go`** (from `oidcprovider/client.go`): identical structure, wraps `*oidc.Client` instead of `*model.Client`.

**`internal/oidc/adapter/refreshtoken.go`** (from `oidcprovider/refreshtoken.go`): wraps `*oidc.RefreshToken` instead of `*model.RefreshToken`.

**`internal/oidc/adapter/keys.go`** (from `oidcprovider/keys.go`): unchanged, no `model.*` dependency.

**`internal/oidc/adapter/provider.go`** (from `oidcprovider/provider.go`): one signature change:
```go
// OLD: func NewProvider(issuer string, cryptoKey [32]byte, storage *Storage, ...) (*op.Provider, error)
// NEW: func NewProvider(issuer string, cryptoKey [32]byte, storage op.Storage, ...) (*op.Provider, error)
//      Accepts the op.Storage interface; main.go passes *StorageAdapter.
```

### 4.7 `internal/oidc/adapter/storage.go` — `op.Storage` Method Map

The new `StorageAdapter` struct:

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
    healthCheck func(context.Context) error // set to db.PingContext in main.go
}

func NewStorageAdapter(
    identity identity.Service,
    authReqs oidc.AuthRequestService,
    clients oidc.ClientService,
    tokens oidc.TokenService,
    signing *SigningKeyWithID,
    public *PublicKeyWithID,
    accessTTL, refreshTTL time.Duration,
    healthCheck func(context.Context) error,
) *StorageAdapter { ... }
```

Complete method delegation table:

```
CreateAuthRequest(ctx, *oidcp.AuthRequest, userID) (op.AuthRequest, error):
  ar := &oidc.AuthRequest{
    ID: newID(),   // ← ulid.Make().String(), replacing uuid.New()
    ClientID: authReq.ClientID,
    ...
    MaxAge: clampMaxAge(authReq.MaxAge), // *uint → *int64, same clamping as current
    Prompt: promptToStrings(authReq.Prompt),
  }
  if userID != "" { ar.UserID = userID }
  if err := authReqs.Create(ctx, ar); err != nil { return nil, err }
  return NewAuthRequest(ar), nil

AuthRequestByID(ctx, id):
  ar, err := authReqs.GetByID(ctx, id)  // TTL check done in service
  if domerr.Is(err, domerr.ErrNotFound): return nil, oidcp.ErrInvalidRequest().WithDescription("auth request not found")
  return NewAuthRequest(ar), nil

AuthRequestByCode(ctx, code):
  ar, err := authReqs.GetByCode(ctx, code)  // TTL check done in service
  if domerr.Is(err, domerr.ErrNotFound): return nil, oidcp.ErrInvalidRequest().WithDescription("auth request not found for code")
  return NewAuthRequest(ar), nil

SaveAuthCode(ctx, id, code) error:
  return authReqs.SaveCode(ctx, id, code)

DeleteAuthRequest(ctx, id) error:
  return authReqs.Delete(ctx, id)

CreateAccessToken(ctx, request):
  exp := time.Now().UTC().Add(s.accessTTL)
  tokenID, err := tokens.CreateAccess(ctx, clientIDFromRequest(request), request.GetSubject(), request.GetAudience(), request.GetScopes(), exp)
  return tokenID, exp, err

CreateAccessAndRefreshTokens(ctx, request, currentRefreshToken):
  accessExp := time.Now().UTC().Add(s.accessTTL)
  refreshExp := time.Now().UTC().Add(s.refreshTTL)
  authTime, amr := extractAuthTimeAMR(request)  // replaces interface assertions
  accessID, refreshToken, err := tokens.CreateAccessAndRefresh(ctx,
    clientIDFromRequest(request), request.GetSubject(), request.GetAudience(), request.GetScopes(),
    accessExp, refreshExp, authTime, amr, currentRefreshToken)
  return accessID, refreshToken, accessExp, err

TokenRequestByRefreshToken(ctx, refreshToken):
  rt, err := tokens.GetRefreshToken(ctx, refreshToken)
  if domerr.Is(err, domerr.ErrNotFound): return nil, op.ErrInvalidRefreshToken
  return NewRefreshTokenRequest(rt), nil

TerminateSession(ctx, userID, clientID) error:
  return tokens.DeleteByUserAndClient(ctx, userID, clientID)

RevokeToken(ctx, tokenOrTokenID, userID, clientID) *oidcp.Error:
  if userID != "":
    tokens.Revoke(ctx, tokenOrTokenID) → wrap in *oidcp.Error
  else:
    tokens.RevokeRefreshToken(ctx, tokenOrTokenID) → wrap in *oidcp.Error

GetRefreshTokenInfo(ctx, clientID, token):
  userID, tokenID, err := tokens.GetRefreshInfo(ctx, token)
  if domerr.Is(err, domerr.ErrNotFound): return "", "", op.ErrInvalidRefreshToken
  return userID, tokenID, nil

GetClientByClientID(ctx, clientID):
  c, err := clients.GetByID(ctx, clientID)
  if domerr.Is(err, domerr.ErrNotFound): return nil, oidcp.ErrInvalidClient().WithDescription("client not found")
  return NewClientAdapter(c), nil

AuthorizeClientIDSecret(ctx, clientID, clientSecret):
  err := clients.AuthorizeSecret(ctx, clientID, clientSecret)
  if domerr.Is(err, domerr.ErrNotFound): return oidcp.ErrInvalidClient().WithDescription("client not found")
  if domerr.Is(err, domerr.ErrUnauthorized): return oidcp.ErrInvalidClient().WithDescription("invalid client secret")
  return err

ClientCredentials(ctx, clientID, clientSecret):
  c, err := clients.ClientCredentials(ctx, clientID, clientSecret)  // ONE repo lookup
  if domerr.Is(err, domerr.ErrNotFound): return nil, oidcp.ErrInvalidClient().WithDescription("client not found")
  if domerr.Is(err, domerr.ErrUnauthorized): return nil, oidcp.ErrInvalidClient().WithDescription("invalid client secret")
  return NewClientAdapter(c), nil

ClientCredentialsTokenRequest(_, clientID, scopes):
  return &clientCredentialsTokenRequest{clientID, scopes}, nil  // private struct, unchanged

SetUserinfoFromScopes, SetUserinfoFromRequest, SetUserinfoFromToken, SetIntrospectionFromToken:
  unchanged logic, already using identity.Service

GetPrivateClaimsFromScopes: return map[string]any{}, nil
GetKeyByIDAndClientID:      return nil, errors.New("jwt profile grant not supported")
ValidateJWTProfileScopes:   return nil, errors.New("jwt profile grant not supported")
Health: return s.healthCheck(ctx)
SigningKey: return s.signing, nil
SignatureAlgorithms: return []jose.SignatureAlgorithm{jose.RS256}, nil
KeySet: return []op.Key{s.public}, nil
```

**Private helpers that stay in `adapter/storage.go`:**
- `clientIDFromRequest(op.TokenRequest) string` — type switch over `*AuthRequest`, `*RefreshTokenRequest`, `*clientCredentialsTokenRequest` (all now in adapter package)
- `extractAuthTimeAMR(op.TokenRequest) (time.Time, []string)` — extracted from the inline interface assertions in current `CreateAccessAndRefreshTokens`
- `promptToStrings([]string) []string` — unchanged, unexported
- `clampMaxAge(*uint) *int64` — extracted from current inline `min(*authReq.MaxAge, uint(math.MaxInt64))` logic
- `clientCredentialsTokenRequest` private struct — unchanged

### 4.8 Error Translation Layer Map

Three translation boundaries, each handling a specific set of errors:

**Boundary 1: Persistence → Domain** (in `postgres/` adapters)

```
sql.ErrNoRows  →  domerr.ErrNotFound
(other DB errors)  →  pass through as-is
```

Applied in: all `GetBy*` methods in `authrequest_repo.go`, `client_repo.go`, `token_repo.go`.

**Boundary 2: Domain → OIDC Protocol** (in `adapter/storage.go`)

```
domerr.ErrNotFound   (auth req)  →  oidc.ErrInvalidRequest().WithDescription("auth request not found")
domerr.ErrNotFound   (ar code)   →  oidc.ErrInvalidRequest().WithDescription("auth request not found for code")
domerr.ErrNotFound   (token)     →  oidc.ErrInvalidRequest().WithDescription("token not found")
domerr.ErrNotFound   (rf token)  →  op.ErrInvalidRefreshToken
domerr.ErrNotFound   (rf info)   →  op.ErrInvalidRefreshToken
domerr.ErrNotFound   (user)      →  oidc.ErrInvalidRequest().WithDescription("user not found")
domerr.ErrNotFound   (client)    →  oidc.ErrInvalidClient().WithDescription("client not found")
domerr.ErrUnauthorized (client)  →  oidc.ErrInvalidClient().WithDescription("invalid client secret")
(revoke errors)                  →  oidc.ErrServerError().WithParent(err)
```

**Boundary 3: Domain → HTTP** (already in `authn.Handler`, unchanged)

```
any error from identity.Service  →  500 Internal Server Error
```

### 4.9 `main.go` No-Break Rewire Sequence

This section shows the exact state of `main.go` imports and wiring before and after the atomic switch in Step 7.

**Phase A–F (Steps 1–6): No changes to `main.go`.** New packages exist and compile but nothing is wired.

**Step 7 atomic switch — `main.go` before:**

```go
import (
    "github.com/barn0w1/hss-science/server/services/accounts/oidcprovider"
    "github.com/barn0w1/hss-science/server/services/accounts/repo"
)

clientRepo  := repo.NewClientRepository(db)
authReqRepo := repo.NewAuthRequestRepository(db)
tokenRepo   := repo.NewTokenRepository(db)

storage := oidcprovider.NewStorage(db, identitySvc, clientRepo, authReqRepo, tokenRepo,
    signingKey, publicKey, accessTTL, refreshTTL)
provider, _ := oidcprovider.NewProvider(cfg.Issuer, cfg.CryptoKey, storage, logger)

loginHandler := authn.NewHandler(..., &authReqAdapter{repo: authReqRepo}, ...)

type authReqAdapter struct { repo *repo.AuthRequestRepository }
func (a *authReqAdapter) GetByID(ctx, id) (authn.AuthRequestInfo, error) { ... }
func (a *authReqAdapter) CompleteLogin(...) error { ... }
```

**Step 7 atomic switch — `main.go` after:**

```go
import (
    oidcadapter "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc/adapter"
    oidcdom    "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
    oidcpg    "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc/postgres"
)

// Repos
authReqRepo := oidcpg.NewAuthRequestRepository(db)
clientRepo  := oidcpg.NewClientRepository(db)
tokenRepo   := oidcpg.NewTokenRepository(db)

// Services
authReqSvc := oidcdom.NewAuthRequestService(authReqRepo, time.Duration(cfg.AuthRequestTTLMinutes)*time.Minute)
clientSvc  := oidcdom.NewClientService(clientRepo)
tokenSvc   := oidcdom.NewTokenService(tokenRepo)

// Keys (moved to oidcadapter package)
signingKey := oidcadapter.NewSigningKey(cfg.SigningKey)
publicKey  := oidcadapter.NewPublicKey(cfg.SigningKey)

// StorageAdapter replaces oidcprovider.Storage
storage := oidcadapter.NewStorageAdapter(
    identitySvc, authReqSvc, clientSvc, tokenSvc,
    signingKey, publicKey, accessTTL, refreshTTL, db.PingContext,
)

// NewProvider now accepts op.Storage interface
provider, _ := oidcadapter.NewProvider(cfg.Issuer, cfg.CryptoKey, storage, logger)

// authReqBridge: wraps AuthRequestService instead of repo.AuthRequestRepository
// Bridge is still needed because authn.AuthRequestQuerier.GetByID returns
// authn.AuthRequestInfo (not *oidc.AuthRequest). This is intentional --
// authn has no dependency on the oidc domain package.
loginHandler := authn.NewHandler(..., &authReqBridge{svc: authReqSvc}, ...)

type authReqBridge struct { svc oidcdom.AuthRequestService }
func (b *authReqBridge) GetByID(ctx context.Context, id string) (authn.AuthRequestInfo, error) {
    ar, err := b.svc.GetByID(ctx, id)
    if err != nil { return authn.AuthRequestInfo{}, err }
    return authn.AuthRequestInfo{ID: ar.ID}, nil
}
func (b *authReqBridge) CompleteLogin(ctx context.Context, id, userID string, authTime time.Time, amr []string) error {
    return b.svc.CompleteLogin(ctx, id, userID, authTime, amr)
}
```

**Why the bridge remains:** `authn.AuthRequestQuerier.GetByID` returns `authn.AuthRequestInfo` (a type defined in `internal/authn`). `AuthRequestService.GetByID` returns `*oidc.AuthRequest`. If `internal/oidc` imported `internal/authn` to satisfy this interface, it would create a circular dependency. The bridge in `main.go` is the correct architectural solution — it's wire-up code, not domain logic. It is now 8 lines and wraps the clean service, not a legacy repo.

**Config change required:** `AuthRequestTTLMinutes` needs to be added to `config.Config` with a default of `30`. This is the one config change needed.

### 4.10 Test Migration Guide

**`oidcprovider/storage_test.go`** (20 tests) → **`internal/oidc/adapter/storage_test.go`**

Constructor change:
```go
// OLD:
func newTestStorage(t *testing.T) *Storage {
    return NewStorage(db,
        identity.NewService(identitypg.NewUserRepository(db)),
        repo.NewClientRepository(db),
        repo.NewAuthRequestRepository(db),
        repo.NewTokenRepository(db),
        sk, pk, 15*time.Minute, 7*24*time.Hour)
}

// NEW:
func newTestAdapter(t *testing.T) *StorageAdapter {
    authReqRepo := oidcpg.NewAuthRequestRepository(db)
    clientRepo  := oidcpg.NewClientRepository(db)
    tokenRepo   := oidcpg.NewTokenRepository(db)
    authReqSvc  := oidcdom.NewAuthRequestService(authReqRepo, 30*time.Minute)
    clientSvc   := oidcdom.NewClientService(clientRepo)
    tokenSvc    := oidcdom.NewTokenService(tokenRepo)
    identitySvc := identity.NewService(identitypg.NewUserRepository(db))
    return NewStorageAdapter(identitySvc, authReqSvc, clientSvc, tokenSvc,
        sk, pk, 15*time.Minute, 7*24*time.Hour, db.PingContext)
}
```

Test body changes — the most impactful is where tests create `AuthRequest` objects directly:
```go
// OLD — embeds model struct:
ar := &AuthRequest{model: &model.AuthRequest{
    ClientID: "test-client", UserID: user.ID, Scopes: []string{"openid"},
    AuthTime: time.Now().UTC(), AMR: []string{"federated"},
}}

// NEW — embeds domain struct:
ar := &AuthRequest{domain: &oidcdom.AuthRequest{
    ClientID: "test-client", UserID: user.ID, Scopes: []string{"openid"},
    AuthTime: time.Now().UTC(), AMR: []string{"federated"},
}}
```

All 20 test functions keep their names and assertions unchanged. Only the constructor and the `AuthRequest` struct literal syntax changes.

**`repo/repo_test.go`** → `internal/oidc/postgres/repo_test.go`

- `TestMain` is identical (testcontainers setup)
- `TestClientRepository_GetByID` — `repo.NewClientRepository` → `oidcpg.NewClientRepository`; error check changes: `sql.ErrNoRows` → `domerr.ErrNotFound` (since new repo translates)
- `TestAuthRequestRepository_CRUD` — same, new repo used; imports `oidcdom` for `*oidcdom.AuthRequest` instead of `*model.AuthRequest`; note that the repo no longer does TTL filtering, so the time-sensitive parts of the test don't need to worry about the 30-min window
- `TestTokenRepository_*` — same pattern, `uuid.New().String()` in test setup can stay (these are just test values, not PKs we generate)

---

## 5. Iteration 2 Implementation Checklist

Each item is small enough to verify independently. "Compiles" means `go build ./services/accounts/...` passes.

### Step 1: Add `internal/oidc/domain.go`

- [x] Create `internal/oidc/domain.go` with exactly 4 structs: `AuthRequest`, `Client`, `Token`, `RefreshToken`
- [x] Zero imports except `"time"`
- [x] No `db:` tags on any field
- [x] **Verify:** `go build ./services/accounts/...` compiles (no other files changed)

### Step 2: Add `internal/oidc/ports.go`

- [x] Create `internal/oidc/ports.go` with exactly 6 interfaces: `AuthRequestRepository`, `ClientRepository`, `TokenRepository`, `AuthRequestService`, `ClientService`, `TokenService`
- [x] All signatures match Section 4.2 exactly
- [x] Import only `"context"` and `"time"` — no other packages
- [x] **Verify:** `go build ./services/accounts/...` compiles

### Step 3: Create `internal/oidc/postgres/authrequest_repo.go`

- [x] Create `authrequest_repo.go` with `AuthRequestRepository` struct, `authRequestRow` scan target
- [x] `GetByID` query: `WHERE id = $1` — **no time filter** (TTL moves to service)
- [x] `GetByCode` query: `WHERE code = $1` — **no time filter**
- [x] All nullable columns (`user_id`, `auth_time`, `code`, `max_age`) scanned as pointers
- [x] `sql.ErrNoRows` → `domerr.ErrNotFound` in `GetByID` and `GetByCode`
- [x] Compile-time check: `var _ oidc.AuthRequestRepository = (*AuthRequestRepository)(nil)`
- [x] **Verify:** `go build ./services/accounts/...` compiles

### Step 4: Create `internal/oidc/postgres/client_repo.go`

- [x] Create `client_repo.go` with `ClientRepository` struct, `clientRow` scan target
- [x] All array columns scanned as `pq.StringArray`
- [x] `sql.ErrNoRows` → `domerr.ErrNotFound` in `GetByID`
- [x] Compile-time check: `var _ oidc.ClientRepository = (*ClientRepository)(nil)`
- [x] **Verify:** `go build ./services/accounts/...` compiles

### Step 5: Create `internal/oidc/postgres/token_repo.go`

- [x] Create `token_repo.go` with `TokenRepository` struct, `tokenRow` and `refreshTokenRow` scan targets
- [x] `CreateAccess` receives a fully-formed `*Token` (ID already set by service)
- [x] `CreateAccessAndRefresh` runs the 3-step transaction (Section 4.5): DELETE old, INSERT token, INSERT refresh_token
- [x] `GetByID` query retains `AND expiration > now()` — **keep in SQL**
- [x] `GetRefreshToken` query retains `AND expiration > now()` — **keep in SQL**
- [x] `GetRefreshInfo` query retains `AND expiration > now()` — **keep in SQL**
- [x] Nullable columns (`refresh_token_id`, `access_token_id`) scanned as `*string`
- [x] `sql.ErrNoRows` → `domerr.ErrNotFound` in `GetByID`, `GetRefreshToken`, `GetRefreshInfo`
- [x] Compile-time check: `var _ oidc.TokenRepository = (*TokenRepository)(nil)`
- [x] **Verify:** `go build ./services/accounts/...` compiles

### Step 6: Create `internal/oidc/postgres/repo_test.go`

- [x] Copy `repo/repo_test.go`'s `TestMain`, change imports to `oidcpg`/`oidcdom`
- [x] `TestClientRepository_GetByID`: assert `domerr.ErrNotFound` (not `sql.ErrNoRows`) for not-found case
- [x] `TestAuthRequestRepository_CRUD`: use `*oidcdom.AuthRequest`; verify GetByID after 31-min-old record returns `domerr.ErrNotFound` (or skip TTL test and leave it to service test)
- [x] `TestTokenRepository_*`: use `*oidcdom.Token`/`*oidcdom.RefreshToken`; IDs set by test harness (can be `ulid.Make().String()`)
- [x] **Verify:** `go test ./services/accounts/internal/oidc/postgres/...` passes

### Step 7: Create `internal/oidc/authrequest_svc.go`

- [x] `NewAuthRequestService(repo AuthRequestRepository, authRequestTTL time.Duration) AuthRequestService`
- [x] `Create`: sets no ID (caller provides it); calls `repo.Create(ctx, ar)`
- [x] `GetByID`: calls `repo.GetByID`; if `time.Now().After(ar.CreatedAt.Add(ttl))` → return `domerr.ErrNotFound`
- [x] `GetByCode`: same TTL check as `GetByID`
- [x] `SaveCode`, `CompleteLogin`, `Delete`: pure delegation to repo
- [x] Compile-time check: `var _ AuthRequestService = (*authRequestService)(nil)`

### Step 8: Create `internal/oidc/authrequest_svc_test.go`

- [x] Unit tests with mock `AuthRequestRepository` (no DB)
- [x] Test: `GetByID` returns `domerr.ErrNotFound` when `CreatedAt + TTL` is in the past
- [x] Test: `GetByID` returns record when TTL not yet expired
- [x] Test: `GetByCode` same two cases
- [x] Test: `Create` delegates to repo unchanged
- [x] Test: `CompleteLogin` delegates unchanged
- [x] **Verify:** `go test ./services/accounts/internal/oidc/...` passes

### Step 9: Create `internal/oidc/client_svc.go`

- [x] `NewClientService(repo ClientRepository) ClientService`
- [x] `GetByID`: delegates; error passes through (repo already sets `domerr.ErrNotFound`)
- [x] `AuthorizeSecret`: calls `GetByID`, then `bcrypt.CompareHashAndPassword`; returns `domerr.ErrUnauthorized` on mismatch
- [x] `ClientCredentials`: calls `GetByID` once, then bcrypt; returns `domerr.ErrUnauthorized` on mismatch
- [x] Compile-time check: `var _ ClientService = (*clientService)(nil)`

### Step 10: Create `internal/oidc/client_svc_test.go`

- [x] Unit tests with mock `ClientRepository`
- [x] Test: `ClientCredentials` correct secret returns `*Client`
- [x] Test: `ClientCredentials` wrong secret returns `domerr.ErrUnauthorized`
- [x] Test: `ClientCredentials` no client returns `domerr.ErrNotFound`
- [x] Test: `AuthorizeSecret` same three cases
- [x] **Verify:** `go test ./services/accounts/internal/oidc/...` passes

### Step 11: Create `internal/oidc/token_svc.go`

- [x] `NewTokenService(repo TokenRepository) TokenService`
- [x] `CreateAccess`: generates `id = ulid.Make().String()`; constructs `*Token`; calls `repo.CreateAccess(ctx, token)`;  returns `id`
- [x] `CreateAccessAndRefresh`: generates 3 IDs (`accessID`, `refreshID`, `refreshTokenValue`); constructs `*Token` and `*RefreshToken`; calls `repo.CreateAccessAndRefresh(ctx, access, refresh, currentRefreshToken)`; returns `(accessID, refreshTokenValue, nil)`
- [x] `GetByID`, `GetRefreshToken`, `GetRefreshInfo`: pure delegation
- [x] `DeleteByUserAndClient`, `Revoke`, `RevokeRefreshToken`: pure delegation
- [x] Compile-time check: `var _ TokenService = (*tokenService)(nil)`

### Step 12: Create `internal/oidc/token_svc_test.go`

- [x] Unit tests with mock `TokenRepository`
- [x] Test: `CreateAccess` returns non-empty ULID
- [x] Test: `CreateAccessAndRefresh` returns two non-empty IDs; calls `repo.CreateAccessAndRefresh` with correct cross-references
- [x] Test: error propagation for all methods
- [x] **Verify:** `go test ./services/accounts/internal/oidc/...` passes

### Step 13: Move adapter files from `oidcprovider/`

- [x] Create `internal/oidc/adapter/authrequest.go` — copy `oidcprovider/authrequest.go`, change `*model.AuthRequest` → `*oidcdom.AuthRequest`, field accesses now use struct fields (e.g., `a.domain.ID` not `a.model.ID`)
- [x] Create `internal/oidc/adapter/client.go` — same for `*model.Client` → `*oidcdom.Client`
- [x] Create `internal/oidc/adapter/refreshtoken.go` — same for `*model.RefreshToken` → `*oidcdom.RefreshToken`
- [x] Create `internal/oidc/adapter/keys.go` — copy `oidcprovider/keys.go` unchanged
- [x] Create `internal/oidc/adapter/provider.go` — copy `oidcprovider/provider.go`; change `storage *Storage` → `storage op.Storage` in `NewProvider` signature
- [x] Move/adapt corresponding `_test.go` files
- [x] **Verify:** `go build ./services/accounts/...` compiles (adapter package exists; still not wired in)

### Step 14: Create `internal/oidc/adapter/storage.go`

- [x] `StorageAdapter` struct with 9 fields (Section 4.7)
- [x] `NewStorageAdapter(...)` constructor
- [x] Implement all methods per the delegation table in Section 4.7
- [x] Error translations per Section 4.8 (use `domerr.Is`, not `errors.Is(err, domerr.ErrNotFound)`)
- [x] `clientIDFromRequest`, `extractAuthTimeAMR`, `promptToStrings`, `clampMaxAge` as private helpers
- [x] `clientCredentialsTokenRequest` private struct with 3 getters
- [x] Compile-time check: `var _ op.Storage = (*StorageAdapter)(nil)`
- [x] **Verify:** `go build ./services/accounts/...` compiles

### Step 15: Move `oidcprovider/storage_test.go` → `internal/oidc/adapter/storage_test.go`

- [x] Update `TestMain` from `storageTestDB` package var to the new package
- [x] Change `newTestStorage` → `newTestAdapter` per Section 4.10
- [x] Change all `&AuthRequest{model: &model.AuthRequest{...}}` → `&AuthRequest{domain: &oidcdom.AuthRequest{...}}`
- [x] **Verify:** `go test ./services/accounts/internal/oidc/adapter/...` passes (all 20 tests)

### Step 16: Add `AuthRequestTTLMinutes` to `config.Config`

- [x] Add `AuthRequestTTLMinutes int` field to `config.Config`
- [x] In `config.Load()`: `cfg.AuthRequestTTLMinutes = getEnvInt("AUTH_REQUEST_TTL_MINUTES", 30)` with range validation `1–60`
- [x] Update `.env.example` with `AUTH_REQUEST_TTL_MINUTES=30`
- [x] **Verify:** `go test ./services/accounts/config/...` passes

### Step 17: Rewire `main.go` (atomic switch)

- [x] Remove imports: `oidcprovider`, all of `repo`
- [x] Add imports: `oidcadapter`, `oidcdom`, `oidcpg`
- [x] Build new repos, services, adapter per Section 4.9
- [x] Replace `authReqAdapter` bridge struct with `authReqBridge` wrapping `oidcdom.AuthRequestService`
- [x] Call `oidcadapter.NewProvider(...)` (not `oidcprovider.NewProvider`)
- [x] **Verify:** `go build ./services/accounts/...` compiles

### Step 18: Delete legacy packages

- [x] Delete `model/` directory (3 files: `authrequest.go`, `client.go`, `token.go`)
- [x] Delete `repo/` directory (4 files: `authrequest.go`, `client.go`, `token.go`, `repo_test.go`)
- [x] Delete `oidcprovider/` directory (9 files)
- [x] **Verify:** `go build ./services/accounts/...` compiles

### Step 19: Final verification

- [x] `go build ./services/accounts/...` — clean
- [x] `go vet ./services/accounts/...` — clean
- [x] `go test ./services/accounts/...` — all pass
- [x] `golangci-lint run ./services/accounts/...` — 0 issues
- [x] Confirm no `db:` tags outside `internal/*/postgres/` packages
- [x] Confirm no imports of `model`, `repo`, or `oidcprovider` anywhere
- [x] Confirm `main.go` imports only `internal/`, `config/`, standard library, and third-party packages
- [x] Confirm `authReqBridge` wraps `oidcdom.AuthRequestService` (not a legacy repo)

---

## 6. Iteration 3: Cleanup and Polish

### 6.1 Testcontainers Unification

Three separate `TestMain` containers after Iteration 2:
- `internal/identity/postgres/user_repo_test.go`
- `internal/oidc/postgres/repo_test.go`
- `internal/oidc/adapter/storage_test.go`

Preferred approach: shared `testhelper.NewTestDB(t)` function returning `*sqlx.DB` backed by a per-package testcontainer. Each test package starts its own container (acceptable) but uses consistent helper code.

### 6.2 Observability

- Add OpenTelemetry tracing spans to service method entry/exit in `identity.Service`, `AuthRequestService`, `ClientService`, `TokenService`.
- Add request-ID middleware to chi router.
- Propagate `*slog.Logger` into all service constructors.

### 6.3 Security Hardening

- **CSRF on `/login/select`:** Add `SameSite=Strict` to session cookies, or add a synchronizer token to the provider-selection form.
- **GitHub primary email:** Fall back to `/user/emails` endpoint when `/user` returns empty email.

### 6.4 Key Rotation

- Add `signing_keys` table with `id TEXT`, `private_key_pem TEXT`, `created_at TIMESTAMPTZ`, `expires_at TIMESTAMPTZ`.
- `KeySet()` returns all non-expired keys; `SigningKey()` returns the most recently created key.

### 6.5 Config Decomposition

- Extract `oidc.Config` from `config.Config` with fields: `AuthRequestTTLMinutes`, `AccessTokenLifetimeMinutes`, `RefreshTokenLifetimeDays`.
- `config.Load()` constructs and validates these; `main.go` maps them to module configs without direct `config.Config` coupling.

### 6.6 Auth Request Cleanup Job

Add a periodic goroutine that runs `DELETE FROM auth_requests WHERE created_at < now() - $1` with the configured TTL to prevent unbounded row growth.

### 6.7 Cleanup Checklist

- [ ] Unify testcontainers via shared `testhelper.NewTestDB`
- [ ] Add OpenTelemetry tracing spans to all service methods
- [ ] Add request-ID middleware
- [ ] Add CSRF protection to provider selection form
- [ ] Harden GitHub provider with `/user/emails` fallback
- [ ] Implement signing key rotation
- [ ] Decompose `config.Config` into per-module subsets
- [ ] Add periodic expired auth request cleanup job
- [ ] Propagate `*slog.Logger` to all service constructors

---

## 7. Target End-State Directory Tree

After all iterations:

```
services/accounts/
  main.go
  Dockerfile
  .env.example

  internal/
    pkg/
      domerr/errors.go
      domerr/errors_test.go
      crypto/aes.go
      crypto/aes_test.go

    identity/
      domain.go
      ports.go
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
      domain.go                   -- AuthRequest, Client, Token, RefreshToken (no db tags)
      ports.go                    -- 6 interfaces: 3 repos + 3 services
      authrequest_svc.go
      authrequest_svc_test.go
      client_svc.go
      client_svc_test.go
      token_svc.go
      token_svc_test.go
      adapter/
        storage.go                -- thin compositor ~120 lines (vs 425 now)
        storage_test.go           -- 20 tests migrated from oidcprovider/
        authrequest.go            -- op.AuthRequest wrapper
        authrequest_test.go
        client.go                 -- op.Client wrapper
        client_test.go
        refreshtoken.go           -- op.RefreshTokenRequest wrapper
        keys.go                   -- SigningKeyWithID, PublicKeyWithID
        keys_test.go
        provider.go               -- NewProvider(op.Storage, ...)
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

Packages deleted by end of Iteration 2: `model/`, `repo/`, `oidcprovider/`, `login/` (already gone from Iteration 1).

Import hierarchy (no cycles):
```
main.go
  → internal/oidc/adapter   (uses op.Storage)
  → internal/oidc           (domain + services)
  → internal/oidc/postgres  (persistence adapters)
  → internal/identity       (unchanged)
  → internal/authn          (unchanged)
  → config

internal/oidc/adapter  → internal/oidc, internal/identity, pkg/domerr, zitadel/oidc
internal/oidc          → pkg/domerr, ulid
internal/oidc/postgres → internal/oidc, pkg/domerr, sqlx, pq
internal/authn         → internal/identity, pkg/crypto
internal/identity      → pkg/domerr, ulid
```
