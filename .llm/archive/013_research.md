# accounts service — deep research

_Last updated: 2026-03-16. Based on a full read of every source file under
`server/services/identity-service/`._

---

## 1. Purpose & Scope

The `accounts` service is the **single OIDC/OAuth 2.0 authorization server** for
the hss-science platform. It:

- Implements a full OIDC Provider using `github.com/zitadel/oidc/v3` (v3.45.5).
- Delegates user authentication entirely to **external OAuth2 / OIDC upstream
  providers** (currently Google OIDC and GitHub OAuth2). There is no
  password-based login.
- Issues JWT access tokens and opaque (hashed) refresh tokens.
- Maintains a `users` table synthesised from upstream identity claims.
- Tracks **device sessions** linked to refresh tokens for session management.
- Exposes the standard OIDC discovery, authorisation, token, userinfo,
  introspection, and revocation endpoints via the zitadel `op.Provider`.
- Has a subcommand interface: `server` (default) and `cleanup` (token GC).
- One seeded downstream client: `myaccount-bff`.

---

## 2. Repository Layout

```
server/services/identity-service/
├── main.go                        # wiring, HTTP router, cleanup goroutines
├── config/
│   └── config.go                  # env-based config loading, RSA parse, bounds
├── internal/
│   ├── authn/                     # HTTP login flow (provider selection + callback)
│   │   ├── config.go
│   │   ├── handler.go             # SelectProvider · FederatedRedirect · FederatedCallback
│   │   ├── login_usecase.go       # CompleteFederatedLogin use-case
│   │   ├── provider.go            # Provider struct + NewProviders factory
│   │   ├── provider_google.go     # Google OIDC upstream
│   │   ├── provider_github.go     # GitHub OAuth2 upstream
│   │   ├── devicename.go          # UA parser (OS + browser string)
│   │   ├── ip.go                  # clientIP() from CF-Connecting-IP / RemoteAddr
│   │   ├── embed.go               # embed templateFS
│   │   └── templates/
│   │       └── select_provider.html  # Material Design 3 sign-in UI
│   ├── identity/                  # User domain
│   │   ├── domain.go              # User, FederatedIdentity, FederatedClaims
│   │   ├── ports.go               # Repository + Service interfaces
│   │   ├── service.go             # FindOrCreateByFederatedLogin, GetUser
│   │   └── postgres/
│   │       └── user_repo.go       # GetByID, FindByFederatedIdentity,
│   │                              #   CreateWithFederatedIdentity,
│   │                              #   UpdateUserFromClaims, UpdateFederatedIdentityClaims
│   ├── oidc/                      # OIDC domain
│   │   ├── domain.go              # AuthRequest, Client, Token, RefreshToken, DeviceSession
│   │   ├── ports.go               # Repository + Service interfaces (all 5 aggregates)
│   │   ├── authrequest_svc.go     # TTL-aware GetByID / GetByCode
│   │   ├── client_svc.go          # bcrypt secret validation
│   │   ├── token_svc.go           # ULID id gen, SHA-256 refresh token hashing
│   │   ├── device_session_svc.go  # thin wrapper over repo
│   │   ├── adapter/               # zitadel/oidc op.Storage implementation
│   │   │   ├── storage.go         # StorageAdapter — 100% of op.Storage
│   │   │   ├── provider.go        # op.Provider factory + Config
│   │   │   ├── authrequest.go     # AuthRequest adapter (op.AuthRequest)
│   │   │   ├── client.go          # ClientAdapter (op.Client)
│   │   │   ├── keys.go            # SigningKeyWithID, PublicKeySet, deriveKeyID
│   │   │   ├── refreshtoken.go    # RefreshTokenRequest adapter
│   │   │   └── userinfo.go        # UserClaims struct + UserClaimsSource interface
│   │   └── postgres/
│   │       ├── authrequest_repo.go
│   │       ├── client_repo.go
│   │       ├── token_repo.go
│   │       └── device_session_repo.go
│   ├── middleware/
│   │   ├── ratelimit.go           # per-IP token-bucket limiter (golang.org/x/time/rate)
│   │   └── securityheaders.go     # X-Content-Type-Options, X-Frame-Options, HSTS, Referrer
│   └── pkg/
│       ├── crypto/aes.go          # AES-256-GCM encrypt/decrypt (state cookie)
│       └── domerr/errors.go       # ErrNotFound, ErrAlreadyExists, ErrUnauthorized, ErrInternal
├── migrations/
│   ├── 1_initial.up.sql           # users, federated_identities, clients, auth_requests,
│   │                              #   tokens, refresh_tokens
│   ├── 2_seed_clients.up.sql      # INSERT myaccount-bff client
│   └── 3_device_sessions.up.sql   # device_sessions table + FK on refresh_tokens + auth_requests
└── testhelper/testdb.go           # testcontainers-go Postgres helper
```

