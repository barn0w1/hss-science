# Research Report: Accounts Service — Federated Login & OIDC Flow

## 1. System Overview

The `accounts` service is a self-contained OpenID Connect (OIDC) Identity Provider (IdP) built in Go. It acts as a **federated identity broker**: it has no local credential store (no passwords), and instead delegates all actual authentication to upstream OAuth 2.0 / OIDC providers (Google, GitHub). Once an upstream provider authenticates the user, the service issues its own OIDC tokens to downstream relying-party applications.

The service uses **zitadel/oidc v3** as its OIDC provider framework, **chi** as the HTTP router, and **PostgreSQL** as the sole persistent store. There is no in-memory session, no cookie, and no browser-side state beyond what is carried through URL parameters during the login flow.

---

## 2. Route Layout

```
GET  /login/                  → SelectProvider     (show IdP picker UI)
POST /login/select            → FederatedRedirect  (redirect to upstream IdP)
GET  /login/callback          → FederatedCallback  (receive upstream callback)

GET  /healthz                 → liveness probe
GET  /readyz                  → readiness probe (DB ping)
GET  /logged-out              → post-logout landing page

/*   (mounted: op.Provider)   → all OIDC standard endpoints
     /.well-known/openid-configuration
     /oauth/v2/authorize
     /oauth/v2/token
     /oauth/v2/introspect
     /logout
     /keys
     etc.
```

The OIDC framework (`op.Provider`) mounts at the root and handles all standard OIDC endpoints. The `/login/*` routes are the custom federated-login UI, placed under a rate-limiter-gated sub-router and wrapped with `op.NewIssuerInterceptor` (which injects the issuer into the request context, required by the OIDC library).

---

## 3. End-to-End Federated Login Flow

### Step 0 — Client initiates OIDC Authorization Code flow

A relying party (e.g. `myaccount-bff`) sends the user to:

```
GET /oauth/v2/authorize?client_id=myaccount-bff&response_type=code
    &redirect_uri=https://myaccount.hss-science.org/api/v1/auth/callback
    &scope=openid email profile offline_access
    &code_challenge=<S256 verifier>
    &code_challenge_method=S256
    &state=<rp-state>
    &nonce=<nonce>
```

### Step 1 — `CreateAuthRequest` (OIDC framework → `StorageAdapter`)

The `op.Provider` framework validates the request parameters and calls `StorageAdapter.CreateAuthRequest`. This method:

1. Constructs a domain `oidc.AuthRequest` struct and assigns it a **ULID** as its primary key.
2. Checks if the client is a public client (auth_method = `"none"`); if so, enforces that `code_challenge` is present (PKCE S256 mandatory).
3. Persists it to the `auth_requests` table via `AuthRequestService.Create` → `AuthRequestRepository.Create`.
4. Returns an `adapter.AuthRequest` (wrapping the domain struct) that satisfies the `op.AuthRequest` interface.

The auth request row at this point has `user_id = NULL`, `auth_time = NULL`, `amr = {}`, `is_done = false`, `code = NULL`.

### Step 2 — Framework redirects the user to the login UI

The OIDC framework's authorization endpoint, upon receiving the `op.AuthRequest` from storage, calls `client.LoginURL(authRequestID)`, which is implemented as:

```go
func (c *ClientAdapter) LoginURL(id string) string {
    return "/login?authRequestID=" + id
}
```

The browser is redirected to `/login?authRequestID=<ULID>`.

### Step 3 — `GET /login/` → `SelectProvider`

`handler.SelectProvider` reads `authRequestID` from the query string and renders a minimal HTML page listing the configured providers (Google, GitHub). Each provider is rendered as a `<form method="POST" action="/login/select">` with hidden fields for `authRequestID` and `provider`. No database lookup or auth request validation happens here.

### Step 4 — `POST /login/select` → `FederatedRedirect`

`handler.FederatedRedirect`:

1. Reads `authRequestID` and `provider` from the POST form.
2. Looks up the provider by name in the in-memory `providerMap`.
3. Constructs a `federatedState` struct:
   ```go
   type federatedState struct {
       AuthRequestID string `json:"a"`
       Provider      string `json:"p"`
       Nonce         string `json:"n"`  // random UUID
   }
   ```
