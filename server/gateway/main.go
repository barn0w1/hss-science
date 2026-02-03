package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	gen "github.com/barn0w1/hss-science/server/gateway/gen/accounts/v1"
	"github.com/barn0w1/hss-science/server/gateway/handler"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	myHandler := &handler.AccountsHandler{
		// grpcClient: ...
	}

	// 2. StrictHandler (型安全ラッパー) に変換
	//    第2引数の nil は、個別のメソッドに挟むミドルウェア用ですが、通常は nil でOK
	strictHandler := gen.NewStrictHandler(myHandler, nil)

	// 3. ルーティング登録
	//    BaseURL ("/api/accounts/v1") を指定してグループ化
	r.Route("/api/accounts/v1", func(r chi.Router) {
		// 生成された HandlerFromMux を使う
		gen.HandlerFromMux(strictHandler, r)
	})

	// 4. サーバー起動
	http.ListenAndServe(":3000", r)
}
