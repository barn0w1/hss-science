# Deep Technical Research: `github.com/zitadel/oidc/v3` Library Internals

## 1. Architectural Overview

The library is split into three layers:

| Layer | Package | Role |
|-------|---------|------|
| **Protocol Types** | `pkg/oidc` | Pure data: request/response structs, error types, OIDC constants, JWT claims, verifiers. Zero HTTP logic. |
| **Provider Engine** | `pkg/op` | The OIDC Provider runtime: HTTP routing, request validation, token creation, signing, discovery. Delegates all persistence to a `Storage` interface the host must implement. |
| **Support** | `pkg/crypto`, `pkg/http` | AES encrypt/decrypt, JOSE signing helpers, HTTP marshal/cookie utilities, form decoder. |

There is no database driver, no migration, no ORM. All state management is the host's responsibility via the `op.Storage` interface contract.

---

## 2. The Two Provider APIs (Legacy vs. Server)

The library exposes **two** ways to build an OpenID Provider. Understanding both is critical because the example uses the Legacy path, but the `Server` path is the newer, more explicit API.

### 2.1 Legacy Path: `op.NewOpenIDProvider`

```
op.NewOpenIDProvider(issuer string, config *op.Config, storage op.Storage, opts ...op.Option) (op.OpenIDProvider, error)
```

- Returns a `*Provider` which implements `OpenIDProvider` **and** `http.Handler`.
- Internally creates a `chi.Router`, registers all OIDC endpoints, wires up discovery, CORS, probes, signing key, decoder, and a `Crypto` (AES) instance from `config.CryptoKey`.
- The `OpenIDProvider` is itself the HTTP handler -- you mount it on your router with `router.Mount("/", provider)`.
- Endpoint paths are hardcoded defaults (`/authorize`, `/oauth/token`, `/oauth/introspect`, etc.) unless customized via `op.WithCustom*Endpoint` options.
- The constructor calls `storage.SigningKey(ctx)` and `SignerFromKey()` to pre-build a `jose.Signer` at startup.

### 2.2 New Path: `op.RegisterServer` (Experimental, pre-v4)

```
op.RegisterServer(server op.Server, endpoints op.Endpoints, options ...op.ServerOption) http.Handler
```

- You implement the full `op.Server` interface (20+ methods) yourself.
- `RegisterServer` builds a `chi.Router` that parses requests, calls your `Server` methods, and serializes responses.
- Gives complete control over every endpoint's business logic.
- `op.NewLegacyServer(provider, endpoints)` exists as a bridge: wraps an `OpenIDProvider` to satisfy the `Server` interface using the old storage-based flow.

**Key takeaway**: For our use case (custom federated auth, full control), we will likely either:
- Use the Legacy path with a custom `op.Storage`, or
- Use the `Server` path for full control and implement the interface directly.

The Legacy path is simpler; the Server path is more explicit but requires implementing every protocol operation.

---

## 3. The `op.Storage` Interface -- The Central Contract

This is the single most important interface. Every persistence and identity operation is delegated here. The library calls these methods during its protocol flows. Here is the **complete** interface:

```go
type Storage interface {
    // === Auth Request Lifecycle ===
    CreateAuthRequest(ctx context.Context, authReq *oidc.AuthRequest, userID string) (AuthRequest, error)
    AuthRequestByID(ctx context.Context, id string) (AuthRequest, error)
    AuthRequestByCode(ctx context.Context, code string) (AuthRequest, error)
    SaveAuthCode(ctx context.Context, id string, code string) error
    DeleteAuthRequest(ctx context.Context, id string) error

    // === Token Creation ===
    CreateAccessToken(ctx context.Context, request TokenRequest) (accessTokenID string, expiration time.Time, err error)
    CreateAccessAndRefreshTokens(ctx context.Context, request TokenRequest, currentRefreshToken string) (accessTokenID string, newRefreshTokenID string, expiration time.Time, err error)
    TokenRequestByRefreshToken(ctx context.Context, refreshToken string) (RefreshTokenRequest, error)

    // === Session Management ===
    TerminateSession(ctx context.Context, userID string, clientID string) error

    // === Token Revocation ===
    RevokeToken(ctx context.Context, tokenIDOrToken string, userID string, clientID string) *oidc.Error
    GetRefreshTokenInfo(ctx context.Context, clientID string, token string) (userID string, tokenID string, err error)

    // === Signing / Crypto ===
    SigningKey(ctx context.Context) (SigningKey, error)
    SignatureAlgorithms(ctx context.Context) ([]jose.SignatureAlgorithm, error)
    KeySet(ctx context.Context) ([]Key, error)

    // === Client Registry ===
    GetClientByClientID(ctx context.Context, id string) (Client, error)
    AuthorizeClientIDSecret(ctx context.Context, clientID string, clientSecret string) error

    // === UserInfo Population ===
    SetUserinfoFromScopes(ctx context.Context, userinfo *oidc.UserInfo, userID string, clientID string, scopes []string) error
    SetUserinfoFromToken(ctx context.Context, userinfo *oidc.UserInfo, tokenID string, subject string, origin string) error

    // === Introspection ===
    SetIntrospectionFromToken(ctx context.Context, introspection *oidc.IntrospectionResponse, tokenID string, subject string, clientID string) error

    // === JWT Profile Grant ===
    GetPrivateClaimsFromScopes(ctx context.Context, userID string, clientID string, scopes []string) (map[string]any, error)
    GetKeyByIDAndClientID(ctx context.Context, keyID string, clientID string) (*jose.JSONWebKey, error)
    ValidateJWTProfileScopes(ctx context.Context, userID string, scopes []string) ([]string, error)

    // === Health ===
    Health(ctx context.Context) error
}
```

### 3.1 Extended Storage Interfaces (Optional)

The library uses Go interface assertion (`.(type)`) at runtime to detect additional capabilities. These are **opt-in** by implementing the interface on your Storage struct:

| Interface | When Asserted | Purpose |
|-----------|--------------|---------|
| `ClientCredentialsStorage` | Token endpoint, `grant_type=client_credentials` | `ClientCredentials(ctx, clientID, secret) (Client, error)` + `ClientCredentialsTokenRequest(ctx, clientID, scopes) (TokenRequest, error)` |
| `TokenExchangeStorage` | Token endpoint, `grant_type=urn:ietf:params:oauth:grant-type:token-exchange` | `ValidateTokenExchangeRequest`, `CreateTokenExchangeRequest`, `GetPrivateClaimsFromTokenExchangeRequest`, `SetUserinfoFromTokenExchangeRequest` |
| `DeviceAuthorizationStorage` | Device authorization endpoint | `StoreDeviceAuthorization(ctx, clientID, deviceCode, userCode, expires, scopes) error`, `GetDeviceAuthorizatonState(ctx, clientID, deviceCode) (*DeviceAuthorizationState, error)` |
| `CanSetUserinfoFromRequest` | ID token creation | `SetUserinfoFromRequest(ctx, userinfo, IDTokenRequest, scopes) error` -- **preferred over `SetUserinfoFromScopes`**; gives access to the full token request including `GetClientID()`. Will become required in v4. |
| `CanTerminateSessionFromRequest` | End session endpoint | `TerminateSessionFromRequest(ctx, *EndSessionRequest) (redirectURI string, err error)` -- replaces the simpler `TerminateSession` if present. |

**Critical pattern**: If your storage does NOT implement an optional interface, the library either returns `ErrUnimplemented` (for device auth, client credentials, etc.) or falls back to the base method.

---

## 4. The `op.AuthRequest` Interface -- What the Library Expects from Your Auth State

When the library calls `Storage.CreateAuthRequest()`, it passes in a raw `*oidc.AuthRequest` (the parsed HTTP form/query). Your storage must:
1. Persist the relevant fields.
2. Return an object that satisfies the `op.AuthRequest` interface:

```go
type AuthRequest interface {
    GetID() string
    GetACR() string
    GetAMR() []string
    GetAudience() []string
    GetAuthTime() time.Time
    GetClientID() string
    GetCodeChallenge() *oidc.CodeChallenge
    GetNonce() string
    GetRedirectURI() string
    GetResponseType() oidc.ResponseType
    GetResponseMode() oidc.ResponseMode
    GetScopes() []string
    GetState() string
    GetSubject() string
    Done() bool
}
```

### 4.1 The `Done()` Method is the Authentication Gate

This is the **critical flow control mechanism**. The library's authorization callback handler does:

```go
authReq, err := storage.AuthRequestByID(ctx, id)
// ...
if !authReq.Done() {
    // redirect to client.LoginURL(id)
}
// if Done(), proceed to issue code/tokens
```