4. **Encrypts** the state using AES-GCM (the `CRYPTO_KEY` config variable) and base64-encodes the result. This is the `state` parameter sent to the upstream IdP.
5. Redirects the browser to the upstream IdP's OAuth 2.0 authorization URL, passing `state=<encrypted>` and `access_type=offline`.

Key details:
- There is **no CSRF token separately stored** in a session. The encryption of the state provides integrity, but because there is no server-side state tied to the user's browser, any party with the encrypted blob could complete the flow. The nonce inside the state is not verified against anything server-side.
- The upstream OAuth2 `state` parameter does double duty as both CSRF protection and a transport for the `authRequestID`.

### Step 5 — Upstream IdP authenticates the user

The user authenticates with Google or GitHub. The upstream IdP redirects back to the **single shared callback URL**:

```
GET /login/callback?code=<upstream-code>&state=<encrypted-state>
```

### Step 6 — `GET /login/callback` → `FederatedCallback`

`handler.FederatedCallback`:

1. Reads `code` and `state` from query params.
2. **Decrypts** and JSON-unmarshals the state to recover `authRequestID`, `provider`, and the nonce (the nonce is not validated — it is present in the struct but discarded after decryption).
3. Exchanges the upstream `code` for tokens via `provider.OAuth2Config.Exchange`.
4. Calls `provider.Claims.FetchClaims` to retrieve the user's identity claims from the upstream IdP.
5. Calls `loginUC.Execute` (the `CompleteFederatedLogin` use case).
6. Calls `op.AuthCallbackURL(provider)(ctx, authRequestID)` to get the OIDC framework's authorization callback URL (e.g. `/oauth/v2/callback?id=<authRequestID>`).
7. Redirects the browser to that URL, completing the OIDC framework's authorization code flow.

#### 6a. Google claims (`googleClaimsProvider.FetchClaims`)
- Extracts the `id_token` from the OAuth2 token response.
- Verifies it using `go-oidc`'s `IDTokenVerifier` (signature + audience + expiry).
- Parses claims: subject (`sub`), email, email_verified, name, given_name, family_name, picture.

#### 6b. GitHub claims (`githubClaimsProvider.FetchClaims`)
- Calls `GET https://api.github.com/user` with the access token.
- Uses `strconv.FormatInt(ghUser.ID, 10)` as the subject (numeric GitHub user ID).
- If the primary email is absent from the `/user` response, falls back to `GET https://api.github.com/user/emails` to find the primary verified email. `email_verified` is set to `true` only when obtained from the emails endpoint.

---

## 4. `CompleteFederatedLogin` Use Case

```go
func (uc *CompleteFederatedLogin) Execute(ctx, provider, claims, authRequestID) (userID, error)
```

1. Calls `identity.FindOrCreateByFederatedLogin(ctx, provider, claims)` — resolves or creates the local user.
2. Sets `authTime = time.Now().UTC()` and `amr = []string{"fed"}`.
3. Calls `loginCompleter.CompleteLogin(ctx, authRequestID, user.ID, authTime, amr)`.

`loginCompleter` is `authReqSvc` (an `AuthRequestService`), which delegates to:

```sql
UPDATE auth_requests
SET user_id = $1, auth_time = $2, amr = $3, is_done = true
WHERE id = $4
```

After this update, the auth request row has:
- `user_id = <local user ULID>`
- `auth_time = <now>`
- `amr = {"fed"}`
- `is_done = true`

---

## 5. User Identity: `FindOrCreateByFederatedLogin`

The identity service implements a **find-or-create** pattern keyed on `(provider, provider_subject)`:

### Lookup
```sql
SELECT u.* FROM users u
JOIN federated_identities fi ON fi.user_id = u.id
WHERE fi.provider = $1 AND fi.provider_subject = $2
```

### If user exists (returning login)
- Updates `users` row with fresh claims (email, name, picture, etc.) — always overwrites from upstream IdP.
- Updates `federated_identities` row with fresh claims and sets `last_login_at = now()`.
- Returns the (in-memory updated) `User` struct.

### If user does not exist (first login)
- Generates a new ULID for both `users.id` and `federated_identities.id`.
- Inserts both rows atomically in a transaction.

