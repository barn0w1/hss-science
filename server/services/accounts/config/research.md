# Config Package Research & Analysis

## 1. Responsibilities & Core Logic

The `config` package is a **configuration loader and validator** responsible for bootstrapping the accounts microservice. Its primary responsibilities are:

- **Environment Variable Parsing**: Extract and parse configuration values from the process environment
- **Cryptographic Key Management**: Load and parse RSA private keys (for JWT signing) and AES-256 encryption keys (for OIDC state/code encryption)
- **Multi-Provider Authentication Setup**: Validate the presence of at least one upstream identity provider (Google OAuth2 or GitHub OAuth2)
- **Token Lifetime Configuration**: Provide sensible defaults and enforce boundaries for token expiration policies
- **Validation & Fail-Fast Design**: Perform comprehensive validation during application startup, ensuring the service cannot start with misconfigured settings

The package operates at the **application bootstrap boundary**—it sits between the OS/environment and the accounts service business logic. It enforces the principle that invalid configuration should prevent service startup rather than causing runtime failures.

## 2. Domain Models

### `Config` Struct (Primary Entity)

```go
type Config struct {
    Port        string
    Issuer      string
    DatabaseURL string
    CryptoKey   [32]byte
    SigningKey  *rsa.PrivateKey

    AccessTokenLifetimeMinutes int
    RefreshTokenLifetimeDays   int
    AuthRequestTTLMinutes      int

    GoogleClientID     string
    GoogleClientSecret string
    GitHubClientID     string
    GitHubClientSecret string
}
```

**Field Semantics**:

| Field | Type | Purpose | Validation |
|-------|------|---------|-----------|
| `Port` | `string` | HTTP server listen port | Optional, defaults to "8080" |
| `Issuer` | `string` | OIDC issuer URL (must match external URL clients use) | Required, no validation on format |
| `DatabaseURL` | `string` | PostgreSQL connection string | Required, no connection validation (deferred to runtime) |
| `CryptoKey` | `[32]byte` | AES-256 key for symmetric encryption (OIDC state/codes) | Required, must be exactly 32 bytes, hex-encoded input |
| `SigningKey` | `*rsa.PrivateKey` | RSA private key for JWT signing | Required, supports PKCS#1 and PKCS#8 formats |
| `AccessTokenLifetimeMinutes` | `int` | JWT access token validity period | Optional default: 15, valid range: 1-60 (or 0 to use default) |
| `RefreshTokenLifetimeDays` | `int` | Refresh token validity period | Optional default: 7, valid range: 1-90 (or 0 to use default) |
| `AuthRequestTTLMinutes` | `int` | Auth request (code) validity period | Optional default: 30, valid range: 1-60 (or 0 to use default) |
| `GoogleClientID` / `GoogleClientSecret` | `string` | Google OAuth2 upstream provider credentials | Optional together (but at least one IdP required) |
| `GitHubClientID` / `GitHubClientSecret` | `string` | GitHub OAuth2 upstream provider credentials | Optional together (but at least one IdP required) |

**Data Type Characteristics**:
- Fixed-size byte array `[32]byte` for `CryptoKey` ensures compile-time bounds for the encryption key
- RSA key is a pointer to allow nil-checking, though it's always non-nil when successfully loaded
- Integer token lifetimes use range [1, N] with special semantic: 0 → use default

## 3. Ports & Interfaces

This package **does not expose any interfaces or ports**. It is a pure **adapter/utility package** with no abstraction boundaries.

**External Contract** (what it provides):
- Single public function: `Load() (*Config, error)` — synchronous, side-effect-free configuration loader
- Single helper function: `parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error)` — internal parsing logic (unexported but used in tests)

**No Dependency Injection**:
- The package directly reads `os.Getenv()` instead of accepting an environment map or provider interface
- This creates tight coupling to the process environment
- Makes unit testing require `t.Setenv()` to mutate global environment state during tests

