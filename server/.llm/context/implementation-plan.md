# Blob Service — Implementation Plan

**Author:** Lead Architect
**Date:** 2026-03-19
**Ref:** `server/.llm/context/architecture.md`
**Status:** Ready for Implementation

This document is the step-by-step execution plan derived from the approved architecture design. Each phase produces a discrete, reviewable artifact. Phases must be executed in order; within a phase, individual files can be written in parallel.

---

## Dependency Map

```
Phase 0 (deps)
    └─► Phase 1 (proto + gen)
            └─► Phase 2 (scaffold + config)
                    └─► Phase 3 (domain layer)
                            ├─► Phase 4 (repository)
                            ├─► Phase 5 (storage)
                            └─► Phase 6 (app layer)  ◄── requires 4 + 5
                                    └─► Phase 7 (transport)
                                            └─► Phase 8 (main.go)
Phase M (migrations)  ← independent, can run alongside Phase 3
Phase T (tests)       ← requires all implementation phases
```

---

## Phase 0 — New Go Dependencies

The existing `server/go.mod` does not include an AWS SDK. The S3 storage adapter requires it.

**Dependencies to add via `go get`:**

| Package | Purpose |
|---------|---------|
| `github.com/aws/aws-sdk-go-v2/aws` | Core AWS types |
| `github.com/aws/aws-sdk-go-v2/config` | AWS config loading |
| `github.com/aws/aws-sdk-go-v2/credentials` | Static credentials (R2 key/secret) |
| `github.com/aws/aws-sdk-go-v2/service/s3` | S3 client |
| `github.com/aws/aws-sdk-go-v2/service/s3/s3manager` | Multipart helpers (informational only — presigned URLs are generated manually) |

**Command:**
```bash
cd server && go get \
  github.com/aws/aws-sdk-go-v2/aws \
  github.com/aws/aws-sdk-go-v2/config \
  github.com/aws/aws-sdk-go-v2/credentials \
  github.com/aws/aws-sdk-go-v2/service/s3
```

No other new dependencies are required. The following already present in `go.mod` are consumed:

| Existing Package | Used By |
|-----------------|---------|
| `github.com/coreos/go-oidc/v3` | Auth interceptor (JWT/JWKS validation) |
| `github.com/jmoiron/sqlx` | PostgreSQL repository |
| `github.com/lib/pq` | PostgreSQL driver |
| `github.com/google/uuid` | _Not used_ — CAS uses SHA-256, not UUID |
| `google.golang.org/grpc` | gRPC server and interceptors |
| `google.golang.org/protobuf` | Protobuf generated types |
| `github.com/stretchr/testify` | Test assertions |
| `github.com/testcontainers/testcontainers-go` | Integration test containers |
| `github.com/testcontainers/testcontainers-go/modules/postgres` | PostgreSQL container for integration tests |

---

## Phase 1 — Protobuf Definition & Code Generation

### 1.1 File: `api/proto/blob/v1/blob.proto`

Create the file at the path above. Key decisions:

- **Package:** `blob.v1`
- **Go package option:** `github.com/barn0w1/hss-science/server/gen/blob/v1;blobv1`
- **Import:** `google/protobuf/timestamp.proto` for all time fields
- **Enum:** `UploadState { UPLOAD_STATE_UNSPECIFIED = 0; PENDING = 1; COMMITTED = 2; }`

**Messages to define (one per RPC pair, plus shared types):**

```
// Shared
message PartUploadURL    { int32 part_number = 1; string presigned_put_url = 2; }
message CompletedPart    { int32 part_number = 1; string etag = 2; }

// RPCs
InitiateUploadRequest / InitiateUploadResponse
CompleteUploadRequest / CompleteUploadResponse
InitiateMultipartUploadRequest / InitiateMultipartUploadResponse
CompleteMultipartUploadRequest / CompleteMultipartUploadResponse
AbortMultipartUploadRequest / AbortMultipartUploadResponse
GetDownloadURLRequest / GetDownloadURLResponse
GetBlobInfoRequest / GetBlobInfoResponse
```

Full field specs are in `architecture.md §3.2`. Reproduce them exactly in proto syntax.

**Service block name:** `BlobService` with all 7 RPCs from `architecture.md §3.1`.

### 1.2 Generate

```bash
# from repo root
buf generate
```

Verify that `server/gen/blob/v1/` is created containing:
- `blob.pb.go` (message types)
- `blob_grpc.pb.go` (server/client interfaces)

