# gRPC Account Management API — Implementation Plan

_Based on full source read of `server/services/identity-service/` as of 2026-03-16._

---

## 1. Overview

### Goal
Add a gRPC `AccountManagementService` to the `accounts` binary that runs on a
separate port (`:50051`), sharing the same process, DB pool, and service
instances as the existing HTTP/OIDC server on `:8080`.

### Design constraints (already decided)
| Constraint | Detail |
|---|---|
| Transport | gRPC over TCP on `GRPC_PORT` (default `50051`); plain HTTP/2, no TLS termination in-process (handled by infra). |
| Authentication | RS256 JWT via `Authorization: Bearer <token>` gRPC metadata header. Verified in-process against the RSA public key already in memory. No DB lookup per request. |
| Principal | `sub` claim from the verified JWT (a ULID user ID). |
| Authorization | Self-only: all RPCs operate on the authenticated user's own data. |
| Profile ownership | Local edits win — user-set `name`/`picture` survive re-login. Only `federated_identities` continues to mirror upstream claims wholesale. |
| Account deletion | Out of scope. |
| Proto location | `api/proto/` at the repo root (managed by the existing `buf.yaml`). Generated code goes to `server/gen/`. |

### Not in this plan
- HTTP/REST gateway (no `grpc-gateway` annotations).
- Admin RPCs (cross-user access).
- Pagination on session listing (known gap; acceptable for v1; noted under trade-offs).
- Rate limiting on the gRPC port (not wired to `IPRateLimiter`; gRPC sits behind internal infra).

---

## 2. File Inventory

### New files
| File | Purpose |
|---|---|
| `api/proto/accounts/v1/account_management.proto` | Service definition |
| `server/gen/accounts/v1/account_management.pb.go` | Generated (buf generate) |
| `server/gen/accounts/v1/account_management_grpc.pb.go` | Generated (buf generate) |
| `server/services/identity-service/migrations/4_local_profile.up.sql` | Add `local_name`, `local_picture` columns |
| `server/services/identity-service/internal/grpc/server.go` | `grpc.Server` factory |
| `server/services/identity-service/internal/grpc/interceptor.go` | JWT auth unary interceptor |
| `server/services/identity-service/internal/grpc/errors.go` | `domerr` → gRPC status mapping |
| `server/services/identity-service/internal/grpc/handler.go` | `AccountManagementServiceServer` implementation |

### Modified files
| File | Change |
|---|---|
| `config/config.go` | Add `GRPCPort string` |
| `internal/pkg/domerr/errors.go` | Add `ErrFailedPrecondition` |
| `internal/identity/domain.go` | Add `LocalName *string`, `LocalPicture *string` to `User` |
| `internal/identity/ports.go` | Add 4 repo methods + 3 service methods |
| `internal/identity/service.go` | Implement new service methods; apply effective fields in `GetUser` |
| `internal/identity/postgres/user_repo.go` | Implement new repo methods; update `UpdateUserFromClaims` SQL |
| `main.go` | Wire gRPC server + listener + graceful shutdown |

---

## 3. Proto Definition

**Path**: `api/proto/accounts/v1/account_management.proto`
**buf module root**: `api/proto` (already declared in `buf.yaml`)

```protobuf
syntax = "proto3";

package accounts.v1;

option go_package = "github.com/barn0w1/hss-science/server/gen/accounts/v1;accountsv1";

import "google/protobuf/empty.proto";
import "google/protobuf/timestamp.proto";

// AccountManagementService provides self-service account operations.
// All RPCs require a valid Bearer JWT in the Authorization metadata header.
service AccountManagementService {
  // Profile
  rpc GetMyProfile(GetMyProfileRequest)       returns (Profile);
  rpc UpdateMyProfile(UpdateMyProfileRequest) returns (Profile);

  // Linked federated identities
  rpc ListLinkedProviders(ListLinkedProvidersRequest) returns (ListLinkedProvidersResponse);
  rpc UnlinkProvider(UnlinkProviderRequest)           returns (google.protobuf.Empty);

  // Sessions
  rpc ListActiveSessions(ListActiveSessionsRequest)       returns (ListActiveSessionsResponse);
  rpc RevokeSession(RevokeSessionRequest)                 returns (google.protobuf.Empty);
  rpc RevokeAllOtherSessions(RevokeAllOtherSessionsRequest) returns (google.protobuf.Empty);
}

// Profile is the canonical view of the authenticated user's account.
message Profile {
  string user_id         = 1;
  string email           = 2;
  bool   email_verified  = 3;
  // Effective display name: local override if set, otherwise upstream name.
  string name            = 4;
  string given_name      = 5;
  string family_name     = 6;
  // Effective picture URL: local override if set, otherwise upstream picture.
  string picture         = 7;
  // name_is_local is true if the user has set a local display name override.
  bool   name_is_local   = 8;
  // picture_is_local is true if the user has set a local picture override.
  bool   picture_is_local = 9;
  google.protobuf.Timestamp created_at = 10;
  google.protobuf.Timestamp updated_at = 11;
}

message GetMyProfileRequest {}

// UpdateMyProfileRequest uses proto3 optional so the client can distinguish
// "field not in request" (no change) from "field set to empty string" (clear override).
message UpdateMyProfileRequest {
  // If absent: do not change the name override.
  // If empty string: clear the local name override (revert to upstream name).
  // If non-empty: set as local name override.
  optional string name    = 1;
  // Same semantics as name.
  optional string picture = 2;
}

message FederatedProviderInfo {
  string identity_id       = 1;
  string provider          = 2;  // e.g. "google", "github"
  string provider_email    = 3;
  google.protobuf.Timestamp last_login_at = 4;
}

message ListLinkedProvidersRequest {}

message ListLinkedProvidersResponse {
  repeated FederatedProviderInfo providers = 1;
}

message UnlinkProviderRequest {
  string identity_id = 1;
}

message Session {
  string session_id   = 1;
  string device_name  = 2;
  string ip_address   = 3;
  google.protobuf.Timestamp created_at   = 4;
  google.protobuf.Timestamp last_used_at = 5;
}

message ListActiveSessionsRequest {}

message ListActiveSessionsResponse {
  repeated Session sessions = 1;
}

message RevokeSessionRequest {
  string session_id = 1;
}

// current_session_id is optional: the BFF passes the user's own dsid cookie value.
// If provided, that session is excluded from revocation.
// If empty, all of the user's active sessions are revoked.
message RevokeAllOtherSessionsRequest {
  string current_session_id = 1;
}
```

