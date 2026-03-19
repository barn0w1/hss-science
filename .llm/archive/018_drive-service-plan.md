# drive-service Implementation Plan

**Status**: Pre-implementation plan — do not implement until approved.
**Date**: 2026-03-17
**Author**: LLM planning session

---

## 1. Overview

`drive-service` is the backend gRPC resource server for `drive.hss-science.org` — an hss-science cloud storage product. It is called exclusively by `drive-bff` (not yet in the repo) over an internal gRPC channel.

### Key properties
- **Storage model**: CAS (Content-Addressable Storage), git-inspired
  - Blob: file content addressed by `sha256(content)`, object stored in Cloudflare R2
  - Node: named reference to a Blob (file) or collection of Nodes (directory), always inside a Space
  - Space: collaboration unit; one personal Space per user, additional Spaces can be created and shared
- **Auth**: JWT Bearer tokens issued by the accounts OIDC provider; RS256, verified at gRPC interceptor
- **Authorization**: OpenFGA with Space-level RBAC (owner / editor / viewer)
- **Caller**: drive-bff via gRPC; no public-facing grpc endpoint

---

## 2. Technical Stack

Follows all existing patterns from `accounts` and `myaccount-bff`:

| Layer | Choice | Notes |
|---|---|---|
| Language | Go 1.26 | existing module |
| HTTP router | — | gRPC only; no HTTP |
| gRPC framework | `google.golang.org/grpc` v1.79.1 | existing dep |
| Proto generation | `buf` v2 | existing `buf.yaml`/`buf.gen.yaml` |
| Database | PostgreSQL via `jmoiron/sqlx` + `lib/pq` | raw SQL, no ORM |
| IDs | `oklog/ulid/v2` | TEXT PRIMARY KEY |
| JWT verification | `go-jose/go-jose/v4` (static key) or `coreos/go-oidc/v3` JWKS (dynamic) | see §7 |
| Authorization | OpenFGA SDK (`openfga/go-sdk`) | new dep |
| Object storage | Cloudflare R2 via `aws-sdk-go-v2` + S3-compatible endpoint | new dep |
| Error model | `pkg/domerr` sentinel errors | same as accounts |
| Migrations | `embed.FS` numbered `.up.sql`/`.down.sql` | same as accounts |
| Tests | `testcontainers-go` for Postgres | same as accounts |

---

## 3. Domain Model

### 3.1 Blob

Immutable. Content-addressed. Shared across all Spaces/Nodes that reference the same content.

```
Blob
  sha256     string    — hex-encoded SHA-256, PRIMARY KEY
  size       int64     — bytes
  r2Key      string    — R2 object key (derived, e.g. "blobs/ab/cd/ef...")
  createdAt  time.Time
```

A Blob record is inserted **once** when first uploaded; subsequent uploads of the same content are deduped by the sha256 lookup.

### 3.2 Space

The collaboration unit. Nodes do not have per-node ACLs — all permissions flow from the Space.

```
Space
  id         string    — ULID
  name       string
  kind       SpaceKind — PERSONAL | SHARED
  ownerID    string    — accounts service user `sub`
  createdAt  time.Time
  updatedAt  time.Time
```

**Invariants:**
- Each user has exactly one PERSONAL Space (created automatically on first drive access).
- SHARED Spaces are created explicitly and have no hard limit per user.
- The `ownerID` is always an `owner`-role member in OpenFGA.

### 3.3 SpaceMember

```
SpaceMember
  spaceID  string     — FK → Space
  userID   string     — accounts service user `sub`
  role     MemberRole — OWNER | EDITOR | VIEWER
  addedAt  time.Time
  addedBy  string     — user `sub` of the actor who added this member
```

The space owner is always represented as a SpaceMember with role OWNER. Removing or downgrading the sole owner is forbidden.

### 3.4 Node

A named entry in a Space's file tree. Either a file (references a Blob) or a directory (holds child Nodes).

```
Node
  id         string      — ULID
  spaceID    string      — FK → Space
  parentID   *string     — FK → Node (nil = root-level node of the Space)
  name       string      — basename only, not a full path
  kind       NodeKind    — FILE | DIRECTORY
  blobID     *string     — FK → Blob (only set when kind == FILE)
  createdBy  string      — user `sub`
  createdAt  time.Time
  updatedAt  time.Time
  deletedAt  *time.Time  — soft-delete (nil = not deleted)
```

