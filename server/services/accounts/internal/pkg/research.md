# Research Report: `/internal/pkg` Package

## 1. Responsibilities & Core Logic

The `pkg` package is a collection of cross-cutting domain utilities for the `accounts` microservice. It has two primary responsibilities:

### Crypto Subpackage
Provides **authenticated encryption/decryption services** using AES-256-GCM (Galois/Counter Mode). The main responsibility is to securely encrypt sensitive data (e.g., user credentials, API tokens, session secrets) with a 256-bit symmetric key and decrypt them with integrity verification. The use of AES-GCM provides both **confidentiality** (encryption) and **authenticity** (authentication), ensuring ciphertexts have not been tampered with.

### Domerr Subpackage
Defines **domain-specific sentinel error types** for consistent, semantic error handling across the accounts service. It provides a small error catalog that maps to common domain concerns:
- Resource existence (not found, already exists)
- Access control (unauthorized)
- Server state (internal error)

These errors enable upstream layers (API handlers, business logic) to respond to specific failure modes appropriately (e.g., 404, 409, 401, 500).

## 2. Domain Models

### Crypto Package
**No explicit data structures** are defined. The package operates on primitive types:
- `key [32]byte` - A 256-bit AES key (fixed-size array)
- `plaintext []byte` - Raw unencrypted data
- `encoded string` - Base64-URL-encoded ciphertext (concatenation of nonce + authenticator + ciphertext)

The internal data format is opaque: `base64(nonce || GCM(plaintext))`, where the nonce is the GCM-required initialization vector and is prepended to the encrypted output for transmission/storage.

### Domerr Package
**Sentinel error variables** serve as domain models:
```
ErrNotFound      - Indicates a requested resource does not exist
ErrAlreadyExists - Indicates duplicate resource creation attempt
ErrUnauthorized  - Indicates access denied due to insufficient permissions
ErrInternal      - Indicates internal service failures
```

These are **immutable, zero-allocation sentinel values** (using `errors.New`), enabling identity-based comparisons via `errors.Is()`.

## 3. Ports & Interfaces

### Crypto Package
**No formal interface definition.** The package follows a **port pattern implicitly**:
- **Outbound Port (Dependency)**: Relies on Go's standard library `crypto/aes`, `crypto/cipher`, and `crypto/rand` for cryptographic primitives and secure randomness.
- **Service Functions as Ports**: `Encrypt()` and `Decrypt()` act as the external contract for encryption operations.
- **Data Exchange**: Accepts byte slices and keys, returns encoded strings and decrypted bytes.

### Domerr Package
**Minimal interface contract:**
- Exports 4 sentinel error constants for external use
- Provides `Is(err, target error) bool` as a thin wrapper over `errors.Is()` (the standard library error matching function)
- No type assertions or structural checks; purely sentinel-based matching

### Missing Abstractions
Neither subpackage defines Go interfaces, which limits:
- Testability (cannot mock the crypto functions in unit tests without integration)
- Polymorphism (cannot swap implementations at runtime)
- Loose coupling (functions are directly coupled to stdlib implementations)

## 4. Dependencies

### Crypto Package
**Internal Dependencies:**
- None (self-contained)

**External Dependencies:**
- `crypto/aes` - Go standard library; provides AES cipher primitives
- `crypto/cipher` - Go standard library; provides GCM mode and AEAD interface
- `crypto/rand` - Go standard library; cryptographically secure random number generation for nonces
- `encoding/base64` - Go standard library; for encoding ciphertext to transmissible string format
- `fmt` - Go standard library; error wrapping and formatting

**No database, no domain model, no intricate interdependencies.**

### Domerr Package
**Internal Dependencies:**
- None

**External Dependencies:**
- `errors` - Go standard library; error wrapping and sentinel matching
- `fmt` - Go standard library; error formatting (used in tests)

**Both packages are completely isolated and dependency-free beyond stdlib.**

## 5. Specifications from Tests

### Crypto Tests (`aes_test.go`)

#### TestEncryptDecrypt_RoundTrip
- **Requirement:** Encryption must be reversible; a plaintext encrypted and then decrypted must yield the original plaintext bitwise.
- **Test:** Encrypts `"hello world"`, verifies non-empty output, then decrypts and asserts equality.
- **Enforces:** Correct GCM implementation, proper nonce handling, and base64 round-tripping.

#### TestDecrypt_WrongKey
- **Requirement:** Decryption must fail if the decryption key differs from the encryption key. This is a **security critical** specification: GCM's authentication tag will not verify.
- **Test:** Encrypts with `key1`, attempts decryption with `key2`, expects error.
- **Enforces:** GCM's authenticated encryption property; tampering or key mismatch is detected.

#### TestDecrypt_InvalidBase64
- **Requirement:** Ciphertext input must be valid base64-URL-encoded data.
- **Test:** Attempts to decrypt malformed base64 string `"not-valid-base64!!!"`, expects error.
- **Enforces:** Input validation before cryptographic operations; prevents panic or undefined behavior.