**Rationale for `optional string`**: proto3 optional generates a `*string` in Go,
allowing the handler to distinguish "not set" (nil) from "set to empty" (pointer to
`""`). This is the idiomatic way to express partial updates in proto3 without a
`FieldMask`.

**Why no `FieldMask`**: The updateable surface is small (name + picture). A FieldMask
is warranted when there are many fields with heterogeneous types. For two string
fields, optional presence is cleaner.

**`name_is_local` / `picture_is_local`**: Lets the frontend render a "reset to
provider default" affordance without a second RPC.

**`current_session_id` via client**: The BFF already reads the `dsid` cookie from the
user's browser request. Passing it here is the cleanest way to exclude the current
session without a DB lookup on the token.

---

## 4. Database Migration

**Path**: `server/services/identity-service/migrations/4_local_profile.up.sql`

```sql
ALTER TABLE users
  ADD COLUMN local_name    TEXT DEFAULT NULL,
  ADD COLUMN local_picture TEXT DEFAULT NULL;
```

**Rationale**: Nullable columns default to NULL, meaning no override. Existing rows
are unaffected. The login flow (`UpdateUserFromClaims`) will be updated to preserve
non-null values rather than touching them.

**Trade-off**: A separate `user_profile_overrides` table would be more normalized, but
the extra JOIN on every profile read is not justified for two fields. Nullable columns
on the existing `users` table are simpler, faster, and fully backward-compatible.

**What stays in `users.name`/`users.picture`**: These continue to hold the
upstream-provided values, refreshed on every login. `local_name`/`local_picture` are
the override layer. The service computes the effective value (COALESCE) so callers
always receive the right value and only need to inspect `LocalName`/`LocalPicture` to
know if an override is active.

---

## 5. Domain Layer Changes

### 5a. `internal/pkg/domerr/errors.go`

Add one sentinel:

```go
var (
    ErrNotFound          = errors.New("not found")
    ErrAlreadyExists     = errors.New("already exists")
    ErrUnauthorized      = errors.New("unauthorized")
    ErrInternal          = errors.New("internal error")
    ErrFailedPrecondition = errors.New("precondition failed") // NEW
)
```

Used by `UnlinkProvider` when the user attempts to remove their last federated
identity. Mapped to `codes.FailedPrecondition` in the gRPC error mapper.

---

### 5b. `internal/identity/domain.go`

Add two nullable fields to `User`:

```go
type User struct {
    ID            string
    Email         string
    EmailVerified bool
    Name          string     // effective display name (upstream or local override)
    GivenName     string
    FamilyName    string
    Picture       string     // effective picture (upstream or local override)
    CreatedAt     time.Time
    UpdatedAt     time.Time

    // LocalName is non-nil if the user has set a local display name override.
    // The Name field already reflects this override when returned from the service.
    LocalName    *string
    // LocalPicture is non-nil if the user has set a local picture URL override.
    LocalPicture *string
}
```

`Name` and `Picture` always hold the **effective** value (upstream name if no
override, local override if set). This keeps `userClaimsBridge.UserClaims()` in
`main.go` working without modification — it reads `user.Name` and `user.Picture`,
which are already correct.

`LocalName` and `LocalPicture` are exposed so the gRPC handler can populate
`name_is_local` / `picture_is_local` in the proto `Profile` message.

---

### 5c. `internal/identity/ports.go`

#### Repository interface additions

```go
type Repository interface {
    // ... existing methods unchanged ...

    // ListFederatedIdentities returns all federated identities for a user.
    ListFederatedIdentities(ctx context.Context, userID string) ([]*FederatedIdentity, error)

    // DeleteFederatedIdentity removes the federated identity with the given id
    // for the given userID. It returns ErrFailedPrecondition if the user has
    // only one linked identity (atomically checked in a transaction).
    // Returns ErrNotFound if the identity does not exist or belongs to another user.
    DeleteFederatedIdentity(ctx context.Context, id, userID string) error

    // UpdateLocalProfile sets local overrides for name and/or picture.
    // namePtr / picturePtr semantics:
    //   nil      → do not change this field
    //   &""      → clear the override (set column to NULL)
    //   &"value" → set the override
    UpdateLocalProfile(ctx context.Context, userID string, name, picture *string, updatedAt time.Time) error
}
```

#### Service interface additions

```go
type Service interface {
    // ... existing methods unchanged ...

    // UpdateProfile applies local profile overrides (name, picture).
    // namePtr / picturePtr follow the same nil/""/value semantics as the repo.
    UpdateProfile(ctx context.Context, userID string, name, picture *string) (*User, error)

    // ListLinkedProviders returns all federated identities for the user.
    ListLinkedProviders(ctx context.Context, userID string) ([]*FederatedIdentity, error)

    // UnlinkProvider removes the federated identity with the given identityID.
    // Returns ErrFailedPrecondition if it's the last linked identity.
    UnlinkProvider(ctx context.Context, userID, identityID string) error
}
```

---

### 5d. `internal/identity/service.go`

#### Modify `GetUser` — apply effective fields

