# Identity Package Research Report

## 1. Responsibilities & Core Logic

The `identity` package is the core domain layer of the `accounts` microservice responsible for managing **user identity and federated authentication**. It implements a federated identity pattern where external authentication providers (OAuth providers like Google, GitHub) are the authoritative sources of user authentication.

**Primary responsibilities:**
- **User lifecycle management**: Retrieve users by ID and manage user data (profile, email, name, picture)
- **Federated authentication**: Handle OAuth-style login flows by linking external provider identities to internal users
- **Find-or-create pattern**: Implement idempotent login operations—when a user authenticates via a federated provider, the service either retrieves their existing account or atomically creates a new user and federated identity link
- **Claims synchronization**: Update federated identity claims (profile data, email, display name) from external providers on subsequent logins while preserving the canonical user record

The architecture cleanly separates **domain logic** (business rules, entity definitions) from **data access** (PostgreSQL adapter), enabling testability and flexibility in persistence mechanisms.

---

## 2. Domain Models

### **User**
```go
type User struct {
    ID            string
    Email         string
    EmailVerified bool
    Name          string
    GivenName     string
    FamilyName    string
    Picture       string
    CreatedAt     time.Time
}
```
- **Role**: Core identity entity representing a person in the system
- **Semantics**: 
  - `ID` is the canonical user identifier (ULID-based)
  - `Email`, `EmailVerified`, `Name*` are the **primary/canonical** user profile attributes
  - These fields are immutable after initial creation (only set during federated user creation)
  - The user record is NOT updated on subsequent federated logins; only the linked federated identity is refreshed

### **FederatedIdentity**
```go
type FederatedIdentity struct {
    ID                    string
    UserID                string
    Provider              string
    ProviderSubject       string
    ProviderEmail         string
    ProviderEmailVerified bool
    ProviderDisplayName   string
    ProviderGivenName     string
    ProviderFamilyName    string
    ProviderPictureURL    string
    LastLoginAt           time.Time
    CreatedAt             time.Time
    UpdatedAt             time.Time
}
```
- **Role**: External identity link binding a User to a specific OAuth provider
- **Semantics**:
  - `Provider` + `ProviderSubject`: Globally unique composite key identifying the external account
  - `ProviderEmail`, `ProviderDisplayName`, `ProviderGivenName`, `ProviderFamilyName`, `ProviderPictureURL`: Mutable claims from the external provider, synchronized on every login
  - `LastLoginAt`: Temporal marker for tracking user activity
  - Database constraint: **One federated identity per user** (UNIQUE on user_id implied by tests; users cannot link multiple identities from the same provider)

### **FederatedClaims**
```go
type FederatedClaims struct {
    Subject       string
    Email         string
    EmailVerified bool
    Name          string
    GivenName     string
    FamilyName    string
    Picture       string
}
```
- **Role**: Data transfer object (DTO) representing claims extracted from an external provider (e.g., OpenID Connect or OAuth identi­ty response)
- **Lifecycle**: Ephemeral; passed into the service layer to create or update user/federated identity records

---

## 3. Ports & Interfaces

### **Repository Interface** (Data Access Abstraction)
```go
type Repository interface {
    GetByID(ctx context.Context, id string) (*User, error)
    FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*User, error)
    CreateWithFederatedIdentity(ctx context.Context, user *User, fi *FederatedIdentity) error
    UpdateFederatedIdentityClaims(
        ctx context.Context,
        provider, providerSubject string,
        claims FederatedClaims,
        lastLoginAt time.Time,
    ) error
}
```
- **Inbound port**: Used by the service to abstract database operations
- **Contracts**:
  - `GetByID`: Exact match on user ID; returns `ErrNotFound` if missing
  - `FindByFederatedIdentity`: Joins User ↔ FederatedIdentity; returns `nil` (not error) if federated identity doesn't exist
  - `CreateWithFederatedIdentity`: **Atomic transaction** creating user and federated identity together
  - `UpdateFederatedIdentityClaims`: Idempotent updates to federated identity only (user record unchanged)

