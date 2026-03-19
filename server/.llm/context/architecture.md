# Blob Service — Domain Architecture Design Document

**Author:** Lead Architect
**Date:** 2026-03-19
**Branch:** `feat/blob-service`
**Status:** Approved for Implementation

---

## 1. Purpose and Context Boundary

### 1.1 What blob-service Is

`blob-service` is the **sole, platform-wide Blobstore** for HSS-Science. It is a pure infrastructure service: it stores and retrieves opaque byte sequences identified by their content hash. It has no knowledge of business domains.

| blob-service KNOWS | blob-service DOES NOT KNOW |
|--------------------|---------------------------|
| SHA-256 hash (blob identity) | What the blob represents (image, document, avatar…) |
| `size_bytes`, `content_type` | Which user uploaded it |
| Upload state (pending / committed) | Which application it belongs to |
| Presigned URL TTLs | Relationships between blobs |

Callers (BFFs, domain services) own all business semantics. They store blob SHA-256 references in their own databases alongside their domain entities. blob-service is only ever queried to issue or validate presigned URLs and to record/confirm raw object existence.

### 1.2 Context Boundary Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                      HSS-Science Platform                        │
│                                                                  │
│  ┌──────────────┐   ┌──────────────┐   ┌──────────────────────┐ │
│  │  chat-service│   │ drive-service│   │   (future services)  │ │
│  └──────┬───────┘   └──────┬───────┘   └──────────┬───────────┘ │
│         │  gRPC (JWT)      │  gRPC (JWT)           │  gRPC (JWT) │
│         └──────────────────┴───────────────────────┘            │
│                            │                                     │
│              ┌─────────────▼──────────────┐                     │
│              │        blob-service         │  ◄── This document  │
│              │  (metadata + URL issuance)  │                     │
│              └──────────┬─────────────────┘                     │
│                         │  SQL                                   │
│              ┌──────────▼──────┐                                 │
│              │   PostgreSQL    │                                  │
│              │  (blob metadata)│                                  │
│              └─────────────────┘                                 │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                  accounts.hss-science.org                 │   │
│  │           (OIDC Provider — JWKS endpoint)                 │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘

       Client (browser)
           │   ▲
           │   │  Direct PUT / GET (Presigned URL — no server in path)
           ▼   │
  ┌─────────────────────┐
  │   Cloudflare R2     │
  │  (S3-compatible)    │
  └─────────────────────┘
```

**Critical invariant:** No binary file data ever flows through `blob-service` or any BFF. All I/O between clients and R2 is direct via presigned URLs.

---

## 2. Foundational Concepts (Non-Negotiable)

These three concepts form the immutable foundation of the design. No alternative shall be introduced.

### 2.1 CAS — Content-Addressable Storage

The SHA-256 hash of the raw file content is the **blob identity** (`blob_id`). This single decision enables:

- **Platform-wide deduplication**: If two callers upload the same file, R2 stores exactly one object.
- **Integrity verification**: R2 / the S3 API can verify the ETag or SHA-256 checksum on receipt.
- **Immutability**: A blob, once committed, never changes. Changing content means a different `blob_id`.

`blob_id` format: lowercase hex-encoded SHA-256, 64 characters. Example:
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`

The caller (e.g., `drive-service`) **must compute the SHA-256 client-side** before calling `InitiateUpload`. blob-service trusts this value and uses it as the R2 object key.

### 2.2 S3 Presigned URLs and Multipart Upload

All data transfer uses the unmodified S3 API specification against Cloudflare R2:

- **Single-part upload**: A presigned `PUT` URL is returned to the caller, which passes it to the client. The client performs one `HTTP PUT` directly to R2.
- **Multipart upload**: For objects above the threshold (configurable; default 10 MiB, minimum 5 MiB per S3 spec), blob-service orchestrates:
  1. `CreateMultipartUpload` → R2 issues an `UploadId`
  2. A presigned URL is generated for each part
  3. Caller passes part URLs to the client; client uploads parts directly
  4. `CompleteMultipartUpload` assembles the object server-side in R2
