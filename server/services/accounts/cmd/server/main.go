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

	// App
	"github.com/barn0w1/hss-science/server/services/accounts/internal/handler"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/repository"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/service"

	// DB
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"google.golang.org/grpc"
)

func main() {
	// 1. Load Configuration
	cfg := config.LoadBase()

	// 2. Setup Logger
	logger.Setup(logger.Config{
		LogLevel:  cfg.LogLevel,
		LogFormat: cfg.LogFormat,
	})

	slog.Info("Starting Accounts Service", "env", cfg.Env)

	// 3. Connect to Database
	dbDSN := os.Getenv("DATABASE_URL")
	if dbDSN == "" {
		// Fallback for development if not set
		dbDSN = "postgres://user:password@localhost:5432/accounts?sslmode=disable"
	}

	db, err := sqlx.Connect("postgres", dbDSN)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// 4. Initialize Domain Layers
	repo := repository.NewPostgresRepo(db)
	svc := service.NewAuthService(repo)
	h := handler.NewGrpcServer(svc)

	// 5. Configure Server
	srvCfg := server.Config{
		GRPCPort: 9090,
		HTTPPort: 8080,
		IsDev:    cfg.Env == "dev",
	}

	// 6. Run Server (Blocking)
	err = server.Run(
		context.Background(),
		srvCfg,
		// Register gRPC Service
		func(s *grpc.Server) {
			accountsv1.RegisterAccountsServiceServer(s, h)
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
