accounts serviceは、SSOを提供する。

accounts serviceは、nativeのHTTP serverと、grpc serverを持つ。

HTTP serverは、ブラウザからのリクエストを受け、主に、redirectやcookieの発行を行う。
grpc serverは、他サービスからの認証コード検証リクエストを受け、認証コードの検証を行う。