The existing `GetUser` is:
```go
func (s *identityService) GetUser(ctx context.Context, userID string) (*User, error) {
    user, err := s.repo.GetByID(ctx, userID)
    ...
    return user, nil
}
```

Change to apply local overrides before returning:

```go
func (s *identityService) GetUser(ctx context.Context, userID string) (*User, error) {
    user, err := s.repo.GetByID(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("identity.GetUser(%s): %w", userID, err)
    }
    applyLocalOverrides(user)
    return user, nil
}

// applyLocalOverrides replaces Name/Picture with local overrides when set,
// leaving the LocalName/LocalPicture fields untouched for callers that need them.
func applyLocalOverrides(u *User) {
    if u.LocalName != nil && *u.LocalName != "" {
        u.Name = *u.LocalName
    }
    if u.LocalPicture != nil && *u.LocalPicture != "" {
        u.Picture = *u.LocalPicture
    }
}
```

`applyLocalOverrides` is applied in `GetUser` only. `FindOrCreateByFederatedLogin`
does not go through this path because it writes directly from the upstream claims.

#### New method `UpdateProfile`

```go
func (s *identityService) UpdateProfile(
    ctx context.Context, userID string, name, picture *string,
) (*User, error) {
    now := time.Now().UTC()
    if err := s.repo.UpdateLocalProfile(ctx, userID, name, picture, now); err != nil {
        return nil, fmt.Errorf("identity.UpdateProfile: %w", err)
    }
    return s.GetUser(ctx, userID)
}
```

Returns the updated `User` (with effective fields resolved) without a separate lookup
round-trip: `GetUser` calls `repo.GetByID`, which is a single indexed PK scan.

#### New method `ListLinkedProviders`

```go
func (s *identityService) ListLinkedProviders(
    ctx context.Context, userID string,
) ([]*FederatedIdentity, error) {
    fis, err := s.repo.ListFederatedIdentities(ctx, userID)
    if err != nil {
        return nil, fmt.Errorf("identity.ListLinkedProviders: %w", err)
    }
    return fis, nil
}
```

#### New method `UnlinkProvider`

```go
func (s *identityService) UnlinkProvider(
    ctx context.Context, userID, identityID string,
) error {
    if err := s.repo.DeleteFederatedIdentity(ctx, identityID, userID); err != nil {
        return fmt.Errorf("identity.UnlinkProvider: %w", err)
    }
    return nil
}
```

The last-provider guard is enforced inside `DeleteFederatedIdentity` at the repo
(DB) level using a transaction, guaranteeing atomicity even under concurrent calls.

---

### 5e. `internal/identity/postgres/user_repo.go`

#### Update `GetByID` — select `local_name`, `local_picture`

Current `userRow`:
```go
type userRow struct {
    ID            string    `db:"id"`
    Email         string    `db:"email"`
    ...
}
```

Add two nullable columns:
```go
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
    LocalName     *string   `db:"local_name"`    // NEW
    LocalPicture  *string   `db:"local_picture"` // NEW
}
```

Update `toUser`:
```go
func toUser(row userRow) *identity.User {
    return &identity.User{
        ...existing fields...
        LocalName:    row.LocalName,
        LocalPicture: row.LocalPicture,
    }
}
```

Update `GetByID` SELECT query to include the two new columns:
```sql
SELECT id, email, email_verified, name, given_name, family_name, picture,
       created_at, updated_at, local_name, local_picture
FROM users WHERE id = $1
```

#### Update `UpdateUserFromClaims` — preserve local overrides

Current SQL updates `name` and `picture` unconditionally. Change to:

```sql
UPDATE users
SET email          = $1,
    email_verified = $2,
    name           = CASE WHEN local_name    IS NULL THEN $3 ELSE name    END,
    given_name     = $4,
    family_name    = $5,
    picture        = CASE WHEN local_picture IS NULL THEN $6 ELSE picture END,
    updated_at     = $7
WHERE id = $8
```

Parameters $1–$8 are unchanged — only the body of the name/picture assignments
changes. This means the method signature is unchanged and no callers need updating.

**Rationale**: `CASE WHEN local_name IS NULL THEN $3 ELSE name END` is evaluated
atomically within the UPDATE. If `local_name` is NULL (no override), the upstream
name is written. If `local_name` is set (override active), the `name` column is left
as-is. Same logic for picture.

**`given_name` and `family_name`**: These are always overwritten by the upstream
provider. The design decision says "local edits win" only for `name` (display name)
and `picture`. `given_name` / `family_name` / `email` are identity attributes that
always come from the upstream IdP. They are not user-editable via the gRPC API.

#### New method `ListFederatedIdentities`

```go
func (r *UserRepository) ListFederatedIdentities(
    ctx context.Context, userID string,
) ([]*identity.FederatedIdentity, error) {
    var rows []fiRow
    err := r.db.SelectContext(ctx, &rows,
        `SELECT id, user_id, provider, provider_subject,
                provider_email, provider_email_verified,
                provider_display_name, provider_given_name, provider_family_name,
                provider_picture_url, last_login_at, created_at, updated_at
         FROM federated_identities
         WHERE user_id = $1
         ORDER BY created_at ASC`, userID)
    if err != nil {
        return nil, err
    }
    result := make([]*identity.FederatedIdentity, len(rows))
    for i, row := range rows {
        result[i] = toFederatedIdentity(row)
    }
    return result, nil
}
```

Uses the existing `federated_identities_user_id_idx` index from migration 1.

Define a `fiRow` struct mirroring the `federated_identities` columns (same pattern as
existing `userRow`), and a `toFederatedIdentity` mapping function.

#### New method `DeleteFederatedIdentity`

