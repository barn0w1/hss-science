package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"golang.org/x/sync/errgroup"

	// Internal packages
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/handler"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/middleware"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/oauth"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/repository/postgres"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/config"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"

	// Generated Proto
	internalpb "github.com/barn0w1/hss-science/server/gen/internal_/accounts/v1"

	// Platform packages
	"github.com/barn0w1/hss-science/server/platform/logger"
	"github.com/barn0w1/hss-science/server/platform/server"
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

	// 3. Connect to Database
	db, err := sqlx.Connect("pgx", cfg.DB.DSN())
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	slog.Info("Database connected", "host", cfg.DB.Host)

	// 4. Initialize Dependency Injection (DI)

	// Adapter Layer (Infra)
	userRepo := postgres.NewUserRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	authCodeRepo := postgres.NewAuthCodeRepository(db)
	oauthProvider := oauth.NewDiscordProvider(cfg)

	// Usecase Layer (Business Logic)
	authUsecase := usecase.NewAuthUsecase(cfg, userRepo, sessionRepo, authCodeRepo, oauthProvider)

	// Middleware Layer (gRPC only)
	authMiddleware := middleware.NewAuthMiddleware()

	// Handler Layer (Interface Adapter)
	internalHandler := handler.NewInternalHandler(authUsecase)
	publicHandler := handler.NewPublicHandler(authUsecase, cfg)

	// 5. Setup gRPC Server (Internal)
	srv := server.New(cfg.AppConfig, authMiddleware.UnaryServerInterceptor())
	internalpb.RegisterAccountsInternalServiceServer(srv.GrpcServer(), internalHandler)

	// 6. Setup HTTP Server (Public)
	publicMux := http.NewServeMux()
	publicMux.HandleFunc("/v1/authorize", publicHandler.Authorize)
	publicMux.HandleFunc("/v1/oauth/callback", publicHandler.OAuthCallback)

	httpServer := &http.Server{
		Addr:    ":" + fmt.Sprint(cfg.HTTPPort),
		Handler: publicMux,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	// gRPC server
	g.Go(func() error {
		addr := ":" + fmt.Sprint(cfg.GRPCPort)
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		slog.Info("Starting internal gRPC server", "addr", addr)
		go func() {
			<-ctx.Done()
			srv.GrpcServer().GracefulStop()
		}()
		return srv.GrpcServer().Serve(lis)
	})

	// HTTP server
	g.Go(func() error {
		slog.Info("Starting public HTTP server", "addr", httpServer.Addr)
		go func() {
			<-ctx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			httpServer.Shutdown(shutdownCtx)
		}()
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		slog.Error("Server exited", "error", err)
		os.Exit(1)
	}
}