---

## 3. Architectural Pattern

The service follows a **ports-and-adapters (hexagonal) layout**:

| Layer | Package | Role |
|---|---|---|
| Domain models | `internal/oidc/domain.go`, `internal/identity/domain.go` | Plain structs, no infrastructure deps |
| Ports (interfaces) | `internal/oidc/ports.go`, `internal/identity/ports.go` | Repository + Service contracts |
| Service (use-case) | `*_svc.go` files | Business logic, calls repositories |
| Adapters — DB | `postgres/` sub-packages | sqlx-based Postgres implementations |
| Adapters — OIDC | `internal/oidc/adapter/` | Translates zitadel `op.*` contracts to domain calls |
| Delivery — HTTP | `main.go` + `internal/authn/` | chi router, login flow handlers |
| Infrastructure | `config/`, `middleware/`, `pkg/crypto/`, `pkg/domerr/` | Cross-cutting concerns |

The domain packages (`oidc`, `identity`) import **only** standard library and
internal `pkg/domerr`. No infrastructure leaks into business logic.

---

## 4. Database Schema

### Tables

| Table | Primary purpose |
|---|---|
| `users` | Canonical user records (id TEXT, email, name, etc.) |
| `federated_identities` | One row per (provider, user) link. UNIQUE(provider, provider_subject) |
| `clients` | OIDC registered clients (configurable per-client scopes, grant types, etc.) |
| `auth_requests` | Short-lived OAuth2 authorisation requests (TTL-cleaned per config) |
| `tokens` | Access tokens (opaque ID, not stored as JWT text) |
| `refresh_tokens` | Refresh tokens stored as SHA-256 hashes |
| `device_sessions` | One row per device fingerprint, linked to refresh tokens |

### Key design decisions

- **Access tokens are opaque references** to DB rows. The JWT body is produced
  by zitadel/oidc at signing time with the token ID embedded. The DB row is the
  source of truth for introspection / revocation.
- **Refresh tokens are hashed** (SHA-256 hex). Raw token is never stored.
- **Refresh token rotation** is atomic: old token + its access token are deleted
  in the same transaction that inserts the new pair. A `n == 0` (already used)
  check prevents replay.
- `device_session_id` FK is SET NULL on device session delete (migration 3),
  so deleting a session doesn't orphan refresh tokens—it just unlinks them.
- `auth_requests` has no FK into `clients`; client validation is done in
  application code.
- `users.id` and all primary keys use ULID (26-char TEXT).

---

## 5. OIDC Flow

### 5.1 Authorisation Code (PKCE) flow

```
Client  →  GET /authorize?...code_challenge=S256...
           StorageAdapter.CreateAuthRequest()
              - validates PKCE required for public clients (AuthMethod=none)
              - persists AuthRequest to DB (TTL: 30 min default)
           → redirect to /login?authRequestID=<id>

/login  →  SelectProvider (GET) renders HTML template
/login/select (POST) → FederatedRedirect
           - encrypt(state={authRequestID, provider, nonce}) with AES-256-GCM
           - redirect to upstream provider (Google / GitHub)

/login/callback → FederatedCallback
           - decrypt + verify state
           - exchange code with upstream provider
           - FetchClaims (Google: verify id_token; GitHub: /user + /user/emails)
           - identity.FindOrCreateByFederatedLogin → upsert user + federated_identity
           - device session FindOrCreate (dsid cookie, 2-year max-age)
           - authReqs.CompleteLogin(authRequestID, userID, authTime, amr=["fed"], deviceSessionID)
           - redirect to op.AuthCallbackURL(provider)(ctx, authRequestID)

op.Provider  →  GET /authorize/callback (internal zitadel flow)
           - StorageAdapter.AuthRequestByID → check IsDone=true
           - StorageAdapter.SaveAuthCode(id, code)
           - redirect to client redirect_uri with code

Client  →  POST /oauth/v2/token?grant_type=authorization_code&code=...
           StorageAdapter.AuthRequestByCode()
           StorageAdapter.CreateAccessAndRefreshTokens()
              - token_svc.CreateAccessAndRefresh(...)
              - returns (accessTokenID, rawRefreshToken, accessExpiry)
           StorageAdapter.DeleteAuthRequest()
           → {access_token, refresh_token, id_token, token_type, expires_in}
```

