# HSS Science Platform

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

The unified distributed platform for the HSS Science community.
Engineered to create order out of chaos through strict architectural discipline.

## Philosophy

We believe that information naturally tends toward disorder. This platform is our resistance.

- **Single Source of Truth**: A unified monorepo to maintain absolute consistency across all services.
- **Do One Thing Well**: Strictly decoupled microservices (`apps`) sharing a common foundation (`pkg`).
- **Simplicity as Strength**: Powered by Go to enforce structural integrity and performance.

## Architecture

This repository hosts the entire ecosystem, orchestrated as a distributed system:

### Core Services
- **`apps/auth`**: Identity Provider (IdP) and SSO foundation based on JWT.
- **`apps/drive`**: Content Addressable Storage (CAS) for immutable data management.
- **`apps/render`**: Distributed render farm controller orchestrating GPU instances.

### Infrastructure
- **`pkg`**: Shared standard libraries (Logger, Config, Discord Audit).
- **`proto`**: gRPC definitions serving as the immutable contract between services.

## Status

**Pre-alpha.** The universe is currently being formed.