- **Download**: A presigned `GET` URL is generated on demand and returned. blob-service never proxies the response body.

### 2.3 OIDC M2M Authentication (Zero Trust)

blob-service exposes no user-facing endpoints. Every inbound gRPC call must carry a JWT Bearer token (in gRPC metadata: `authorization: Bearer <token>`) issued for a **service account** by `accounts.hss-science.org`.

Validation is performed via standard OIDC JWKS discovery — the same library (`github.com/coreos/go-oidc/v3`) already used in the platform. No custom token format or shared-secret scheme is permitted.

Claims required on every inbound JWT:

| Claim | Constraint |
|-------|-----------|
| `iss` | `https://accounts.hss-science.org` |
| `aud` | `blob-service` |
| `exp` | must not be in the past |
| `sub` | identifies the calling service (e.g., `chat-service`, `drive-service`) |

---

## 3. gRPC API Surface

**Package path:** `api/proto/blob/v1/blob.proto`
**Generated Go output:** `server/gen/blob/v1/`

### 3.1 Service Definition (Intent)

```
service BlobService {
  // --- Upload lifecycle ---
  rpc InitiateUpload        (InitiateUploadRequest)        returns (InitiateUploadResponse);
  rpc CompleteUpload        (CompleteUploadRequest)        returns (CompleteUploadResponse);

  // --- Multipart upload lifecycle ---
  rpc InitiateMultipartUpload  (InitiateMultipartUploadRequest)  returns (InitiateMultipartUploadResponse);
  rpc CompleteMultipartUpload  (CompleteMultipartUploadRequest)  returns (CompleteMultipartUploadResponse);
  rpc AbortMultipartUpload     (AbortMultipartUploadRequest)     returns (AbortMultipartUploadResponse);

  // --- Query ---
  rpc GetDownloadURL        (GetDownloadURLRequest)        returns (GetDownloadURLResponse);
  rpc GetBlobInfo           (GetBlobInfoRequest)           returns (GetBlobInfoResponse);
}
```

### 3.2 Key Message Contracts (Intent, not final proto syntax)

**`InitiateUploadRequest`**
- `blob_id` (string) — SHA-256 hex
- `size_bytes` (int64)
- `content_type` (string)

**`InitiateUploadResponse`**
- `already_exists` (bool) — if `true`, the blob is already committed; no upload needed
- `presigned_put_url` (string) — only set if `already_exists == false`
- `url_expires_at` (google.protobuf.Timestamp)

**`CompleteUploadRequest`**
- `blob_id` (string)

**`CompleteUploadResponse`**
- `blob_id` (string)
- `committed_at` (google.protobuf.Timestamp)

**`InitiateMultipartUploadRequest`**
- `blob_id` (string) — SHA-256 hex
- `size_bytes` (int64)
- `content_type` (string)
- `part_count` (int32)

**`InitiateMultipartUploadResponse`**
- `already_exists` (bool)
- `upload_id` (string) — R2 multipart UploadId (opaque)
- `parts` (repeated PartUploadURL) — `{ part_number: int32, presigned_put_url: string }`
- `url_expires_at` (google.protobuf.Timestamp)

**`CompleteMultipartUploadRequest`**
- `blob_id` (string)
- `upload_id` (string)
- `parts` (repeated CompletedPart) — `{ part_number: int32, etag: string }`

**`GetDownloadURLRequest`**
- `blob_id` (string)
- `ttl_seconds` (int32) — caller hints desired TTL; service applies a ceiling (e.g., 3600 s)

**`GetDownloadURLResponse`**
- `presigned_get_url` (string)
- `url_expires_at` (google.protobuf.Timestamp)

**`GetBlobInfoRequest`**
- `blob_id` (string)

**`GetBlobInfoResponse`**
- `blob_id` (string)
- `size_bytes` (int64)
- `content_type` (string)
- `upload_state` (enum: PENDING | COMMITTED)
- `committed_at` (google.protobuf.Timestamp, nullable)

