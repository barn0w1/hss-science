# HSS Science Platform - 認証・認可アーキテクチャ設計書

## 1. 設計思想と基本方針 (Design Philosophy)

本プラットフォームは、最新のエンタープライズ・セキュリティのベストプラクティスに基づき、以下の基本方針で設計されています。

* **完全なパスワードレス (Passwordless):** 自システムでパスワードを管理せず、認証（AuthN）はすべてGoogle OIDCに委譲する。
* **関心の分離 (Separation of Concerns):** BFFはルーティングとセッション（Cookie）管理に徹し、ビジネスロジックとトークン発行（鍵管理）はバックエンドの `Account Service` に隠蔽する。
* **強固なセッション管理:** XSS攻撃を無効化する `HttpOnly` Cookieと、CSRFを防ぐ `SameSite=Strict` + カスタムヘッダー検証を採用する。
* **ハイブリッド・トークン:** 普段のAPIはDBアクセスゼロの「ステートレス（Access Token）」で高速化しつつ、デバイスごとの強制ログアウトを可能にする「ステートフル（Refresh Token）」なセッション管理を両立する。
* **ドメインごとのセキュリティ境界:** DriveとChatで完全に独立したCookieを発行し、一方の脆弱性が他方に波及しないゼロトラスト構造とする。

---

## 2. システム・コンポーネント構成 (Architecture Diagram)

ネットワークのエッジ（境界）から最深部のDBまでのデータの流れとコンポーネントの配置です。

```mermaid
graph TD
    %% Clients
    SPA_Drive["Drive SPA (React/Vue)"]
    SPA_Chat["Chat SPA (React/Vue)"]

    %% Edge & Proxy
    subgraph "Edge & Network Layer"
        CF["Cloudflare Edge & Tunnel"]
        Caddy["Reverse Proxy (Caddy)"]
    end

    %% BFF Layer
    subgraph "Gateway Layer (Stateless)"
        BFF_Drive["Drive BFF (Go)"]
        BFF_Chat["Chat BFF (Go)"]
    end

    %% Backend Layer
    subgraph "Backend Services (gRPC / Isolated Network)"
        AccountSvc["Accounts Service (Go) <br> *JWT Minting & Session Mgt*"]
        DriveSvc["Drive Service"]
        ChatSvc["Chat Service"]
    end

    %% Data & AuthZ Layer
    subgraph "Data & Authorization Layer"
        DB[(PostgreSQL)]
        OpenFGA{"OpenFGA (AuthZ)"}
    end

    %% External
    Google(("Google OIDC"))

    %% Connections
    SPA_Drive -->|HTTPS| CF
    SPA_Chat -->|HTTPS| CF
    CF --> Caddy
    
    Caddy -->|/api/*| BFF_Drive
    Caddy -->|/api/*| BFF_Chat

    BFF_Drive <-->|OAuth / PKCE| Google
    BFF_Chat <-->|OAuth / PKCE| Google

    BFF_Drive -->|gRPC| AccountSvc
    BFF_Chat -->|gRPC| AccountSvc
    
    BFF_Drive -->|gRPC| DriveSvc
    BFF_Chat -->|gRPC| ChatSvc

    AccountSvc -->|Read/Write User & Session| DB
    
    DriveSvc -->|Check Permission| OpenFGA
    ChatSvc -->|Check Permission| OpenFGA

```

---

## 3. コンポーネントの責務定義

| コンポーネント | 役割と責務 |
| --- | --- |
| **SPA (Client)** | UIの描画。トークンの存在（中身）は一切知らず、リクエスト時に自動付与されるCookieに依存する。 |
| **Cloudflare / Caddy** | SSL終端、DDoS防御、内部ネットワーク（VPC等）への安全なトンネリング。 |
| **BFF (Gateway)** | Google OIDCのコールバック処理。SPAへの `Set-Cookie`（`HttpOnly`, `SameSite=Strict`）。gRPC通信時の `x-user-id` メタデータ付与。CSRFヘッダーの検証。**※秘密鍵は持たない。** |
| **Accounts Service** | システムの「IdP兼ユーザー管理」の心臓部。Google IDからの内部ID (`usr_xxx`) の生成（JITプロビジョニング）。デバイスごとのセッションID管理。**JWT（Access/Refresh）の署名と発行。** |
| **Domain Services** | BFFから渡された内部ID (`x-user-id`) を絶対的に信頼し、ビジネスロジックを実行する。 |
| **OpenFGA / DB** | DBは全ユーザーとセッションの唯一のソース（Single Source of Truth）。OpenFGAはドメインサービスからの認可（ファイルアクセス権など）を判定する。 |

---

## 4. 認証・認可・セッション管理フロー (Sequence Diagram)

JWTの発行責務をAccount Serviceに移譲した、最も堅牢なシーケンスです。