```go
func (r *UserRepository) DeleteFederatedIdentity(
    ctx context.Context, id, userID string,
) error {
    tx, err := r.db.BeginTxx(ctx, nil)
    if err != nil {
        return err
    }
    defer func() { _ = tx.Rollback() }()

    var count int
    if err := tx.QueryRowxContext(ctx,
        `SELECT COUNT(*) FROM federated_identities WHERE user_id = $1`, userID,
    ).Scan(&count); err != nil {
        return err
    }
    if count <= 1 {
        return domerr.ErrFailedPrecondition
    }

    result, err := tx.ExecContext(ctx,
        `DELETE FROM federated_identities WHERE id = $1 AND user_id = $2`, id, userID)
    if err != nil {
        return err
    }
    n, _ := result.RowsAffected()
    if n == 0 {
        return domerr.ErrNotFound
    }

    return tx.Commit()
}
```

**Why the guard is in the repo, not the service**: The SELECT COUNT + DELETE must be
atomic to prevent a TOCTOU race where two concurrent UnlinkProvider calls both pass
the count check but together remove all identities. Wrapping in a transaction
eliminates this race.

**Lock consideration**: On busy systems, `SELECT COUNT` + `DELETE` in a TX at READ
COMMITTED could still race. Use `SELECT COUNT(*) ... FOR UPDATE` to hold a row lock
on the identity being deleted:
```sql
SELECT COUNT(*) FROM federated_identities WHERE user_id = $1 FOR UPDATE
```

#### New method `UpdateLocalProfile`

```go
func (r *UserRepository) UpdateLocalProfile(
    ctx context.Context,
    userID string,
    name, picture *string,
    updatedAt time.Time,
) error {
    _, err := r.db.ExecContext(ctx,
        `UPDATE users
         SET local_name    = CASE WHEN $2 THEN $3 ELSE local_name    END,
             local_picture = CASE WHEN $4 THEN $5 ELSE local_picture END,
             updated_at    = $6
         WHERE id = $1`,
        userID,
        name != nil, nullableString(name),
        picture != nil, nullableString(picture),
        updatedAt,
    )
    return err
}

// nullableString converts a *string to a sql.NullString.
// *string == nil → sql.NullString{Valid: false} (NULL)
// *string == ""  → sql.NullString{Valid: false} (NULL, clears override)
// *string == "x" → sql.NullString{String: "x", Valid: true}
func nullableString(s *string) sql.NullString {
    if s == nil || *s == "" {
        return sql.NullString{}
    }
    return sql.NullString{String: *s, Valid: true}
}
```

The `CASE WHEN $2 THEN $3 ELSE local_name END` pattern means:
- `$2 = false` (name==nil) → column is unchanged
- `$2 = true, $3 = NULL` (name=&"") → column becomes NULL (clears override)
- `$2 = true, $3 = "Alice"` → column becomes "Alice"

This handles all three update semantics without requiring dynamic SQL.

---

## 6. gRPC Package

**New directory**: `server/services/identity-service/internal/grpc/`

The package is named `grpcserver` to avoid shadowing the `google.golang.org/grpc`
import alias in calling code. Import alias in `main.go`: `grpcserver "...internal/grpc"`.

---

### 6a. `internal/grpc/interceptor.go`

Extracts and verifies the JWT from incoming gRPC metadata, then injects `userID`
and `tokenID` into the context.

```go
package grpcserver

import (
    "context"
    "crypto/rsa"
    "encoding/json"
    "strings"
    "time"

    jose "github.com/go-jose/go-jose/v4"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"

    oidcadapter "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc/adapter"
)

type contextKey int

const (
    ctxKeyUserID  contextKey = iota
    ctxKeyTokenID
)

func UserIDFromContext(ctx context.Context) string {
    v, _ := ctx.Value(ctxKeyUserID).(string)
    return v
}

func tokenIDFromContext(ctx context.Context) string {
    v, _ := ctx.Value(ctxKeyTokenID).(string)
    return v
}

type jwtClaims struct {
    Subject string `json:"sub"`
    Expiry  int64  `json:"exp"`
    Issuer  string `json:"iss"`
    TokenID string `json:"jti"`
}

// NewJWTAuthInterceptor returns a unary interceptor that validates the Bearer
// JWT in every incoming RPC.
func NewJWTAuthInterceptor(publicKeys *oidcadapter.PublicKeySet, issuer string) grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context, req any,
        _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
    ) (any, error) {
        rawToken, err := extractBearerToken(ctx)
        if err != nil {
            return nil, err
        }

        claims, err := verifyJWT(rawToken, publicKeys)
        if err != nil {
            return nil, status.Error(codes.Unauthenticated, "invalid token")
        }

        if claims.Issuer != issuer {
            return nil, status.Error(codes.Unauthenticated, "invalid issuer")
        }
        if time.Now().Unix() > claims.Expiry {
            return nil, status.Error(codes.Unauthenticated, "token expired")
        }
        if claims.Subject == "" {
            return nil, status.Error(codes.Unauthenticated, "missing sub claim")
        }

        ctx = context.WithValue(ctx, ctxKeyUserID, claims.Subject)
        ctx = context.WithValue(ctx, ctxKeyTokenID, claims.TokenID)
        return handler(ctx, req)
    }
}

func extractBearerToken(ctx context.Context) (string, error) {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return "", status.Error(codes.Unauthenticated, "missing metadata")
    }
    vals := md.Get("authorization")
    if len(vals) == 0 {
        return "", status.Error(codes.Unauthenticated, "missing authorization header")
    }
    raw := vals[0]
    const prefix = "Bearer "
    if !strings.HasPrefix(raw, prefix) {
        return "", status.Error(codes.Unauthenticated, "authorization must use Bearer scheme")
    }
    return strings.TrimPrefix(raw, prefix), nil
}

func verifyJWT(rawToken string, publicKeys *oidcadapter.PublicKeySet) (*jwtClaims, error) {
    tok, err := jose.ParseSigned(rawToken, []jose.SignatureAlgorithm{jose.RS256})
    if err != nil {
        return nil, err
    }

    // Match key by kid header.
    kid := ""
    if len(tok.Signatures) > 0 {
        kid = tok.Signatures[0].Header.KeyID
    }

    var payload []byte
    for _, k := range publicKeys.All() {
        if k.ID() != kid {
            continue
        }
        rsaPub, ok := k.Key().(*rsa.PublicKey)
        if !ok {
            continue
        }
        payload, err = tok.Verify(rsaPub)
        if err == nil {
            break
        }
    }
    if payload == nil {
        return nil, fmt.Errorf("no matching signing key for kid %q", kid)
    }

    var claims jwtClaims
    if err := json.Unmarshal(payload, &claims); err != nil {
        return nil, err
    }
    return &claims, nil
}
```

