# Role & Mission
You are an expert Go backend engineer, frontend developer, and system architect.
Your mission is to design and implement the "MyAccount" ecosystem. This should not be just a simple profile viewer, but a comprehensive account management platform similar to `myaccount.google.com`, allowing users to securely view and manage their personal information, security settings, and linked accounts.

To achieve this, you need to establish a complete end-to-end flow across three distinct components:
1. **`accounts` (Backend Modification):** Upgrade our existing OpenID Provider (`accounts-idp`) to ALSO serve as a gRPC Resource Server.
2. **`myaccount-bff` (New):** A Backend-For-Frontend acting as the OIDC Relying Party (RP) and REST API Gateway (defined via OpenAPI).
3. **`myaccount-spa` (New):** The React frontend UI using generated type-safe API clients.

**Important Context & Freedom:** - Please read `docs/architecture.md` and `docs/ecosystem.md` to understand our architectural guardrails.
- **Codebase Mutability:** You are free to heavily refactor, restructure, or rewrite the existing codebase (including `accounts-idp`) to achieve the best architectural practices. Do not hesitate to break things if it leads to a cleaner, more maintainable, and robust system.

# Component Requirements & Execution Phases

Agent, please analyze the current codebase and formulate the most appropriate best-practice architecture. You have the autonomy to plan and execute. **Proceed with the implementation step-by-step.**

## Phase 1: Enhance `accounts` with gRPC (Resource Server)
Previously, `accounts` was strictly built as an OP without gRPC. **We are now lifting that constraint.** The `accounts` database is the Single Source of Truth for user data. It must now serve that data internally via gRPC for account management purposes.
- **Protobuf:** Plan and define the necessary RPCs and messages in `api/proto/accounts/v1/accounts.proto` to support a robust account management system.
- **Implementation:** Implement the gRPC server in `server/services/accounts`.
- **Coexistence:** Modify `cmd/server/main.go` so that the application runs BOTH the existing HTTP OP server (for `zitadel/oidc`) AND the new gRPC server concurrently (e.g., on different ports).
- **Tech Stack:** Go 1.25.5, standard `google.golang.org/grpc`, `database/sql` + `sqlx` (reuse the existing DB connection).

## Phase 2: Build `myaccount-bff` (Integration Layer & OpenAPI)
This is a completely new Go service located in `server/bff/myaccount`. It has two primary responsibilities:
1. **OIDC Relying Party (RP):** - Implement the OIDC Authorization Code Flow against our `accounts-idp`.
   - Use standard `golang.org/x/oauth2` or `coreos/go-oidc/v3`.
   - **Crucial:** Store the retrieved tokens securely in Redis. Do NOT send JWTs to the frontend. Issue a secure, HTTP-Only session cookie to the SPA.
2. **OpenAPI Definition & gRPC Proxy:**
   - **Contract First:** Design the REST API contract in `api/openapi/myaccount/v1/openapi.yaml`.
   - Expose REST/JSON endpoints for the SPA based strictly on this OpenAPI spec.
   - **Constraint:** ALL backend API routes MUST be prefixed with `/api/` (e.g., `/api/v1/*`) to avoid conflicts with the SPA's client-side routing.
   - When an `/api/*` endpoint is called, validate the session cookie, extract the user's token from Redis, attach it to the gRPC metadata (e.g., `authorization: Bearer <token>`), and proxy the call to the `accounts` gRPC server.
- **Tech Stack:** Go 1.25.5, `go-chi/chi` for routing, `redis/go-redis/v9` (You may use `oapi-codegen` for the BFF backend if it fits our simple architecture).

## Phase 3: Build `myaccount-spa` (Frontend)
A new React application located in `web/apps/myaccount`.
- **Type-Safe API Client:** Use `openapi-typescript` and `openapi-typescript-fetch` to automatically generate TypeScript interfaces and the fetch client based on the OpenAPI specification created in Phase 2.
- **Reference:** You MUST refer to the existing codebase in `web/apps/chat` as a structural and architectural baseline for building this new SPA.
- Implement a robust UI for account management. 
- Ensure the SPA handles its own client-side routing cleanly, while making all data-fetching network requests strictly using the generated fetch client (ensure `credentials: 'include'` is set to send the HTTP-Only cookie).
- **Tech Stack:** React, TypeScript, Vite, Tailwind CSS. 

# Strict Architectural Constraints
- **Do NOT duplicate databases:** `myaccount` does not have its own PostgreSQL database. It relies entirely on the `accounts` service for user data.
- **Stateless Services:** Both `accounts` and `myaccount-bff` must remain stateless (except for Redis in the BFF).
- **Opaque Tokens for Browser:** The SPA must never see or handle JWTs. It only knows about the opaque session cookie provided by the BFF.

Please outline your best-practice architectural plan for the entire ecosystem first, detailing `accounts.proto` and `openapi.yaml`. Once you have output the plan, immediately begin implementing Phase 1.