### 3.3 gRPC Error Codes

| Situation | gRPC Status |
|-----------|-------------|
| JWT absent or malformed | `UNAUTHENTICATED` |
| JWT valid but service not in allowed list | `PERMISSION_DENIED` |
| `blob_id` not found in PostgreSQL | `NOT_FOUND` |
| `blob_id` in PENDING state when COMMITTED required | `FAILED_PRECONDITION` |
| `blob_id` malformed (not 64-char hex) | `INVALID_ARGUMENT` |
| R2 / S3 transient error | `UNAVAILABLE` |

---

## 4. Internal Architecture

### 4.1 Hexagonal Structure

blob-service follows a strict **Ports & Adapters (Hexagonal)** layout. The `domain` core has zero imports from transport or infrastructure packages.

```
server/services/blob-service/
├── cmd/
│   └── server/
│       └── main.go              # Wiring: DI, config, server startup
├── internal/
│   ├── domain/
│   │   ├── blob.go              # Blob aggregate + state machine (PENDING→COMMITTED)
│   │   └── service.go           # Domain service interface (port)
│   ├── app/
│   │   └── blob_app.go          # Application use-case orchestration (implements domain service)
│   ├── transport/
│   │   └── grpc/
│   │       ├── server.go        # gRPC handler implementations
│   │       └── interceptor/
│   │           └── auth.go      # OIDC JWT validation interceptor (unary + stream)
│   ├── repository/
│   │   └── postgres/
│   │       └── blob_repo.go     # PostgreSQL adapter (implements BlobRepository port)
│   └── storage/
│       └── s3/
│           └── r2_client.go     # S3-compatible adapter (presigned URL generation, multipart)
└── config/
    └── config.go                # Env-based config (no hardcoded values)
```

**Dependency direction:** `transport` → `app` → `domain` ← `repository` / `storage`

`domain` defines interfaces (`BlobRepository`, `ObjectStorage`); `repository` and `storage` implement them. `app` depends only on `domain` interfaces.

### 4.2 Domain Model

#### Blob Aggregate

```
Blob {
  ID          BlobID       // SHA-256 hex, 64 chars (the primary key)
  SizeBytes   int64
  ContentType string
  R2Key       string       // == ID (CAS: content hash is the storage key)
  State       UploadState  // PENDING | COMMITTED
  CreatedAt   time.Time
  CommittedAt *time.Time
}

UploadState = PENDING | COMMITTED
```

#### State Machine

```
        InitiateUpload / InitiateMultipartUpload
                │
                ▼
           ┌─────────┐
           │ PENDING │
           └────┬────┘
                │  CompleteUpload / CompleteMultipartUpload
                ▼
          ┌───────────┐
          │ COMMITTED │  (terminal — immutable by CAS contract)
          └───────────┘
```

A `COMMITTED` blob is **never re-uploaded**. `InitiateUpload` with an existing committed `blob_id` returns `already_exists: true` immediately.

#### Port Interfaces

```go
// BlobRepository — implemented by postgres adapter
type BlobRepository interface {
    FindByID(ctx context.Context, id BlobID) (*Blob, error)
    Create(ctx context.Context, b *Blob) error
    MarkCommitted(ctx context.Context, id BlobID, at time.Time) error
}

// ObjectStorage — implemented by s3/r2 adapter
type ObjectStorage interface {
    PresignedPutURL(ctx context.Context, key string, ttl time.Duration) (string, time.Time, error)
    PresignedGetURL(ctx context.Context, key string, ttl time.Duration) (string, time.Time, error)
    CreateMultipartUpload(ctx context.Context, key, contentType string) (uploadID string, err error)
    PresignedPartURL(ctx context.Context, key, uploadID string, partNumber int, ttl time.Duration) (string, time.Time, error)
    CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []CompletedPart) error
    AbortMultipartUpload(ctx context.Context, key, uploadID string) error
}
```

### 4.3 Auth Interceptor

