# myaccount-bff

A **Backend-For-Frontend (BFF)** service for the MyAccount SPA. It acts as an OIDC Relying Party, manages user sessions in Redis, and proxies authenticated requests to the `accounts-idp` gRPC Resource Server.

The browser never touches a token. It only ever holds an opaque, HTTP-Only session cookie. All JWT access tokens live exclusively in Redis, server-side.

---

## Architecture

```
         Browser
            │
            │  HTTP-Only session cookie (__Host-myaccount_session)
            │  No tokens ever sent to browser
            ▼
   myaccount-bff  ◄──────────────────────── This service
      (chi, :8081)
            │  ▲
            │  │  Session store (JSON-serialized session data)
            │  └─ Redis
            │
            │  gRPC (Bearer JWT access token in metadata)
            ▼
   accounts-idp  (:9090)
      gRPC Resource Server
            │
            ▼
        PostgreSQL
```

**Role summary:**

| Role | Counterpart | Responsibility |
|------|-------------|----------------|
| OIDC Relying Party (RP) | accounts-idp (OP) | Run the Authorization Code flow; exchange code for tokens |
| Session Manager | Redis | Store tokens server-side; issue opaque cookie to browser |
| REST-to-gRPC Gateway | accounts-idp (RS) | Translate REST API calls into gRPC calls using the session's access token |
| CORS gatekeeper | myaccount-spa | Allow credentials from the SPA origin only |

---

## Code Generation

The HTTP API is **schema-driven**: [`api/openapi/myaccount/v1/openapi.yaml`](../../../../api/openapi/myaccount/v1/openapi.yaml) is the single source of truth.

`oapi-codegen` generates the types, chi server wrapper, and strict server interface from the spec:

```bash
# From repo root
make gen-myaccount
```

Output: `server/bff/gen/myaccount/v1/myaccount.gen.go`

This file is checked in and **must not be edited by hand**. Re-generate it whenever `openapi.yaml` changes.

---

## Directory Structure

```
server/bff/myaccount/
├── cmd/server/main.go                  # Entry point: router, Redis, OIDC discovery, gRPC client
└── internal/
    ├── config/config.go                # 12-Factor env config with validation
    ├── session/
    │   ├── store.go                    # Redis session CRUD (Create/Get/Delete), 64-byte hex IDs
    │   └── middleware.go               # chi middleware: cookie → Redis lookup → inject into ctx
    ├── grpcclient/
    │   └── client.go                   # gRPC client wrapper + WithToken(ctx, token) helper
    └── handler/
        ├── auth.go                     # Login / Callback handlers (OIDC RP flow, manual chi routes)
        ├── server.go                   # StrictServerInterface implementation (all JSON endpoints)
        ├── httpctx.go                  # Strict middleware: injects http.ResponseWriter into context
        ├── convert.go                  # Proto message → generated OpenAPI type converters
        └── response.go                 # apiError / mapGRPCError / HandleStrictError helpers

server/bff/gen/myaccount/v1/
└── myaccount.gen.go                    # Generated: types, ServerInterface, StrictServerInterface
```

---

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Schema-driven** | `openapi.yaml` → `oapi-codegen` → `StrictServerInterface`. Handlers are compile-time checked against the spec. Adding or changing an endpoint means editing the spec first. |
| **No tokens in the browser** | The SPA only sends an opaque session cookie. Tokens live in Redis, keyed by random 64-byte hex IDs. XSS cannot exfiltrate tokens. |
| **`__Host-` cookie prefix** | `__Host-myaccount_session` enforces `Secure`, `Path=/`, and no `Domain` attribute, preventing subdomain cookie injection. Falls back to `myaccount_session` in dev mode (`DEV_MODE=true`). |
| **Login/Callback are manual routes** | These endpoints perform HTTP redirects and set cookies — they don't fit the JSON request/response model of `StrictServerInterface`. They stay as direct chi handlers. All other endpoints use the generated wrapper. |
| **UserInfo fallback in Callback** | Some OIDC providers include `email`/`given_name`/etc. only in the UserInfo endpoint, not the ID token. The Callback handler falls back to `provider.UserInfo()` when ID token claims are empty. |
| **OIDC state cookie** | `state|nonce|return_to` is stored in a short-lived cookie during the auth flow. Validated on callback to prevent CSRF and replay attacks. |
| **gRPC metadata forwarding** | Each handler calls `grpcclient.WithToken(ctx, session.AccessToken)` to attach the JWT as a Bearer token in gRPC metadata. |
| **Session invalidation on logout** | Logout deletes the Redis key before clearing the cookie, ensuring the session cannot be replayed even if the cookie was intercepted. |

---

## Prerequisites

- Go 1.25+
- Redis 7+
- A running `accounts-idp` instance (HTTP `:8080` for OIDC discovery, gRPC `:9090` for API calls)
- A registered `myaccount-bff` client in accounts-idp's `clients` table (see below)

---

## Local Development

### 1. Register the BFF client in accounts-idp

```sql
INSERT INTO clients (
  id, secret_hash, application_type, auth_method,
  redirect_uris, response_types, grant_types, access_token_type
) VALUES (
  'myaccount-bff',
  -- bcrypt hash of your OIDC_CLIENT_SECRET; generate with:
  --   htpasswd -bnBC 12 "" <secret> | tr -d ':\n'
  '$2y$12$<hash>',
  'web',
  'client_secret_post',
  '{"http://localhost:8081/auth/callback"}',
  '{code}',
  '{authorization_code,refresh_token}',
  'jwt'
);
```

> `access_token_type = 'jwt'` is mandatory. The accounts-idp gRPC interceptor verifies access tokens locally and only accepts signed JWTs.

### 2. Start Redis

