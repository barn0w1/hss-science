# HSS Science Platform

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)

> **The private cloud infrastructure for HSS Science.**

[ **English** | [日本語](./README.ja.md) ]

## Overview

**hss-science** is a private cloud platform engineered to support the collaborative workflows of HSS Science.

Using multiple tools brings flexibility, but the lack of integration between them often becomes a bottleneck. Managing file storage, processing automation, and compute resources across different services creates redundant authentication, data fragmentation, and latency.

This platform unifies identity, storage, and compute to deliver **performance**, **data sovereignty**, and **simple developer experience**. Built on microservices architecture, it evolves through incremental feature additions and improvements to support expanding workflows.

## Services

The platform operates as a set of gRPC microservices, exposed to the frontend via a unified REST API (gRPC-Gateway).

| Service | Endpoint | Description |
| :--- | :--- | :--- |
| **Auth** | `accounts.hss-science.org` | **Identity & Session Management.** Centralized authentication via Discord OAuth. Handles JWT issuance and session management. |
| **Drive** | `drive.hss-science.org` | **Unified Storage.** A Content Addressable Storage (CAS) system backed by Cloudflare R2, ensuring data immutability and efficient deduplication. |
| **Compute** | `compute.hss-science.org` | **Dynamic Compute Foundation.** Orchestrates ephemeral GPU/CPU instances to execute heavy workloads such as rendering, data processing, and firmware compilation. |

## Architecture

### Design Principles

**Schema-Driven Development**  
All API definitions are centralized in `proto/`. Go servers and TypeScript clients are automatically generated from these definitions, ensuring type safety across the stack.

**Service Isolation**  
Each service is completely decoupled at domain boundaries. Inter-service communication uses gRPC exclusively.

**Explicit Configuration**  
Dependencies and settings are explicitly declared in code. No implicit behavior—clarity over convenience.

### Tech Stack

- **Backend**: Go 1.25+, gRPC, sqlx
- **Frontend**: TypeScript, React, pnpm workspaces
- **Infrastructure**: PostgreSQL, Redis, Cloudflare R2, Docker

## Getting Started

### Prerequisites

- Go 1.25+
- Docker & Docker Compose
- `buf` (Protocol Buffers code generation tool)

### Quick Start

**1. Generate code from Proto definitions**
```bash
make gen
```

**2. Start the local development environment**
```bash
make infra-up    # Start PostgreSQL, Redis
make infra-down  # Stop
```

**3. Run backend services**
```bash
make server-run
```

See [Makefile](./Makefile) for additional commands.

## License

GNU Affero General Public License v3.0 (AGPL-3.0)  
See [LICENSE](./LICENSE) for details.