A single gRPC **unary + streaming server interceptor** handles Zero Trust authentication:

1. Extract `authorization` metadata from the incoming gRPC context.
2. Strip the `Bearer ` prefix to get the raw JWT.
3. Validate the JWT using `coreos/go-oidc/v3` with JWKS auto-refresh:
   - Verify signature against JWKS from `https://accounts.hss-science.org/.well-known/jwks.json`
   - Verify `iss`, `aud` (`blob-service`), `exp`
4. Extract `sub` claim and store the calling service identity in the request context (for audit logging).
5. Return `UNAUTHENTICATED` or `PERMISSION_DENIED` on failure; otherwise call the next handler.

The interceptor is **stateless per request** and relies entirely on JWKS caching provided by the oidc library (keys are fetched once and refreshed on key-rotation signals).

---

## 5. Data Model (PostgreSQL)

### 5.1 Table: `blobs`

```sql
CREATE TABLE blobs (
    id           CHAR(64)     PRIMARY KEY,           -- SHA-256 hex (CAS identity)
    size_bytes   BIGINT       NOT NULL,
    content_type TEXT         NOT NULL DEFAULT '',
    r2_key       CHAR(64)     NOT NULL,               -- == id (CAS invariant)
    state        TEXT         NOT NULL DEFAULT 'PENDING'
                              CHECK (state IN ('PENDING', 'COMMITTED')),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now(),
    committed_at TIMESTAMPTZ
);
```

No foreign keys to any other service's tables. This table is entirely self-contained.

### 5.2 Design Notes

- `id` is `CHAR(64)` (fixed-length), not `UUID`. CAS mandates content-derived identity.
- `r2_key` is always equal to `id` (enforced at the application layer). It is stored explicitly to allow future migration to a different key scheme without schema breakage.
- `state` uses a text enum for readability in pg_dump / migration tools; the CHECK constraint is the enforcement.
- No soft-delete or versioning: CAS blobs are immutable and eternal (garbage collection policy is out of scope for this document).
- Index on `state` is optional for now; the primary access pattern is PK lookup.

### 5.3 Migration Strategy

Migrations are managed by plain `.sql` files (no ORM migration framework). Files are named `NNNN_description.sql` and applied in order. The blob-service owns its schema; no other service may reference `blobs` via foreign key.

> Annotation: We use `golang-migrate` managed by an external system. You MUST use paired `.up.sql` and `.down.sql` files (e.g., `001_create_blobs_table.up.sql` and `001_create_blobs_table.down.sql`).These files MUST be placed strictly in the `server/services/blob-service/migrations/` directory.

---

## 6. Configuration

All configuration is injected via environment variables. No config files are committed to the repository.

| Variable | Description | Example |
|----------|-------------|---------|
| `GRPC_LISTEN_ADDR` | gRPC bind address | `:50052` |
| `DATABASE_URL` | PostgreSQL DSN | `postgres://user:pass@host/blob` |
| `OIDC_ISSUER_URL` | OIDC Provider base URL | `https://accounts.hss-science.org` |
| `R2_ENDPOINT` | S3-compatible endpoint | `https://<account>.r2.cloudflarestorage.com` |
| `R2_BUCKET` | R2 bucket name | `hss-blob-store` |
| `R2_ACCESS_KEY_ID` | S3 access key | — |
| `R2_SECRET_ACCESS_KEY` | S3 secret key | — |
| `PRESIGN_PUT_TTL_SECONDS` | Upload URL TTL (default 900) | `900` |
| `PRESIGN_GET_TTL_MAX_SECONDS` | Maximum download URL TTL (ceiling for caller hint) | `3600` |
| `MULTIPART_THRESHOLD_BYTES` | Minimum size to trigger multipart (default 10 MiB) | `10485760` |

---

## 7. Upload Data Flow (End-to-End)

### 7.1 Single-Part Upload