### **Service Interface** (Domain/Business Logic)
```go
type Service interface {
    GetUser(ctx context.Context, userID string) (*User, error)
    FindOrCreateByFederatedLogin(ctx context.Context, provider string, claims FederatedClaims) (*User, error)
}
```
- **Outbound port**: Exported to HTTP handlers, GRPC services, or other callers
- **Contracts**:
  - `GetUser`: Simple retrieval with error propagation
  - `FindOrCreateByFederatedLogin`: Complex business logic orchestrating the find-or-create pattern

---

## 4. Dependencies

### Internal Dependencies
- **`domerr`** package: Domain error definitions (e.g., `ErrNotFound` for consistent error handling across layers)
- **`testhelper`** package: Test database helpers for migration management and table cleanup

### External Dependencies
- **`github.com/jmoiron/sqlx`**: SQL database abstraction library used for parameterized queries, transaction management, and struct-to-row mapping
- **`github.com/oklog/ulid/v2`**: ULID (Universally Unique Lexicographically Sortable Identifiers) generator for creating sortable, unique IDs
- **Standard library**:
  - `database/sql`: Raw SQL error handling (e.g., `sql.ErrNoRows`)
  - `context`: Context passing for graceful shutdown, cancellation, and deadlines
  - `time`: Temporal operations (timestamps, duration)
  - `fmt`: Error wrapping and formatting

### Database
- **PostgreSQL** (version 16 based on acceptance tests)
- **Schema assumptions**: Two tables referenced:
  - `users`: Columns (id, email, email_verified, name, given_name, family_name, picture, created_at)
  - `federated_identities`: Columns (id, user_id, provider, provider_subject, provider_email, provider_email_verified, provider_display_name, provider_given_name, provider_family_name, provider_picture_url, last_login_at, created_at, updated_at)
  - **Constraints** (inferred from tests, not explicit in this folder):
    - Primary keys on `users.id` and `federated_identities.id`
    - Foreign key `federated_identities.user_id` → `users.id`
    - Composite unique key on `(federated_identities.provider, federated_identities.provider_subject)`
    - Unique key on `federated_identities.user_id` (one federated identity per user)

---

## 5. Specifications from Tests

### **Service-Level Behavior** (`service_test.go`)

#### GetUser
- **Happy path**: Returns exact user match by ID
- **Error case**: Returns wrapped error with context ("identity.GetUser($id): $err") on not-found or database error
- **Error semantics**: Domain error `ErrNotFound` is preserved through error chain

#### FindOrCreateByFederatedLogin
**Existing user path:**
- If provider+subject combination exists, the service **does NOT create** a new user
- Calls `UpdateFederatedIdentityClaims` to refresh provider claims and set `LastLoginAt` to current UTC time
- Returns the existing user unchanged (canonical user profile not modified)
- Must ensure update is called; test validates this with mock spy

**New user path:**
- If provider+subject is not found, atomically creates:
  - New `User` with ULID-generated ID, claimed attributes from `FederatedClaims`, creation timestamp
  - New `FederatedIdentity` with ULID ID, matching user ID, provider details, creation/update timestamps
- FederatedIdentity's UserID must reference the created User's ID (referential integrity)
- Claims are fully copied: Email, EmailVerified, Name, GivenName, FamilyName, Picture from FederatedClaims
- LastLoginAt is set to current UTC time
- Returns the newly created user

**Error handling:**
- Lookup error: Immediate error propagation ("identity.FindOrCreate: lookup: $err")
- Update error (existing user): Wrapped error ("identity.FindOrCreate: update claims: $err")
- Create error (new user race): Wrapped error ("identity.FindOrCreate: create: $err")
- All errors indicate failure of the entire operation (no partial state)

#### FindOrCreateByFederatedLogin - ID Generation
- User and FederatedIdentity IDs must both be **non-empty** (validated by test)
- IDs are generated via `newID()` which uses ULID for sortable, unique IDs

### **Repository-Level Behavior** (`postgres/user_repo_test.go`)