#### TestDecrypt_TooShort
- **Requirement:** Ciphertext must be at least `nonceSize` bytes after base64 decoding. GCM requires the nonce to be present in the output.
- **Test:** Attempts to decrypt `"AQID"` (4 bytes), expects error because `nonceSize` is 12 for GCM.
- **Enforces:** Protection against malformed or truncated ciphertexts; prevents out-of-bounds access.

### Domerr Tests (`errors_test.go`)

#### TestSentinelErrors_AreDistinct
- **Requirement:** Each sentinel error must be distinct; `errors.Is(ErrA, ErrB)` must be false for `A != B`.
- **Test:** Exhaustive pairwise comparison of all four sentinels; ensures no cross-contamination.
- **Enforces:** Sentinel independence; prevents false matching and ensures error semantics are preserved through wrapping.

#### TestIs_WrappedError
- **Requirement:** When a sentinel error is wrapped (e.g., `fmt.Errorf("context: %w", ErrNotFound)`), the `Is()` function must still identify it.
- **Test:** Wraps `ErrNotFound` with context, asserts `Is(wrapped, ErrNotFound)` is true and `Is(wrapped, ErrAlreadyExists)` is false.
- **Enforces:** The `errors.Is()` semantic of error chain unwrapping; supports rich error context without losing error type information.

#### TestIs_NilError
- **Requirement:** Nil errors must not match any sentinel.
- **Test:** `Is(nil, ErrNotFound)` must be false.
- **Enforces:** Null safety and prevents erroneous error matching on uninitialized/success paths.

## 6. Tech Debt & Refactoring Candidates

### Crypto Package

#### 1. **Lack of Interface Abstraction** (Clean Architecture Violation)
- **Issue:** `Encrypt()` and `Decrypt()` are concrete functions with no interface contract. This makes it difficult for higher-level domain logic to mock crypto operations in unit tests.
- **Impact:** Forces integration testing or careful test structure to avoid real encryption/decryption.
- **Recommendation:** Define a `Cipher` interface:
  ```go
  type Cipher interface {
      Encrypt(plaintext []byte) (string, error)
      Decrypt(encoded string) ([]byte, error)
  }
  ```
  Implement it with a concrete `AESCipher` struct that holds the key. This enables:
  - Constructor-based dependency injection
  - Easy mocking in tests
  - Future swapping of cipher algorithms without changing call sites

#### 2. **Hard-coded Base64 URL Encoding** (Lack of Flexibility)
- **Issue:** `base64.URLEncoding` is hard-coded. If the service later needs different encoding (e.g., standard base64, hex), the function must be modified.
- **Impact:** Tight coupling to encoding choice; limits flexibility for API versioning or alternative serialization.
- **Recommendation:** Either:
  - Make encoding configurable as a field on a `Cipher` interface implementation
  - Document the choice and accept it as a stable contract
  - Consider if a different transport encoding (JSON, protobuf) might be better at the API boundary

#### 3. **No Key Rotation/Versioning Support** (Production Concern)
- **Issue:** The functions accept a single 256-bit key with no versioning or rotation mechanism. In production, key rotation is critical for security.
- **Impact:** Cannot decrypt old data encrypted with rotated keys; introduces operational constraints.
- **Recommendation:** Consider future support for key versioning (e.g., prepend a key ID to ciphertext, maintain a key store). This is a design-time decision but should be documented.

#### 4. **Random Nonce Generation is Stateless** (Potential Misuse Risk)
- **Issue:** `crypto/rand.Read()` is called per encryption, which is correct for GCM. However, there is no guard against nonce reuse with the same key (which would be catastrophic for security).
- **Impact:** If a caller reuses the key for millions of encryptions, the birthday paradox means eventual nonce collision becomes likely, compromising confidentiality.
- **Recommendation:** Document the assumption (nonce uniqueness per API call) and consider:
  - Adding a counter-based nonce generator for deterministic, collision-free nonces
  - Or enforce in higher-level code that keys are not reused above a threshold

#### 5. **Error Messages Lack Contextual Information** (Debugging)
- **Issue:** Errors are generic (e.g., "decrypt: %w") and don't include the size of the ciphertext, key ID, or other diagnostic info.
- **Impact:** Harder to debug failures in production; cannot distinguish between user error, key mismatch, and corruption.
- **Recommendation:** Enhance error messages with contextual data:
  ```go
  return nil, fmt.Errorf("decrypt ciphertext of %d bytes with key: %w", len(ciphertext), err)
  ```

### Domerr Package

#### 1. **Sentinel Errors Are Not Structured** (Limited Context)
- **Issue:** Errors are immutable sentinel values with no fields for additional context (e.g., which resource was not found, which field caused conflict).
- **Impact:** When wrapping `ErrNotFound`, callers must use `fmt.Errorf("user %d: %w", id, ErrNotFound)`, which is repetitive and error-prone.
- **Recommendation:** Consider an alternative design using structured error types:
  ```go
  type NotFoundError struct {
      Entity   string // e.g., "user"
      ID       string // e.g., "12345"
      Cause    error
  }
  ```
  Or use a minimal wrapper factory:
  ```go
  func NotFound(entity string, id string) error {
      return fmt.Errorf("%s %s: %w", entity, id, ErrNotFound)
  }
  ```
  This reduces boilerplate while maintaining `errors.Is()` semantics.