**Invariants:**
- A FILE node always has `blobID != nil`.
- A DIRECTORY node always has `blobID == nil`.
- Names are unique within a parent (or within the Space root when `parentID == nil`), excluding soft-deleted nodes.
- Moving a node across Spaces is not supported in v1 (cross-Space move would violate shared blob ownership semantics).

---

## 4. Data Model (PostgreSQL Schema)

### Migration 1: `1_initial.up.sql`

```sql
-- Blobs: content-addressed, immutable
CREATE TABLE blobs (
  sha256      TEXT PRIMARY KEY,
  size        BIGINT NOT NULL,
  r2_key      TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Spaces
CREATE TABLE spaces (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  kind        TEXT NOT NULL CHECK (kind IN ('personal', 'shared')),
  owner_id    TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX spaces_owner_id_idx ON spaces (owner_id);

-- Space membership (source of truth alongside OpenFGA)
-- OpenFGA is authoritative for permission checks;
-- this table is the authoritative record for membership enumeration.
CREATE TABLE space_members (
  space_id    TEXT NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
  user_id     TEXT NOT NULL,
  role        TEXT NOT NULL CHECK (role IN ('owner', 'editor', 'viewer')),
  added_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  added_by    TEXT NOT NULL,
  PRIMARY KEY (space_id, user_id)
);

-- Nodes: file/folder tree
CREATE TABLE nodes (
  id          TEXT PRIMARY KEY,
  space_id    TEXT NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
  parent_id   TEXT REFERENCES nodes(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  kind        TEXT NOT NULL CHECK (kind IN ('file', 'directory')),
  blob_id     TEXT REFERENCES blobs(sha256),
  created_by  TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at  TIMESTAMPTZ,
  -- FILE must have blob, DIRECTORY must not
  CONSTRAINT nodes_file_has_blob   CHECK (kind != 'file'      OR blob_id IS NOT NULL),
  CONSTRAINT nodes_dir_no_blob     CHECK (kind != 'directory' OR blob_id IS NULL)
);

-- Listing children efficiently
CREATE INDEX nodes_parent_id_idx        ON nodes (parent_id)          WHERE deleted_at IS NULL;
CREATE INDEX nodes_space_id_idx         ON nodes (space_id)           WHERE deleted_at IS NULL;

-- Uniqueness: name must be unique within a parent (scoped by space for root nodes)
CREATE UNIQUE INDEX nodes_unique_name_in_parent
  ON nodes (parent_id, name)
  WHERE parent_id IS NOT NULL AND deleted_at IS NULL;

CREATE UNIQUE INDEX nodes_unique_name_in_space_root
  ON nodes (space_id, name)
  WHERE parent_id IS NULL AND deleted_at IS NULL;
```

### Notes
- All IDs are TEXT (ULID) following the existing pattern.
- `space_members` mirrors OpenFGA tuples; kept in sync transactionally.
- Soft-delete via `deleted_at`; a separate "purge" operation does hard delete.
- Cascading deletes propagate space deletion to nodes; node cascade propagates directory deletion to children.

---

## 5. OpenFGA Authorization Model

### 5.1 FGA Model (DSL)

```python
model
  schema 1.1

type user

type space
  relations
    define owner:  [user]
    define editor: [user] or owner
    define viewer: [user] or editor

  define can_read:           viewer
  define can_write:          editor
  define can_delete_node:    editor
  define can_manage_members: owner
  define can_delete_space:   owner
  define can_rename:         editor
```

**Key properties of this model:**
- Permissions escalate: `owner` → `editor` → `viewer` (via union).
- A single object type (`space`) governs all node access — no per-node FGA objects.
- `can_write` covers: create node, update node content (MoveNode, replace blob), delete (soft).
- `can_delete_space` / `can_manage_members` are owner-only.

### 5.2 Relationship Tuples

Tuples written when membership changes:

| Event | Tuple written |
|---|---|
| Space created (owner) | `(user:<sub>, owner, space:<id>)` |
| Member added (editor) | `(user:<sub>, editor, space:<id>)` |
| Member added (viewer) | `(user:<sub>, viewer, space:<id>)` |
| Member role changed | Delete old tuple, write new tuple |
| Member removed | Delete tuple |
| Space deleted | All tuples deleted (bulk) |