**Do not hand-edit generated files.**

---

## Phase 2 — Directory Scaffold & Configuration

### 2.1 Directory Structure

Create the full directory tree (empty files acceptable at this stage):

```
server/services/blob-service/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── domain/
│   │   ├── blob.go
│   │   ├── errors.go
│   │   ├── repository.go
│   │   └── storage.go
│   ├── app/
│   │   └── blob_app.go
│   ├── transport/
│   │   └── grpc/
│   │       ├── server.go
│   │       └── interceptor/
│   │           └── auth.go
│   ├── repository/
│   │   └── postgres/
│   │       └── blob_repo.go
│   └── storage/
│       └── s3/
│           └── r2_client.go
├── migrations/
│   ├── 001_create_blobs_table.up.sql
│   └── 001_create_blobs_table.down.sql
└── config/
    └── config.go
```

### 2.2 File: `config/config.go`

Parse all environment variables with `os.Getenv`. Fail fast at startup if required variables are missing.

```go
type Config struct {
    GRPCListenAddr         string        // GRPC_LISTEN_ADDR, default ":50052"
    DatabaseURL            string        // DATABASE_URL, required
    OIDCIssuerURL          string        // OIDC_ISSUER_URL, required
    R2Endpoint             string        // R2_ENDPOINT, required
    R2Bucket               string        // R2_BUCKET, required
    R2AccessKeyID          string        // R2_ACCESS_KEY_ID, required
    R2SecretAccessKey      string        // R2_SECRET_ACCESS_KEY, required
    PresignPutTTL          time.Duration // PRESIGN_PUT_TTL_SECONDS, default 900s
    PresignGetMaxTTL       time.Duration // PRESIGN_GET_TTL_MAX_SECONDS, default 3600s
    MultipartThresholdBytes int64        // MULTIPART_THRESHOLD_BYTES, default 10485760
}

func Load() (*Config, error) { ... }
```

Use `strconv.ParseInt` / `strconv.ParseDuration` for numeric fields. Return a descriptive error listing all missing required variables at once (do not fail on first missing).

---

## Phase 3 — Domain Layer

The domain layer has **zero external imports** (only stdlib). It defines types, interfaces, and invariants.

### 3.1 File: `internal/domain/blob.go`

```go
type BlobID string  // 64-char lowercase hex SHA-256

func (id BlobID) Validate() error  // check len == 64 and all chars are hex

type UploadState string
const (
    StatePending   UploadState = "PENDING"
    StateCommitted UploadState = "COMMITTED"
)

type Blob struct {
    ID          BlobID
    SizeBytes   int64
    ContentType string
    R2Key       string       // invariant: always == string(ID)
    State       UploadState
    CreatedAt   time.Time
    CommittedAt *time.Time
}

// NewBlob constructs a new PENDING blob. Enforces R2Key == ID invariant.
func NewBlob(id BlobID, sizeBytes int64, contentType string, now time.Time) (*Blob, error)

// Commit transitions PENDING → COMMITTED. Returns ErrAlreadyCommitted if already committed.
func (b *Blob) Commit(at time.Time) error
```

### 3.2 File: `internal/domain/errors.go`

Define sentinel errors used across all layers:

```go
var (
    ErrBlobNotFound       = errors.New("blob not found")
    ErrAlreadyCommitted   = errors.New("blob already committed")
    ErrInvalidBlobID      = errors.New("invalid blob_id: must be 64-char lowercase hex")
    ErrBlobPending        = errors.New("blob is in PENDING state")
)
```

gRPC status mapping happens in the transport layer; domain errors are plain Go errors.

### 3.3 File: `internal/domain/repository.go`

```go
type BlobRepository interface {
    FindByID(ctx context.Context, id BlobID) (*Blob, error)  // ErrBlobNotFound if absent
    Create(ctx context.Context, b *Blob) error
    MarkCommitted(ctx context.Context, id BlobID, at time.Time) error
}
```

### 3.4 File: `internal/domain/storage.go`