### 5.2 Refresh Token flow

```
Client  →  POST /oauth/v2/token?grant_type=refresh_token&refresh_token=<raw>
           StorageAdapter.GetRefreshTokenInfo(raw) → (userID, tokenID)
           StorageAdapter.TokenRequestByRefreshToken(raw)
           StorageAdapter.CreateAccessAndRefreshTokens(request, currentRefreshToken=raw)
              → atomic tx: delete old RT + AT, insert new pair
           → new {access_token, refresh_token, ...}
```

### 5.3 Token Revocation

```
Client  →  POST /oauth/v2/revoke?token=<token>
           StorageAdapter.GetRefreshTokenInfo(token)
             if found → RevokeToken with userID != "" → RevokeRefreshToken
             if not found → RevokeToken with userID == "" → Revoke (access token)
```

### 5.4 Userinfo / Introspection

- `SetUserinfoFromScopes` / `SetUserinfoFromRequest`: scope-driven claim selection
  (openid → sub, profile → name/given_name/family_name/picture/updated_at,
   email → email/email_verified).
- `SetIntrospectionFromToken`: resolves token by ID, sets active=true/false,
  attaches user claims by scopes.

### 5.5 Provider configuration (op.Config)

| Flag | Value | Effect |
|---|---|---|
| `CodeMethodS256` | `true` | PKCE S256 enforced at provider level |
| `AuthMethodPost` | `true` | client_secret_post supported |
| `AuthMethodPrivateKeyJWT` | `false` | Disabled |
| `GrantTypeRefreshToken` | `true` | Refresh tokens enabled |
| `RequestObjectSupported` | `false` | JAR disabled |
| `SupportedUILocales` | en, ja | |

Public clients (AuthMethod=`none`) additionally enforce PKCE at
`CreateAuthRequest` time (storage.go:81-82).

---

## 6. Identity & Federated Login

### User model (identity.User)

```
ID            — ULID
Email         — from IdP, always overwritten on re-login
EmailVerified — from IdP
Name          — display name
GivenName
FamilyName
Picture       — avatar URL
CreatedAt / UpdatedAt
```

### Federated identity linking

- Lookup: JOIN users + federated_identities WHERE provider + provider_subject.
- First login → INSERT user + federated_identity in a TX.
- Subsequent logins → UPDATE user claims + federated_identity claims + last_login_at.
- **No support** for linking multiple providers to the same user account (no
  cross-provider merge, no "add another account" flow).
- **No unlink** operation.

### User identity update strategy

The user row is a **mirror** of the latest upstream claims. Every login
overwrites email, name, picture, etc. There is no "local profile" concept today.

---

## 7. Device Sessions

- Identified by a `dsid` cookie (ULID, 2-year max-age, HttpOnly, Secure, SameSite=Lax).
- `FindOrCreate`: if existing session row belongs to same user and is not
  revoked → update user_agent + ip_address + last_used_at. If user mismatch or
  revoked → generate fresh ULID and create new row.
- `RevokeByID(id, userID)`: marks `revoked_at`, cascades to delete all linked
  refresh_tokens in the same TX (immediate forced logout from that device).
- `ListActiveByUserID(userID)`: returns all non-revoked sessions, ordered by
  last_used_at DESC.
- Cleanup: `DeleteRevokedBefore(before)` — only deletes revoked sessions that
  have no live (unexpired) refresh tokens attached.
- Token repo also updates `last_used_at` on every refresh token creation.

---

## 8. Config & Operations

### Env vars

| Var | Required | Default |
|---|---|---|
| `ISSUER` | yes | — |
| `DATABASE_URL` | yes | — |
| `CRYPTO_KEY` | yes | 32 bytes hex |
| `SIGNING_KEY_PEM` | yes | RSA >= 2048 bit |
| `SIGNING_KEY_PREVIOUS_PEM` | no | Multiple via `---NEXT---` separator |
| `GOOGLE_CLIENT_ID/SECRET` | no* | — |
| `GITHUB_CLIENT_ID/SECRET` | no* | — |
| `ACCESS_TOKEN_LIFETIME_MINUTES` | no | 15 (1–60) |
| `REFRESH_TOKEN_LIFETIME_DAYS` | no | 7 (1–90) |
| `AUTH_REQUEST_TTL_MINUTES` | no | 30 (1–60) |
| `DB_MAX_OPEN_CONNS` | no | 25 |
| `RATE_LIMIT_ENABLED` | no | true |