**Key points**:

- Uses `go-jose/go-jose/v4` which is already a direct dependency (`go.mod` line 8).
  No new import is needed.
- Iterates `publicKeys.All()` to support key rotation: current key + previous keys
  are all tried. The `kid` header picks the right key without brute-forcing.
- No DB lookup. If a revoked-but-not-expired token arrives, it will pass verification
  and the RPC will execute. This is an accepted trade-off (see section 9).
- `iss` check prevents tokens issued by other services from being used here.

**What to verify during implementation**: Confirm that `zitadel/oidc` embeds a `kid`
in the JWT header. Inspect an actual token at `/oauth/v2/token`. If `kid` is absent,
fall back to trying all public keys.

---

### 6b. `internal/grpc/errors.go`

```go
package grpcserver

import (
    "errors"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    "github.com/barn0w1/hss-science/server/services/identity-service/internal/pkg/domerr"
)

// domainStatus converts a domain error to a gRPC status error.
// Callers must return the result directly: return nil, domainStatus(err)
func domainStatus(err error) error {
    switch {
    case errors.Is(err, domerr.ErrNotFound):
        return status.Error(codes.NotFound, "not found")
    case errors.Is(err, domerr.ErrUnauthorized):
        return status.Error(codes.PermissionDenied, "permission denied")
    case errors.Is(err, domerr.ErrFailedPrecondition):
        return status.Error(codes.FailedPrecondition, err.Error())
    default:
        return status.Error(codes.Internal, "internal error")
    }
}
```

Note: `ErrAlreadyExists` is not wired yet because it is never produced by any
existing code. It should be mapped to `codes.AlreadyExists` if it ever gets used.

---

### 6c. `internal/grpc/server.go`

```go
package grpcserver

import (
    "google.golang.org/grpc"

    oidcadapter "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc/adapter"
    oidcdom "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc"
    "github.com/barn0w1/hss-science/server/services/identity-service/internal/identity"
    pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

// NewServer constructs the gRPC server with auth interceptor and all
// registered service implementations.
func NewServer(
    identitySvc      identity.Service,
    deviceSessionSvc oidcdom.DeviceSessionService,
    publicKeys       *oidcadapter.PublicKeySet,
    issuer           string,
) *grpc.Server {
    srv := grpc.NewServer(
        grpc.ChainUnaryInterceptor(
            NewJWTAuthInterceptor(publicKeys, issuer),
        ),
    )
    pb.RegisterAccountManagementServiceServer(srv, &Handler{
        identitySvc:      identitySvc,
        deviceSessionSvc: deviceSessionSvc,
    })
    return srv
}
```

**Why no `TokenService` in the handler**: No RPC currently needs it. `RevokeSession`
calls `DeviceSessionService.RevokeByID`, which internally deletes linked refresh
tokens (already implemented in `device_session_repo.go:97-101`). `TokenService` can
be added later if needed (e.g., a future "revoke all tokens for a specific client"
RPC).

**Why no separate error-mapping interceptor**: The domain error mapping is a single
function call at each handler site. An interceptor wrapper adds boilerplate for no
benefit at this scale. If the API grows to 20+ RPCs, a panic-recovery + error-mapping
interceptor becomes worthwhile.

---

### 6d. `internal/grpc/handler.go`