```go
type CompletedPart struct {
    PartNumber int32
    ETag       string
}

type ObjectStorage interface {
    PresignedPutURL(ctx context.Context, key string, ttl time.Duration) (url string, expiresAt time.Time, err error)
    PresignedGetURL(ctx context.Context, key string, ttl time.Duration) (url string, expiresAt time.Time, err error)
    CreateMultipartUpload(ctx context.Context, key, contentType string) (uploadID string, err error)
    PresignedPartURL(ctx context.Context, key, uploadID string, partNumber int32, ttl time.Duration) (url string, expiresAt time.Time, err error)
    CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []CompletedPart) error
    AbortMultipartUpload(ctx context.Context, key, uploadID string) error
}
```

---

## Phase 4 — PostgreSQL Repository Adapter

### File: `internal/repository/postgres/blob_repo.go`

Implement `domain.BlobRepository` using `sqlx`.

**Constructor:**
```go
func New(db *sqlx.DB) *BlobRepo
```

**Implementation notes:**

- `FindByID`: `SELECT * FROM blobs WHERE id = $1`. Map `sql.ErrNoRows` → `domain.ErrBlobNotFound`.
- `Create`: `INSERT INTO blobs (id, size_bytes, content_type, r2_key, state, created_at) VALUES (...)`. On duplicate key (`pq.ErrorCode == "23505"`), return `domain.ErrAlreadyCommitted` if the existing row is COMMITTED, or a silent no-op / re-query if PENDING (caller handles idempotency at the app layer).
- `MarkCommitted`: `UPDATE blobs SET state = 'COMMITTED', committed_at = $2 WHERE id = $1 AND state = 'PENDING'`. Check `RowsAffected() == 0` → return `domain.ErrAlreadyCommitted`.

**DB connection helper** (can live in `cmd/server/main.go` or a shared `internal/db` package):
```go
func Open(dsn string) (*sqlx.DB, error) {
    db, err := sqlx.Open("postgres", dsn)
    // set connection pool limits
    db.SetMaxOpenConns(25)
    db.SetMaxIdleConns(5)
    db.SetConnMaxLifetime(5 * time.Minute)
    return db, err
}
```

---

## Phase 5 — S3 / R2 Storage Adapter

### File: `internal/storage/s3/r2_client.go`

Implement `domain.ObjectStorage` using `aws-sdk-go-v2/service/s3`.

**Constructor:**
```go
func New(endpoint, bucket, accessKeyID, secretKey string) (*R2Client, error)
```

R2 requires a **custom endpoint resolver** — the standard AWS endpoint must be overridden with the R2 URL. Use `aws.EndpointResolverWithOptionsFunc` or the v2 SDK's `BaseEndpoint` option on the S3 client.

**Credentials:** `credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, "")` — no session token.

**Presigned URL generation:**
- Use `s3.NewPresignClient(s3Client)`
- `PresignedPutURL` → `presignClient.PresignPutObject(...)`
- `PresignedGetURL` → `presignClient.PresignGetObject(...)`
- `PresignedPartURL` → `presignClient.PresignUploadPart(...)`

**Multipart:**
- `CreateMultipartUpload` → `s3Client.CreateMultipartUpload(...)` → return `*output.UploadId`
- `CompleteMultipartUpload` → `s3Client.CompleteMultipartUpload(...)` with `types.CompletedMultipartUpload`
- `AbortMultipartUpload` → `s3Client.AbortMultipartUpload(...)`

**Error handling:** Wrap AWS SDK errors as `fmt.Errorf("r2: %w", err)`. The app layer treats all storage errors as transient (`codes.Unavailable`).

---

## Phase 6 — Application Layer

### File: `internal/app/blob_app.go`

This layer orchestrates use cases. It depends only on `domain` interfaces — never on concrete adapters.

**Constructor:**
```go
type App struct {
    repo    domain.BlobRepository
    storage domain.ObjectStorage
    cfg     AppConfig  // presign TTLs, multipart threshold
}

type AppConfig struct {
    PresignPutTTL          time.Duration
    PresignGetMaxTTL       time.Duration
    MultipartThresholdBytes int64
}

func New(repo domain.BlobRepository, storage domain.ObjectStorage, cfg AppConfig) *App
```

**Methods (one per gRPC RPC):**

#### `InitiateUpload`
```
1. Validate blob_id (BlobID.Validate())
2. repo.FindByID(id)
   - COMMITTED → return (alreadyExists=true, no URL)
   - PENDING   → generate fresh presigned PUT URL (idempotent re-issue)
   - NotFound  → create new Blob (NewBlob), repo.Create, generate presigned PUT URL
3. Return presigned URL + expiry
```