\* At least one IdP is required.

### Subcommands

- `server` — runs the HTTP server + background goroutines
- `cleanup` — one-shot token GC (suitable for a Kubernetes CronJob)

### Background goroutines (inside `server`)

| Goroutine | Interval | Action |
|---|---|---|
| `runAuthRequestCleanup` | every AuthRequestTTL | DELETE auth_requests WHERE created_at < cutoff |
| `runTokenCleanupLoop` | every 1 hour | DELETE expired access + refresh tokens |
| `runDeviceSessionCleanup` | every 24 hours | DELETE old revoked device sessions |
| Rate limiter cleanup | every 10 minutes | Evict idle IP entries (15 min TTL) |

### Rate limiting

Three per-IP token buckets, each skipping private/loopback networks:
- `globalLimiter` — all routes, default 120 RPM, burst 30
- `loginLimiter` — `/login/*`, default 20 RPM, burst 5
- `tokenLimiter` — `/oauth/v2/token` + `/oauth/v2/introspect` only, default 60 RPM, burst 10

IP is read from `CF-Connecting-IP` (Cloudflare Tunnel) with fallback to
`RemoteAddr`. XFF is trusted only when RemoteAddr is 127.0.0.1/::1 (local proxy).

### Security headers

`X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`,
`Referrer-Policy: strict-origin-when-cross-origin`,
`Strict-Transport-Security: max-age=63072000; includeSubDomains; preload`.

No `Content-Security-Policy`. The login page loads Google Fonts from an external CDN.

### Key management

- Current key: `SIGNING_KEY_PEM` → derives kid from SHA-256 of DER-encoded public key (first 8 bytes, hex).
- Previous keys: `SIGNING_KEY_PREVIOUS_PEM` → multiple keys separated by `---NEXT---`.
- All keys served at `/.well-known/jwks.json`; RS256 only.
- In-memory at startup. Key rotation requires a restart (no hot reload).

### AES-256-GCM usage

Used **only** to protect the OAuth2 `state` parameter during the federated login
redirect (plaintext = JSON `{authRequestID, provider, nonce}`). The `nonce` field
is a UUID, adding uniqueness but **not verified** on decryption — the nonce is
effectively decorative today.

---

## 9. Honest Assessment: What Is Missing

### 9.1 No direct account management API

There is **no HTTP or gRPC endpoint** for:

- Updating a user's profile (name, picture, etc.) outside of a login event.
- Fetching a user's own profile without going through the OIDC `userinfo` endpoint.
- Listing or managing linked federated identities.
- Unlinking a federated identity.
- Deleting a user account.
- Listing active sessions (the `ListActiveByUserID` repo method exists but is
  never called from any endpoint).
- Revoking a specific device session via API (the revocation logic exists in the
  service and repo, but again no endpoint).

### 9.2 No multi-provider account linking

`FindByFederatedIdentity` returns `nil, nil` when no match is found (i.e., new
user). There is no code to find a pre-existing user by email and link the new
provider to them. Each unique `(provider, provider_subject)` always creates a
distinct user if email already exists via another provider.

### 9.3 No local profile editing

`UpdateUserFromClaims` is called on every login with upstream claims. Any local
edit would be overwritten on next login. The service has no `UpdateProfile`
operation on `identity.Service` or `identity.Repository`.

### 9.4 ErrAlreadyExists is dead code

`domerr.ErrAlreadyExists` is defined but never produced nor handled anywhere in
the codebase. The Postgres UNIQUE constraint on `federated_identities(provider,
provider_subject)` would produce a raw pq error on duplicate insert, which would
surface as a 500.

### 9.5 ACR is stub

`adapter/authrequest.go` `GetACR()` always returns `""`. If a downstream client
requests a specific acr_values (e.g., requiring MFA), the service cannot satisfy
it.

### 9.6 IP extraction is duplicated

`authn/ip.go` (`clientIP`) and `middleware/ratelimit.go` (`clientIP`) are two
separate implementations. They differ subtly: the authn version trusts
`CF-Connecting-IP` without validation; the middleware version validates it via
`net.ParseIP`.