**No Configuration Sources**:
- Configuration originates only from OS environment variables
- No support for:
  - Configuration files (.env, YAML, TOML)
  - Command-line flags
  - Configuration services or secret managers
- This limits flexibility for different deployment scenarios (Kubernetes, Docker Compose, etc.)

## 4. Dependencies

### External Dependencies

**Standard Library Only**:
- `crypto/rsa` — RSA key pair operations
- `crypto/x509` — X.509 certificate/key parsing (PKCS#1, PKCS#8)
- `encoding/hex` — Hexadecimal encoding/decoding for `CRYPTO_KEY`
- `encoding/pem` — PEM block parsing for `SIGNING_KEY_PEM`
- `fmt` — Error message formatting
- `os` — Environment variable access
- `strconv` — String to integer conversion

**No External Libraries**:
- Zero third-party Go dependencies in this package

### Internal Dependencies

**Within the Accounts Service**:
- None directly. The Config package has no import dependencies on other `accounts/internal/*` packages.
- However, it is imported by `main.go`, which then uses the `Config` to initialize:
  - `identity.Service` — user identity domain service
  - `oidcdom.*` — OIDC domain services (auth requests, clients, tokens)
  - `oidcadapter.StorageAdapter` — persistence/storage adapter for OIDC operations

### Implicit Runtime Dependencies

- **PostgreSQL Database**: `DatabaseURL` points to a PostgreSQL instance (but connection happens in `main.go`, not here)
- **Google & GitHub OIDC Servers**: The `GoogleClientID`/`GitHubClientID` are later used by `authn` package to make calls to external IdP endpoints

## 5. Specifications from Tests

The test suite (`config_test.go`) with 20+ test cases acts as a comprehensive specification. Key behaviors:

### 5.1 Required Fields (Fail-Fast Validation)

- **ISSUER**: Must not be empty → Error: `"ISSUER is required"`
- **DATABASE_URL**: Must not be empty → Error: `"DATABASE_URL is required"`
- **CRYPTO_KEY**: Must not be empty → Error: `"CRYPTO_KEY is required"`
- **SIGNING_KEY_PEM**: Must not be empty → Error: `"SIGNING_KEY_PEM is required"`
- **At Least One IdP**: Either `GOOGLE_CLIENT_ID` or `GITHUB_CLIENT_ID` must be set → Error: `"at least one upstream IdP must be configured"`

**Test Evidence**:
- `TestLoad_MissingIssuer`
- `TestLoad_MissingDatabaseURL`
- `TestLoad_MissingCryptoKey`
- `TestLoad_MissingSigningKey`
- `TestLoad_NoUpstreamIdP`

### 5.2 Crypto Key Validation

- **Format**: Must be hex-encoded (base 16)
  - Invalid hex input → Error: `"CRYPTO_KEY must be hex-encoded: ..."`
  - Test: `TestLoad_InvalidCryptoKeyHex`

- **Length**: After hex decoding, must be exactly 32 bytes
  - 16 bytes (32 hex chars) → Error: `"CRYPTO_KEY must be exactly 32 bytes (64 hex chars), got 16 bytes"`
  - Test: `TestLoad_CryptoKeyWrongLength`

**Business Rationale**: 32 bytes = 256 bits, required for AES-256 symmetric encryption.

### 5.3 RSA Signing Key Parsing

- **Supported PEM Block Types**:
  - `"RSA PRIVATE KEY"` (PKCS#1 format) → Parsed via `x509.ParsePKCS1PrivateKey()`
  - `"PRIVATE KEY"` (PKCS#8 format) → Parsed via `x509.ParsePKCS8PrivateKey()` with runtime type assertion to `*rsa.PrivateKey`
  - Test Evidence: `TestLoad_PKCS8Key`, `TestLoad_Success`

- **Invalid/Unsupported Formats**:
  - Missing PEM block (malformed) → Error: `"failed to decode PEM block"`
  - EC private key (wrong key type) → Error: `"unsupported PEM block type: EC PRIVATE KEY"`
  - Non-RSA key from PKCS#8 → Error: `"PKCS#8 key is not RSA"`
  - Test Evidence: `TestLoad_InvalidSigningKey`, `TestParseRSAPrivateKey_UnsupportedBlockType`

### 5.4 Token Lifetime Configuration

All three token lifetime values follow the same validation pattern:

**Pattern**:
1. Read from env, use default if not set or parsing fails
2. If value is 0, replace with hardcoded default
3. If value is outside range [1, MAX], return error
4. Valid range depends on token type:
   - `ACCESS_TOKEN_LIFETIME_MINUTES`: 1-60, default=15
   - `REFRESH_TOKEN_LIFETIME_DAYS`: 1-90, default=7
   - `AUTH_REQUEST_TTL_MINUTES`: 1-60, default=30

**Test Evidence**:
- `TestLoad_TokenLifetimeDefaults` — All three use defaults
- `TestLoad_TokenLifetimeCustom` — Explicit values accepted
- `TestLoad_TokenLifetimeZeroFallsBackToDefault` — 0 → default behavior
- `TestLoad_TokenLifetimeOutOfRange` — 120 min (>60) rejected
- `TestLoad_RefreshTokenLifetimeOutOfRange` — 365 days (>90) rejected
- `TestLoad_AuthRequestTTLOutOfRange` — 120 min (>60) rejected

**Business Rationale**:
- Access tokens: 15 min (short-lived)
- Refresh tokens: 7 days (long-lived)
- Auth requests: 30 min (session timeout for auth flow)
- Ranges enforce reasonable security bounds (no perpetual tokens)

### 5.5 Port Configuration

- **Behavior**: Optional, defaults to `"8080"`
- **Test Evidence**: `TestLoad_CustomPort` — Honors `PORT` env var, `TestLoad_Success` — Defaults to "8080"

### 5.6 Provider Configuration Flexibility

- **Allowed Combinations**:
  - Google only (with Google credentials)
  - GitHub only (with GitHub credentials)
  - Both (with both credentials)
  - **Not Allowed**: Neither

- **Test Evidence**: `TestLoad_GitHubOnly` — GitHub-only configuration accepted

### 5.7 Error Semantics

All validation errors return immediately (fail-fast). Error messages are descriptive and include context:
- `"ISSUER is required"`
- `"CRYPTO_KEY must be exactly 32 bytes (64 hex chars), got 16 bytes"`
- `"ACCESS_TOKEN_LIFETIME_MINUTES must be 0 (default) or 1-60, got 120"`

No partial cfg is returned on error; the entire `Load()` call fails.

## 6. Tech Debt & Refactoring Candidates

### 6.1 Monolithic Load Function

**Issue**: The `Load()` function is 100+ lines of sequential parsing, validation, and loading logic without clear separation of concerns.

**Symptom**:
```go
func Load() (*Config, error) {
    cfg := &Config{ ... }      // Initialize
    // Validate ISSUER
    if cfg.Issuer == "" { ... }
    // Validate DATABASE_URL
    if cfg.DatabaseURL == "" { ... }
    // Validate CRYPTO_KEY (decode hex, check length)
    // Validate SIGNING_KEY (parse PEM, type check)
    // Validate IdP configuration
    // Validate & set token lifetimes (x3 with repeated logic)
    return cfg, nil
}
```

**Recommendation**:
- Extract validator functions: `validateIssuer()`, `validateDatabaseURL()`, `validateCryptoKey()`, etc.
- Extract parser functions: `parseTokenLifetimes()`, `parseSigningKey()`
- Consider a builder pattern or validator chain to modularize validation
- Each validation should be independently testable

### 6.2 Repeated Token Lifetime Validation Logic

**Issue**: Same validation pattern (getEnvInt → zero check → range check → error) repeated 3 times.

**Code Duplication**:
```go
cfg.AccessTokenLifetimeMinutes = getEnvInt("ACCESS_TOKEN_LIFETIME_MINUTES", 15)
if cfg.AccessTokenLifetimeMinutes == 0 {
    cfg.AccessTokenLifetimeMinutes = 15
}
if cfg.AccessTokenLifetimeMinutes < 1 || cfg.AccessTokenLifetimeMinutes > 60 {
    return nil, fmt.Errorf("...")
}
// ... repeated for RefreshTokenLifetimeDays and AuthRequestTTLMinutes
```

**Recommendation**:
- Create a reusable function: `loadAndValidateIntParam(envKey string, defaultVal, minVal, maxVal int) (int, error)`
- Apply DRY principle to eliminate duplication

### 6.3 No Configuration Source Abstraction

**Issue**: Hard-coded dependency on `os.Getenv()` makes it impossible to use alternative configuration sources (files, secret managers, etc.) without modifying the package.

**Current Tight Coupling**:
```go
cfg := &Config{
    Port:               getEnv("PORT", "8080"),
    Issuer:             os.Getenv("ISSUER"),
    Database URL:       os.Getenv("DATABASE_URL"),
    // ...
}
```

**Recommendation**:
- Define an interface: `type ConfigSource interface { Get(key string) string }`
- Implement concrete sources: `OSEnvSource`, `FileSource`, `VaultSource`
- Refactor `Load()` to accept a `ConfigSource` parameter
- Enable testing with mock sources and support multiple deployment scenarios (K8s, Docker, local development)

### 6.4 Testing Requires Global State Mutation

**Issue**: Tests must use `t.Setenv()` to mutate the global process environment, which is awkward and couples tests to the implementation.

**Current Test Pattern**:
```go
func TestLoad_Success(t *testing.T) {
    pemKey := generateTestKey(t)
    setRequiredEnv(t, pemKey)  // Uses t.Setenv() internally
    cfg, err := Load()
    // assertions...
}
```

**Recommendation**: Once `ConfigSource` abstraction is in place (see 6.3), tests can pass a `MockConfigSource` directly:
```go
source := &MockConfigSource{data: map[string]string{ ... }}
cfg, err := LoadFromSource(source)
```

### 6.5 Weak Helper Functions

**Issue**: `getEnv()` and `getEnvInt()` are duplicated logic that should be consolidated.

**Current Code**:
```go
func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func getEnvInt(key string, defaultVal int) int {
    v := os.Getenv(key)
    if v == "" {
        return defaultVal
    }
    n, err := strconv.Atoi(v)
    if err != nil {
        return defaultVal
    }
    return n
}
```

**Problem**:
- `getEnv()` uses a fallback parameter pattern; `getEnvInt()` uses a default parameter pattern — inconsistency
- `getEnvInt()` silently fails on parse error (returns default instead of error)
- Both should be part of the `ConfigSource` abstraction

### 6.6 No Structured Validation Error Context

**Issue**: Validation errors are formatted strings without structured context. Difficult for automation to parse or for clients to provide localized error messages.

**Current**:
```go
return nil, fmt.Errorf("ACCESS_TOKEN_LIFETIME_MINUTES must be 0 (default) or 1-60, got %d", cfg.AccessTokenLifetimeMinutes)
```

**Recommendation**:
- Use structured error types: `type ValidationError struct { Field string; Value interface{}; Reason string }`
- Enables callers to programmatically handle different error types
- Better integration with logging frameworks that expect structured fields

### 6.7 No Validation of URL Formats

**Issue**: `ISSUER` and `DATABASE_URL` are accepted as-is without format validation.

**Current Behavior**:
- `ISSUER` is just checked for non-empty; no URL parsing, no HTTPS requirement
- `DATABASE_URL` is passed directly to `sqlx.Connect()` with no validation

**Risk**:
- Typos in URLs lead to runtime failures (deferred until server boot in `main.go`)
- ISSUER format errors (malformed URL) not caught at configuration load time

**Recommendation**:
- Parse `ISSUER` as a URL and validate scheme/host: `url.Parse(cfg.Issuer); if url.Scheme not in ["http", "https"]`
- Validate `DATABASE_URL` format: Ensure it's a valid PostgreSQL connection string
- Fail fast at config load time rather than at database connection time

### 6.8 RSA Key Size Not Enforced

**Issue**: Any RSA key size is accepted; no validation that the key has sufficient bits.

**Current Code**:
```go
rsaKey, ok := key.(*rsa.PrivateKey)
if !ok {
    return nil, fmt.Errorf("PKCS#8 key is not RSA")
}
return rsaKey, nil
```

**No Check**: `rsaKey.N.BitLen()` or minimum key size (e.g., 2048 bits)

**Recommendation**:
- Enforce minimum RSA key size (2048 bits minimum, 4096 recommended for production)
- Add validation: `if rsaKey.N.BitLen() < 2048 { return nil, fmt.Errorf("RSA key must be at least 2048 bits, got %d", rsaKey.N.BitLen()) }`

### 6.9 No Configuration Documentation/Schema

**Issue**: Configuration requirements are implicit in code and `.env.example`. No single source of truth for what fields are required, optional, or have constraints.

**Recommendation**:
- Create a structured schema (JSON Schema, or a Go struct with tags):
  ```go
  type ConfigSchema struct {
      Port struct {
          Required bool
          Default  string
          Env      string
      }
      Issuer struct {
          Required bool
          Env      string
          Format   string // "url"
      }
      // ...
  }
  ```
- Auto-generate documentation and validation from schema
- Enable IDE/tooling support for configuration files

### 6.10 Coupling Between Token Lifetime Bounds and Business Logic

**Issue**: Hard-coded min/max bounds (e.g., 1-60 for minutes) are mixed into the loader. If security policy changes (e.g., max access token = 30 min), code must be modified.

**Recommendation**:
- Extract bounds into configuration constants or a policy struct:
  ```go
  type TokenPolicy struct {
      AccessTokenMinMinutes, AccessTokenMaxMinutes int
      RefreshTokenMinDays, RefreshTokenMaxDays     int
      AuthRequestMinMinutes, AuthRequestMaxMinutes int
  }
  ```
- Accept policy as a parameter to `Load()` or use a global policy constant
- Enables dynamic policy updates without code changes

---

## Summary Table: Issues & Priorities

| Issue | Severity | Effort | Priority | Impact |
|-------|----------|--------|----------|--------|
| Monolithic Load function | Medium | High | High | Maintainability |
| Repeated DRY violations (token lifetime) | Low | Low | High | Code Quality |
| No ConfigSource abstraction | High | High | High | Flexibility, Testability, Deployment |
| Testing couples to global env | Medium | High | Medium | Testing DX |
| Weak helper functions | Low | Low | Medium | Code Clarity |
| No structured error types | Low | Medium | Low | Error Handling |
| No URL format validation | Medium | Low | Medium | Data Integrity |
| No RSA key size enforcement | Medium | Low | Medium | Security |
| No configuration schema/docs | Low | Medium | Low | Developer Experience |
| Hard-coded token bounds | Low | Medium | Low | Policy Flexibility |

---

## Architectural Observations

### Clean Architecture Alignment
The package currently **violates Ports & Adapters principles**:
- ✗ No abstraction over environment source (tightly coupled to `os.Getenv()`)
- ✗ No interface contracts; entire logic is imperative
- ✓ Simple, single responsibility (load & validate configuration)
- ✓ No business domain leakage

### Recommendations for Architectural Review
1. **Introduce ConfigSource Interface**: Decouple from `os.Getenv()` to support multiple configuration sources
2. **Separate Concerns**: Split Load() into Parse → Validate → Construct phases
3. **Structured Validation**: Use error types that enable programmatic handling
4. **Runtime Configuration Validation**: Defer some checks (e.g., database connectivity) to health checks, not config load
5. **Configuration as Code**: Accept configuration imports/overrides for testing and multi-environment support
