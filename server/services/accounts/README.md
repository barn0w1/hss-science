# accounts-idp

A fully compliant **OpenID Connect Provider (OP)** and **gRPC Resource Server (RS)** for the HSS Science platform. It is the central identity authority: it issues ID tokens and access tokens to downstream services, and exposes a gRPC API for reading and managing user data.

Rather than managing passwords directly, it delegates user authentication to upstream identity providers — starting with Google — and maps external identities to stable internal user accounts.

---

## Architecture

```
Downstream RP (e.g. myaccount-bff)
        │
        │  1. OIDC Authorization Code Flow (HTTP :8080)
        ▼
 accounts-idp  ◄─────────────────────────────── This service
        │  ▲
        │  │  2. User data API (gRPC :9090, JWT-authenticated)
        │  └─ Internal callers (myaccount-bff, future services)
        │
        │  3. Delegates AuthN to upstream IdP
        ▼
 Google OIDC   ◄─── Upstream identity provider
        │
        ▼
   PostgreSQL   ◄─── Single source of truth for all identity data
```

**Role summary:**

| Role | Counterpart | Protocol | Responsibility |
|------|-------------|----------|----------------|
| OpenID Provider (OP) | Downstream RPs | HTTP | Issue ID tokens, access tokens, refresh tokens via OIDC |
| Relying Party (RP) | Google (upstream) | HTTP | Delegate user authentication to the upstream IdP |
| gRPC Resource Server (RS) | Internal services (e.g. BFF) | gRPC | Serve user profile, linked accounts, and sessions; handle account deletion |

### Key design decisions

- **Pluggable authentication** — The upstream AuthN layer uses the Strategy pattern (`internal/authn`). Google is the first implementation; additional providers can be added without changing storage or the OIDC protocol layer.
- **Stateless HTTP server** — All state (auth requests, tokens, signing keys) lives in PostgreSQL. Multiple instances can run behind a load balancer without sticky sessions.
- **Auto-provisioning** — On first login via Google, a local `users` record is created automatically and linked to the Google identity via `federated_identities`. Subsequent logins update the cached profile.
- **RSA signing keys in DB** — The OP's RSA key pair is generated on first startup and persisted in PostgreSQL, so token signatures remain verifiable across restarts and deployments.
- **JWT access tokens for gRPC clients** — Clients registered with `access_token_type = 'jwt'` receive verifiable JWTs. The gRPC interceptor checks them locally using `storage.LoadAllPublicKeys(db)` — no network call to `/introspect` or the JWKS endpoint.
- **Dual server: HTTP + gRPC** — The HTTP OP and the gRPC RS run in the same process, sharing the database connection pool. The gRPC server listens on a separate port (`GRPC_PORT`, default `9090`).

---

## Prerequisites

- Go 1.25.5+
- PostgreSQL 14+
- A Google Cloud project with an OAuth 2.0 Web Client credential

---

## Local Development

### 1. Apply the database schema

Migrations are **not** run at startup. Apply them before the service starts:

```bash
psql "$DATABASE_URL" -f server/services/accounts/migrations/000001_init.up.sql
psql "$DATABASE_URL" -f server/services/accounts/migrations/000002_add_sessions_index.up.sql
```

### 2. Seed downstream clients

Add at least one client row to the `clients` table. The `access_token_type` column controls whether the OP issues opaque or signed JWT access tokens.

**Server-side confidential client that calls the gRPC RS (e.g. myaccount-bff):**

```sql
INSERT INTO clients (id, secret_hash, application_type, auth_method, redirect_uris, response_types, grant_types, access_token_type)
VALUES (
  'myaccount-bff',
  -- bcrypt hash of the secret; generate with: htpasswd -bnBC 12 "" <secret> | tr -d ':\n'
  '$2y$12$...',
  'web',
  'client_secret_post',
  '{"http://localhost:8081/auth/callback"}',
  '{code}',
  '{authorization_code,refresh_token}',
  'jwt'  -- required: the gRPC interceptor verifies JWTs locally; it cannot verify opaque tokens
);
```

