package main

import (
	"context"
	"fmt"
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

// authServer implements the AccountsService gRPC interface.
// Ideally, this should be moved to internal/handler/grpc/server.go in the future.
type authServer struct {
	accountsv1.UnimplementedAccountsServiceServer
}

// GetLoginUrl handles the request to generate a Discord OAuth URL.
// It logs the request and returns a constructed URL.
func (s *authServer) GetLoginUrl(ctx context.Context, req *accountsv1.GetLoginUrlRequest) (*accountsv1.GetLoginUrlResponse, error) {
	slog.Info("GetLoginUrl called", "redirect_to", req.RedirectTo)

	// TODO: Replace with actual Discord Client ID and logic from the service layer.
	mockURL := fmt.Sprintf(
		"https://discord.com/oauth2/authorize?client_id=FAKE_CLIENT_ID&redirect_uri=%s&response_type=code&scope=identify",
		req.RedirectTo,
	)

	return &accountsv1.GetLoginUrlResponse{
		Url: mockURL,
	}, nil
}

func main() {
	// 1. Load Configuration
	cfg := config.Load()

	// 2. Setup Logger
	logger.Setup(logger.Config{
		LogLevel:  cfg.LogLevel,
		LogFormat: cfg.LogFormat,
	})

	slog.Info("Starting Accounts Service", "env", cfg.Env)

	// 3. Configure Server
	srvCfg := server.Config{
		GRPCPort: 9090,
		HTTPPort: 8080,
		IsDev:    cfg.Env == "dev",
	}

	// 4. Run Server (Blocking)
	err := server.Run(
		context.Background(),
		srvCfg,
		// Register gRPC Service
		func(s *grpc.Server) {
			accountsv1.RegisterAccountsServiceServer(s, &authServer{})
		},
		// Register HTTP Gateway Handler
		func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
			return accountsv1.RegisterAccountsServiceHandler(ctx, mux, conn)
		},
	)

	if err != nil {
		slog.Error("Server stopped with error", "error", err)
		os.Exit(1)
	}
}