```go
package grpcserver

import (
    "context"
    "time"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"
    "google.golang.org/protobuf/types/known/emptypb"
    "google.golang.org/protobuf/types/known/timestamppb"

    oidcdom "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc"
    "github.com/barn0w1/hss-science/server/services/identity-service/internal/identity"
    pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

var _ pb.AccountManagementServiceServer = (*Handler)(nil)

type Handler struct {
    pb.UnimplementedAccountManagementServiceServer
    identitySvc      identity.Service
    deviceSessionSvc oidcdom.DeviceSessionService
}

// GetMyProfile

func (h *Handler) GetMyProfile(ctx context.Context, _ *pb.GetMyProfileRequest) (*pb.Profile, error) {
    userID := UserIDFromContext(ctx)
    user, err := h.identitySvc.GetUser(ctx, userID)
    if err != nil {
        return nil, domainStatus(err)
    }
    return userToProto(user), nil
}

// UpdateMyProfile

func (h *Handler) UpdateMyProfile(
    ctx context.Context, req *pb.UpdateMyProfileRequest,
) (*pb.Profile, error) {
    userID := UserIDFromContext(ctx)
    user, err := h.identitySvc.UpdateProfile(ctx, userID, req.Name, req.Picture)
    if err != nil {
        return nil, domainStatus(err)
    }
    return userToProto(user), nil
}

// ListLinkedProviders

func (h *Handler) ListLinkedProviders(
    ctx context.Context, _ *pb.ListLinkedProvidersRequest,
) (*pb.ListLinkedProvidersResponse, error) {
    userID := UserIDFromContext(ctx)
    fis, err := h.identitySvc.ListLinkedProviders(ctx, userID)
    if err != nil {
        return nil, domainStatus(err)
    }
    providers := make([]*pb.FederatedProviderInfo, len(fis))
    for i, fi := range fis {
        providers[i] = &pb.FederatedProviderInfo{
            IdentityId:    fi.ID,
            Provider:      fi.Provider,
            ProviderEmail: fi.ProviderEmail,
            LastLoginAt:   timestamppb.New(fi.LastLoginAt),
        }
    }
    return &pb.ListLinkedProvidersResponse{Providers: providers}, nil
}

// UnlinkProvider

func (h *Handler) UnlinkProvider(
    ctx context.Context, req *pb.UnlinkProviderRequest,
) (*emptypb.Empty, error) {
    if req.IdentityId == "" {
        return nil, status.Error(codes.InvalidArgument, "identity_id is required")
    }
    userID := UserIDFromContext(ctx)
    if err := h.identitySvc.UnlinkProvider(ctx, userID, req.IdentityId); err != nil {
        return nil, domainStatus(err)
    }
    return &emptypb.Empty{}, nil
}

// ListActiveSessions

func (h *Handler) ListActiveSessions(
    ctx context.Context, _ *pb.ListActiveSessionsRequest,
) (*pb.ListActiveSessionsResponse, error) {
    userID := UserIDFromContext(ctx)
    sessions, err := h.deviceSessionSvc.ListActiveByUserID(ctx, userID)
    if err != nil {
        return nil, domainStatus(err)
    }
    pbSessions := make([]*pb.Session, len(sessions))
    for i, s := range sessions {
        pbSessions[i] = &pb.Session{
            SessionId:  s.ID,
            DeviceName: s.DeviceName,
            IpAddress:  s.IPAddress,
            CreatedAt:  timestamppb.New(s.CreatedAt),
            LastUsedAt: timestamppb.New(s.LastUsedAt),
        }
    }
    return &pb.ListActiveSessionsResponse{Sessions: pbSessions}, nil
}

// RevokeSession

func (h *Handler) RevokeSession(
    ctx context.Context, req *pb.RevokeSessionRequest,
) (*emptypb.Empty, error) {
    if req.SessionId == "" {
        return nil, status.Error(codes.InvalidArgument, "session_id is required")
    }
    userID := UserIDFromContext(ctx)
    if err := h.deviceSessionSvc.RevokeByID(ctx, req.SessionId, userID); err != nil {
        return nil, domainStatus(err)
    }
    return &emptypb.Empty{}, nil
}

// RevokeAllOtherSessions

func (h *Handler) RevokeAllOtherSessions(
    ctx context.Context, req *pb.RevokeAllOtherSessionsRequest,
) (*emptypb.Empty, error) {
    userID := UserIDFromContext(ctx)
    sessions, err := h.deviceSessionSvc.ListActiveByUserID(ctx, userID)
    if err != nil {
        return nil, domainStatus(err)
    }
    for _, s := range sessions {
        if s.ID == req.CurrentSessionId {
            continue
        }
        // Best-effort: log but don't abort on individual revocation failure.
        _ = h.deviceSessionSvc.RevokeByID(ctx, s.ID, userID)
    }
    return &emptypb.Empty{}, nil
}

// --- helpers ---

func userToProto(u *identity.User) *pb.Profile {
    p := &pb.Profile{
        UserId:        u.ID,
        Email:         u.Email,
        EmailVerified: u.EmailVerified,
        Name:          u.Name, // already effective (service applied COALESCE)
        GivenName:     u.GivenName,
        FamilyName:    u.FamilyName,
        Picture:       u.Picture, // already effective
        NameIsLocal:   u.LocalName != nil && *u.LocalName != "",
        PictureIsLocal: u.LocalPicture != nil && *u.LocalPicture != "",
        CreatedAt:     timestamppb.New(u.CreatedAt),
        UpdatedAt:     timestamppb.New(u.UpdatedAt),
    }
    return p
}
```

**`RevokeByID` ownership check**: `device_session_repo.go:86-88` uses
`WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL` with a row-count check.
If the session doesn't belong to the authenticated user, or is already revoked,
it returns a wrapped `ErrNotFound`. The `domainStatus` mapper turns this into
`codes.NotFound`.

**`RevokeAllOtherSessions` best-effort design**: Errors on individual session
revocation are silently dropped. Rationale: showing a partial failure to the user
("2 of 3 sessions revoked") is worse UX than "all done". Failures here are unlikely
(DB connectivity issue during an established RPC). This can be improved with
structured logging if needed.

---

## 7. Config Changes

**File**: `server/services/identity-service/config/config.go`

Add `GRPCPort string` to the `Config` struct:

```go
type Config struct {
    Port     string
    GRPCPort string // NEW — default "50051", env GRPC_PORT
    ...
}
```

In `LoadFrom`:
```go
cfg := &Config{
    Port:     getFrom(src, "PORT", "8080"),
    GRPCPort: getFrom(src, "GRPC_PORT", "50051"), // NEW
    ...
}
```

`getFrom` with a default of `"50051"` means the field is always valid (never empty).
No validation beyond the non-empty guarantee from `getFrom` is needed for port values.

**Env var doc**: Add `GRPC_PORT` to `.env.example`:
```
GRPC_PORT=50051
```

---

## 8. `main.go` Wiring

### Import additions

```go
import (
    "net"
    ...
    grpcserver "github.com/barn0w1/hss-science/server/services/identity-service/internal/grpc"
)
```

### gRPC server construction and startup

After `signingKey` and `publicKeys` are created (line 94–95 of current `main.go`),
and after `identitySvc` and `deviceSessionSvc` are available:

```go
grpcSrv := grpcserver.NewServer(identitySvc, deviceSessionSvc, publicKeys, cfg.Issuer)

grpcListener, err := net.Listen("tcp", ":"+cfg.GRPCPort)
if err != nil {
    logger.Error("failed to listen on gRPC port", "error", err, "port", cfg.GRPCPort)
    os.Exit(1)
}

go func() {
    logger.Info("gRPC server starting", "port", cfg.GRPCPort)
    if err := grpcSrv.Serve(grpcListener); err != nil {
        logger.Error("gRPC server exited", "error", err)
    }
}()
```

