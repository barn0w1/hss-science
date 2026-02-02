# Accounts Service Runtime Notes

## Endpoints
- Public HTTP
  - `GET /v1/authorize`
  - `GET /v1/oauth/callback`
- Internal gRPC
  - `VerifyAuthCode`

## Cookie Behavior
- Cookie is issued only after OAuth callback.
- Cookie value is the raw session token (opaque, random).
- DB stores only `token_hash` (SHA-256 hex).
- Default cookie name:
  - prod: `__Secure-accounts_session`
  - dev: `accounts_session`
- `SameSite=Lax` by default.

## Redirect Handling
- `redirect_uri` must be absolute (scheme + host) and must not include fragments.
- Redirect URL is reconstructed using parsed query (no string concatenation).
- No redirect whitelist by design (development stage).

## OAuth (Discord)
- OAuth HTTP calls use a dedicated `http.Client` with a configured timeout.
- State is HMAC signed and time-bounded.

## Server Configuration
The service is fully configured via environment variables.

### OAuth / Security
- `STATE_SECRET`
- `SESSION_TTL_HOURS`
- `AUTH_CODE_TTL_SECONDS`
- `OAUTH_STATE_TTL_SECONDS`
- `OAUTH_HTTP_TIMEOUT_SECONDS`

### Cookie
- `SESSION_COOKIE_NAME`
- `COOKIE_DOMAIN`
- `COOKIE_SECURE`
- `COOKIE_SAMESITE`

### HTTP Server
- `HTTP_READ_TIMEOUT_SECONDS`
- `HTTP_WRITE_TIMEOUT_SECONDS`
- `HTTP_IDLE_TIMEOUT_SECONDS`
- `HTTP_READ_HEADER_TIMEOUT_SECONDS`
- `HTTP_SHUTDOWN_TIMEOUT_SECONDS`

### Database Pool
- `DB_CONNECT_TIMEOUT_SECONDS`
- `DB_MAX_OPEN_CONNS`
- `DB_MAX_IDLE_CONNS`
- `DB_CONN_MAX_LIFETIME_MINUTES`
- `DB_CONN_MAX_IDLE_TIME_MINUTES`