Your login UI must:
1. Receive the auth request ID (as a query parameter).
2. Authenticate the user (however you want -- federated, password, etc.).
3. Update the auth request in storage to mark it as "done" AND set the `UserID`/`Subject`.
4. Redirect back to the callback URL: `op.AuthCallbackURL(provider)` with `?id=<authRequestID>`.

The library then re-fetches the auth request by ID, checks `Done() == true`, and proceeds to generate the authorization code or implicit response.

---

## 5. The `op.Client` Interface -- Client Registration Contract

Every time the library needs client information, it calls `Storage.GetClientByClientID()`. The returned object must satisfy:

```go
type Client interface {
    GetID() string
    RedirectURIs() []string
    PostLogoutRedirectURIs() []string
    ApplicationType() ApplicationType
    AuthMethod() oidc.AuthMethod
    ResponseTypes() []oidc.ResponseType
    GrantTypes() []oidc.GrantType
    LoginURL(string) string                           // receives auth request ID, returns login page URL
    AccessTokenType() AccessTokenType                  // Bearer (opaque) or JWT
    IDTokenLifetime() time.Duration
    DevMode() bool
    RestrictAdditionalIdTokenScopes() func([]string) []string
    RestrictAdditionalAccessTokenScopes() func([]string) []string
    IsScopeAllowed(scope string) bool
    IDTokenUserinfoClaimsAssertion() bool
    ClockSkew() time.Duration
}
```

### 5.1 `LoginURL(id string) string`

This is how the library knows where to send the user for authentication. On an authorization request, if the user hasn't authenticated yet (`Done() == false`), the library redirects to `client.LoginURL(authRequestID)`. Your login handler receives this ID and uses it throughout the authentication flow.

### 5.2 `ApplicationType`