# Current directory structure
```txt
.
├── Makefile
├── README.md
├── api
│   ├── openapi
│   │   └── chat
│   │       └── v1
│   └── proto
│       ├── accounts
│       │   └── v1
│       │       └── accounts.proto
│       ├── chat
│       │   └── v1
│       └── drive
│           └── v1
│               └── drive.proto
├── buf.gen.yaml
├── buf.lock
├── buf.yaml
├── docs
│   ├── architecture.md
│   ├── ecosystem.md
│   └── memo
│       ├── architecture.md
│       └── components.md
├── prompts
│   ├── IDP_REQUIREMENTS.md
│   └── MYACCOUNT.md
├── server
│   ├── bff
│   ├── go.mod
│   ├── go.sum
│   └── services
│       └── accounts
│           ├── Dockerfile
│           ├── README.md
│           ├── cmd
│           │   └── server
│           │       └── main.go
│           ├── internal
│           │   ├── authn
│           │   │   ├── google.go
│           │   │   └── provider.go
│           │   ├── config
│           │   │   └── config.go
│           │   ├── database
│           │   │   └── postgres.go
│           │   ├── storage
│           │   │   ├── auth_request.go
│           │   │   ├── client.go
│           │   │   ├── keys.go
│           │   │   ├── storage.go
│           │   │   ├── token.go
│           │   │   └── user.go
│           │   └── web
│           │       ├── login.go
│           │       ├── templates
│           │       │   ├── error.html
│           │       │   └── login.html
│           │       └── templates.go
│           └── migrations
│               ├── 000001_init.down.sql
│               └── 000001_init.up.sql
└── web
    ├── CLAUDE.md
    ├── apps
    │   ├── chat
    │   │   ├── Dockerfile
    │   │   ├── README.md
    │   │   ├── dist
    │   │   │   ├── 01.svg
    │   │   │   ├── assets
    │   │   │   │   ├── index-BfRlFBi0.css
    │   │   │   │   └── index-zq7xWvU3.js
    │   │   │   ├── chat.svg
    │   │   │   ├── icon.svg
    │   │   │   ├── index.html
    │   │   │   ├── manifest.webmanifest
    │   │   │   ├── pwa-256x256.png
    │   │   │   ├── pwa-512x512.png
    │   │   │   ├── registerSW.js
    │   │   │   ├── relaxing-outdoors.svg
    │   │   │   ├── sw.js
    │   │   │   └── workbox-01f28f5c.js
    │   │   ├── eslint.config.js
    │   │   ├── index.html
    │   │   ├── nginx.conf
    │   │   ├── package.json
    │   │   ├── public
    │   │   │   ├── 01.svg
    │   │   │   ├── chat.svg
    │   │   │   ├── icon.svg
    │   │   │   ├── pwa-256x256.png
    │   │   │   ├── pwa-512x512.png
    │   │   │   └── relaxing-outdoors.svg
    │   │   ├── src
    │   │   │   ├── app
    │   │   │   │   ├── App.tsx
    │   │   │   │   ├── layouts
    │   │   │   │   │   ├── MainAreaLayout.tsx
    │   │   │   │   │   └── MainLayout.tsx
    │   │   │   │   └── router.tsx
    │   │   │   ├── assets
    │   │   │   ├── features
    │   │   │   │   ├── auth
    │   │   │   │   │   ├── api.ts
    │   │   │   │   │   ├── components
    │   │   │   │   │   ├── state.ts
    │   │   │   │   │   └── types.ts
    │   │   │   │   ├── chat
    │   │   │   │   │   ├── api
    │   │   │   │   │   │   ├── index.ts
    │   │   │   │   │   │   ├── mock.ts
    │   │   │   │   │   │   └── rest.ts
    │   │   │   │   │   ├── components
    │   │   │   │   │   │   ├── ChatContent.tsx
    │   │   │   │   │   │   ├── ChatHeader.tsx
    │   │   │   │   │   │   └── ChatSidebar.tsx
    │   │   │   │   │   ├── hooks
    │   │   │   │   │   │   └── useSidebarData.ts
    │   │   │   │   │   ├── mock.ts
    │   │   │   │   │   ├── state.ts
    │   │   │   │   │   └── types.ts
    │   │   │   │   └── search
    │   │   │   │       ├── api
    │   │   │   │       ├── components
    │   │   │   │       │   └── GlobalSearchBar.tsx
    │   │   │   │       └── hooks
    │   │   │   ├── main.tsx
    │   │   │   ├── pages
    │   │   │   │   └── ChatPage.tsx
    │   │   │   ├── shared
    │   │   │   │   ├── api
    │   │   │   │   │   └── client.ts
    │   │   │   │   ├── hooks
    │   │   │   │   ├── types
    │   │   │   │   ├── ui
    │   │   │   │   │   ├── DebugPlaceholder.tsx
    │   │   │   │   │   ├── PanelHeader.tsx
    │   │   │   │   │   └── PanelLayout.tsx
    │   │   │   │   └── utils
    │   │   │   │       └── constants.ts
    │   │   │   └── styles
    │   │   │       ├── index.css
    │   │   │       └── tailwind.css
    │   │   ├── tsconfig.app.json
    │   │   ├── tsconfig.json
    │   │   ├── tsconfig.node.json
    │   │   └── vite.config.ts
    │   └── drive
    ├── package.json
    ├── pnpm-lock.yaml
    └── pnpm-workspace.yaml
```