#### GetByID
- **Found**: Returns hydrated `User` with all fields (email, verified flag, name components, picture, creation time)
- **Not found**: Returns `ErrNotFound`, not a nil user
- **Timestamp handling**: Queries must preserve created_at timestamp; tests use microsecond-truncated times

#### FindByFederatedIdentity
- **Found**: JOINs `federated_identities` → `users` on FK; returns the joined user
- **Not found**: Returns `nil` (not error) if provider+subject doesn't exist
- **Query semantics**: (provider, provider_subject) are composite lookup keys

#### CreateWithFederatedIdentity
- **Atomicity**: Uses database transaction (BEGIN/COMMIT on success, ROLLBACK on error)
- **Insert order**: User must be inserted before FederatedIdentity (foreign key constraint)
- **Data mapping**: All 8 user fields + all 13 federated identity fields must be inserted exactly
- **Failure modes**: 
  - Transaction rollback on any error
  - Unique constraint violations propagate (e.g., duplicate provider+subject)
  - Foreign key errors caught

#### UpdateFederatedIdentityClaims
- **Selective update**: Only updates federated_identities columns (provider_email, provider_email_verified, provider_display_name, provider_given_name, provider_family_name, provider_picture_url, last_login_at)
- **Trigger**: Uses database NOW() for updated_at timestamp (not client-side time)
- **User immutability**: Base user record IS NOT MODIFIED; user.email and user.name remain unchanged
- **Idempotency**: Multiple calls with same claims/timestamp are safe (no side effects)

#### Constraints & Cardinality
- **Test: TestUniqueUserID_Constraint**
  - Inserting a second federated identity for the same user_id violates UNIQUE constraint
  - Indicates one-to-one relationship: User ↔ FederatedIdentity
  - Users **cannot have multiple federated identities** (no multi-provider linking in current design)

---

## 6. Tech Debt & Refactoring Candidates

### **Architectural Issues**

1. **Single Federated Identity per User Limitation**
   - **Issue**: Database constraint (UNIQUE on user_id) enforces 1:1 User ↔ FederatedIdentity relationship
   - **Risk**: Users cannot link multiple OAuth providers (e.g., "login with Google" and "login with GitHub" for same account)
   - **Impact**: Fragmented user accounts if user has multiple provider identities
   - **Recommendation**: Refactor to allow 1:N relationship (User ↔ multiple FederatedIdentities). Update domain contract: `FindByFederatedIdentity` returns User, but multiple FIs can reference same User. Requires schema change and new index on (provider, provider_subject, user_id).

2. **User Email Synchronization Drift**
   - **Issue**: User.Email set at creation time from federated claims; never updated on subsequent logins
   - **Risk**: If user changes email in external provider, the internal User.Email becomes stale
   - **Current behavior**: Only FederatedIdentity fields are refreshed; User is immutable
   - **Recommendation**: Decide on canonical email source. Either:
     - (A) Always sync User.Email from federated claims on login (requires schema/API change)
     - (B) Accept that User.Email is historical; query FederatedIdentity.ProviderEmail as current email
     - Document this clearly in domain model

3. **Implicit Database Schema**
   - **Issue**: Schema is not captured in this package; inferred from code and tests
   - **Risk**: Schema evolution becomes opaque; migrations live elsewhere (`testhelper`, external migration system)
   - **Recommendation**: Add schema definition or migration files to this package. Include indexes, constraints, and cardinality contracts as documentation.

### **Code Quality Issues**

4. **Tight Coupling: postgres Adapter**
   - **Issue**: `postgres.UserRepository` is tightly coupled to `sqlx.DB` with no interface abstraction
   - **Risk**: Hard to test with different databases or mock implementations (though tests use real PostgreSQL via testcontainers)
   - **Current state**: Tests use integration testing with real database, which is thorough but slower
   - **Recommendation**: Consider extracting a low-level `Executor` interface for query/exec operations, allowing easier unit testing or database switching. However, current integration test approach is stronger for catching real database issues.

