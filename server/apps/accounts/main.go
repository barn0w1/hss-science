package main

import (
	"context"
	"log/slog"
	"os"

	// Generated Code
	accountsv1 "github.com/barn0w1/hss-science/server/gen/public/accounts/v1"

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

	// 3. Server Manager Setup
	mgr := server.New(server.Config{
		GRPCPort: 9090,
		HTTPPort: 8080,
		IsDev:    cfg.Env == "dev",
	})

	// 4. Register gRPC
	mgr.AddGRPC(func(s *grpc.Server) {
		accountsv1.RegisterAccountsServiceServer(s, &authServer{})
	})

	// 5. Register Gateway
	mgr.AddGateway(accountsv1.RegisterAccountsServiceHandler)

	// 6. Run
	if err := mgr.Run(context.Background()); err != nil {
		slog.Error("Server stopped with error", "error", err)
		os.Exit(1)
	}
}
