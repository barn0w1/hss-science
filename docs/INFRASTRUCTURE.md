# hss-science Infrastructure & Operations Guidelines

本ドキュメントは、hss-scienceシステムのインフラストラクチャ、デプロイメント、および運用（Production Readiness）に関する基本ルールを定義します。
すべてのサービス（BFF、gRPC）は、このガイドラインに準拠して実装・構成される必要があります。

## 1. ドメインとルーティング設計 (Reverse Proxy)

本システムは、Caddy を最前面のリバースプロキシとして配置し、サブドメインとパスベースのルーティングを行います。

* **ルーティングの基本原則:**
  * `/*` (ルート配下): フロントエンドのSPA（Viteビルドの静的ファイル）を配信、または該当のWebサーバーへルーティング。
  * `/api/*`: 各サブドメインに対応する BFF (Backend For Frontend) へリバースプロキシ。
* **ドメイン構成:**
  * `accounts.hss-science.org` -> Auth SPA & Accounts BFF (`server/bff/accounts`)
  * `drive.hss-science.org` -> Drive SPA & Drive BFF (`server/bff/drive`)

## 2. 設定管理 (12-Factor App)

本番環境、ステージング、ローカル開発環境の差異は、すべて**環境変数 (Environment Variables)** で吸収します。コード内へのハードコードは厳禁です。

* **必須の環境変数（例）:**
  * `ENV`: 実行環境 (`development`, `production` など)
  * `PORT`: サーバーの待受ポート
  * `LOG_LEVEL`: `DEBUG`, `INFO`, `WARN`, `ERROR`
* 各サービスは起動時に環境変数をパースし、不足している場合はFail-Fast（即座にパニックまたはFatal終了）させてください。

## 3. 構造化ロギング (Structured Logging)

オブザーバビリティ（可観測性）を統一するため、すべてのバックエンドサービス（BFF / gRPC）において以下のロギングポリシーを遵守します。

* **ライブラリ:** Go標準の `log/slog` を使用すること。
* **出力先:** 常に標準出力 (`os.Stdout`) に出力すること。ファイルへの書き出しは行わない（コンテナオーケストレータやログルーターに収集を委ねるため）。
* **フォーマット:** JSON形式 (`slog.NewJSONHandler`) を基本とする（ただし、`ENV=development` の場合は人間が読みやすい Text 形式にフォールバックしてもよい）。
* **必須コンテキスト:** リクエストを跨いで追跡できるよう、可能な限りログに以下を含めること。
  * `service` (例: `accounts-bff`, `accounts-grpc`)
  * `trace_id` または `request_id` (HTTPミドルウェアやgRPCインターセプタで付与)