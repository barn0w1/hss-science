package app

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

	internalpb "github.com/barn0w1/hss-science/server/gen/internal_/accounts/v1"
	"github.com/barn0w1/hss-science/server/platform/logger"
	"github.com/barn0w1/hss-science/server/platform/server"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/oauth"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/repository/postgres"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/transport"
	grpctransport "github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/transport/grpc"
	httptransport "github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/transport/http"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/config"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"
)

func Run() error {
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

	authMiddleware := grpctransport.NewAuthMiddleware()
	internalHandler := grpctransport.NewInternalHandler(authUsecase)
	publicHandler := httptransport.NewPublicHandler(authUsecase, cfg)

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

func newHTTPServer(cfg *config.Config, publicHandler *httptransport.PublicHandler) *http.Server {
	publicMux := http.NewServeMux()
	publicMux.HandleFunc(transport.AuthorizePath, publicHandler.Authorize)
	publicMux.HandleFunc(transport.OAuthCallbackPath, publicHandler.OAuthCallback)

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