```
Client (browser)                  drive-service                  blob-service          R2
      │                                │                               │                │
      │  1. User selects file          │                               │                │
      │  ── compute SHA-256 ──►        │                               │                │
      │                                │ 2. InitiateUpload(sha256,     │                │
      │                                │    size, content_type) ──────►│                │
      │                                │                               │ 3. Check DB    │
      │                                │                               │    (blob_id)   │
      │                                │                               │ 4. Insert PENDING
      │                                │                               │ 5. PresignPUT ─►
      │                                │                               │◄── presigned URL
      │                                │◄── {presigned_put_url} ───────│                │
      │◄── {presigned_put_url} ────────│                               │                │
      │                                │                               │                │
      │  6. PUT file bytes ────────────────────────────────────────────────────────────►│
      │◄── 200 OK ─────────────────────────────────────────────────────────────────────│
      │                                │                               │                │
      │  7. notify upload done ───────►│                               │                │
      │                                │ 8. CompleteUpload(sha256) ───►│                │
      │                                │                               │ 9. UPDATE state
      │                                │                               │    → COMMITTED │
      │                                │◄── {committed_at} ────────────│                │
```

### 7.2 Already-Exists (Deduplication) Path

When `InitiateUpload` is called with a `blob_id` that is already `COMMITTED`:

```
drive-service               blob-service
      │                          │
      │ InitiateUpload(sha256…) ►│
      │                          │ SELECT → COMMITTED
      │◄── {already_exists:true} │
      │  (no presigned URL)      │
      │                          │
      │ (store sha256 reference  │
      │  in drive-service DB)    │
```

No R2 interaction occurs. The blob is referenced by its hash alone.

### 7.3 Download

```
Client (browser)          chat-service            blob-service          R2
      │                       │                        │                 │
      │  request attachment ─►│                        │                 │
      │                       │ GetDownloadURL(sha256)►│                 │
      │                       │                        │ PresignGET ────►│
      │                       │                        │◄── presigned URL│
      │                       │◄── {presigned_get_url} │                 │
      │◄── redirect / URL ────│                        │                 │
      │                        GET ──────────────────────────────────────►│
      │◄────────────────────────── file bytes ──────────────────────────── │
```

---

## 8. Key Architectural Decisions and Rationale

| Decision | Rationale |
|----------|-----------|
| SHA-256 as primary key (CAS) | Enables platform-wide deduplication with zero coordination. Content identity is universal and deterministic. |
| gRPC only (no REST/HTTP) | Internal M2M communication only. gRPC provides strong typing via Protobuf, efficient binary framing, and native streaming. |
| No data proxying | Eliminates blob-service as a bandwidth bottleneck. Server resources remain proportional to metadata volume, not file volume. |
| PostgreSQL for metadata | Transactional guarantees for the PENDING→COMMITTED state transition prevent double-uploads and phantom references. |
| OIDC JWKS validation | Standard, auditable, key-rotation-safe. No shared secrets to manage or rotate manually. |
| No domain semantics in blob-service | Enforces the context boundary. Domain services own meaning; blob-service owns bytes. Coupling is one-directional. |
| Multipart threshold configurable | The S3 spec mandates ≥5 MiB per part. The threshold is tunable via config to match network conditions or client capabilities. |
| `r2_key == blob_id` (CAS invariant) | Removes an entire class of mapping bugs. The object key is always the content hash. Stored explicitly to allow future schema evolution. |

---

## 9. What blob-service Is Explicitly NOT

To prevent scope creep, the following are out of scope now and in future iterations unless this document is revised:

- **Access control lists (ACLs)**: Who may download a blob is the caller's responsibility. blob-service issues URLs to any authenticated service that asks.
- **Blob expiry / garbage collection**: blob-service does not delete blobs. A future `GC` subsystem may be designed separately.
- **Virus / content scanning**: Out of scope. May be implemented as an async side-channel by callers if required.
- **Serving a public HTTP API**: All API surface is gRPC over an internal network. No public ingress.
- **Thumbnailing / transcoding / transformation**: Callers are responsible for all content transformation. blob-service stores only the original bytes.