### 5.3 Check patterns

Every gRPC handler that touches a Space or Node calls OpenFGA:

```
// Before any node read:
Check(user:<caller_sub>, can_read, space:<node.spaceID>)

// Before node create/move/delete:
Check(user:<caller_sub>, can_write, space:<node.spaceID>)

// Before AddSpaceMember / RemoveSpaceMember:
Check(user:<caller_sub>, can_manage_members, space:<spaceID>)
```

Nodes never appear as FGA objects — only Spaces do.

### 5.4 OpenFGA Deployment

drive-service connects to a dedicated OpenFGA instance (or shared one if other services use it). Configured via `OPENFGA_API_URL` and `OPENFGA_STORE_ID`. Authorization model ID stored in config.

---

## 6. gRPC API Surface

### 6.1 Proto Location

```
api/proto/drive/v1/drive.proto
```

Goes into `buf.yaml`'s existing module (same `api/proto` root), generating into `server/gen/drive/v1/`.

### 6.2 Messages

```protobuf
// ── Blob ──────────────────────────────────────────────────
message Blob {
  string sha256     = 1;
  int64  size       = 2;
  google.protobuf.Timestamp created_at = 3;
}

// ── Space ─────────────────────────────────────────────────
enum SpaceKind {
  SPACE_KIND_UNSPECIFIED = 0;
  SPACE_KIND_PERSONAL    = 1;
  SPACE_KIND_SHARED      = 2;
}

message Space {
  string    id         = 1;
  string    name       = 2;
  SpaceKind kind       = 3;
  string    owner_id   = 4;
  google.protobuf.Timestamp created_at = 5;
  google.protobuf.Timestamp updated_at = 6;
}

// ── SpaceMember ───────────────────────────────────────────
enum MemberRole {
  MEMBER_ROLE_UNSPECIFIED = 0;
  MEMBER_ROLE_OWNER       = 1;
  MEMBER_ROLE_EDITOR      = 2;
  MEMBER_ROLE_VIEWER      = 3;
}

message SpaceMember {
  string     space_id = 1;
  string     user_id  = 2;
  MemberRole role     = 3;
  google.protobuf.Timestamp added_at = 4;
  string     added_by = 5;
}

// ── Node ──────────────────────────────────────────────────
enum NodeKind {
  NODE_KIND_UNSPECIFIED = 0;
  NODE_KIND_FILE        = 1;
  NODE_KIND_DIRECTORY   = 2;
}

message Node {
  string   id         = 1;
  string   space_id   = 2;
  string   parent_id  = 3;  // empty string = root-level
  string   name       = 4;
  NodeKind kind       = 5;
  string   blob_id    = 6;  // sha256; empty for directories
  string   created_by = 7;
  google.protobuf.Timestamp created_at = 8;
  google.protobuf.Timestamp updated_at = 9;
}
```

### 6.3 Service Definition