### Key observations
- **No email-based linking**: Two different upstream IdPs providing the same email address will create two separate local `users` rows. There is no merging of accounts based on email.
- **User profile is always overwritten**: Every login from the upstream IdP overwrites the local profile fields unconditionally. There is no "canonical" profile that a user can edit independently.
- **Single federated identity per provider per user**: The `UNIQUE(provider, provider_subject)` constraint prevents duplicate rows. A user can theoretically link both Google and GitHub accounts if they arrive via both (two separate `federated_identities` rows pointing to the same `users` row) — but that is not currently supported because there is no linking flow. Each login path that produces a new `(provider, subject)` pair will create a new `users` row.

---

## 6. OIDC Authorization Completion (Framework Side)

After `FederatedCallback` redirects to `/oauth/v2/callback?id=<authRequestID>`, the `op.Provider` framework:

1. Calls `StorageAdapter.AuthRequestByID` — fetches the (now `is_done=true`) auth request.
2. Checks `authRequest.Done()` — returns `true`.
3. Calls `StorageAdapter.SaveAuthCode` — generates a random authorization code, calls:
   ```sql
   UPDATE auth_requests SET code = $1 WHERE id = $2
   ```
4. Redirects the browser back to the RP's `redirect_uri` with `?code=<code>&state=<rp-state>`.

---

## 7. Token Issuance

The RP's BFF calls `POST /oauth/v2/token` with `grant_type=authorization_code` and the PKCE verifier.

The framework calls:
1. `StorageAdapter.AuthRequestByCode` — looks up the auth request by code.
2. `StorageAdapter.CreateAccessAndRefreshTokens` — creates both tokens.

### Access token
- Token ID is a ULID, stored opaquely in `tokens` table.
- The client uses JWT access tokens (`access_token_type = 'jwt'` in the `clients` table). The framework wraps the opaque ID in a signed JWT using the RSA signing key.
- Default lifetime: 15 minutes (configurable 1–60 min).

### Refresh token
- A 32-byte random value, base64url-encoded, returned raw to the client.
- Stored **hashed** (SHA-256 hex) in `refresh_tokens.token_hash`.
- Linked to the access token via `refresh_tokens.access_token_id`.
- Carries `auth_time` and `amr` from the original auth request.
- Default lifetime: 7 days (configurable 1–90 days).
- **Rotation is enforced**: on refresh, the old refresh token and its linked access token are deleted atomically. If the refresh token is already used or expired, `domerr.ErrNotFound` is returned → `op.ErrInvalidRefreshToken`.

### ID token
- Built by the framework using `SetUserinfoFromRequest` → `setUserinfo`, which queries the `users` table via `userClaimsBridge.UserClaims`.
- Claims populated by scope:
  - `openid`: sets `sub` (user ULID)
  - `profile`: sets `name`, `given_name`, `family_name`, `picture`, `updated_at`
  - `email`: sets `email`, `email_verified`
- `acr` is always empty string (`GetACR()` returns `""`).
- `amr` is `["fed"]` for all logins.
- `auth_time` is the timestamp set during `CompleteLogin`.

### Signing
- RSA-RS256, key ID derived from SHA-256 of the DER-encoded public key (first 8 bytes, hex).
- Key rotation supported: `SIGNING_KEY_PREVIOUS_PEM` can hold multiple previous keys (separated by `---NEXT---`), all published in `GET /keys` (JWKS endpoint) for verification.

---

## 8. Database Schema Summary

| Table | Key Columns | Notes |
|---|---|---|
| `users` | `id` (ULID), `email`, `email_verified`, `name`, `given_name`, `family_name`, `picture` | Local user record, always synced from upstream IdP on each login |
| `federated_identities` | `id`, `user_id` (FK→users), `provider`, `provider_subject`, `last_login_at` | UNIQUE on `(provider, provider_subject)` |
| `clients` | `id`, `secret_hash`, `redirect_uris`, `grant_types`, `allowed_scopes`, etc. | Statically seeded via migration |
| `auth_requests` | `id` (ULID), `client_id`, PKCE fields, `user_id` (nullable), `auth_time`, `amr`, `is_done`, `code`, `created_at` | Transient; TTL-cleaned |
| `tokens` | `id` (ULID), `client_id`, `subject`, `audience`, `scopes`, `expiration`, `refresh_token_id` | Access tokens |
| `refresh_tokens` | `id`, `token_hash` (SHA-256), `user_id` (FK→users), `auth_time`, `amr`, `access_token_id`, `expiration` | Refresh tokens; rotation enforced |