#### `CompleteUpload`
```
1. Validate blob_id
2. repo.FindByID → must exist, must be PENDING (else FAILED_PRECONDITION)
3. repo.MarkCommitted(id, now)
4. Return blob_id + committed_at
```

#### `InitiateMultipartUpload`
```
1. Validate blob_id, part_count ≥ 1
2. repo.FindByID(id)
   - COMMITTED → return (alreadyExists=true)
   - PENDING   → skip create; generate new multipart session (see below)
   - NotFound  → repo.Create(blob PENDING)
3. storage.CreateMultipartUpload(id, contentType) → uploadID
4. For each part [1..part_count]: storage.PresignedPartURL(id, uploadID, partNumber, ttl)
5. Return uploadID + slice of PartUploadURL
```

#### `CompleteMultipartUpload`
```
1. Validate blob_id
2. repo.FindByID → must exist and be PENDING
3. storage.CompleteMultipartUpload(id, uploadID, parts)
4. repo.MarkCommitted(id, now)
5. Return blob_id + committed_at
```

#### `AbortMultipartUpload`
```
1. storage.AbortMultipartUpload(id, uploadID)
   (do NOT delete the DB record — the blob may be re-uploaded)
2. Return empty success
```

#### `GetDownloadURL`
```
1. Validate blob_id
2. repo.FindByID → must be COMMITTED (else FAILED_PRECONDITION)
3. Cap ttl_seconds to PresignGetMaxTTL
4. storage.PresignedGetURL(id, ttl)
5. Return URL + expiry
```

#### `GetBlobInfo`
```
1. Validate blob_id
2. repo.FindByID → return full Blob mapped to response proto
```

---

## Phase 7 — gRPC Transport Layer

### 7.1 File: `internal/transport/grpc/interceptor/auth.go`

Implement a gRPC **UnaryServerInterceptor** (and a matching **StreamServerInterceptor** wrapping it).

**Logic:**
```
1. Extract metadata from ctx: md, ok := metadata.FromIncomingContext(ctx)
2. Get md["authorization"][0] → strip "Bearer " prefix
3. oidcProvider.Verifier(&oidc.Config{ClientID: "blob-service"}).Verify(ctx, rawToken)
   → returns *oidc.IDToken or error
4. Extract "sub" claim from token; store in ctx via a context key
5. On any failure: return nil, status.Error(codes.Unauthenticated, "...")
```

**OIDC provider setup** (happens once at startup in main.go):
```go
provider, err := oidc.NewProvider(ctx, cfg.OIDCIssuerURL)
// provider is passed into the interceptor constructor
```

The interceptor struct holds the `*oidc.Provider` and the `ClientID` (`"blob-service"`).

**Context key type:** define an unexported `contextKey` type to avoid collision with other packages.

```go
type contextKey struct{}
var callerSubKey = contextKey{}

func CallerSub(ctx context.Context) string { ... }
```

### 7.2 File: `internal/transport/grpc/server.go`

Implement the generated `blobv1.BlobServiceServer` interface.

**Constructor:**
```go
type Server struct {
    app *app.App
}
func NewServer(app *app.App) *Server
```

**Handler pattern for each RPC:**
```
1. Validate request fields (non-empty blob_id, etc.) → codes.InvalidArgument
2. Call app method
3. Map domain errors to gRPC status codes:
   - domain.ErrBlobNotFound       → codes.NotFound
   - domain.ErrAlreadyCommitted   → codes.FailedPrecondition
   - domain.ErrInvalidBlobID      → codes.InvalidArgument
   - domain.ErrBlobPending        → codes.FailedPrecondition
   - all other errors             → codes.Internal (log the underlying error)
4. Map domain result to proto response
```

**Proto timestamp conversion helper:**
```go
import "google.golang.org/protobuf/types/known/timestamppb"
timestamppb.New(t)   // time.Time → *timestamppb.Timestamp
```

---

## Phase 8 — Database Migrations

Migration files live in `server/services/blob-service/migrations/` and follow the `golang-migrate` naming convention: `NNN_<description>.up.sql` / `NNN_<description>.down.sql`.

### File: `migrations/001_create_blobs_table.up.sql`

```sql
CREATE TABLE blobs (
    id           CHAR(64)    PRIMARY KEY,
    size_bytes   BIGINT      NOT NULL,
    content_type TEXT        NOT NULL DEFAULT '',
    r2_key       CHAR(64)    NOT NULL,
    state        TEXT        NOT NULL DEFAULT 'PENDING'
                             CHECK (state IN ('PENDING', 'COMMITTED')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    committed_at TIMESTAMPTZ
);
```

