# HSS Science

**HSS Science** is a unified cloud platform—envisioned as a custom, Google Workspace-like suite of interconnected applications—developed for a specific community under the domain **hss-science.org**.

Centered around a core Identity Provider (IdP) and Single Sign-On (SSO) foundation, this repository (`hss-science`) contains the application source code for the entire platform, including backend microservices, frontend applications, and edge components.

---

## Repository Responsibility

This repository is responsible for:

* Maintaining the **application source code**
* Building applications
* Producing artifacts such as **container images**
* Pushing those artifacts to a container registry

This repository is **not responsible** for:

* Deployment to any environment (e.g., staging, production)
* Infrastructure provisioning
* Runtime orchestration
* Database migrations execution

Database migration **schemes and definitions** may be stored in this repository,
however, the execution of migrations is handled outside of this repository's scope.

In short:

> This repository defines and builds the software.
> It does not deploy or operate the infrastructure.

---

## Testing Policy

This project follows a **Unit Test-centric** approach to ensure individual component reliability and fast feedback loops.

### Unit Testing

* **Focus:** Most of the business logic and utility functions must be covered by unit tests.
* **Execution:** Developers are encouraged to run tests frequently during local development:
```bash
go test ./...
```

### End-to-End (E2E) Testing

To keep the local development environment lightweight and focused on code iteration, **E2E tests are not performed locally.**

* **Local Environment:** Limited to unit and integration tests that do not require external cloud dependencies.
* **Staging Environment:** Full E2E testing is conducted exclusively in the **Staging environment**. This ensures the platform's interconnected applications (IdP, SSO, and microservices) are validated in a production-like setting before any release.

---

## Repository Metadata

* **Canonical Repository:** [https://github.com/barn0w1/hss-science](https://github.com/barn0w1/hss-science)
* **Visibility:** Private (personal repository)
* **Ownership:** Individual development project

This repository is private and not publicly accessible.
It is not an open-source project.

---

## Development Log

### 2026-01-12

* Project initiated.

---

## Directory Structure (Simplified)
```txt
.
├── Makefile
├── README.md
├── buf.gen.yaml
├── buf.lock
├── buf.yaml
├── server
│   ├── go.mod
│   ├── go.sum
│   └── services
└── web
    ├── CLAUDE.md
    ├── apps
    ├── node_modules
    ├── package.json
    ├── pnpm-lock.yaml
    └── pnpm-workspace.yaml
```