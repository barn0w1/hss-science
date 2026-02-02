# Accounts Domain Refresh Plan

## Objective
Replace the existing accounts domain model and repositories with a minimal, secure SSO design focused on `User`, `Session`, and `AuthCode`. Backward compatibility is not required.

## Steps
1. **Domain Model**
   - Define `User`, `Session`, `AuthCode` with clear invariants and lifecycle methods.
   - Introduce token helpers: secure generation + SHA-256 hashing.

2. **Repository Interfaces**
   - Update interfaces to accept token hashes (not raw tokens).
   - Align repository methods with the new model.

3. **Usecase + Handlers**
   - Generate raw tokens for cookies and auth codes.
   - Hash tokens before DB persistence.
   - Read cookies and auth codes as raw strings, then hash for lookups.

4. **Database Schema**
   - Replace session/auth code primary keys with `token_hash` / `code_hash`.
   - Add/keep indices for TTL cleanup and per-user lookup.

5. **Docs & Validation**
   - Keep `docs/accounts/domain-model.md` up to date.
   - Run build/tests after refactor.