Place this block **before** the shutdown wait (`<-quit`), alongside the existing
`go func() { srv.ListenAndServe() }()`.

### Graceful shutdown

Replace the current shutdown block:

```go
// current
if err := srv.Shutdown(shutdownCtx); err != nil {
    ...
}
```

With parallel shutdown:

```go
// Stop gRPC server first (it's in-memory, fast to drain).
grpcSrv.GracefulStop()

// Then drain HTTP server within the 15-second window.
if err := srv.Shutdown(shutdownCtx); err != nil {
    logger.Error("HTTP server forced to shutdown", "error", err)
    os.Exit(1)
}
```

`grpc.Server.GracefulStop()` blocks until all in-flight RPCs complete or the server
is forcibly stopped. For account management RPCs (single DB reads/writes), in-flight
drain is effectively instant. Running it before `srv.Shutdown` is safe because the
gRPC and HTTP stacks are independent.

**Alternative**: run both shutdowns in goroutines and `sync.WaitGroup` against the
15s `shutdownCtx`. For this service at current scale, sequential is simpler.

---

## 9. Build Steps

After writing all source files:

### Step 1: `buf generate` (from repo root)

```sh
cd hss-science/
buf generate
```

This produces:
```
server/gen/accounts/v1/account_management.pb.go
server/gen/accounts/v1/account_management_grpc.pb.go
```

The import path used in Go code: `github.com/barn0w1/hss-science/server/gen/accounts/v1`.
This is part of the same `github.com/barn0w1/hss-science/server` module (`server/` is
the module root per `go.mod`), so no module boundary crossing.

**Why `buf.gen.yaml` needs no changes**: It already targets `api/proto` and outputs to
`server/gen` with `paths=source_relative`. The new proto at
`api/proto/accounts/v1/account_management.proto` fits this layout exactly.

### Step 2: `go mod tidy` (from `server/`)

```sh
cd hss-science/server/
go mod tidy
```

This promotes:
- `google.golang.org/grpc` from `// indirect` to a direct dependency
- `google.golang.org/protobuf` from `// indirect` to a direct dependency

Both are already resolved in `go.sum`. No compatibility issues expected —
`grpc v1.79.1` is already present.

---

## 10. Trade-offs Considered

### 10.1 Token revocation window (chosen: accept it)

The interceptor does not check the DB whether the token has been explicitly revoked.
A user who calls `POST /oauth/v2/revoke` (revoking their access token) will still be
able to call gRPC RPCs with that token until it expires (up to 15 minutes by default).

- **Accepted**: Account management operations (profile read/update, session list) are
  low-risk. The 15-minute window is short.
- **Mitigated**: `RevokeSession` also deletes all linked refresh tokens, preventing
  token refresh. The access token window is bounded.
- **Alternative for higher security**: Add a lightweight DB lookup in the interceptor
  (`tokenRepo.GetByID(jti)`) to confirm the token row exists. This adds ~1 ms DB
  latency per RPC. Not implemented until required.

### 10.2 No pagination on `ListActiveSessions` (chosen: defer)

`DeviceSessionRepository.ListActiveByUserID` returns all rows. For a normal user this
is < 10 sessions. A cursor-based page token would add proto complexity with minimal
benefit today. Noted as a v2 improvement.

### 10.3 Profile picture URL validation (chosen: not validated)

`UpdateMyProfile.picture` accepts any string. If an attacker can set the picture URL
and the frontend renders it in an `<img>` tag, it could be used for tracking pixels
or SSRF depending on the client. Recommendation for the BFF: validate that the URL
is an allowed domain (e.g., from a configurable allowlist) before passing to the gRPC
API. Validation inside the accounts service would require configuring the allowlist
there, which is tight coupling. Left as BFF responsibility.

### 10.4 `given_name`/`family_name` not user-editable (chosen: upstream wins)

The design decision says "local edits win" for **display name** and picture only.
`given_name` and `family_name` are structural identity attributes (used for name
matching, invoice generation, etc.) that should reflect the upstream provider's truth.
If this needs to change, add `local_given_name`/`local_family_name` columns following
the same pattern.

### 10.5 `local_name = ""` clears the override (chosen: symmetry)

Passing an empty string for `name` in `UpdateMyProfileRequest` resets the user's
local display name to null (falling back to the upstream name). This is intentional
design: an empty display name would look broken in the UI, so it's treated as "revert
to default" instead of "set an empty name". The BFF should present this as a "Reset
to provider name" action, not as a free-form field that allows blank names.

### 10.6 gRPC-only, no HTTP gateway (chosen: match requirement)

`grpc-ecosystem/grpc-gateway/v2` is already in `go.mod` as an indirect dependency.
Adding `google.api.http` annotations to the proto and wiring the gateway would expose
an HTTP REST API on the same port. Not implemented — the BFF communicates over gRPC.
Annotations can be added later without a breaking proto change.

### 10.7 Same port for ops and gRPC (chosen: separate port)

Using a single port with a mux that demultiplexes gRPC (HTTP/2) from OIDC
(HTTP/1.1 + HTTP/2) would require `cmux`. The zitadel `op.Provider` handler does
not advertise `"h2"` ALPN, so a demux by Content-Type (`application/grpc`) on a
single `net.Listener` is technically feasible but operationally harder (one listener,
two routing layers). A dedicated port is simpler, clearer in firewall rules, and
allows rate limiting to be applied independently. Chosen approach: `:50051` for gRPC,
`:8080` for OIDC/HTTP.

### 10.8 `UnlinkProvider` SELECT FOR UPDATE (recommended guard)

