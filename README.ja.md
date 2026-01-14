# HSS Science Platform

[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

> **HSS Science クラウドプラットフォーム**

[ [English](./README.md) | **日本語** ]

## Overview

**HSS Science Platform（hss-science）** は、HSS Science が利用する内部クラウド基盤です。

認証・ストレージ・計算といった基本的な機能を  
**小さく独立したサービスとして実装**し、それらを組み合わせることで  
柔軟で拡張可能なシステムを構成します。

システムの成長に伴い、複雑さは不可避に増加します。  
本プラットフォームでは、責務の境界を明確に保ち、  
各コンポーネントを可能な限り単純に維持することで、  
全体の複雑性と運用コストの増大を抑制します。

## Services

各サービスは gRPC ベースのマイクロサービスとして実装されています。  
gRPC-Gateway を通じて REST API も提供されます。

| Service | Endpoint | Description |
| :--- | :--- | :--- |
| **Auth** | `accounts.hss-science.org` | 認証およびセッション管理。Discord OAuth を用いて JWT を発行します。 |
| **Drive** | `drive.hss-science.org` | Cloudflare R2 をバックエンドとする Content Addressable Storage。 |
| **Compute** | `compute.hss-science.org` | CPU / GPU リソースを管理し、計算集約型ワークロードを実行します。 |

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

詳細なコマンドについては [Makefile](./Makefile) を参照してください。

## License

GNU Affero General Public License v3.0 (AGPL-3.0)
詳細は [LICENSE](./LICENSE) を参照してください。