#### 2. **Limited Error Catalog** (Extensibility)
- **Issue:** Only 4 errors are defined. As the service grows, more specific domain errors (e.g., `ErrInvalidEmail`, `ErrPasswordViolatesPolicy`, `ErrTOTPMismatch`) may be needed.
- **Impact:** Either sentinels proliferate in this package, or callers resort to string-based error checking, losing type safety.
- **Recommendation:** Consider organizing errors by domain concern:
  ```go
  // domerr/auth.go
  var (
      ErrInvalidCredentials = errors.New("invalid credentials")
      ErrMFARequired        = errors.New("multi-factor authentication required")
  )
  
  // domerr/user.go
  var (
      ErrUserNotFound       = errors.New("user not found")
      ErrEmailAlreadyExists = errors.New("email already in use")
  )
  ```
  Or use a single structured error type with an enumerated `Code` field for backward compatibility.

#### 3. **The `Is()` Wrapper Function is Redundant** (Unnecessary Abstraction)
- **Issue:** `func Is(err, target error) bool { return errors.Is(err, target) }` adds no value over calling `errors.Is()` directly.
- **Impact:** Adds cognitive load and indirection without benefit; callers must import both `domerr` and understand why they should use `domerr.Is()` instead of the stdlib function.
- **Recommendation:** Remove the wrapper. If desired for consistency, document that callers should use `errors.Is()` from the standard library. If the team wants abstraction, define it at the handler/API boundary, not in the domain package.

#### 4. **No Guidance on When to Use Which Error** (Missing Contract)
- **Issue:** The package defines errors but provides no documentation or examples of when to use each in the context of the accounts service.
- **Impact:** Different handler functions may use errors inconsistently (e.g., using `ErrInternal` for all failures instead of `ErrNotFound` for missing resources).
- **Recommendation:** Add package-level documentation with usage examples:
  ```go
  // Package domerr defines canonical domain errors for the accounts service.
  //
  // Usage:
  //  - ErrNotFound: Return when a user or account lookup fails.
  //  - ErrAlreadyExists: Return when sign-up attempts duplicate email/username.
  //  - ErrUnauthorized: Return when authentication fails or user lacks permission.
  //  - ErrInternal: Return for unexpected failures (DB errors, panics, etc.).
  ```

### Cross-Package Observations

#### 1. **No Public Dependency on `accounts` Business Logic**
- **Issue:** Neither `crypto` nor `domerr` imports or references higher-level domain packages. This is good (avoids circular dependencies), but it also means they are generic utilities.
- **Observation:** These packages would be reusable in other services (e.g., `payments`, `auth`), suggesting they could be promoted to a shared internal library.
- **Recommendation:** If both `crypto` and `domerr` are used elsewhere, consider moving them to a `services/common/pkg/` or `shared/pkg/` folder. If they are accounts-specific, document this intent.

#### 2. **No Integration Tests Between Crypto and Domerr**
- **Issue:** The two packages are tested in isolation. There are no tests showing how `Decrypt()` errors map to `domerr` sentinels.
- **Impact:** Callers must manually wrap crypto errors into domain errors; easy to miss edge cases.
- **Recommendation:** Consider a wrapper layer (e.g., `internal/adapters/crypto.go`) that:
  ```go
  func DecryptSecret(key [32]byte, encoded string) ([]byte, error) {
      plaintext, err := crypto.Decrypt(key, encoded)
      if err != nil {
          return nil, fmt.Errorf("failed to decrypt secret: %w", domerr.ErrInternal)
      }
      return plaintext, nil
  }
  ```
  This bridges domain concerns and ensures consistent error handling.

#### 3. **Missing Validation Layer**
- **Issue:** `Encrypt()` and `Decrypt()` do not validate inputs beyond what the crypto/base64 libraries enforce. For example:
  - No check that `plaintext` is not too large for serialization
  - No check that `encoded` is not suspiciously large (potential DoS vector)
- **Recommendation:** Consider:
  - Maximum ciphertext size constant (e.g., `MaxCiphertextSize = 10MB`)
  - Maximum plaintext size constraint
  - Document these limits in function comments

---

## Summary

The `pkg` package provides **two orthogonal utilities**:
1. **Crypto:** Battle-tested AES-256-GCM encryption with clean error handling.
2. **Domerr:** Domain-specific error sentinels for semantic error categorization.

### Strengths
- ✅ No external dependencies (stdlib only)
- ✅ Correct cryptography (AES-GCM provides authentication)
- ✅ Comprehensive test coverage for edge cases
- ✅ Simple, readable code

### Refactoring Priorities (Highest to Lowest)
1. **Extract Crypto Service Interface** - Enables testability and decoupling
2. **Add Contextual Error Information** - Improves debuggability in production
3. **Document Error Usage Patterns** - Prevents inconsistent error handling
4. **Consider Error Structuring** - If domain errors grow beyond 4 sentinels
5. **Evaluate pkg Reusability** - Potential for shared library extraction