**Public SPA (no gRPC RS access):**

```sql
INSERT INTO clients (id, application_type, auth_method, redirect_uris, response_types, grant_types, access_token_type)
VALUES ('my-spa', 'native', 'none', '{"http://localhost:3000/callback"}', '{code}', '{authorization_code,refresh_token}', 'bearer');
```

### 3. Configure environment variables

```bash
cp server/services/accounts/.env.example server/services/accounts/.env
# Fill in: DATABASE_URL, ENCRYPTION_KEY, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GOOGLE_REDIRECT_URI
```

### 4. Run the service

```bash
cd server
set -a && source services/accounts/.env && set +a
go run ./services/accounts/cmd/server
```

**Verify both servers are up:**

```bash
# HTTP OP: OIDC discovery
curl http://localhost:8080/.well-known/openid-configuration | jq .issuer

# gRPC RS: service listing (requires grpcurl)
grpcurl -plaintext localhost:9090 list
# Expected output includes: hss_science.accounts.v1.AccountsService
```

---

## Environment Variables

See [`.env.example`](.env.example) for full descriptions and example values.

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `PORT` | | `8080` | HTTP server listen port |
| `GRPC_PORT` | | `9090` | gRPC Resource Server listen port |
| `ISSUER` | ✓ | | Full OP base URL (e.g. `https://accounts.example.com`). Used as the JWT `iss` claim and discovery document base. |
| `DATABASE_URL` | ✓ | | PostgreSQL connection string |
| `ENCRYPTION_KEY` | ✓ | | Base64-encoded 32-byte AES key. Used by zitadel/oidc to encrypt auth codes and internal state. |
| `GOOGLE_CLIENT_ID` | ✓ | | Google OAuth2 client ID |
| `GOOGLE_CLIENT_SECRET` | ✓ | | Google OAuth2 client secret |
| `GOOGLE_REDIRECT_URI` | ✓ | | Must match `<ISSUER>/login/callback` and be listed in the Google Cloud Console |
| `DEV_MODE` | | `false` | `true` allows an HTTP (non-HTTPS) issuer URL. **Local dev only.** |
| `LOG_LEVEL` | | `info` | `debug` \| `info` \| `warn` \| `error` |
| `DB_MAX_OPEN_CONNS` | | `25` | PostgreSQL pool: max open connections |
| `DB_MAX_IDLE_CONNS` | | `5` | PostgreSQL pool: max idle connections |
| `DB_CONN_MAX_LIFETIME` | | `5m` | PostgreSQL pool: max connection reuse duration (Go duration string) |
| `ACCESS_TOKEN_LIFETIME` | | `5m` | Issued access token TTL (Go duration string) |
| `REFRESH_TOKEN_LIFETIME` | | `5h` | Issued refresh token TTL (Go duration string) |
| `ID_TOKEN_LIFETIME` | | `1h` | Issued ID token TTL (Go duration string) |

---