### File: `migrations/001_create_blobs_table.down.sql`

```sql
DROP TABLE IF EXISTS blobs;
```

---

## Phase 9 — Main Entry Point

### File: `cmd/server/main.go`

Wire all layers together. Order of initialization:

```
1.  Load config (config.Load() — fail fast on missing vars)
2.  Initialize OIDC provider (oidc.NewProvider(ctx, cfg.OIDCIssuerURL))
3.  Open PostgreSQL connection (sqlx.Open + db.PingContext)
4.  Build repository (postgres.New(db))
5.  Build storage adapter (s3.New(cfg.R2Endpoint, cfg.R2Bucket, ...))
6.  Build app layer (app.New(repo, storage, appCfg))
7.  Build gRPC server (grpc.NewServer with interceptors)
8.  Register service (blobv1.RegisterBlobServiceServer(grpcServer, transport.NewServer(app)))
9.  net.Listen("tcp", cfg.GRPCListenAddr)
10. grpcServer.Serve(listener)
11. Handle OS signals (SIGINT, SIGTERM) → grpcServer.GracefulStop()
```

**Interceptor chain:** auth interceptor wraps the server. Use `grpc.ChainUnaryInterceptor` and `grpc.ChainStreamInterceptor`.

**Logging:** use `log/slog` (stdlib, Go 1.21+). Structured JSON output at startup and on each request (log the caller `sub` from context, the RPC method, and duration).

---

## Phase T — Tests

### T.1 Testing Philosophy

| Layer | Test Type | Isolation |
|-------|-----------|-----------|
| `internal/domain` | Unit | Pure Go, no external deps |
| `internal/app` | Unit | Mock `BlobRepository` + `ObjectStorage` |
| `internal/repository/postgres` | Integration | Real PostgreSQL via testcontainers |
| `internal/transport/grpc` | Integration | In-process gRPC server, mock app |
| `internal/storage/s3` | Integration | _(see note below)_ |

> **S3 integration tests:** The existing `go.mod` does not include a MinIO or LocalStack testcontainer module. For this iteration, the `ObjectStorage` interface is mocked in app-layer tests. The `r2_client.go` adapter itself is tested by verifying the presigned URL shape / SDK calls via a test-mode S3 client pointing to a locally spun-up MinIO container using `testcontainers-go` with the generic `testcontainers.GenericContainer` API (MinIO image). This is optional for the first iteration — mark with `//go:build integration` build tag and skip in CI if MinIO image is unavailable.

### T.2 Unit Tests — Domain

**File:** `internal/domain/blob_test.go`

Test cases:
- `TestBlobID_Validate`: valid 64-char hex passes; 63-char, uppercase, non-hex fail
- `TestNewBlob`: constructs correctly; enforces `R2Key == ID`; rejects invalid BlobID
- `TestBlob_Commit`: PENDING → COMMITTED transitions; second Commit returns `ErrAlreadyCommitted`

Libraries: `github.com/stretchr/testify/assert` and `testify/require`.

### T.3 Unit Tests — Application Layer

**File:** `internal/app/blob_app_test.go`

Define mock implementations of `domain.BlobRepository` and `domain.ObjectStorage` inline (no mock generation framework — keep it simple, implement the interfaces manually with recorded calls).

**Test cases per use-case:**

`InitiateUpload`:
- Blob not found → creates record, returns presigned URL
- Blob COMMITTED → returns `already_exists: true`, no URL
- Blob PENDING → returns fresh presigned URL (idempotent)
- Invalid blob_id → returns `ErrInvalidBlobID`

`CompleteUpload`:
- Happy path: PENDING → COMMITTED
- Blob not found → propagates `ErrBlobNotFound`
- Already committed → propagates `ErrAlreadyCommitted`

`InitiateMultipartUpload`:
- Not found → creates record, calls `CreateMultipartUpload`, returns N part URLs
- Already committed → returns `already_exists: true`
- `part_count = 0` → returns error

`CompleteMultipartUpload`:
- Happy path: calls `storage.CompleteMultipartUpload`, then `repo.MarkCommitted`
- Storage failure → repo not marked committed (transactional intent)