---

## 9. Absence of Session Management and Device Tracking

This is the most significant architectural characteristic of the current system:

### No local IdP session
There is **no session cookie** set by this IdP. After `FederatedCallback` completes, the browser is redirected to the RP with an authorization code. The IdP does not set any first-party cookie. Consequently:

- **Every OIDC authorization flow requires a full upstream IdP round-trip.** If a user visits the RP again and the RP initiates a new authorization request, the user will always land on the provider picker (`/login/`) and then be sent to the upstream IdP. What happens next depends on the upstream IdP's own session (Google/GitHub may silently re-authenticate if the user is still logged in there).
- **`prompt=none` will always fail**. The `is_done` field on a new auth request starts as `false`. Since there is no local session to associate with the new request, the IdP cannot mark it `is_done=true` without a new upstream login. The OIDC `prompt=none` parameter (which requests authentication without user interaction) is stored in `auth_requests.prompt` but is never acted upon — there is no code path that checks for a pre-existing authenticated session and auto-completes an auth request.
- **`max_age` is not enforced** in any meaningful way. It is stored in `auth_requests.max_age` but the system never checks whether a user has an existing session or when they last authenticated.
- **`login_hint` is stored** in `auth_requests.login_hint` but is not used — there is no code that reads it to pre-select a provider or skip the picker.
- **Single logout / front-channel logout** is not fully functional. `TerminateSession` deletes tokens for a `(userID, clientID)` pair, but since there is no IdP session cookie to destroy, there is nothing to invalidate at the IdP level.

### No device tracking
- There is no concept of a device, session, or browser fingerprint in the data model.
- `refresh_tokens` does carry `auth_time` and `amr`, and is linked to a user, but there is no device ID, user agent, IP address, or named session associated with it.
- A user cannot inspect or revoke individual "sessions" because none are represented — only bare refresh tokens, which have no human-readable context.

### `amr` is always `["fed"]`
The Authentication Methods Reference is hardcoded to `["fed"]` in `login_usecase.go`:
```go
amr := []string{"fed"}
```
This accurately indicates federated authentication but provides no further detail (e.g. whether the upstream used MFA). The IdP has no way to know whether Google/GitHub required a second factor.

### State/nonce lifecycle
- The encrypted `state` parameter is entirely stateless on the server side. The nonce embedded in it (`federatedState.Nonce`) is generated but **never stored or validated** — it is present but unchecked. This means the nonce provides no additional protection beyond what the encryption already provides.
- Auth requests are cleaned up by a background goroutine that runs on every `AuthRequestTTLMinutes` tick, deleting rows where `created_at < cutoff`.

---

## 10. Configuration and Key Management