```bash
docker run -d -p 6379:6379 redis:7-alpine
```

### 3. Configure environment variables

```bash
cp server/bff/myaccount/.env.example server/bff/myaccount/.env
# Fill in: OIDC_CLIENT_SECRET, SESSION_SECRET
```

### 4. Run the service

```bash
cd server
set -a && source bff/myaccount/.env && set +a
go run ./bff/myaccount/cmd/server
```

**Verify:**

```bash
curl http://localhost:8081/healthz          # → ok
curl -i http://localhost:8081/auth/session  # → 401 (no cookie)
open http://localhost:8081/auth/login       # full OIDC flow in browser
```

---

## Environment Variables

See [`.env.example`](.env.example) for full descriptions.

| Variable | Required | Default | Description |
|----------|:--------:|---------|-------------|
| `PORT` | | `8081` | HTTP server listen port |
| `LOG_LEVEL` | | `info` | `debug` \| `info` \| `warn` \| `error` |
| `DEV_MODE` | | `false` | Plain cookie name, no HTTPS requirement. **Local dev only.** |
| `OIDC_ISSUER` | ✓ | | accounts-idp base URL (e.g. `http://localhost:8080`) |
| `OIDC_CLIENT_ID` | ✓ | | Client ID registered in accounts-idp |
| `OIDC_CLIENT_SECRET` | ✓ | | Client secret (plaintext) |
| `OIDC_REDIRECT_URI` | ✓ | | Must match `redirect_uris` in accounts-idp (e.g. `http://localhost:8081/auth/callback`) |
| `REDIS_URL` | ✓ | | Redis connection URL (e.g. `redis://localhost:6379/0`) |
| `SESSION_SECRET` | ✓ | | Base64-encoded 32-byte key. Generate: `openssl rand -base64 32` |
| `ACCOUNTS_GRPC_ADDR` | ✓ | | Host:port of accounts-idp gRPC server (e.g. `localhost:9090`) |
| `SPA_ORIGIN` | ✓ | | SPA origin for CORS (e.g. `http://localhost:5174`) |
| `SESSION_MAX_AGE` | | `24h` | Redis session TTL and cookie `Max-Age` |

---

## HTTP Endpoints

All endpoints are on port `8081` by default.

### Auth Endpoints

| Method | Path | Auth | Description |
|--------|------|:----:|-------------|
| `GET` | `/auth/login` | — | Redirect to accounts-idp authorization endpoint. Accepts optional `?return_to=`. |
| `GET` | `/auth/callback` | — | Handle OIDC callback: validate state, exchange code, store session, set cookie, redirect to SPA. |
| `POST` | `/auth/logout` | ✓ | Delete Redis session, clear cookie. Returns `{"message": "logged out"}`. |
| `GET` | `/auth/session` | ✓ | Return cached session info (user_id, email, given_name, family_name, picture). No gRPC call. |
| `GET` | `/healthz` | — | Health probe. Returns `200 ok`. |

### API Endpoints

All require a valid session cookie. Session-less requests return `401`.

| Method | Path | gRPC RPC | Description |
|--------|------|----------|-------------|
| `GET` | `/api/v1/profile` | `GetProfile` | Return the authenticated user's profile |
| `PATCH` | `/api/v1/profile` | `UpdateProfile` | Update profile fields (given_name, family_name, picture, locale) |
| `GET` | `/api/v1/linked-accounts` | `ListLinkedAccounts` | List federated identity providers linked to the account |
| `DELETE` | `/api/v1/linked-accounts/{id}` | `UnlinkAccount` | Remove a linked provider. Returns `409` if it is the only one. |
| `GET` | `/api/v1/sessions` | `ListActiveSessions` | List active sessions |
| `DELETE` | `/api/v1/sessions/{id}` | `RevokeSession` | Revoke a specific session |
| `DELETE` | `/api/v1/account` | `DeleteAccount` | Permanently delete the account and all data |

### Error Responses

All errors use `application/json` with schema `{"code": "...", "message": "..."}`.

| HTTP | Cause |
|------|-------|
| `400` | Invalid request parameters or body |
| `401` | Missing or expired session cookie |
| `404` | Resource not found (gRPC `NOT_FOUND`) |
| `409` | Conflict, e.g. unlinking the only remaining identity |
| `500` | Unexpected internal or gRPC error |

---

## OIDC Login Flow

```
SPA / Browser              myaccount-bff               accounts-idp
     │                           │                           │
     │── GET /auth/login ───────►│                           │
     │                           │  Generate state + nonce   │
     │                           │  Store in state cookie    │
     │◄─ 302 /authorize?... ─────│                           │
     │                           │                           │
     │──────────────────────────►│ (accounts-idp OIDC flow)  │
     │◄─ 302 /auth/callback ─────│◄──────────────────────────│
     │                           │                           │
     │── GET /auth/callback ────►│                           │
     │                           │  Validate state cookie    │
     │                           │  Exchange code → tokens   │
     │                           │  Verify ID token + nonce  │
     │                           │  Fetch UserInfo if needed │
     │                           │  Store session in Redis   │
     │◄─ 302 / (SPA) ────────────│  Set __Host-myaccount_    │
     │  Set-Cookie: session      │  session cookie           │
     │                           │                           │
     │── GET /api/v1/profile ───►│                           │
     │  Cookie: session=<id>     │  Lookup session in Redis  │
     │                           │  Call gRPC GetProfile     │
     │◄─ 200 {profile} ──────────│  with access_token        │
```

---

## Docker

```bash
# Run from the repository root (build context must be server/)
docker build \
  -f server/bff/myaccount/Dockerfile \
  -t myaccount-bff:local \
  server/
```

The image runs as a non-root user and exposes port `8081`.