```protobuf
service DriveService {

  // ── Blobs ────────────────────────────────────────────────
  // Step 1: caller computes sha256 client-side; if server already has this
  // sha256, returns existing Blob (dedup). Otherwise returns a presigned R2
  // PUT URL for direct upload.
  rpc InitiateUpload(InitiateUploadRequest)
      returns (InitiateUploadResponse);

  // Step 2 (only needed when server returned a presigned URL): confirms that
  // the R2 PUT completed. Server does HeadObject to verify, then inserts Blob.
  rpc CompleteUpload(CompleteUploadRequest)
      returns (Blob);

  // Returns a short-lived presigned R2 GET URL for a Blob (caller must have
  // can_read on the Space that owns at least one Node referencing this Blob).
  rpc GetBlobDownloadUrl(GetBlobDownloadUrlRequest)
      returns (GetBlobDownloadUrlResponse);


  // ── Spaces ───────────────────────────────────────────────
  // Returns caller's personal Space, auto-creating it on first call.
  rpc GetOrCreatePersonalSpace(GetOrCreatePersonalSpaceRequest)
      returns (Space);

  rpc GetSpace(GetSpaceRequest)       returns (Space);
  rpc ListSpaces(ListSpacesRequest)   returns (ListSpacesResponse);
  rpc CreateSpace(CreateSpaceRequest) returns (Space);
  rpc UpdateSpace(UpdateSpaceRequest) returns (Space);
  rpc DeleteSpace(DeleteSpaceRequest) returns (google.protobuf.Empty);


  // ── Space Members ────────────────────────────────────────
  rpc ListSpaceMembers(ListSpaceMembersRequest)   returns (ListSpaceMembersResponse);
  rpc AddSpaceMember(AddSpaceMemberRequest)       returns (SpaceMember);
  rpc UpdateSpaceMember(UpdateSpaceMemberRequest) returns (SpaceMember);
  rpc RemoveSpaceMember(RemoveSpaceMemberRequest) returns (google.protobuf.Empty);


  // ── Nodes ────────────────────────────────────────────────
  rpc GetNode(GetNodeRequest)       returns (Node);
  rpc ListNodes(ListNodesRequest)   returns (ListNodesResponse);
  // Creates a node. For FILE nodes, blob_id must reference an existing Blob.
  rpc CreateNode(CreateNodeRequest) returns (Node);
  // Rename and/or reparent within the same Space.
  rpc MoveNode(MoveNodeRequest)     returns (Node);
  // Deep copy within same Space (new ULIDs, same blob references).
  rpc CopyNode(CopyNodeRequest)     returns (Node);
  // Soft-delete (sets deleted_at). Applies recursively to directory subtree.
  rpc DeleteNode(DeleteNodeRequest) returns (google.protobuf.Empty);
  // Restore soft-deleted node. Parent must not be soft-deleted.
  rpc RestoreNode(RestoreNodeRequest) returns (Node);
  // Hard-delete a soft-deleted node (owner only). Frees no R2 storage unless
  // blob ref-count reaches zero (out of scope for v1).
  rpc PurgeNode(PurgeNodeRequest)   returns (google.protobuf.Empty);
}
```

### 6.4 Key Request/Response messages (abbreviated)

```protobuf
message InitiateUploadRequest {
  string sha256          = 1;  // hex, client-computed
  int64  content_length  = 2;
  string content_type    = 3;  // MIME type hint
}
message InitiateUploadResponse {
  bool   already_exists  = 1;  // if true, blob is ready; no upload needed
  string presigned_url   = 2;  // R2 PUT URL (empty when already_exists)
  Blob   blob            = 3;  // populated only when already_exists
}

message CompleteUploadRequest {
  string sha256 = 1;
}

message GetBlobDownloadUrlRequest {
  string sha256    = 1;
  string node_id   = 2;  // used for auth check (must have can_read on its space)
}
message GetBlobDownloadUrlResponse {
  string url        = 1;
  google.protobuf.Timestamp expires_at = 2;
}

message ListSpacesRequest  { /* caller identity from JWT */ }
message ListSpacesResponse { repeated Space spaces = 1; }

message ListNodesRequest {
  string  space_id  = 1;
  string  parent_id = 2;  // empty = list Space root
  bool    include_deleted = 3;  // default false
}
message ListNodesResponse { repeated Node nodes = 1; }

message CreateNodeRequest {
  string   space_id  = 1;
  string   parent_id = 2;
  string   name      = 3;
  NodeKind kind      = 4;
  string   blob_id   = 5;  // required for FILE
}

message MoveNodeRequest {
  string node_id       = 1;
  string new_parent_id = 2;  // empty = move to Space root
  string new_name      = 3;  // empty = keep existing name
}

message CopyNodeRequest {
  string node_id          = 1;
  string dest_parent_id   = 2;
  string dest_name        = 3;
}
```

---

## 7. Service Architecture

Follows the hexagonal layout from `accounts`:

```
server/services/drive-service/
├── main.go                       — DI wiring; subcommands: server | migrate
├── Dockerfile
├── .env.example
│
├── config/
│   └── config.go                 — ConfigSource with OSEnvSource + MapSource for tests
│                                   Fields: DBConn, R2AccountID, R2AccessKey, R2SecretKey,
│                                   R2Bucket, R2Endpoint, OpenFGAApiURL, OpenFGAStoreID,
│                                   OpenFGAModelID, AccountsIssuer, AccountsJWKSURL,
│                                   GRPCAddr, PresignTTL
│
├── migrations/
│   ├── embed.go                  — go:embed *.sql
│   ├── 1_initial.up.sql
│   └── 1_initial.down.sql
│
└── internal/
    ├── blob/                     — Domain: Blob & R2 upload/download
    │   ├── domain.go             — Blob struct
    │   ├── ports.go              — BlobRepository, R2Store interfaces
    │   ├── service.go            — CAS dedup, presign logic, CompleteUpload
    │   └── postgres/
    │       └── blob_repo.go      — INSERT/SELECT on blobs table
    │
    ├── space/                    — Domain: Space + SpaceMember
    │   ├── domain.go             — Space, SpaceMember, MemberRole, SpaceKind
    │   ├── ports.go              — SpaceRepository, SpaceMemberRepository
    │   ├── service.go            — CreateSpace, GetOrCreatePersonal, AddMember, etc.
    │   └── postgres/
    │       └── space_repo.go
    │
    ├── node/                     — Domain: Node tree
    │   ├── domain.go             — Node, NodeKind
    │   ├── ports.go              — NodeRepository
    │   ├── service.go            — CreateNode, MoveNode, CopyNode, DeleteNode, Restore, Purge
    │   └── postgres/
    │       └── node_repo.go
    │
    ├── authz/                    — OpenFGA integration
    │   ├── client.go             — Check(ctx, user, relation, object) bool
    │   │                           WriteTuple / DeleteTuple / DeleteAllSpaceTuples
    │   └── model.go              — FGA model DSL embedded as string constant
    │
    ├── r2/
    │   └── store.go              — aws-sdk-go-v2 S3 client pointed at R2 endpoint
    │                               PresignPut, PresignGet, HeadObject
    │
    ├── grpc/
    │   ├── server.go             — grpc.NewServer with JWT interceptor + OpenFGA check interceptor
    │   ├── interceptor.go        — JWT verification: fetch JWKS from AccountsJWKSURL at startup
    │   │                           (coreos/go-oidc/v3 JWKS provider), extract sub → context
    │   ├── handler.go            — DriveServiceServer: orchestrates domain services + authz
    │   └── errors.go             — domerr → gRPC status codes (same as accounts pattern)
    │
    └── pkg/
        └── domerr/
            └── errors.go         — ErrNotFound, ErrAlreadyExists, ErrUnauthorized,
                                    ErrInternal, ErrFailedPrecondition (same sentinels)
```

---

## 8. JWT Authentication

drive-service is a pure resource server; it does not issue tokens.

**Strategy**: Dynamic JWKS via `coreos/go-oidc/v3`

At startup, `grpc/interceptor.go`:
1. Fetches `AccountsIssuer/.well-known/openid-configuration` to discover `jwks_uri`.
2. Creates an `oidc.Provider`; its `JWKS()` verifier automatically caches and rotates keys.
3. On each gRPC call, extracts `Authorization: Bearer <token>` from metadata, verifies RS256 signature + `iss` + `exp`, injects `sub` into `context.Context`.

This is more robust than the static key approach in `accounts/grpc/interceptor.go` because drive-service has no access to the private key and benefits from automatic key rotation.

---

## 9. R2 Storage Integration

### R2 Key Format
Objects are sharded to avoid hot prefixes under a single key:
```
blobs/<sha256[0:2]>/<sha256[2:4]>/<sha256[4:]>
```
Example: `sha256 = abcdef1234...` → `blobs/ab/cd/ef1234...`

### Upload Flow
```
drive-bff                drive-service                 R2
  │                           │                         │
  │── InitiateUpload ─────────►                         │
  │   (sha256, size)          │                         │
  │                      DB lookup by sha256            │
  │                      ┌── already_exists? ──────────►│
  │                      │   No: PresignPut(sha256)     │
  │◄── {presigned_url} ──┘                             │
  │                                                     │
  │── PUT content ──────────────────────────────────────►
  │◄── 200 OK ───────────────────────────────────────────
  │                           │                         │
  │── CompleteUpload ─────────►                         │
  │   (sha256)           HeadObject(r2_key) ───────────►│
  │                           │◄── ETag/size ───────────│
  │                      INSERT INTO blobs              │
  │◄── Blob ──────────────────│                         │
```

### Download Flow
`GetBlobDownloadUrl` generates a presigned R2 GET URL (TTL from config, default 1 hour). The drive-bff or client follows the redirect directly to R2.