5. **Error Wrapping Inconsistency**
   - **Issue**: Service layer uses `fmt.Errorf()` for wrapping; inconsistent message styles
     - "identity.GetUser(...): %w"
     - "identity.FindOrCreate: lookup: %w"
   - **Risk**: Callers struggle to parse error context
   - **Recommendation**: Use structured error context or standardized format (e.g., `fmt.Errorf("[service.method] %w")`)

6. **No Input Validation**
   - **Issue**: Service accepts provider string and claims without validation
   - **Risk**: Empty provider strings, missing subject, etc. may cause silent failures or nonsensical records
   - **Recommendation**: Add validation layer:
     ```go
     if provider == "" || claims.Subject == "" {
         return nil, ErrInvalidClaims
     }
     ```

7. **Hardcoded Timestamps**
   - **Issue**: Service uses `time.Now().UTC()` for all timestamps
   - **Risk**: Not injectable; difficult to test time-dependent behavior (e.g., LastLoginAt progression)
   - **Recommendation**: Inject a clock abstraction (e.g., `type Clock interface { Now() time.Time }`) for testability

8. **Missing Context Deadline Handling**
   - **Issue**: Service and repository don't check `ctx.Err()` before database operations
   - **Risk**: Cancelled contexts may still execute queries
   - **Current state**: `sqlx` and database/sql do respect context cancellation during Exec/QueryRow, so this is mostly safe
   - **Recommendation**: No immediate action; sqlx handles this well

### **Performance & Scalability**

9. **No Query Indexing Guidance**
   - **Issue**: Repository assumes indexes exist; no documentation or hints
   - **Recommendation**: Add comments documenting required indexes:
     ```go
     // Requires: CREATE UNIQUE INDEX idx_fi_provider_subject ON federated_identities(provider, provider_subject)
     // Requires: CREATE FOREIGN KEY fk_fi_user_id ON federated_identities(user_id) REFERENCES users(id)
     ```

10. **No Caching Layer**
    - **Issue**: Every `GetByID` or `FindByFederatedIdentity` hits the database
    - **Risk**: High-traffic accounts service could experience database bottleneck
    - **Recommendation**: Optional—add cache interface (e.g., Redis) as secondary port. Keep domain layer unaware; implement in adapter.

### **Testing & Documentation**

11. **Incomplete Test Coverage of Edge Cases**
    - **Missing**: 
      - Concurrent create attempts (race condition testing)
      - Claims with empty/null fields (partially populated claims)
      - Very long email or name strings (validation bounds)
    - **Recommendation**: Add property-based tests or fuzz testing for claims fields

12. **No API Documentation**
    - **Issue**: Service interface lacks docstrings explaining contract guarantees
    - **Recommendation**: Add GoDoc comments explaining:
      - Idempotency: `FindOrCreateByFederatedLogin` is idempotent for same (provider, subject, claims)
      - Error semantics: Which errors are recoverable
      - Timestamp semantics: All times are UTC

### **Domain Model Issues**

13. **FederatedClaims vs FederatedIdentity Field Mismatch**
    - **Issue**: `FederatedClaims` structure doesn't match update signature
      - Claims has `Subject`, `Email`, `EmailVerified`, `Name`, `GivenName`, `FamilyName`, `Picture`
      - UpdateFederatedIdentityClaims takes these fields individually
    - **Risk**: Unclear mapping; error-prone to add new claim types
    - **Recommendation**: Pass `FederatedClaims` directly to `UpdateFederatedIdentityClaims`:
      ```go
      UpdateFederatedIdentityClaims(ctx, provider, subject, claims, lastLoginAt time.Time) error
      ```

---

## Summary

The `identity` package implements a **clean domain layer** with clear separation of concerns (ports/adapters pattern). Its strength lies in atomic transactions, straightforward federated authentication logic, and comprehensive integration tests. 

Key production considerations:
- One federated identity per user limits multi-provider linking
- User profile data is immutable post-creation; federated claims are mutable
- Robust error handling with context wrapping
- PostgreSQL-specific; schema must exist with proper constraints

Refactoring priorities for maturity:
1. Support multiple federated identities per user (highest impact)
2. Standardize error wrapping and add validation
3. Document implicit schema contracts
4. Consider caching for high-traffic scenarios
