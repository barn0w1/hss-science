# HSS Science Platform

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)

> **HSS Science Cloud Platform**

[ **English** | [日本語](./README.ja.md) ]

## Overview

**HSS Science Platform (hss-science)** is an internal cloud platform used by HSS Science.

Core capabilities such as authentication, storage, and compute are implemented as  
**small, independent services** and composed to form a flexible and scalable system.

As systems grow, complexity increases naturally.  
This platform mitigates that growth by maintaining clear responsibility boundaries and  
keeping each component as simple as possible, reducing overall system complexity and operational overhead.

## Services

Each service is implemented as a gRPC-based microservice.  
REST APIs are also exposed via gRPC-Gateway.

| Service | Endpoint | Description |
| :--- | :--- | :--- |
| **Auth** | `accounts.hss-science.org` | Authentication and session management. Issues JWTs via Discord OAuth. |
| **Drive** | `drive.hss-science.org` | Content-addressable storage backed by Cloudflare R2. |
| **Compute** | `compute.hss-science.org` | Manages CPU and GPU resources for compute-intensive workloads. |

## Technology Stack

- **Backend**: Go 1.25+, gRPC, sqlx  
- **Frontend**: TypeScript, React, pnpm workspaces  
- **Infrastructure**: PostgreSQL, Redis, Cloudflare R2, Docker  

## Development

### Requirements

- Go 1.25+
- Docker / Docker Compose
- buf

### Getting Started

```bash
make gen
make infra-up
make server-run
````

For additional details, see the [Makefile](./Makefile).

## License

GNU Affero General Public License v3.0 (AGPL-3.0)
See [LICENSE](./LICENSE) for details.