`GetDownloadURL`:
- COMMITTED → returns URL with capped TTL
- PENDING → returns `ErrBlobPending`
- ttl_seconds exceeding ceiling → capped to `PresignGetMaxTTL`

### T.4 Integration Tests — PostgreSQL Repository

**File:** `internal/repository/postgres/blob_repo_test.go`

Use `testcontainers-go/modules/postgres` to spin up a real PostgreSQL instance. Apply the migration SQL from `migrations/001_create_blobs_table.up.sql` in the test setup using `db.ExecContext`.

**Test cases:**
- `TestCreate_and_FindByID`: create a blob, find it by ID
- `TestFindByID_NotFound`: returns `domain.ErrBlobNotFound`
- `TestMarkCommitted`: transitions state, sets `committed_at`
- `TestMarkCommitted_AlreadyCommitted`: returns `domain.ErrAlreadyCommitted`
- `TestCreate_Idempotency`: inserting same `blob_id` twice — second insert behaviour

Use `t.Parallel()` at the test function level (each test gets its own schema or truncates the table in setup).

### T.5 Integration Tests — gRPC Handler

**File:** `internal/transport/grpc/server_test.go`

Stand up an in-process gRPC server using `google.golang.org/grpc/test/bufconn` (buffer connection — no real TCP port). Inject a mock `*app.App` or use the real app wired to a test PostgreSQL container.

**Auth interceptor:** In handler tests, bypass auth by injecting a test interceptor that always succeeds and sets a dummy `sub` in context. Test auth separately in `interceptor/auth_test.go`.

**Test cases:**
- `TestInitiateUpload_gRPC`: end-to-end gRPC call, verify proto response shape
- `TestGetBlobInfo_NotFound`: verify `codes.NotFound` is returned
- `TestGetDownloadURL_Pending`: verify `codes.FailedPrecondition`
- Repeat happy-path for each of the 7 RPCs

**Auth interceptor tests** (`interceptor/auth_test.go`):
- Missing token → `codes.Unauthenticated`
- Malformed bearer string → `codes.Unauthenticated`
- Expired JWT → `codes.Unauthenticated`
- Valid JWT, wrong audience → `codes.Unauthenticated`
- Valid JWT → handler called, `sub` in context

For auth tests, use a local OIDC test server (generate a test RSA key pair, serve a mock JWKS endpoint on `httptest.NewServer`). The `coreos/go-oidc` library will discover the JWKS from the test server URL.

---

## Implementation Order Summary

| # | Phase | Output |
|---|-------|--------|
| 0 | Add AWS SDK v2 to go.mod | Updated `go.mod` / `go.sum` |
| 1 | Write proto, run buf generate | `api/proto/blob/v1/blob.proto`, `server/gen/blob/v1/` |
| 2 | Scaffold directories + config | Directory tree, `config/config.go` |
| 3 | Domain layer | `internal/domain/*.go` (4 files) |
| M | Migration files | `migrations/001_*.up.sql`, `*.down.sql` |
| 4 | PostgreSQL repository | `internal/repository/postgres/blob_repo.go` |
| 5 | S3 storage adapter | `internal/storage/s3/r2_client.go` |
| 6 | Application layer | `internal/app/blob_app.go` |
| 7 | gRPC transport | `internal/transport/grpc/server.go`, `interceptor/auth.go` |
| 8 | Main entry point | `cmd/server/main.go` |
| T | Tests | `*_test.go` files throughout |

---

## Checklist for "Done"

- [ ] `buf generate` produces `server/gen/blob/v1/` without errors
- [ ] `go build ./services/blob-service/...` passes with no warnings
- [ ] `go vet ./services/blob-service/...` passes cleanly
- [ ] All domain unit tests pass (`go test ./services/blob-service/internal/domain/...`)
- [ ] All app unit tests pass (`go test ./services/blob-service/internal/app/...`)
- [ ] PostgreSQL integration tests pass (`go test ./services/blob-service/internal/repository/...`)
- [ ] gRPC handler integration tests pass (`go test ./services/blob-service/internal/transport/...`)
- [ ] No binary file data is ever read or written by blob-service (architecture invariant)
- [ ] `CAS invariant: R2Key == BlobID` is enforced in `NewBlob` and asserted in tests
- [ ] All gRPC errors use the correct status codes from `architecture.md §3.3`
- [ ] `migrations/001_create_blobs_table.up.sql` and `.down.sql` exist and are valid SQL
