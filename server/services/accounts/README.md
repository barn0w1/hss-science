# accounts-idp

A fully compliant **OpenID Connect Provider (OP)** for the HSS Science platform. It acts as the central identity authority for all downstream services (Drive, Chat, etc.), issuing ID tokens and access tokens that those services can verify.

Rather than managing passwords directly, it delegates user authentication to upstream identity providers — starting with Google — and maps external identities to stable internal user accounts.

---

## Architecture

```
Downstream RP (e.g. Drive)
        │
        │  1. OIDC Authorization Code Flow
        ▼
 accounts-idp  ◄─── This service (OpenID Provider)
        │
        │  2. Delegates AuthN to Google
        ▼
 Google OIDC   ◄─── Upstream identity provider
```

**Role summary:**

| Role | Counterpart | Responsibility |
|------|-------------|----------------|
| OpenID Provider (OP) | Downstream services | Issue ID tokens, access tokens, refresh tokens |
| Relying Party (RP) | Google | Delegate user authentication |

### Key design decisions

- **Pluggable authentication** — The upstream AuthN layer uses the Strategy pattern (`internal/authn`). Google is the first implementation; additional providers can be added without changing storage or the OIDC protocol layer.
- **Stateless** — All state (auth requests, tokens, signing keys) lives in PostgreSQL. Multiple instances can run behind a load balancer without sticky sessions.
- **Auto-provisioning** — On first login via Google, a local `users` record is created automatically and linked to the Google identity via `federated_identities`. Subsequent logins update the cached profile.
- **RSA signing keys in DB** — The OP's RSA key pair is generated on first startup and persisted in PostgreSQL, so token signatures remain verifiable across restarts and deployments.

---

## Prerequisites

- Go 1.25.5+
- PostgreSQL 14+
- A Google Cloud project with an OAuth 2.0 Web Client credential

---

## Local Development

### 1. Apply the database schema

```bash
psql -U postgres -d accounts -f server/services/accounts/migrations/001_initial_schema.sql
```

> The application does not run migrations at startup. The schema must be applied externally before the service starts.

### 2. Seed downstream clients

Add at least one client row to the `clients` table for the service you want to test with. Example for a public SPA:

```sql
INSERT INTO clients (id, application_type, auth_method, redirect_uris, response_types, grant_types, access_token_type)
VALUES (
  'my-spa',
  'native',
  'none',
  '{"http://localhost:3000/callback"}',
  '{code}',
  '{authorization_code,refresh_token}',
  'bearer'
);
```

For a service account that uses client credentials:

```sql
INSERT INTO clients (id, secret_hash, application_type, auth_method, grant_types, access_token_type, is_service_account)
VALUES (
  'internal-svc',
  -- bcrypt hash of the secret; generate with: htpasswd -bnBC 12 "" <secret> | tr -d ':\n' | sed 's/\$apr1/\$2y/'
  '$2y$12$...',
  'web',
  'client_secret_basic',
  '{client_credentials}',
  'bearer',
  true
);
```

### 3. Configure environment variables

```bash
cp server/services/accounts/.env.example server/services/accounts/.env
# Edit .env and fill in GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, and DATABASE_URL
```

### 4. Run the service

```bash
cd server
# Source environment variables, then:
go run ./services/accounts/cmd/server
```

The service starts on `http://localhost:8080`. Verify it is running:

```bash
curl http://localhost:8080/.well-known/openid-configuration | jq .
```

---

## Environment Variables

See [`.env.example`](.env.example) for the full list with descriptions. Quick reference:

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `PORT` | | `8080` | HTTP listen port |
| `ISSUER` | ✓ | | Full OP URL (e.g. `https://accounts.example.com`) |
| `DATABASE_URL` | ✓ | | PostgreSQL connection string |
| `ENCRYPTION_KEY` | ✓ | | Base64-encoded 32-byte AES key (`openssl rand -base64 32`) |
| `GOOGLE_CLIENT_ID` | ✓ | | Google OAuth2 client ID |
| `GOOGLE_CLIENT_SECRET` | ✓ | | Google OAuth2 client secret |
| `GOOGLE_REDIRECT_URI` | ✓ | | Callback URL; must match `<ISSUER>/login/callback` and be registered in Google Console |
| `LOG_LEVEL` | | `info` | `debug`, `info`, `warn`, or `error` |
| `DEV_MODE` | | `false` | Allow HTTP issuer (local dev only) |

---

## OIDC Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /.well-known/openid-configuration` | Discovery document |
| `GET /.well-known/jwks.json` | Public signing keys (JWKS) |
| `GET /authorize` | Authorization endpoint |
| `POST /token` | Token endpoint (code exchange, refresh, client credentials) |
| `GET /userinfo` | UserInfo endpoint |
| `POST /introspect` | Token introspection |
| `POST /revoke` | Token revocation |
| `GET /end_session` | Logout |

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

Defined in [`migrations/001_initial_schema.sql`](migrations/001_initial_schema.sql). Applied externally before the service starts.

| Table | Purpose |
|-------|---------|
| `users` | Internal user identities |
| `federated_identities` | Maps `provider` + `external_sub` to an internal user ID |
| `clients` | Registered downstream OIDC clients |
| `auth_requests` | Pending authorization sessions |
| `auth_codes` | Short-lived authorization codes (deleted after token exchange) |
| `access_tokens` | Issued access tokens |
| `refresh_tokens` | Refresh tokens; rotated on every use |
| `signing_keys` | RSA 2048 key pairs for JWT signing; generated on first startup |

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

## Building the Docker Image

The Dockerfile is at the root of this directory and is built from the `server/` module context:

```bash
# Run from the repository root
docker build \
  -f server/services/accounts/Dockerfile \
  -t accounts-idp:local \
  .
```

The image runs as a non-root user and exposes port `8080`.