| Variable | Required | Default | Notes |
|---|---|---|---|
| `ISSUER` | yes | — | Must be valid URL |
| `DATABASE_URL` | yes | — | PostgreSQL DSN |
| `CRYPTO_KEY` | yes | — | 32-byte AES key, hex-encoded; used for OAuth2 state encryption AND zitadel OIDC cookie encryption |
| `SIGNING_KEY_PEM` | yes | — | RSA private key (PKCS#1 or PKCS#8), ≥2048 bits |
| `SIGNING_KEY_PREVIOUS_PEM` | no | — | `---NEXT---`-separated old keys for JWKS rotation |
| `GOOGLE_CLIENT_ID` / `_SECRET` | conditional | — | At least one IdP required |
| `GITHUB_CLIENT_ID` / `_SECRET` | conditional | — | At least one IdP required |
| `ACCESS_TOKEN_LIFETIME_MINUTES` | no | 15 | 1–60 |
| `REFRESH_TOKEN_LIFETIME_DAYS` | no | 7 | 1–90 |
| `AUTH_REQUEST_TTL_MINUTES` | no | 30 | 1–60 |

The `CRYPTO_KEY` is used for two separate purposes:
1. **AES-GCM encryption** of the OAuth2 `state` parameter (`crypto.NewAESCipher(cfg.CryptoKey)`).
2. **zitadel OIDC framework's internal crypto** (`op.NewProvider(config, storage, ..., cryptoKey)`), which the framework uses to sign/encrypt cookies and internal tokens.

This dual use of a single key is a potential concern: rotation of the state-encryption key would also affect the OIDC framework's internal state.

---

## 11. Rate Limiting

Three independent in-memory per-IP token-bucket rate limiters are applied:

| Limiter | Scope | Default RPM | Burst |
|---|---|---|---|
| `globalLimiter` | All routes | 120 | 30 |
| `loginLimiter` | `/login/*` only | 20 | 5 |
| `tokenLimiter` | `/oauth/v2/token`, `/oauth/v2/introspect` only | 60 | 10 |

Rate limiting is enabled by default (`RATE_LIMIT_ENABLED` defaults to `true` unless set to `"false"`). Limiter state is per-process (not shared across replicas) and cleaned up every 10 minutes (dropping entries older than 15 minutes).

---

## 12. Supported OIDC Features (Summary)

| Feature | Status |
|---|---|
| Authorization Code flow | ✅ Supported |
| PKCE (S256) | ✅ Required for public clients |
| Refresh Token grant | ✅ Supported with rotation |
| Client Credentials grant | ✅ Supported |
| Implicit flow | ✅ Declared (response_types can include `token`), but no specific tests |
| JWT access tokens | ✅ (configured per client; `myaccount-bff` uses JWT) |
| Bearer (opaque) access tokens | ✅ (switch per client) |
| Token introspection | ✅ |
| Token revocation | ✅ |
| End-session (logout) | ⚠️ Partial — deletes tokens but no IdP session to destroy |
| JWKS key rotation | ✅ Via `SIGNING_KEY_PREVIOUS_PEM` |
| `prompt=none` / SSO | ❌ Not implemented |
| `max_age` enforcement | ❌ Stored but not enforced |
| `login_hint` | ❌ Stored but not used |
| ACR | ❌ Always empty |
| JWT Profile grant | ❌ Explicitly unsupported |
| Request objects | ❌ Disabled (`RequestObjectSupported: false`) |
| Front-channel / back-channel logout | ❌ Not implemented |
| Local sessions / SSO across RPs | ❌ No local session concept |
| Device tracking / named sessions | ❌ Not modeled |

---

## 13. Implicit Assumptions and Potential Gaps

1. **One user per (provider, subject) pair — no account linking.** A user who signs in with Google and later with GitHub will get two separate accounts. There is no account-linking or merging flow.

2. **Auth request is not validated at `/login/` render time.** `SelectProvider` does not check whether the `authRequestID` in the query string actually exists or has not expired. An attacker could craft a URL with a fake or expired ID; the error would only surface later when the OIDC framework tries to complete the flow.

3. **No `prompt` parameter handling.** The `prompt` array is stored in the `auth_requests` row but is never read by any application code. Standard `prompt=login` (force re-authentication) or `prompt=select_account` (show account picker) have no effect.

4. **`federatedState.Nonce` is decorative.** A nonce is generated and embedded in the encrypted state, but it is never stored server-side and never validated on callback. Its only contribution is slightly increasing entropy of the plaintext before encryption — it does not protect against replay within the same AES-GCM nonce space.

5. **`RevokeToken` logic is inverted.** In `StorageAdapter.RevokeToken`:
   ```go
   if userID != "" {
       s.tokens.Revoke(...)      // revoke access token
   } else {
       s.tokens.RevokeRefreshToken(...)  // revoke refresh token
   }
   ```
   The branching is on whether `userID` is non-empty. However, `userID` is a parameter comes from the zitadel framework's revocation handler, and the intent was to branch on whether the token being revoked *is* a refresh token vs. an access token. The current logic is semantically fragile — it works incidentally because the framework passes a non-empty `userID` only for access token revocation requests, but this is not a documented or guaranteed contract.

6. **`auth_requests.auth_time` field uses application-level `time.Now()`** rather than a database-generated timestamp. This is susceptible to clock skew in multi-instance deployments.

7. **Background cleanup goroutine for auth requests has an off-by-one period.** The cleanup runs every `AuthRequestTTLMinutes` and deletes rows where `created_at < now() - TTL`. However, the ticker period equals the TTL, so in the worst case a row could survive up to `2 × TTL` before being cleaned.

8. **No tracing / correlation IDs.** The service uses structured logging with `slog` but does not propagate request correlation IDs through the flow. Correlating a login failure across the `FederatedCallback` → `CompleteLogin` → `FindOrCreate` chain requires matching timestamps manually.
