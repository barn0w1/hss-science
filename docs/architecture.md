# HSS Science System Architecture

## 1. System Topology & Tech Stack
本システムは Zero Trust および Microservices アーキテクチャを採用する。

* **Edge Layer**
  * Routing: Cloudflare Tunnel (内部クラスタへの直結)
  * Object Storage: Cloudflare R2
* **Integration Layer (BFF)**
  * Role: OIDC Client (RP) / API Gateway / Token Manager
  * Datastore: Redis (Session Cache)
* **Identity Provider Layer (OP)**
  * Domain: `accounts.hss-science.org`
  * Core Library: `zitadel/oidc`
  * Strategy: AuthN（認証）は外部OIDC（Google等）に委譲。自システムはAuthZ（セッション管理とトークン発行）に専念。
* **Resource Server Layer (Microservices)**
  * Protocol: gRPC (Stateless)
  * Inter-service Communication: NATS (Event Bus)
  * Database: PostgreSQL
* **Authorization Layer (PDP)**
  * Engine: OpenFGA
  * Optimization: キャッシュおよびローカル評価を優先し、レイテンシを最小化。

## 2. Request & Data Flow
AIエージェントは、機能実装時に以下のデータフローを厳守すること。

### 2.1. Authentication Flow (Login)
SPAは認証に関与しない。
`SPA` -> `BFF` (Initiate OIDC) -> `OP (accounts)` -> `External IdP (Google)` -> `OP` (Issue JWT) -> `BFF` (Store JWT in Redis, Set Secure HTTP-Only Cookie to SPA)

### 2.2. API Request Flow (Standard)
SPAは直接gRPCを呼び出さない。
`SPA` (Cookie) -> `BFF` (Extract JWT from Redis -> Set to gRPC Metadata) -> `gRPC Service` (Validate JWT signature -> Evaluate OpenFGA -> Execute Logic)

### 2.3. File Upload/Download Flow (Large Payloads)
gRPCのペイロードにファイルを含めない。
`SPA` -> `BFF` -> `gRPC Service` (Check AuthZ -> Generate R2 Presigned URL) -> `BFF` -> `SPA` (Direct PUT/GET) -> `Cloudflare R2`

## 3. Microservices Boundaries
システムはリソースの性質（スケーリング特性）に応じて厳格に分割されている。サービス間の直接的なDB参照は禁止する。

| Service | Scaling Constraint | Primary DB / Storage | Note |
| :--- | :--- | :--- | :--- |
| `chat` | Connection/Memory bound | PostgreSQL | WebSocket/gRPC Streamingの維持 |
| `drive` | Network I/O bound | PostgreSQL (Metadata only), R2 | ファイル実体は持たない |
| `myaccount` | CPU/DB bound | PostgreSQL | OPからのClaimを利用してプロファイルを管理 |

## 4. Strict Guardrails for AI & Developers
コードを生成・変更する際は、以下の制約を**絶対的なルール**として扱うこと。

1. **No Tokens in Browser:** SPAに対してJWTやシークレットを絶対に送信しないこと。BFFとRedisによるセッション管理（Opaque Cookie）を維持する。
2. **Stateless gRPC:** gRPCサービス（RS）に状態（セッションやインメモリのファイルキャッシュ）を持たせないこと。Podがいつ破棄されても良い設計にする。
3. **OpenFGA First:** gRPCリクエストのハンドラーの先頭で、必ずJWTの検証とOpenFGAによる権限チェック（Subject-Relation-Object）を行うこと。
4. **Asynchronous by Default:** 複数サービスにまたがるデータの状態変更は、gRPCの同期呼び出しではなく、NATSを経由したイベント駆動（Event Sourcing / Choreography）で行うこと。

## 5. Repository Scope & Implementation Rules

### 5.1. Repository Boundaries (リポジトリの境界)
本リポジトリ（`hss-science`）の責務は、**アプリケーションのソースコード管理およびコンテナイメージのビルドまで**である。AIエージェントおよび開発者は以下の境界を厳守すること。

* **No Infrastructure Code:** Terraform、Kubernetes manifestsなどのインフラプロビジョニングに関するコードを含めないこと。
* **No DB Migrations:** データベースのマイグレーションは別システムで完全に自動化されている。本リポジトリ内には、migrationsのみを記述する。別システムで、自動的に、golang-migrate等のツールを用いて実行されることを前提とする。
* **Containerization:** 各マイクロサービスやBFFは、最終的にステートレスなDockerコンテナとしてパッケージングされることを前提に実装すること（Dockerfileの提供までが責務）。

### 5.2. Core Libraries & Languages
機能実装には以下の標準スタックを使用すること。

* **Frontend / SPA:** [例: React, TypeScript, Vite, Tailwind CSS]
* **BFF Layer:** [例: Go, net/http, chi]
* **Microservices (gRPC):** [例: Go]
* **Database Access (PostgreSQL):** [例: Go, database/sql, sqlx, pgx]
* **Event Bus (NATS):** [例: nats]