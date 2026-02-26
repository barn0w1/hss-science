# HSS Science Product Vision & Domain Model

## 1. Product Vision
**HSS Science** is a unified cloud platform operated under the domain `hss-science.org`.
本プラットフォームは、特定のコミュニティ向けに複数のWebサービスを統合的に提供するシステムである。

プラットフォーム全体の設計原則として、独自の概念やアーキテクチャの導入を避け、既存の標準的なプロトコル（OIDC, OAuth 2.0等）に厳密に準拠する。これにより、開発者およびAIエージェントの認知的負荷を下げ、予測可能でセキュアな実装を維持する。

## 2. Domain Architecture
本システムは、標準的な認証・認可の概念（OIDC / OAuth 2.0）に基づき、以下のドメインと役割（Role）によって構成される。

### 2.1. Identity Provider Domain (OP)
エコシステム全体の認証の根幹となるドメイン。

* **`accounts.hss-science.org` (OpenID Provider / OP)**
  * **Role:** Identity Provider (OP) / Authorization Server
  * **Detail:** エコシステム全体の認証（Authentication）を単一で引き受けるプロバイダー。OIDC標準に準拠し、エンドユーザーの認証、同意の取得、および各Relying Party (RP) に対するトークン（ID Token, Access Token）の発行を行う。

### 2.2. Service Domain (RP & RS)
OP（`accounts`）を信頼し、認証を委譲するクライアント（RP）およびリソースサーバー（RS）群。
これらはビジネス上の目的は異なるが、OIDCプロトコル上はすべて同等のクライアントとして振る舞う。

* **`myaccount.hss-science.org` (Account Management)**
  * **Role:** Relying Party (RP) / Resource Server (RS)
  * **Detail:** ユーザーが自身のアカウント情報やセキュリティ設定を管理するためのサービス。OPから発行されたトークンを用いて、ユーザープロフィール等の保護されたリソース（RS）にアクセス・更新する。
* **`chat.hss-science.org` (Real-time Communication)**
  * **Role:** Relying Party (RP) / Resource Server (RS)
  * **Detail:** リアルタイムコミュニケーションを提供するサービス。OPのトークンを検証し、保護されたリソース（メッセージ、チャンネル等）を提供する。
* **`drive.hss-science.org` (Storage & File Management)**
  * **Role:** Relying Party (RP) / Resource Server (RS)
  * **Detail:** ファイルの保存および管理を提供するサービス。実データへのアクセス制御とメタデータの管理を行う。

## 3. Ubiquitous Language
システム全体で一貫して使用するドメイン用語。標準的なプロトコルで使用される定義をそのまま採用し、独自解釈を排除する。

* **End-User**
  * システムを利用する主体。OPによって認証される対象。
* **OpenID Provider (OP)**
  * エンドユーザーを認証し、クレーム（Claims）を提供するサーバー（`accounts.hss-science.org`）。
* **Relying Party (RP)**
  * OPに対してエンドユーザーの認証を要求するOIDCクライアント（`myaccount`, `chat`, `drive` などの各アプリケーション）。
* **Resource Server (RS)**
  * アクセストークンを受理し、保護されたリソースを提供するサーバー（各アプリケーションのバックエンドAPI）。
* **Claims**
  * エンドユーザーに関する情報（識別子、プロファイル情報など）。ID Tokenに含まれる。

## 4. Core Principles for Design
新たにサービスや機能を設計する際は、以下の原則を遵守すること。

1. **Adherence to Standards**
   * 独自の認証フローやセッション管理を実装しない。すべてのサービス（RP）はOIDC / OAuth 2.0の標準仕様に従い、`accounts.hss-science.org` に認証を委譲すること。
2. **Centralized Identity, Decentralized Services**
   * 認証の責務はすべてOPに集約する。RPは独自のパスワード検証や認証ロジックを持たず、渡されたトークンの検証のみを行う。
3. **Single Source of Truth**
   * ユーザー情報（プロファイル、状態）のマスターデータはOP（またはそれに隣接する専用のRS）が保持する。各RPは必要なClaimsのみを受け取り、各サービス内でユーザー情報を重複して管理（マスター化）しないこと。