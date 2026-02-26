# hss-science System Architecture & Security Overview

## 1. システムの基本理念

本システム `hss-science` は、クラウドネイティブかつゼロトラストな設計思想に基づく、セキュアなマイクロサービス・アーキテクチャである。
「The Twelve-Factor App」の原則に従い、状態（ステート）を持たないスケーラブルなバックエンド、厳格に分離された認証・認可基盤、および非同期イベント駆動を採用している。

## 2. システム構成レイヤー

システムは以下の7つのレイヤーで構成され、それぞれの責務は厳密に分離されている。

1. **Edge Layer:** 
* **Cloudflare Tunnel:** 外部からの通信を直接内部クラスタへセキュアにルーティングする。
* **Cloudflare R2:** オブジェクトストレージ。画像やファイルの実体はバックエンドを通さず、エッジで処理される。

2. **Integration Layer:**
* **BFF (Backend-For-Frontend):** SPAとバックエンド群を繋ぐゲートウェイ。OIDCクライアント（Relying Party）として機能し、トークンの隠蔽とセッション管理を行う。

3. **Trust & Control Layer:**
* **accounts-idp (OP):** OpenID Provider。ユーザーの認証とJWT（ID/Access Token）の発行を行う。
* **OpenFGA (PDP):** Policy Decision Point。システム全体の認可（誰が・どのリソースに・何を行えるか）を一元管理する。

4. **Core Domain Layer:**
* **gRPC Microservices (RS):** Resource Server。ChatやDriveなどの各ビジネスロジックを担うステートレスなサービス群。

5. **State & Persistence Layer:**
* **PostgreSQL:** 永続化が必要な中核データ。
* **Redis:** 高速なキャッシュおよびBFFのセッションストア。

6. **Asynchronous Event Layer:**
* **NATS Event Bus:** マイクロサービス間の非同期通信を担う。認証済みのドメインイベントを配信し、サービスを疎結合に保つ。

7. **Observability Layer:**
* **Grafana / Loki / Tempo:** `slog` による構造化ログと、OpenTelemetryによる分散トレーシングを統合し、全サービスを監視する。

---

## 3. セキュリティ＆認証・認可設計

本システムにおける最大の原則は **「フロントエンド（SPA）を信用せず、重要なトークンをブラウザに渡さない」** ことである。

### 3.1. 認証フロー

* **BFFパターン（Confidential Client）の採用:** SPA自体は認証の仕組みを持たない。ユーザーのログイン要求はBFFへ送られ、BFFが `accounts-idp` (OP) との間でセキュアなOIDCフロー（認可コードフロー）を実行する。
* **トークンの隠蔽:** 取得したJWT（アクセストークン等）はBFF側のRedis等に保存される。SPAへは、JavaScriptからアクセス不可能な **Secure HTTP-Only Cookie** のみを発行し、XSS攻撃によるトークン流出を完全に防ぐ。

### 3.2. 認可とAPIアクセス

* **gRPC メタデータへの伝播:** SPAからのAPIリクエスト（Cookie付き）を受け取ったBFFは、セッションからJWTを取り出し、gRPC通信のメタデータ（ヘッダー）に付与して背後のマイクロサービス（RS）へプロキシする。
* **gRPC ミドルウェアでの検証:** 各gRPCサービスは、リクエストを受け取るとInterceptor（ミドルウェア）でJWTの署名と有効期限を検証する。
* **OpenFGAによるきめ細やかな認可:** JWTの検証に成功した後、gRPCサービスはOpenFGAに対して「このユーザー（Subject）は、要求されたリソース（Object）に対する権限（Relation）を持っているか？」を問い合わせ、許可された場合のみロジックを実行する。

### 3.3. セキュアなファイル操作 (Secure File Handling)

* **バックエンドのステートレス化:** gRPCサービスなどのバックエンドコンポーネントにはファイルの実体を一切置かない。
* **Presigned URLの活用:** ファイルのアップロード・ダウンロードが必要な場合、gRPCサービスは認証・認可を済ませた上で Cloudflare R2 の **Presigned URL（署名付きURL）** を発行しBFF経由でSPAへ返す。SPAはこのURLを用いて直接R2と通信する。これにより、サーバーの帯域・メモリ枯渇を防ぎつつセキュアなファイル操作を実現する。

---

## Instructions for AI Agents

1. **Never leak tokens:** SPA（フロントエンド）側にJWTやシークレットを露出させるコードを書いてはいけない。トークン管理は常にBFFで行うこと。
2. **Follow Twelve-Factor Config:** 設定値やシークレットはハードコードせず、必ず環境変数から注入する設計にすること。
3. **Respect the Boundaries:** gRPCサービスに認証ロジックを直接実装してはいけない。認証はBFF/IdPの責務であり、gRPC側は「渡されたJWTの検証」と「OpenFGAへの認可問い合わせ」のみを行う。
4. **Propagate Context:** 常に `context.Context` を引き回し、OpenTelemetryの `trace_id` や `slog` のロガーを後続の処理やエラーハンドリングに伝播させること。
5. **No File Payloads:** ファイルのバイナリデータをgRPCのペイロードに乗せてはいけない。常にオブジェクトストレージ（R2）, Cloudflare worketなどのEdge networkを利用すること。
