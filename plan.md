# MyAccount gRPC Module — Implementation Plan

> **Goal:** Add a `myaccount` gRPC API module inside the existing `accounts` service
> (modular monolith), consumed by the `myaccount-bff`.
>
> **This document is a plan only. Nothing is implemented yet.**

---

## 1. Scope — RPC Surface

The API mirrors the backend of a "myaccount.google.com"-style page.
All RPCs operate on **the authenticated caller's own account** (no admin-for-another-user).

| RPC | Purpose | Existing code reused |
|---|---|---|
| `GetProfile` | Return user's personal info | `identity.Service.GetUser` (as-is) |
| `ListLinkedAccounts` | Return federated identity entries for the user | **New** `identity.Repository.ListFederatedIdentities(ctx, userID)` |
| `ListSessions` | Return active refresh-token sessions for the user | **New** `oidc.TokenRepository.ListRefreshTokensByUser(ctx, userID)` |
| `RevokeSession` | Delete one refresh token + its access token | `oidc.TokenService.DeleteByUserAndClient` (close; **new** `RevokeByRefreshTokenID` needed — see §5) |
| `RevokeAllOtherSessions` | Delete all sessions except the caller's current one | **New** repo + service method |

That is five RPCs — enough for a functional "my account" page. Further RPCs (profile edit, account deletion, unlink provider) are out of scope for this phase.

---

## 2. Proto Definition

File: `api/proto/myaccount/v1/myaccount.proto`

```
api/proto/myaccount/v1/
└── myaccount.proto
```

### Proto sketch

```protobuf
syntax = "proto3";

package myaccount.v1;

option go_package = "github.com/barn0w1/hss-science/server/gen/myaccount/v1;myaccountv1";

import "google/protobuf/timestamp.proto";

service MyAccountService {
  rpc GetProfile(GetProfileRequest)                   returns (GetProfileResponse);
  rpc ListLinkedAccounts(ListLinkedAccountsRequest)    returns (ListLinkedAccountsResponse);
  rpc ListSessions(ListSessionsRequest)               returns (ListSessionsResponse);
  rpc RevokeSession(RevokeSessionRequest)              returns (RevokeSessionResponse);
  rpc RevokeAllOtherSessions(RevokeAllOtherSessionsRequest) returns (RevokeAllOtherSessionsResponse);
}

// ---------- GetProfile ----------

message GetProfileRequest {}

message GetProfileResponse {
  string user_id        = 1;
  string email          = 2;
  bool   email_verified = 3;
  string name           = 4;
  string given_name     = 5;
  string family_name    = 6;
  string picture        = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
}

// ---------- ListLinkedAccounts ----------

message ListLinkedAccountsRequest {}

message LinkedAccount {
  string id                      = 1;
  string provider                = 2;
  string provider_email          = 3;
  bool   provider_email_verified = 4;
  string provider_display_name   = 5;
  string provider_picture_url    = 6;
  google.protobuf.Timestamp last_login_at = 7;
  google.protobuf.Timestamp created_at    = 8;
}

message ListLinkedAccountsResponse {
  repeated LinkedAccount accounts = 1;
}

// ---------- ListSessions ----------

message ListSessionsRequest {}

message Session {
  string id        = 1;  // refresh token ID (opaque to client)
  string client_id = 2;
  repeated string scopes = 3;
  google.protobuf.Timestamp auth_time  = 4;
  google.protobuf.Timestamp expiration = 5;
  google.protobuf.Timestamp created_at = 6;
  bool is_current = 7;
}

message ListSessionsResponse {
  repeated Session sessions = 1;
}

// ---------- RevokeSession ----------

message RevokeSessionRequest {
  string session_id = 1;  // refresh token ID
}

message RevokeSessionResponse {}

// ---------- RevokeAllOtherSessions ----------

message RevokeAllOtherSessionsRequest {}

message RevokeAllOtherSessionsResponse {
  int64 revoked_count = 1;
}
```

### Code generation

`buf generate` (via `make gen-proto`) will output to:

```
server/gen/myaccount/v1/
├── myaccount.pb.go
└── myaccount_grpc.pb.go
```