### Deduplication
`InitiateUpload` returns `already_exists = true` if a `blobs` row for that sha256 already exists. The Blob is immediately available; no upload needed. This eliminates redundant R2 transfers for common files.

---

## 10. Proto Changes to `buf.yaml` / `buf.gen.yaml`

No structural changes needed. The existing `buf.yaml` module covers the whole `api/proto/` tree:

```
api/proto/
  accounts/v1/account_management.proto   (existing)
  drive/v1/drive.proto                   (new)
```

`buf.gen.yaml` already outputs to `server/gen` with `paths=source_relative`, so the new proto will generate into `server/gen/drive/v1/`.

---

## 11. CI/CD Changes

Add `drive-service` to `.github/workflows/ci2.yaml` matrix, watching:
- `server/services/drive-service/**`
- `api/proto/drive/**`
- `server/go.mod` (shared)

Add `drive-service` to `.github/workflows/release.yaml` Docker matrix.

---

## 12. Key Design Decisions

### D1: Nodes inherit Space permissions (no per-node ACLs)
FGA objects are Spaces only. Per-node ACLs would require an FGA object per node (potentially millions of tuples) and cross-cutting authorization checks. For v1, Space-level RBAC covers the common case. Future: can introduce Shared Links (time-limited presigned URLs) without changing the FGA model.

### D2: `space_members` table mirrors OpenFGA tuples
OpenFGA is authoritative for `Check` calls. The `space_members` table exists to enumerate members cheaply (SQL `SELECT` vs FGA `ListObjects`, which can be expensive). The two are kept in sync within the same operation (write DB row + write FGA tuple, with FGA write failure rolling back DB transaction).

### D3: Soft-delete before purge
`DeleteNode` sets `deleted_at`. `PurgeNode` performs hard delete. This gives users a trash/restore window, consistent with typical cloud storage UX. The `author` (drive-bff) controls whether the trash window is exposed.

### D4: No cross-Space node moves in v1
Moving a node across Spaces would require re-evaluating authorization (the caller needs `can_write` on both source and destination Spaces) and potentially reassigning blob references. Deferred to v2.

### D5: CAS deduplication is per-platform, not per-Space
Blobs are global. If two users upload the same file, only one R2 object is created. This is not exposed to users (they see Nodes, not Blobs). Privacy implication: knowing a sha256 does not grant read access — access is checked via the Node's Space.

### D6: Dynamic JWKS over static key
drive-service has no access to the accounts service signing key. Using the JWKS endpoint with `coreos/go-oidc/v3` is more operationally sound and handles key rotation automatically.

---

## 13. Open Questions

1. **Blob ref-counting / GC**: When should R2 objects be deleted? If zero Nodes reference a Blob, it is orphaned. A periodic GC job (reference count query → R2 DeleteObject) is the natural cleanup mechanism but adds complexity. Out of scope for v1 — orphaned blobs are benign but waste R2 storage.

2. **Recursive directory delete performance**: `DeleteNode` on a large directory subtree requires either recursive SQL (CTEs) or application-side tree traversal. A `WITH RECURSIVE` CTE setting `deleted_at` on all descendants is the cleanest approach at the cost of one large write.

3. **Maximum upload size**: R2 supports 5 TB via multipart; presigned single-PUT is limited to 5 GB. For large files, multipart upload will be needed. Out of scope for v1 — can constrain `content_length` in `InitiateUploadRequest`.

4. **Pagination**: `ListNodes` and `ListSpaces` should support cursor-based pagination for large directories. v1 can start without pagination (with a server-side cap, e.g., 1,000 nodes) and add it later as a non-breaking change to the proto.

5. **OpenFGA store initialization**: Who creates the FGA store and uploads the authorization model? Options: drive-service CLI subcommand (`drive-service fga init`), or Terraform/manual ops. Needs decision before deployment.

6. **Personal Space creation trigger**: `GetOrCreatePersonalSpace` auto-creates on first call from the user. The drive-bff should call this on first login. Alternative: accounts service emits an event on user creation. For simplicity, on-demand creation (idempotent upsert) is fine for v1.

7. **drive-bff gRPC client auth**: drive-bff will forward the user's access token to drive-service. The drive-service gRPC interceptor extracts `sub` from it. drive-bff does not have its own service account JWT for drive-service — it simply propagates the user token. This is acceptable since drive-bff is trusted and the network channel is internal.
