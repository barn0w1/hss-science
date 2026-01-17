package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	// Internal packages
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/handler"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/oauth"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/repository/postgres"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/config"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"

	// Generated Proto
	pb "github.com/barn0w1/hss-science/server/gen/public/accounts/v1"

	// Platform packages
	"github.com/barn0w1/hss-science/server/platform/logger"
	"github.com/barn0w1/hss-science/server/platform/server"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
)

func main() {
	// 1. Load Config
	cfg := config.Load()

	// 2. Setup Logger
	logger.Setup(logger.Config{
		ServiceName: cfg.ServiceName,
		LogLevel:    cfg.LogLevel,
		LogFormat:   cfg.LogFormat,
	})

	// 3. Connect to Database (using sqlx + pgx driver)
	// DSN() string is provided by platform config
	db, err := sqlx.Connect("pgx", cfg.DB.DSN())
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// DB Connection settings (optional but recommended)
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute) // 5 minutes

	slog.Info("Database connected", "host", cfg.DB.Host)

	// 4. Initialize Dependency Injection (DI)

	// Adapter Layer (Infra)
	userRepo := postgres.NewUserRepository(db)
	tokenRepo := postgres.NewTokenRepository(db)
	oauthProvider := oauth.NewDiscordProvider(cfg)

	// Usecase Layer (Business Logic)
	authUsecase := usecase.NewAuthUsecase(cfg, userRepo, tokenRepo, oauthProvider)

	// Handler Layer (Interface Adapter)
	authHandler := handler.NewAuthHandler(authUsecase)

	// 5. Setup Platform Server
	srv := server.New(cfg.AppConfig)

	// 6. Register gRPC Service
	pb.RegisterAccountsServiceServer(srv.GrpcServer(), authHandler)

	// 7. Register HTTP Gateway
	srv.RegisterGateway(func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error {
		return pb.RegisterAccountsServiceHandlerFromEndpoint(ctx, mux, endpoint, opts)
	})

	// 8. Start Server
	if err := srv.Run(); err != nil {
		slog.Error("Server exited", "error", err)
		os.Exit(1)
	}
}