This matches the existing `buf.gen.yaml`:
- `buf.build/protocolbuffers/go` → structs
- `buf.build/grpc/go` → gRPC server/client stubs

---

## 3. New Internal Package: `internal/myaccount/`

Following the established hexagonal pattern:

```
internal/myaccount/
├── handler.go      # gRPC handler implementing MyAccountServiceServer
└── handler_test.go
```

This package is thin — a **transport adapter**. It depends on:
- `identity.Service` (existing)
- `identity.Repository` (for new `ListFederatedIdentities` — see §5)
- `oidc.TokenService` (existing + extended — see §5)

### Handler struct

```go
type Handler struct {
    myaccountv1.UnimplementedMyAccountServiceServer
    identitySvc  identity.Service
    identityRepo identity.Repository   // for ListFederatedIdentities
    tokenSvc     oidcdom.TokenService   // extended with new methods
    logger       *slog.Logger
}
```

### Error mapping convention

Consistent with the existing `domerr` → protocol error pattern:

| `domerr` sentinel | gRPC status code |
|---|---|
| `ErrNotFound` | `codes.NotFound` |
| `ErrUnauthorized` | `codes.Unauthenticated` |
| `ErrAlreadyExists` | `codes.AlreadyExists` |
| `ErrInternal` / other | `codes.Internal` |

Implement this as a small helper function in the handler file (not a separate package).

---

## 4. Authentication — gRPC Interceptor

File: `internal/grpcauth/interceptor.go`

The interceptor:
1. Extracts `authorization` from gRPC metadata (`Bearer <token_id>`)
2. Calls `oidc.TokenService.GetByID(ctx, tokenID)` — the existing method already validates expiration
3. On success, injects the resolved `Subject` (user ID) **and** `tokenID` into context via `context.WithValue`
4. On failure, returns `codes.Unauthenticated`

```
internal/grpcauth/
├── interceptor.go
├── interceptor_test.go
└── context.go        # UserIDFromContext(ctx), TokenIDFromContext(ctx) helpers
```

The `tokenID` is injected into context so that `RevokeAllOtherSessions` can identify the caller's current session and exclude it.

### Why a separate package

The interceptor needs to be reused by any future gRPC service within accounts (not just myaccount). Putting it in `internal/grpcauth/` keeps it module-agnostic while still internal to the accounts service.

---

## 5. Changes to Existing Code

### 5a. `identity.Repository` — add `ListFederatedIdentities`

**File:** `internal/identity/ports.go` — add one method to the interface:

```go
ListFederatedIdentities(ctx context.Context, userID string) ([]*FederatedIdentity, error)
```

**File:** `internal/identity/postgres/user_repo.go` — implement:

```sql
SELECT id, user_id, provider, provider_subject,
       provider_email, provider_email_verified,
       provider_display_name, provider_given_name,
       provider_family_name, provider_picture_url,
       last_login_at, created_at, updated_at
FROM federated_identities
WHERE user_id = $1
ORDER BY created_at
```

The existing `federated_identities_user_id_idx` index already covers this query.

### 5b. `oidc.TokenRepository` — add session listing and targeted revocation

**File:** `internal/oidc/ports.go` — add to `TokenRepository`:

```go
ListRefreshTokensByUser(ctx context.Context, userID string) ([]*RefreshToken, error)
RevokeRefreshTokenByID(ctx context.Context, refreshTokenID, userID string) error
RevokeAllRefreshTokensByUser(ctx context.Context, userID string, excludeRefreshTokenID string) (int64, error)
```

**File:** `internal/oidc/ports.go` — add to `TokenService`:

```go
ListSessionsByUser(ctx context.Context, userID string) ([]*RefreshToken, error)
RevokeSessionByID(ctx context.Context, refreshTokenID, userID string) error
RevokeAllOtherSessions(ctx context.Context, userID, currentRefreshTokenID string) (int64, error)
```

**File:** `internal/oidc/postgres/token_repo.go` — implement:

`ListRefreshTokensByUser`:
```sql
SELECT id, token_hash, client_id, user_id, audience, scopes,
       auth_time, amr, access_token_id, expiration, created_at
FROM refresh_tokens
WHERE user_id = $1 AND expiration > now()
ORDER BY created_at DESC
```

An index on `refresh_tokens(user_id)` will be needed (see §6).

`RevokeRefreshTokenByID` (in a transaction):
```sql
-- lookup the associated access token
SELECT access_token_id FROM refresh_tokens WHERE id = $1 AND user_id = $2
-- delete access token
DELETE FROM tokens WHERE id = $1
-- delete refresh token
DELETE FROM refresh_tokens WHERE id = $1 AND user_id = $2
```

`RevokeAllRefreshTokensByUser` (in a transaction):
```sql
-- delete associated access tokens
DELETE FROM tokens
WHERE id IN (
    SELECT access_token_id FROM refresh_tokens
    WHERE user_id = $1 AND id != $2
)
-- delete refresh tokens
DELETE FROM refresh_tokens WHERE user_id = $1 AND id != $2
```

**File:** `internal/oidc/token_svc.go` — implement new service methods as thin wrappers.

### 5c. `oidc.TokenService.GetByID` — map the caller's current session

The interceptor calls `GetByID(tokenID)` and gets back `*oidc.Token` which has `RefreshTokenID`. This field is set when the access token was issued alongside a refresh token, allowing `RevokeAllOtherSessions` to know which `refresh_token.id` to exclude.

No changes needed here — the existing `Token.RefreshTokenID` field already carries this information.

---

## 6. Database Migration

File: `migrations/3_myaccount.up.sql`

```sql
-- Index to support ListRefreshTokensByUser (session listing)
CREATE INDEX refresh_tokens_user_id_idx
    ON refresh_tokens (user_id) WHERE expiration > now();
```

File: `migrations/3_myaccount.down.sql`

```sql
DROP INDEX IF EXISTS refresh_tokens_user_id_idx;
```

No new tables are needed. The only DDL change is one partial index.

---

## 7. gRPC Server Wiring in `main.go`

### New config fields

Add to `config.Config`:

```go
GRPCPort string  // env: GRPC_PORT, default "50051"
```

### Server startup

In `runServer()`, after the HTTP server setup:

```go
// gRPC server
grpcSrv := grpc.NewServer(
    grpc.UnaryInterceptor(grpcauth.UnaryServerInterceptor(tokenSvc)),
)
myaccountv1.RegisterMyAccountServiceServer(grpcSrv, myaccount.NewHandler(
    identitySvc,
    identityRepo,  // note: the repo must be exposed, not just the service
    tokenSvc,
    logger,
))

lis, _ := net.Listen("tcp", ":"+cfg.GRPCPort)
go grpcSrv.Serve(lis)
```

### identityRepo exposure

Currently `main.go` constructs `identitypg.NewUserRepository(db)` and immediately passes it to `identity.NewService()`. For the myaccount handler, the repo instance also needs to be available:

```go
identityRepo := identitypg.NewUserRepository(db)
identitySvc  := identity.NewService(identityRepo)
// ...later...
myaccount.NewHandler(identitySvc, identityRepo, tokenSvc, logger)
```

This is a minimal change — the repo was already being created, it just needs to be bound to a variable used in two places.

### Graceful shutdown

Add `grpcSrv.GracefulStop()` alongside the existing `srv.Shutdown()`:

```go
cleanupCancel()
grpcSrv.GracefulStop()
srv.Shutdown(shutdownCtx)
```

---

## 8. New Go Dependencies

```
google.golang.org/grpc       # already in go.mod as indirect via zitadel
google.golang.org/protobuf   # already in go.mod as indirect
```

Promote both from `// indirect` to direct by adding:

```go
import (
    "google.golang.org/grpc"
    myaccountv1 "github.com/barn0w1/hss-science/server/gen/myaccount/v1"
)
```

No new external dependencies required.

---

## 9. File Inventory — What Gets Created / Modified

### New files