The last-provider guard uses `SELECT COUNT(*) ... FOR UPDATE`. This acquires a row
lock on the `federated_identities` rows for this user, preventing concurrent
`DELETE` statements from both passing the count check in parallel. The downside is
slightly increased lock contention. For a self-service account management API with
human-paced requests, this is acceptable.

---

## 11. Implementation Order

1. **Migration 4** — no code changes, safe to run on existing DB.
2. **domerr** — add `ErrFailedPrecondition` (1-line change, no risk).
3. **identity domain + ports** — pure interface changes, no breaking changes.
4. **identity/postgres** — repo method additions + `UpdateUserFromClaims` SQL update.
5. **identity service** — implement new methods + `applyLocalOverrides` in `GetUser`.
6. **Proto + buf generate** — create proto, run buf, commit generated files.
7. **grpc package** — new package with interceptor, errors, server, handler.
8. **config** — add `GRPCPort`.
9. **main.go** — wire gRPC server + listener + shutdown.
10. **go mod tidy** — promote grpc/protobuf to direct deps.
11. **Tests** — integration tests for each RPC using existing testcontainers helper.

Each step compiles independently. Steps 1–5 can be done in a branch without any gRPC
code yet. The gRPC surface lands in steps 6–9.

---

## 12. Implementation Checklist

### Phase 1 — Database Migration
- [x] Create `migrations/4_local_profile.up.sql` — `ALTER TABLE users ADD COLUMN local_name TEXT DEFAULT NULL, ADD COLUMN local_picture TEXT DEFAULT NULL`

### Phase 2 — Domain Errors
- [x] Add `ErrFailedPrecondition` sentinel to `internal/pkg/domerr/errors.go`

### Phase 3 — Identity Domain Model
- [x] Add `LocalName *string` and `LocalPicture *string` fields to `identity.User` in `internal/identity/domain.go`

### Phase 4 — Identity Ports
- [x] Add `ListFederatedIdentities`, `DeleteFederatedIdentity`, `UpdateLocalProfile` to `identity.Repository` in `internal/identity/ports.go`
- [x] Add `UpdateProfile`, `ListLinkedProviders`, `UnlinkProvider` to `identity.Service` in `internal/identity/ports.go`

### Phase 5 — Identity Postgres Repo
- [x] Add `LocalName *string` and `LocalPicture *string` to `userRow` struct in `user_repo.go`
- [x] Update `toUser` to populate `LocalName` and `LocalPicture`
- [x] Update `GetByID` SELECT query to include `local_name, local_picture`
- [x] Update `FindByFederatedIdentity` SELECT query to include `u.local_name, u.local_picture`
- [x] Update `UpdateUserFromClaims` SQL to use `CASE WHEN local_name IS NULL` guard
- [x] Implement `ListFederatedIdentities` method
- [x] Implement `DeleteFederatedIdentity` method (TX with `SELECT COUNT ... FOR UPDATE` guard)
- [x] Implement `UpdateLocalProfile` method (CASE WHEN $N THEN $M ELSE col END pattern)
- [x] Add `fiRow` struct and `toFederatedIdentity` helper for the new federated identity methods

### Phase 6 — Identity Service  _(build + test after)_
- [x] Add `applyLocalOverrides` helper function
- [x] Modify `GetUser` to call `applyLocalOverrides` before returning
- [x] Implement `UpdateProfile` service method
- [x] Implement `ListLinkedProviders` service method
- [x] Implement `UnlinkProvider` service method
- [x] `go build ./...` from `server/` — must pass
- [x] `go test ./...` from `server/` — must pass

### Phase 7 — Protobuf
- [x] Create directory `api/proto/accounts/v1/`
- [x] Write `api/proto/accounts/v1/account_management.proto` (service + all message types)
- [x] Run `buf generate` from repo root — produces `server/gen/accounts/v1/*.go`

### Phase 8 — Config
- [x] Add `GRPCPort string` field to `config.Config`
- [x] Wire `GRPCPort: getFrom(src, "GRPC_PORT", "50051")` in `LoadFrom`

### Phase 9 — gRPC Package
- [x] Create `internal/grpc/interceptor.go` — JWT auth unary interceptor + context key helpers
- [x] Create `internal/grpc/errors.go` — `domainStatus` error mapper
- [x] Create `internal/grpc/server.go` — `NewServer` factory
- [x] Create `internal/grpc/handler.go` — `Handler` implementing all 7 RPCs + `userToProto` helper

### Phase 10 — main.go Wiring  _(build + test after)_
- [x] Add `net` import and `grpcserver` import alias to `main.go`
- [x] Construct gRPC server and TCP listener after `publicKeys`/`identitySvc` are available
- [x] Start gRPC server in a goroutine
- [x] Add `grpcSrv.GracefulStop()` before `srv.Shutdown` in the shutdown block
- [x] Run `go mod tidy` from `server/` to promote grpc + protobuf to direct deps
- [x] `go build ./...` from `server/` — must pass
- [x] `go test ./...` from `server/` — must pass

### Phase 11 — Integration Tests  _(build + test after)_
- [ ] Create `internal/grpc/handler_test.go` — integration tests using testcontainers Postgres helper
- [ ] Test `GetMyProfile` — happy path and not-found
- [ ] Test `UpdateMyProfile` — set name, clear name, set picture
- [ ] Test `ListLinkedProviders` — returns correct federated identities
- [ ] Test `UnlinkProvider` — success path; last-provider guard returns FailedPrecondition
- [ ] Test `ListActiveSessions` — returns active device sessions
- [ ] Test `RevokeSession` — revokes correct session; wrong user returns NotFound
- [ ] Test `RevokeAllOtherSessions` — correct session excluded
- [ ] Test JWT interceptor — missing token, expired token, wrong issuer
- [ ] `go build ./...` from `server/` — must pass
- [ ] `go test ./...` from `server/` — must pass
