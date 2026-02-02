# Flow: Auth SSO

## Goal
- Subdomain 間で安全にログインさせる

## Actors
- Browser
- accounts server
- Discord
- services (e.g., cloud storage "drive")

## Preconditions
- drive に未ログイン

## Steps
1. Browser → drive
2. drive → 302 → accounts /authorize?audience=drive&redirect_uri=...
3. accounts:
   - session exists → step 6
   - session missing → step 4
4. accounts → 302 → Discord OAuth
5. Discord → /oauth/callback
   - create session cookie
6. accounts:
   - issue auth code
   - 302 → drive callback?code=...
7. drive:
   - VerifyAuthCode (internal gRPC)
   - issue drive session

## Data / State Changes
- accounts: Session created
- accounts: AuthCode created (short-lived)
- drive: Session created

## Error Cases
- expired auth code → 401
- audience mismatch → 403

## Security Notes
- auth code is one-time
- state param for CSRF
