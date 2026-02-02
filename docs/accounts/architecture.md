# Accounts Service Architecture

## Responsibility Split

### platform/
Shared cross-service modules.
- `platform/config`: Env parsing + base app config.
- `platform/logger`: Logging setup.
- `platform/server`: gRPC + gateway helper (optional).

### services/accounts/
Service-specific behavior and wiring.
- `internal/app`: Composition root for accounts (config, DB, repos, usecase, servers).
- `internal/config`: Accounts-specific config extensions.
- `internal/domain`: Domain models + repository contracts.
- `internal/usecase`: Auth flows and business rules.
- `internal/adapter`: External-facing implementations.

## Adapter Structure

```
internal/adapter
├── oauth            # Discord OAuth integration
├── repository       # Postgres repositories
└── transport
    ├── routes.go    # HTTP path constants
    ├── http         # Public HTTP endpoints
    └── grpc         # Internal gRPC handlers + middleware
```

### transport/http
- Handles public HTTP endpoints:
  - `GET /v1/authorize`
  - `GET /v1/oauth/callback`
- Issued session cookie is the raw session token; DB stores only token hash.

### transport/grpc
- Internal gRPC APIs (e.g., VerifyAuthCode).
- Middleware is a no-op (network boundary).

## Runtime Flow
1. `cmd/server` calls `internal/app.Run()`.
2. App loads config, sets up logger, connects DB.
3. Repositories and usecases are wired.
4. HTTP server + gRPC server start concurrently.

## Notes
- Accounts uses both HTTP and gRPC, so it does not rely on `platform/server` gateway.
- Configuration is fully environment-driven. Backward compatibility is not required.