| File | Purpose |
|---|---|
| `api/proto/myaccount/v1/myaccount.proto` | Proto definition |
| `server/gen/myaccount/v1/myaccount.pb.go` | Generated (buf) |
| `server/gen/myaccount/v1/myaccount_grpc.pb.go` | Generated (buf) |
| `server/services/accounts/internal/myaccount/handler.go` | gRPC handler |
| `server/services/accounts/internal/myaccount/handler_test.go` | Tests |
| `server/services/accounts/internal/grpcauth/interceptor.go` | Auth interceptor |
| `server/services/accounts/internal/grpcauth/context.go` | Context helpers |
| `server/services/accounts/internal/grpcauth/interceptor_test.go` | Tests |
| `server/services/accounts/migrations/3_myaccount.up.sql` | Index migration |
| `server/services/accounts/migrations/3_myaccount.down.sql` | Rollback |

### Modified files

| File | Change |
|---|---|
| `server/services/accounts/internal/identity/ports.go` | Add `ListFederatedIdentities` to `Repository` interface |
| `server/services/accounts/internal/identity/postgres/user_repo.go` | Implement `ListFederatedIdentities` |
| `server/services/accounts/internal/oidc/ports.go` | Add 3 methods to `TokenRepository`, 3 methods to `TokenService` |
| `server/services/accounts/internal/oidc/token_svc.go` | Implement 3 new service methods |
| `server/services/accounts/internal/oidc/postgres/token_repo.go` | Implement 3 new repo methods |
| `server/services/accounts/config/config.go` | Add `GRPCPort` field |
| `server/services/accounts/.env.example` | Add `GRPC_PORT` |
| `server/services/accounts/main.go` | Wire gRPC server, register handler, graceful shutdown |
| `server/go.mod` / `server/go.sum` | Promote grpc/protobuf to direct dependencies |
| `server/services/accounts/testhelper/testdb.go` | No change needed (existing tables unchanged) |

---

## 10. Implementation Order

Each step results in code that compiles and passes tests before moving to the next.

1. **Proto + codegen** — write `myaccount.proto`, run `buf generate`, verify generated Go code compiles.
2. **Migration** — write `3_myaccount.up.sql` / `3_myaccount.down.sql`.
3. **Extend `identity.Repository`** — add `ListFederatedIdentities` to interface and Postgres implementation. Write test.
4. **Extend `oidc.TokenRepository` / `TokenService`** — add session listing and revocation methods. Write tests.
5. **Auth interceptor** — implement `internal/grpcauth/`. Write test with mock `TokenService`.
6. **gRPC handler** — implement `internal/myaccount/handler.go` with all 5 RPCs. Write tests with mock services.
7. **Wire in `main.go`** — add config, gRPC listener, registration, graceful shutdown.
8. **Manual smoke test** — start service, hit RPCs with `grpcurl`.

---

## 11. Design Decisions & Rationale

| Decision | Rationale |
|---|---|
| Separate gRPC port (not HTTP/gRPC mux) | Simpler deployment; avoids `cmux` complexity; gRPC port can have different network policy (internal-only) |
| Handler depends on `identity.Repository` (not just `Service`) | `identity.Service` has no `ListFederatedIdentities` use case, and wrapping a simple query in a service method adds no logic — the handler can call the repo directly for reads |
| Session = refresh token row | A "session" in this context means "a grant that can produce new access tokens". The refresh token is the ground truth for session liveness |
| `is_current` computed in handler, not DB | The handler knows the caller's `tokenID` → `RefreshTokenID` from context; matching it against the list is trivial in-memory |
| No `UpdateProfile` in this phase | User profile data is currently sourced from upstream IdPs during login. Allowing local edits introduces conflicting-truth complexity. Deferring to a later phase |
| Auth interceptor in `internal/grpcauth/` (not `internal/middleware/`) | `middleware/` is HTTP-specific (chi). gRPC interceptors are a different abstraction. Separate package avoids confusion |
| No new domain package for myaccount | The myaccount module is purely a read-heavy composition of existing domains (`identity`, `oidc`). It doesn't introduce its own domain objects or business rules — a handler package is sufficient |