Three values: `ApplicationTypeWeb`, `ApplicationTypeUserAgent`, `ApplicationTypeNative`. Used for redirect URI validation:
- `Web`: requires HTTPS redirects (or http://localhost).
- `Native`: allows http://localhost (any port) and custom schemes.
- `UserAgent`: requires HTTPS.
- If `DevMode()` returns true, all checks are relaxed.

### 5.3 `AccessTokenType`

Two values: `AccessTokenTypeBearer` (opaque) and `AccessTokenTypeJWT`.

- **Bearer/Opaque**: The library calls `Storage.CreateAccessToken()` and encrypts `tokenID:subject` via AES to produce the opaque token string.
- **JWT**: The library builds a full JWT access token with claims (`iss`, `sub`, `aud`, `exp`, `iat`, `nbf`, `jti`, scopes, plus custom claims from `GetPrivateClaimsFromScopes()`).

### 5.4 Optional `HasRedirectGlobs`

```go
type HasRedirectGlobs interface {
    RedirectURIGlobs() []string
    PostLogoutRedirectURIGlobs() []string
}
```

If a Client also implements this, `path.Match()` glob patterns are used for redirect URI validation in addition to exact matches.

---

## 6. Token Creation Internals

### 6.1 `CreateAccessToken` (the library function, not the storage method)

```go
func CreateAccessToken(ctx, request TokenRequest, tokenType AccessTokenType, creator TokenCreator, client Client, refreshToken string) (token, refreshToken string, validity time.Duration, error)
```

Flow:
1. If `tokenType == AccessTokenTypeJWT`:
   - Calls `Storage.GetPrivateClaimsFromScopes()` for custom claims.
   - Calls `Storage.CreateAccessToken()` to get a `tokenID` and `expiration`.
   - Builds JWT claims: `iss`, `sub`, `aud`, `exp`, `iat`, `nbf`, `jti=tokenID`, `scope`, `client_id`, plus custom claims.
   - Signs with `SignerFromKey(storage.SigningKey())`.
   - The returned token string IS the full JWT.

2. If `tokenType == AccessTokenTypeBearer`:
   - Calls `Storage.CreateAccessToken()` to get `tokenID` and `expiration`.
   - Encrypts `tokenID:subject` via `Crypto.Encrypt()` (AES with the 32-byte key from config).
   - The returned token string is the encrypted opaque blob.

### 6.2 `CreateIDToken`

```go
func CreateIDToken(ctx, issuer, request IDTokenRequest, client Client, lifetime, accessToken, code, storage Storage, client Client) (string, error)
```

- Builds standard OIDC ID token claims: `iss`, `sub`, `aud`, `exp`, `iat`, `auth_time`, `nonce`, `azp`, `acr`, `amr`.
- Adds `at_hash` (access token hash) if `accessToken != ""`.
- Adds `c_hash` (code hash) if `code != ""`.
- Populates user info claims into the ID token:
  - If `client.IDTokenUserinfoClaimsAssertion()` is true OR no access token was issued (implicit flow with `id_token` only), it calls `SetUserinfoFromRequest()` (or falls back to `SetUserinfoFromScopes()`).
  - Extra scopes are filtered through `client.RestrictAdditionalIdTokenScopes()`.
- Signs the ID token JWT with the storage's signing key.

### 6.3 `TokenRequest` Interface (generic token request data)

```go
type TokenRequest interface {
    GetSubject() string
    GetAudience() []string
    GetScopes() []string
}
```

Extended by `IDTokenRequest`:

```go
type IDTokenRequest interface {
    TokenRequest
    GetAMR() []string
    GetAuthTime() time.Time
    GetClientID() string
}
```

Your `AuthRequest` struct must satisfy `IDTokenRequest` (the example's `AuthRequest` does).

### 6.4 `RefreshTokenRequest` Interface

```go
type RefreshTokenRequest interface {
    TokenRequest
    IDTokenRequest  // inherits GetAMR, GetAuthTime, GetClientID
    SetCurrentScopes(scopes []string)
}
```

Called by `Storage.TokenRequestByRefreshToken()`. The library uses `SetCurrentScopes()` to narrow scopes on refresh if the client requested a subset.

---

## 7. Authorization Code Flow -- Complete Internal Sequence

### Step 1: Authorization Request Arrives (`GET /authorize`)

1. Library parses query params into `*oidc.AuthRequest`.
2. Validates: `redirect_uri` against `client.RedirectURIs()`, `response_type` against `client.ResponseTypes()`, scopes (including OIDC standard + `client.IsScopeAllowed()`), PKCE `code_challenge_method`, prompt, max_age.
3. If `request` parameter is present and `config.RequestObjectSupported`, parses and verifies the Request Object JWT.
4. Calls `Storage.CreateAuthRequest(ctx, authReq, userID)` -- `userID` is empty at this point.
5. Gets back an `op.AuthRequest` with an ID.
6. Checks `authReq.Done()`:
   - If false: redirects to `client.LoginURL(authReq.GetID())`.
   - If true (e.g., SSO session): proceeds directly to code/token generation.

### Step 2: User Authenticates (External to Library)

The library provides NO login UI. This is entirely your domain. You must:
1. Accept the user at your login URL.
2. Authenticate them.
3. Update the auth request in your storage: set `UserID`/`Subject`, mark `Done() = true`, record `AuthTime`.
4. Redirect to `op.AuthCallbackURL(provider)` which is `{issuer}/authorize/callback?id={authRequestID}`.

### Step 3: Authorization Callback (`GET /authorize/callback`)

1. Library reads `id` from query.
2. Calls `Storage.AuthRequestByID(ctx, id)`.
3. Asserts `authReq.Done() == true`.
4. Based on `response_type`:

   **Code Flow** (`response_type=code`):
   - Generates a random authorization code (crypto random string).
   - Calls `Storage.SaveAuthCode(ctx, authRequestID, code)`.
   - Redirects to `redirect_uri?code=CODE&state=STATE`.

   **Implicit Flow** (`response_type=id_token` or `response_type=id_token token`):
   - Creates tokens directly (access token + ID token).
   - Calls `Storage.DeleteAuthRequest()` to clean up.
   - Redirects with tokens in the URL fragment.

### Step 4: Token Exchange (`POST /oauth/token`, `grant_type=authorization_code`)

1. Library parses the token request.
2. Authenticates the client (via Basic Auth, POST body, or `private_key_jwt`).
3. Calls `Storage.AuthRequestByCode(ctx, code)`.
4. Validates: `redirect_uri` matches original, `client_id` matches, PKCE `code_verifier`.
5. Creates tokens:
   - If `offline_access` scope: calls `Storage.CreateAccessAndRefreshTokens()`.
   - Otherwise: calls `Storage.CreateAccessToken()`.
6. Creates ID token via `CreateIDToken()`.
7. Calls `Storage.DeleteAuthRequest()`.
8. Returns JSON `{ access_token, token_type, expires_in, id_token, refresh_token? }`.

---

## 8. Other Grant Type Flows

### 8.1 Refresh Token (`grant_type=refresh_token`)

1. Client authenticated.
2. Calls `Storage.TokenRequestByRefreshToken(ctx, refreshToken)` -> `RefreshTokenRequest`.
3. Validates scopes (can only narrow, not expand).
4. Calls `SetCurrentScopes()` on the request.
5. Calls `Storage.CreateAccessAndRefreshTokens(ctx, request, currentRefreshToken)`.
6. Creates new ID token.
7. Returns new token set.

### 8.2 Client Credentials (`grant_type=client_credentials`)

1. Asserts `Storage` implements `ClientCredentialsStorage` (runtime type assertion).
2. Calls `Storage.ClientCredentials(ctx, clientID, clientSecret)` -> Client.
3. Calls `Storage.ClientCredentialsTokenRequest(ctx, clientID, scopes)` -> TokenRequest.
4. Creates access token (no ID token, no refresh token).

### 8.3 JWT Profile Grant (`grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer`)

1. Parses the `assertion` JWT.
2. Calls `Storage.GetKeyByIDAndClientID(ctx, keyID, clientID)` to get the public key.
3. Verifies the JWT signature and claims.
4. Calls `Storage.ValidateJWTProfileScopes(ctx, subject, scopes)` to filter allowed scopes.
5. Creates access token.
6. Optionally creates ID token (if `openid` scope).

### 8.4 Token Exchange (`grant_type=urn:ietf:params:oauth:grant-type:token-exchange`)

1. Asserts `Storage` implements `TokenExchangeStorage`.
2. Validates subject token (verifies as access token or ID token).
3. Optionally validates actor token.
4. Calls `Storage.ValidateTokenExchangeRequest()` for business rules.
5. Calls `Storage.CreateTokenExchangeRequest()` for audit.
6. Creates new tokens using exchange-specific storage methods.

### 8.5 Device Authorization (`grant_type=urn:ietf:params:oauth:grant-type:device_code`)

1. Asserts `Storage` implements `DeviceAuthorizationStorage`.
2. Device authorization request: generates `device_code` + `user_code`, calls `Storage.StoreDeviceAuthorization()`.
3. Device token polling: calls `Storage.GetDeviceAuthorizatonState()`, returns `authorization_pending`, `slow_down`, `expired_token`, `access_denied`, or issues tokens when `state.Done == true`.
4. The user-facing verification UI is entirely your responsibility.

---

## 9. `op.Config` -- Provider Configuration

```go
type Config struct {
    CryptoKey                           [32]byte  // REQUIRED: AES key for opaque token encryption
    DefaultLogoutRedirectURI            string    // fallback after logout
    CodeMethodS256                      bool      // enable PKCE S256
    AuthMethodPost                      bool      // allow client_secret in POST body
    AuthMethodPrivateKeyJWT             bool      // allow private_key_jwt auth
    GrantTypeRefreshToken               bool      // enable refresh tokens
    RequestObjectSupported              bool      // enable JAR (request objects)
    SupportedUILocales                  []language.Tag
    DeviceAuthorization                 DeviceAuthorizationConfig
    SupportedClaims                     []string  // override default claim list
    SupportedScopes                     []string  // override default scope list

    // These enable the corresponding grant types in discovery:
    // GrantTypeClientCredentials, GrantTypeTokenExchange,
    // GrantTypeJWTAuthorization, GrantTypeDeviceCode
    // are derived from optional Storage interface assertions.
}
```

The `CryptoKey` is non-negotiable. It's used to build the `AES` `Crypto` instance that encrypts opaque access tokens. The key must be exactly 32 bytes.

---

## 10. `op.Option` Functions

| Option | Effect |
|--------|--------|
| `WithAllowInsecure()` | Allows `http://` issuer (disables HTTPS requirement). |
| `WithCustomAuthEndpoint(e)` | Overrides `/authorize` path. |
| `WithCustomTokenEndpoint(e)` | Overrides `/oauth/token` path. |
| `WithCustomIntrospectionEndpoint(e)` | Overrides `/oauth/introspect` path. |
| `WithCustomUserinfoEndpoint(e)` | Overrides `/userinfo` path. |
| `WithCustomRevocationEndpoint(e)` | Overrides `/oauth/revoke` path. |
| `WithCustomEndSessionEndpoint(e)` | Overrides `/end_session` path. |
| `WithCustomKeysEndpoint(e)` | Overrides `/keys` path. |
| `WithCustomEndpoints(endpoints)` | Overrides all endpoint paths at once. |
| `WithCrypto(c)` | Replaces the default AES `Crypto` with a custom implementation. |
| `WithLogger(l)` | Sets `*slog.Logger` for the provider. |
| `WithHttpInterceptors(fns...)` | Adds HTTP middleware to the provider's internal router. |
| `WithIssuerFromRequest(fn)` | Dynamic issuer resolution from `*http.Request` (for multi-tenant). |

---

## 11. Issuer Resolution

Two strategies:

### Static Issuer (default)
`IssuerFromRequest` returns the same issuer string for every request. Set at construction time.

### Dynamic Issuer (`WithIssuerFromRequest`)
Issuer is derived per-request from the `*http.Request` (e.g., from the `Host` header). This enables multi-tenant setups where a single provider serves multiple issuers.

The issuer is stored in `context.Context` via `ContextWithIssuer()` and retrieved with `IssuerFromContext(ctx)`. The `IssuerInterceptor` middleware does this automatically. Every endpoint handler reads the issuer from context, not from a static field.

---

## 12. `op.Server` Interface (New API)

The full interface for the new `RegisterServer` API:

```go
type Server interface {
    Health(context.Context, *Request[struct{}]) (*Response, error)
    Ready(context.Context, *Request[struct{}]) (*Response, error)
    Discovery(context.Context, *Request[struct{}]) (*Response, error)
    Keys(context.Context, *Request[struct{}]) (*Response, error)
    VerifyAuthRequest(context.Context, *Request[oidc.AuthRequest]) (*ClientRequest[oidc.AuthRequest], error)
    Authorize(context.Context, *ClientRequest[oidc.AuthRequest]) (*Redirect, error)
    DeviceAuthorization(context.Context, *ClientRequest[oidc.DeviceAuthorizationRequest]) (*Response, error)
    VerifyClient(context.Context, *Request[ClientCredentials]) (Client, error)
    CodeExchange(context.Context, *ClientRequest[oidc.AccessTokenRequest]) (*Response, error)
    RefreshToken(context.Context, *ClientRequest[oidc.RefreshTokenRequest]) (*Response, error)
    JWTProfile(context.Context, *Request[oidc.JWTProfileGrantRequest]) (*Response, error)
    TokenExchange(context.Context, *ClientRequest[oidc.TokenExchangeRequest]) (*Response, error)
    ClientCredentialsExchange(context.Context, *ClientRequest[oidc.ClientCredentialsRequest]) (*Response, error)
    DeviceToken(context.Context, *ClientRequest[oidc.DeviceAccessTokenRequest]) (*Response, error)
    Introspect(context.Context, *Request[IntrospectionRequest]) (*Response, error)
    UserInfo(context.Context, *Request[oidc.UserInfoRequest]) (*Response, error)
    Revocation(context.Context, *ClientRequest[oidc.RevocationRequest]) (*Response, error)
    EndSession(context.Context, *Request[oidc.EndSessionRequest]) (*Redirect, error)
}
```

### `Request[T]` and `ClientRequest[T]` generic wrappers

```go
type Request[T any] struct {
    Method string
    URL    *url.URL
    Header http.Header
    Form   url.Values
    Data   *T
}

type ClientRequest[T any] struct {
    *Request[T]
    Client Client
}
```

The `webServer` (from `RegisterServer`) handles all HTTP parsing, form decoding, client authentication, and grant type routing. Your `Server` implementation only receives typed, pre-parsed requests.

---

## 13. Signing and Key Management

### 13.1 `op.SigningKey` Interface

```go
type SigningKey interface {
    SignatureAlgorithm() jose.SignatureAlgorithm
    Key() any        // *rsa.PrivateKey, *ecdsa.PrivateKey, ed25519.PrivateKey, etc.
    ID() string      // Key ID (kid) -- included in JWT headers
}
```

Called via `Storage.SigningKey(ctx)` during token creation. The library constructs a `jose.Signer` from this key.

### 13.2 `op.Key` Interface (Public Keys for JWKS)

```go
type Key interface {
    ID() string
    Algorithm() jose.SignatureAlgorithm
    Use() string   // "sig"
    Key() any      // the PUBLIC key
}
```

Called via `Storage.KeySet(ctx)` for the `GET /keys` (JWKS) endpoint. This endpoint returns the JWK Set that relying parties use to verify tokens.

### 13.3 Signing Flow
1. `Storage.SigningKey(ctx)` -> `op.SigningKey`
2. `op.SignerFromKey(key)` -> `jose.Signer` (with `typ: "JWT"` header, key ID embedded)
3. `crypto.Sign(claims, signer)` -> compact JWT string

### 13.4 Key Rotation
The library does NOT implement key rotation. It calls `SigningKey()` and `KeySet()` on every relevant request. Your storage is responsible for returning the current signing key and maintaining a JWKS that includes both current and recently-rotated public keys (so tokens signed with the old key can still be verified).

---

## 14. Opaque Token Format

When `AccessTokenType == Bearer` (opaque):
- Token value = `AES-256-GCM-Encrypt(tokenID + ":" + subject)` using the 32-byte `CryptoKey`.
- On the userinfo/introspection endpoints, the library first tries `Crypto.Decrypt(token)` to extract `tokenID:subject`. If decryption fails, it falls back to verifying the token as a JWT.
- The `tokenID` is then passed to `Storage.SetUserinfoFromToken()` or `Storage.SetIntrospectionFromToken()`.

---

## 15. Discovery Endpoint

`GET /.well-known/openid-configuration` is auto-generated from:
- `Configuration` interface methods (endpoints, supported features).
- `Storage.SignatureAlgorithms(ctx)` for `id_token_signing_alg_values_supported`.
- Issuer from context.

Default supported scopes: `openid`, `profile`, `email`, `phone`, `address`, `offline_access`.

Default response types: `code`, `id_token`, `id_token token`.

Grant types are dynamically built based on config flags and optional storage interface assertions.

---

## 16. CORS Handling

The Legacy `Provider` constructor sets up CORS middleware via `github.com/rs/cors`:
```go
defaultCORSOptions = cors.Options{
    AllowCredentials: true,
    AllowedHeaders:   {"Authorization", "Content-Type"},
    AllowedMethods:   {"GET", "HEAD", "POST"},
    AllowOriginFunc:  func(string) bool { return true },
}
```

This is permissive by default. The `RegisterServer` path also creates a CORS handler that can be customized via `WithServerCORSOptions`.

---

## 17. Error Handling

### 17.1 `oidc.Error` Type

```go
type Error struct {
    Parent           error
    ErrorType        errorType   // "invalid_request", "invalid_client", "server_error", etc.
    Description      string
    State            string
    SessionState     string
    redirectDisabled bool
}
```

Constructor functions: `oidc.ErrInvalidRequest()`, `oidc.ErrInvalidClient()`, `oidc.ErrAccessDenied()`, `oidc.ErrServerError()`, `oidc.ErrLoginRequired()`, `oidc.ErrAuthorizationPending()`, etc.

### 17.2 `op.StatusError`

Wraps an error with an HTTP status code. Used by the `Server` API to signal specific HTTP response codes.

### 17.3 Error Routing

On authorization endpoints, errors are redirected to the client's `redirect_uri` with error parameters. On token/introspection/revocation endpoints, errors are returned as JSON with appropriate HTTP status codes.

---

## 18. The Auth Callback URL Mechanism

```go
func AuthCallbackURL(provider OpenIDProvider) func(ctx context.Context, id string) string
```

Returns a function that generates `{issuer}/authorize/callback?id={authRequestID}`.

This URL is what your login UI redirects to after successful authentication. The library registers a handler at `/authorize/callback` that:
1. Reads the `id` parameter.
2. Fetches the auth request from storage.
3. Checks `Done() == true`.
4. Produces the authorization response (code or implicit tokens).

---

## 19. The `IssuerInterceptor` Pattern

When you have login UI routes that are NOT part of the OP handler, but need the issuer in context (for constructing callback URLs), you use:

```go
interceptor := op.NewIssuerInterceptor(provider.IssuerFromRequest)
router.Post("/login", interceptor.HandlerFunc(loginHandler))
```

This middleware injects the issuer into `r.Context()` before your handler runs.

---

## 20. Session End / Logout

### Endpoint: `GET /end_session`

1. Parses `id_token_hint`, `post_logout_redirect_uri`, `client_id`, `state`.
2. If `id_token_hint` is present, verifies it (allows expired tokens via `IDTokenHintExpiredError`).
3. Validates `post_logout_redirect_uri` against `client.PostLogoutRedirectURIs()`.
4. Calls `Storage.TerminateSession(ctx, userID, clientID)` (or `TerminateSessionFromRequest` if implemented).
5. Redirects to `post_logout_redirect_uri` (or `DefaultLogoutRedirectURI`).

---

## 21. OpenTelemetry Tracing

The library includes a package-level `Tracer` variable (from `go.opentelemetry.io/otel`). Every handler and key internal function creates spans:
```go
ctx, span := Tracer.Start(r.Context(), "OperationName")
defer span.End()
```

These are no-ops unless you register a trace provider.

---

## 22. What the Library Does NOT Do

1. **No login/authentication UI**: The library only provides the OIDC protocol. Login pages, federated auth (Google, GitHub), MFA -- all your responsibility.
2. **No user management**: No user creation, password hashing, account linking.
3. **No database layer**: Everything goes through `Storage` -- you pick PostgreSQL, Redis, DynamoDB, etc.
4. **No key rotation automation**: You must implement key rotation in your `SigningKey()` / `KeySet()` methods.
5. **No session management beyond the protocol**: No cookie-based SSO sessions. If you want "remember me" or SSO across apps, you implement that on top.
6. **No consent screens**: The library doesn't know about user consent. If you need consent flows, you implement them in your login UI before marking `Done() = true`.
7. **No rate limiting or abuse prevention**: Token endpoint polling (device flow) has a basic timeout, but nothing else.

---

## 23. Summary of What We Must Implement

To use this library as our OIDC Provider, we need to provide:

1. **A struct implementing `op.Storage`** (and optionally `ClientCredentialsStorage`, `DeviceAuthorizationStorage`, `TokenExchangeStorage` as needed) backed by our database.

2. **A struct implementing `op.Client`** (returned by `GetClientByClientID`) representing our registered OAuth clients.

3. **A struct implementing `op.AuthRequest`** with all the getter methods plus `Done() bool`, representing the in-flight authorization state.

4. **A struct implementing `op.RefreshTokenRequest`** with `SetCurrentScopes()`, for refresh token grant handling.

5. **Signing key management**: `SigningKey()` returns private key, `KeySet()` returns public keys, `SignatureAlgorithms()` returns algorithm list.

6. **A login/authentication UI** that:
   - Receives the auth request ID from the redirect.
   - Authenticates the user (via upstream IdPs like Google/GitHub, or local credentials).
   - Updates the auth request in storage (sets subject, marks done, records auth time).
   - Redirects to the auth callback URL.

7. **An HTTP server** that:
   - Creates the `op.OpenIDProvider` (or `op.Server` + `op.RegisterServer`).
   - Mounts the provider handler.
   - Mounts the login UI routes.
   - Configures the `IssuerInterceptor` for login routes that need the issuer in context.

---

## 24. Key Design Decisions to Make Before Implementation

| Decision | Options | Notes |
|----------|---------|-------|
| API style | Legacy (`NewOpenIDProvider`) vs New (`RegisterServer`) | Legacy is simpler; New gives full control. Example uses Legacy. |
| Access token format | Opaque (Bearer) vs JWT | Per-client via `Client.AccessTokenType()`. JWT is self-verifiable; opaque requires introspection. |
| Signing algorithm | RS256, RS384, RS512, ES256, ES384, ES512, EdDSA | Must match your key type. RS256 is the most compatible. |
| Issuer strategy | Static vs dynamic (from request) | Dynamic enables multi-tenant. |
| Key storage | In-memory, database, HSM, KMS | Affects `SigningKey()` and `KeySet()` implementations. |
| Session management | None (stateless) vs cookie-based SSO | Library doesn't provide this; must be built in login UI layer. |
| Auth request storage | Database, Redis, in-memory | Must survive server restarts if using auth code flow. Redis with TTL is common. |
| Token storage | Database (opaque) vs stateless (JWT) | Opaque tokens need storage for introspection/revocation. JWT tokens are self-contained but harder to revoke. |
