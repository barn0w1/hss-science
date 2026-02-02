# Accounts Domain Model (SSO)

## Goals
- Provide SSO via `accounts.hss-science.org` with a single session cookie shared across subdomains.
- Issue short-lived, one-time auth codes for service login.
- Keep data model minimal: `User`, `Session`, `AuthCode`.
- Prefer security-first defaults (opaque tokens, hashed storage).

## Non-Goals
- Refresh tokens / access tokens for end users.
- Frontend UI or OAuth provider management beyond Discord.
- Backward compatibility with the previous schema.

## Entities

### User
Represents a system-wide identity (Discord-backed).

Fields:
- `ID` (UUID)
- `DiscordID` (string, immutable)
- `Name` (string)
- `AvatarURL` (string, optional)
- `Role` (`system_admin | moderator | user`)
- `CreatedAt`, `UpdatedAt`

Invariants:
- `DiscordID` is unique.
- `Role` is in the defined set.

### Session
Represents an accounts-level login state bound to a browser cookie.

Fields:
- `TokenHash` (string, SHA-256 hex)
- `UserID` (UUID)
- `ExpiresAt`, `CreatedAt`, `RevokedAt`
- `UserAgent`, `IPAddress` (optional, for audit)

Behavior:
- `IsExpired(now)`
- `IsRevoked()`
- `IsValid(now)`
- `Revoke(now)`

### AuthCode
One-time authorization code issued to a service (e.g., drive).

Fields:
- `CodeHash` (string, SHA-256 hex)
- `UserID` (UUID)
- `Audience` (string)
- `RedirectURI` (string)
- `CreatedAt`, `ExpiresAt`, `ConsumedAt`

Behavior:
- `IsExpired(now)`
- `IsConsumed()`
- `Consume(now)`

## Token Design

### Raw Tokens
- Session cookie value and auth code are **opaque random tokens**.
- Generated with `crypto/rand` and encoded using `base64.RawURLEncoding`.
- Default size: 32 bytes (≈ 256 bits).

### Storage
- Only **SHA-256 hashes** of tokens are stored in the database.
- Raw tokens are never persisted or logged.

Rationale: a DB leak does not expose valid session/auth tokens.

## Cookie Spec (accounts session)
- Name: `__Secure-accounts_session` (configurable)
- Domain: `.hss-science.org` (prod), empty for localhost
- Path: `/`
- HttpOnly: `true`
- Secure: `true` (prod), `false` (dev)
- SameSite: `Lax` (default)
- Expires: aligned with session TTL

Notes:
- `SameSite=Lax` is sufficient for top-level OAuth redirects across subdomains.
- If cross-site embedding is introduced, switch to `SameSite=None` + `Secure`.

## Auth Code Spec
- Opaque token, 32 bytes, base64url encoded.
- TTL: short (e.g., 60 seconds).
- One-time use (consume on successful verification).
- Stored as `code_hash` (SHA-256 hex) in DB.

## Repository Interfaces
- `SessionRepository.GetByTokenHash(tokenHash string)`
- `AuthCodeRepository.GetByCodeHash(codeHash string)`
- `AuthCodeRepository.Consume(codeHash string, consumedAt time.Time)`

Hashing is done in the domain/usecase layer; repositories never see raw tokens.

## Database Schema (v2)

```
users
- id (uuid, pk)
- discord_id (unique)
- name
- avatar_url
- role
- created_at
- updated_at

sessions
- token_hash (char(64), pk)
- user_id (fk)
- expires_at
- created_at
- revoked_at
- user_agent
- ip_address

auth_codes
- code_hash (char(64), pk)
- user_id (fk)
- audience
- redirect_uri
- expires_at
- consumed_at
- created_at
```

Indexes:
- `sessions(user_id)`
- `sessions(expires_at)`
- `sessions(user_id) WHERE revoked_at IS NULL`
- `auth_codes(user_id)`
- `auth_codes(audience)`
- `auth_codes(expires_at)`