## HTTP Endpoints (OpenID Provider, port 8080)

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/.well-known/openid-configuration` | OIDC discovery document |
| `GET` | `/.well-known/jwks.json` | Public signing keys (JWKS) |
| `GET` | `/authorize` | Authorization endpoint |
| `POST` | `/token` | Token endpoint (authorization_code, refresh_token, client_credentials) |
| `GET` | `/userinfo` | UserInfo endpoint (Bearer token required) |
| `POST` | `/introspect` | Token introspection |
| `POST` | `/revoke` | Token revocation |
| `GET` | `/end_session` | RP-initiated logout |
| `GET` | `/healthz` | Health probe: `200 ok` or `503 unhealthy` |
| `GET` | `/login/google` | Initiate Google authentication |
| `GET` | `/login/callback` | Google OAuth2 callback |

## gRPC Endpoints (Resource Server, port 9090)

Service: `hss_science.accounts.v1.AccountsService`
Proto: [`api/proto/accounts/v1/accounts.proto`](../../../../api/proto/accounts/v1/accounts.proto)

All RPCs require a valid JWT access token in the `authorization: Bearer <token>` gRPC metadata header. The `sub` claim is extracted as the authenticated user ID. There are no `user_id` request fields — all operations are implicitly scoped to the token's subject.

| RPC | Description |
|-----|-------------|
| `GetProfile` | Return the authenticated user's full profile |
| `UpdateProfile` | Partially update mutable fields via `google.protobuf.FieldMask` (`given_name`, `family_name`, `picture`, `locale`) |
| `ListLinkedAccounts` | List all federated identity providers linked to the account |
| `UnlinkAccount` | Remove a federated identity. Returns `FAILED_PRECONDITION` if it is the only one remaining. |
| `ListActiveSessions` | List active refresh token sessions (client ID, scopes, auth time, expiry) |
| `RevokeSession` | Delete a specific session (removes both the refresh token and its access token) |
| `DeleteAccount` | Permanently delete the user and all associated data (tokens, linked accounts, auth history) |

---

## Authentication Flow

```
Browser / RP                  accounts-idp                   Google
     │                               │                           │
     │── GET /authorize ────────────►│                           │
     │                               │ Creates AuthRequest in DB │
     │◄─ 302 /login/google ──────────│                           │
     │                               │                           │
     │── GET /login/google ─────────►│                           │
     │                               │ Encodes authReqID in      │
     │                               │ OAuth2 state + CSRF cookie│
     │◄─ 302 accounts.google.com ────│                           │
     │                               │                           │
     │────────────────────────────── authenticates ─────────────►│
     │◄─ 302 /login/callback?code= ──────────────────────────────│
     │                               │                           │
     │── GET /login/callback ───────►│                           │
     │                               │ Validates CSRF            │
     │                               │ Exchanges Google code     │
     │                               │ FindOrCreateUser in DB    │
     │                               │ CompleteAuthRequest       │
     │◄─ 302 /authorize/callback ────│                           │
     │                               │ Generates auth code       │
     │◄─ 302 RP redirect_uri?code= ──│                           │
     │                               │                           │
     │── POST /token ───────────────►│                           │
     │◄─ {access_token, id_token} ───│                           │
```

The original RP authorization request is preserved across the Google redirect via the OAuth2 `state` parameter (base64url-encoded `authRequestID:csrfToken`). A short-lived CSRF cookie validates the round-trip on return.

---

## Database Schema

Defined in [`migrations/000001_init.up.sql`](migrations/000001_init.up.sql). Applied externally before the service starts.

| Table | Purpose |
|-------|---------|
| `users` | Internal user identities (UUID primary key, email, profile fields) |
| `federated_identities` | Maps `(provider, external_sub)` → internal `user_id` |
| `clients` | Registered OIDC clients: grant types, redirect URIs, token type |
| `auth_requests` | Pending authorization sessions (TTL-bound; deleted after code exchange) |
| `auth_codes` | Short-lived authorization codes (deleted immediately after token exchange) |
| `access_tokens` | Issued access tokens (opaque or JWT depending on client `access_token_type`) |
| `refresh_tokens` | Long-lived refresh tokens; rotated on every use; indexed on `user_id` |
| `signing_keys` | RSA 2048 key pairs used for JWT signing; generated once on first startup |

---

## Adding a New Authentication Provider

The `internal/authn.AuthnProvider` interface is the extension point:

```go
type AuthnProvider interface {
    Name() string
    AuthURL(state string) string
    HandleCallback(ctx context.Context, r *http.Request) (*Identity, error)
}
```

To add a new provider:

1. Implement the interface in a new file (e.g. `internal/authn/github.go`).
2. Instantiate it in `cmd/server/main.go`.
3. Register a new route in `internal/web/login.go` (e.g. `GET /login/github`).
4. Add a button to `internal/web/templates/login.html`.

No changes to the storage layer, token issuance, or the OIDC protocol handling are required.

---

## Docker

```bash
# Run from the repository root (build context must be server/)
docker build \
  -f server/services/accounts/Dockerfile \
  -t accounts-idp:local \
  server/
```

The image runs as a non-root user and exposes both port `8080` (HTTP) and port `9090` (gRPC).