### 9.7 No pagination on device session listing

`ListActiveByUserID` returns all active sessions with no limit or cursor.

### 9.8 No refresh token listing

There is no way to list active refresh tokens / active sessions for a user outside
of device sessions. A user cannot see "where am I logged in" beyond device
sessions.

### 9.9 No client management API

Clients are managed purely through SQL migrations. No CRUD API exists. Adding
a new downstream client requires a migration file + deployment.

### 9.10 Login page hardcoded to hss-science.org

The `select_provider.html` template contains hard-coded brand copy
(`hss-science.org`, placeholder `#` links for Terms/Privacy/Help). These are not
configurable from an environment variable.

### 9.11 Key rotation requires restart

Signing keys are loaded once at startup and held in memory. Hot key rotation
(without downtime) is not supported.

### 9.12 No audit log

Login events, token issuance, session revocations are not logged to a structured
audit store. Only structured `slog` to stdout (JSON, not queryable).

### 9.13 GitHub email fallback has a silent gap

If a GitHub user has no public email and the `/user/emails` API returns no
primary+verified email, `email` is stored as `""` and `email_verified = false`.
The user row will have an empty email. No error is surfaced; the login still
succeeds.

### 9.14 State nonce not verified

`federatedState.Nonce` is a UUID added to the encrypted state blob but never
checked against any stored value. It prevents deterministic ciphertext but does
not prevent replaying a captured state value within the cipher's integrity
guarantee (GCM already provides that). It is effectively redundant.

### 9.15 No observability beyond healthz/readyz

No Prometheus metrics, no tracing (despite OpenTelemetry being transitively in
the module graph via zitadel dependencies). The `go.mod` has
`go.opentelemetry.io/otel` but it is only an indirect dependency — nothing in
this service instruments spans or metrics.

---

## 10. What Exists That a gRPC API Can Build On

The foundations are well-laid:

| Existing asset | How it helps the gRPC API |
|---|---|
| `identity.Service` interface | `GetUser` and `FindOrCreateByFederatedLogin` are already usable. Extending `Service` with `UpdateProfile`, `DeleteUser`, `ListFederatedIdentities`, `UnlinkFederatedIdentity` is straightforward. |
| `identity.Repository` interface | Additional methods (update, delete, list identities) can be added without touching the service. |
| `oidc.DeviceSessionService` | `ListActiveByUserID` and `RevokeByID` are already implemented end-to-end — only an endpoint is missing. |
| `oidc.TokenService` | `DeleteByUserAndClient` exists for full session termination. `GetRefreshToken` returns AMR + device session ID. |
| `domerr` sentinel errors | gRPC interceptors can map ErrNotFound → codes.NotFound, ErrUnauthorized → codes.PermissionDenied, etc. |
| `go.mod` | Already has `google.golang.org/grpc`, `google.golang.org/protobuf`, `grpc-ecosystem/grpc-gateway/v2`. Infrastructure deps are resolved. |
| `buf.yaml` / `buf.gen.yaml` | Protobuf toolchain already configured in repo root — proto files can be added immediately. |
| Postgres schema | `users`, `federated_identities`, `device_sessions` are already normalised and indexed. |

---

## 11. Implications for gRPC Account Management API

### 11.1 Intended consumer: myaccount-bff

The `myaccount-bff` OIDC client is already seeded in migration 2. The BFF would:
1. Perform the OIDC flow on behalf of the user, obtaining a JWT access token.
2. Use that access token to call the gRPC Account Management API.

This means the gRPC API needs to **verify** the JWT access token as its
authentication mechanism. The token's `sub` claim (a ULID user ID) becomes the
authenticated principal.

### 11.2 Proposed API surface (not yet implemented)

Based on the existing domain and identified gaps:

```protobuf
service AccountManagement {
  // Profile
  rpc GetMyProfile(GetMyProfileRequest) returns (Profile);
  rpc UpdateMyProfile(UpdateMyProfileRequest) returns (Profile);
  rpc DeleteMyAccount(DeleteMyAccountRequest) returns (google.protobuf.Empty);

  // Federated identities
  rpc ListLinkedProviders(ListLinkedProvidersRequest) returns (ListLinkedProvidersResponse);
  rpc UnlinkProvider(UnlinkProviderRequest) returns (google.protobuf.Empty);

  // Sessions / devices
  rpc ListActiveSessions(ListActiveSessionsRequest) returns (ListActiveSessionsResponse);
  rpc RevokeSession(RevokeSessionRequest) returns (google.protobuf.Empty);
  rpc RevokeAllOtherSessions(RevokeAllOtherSessionsRequest) returns (google.protobuf.Empty);
}
```

