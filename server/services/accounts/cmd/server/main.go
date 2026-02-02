package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
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
	if err := run(); err != nil {
		slog.Error("Server exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Load()

	logger.Setup(logger.Config{
		ServiceName: cfg.ServiceName,
		LogLevel:    cfg.LogLevel,
		LogFormat:   cfg.LogFormat,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := connectDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	userRepo := postgres.NewUserRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	authCodeRepo := postgres.NewAuthCodeRepository(db)
	oauthProvider := oauth.NewDiscordProvider(cfg)

	authUsecase := usecase.NewAuthUsecase(cfg, userRepo, sessionRepo, authCodeRepo, oauthProvider)

	authMiddleware := middleware.NewAuthMiddleware()
	internalHandler := handler.NewInternalHandler(authUsecase)
	publicHandler := handler.NewPublicHandler(authUsecase, cfg)

	srv := server.New(cfg.AppConfig, authMiddleware.UnaryServerInterceptor())
	internalpb.RegisterAccountsInternalServiceServer(srv.GrpcServer(), internalHandler)

	httpServer := newHTTPServer(cfg, publicHandler)

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return runGRPC(ctx, srv, cfg.GRPCPort)
	})
	g.Go(func() error {
		return runHTTP(ctx, httpServer, cfg.HTTPShutdownTimeoutSec)
	})

	return g.Wait()
}

func connectDB(ctx context.Context, cfg *config.Config) (*sqlx.DB, error) {
	connectCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.DBConnectTimeoutSec)*time.Second)
	defer cancel()

	db, err := sqlx.ConnectContext(connectCtx, "pgx", cfg.DB.DSN())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.DBConnMaxLifetimeMin) * time.Minute)
	db.SetConnMaxIdleTime(time.Duration(cfg.DBConnMaxIdleTimeMin) * time.Minute)

	slog.Info("Database connected", "host", cfg.DB.Host)
	return db, nil
}

func newHTTPServer(cfg *config.Config, publicHandler *handler.PublicHandler) *http.Server {
	publicMux := http.NewServeMux()
	publicMux.HandleFunc(handler.AuthorizePath, publicHandler.Authorize)
	publicMux.HandleFunc(handler.OAuthCallbackPath, publicHandler.OAuthCallback)

	return &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.HTTPPort),
		Handler:           publicMux,
		ReadTimeout:       time.Duration(cfg.HTTPReadTimeoutSec) * time.Second,
		WriteTimeout:      time.Duration(cfg.HTTPWriteTimeoutSec) * time.Second,
		IdleTimeout:       time.Duration(cfg.HTTPIdleTimeoutSec) * time.Second,
		ReadHeaderTimeout: time.Duration(cfg.HTTPReadHeaderTimeoutSec) * time.Second,
	}
}

func runGRPC(ctx context.Context, srv *server.Server, port int) error {
	addr := ":" + strconv.Itoa(port)
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
}

func runHTTP(ctx context.Context, httpServer *http.Server, shutdownTimeoutSec int) error {
	slog.Info("Starting public HTTP server", "addr", httpServer.Addr)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Duration(shutdownTimeoutSec)*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTP server shutdown error", "error", err)
		}
	}()
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