```mermaid
sequenceDiagram
    autonumber

    participant SPA as SPA (Client)
    participant BFF as BFF (Drive/Chat)
    participant Google as Google (OIDC)
    participant AccountSvc as Accounts Service
    participant DB as PostgreSQL
    participant Backend as Domain Service
    participant AuthZ as OpenFGA

    Note over SPA,DB: 【Phase 1】 OIDC 認証 & JIT プロビジョニング

    SPA->>BFF: GET /api/auth/login
    BFF-->>SPA: 302 Redirect (Google Auth URL + PKCE)
    SPA->>Google: ユーザーがGoogleでログイン・同意
    Google-->>SPA: 302 Redirect (auth_code)

    SPA->>BFF: GET /api/auth/callback?code=xxx
    BFF->>Google: auth_code を ID Token に交換 (PKCE検証)
    Google-->>BFF: ID Token (google_id, email, name)

    Note over BFF,DB: 【Phase 2】 内部セッション作成 & トークン発行 (鍵管理はBackend)
    
    BFF->>AccountSvc: [gRPC] LoginUser(google_id, profile, device_info)
    
    AccountSvc->>DB: UPSERT users (JITプロビジョニング)
    DB-->>AccountSvc: internal_user_id (例: usr_999)
    
    AccountSvc->>DB: INSERT sessions (マルチデバイス対応)
    DB-->>AccountSvc: session_id (例: sess_abc123)
    
    Note over AccountSvc: 秘密鍵を用いてJWTをMint (署名)<br/>1. Access Token (sub: usr_999, 15m)<br/>2. Refresh Token (sub: usr_999, sid: sess_abc, 7d)
    
    AccountSvc-->>BFF: [gRPC] {access_token, refresh_token}

    BFF-->>SPA: 302 Redirect to Dashboard <br/> + Set-Cookie (HttpOnly, Secure, SameSite=Strict)

    Note over SPA,AuthZ: 【Phase 3】 セキュアなAPIリクエスト (完全ステートレス)

    SPA->>BFF: GET /api/files (Cookie自動付与 + X-Requested-With)
    Note over BFF: Access Token の署名・期限を検証 (DBアクセスなし)
    
    BFF->>Backend: [gRPC] (metadata: x-user-id=usr_999)
    Backend->>AuthZ: Check user "usr_999" read resource "files"
    AuthZ-->>Backend: Allowed: true
    Backend-->>BFF: [gRPC] File Data
    BFF-->>SPA: HTTP 200 JSON Response

    Note over SPA,AccountSvc: 【Phase 4】 ステートフルなトークン・リフレッシュ

    SPA->>BFF: GET /api/files (Access Token 期限切れ)
    BFF-->>SPA: HTTP 401 Unauthorized

    SPA->>BFF: POST /api/auth/refresh (Refresh Cookie自動付与)
    Note over BFF: Refresh Tokenから session_id をデコード
    
    BFF->>AccountSvc: [gRPC] ValidateAndRefresh(session_id)
    AccountSvc->>DB: session_id が有効か確認 (強制ログアウト確認)
    
    alt セッションが無効（スマホ紛失等でRevoke済）
        DB-->>AccountSvc: Not Found / Revoked
        AccountSvc-->>BFF: [gRPC] Error: Invalid Session
        BFF-->>SPA: HTTP 403 (強制ログアウト処理へ)
    else セッションが有効
        DB-->>AccountSvc: Valid
        Note over AccountSvc: 新しい Access Token をMint
        AccountSvc-->>BFF: [gRPC] {new_access_token}
        BFF-->>SPA: HTTP 200 + Set-Cookie (New Access Token)
        SPA->>BFF: 失敗した GET /api/files を再実行
    end

```

---

## 5. 脅威モデリングとセキュリティ対策 (Security Mitigations)

本アーキテクチャは、Webアプリケーションにおける主要な攻撃ベクタに対して設計レベルで対策を講じています。

| 攻撃手法 | 本システムでの防御策 |
| --- | --- |
| **XSS (クロスサイトスクリプティング)** | すべてのトークンを `HttpOnly` 属性のCookieに格納。JavaScript（`document.cookie` 等）からの読み取りをブラウザレベルで完全にブロック。 |
| **CSRF (クロスサイトリクエストフォージェリ)** | Cookieに `SameSite=Strict` を付与し、別ドメインからのリクエスト送信を遮断。さらにBFFでカスタムヘッダー（`X-Requested-With` 等）を要求し、通常のForm送信等による攻撃を無効化。 |
| **OAuth 認可コードの横取り** | Google OIDCとの通信時に **PKCE (Proof Key for Code Exchange)** を必須化。 |
| **トークン漏洩時の被害拡大** | Access Tokenの寿命を15分と短く設定。 |
| **デバイス紛失時の不正アクセス** | Account ServiceとDBによるステートフルなRefresh Token管理。別の端末から対象セッション（`session_id`）をDB上でRevoke（無効化）することで、最長15分以内に強制ログアウトを完了させる。 |
| **内部ネットワークでのなりすまし** | gRPCポートは内部VPCのみに解放し、インターネットから直接 `AccountSvc` や `DomainSvc` を叩けないよう Cloudflare Tunnel と Docker Network で厳格に隔離。 |
| **SPAのトークンリフレッシュ競合** | クライアント側（Axios Interceptor等）で「Refresh Lock（排他制御キュー）」を実装し、複数API同時発火による無駄な401エラー連鎖を防止。 |