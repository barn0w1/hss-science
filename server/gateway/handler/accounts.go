package handler

import (
	"context"
	"fmt"

	gen "github.com/barn0w1/hss-science/server/gateway/gen/accounts/v1"
	// 必要なら gRPC の定義も import
	// pb "github.com/barn0w1/hss-science/proto/accounts/v1"
)

// ハンドラ構造体 (ここに gRPC クライアントなどを持たせる)
type AccountsHandler struct {
	// grpcClient pb.AccountsServiceClient
}

// コンパイル時のインターフェース適合チェック
var _ gen.StrictServerInterface = (*AccountsHandler)(nil)

// GET /authorize
func (h *AccountsHandler) StartOAuthFlow(ctx context.Context, request gen.StartOAuthFlowRequestObject) (gen.StartOAuthFlowResponseObject, error) {
	// 1. リクエストパラメータは自動でパースされている
	audience := request.Params.Audience
	redirectURI := request.Params.RedirectUri

	fmt.Printf("Auth requested for: %s, redirect to: %s\n", audience, redirectURI)

	// 2. ロジック (Cookieチェックなど)
	// ...

	// 3. レスポンスを返す (302 Redirect)
	// 生成されたコードにある「StartOAuthFlow302Response」を使います
	targetURL := "https://discord.com/api/oauth2/authorize?..."

	return gen.StartOAuthFlow302Response{
		Headers: gen.StartOAuthFlow302ResponseHeaders{
			Location: targetURL,
		},
	}, nil
}

// GET /oauth/callback
func (h *AccountsHandler) OAuthCallback(ctx context.Context, request gen.OAuthCallbackRequestObject) (gen.OAuthCallbackResponseObject, error) {
	code := request.Params.Code
	state := request.Params.State

	fmt.Printf("Callback received. Code: %s, State: %s\n", code, state)

	// 1. gRPC を呼んで検証する
	// verifyResp, err := h.grpcClient.VerifyAuthCode(ctx, ...)

	// 2. クライアントへ戻す (302 Redirect + Set-Cookie)
	return gen.OAuthCallback302Response{
		Headers: gen.OAuthCallback302ResponseHeaders{
			Location:  "https://drive.hss-science.org/callback?code=internal_code",
			SetCookie: "accounts_session=xyz123; Path=/; HttpOnly; Secure",
		},
	}, nil
}