### 11.3 What needs to be added to the existing code

1. **`identity.Repository`** — needs:
   - `ListFederatedIdentities(ctx, userID) ([]*FederatedIdentity, error)`
   - `DeleteFederatedIdentity(ctx, id, userID) error` (with guard: can't delete last identity)
   - `DeleteUser(ctx, userID) error`

2. **`identity.Service`** — needs:
   - `UpdateProfile(ctx, userID, fields) (*User, error)`
   - `ListLinkedProviders(ctx, userID) ([]*FederatedIdentity, error)`
   - `UnlinkProvider(ctx, userID, provider string) error`
   - `DeleteAccount(ctx, userID) error`

3. **`oidc.TokenService` / repo** — needs:
   - `ListRefreshTokensByUser(ctx, userID) ([]*RefreshToken, error)` (for full session list if not using device sessions exclusively)
   - `DeleteAllByUser(ctx, userID)` (for account deletion cascade)

4. **`oidc.DeviceSessionService`** — already has `ListActiveByUserID` and `RevokeByID` — no new domain methods needed, just expose via gRPC.

5. **gRPC server** — new `server/services/identity-service/internal/grpc/` package:
   - JWT interceptor: extract + validate access token, inject user ID into context.
   - Handler implementations calling domain services.
   - Error mapping interceptor: domerr → gRPC status codes.

6. **Protobuf definitions** — new `.proto` file under the repo's `buf.yaml`-managed path.

7. **Profile update conflict** — `UpdateUserFromClaims` on login overwrites
   all profile fields from the upstream provider. The team must decide: do user-edited
   fields survive a re-login, or does upstream always win? The schema has no
   `overridden_*` columns today.

8. **Token verification in gRPC** — The gRPC API must verify the JWT issued by
   this same service. It can do this by calling `StorageAdapter.GetByID` (access
   token lookup) or by verifying the RS256 signature against the public key in
   memory (preferred: avoids DB hop for every RPC, but needs key set accessible
   to the gRPC server).

### 11.4 Modular monolith wiring

Because everything is in the same binary (`main.go` wires all services), adding
a gRPC listener is straightforward:

```go
// in runServer():
grpcSrv := grpc.NewServer(grpc.ChainUnaryInterceptor(
    jwtAuthInterceptor(signingKey),
    errorMappingInterceptor(),
))
accountMgmtpb.RegisterAccountManagementServer(grpcSrv, grpchandler.New(identitySvc, deviceSessionSvc, tokenSvc))
go grpcSrv.Serve(grpcListener)
```

The HTTP server (OIDC) and gRPC server (account management) run in the same
process sharing the same service instances and DB connection pool. No separate
deployment needed.

### 11.5 Authorization boundary

The gRPC API should enforce:
- **Authenticated**: valid JWT required (interceptor).
- **Self-only**: the `sub` from the token must match the resource being operated
  on. No admin/super-user reads of other users' data (unless a separate admin role
  is introduced).
- **Last-provider guard**: `UnlinkProvider` must reject if it would leave the user
  with zero federated identities (they'd be locked out).

---

## 12. Decisions That Should Be Made Before Implementing

1. **Profile ownership**: Do local edits survive re-login (upstream always
   overwrites)? Needs a `profile_overrides` column strategy or a "local edit wins"
   flag per field.

2. **Account deletion semantics**: Cascade-delete vs. soft-delete vs. anonymise?
   `users.id` is referenced by `refresh_tokens` (hard FK) and `device_sessions`
   (ON DELETE CASCADE). A DELETE would cascade device sessions but tokens would
   FK-fail unless handled.

3. **gRPC port vs. HTTP/2 on same port**: Use a different port (e.g., 9090) or
   use a mux that demuxes gRPC and HTTP/1.1 on the same port?

4. **Token verification strategy for gRPC**: In-process JWKS lookup vs. local
   DB lookup vs. calling own `/introspect` endpoint?

5. **Multi-provider linking**: Do we add a flow for linking a second provider to an
   existing account? Currently impossible as there's no email-based merge.

6. **Proto file location**: Under `server/services/identity-service/proto/` or the
   existing repo-level proto path managed by `buf.yaml`?
