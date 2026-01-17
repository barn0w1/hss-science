/project-root
├── cmd/                # Application entry points (main executables)
├── internal/           # Private application code (cannot be imported by external projects)
│   ├── entity/         # Core business entities and domain models (innermost layer)
│   ├── usecase/        # Application-specific business logic and interfaces (application layer)
│   ├── handler/        # Handlers/controllers (API, gRPC, CLI) - interface adapters layer
│   ├── repository/     # Repository interfaces (defined here) and implementations (in infrastructure)
│   └── infrastructure/ # Database connections, external APIs, frameworks implementations
├── pkg/                # Public utility code (safe to import by other projects)
└── go.mod              # Go module file