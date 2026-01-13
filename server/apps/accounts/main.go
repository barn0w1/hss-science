package main

import (
	"context"
	"log/slog"
	"os"

	// Generated Code
	accountsv1 "github.com/barn0w1/hss-science/server/gen/public/accounts/v1"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"

	// Platform
	"github.com/barn0w1/hss-science/server/platform/config"
	"github.com/barn0w1/hss-science/server/platform/logger"
	"github.com/barn0w1/hss-science/server/platform/server"

	"google.golang.org/grpc"
)

// Implementation (本来は internal/handler/grpc/server.go などに書く)
type authServer struct {
	accountsv1.UnimplementedAccountsServiceServer
}

func main() {
	// 1. Config Load
	cfg := config.Load()

	// 2. Logger Setup (★ここを修正)
	// Configパッケージの値を、LoggerパッケージのConfig型に詰め替えて渡す
	logger.Setup(logger.Config{
		LogLevel:  cfg.LogLevel,
		LogFormat: cfg.LogFormat,
	})

	slog.Info("Starting Accounts Service", "env", cfg.Env)

	// 3. Server Config
	srvCfg := server.Config{
		GRPCPort: 9090,
		HTTPPort: 8080,
		IsDev:    cfg.Env == "dev",
	}

	// 4. Run Server
	// platform/server.Run はブロック呼び出しなので、
	// 設定とハンドラ登録関数を渡して起動します。
	err := server.Run(
		context.Background(),
		srvCfg,
		// gRPC Service Registerer
		func(s *grpc.Server) {
			accountsv1.RegisterAccountsServiceServer(s, &authServer{})
		},
		// Gateway Handler Registerer
		func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
			return accountsv1.RegisterAccountsServiceHandler(ctx, mux, conn)
		},
	)

	if err != nil {
		slog.Error("Server stopped with error", "error", err)
		os.Exit(1)
	}
}
