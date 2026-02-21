# hss-science System Architecture

このドキュメントは、hss-scienceシステム全体のアーキテクチャと、各コンポーネントの関心の分離（分離原則）を定義します。AIエージェントはコードを生成する際、必ずこの制約に従ってください。

## システムトポロジー
本システムは、BFF（Backend for Frontend）とバックエンドのマイクロサービス群（gRPC）から構成されます。

* **Reverse Proxy (Caddy):** トラフィックの入り口。TLS終端とルーティングを担当。
* **BFF (gateway):** HTTP APIの提供、セッション（Cookie）管理、gRPCエラーのHTTPステータスへの変換を担当。
* **Microservices (gRPC):** `auth`, `drive`, `chat` などの各ドメイン領域。純粋なビジネスロジックとデータベース操作のみを担当。

## 設計ルール

### Rule 1: 関心の完全な分離
* **BFF (`server/gateway/`)**: HTTPリクエスト、Cookie、CORS、リダイレクトなどの「Webの都合」は**すべてここで処理**します。データベースには絶対に直接アクセスしません。
* **gRPC Services (`server/services/`)**: HTTPやCookieの概念を**一切持たせてはいけません**。BFFからgRPCのメタデータ（Context）経由で渡されたユーザーIDなどの情報のみを信頼して、ビジネスロジックを実行します。

### Rule 2: Database per Service
* 各gRPCサービス（`auth`, `drive`, `chat`）は、**完全に独立したデータベース（PostgreSQL）**を持ちます。
* 他のサービスのデータベースを直接JOINしたり、クエリを発行したりすることは厳禁です。データ連携が必要な場合は、必ずgRPCのAPI経由で行います。

## 認証アーキテクチャとSSOフロー
本システムは独自のシングルサインオン（SSO）を提供します。

1.  **Identity Provider (IdP):** `auth` サービスがシステム全体のIdPとして機能します（親プロバイダーはDiscord OAuth）。
2.  **画面を持たないAuth:** `auth` サービスはフロントエンドを持ちません。リダイレクトベースの認可コードフローのみを提供します。
3.  **セッションの独立性:** `auth` を経由して認証が完了したのち、各サービス（`drive`, `chat` のBFF）は**それぞれ独自のセッション**を発行し、管理します。

## デプロイメント
* 本番環境はDockerコンテナとして各サービスが独立してデプロイされます。開発環境でもこの分離を意識したディレクトリ構造（`Dockerfile`が各サービスにある等）を維持してください。