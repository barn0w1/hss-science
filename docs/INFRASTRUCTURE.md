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

## 2. デプロイメント (Deployment)

一般的な Kubernetes や Docker Compose は採用せず、**自作の Go 製コンテナオーケストレーター**を使用します。このオーケストレーターは Docker Engine API を直接操作し、コンテナのライフサイクル（Pull / Run / Stop / Health Check）を管理します。

### 2.1 CI/CD パイプライン

```
[GitHub Actions]  ──build & push──>  [GHCR (GitHub Packages)]
                                            │
                                            │ pull & deploy
                                            ▼
[Discord Bot (ChatOps)]  ──指示──>  [Go製オーケストレーター]  ──Docker Engine API──>  [コンテナ群]
```

1. **ビルド (GitHub Actions):** Push / PR マージをトリガーに、各サービスの Docker イメージをビルドし、GHCR (`ghcr.io`) へ Push する。
2. **デプロイ (ChatOps):** Discord Bot 経由のコマンドでオーケストレーターに指示を送り、GHCR から最新イメージを Pull してデプロイを実行する。手動承認とロールバックが容易な運用フローを実現する。
3. **監視 (Monitoring):** Grafana 等のモニタリングスタックは将来のフェーズで導入予定。現時点ではコンテナログ（`slog` による構造化 JSON）と Discord 通知をベースに運用する。

### 2.2 コンテナイメージの方針

* イメージレジストリは **GHCR (GitHub Packages)** を使用する。
* 各サービスの Dockerfile はマルチステージビルドとし、最終イメージは `gcr.io/distroless/static-debian12` 等の最小イメージを使用する。
* イメージタグは Git の SHA ハッシュを基本とし、`latest` タグは明示的なデプロイ指示時にのみ更新する。

## 3. 設定管理 (12-Factor App)

本番環境、ステージング、ローカル開発環境の差異は、すべて**環境変数 (Environment Variables)** で吸収します。コード内へのハードコードは厳禁です。

* **必須の環境変数（例）:**
  * `ENV`: 実行環境 (`development`, `production` など)
  * `PORT`: サーバーの待受ポート
  * `LOG_LEVEL`: `DEBUG`, `INFO`, `WARN`, `ERROR`
* 各サービスは起動時に環境変数をパースし、不足している場合はFail-Fast（`fmt.Fprintf(os.Stderr, ...)` + `os.Exit(1)` で即時終了）させてください。
* すべての環境変数は `.env.example` に一覧化し、ダミー値とコメントを添えて管理する。

## 4. 構造化ロギング (Structured Logging)

オブザーバビリティ（可観測性）を統一するため、すべてのバックエンドサービス（BFF / gRPC）において以下のロギングポリシーを遵守します。

* **ライブラリ:** Go標準の `log/slog` を使用すること。
* **出力先:** 常に標準出力 (`os.Stdout`) に出力すること。ファイルへの書き出しは行わない（コンテナオーケストレータやログルーターに収集を委ねるため）。
* **フォーマット:** JSON形式 (`slog.NewJSONHandler`) を基本とする（ただし、`ENV=development` の場合は人間が読みやすい Text 形式にフォールバックしてもよい）。
* **必須コンテキスト:** リクエストを跨いで追跡できるよう、可能な限りログに以下を含めること。
  * `service` (例: `accounts-bff`, `accounts-grpc`)
  * `trace_id` または `request_id` (HTTPミドルウェアやgRPCインターセプタで付与)

## 5. テスト戦略 (Testing Strategy)

システムのスケール時に依存地獄を避けるため、ローカルでの全サービス連動型 E2E テスト（大規模な docker-compose 等）は**行いません**。テストは各層で完結させることを原則とします。

### 5.1 テストピラミッド

```
        ┌───────────┐
        │  手動検証  │  ← 本番/ステージング環境でのスモークテスト
       ─┴───────────┴─
      ┌─────────────────┐
      │ Integration Test │  ← testcontainers 等で DB/外部依存を含む限定テスト
     ─┴─────────────────┴─
    ┌───────────────────────┐
    │      Unit Test        │  ← モック/スタブを活用した高速なテスト（主軸）
    └───────────────────────┘
```

### 5.2 各層のテスト方針

| 層 | テスト手法 | ツール・技法 |
|---|---|---|
| **domain / service** | ユニットテスト | モックリポジトリ、モックプロバイダを注入してビジネスロジックを検証 |
| **store (永続化)** | インテグレーションテスト | `testcontainers-go` で PostgreSQL コンテナを起動し、実際の SQL を検証 |
| **transport (gRPC)** | ユニットテスト | `bufconn` を使ったインプロセス gRPC テスト |
| **BFF (HTTP)** | ユニットテスト | `httptest` + モック gRPC クライアントでハンドラを検証 |

### 5.3 禁止事項

* ローカルで全サービスを `docker-compose up` して行うフルスタック E2E テストは作成しない。
* テスト用の共有データベースやテスト環境への依存を前提としたテストは作成しない。
* 各テストは独立して実行可能であること（テスト間の実行順序依存を排除する）。