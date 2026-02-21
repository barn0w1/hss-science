# Accounts (Auth) Service

本サービスは、`hss-science` システム全体における統合認証基盤（Identity Provider: IdP）です。
外部のOAuthプロバイダー（Discord等）を信頼できる親プロバイダーとして利用し、システム内の各マイクロサービス（Drive, Chat等）に対して、標準的な OAuth 2.0 の「認可コードフロー（Authorization Code Flow）」に準拠したSSO（シングルサインオン）を提供します。



## 1. BFFとgRPCの厳格な責務分離 (Crucial Rule)
本サービスにおいて、最も重要なのはBFF（Web層）とgRPC（ドメイン層）の役割を明確に分けることです。AIエージェントはこの境界を絶対に越えない設計を行ってください。

* **Auth BFF (Gateway) の責務:**
  * クライアントの差異やHTTPプロトコルの都合をすべて吸収する。
  * HTTPリダイレクトの発行、Cookie（OAuthの `state` や `nonce` 等の一時的な検証用）の読み書き。
  * Discordからのコールバック（HTTP GET）を受け取り、必要なパラメータを抽出して裏側のgRPCへ渡す。
* **Auth gRPC (Microservice) の責務:**
  * 本質的な認証・認可のビジネスロジックのみを実行する。
  * OAuthの `state` の生成・検証、Discord APIとの通信（トークン交換・ユーザー情報取得）、DBへのユーザーのUpsert、システム内部用 `auth_code` の生成と検証。
  * **※注意:** gRPC層はHTTP、Cookie、リダイレクトの概念を一切知りません。

## 2. SSO認証フローの概念 (OAuth 2.0 Authorization Code Flow準拠)

Driveサービスにログインする場合を例とした、全体的なシーケンスの概念です。

1. **[Drive BFF] ログイン開始:** セッションがない場合、Drive BFFは Auth BFF のログインエンドポイントへリダイレクト。
2. **[Auth BFF -> Discord] 外部認証:** Auth BFFは、gRPCからセキュアなURLとstateを取得し、Discordへリダイレクト。
3. **[Discord -> Auth BFF -> Auth gRPC] コールバック:** DiscordからAuth BFFへ戻る。BFFはパラメータをgRPCへ流す。gRPCはDiscordと通信し、ユーザーを同定・DB登録する。
4. **[Auth gRPC] 認可コードの発行:** Auth gRPCは、短命な内部用 `auth_code` を生成してBFFに返す。
5. **[Auth BFF -> Drive BFF] リダイレクトバック:** Auth BFFは、指定された `redirect_uri` へ `auth_code` を付与してリダイレクト。
6. **[Drive BFF -> Auth gRPC] トークン交換:** Drive BFFは `auth_code` を用いて、裏側から Auth gRPC (`ExchangeToken`) を呼び出し、内部ユーザー情報を取得。
7. **[Drive BFF] セッション確立:** Drive BFFが独自のセキュアなCookie（セッション）を発行し、ログイン完了。

## 3. gRPC API 設計のベースライン (Propose the Best)

以下は概念的なAPIのリストです。**AIエージェントはこれに縛られず、上記の責務分離を満たす上で最も美しく、拡張性の高い `.proto` の設計を自ら考え、提案してください。**

* `GetAuthURL`: DiscordのOAuthログインURLと状態（state）を生成する。
* `HandleProviderCallback`: Discordの認証結果（code/state）を受け取り、検証とユーザー登録を行い、内部の `auth_code` を返す。
* `ExchangeToken`: 各サービスのBFFが `auth_code` を提示した際に、それを検証して内部ユーザー情報を返す。
* `GetUser`: ユーザー情報を取得する。

## 4. データモデルのベースライン (Propose the Best)

以下は想定される概念的なエンティティです。**後方互換性は一切気にせず、将来的な複数IdP（GoogleやGitHubなど）の追加も見据えた、AIが考える最強のテーブル設計（PostgreSQL）を提案してください。**

* `users`: システムの統合ユーザー情報。
* `user_identities`: 外部プロバイダーのIDと内部ユーザーIDの紐付け。
* `auth_codes`: SSOのための短命な認可コード。