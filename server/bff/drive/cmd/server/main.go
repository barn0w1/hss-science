package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	drive "github.com/barn0w1/hss-science/server/bff/drive"
	driveapi "github.com/barn0w1/hss-science/server/bff/gen/drive/v1"
	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "drive-bff: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := drive.ParseConfig()
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	log := newLogger(cfg.Env, cfg.LogLevel)
	log = log.With("service", "drive-bff")

	// Initialize OIDC provider (performs Google discovery at startup).
	oidcProvider, err := drive.NewOIDCProvider(context.Background(), cfg)
	if err != nil {
		return fmt.Errorf("init oidc provider: %w", err)
	}
	log.Info("oidc provider initialized")

	// Dial gRPC connection to Accounts Service.
	conn, err := grpc.NewClient(cfg.AccountsGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial accounts service: %w", err)
	}
	defer conn.Close()
	accountsClient := accountsv1.NewAccountsServiceClient(conn)

	// Create server implementing ServerInterface.
	srv := drive.NewServer(cfg, log, oidcProvider, accountsClient)

	// Setup chi router with middleware.
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(drive.LoggingMiddleware(log))
	r.Use(middleware.Recoverer)

	// Register generated routes.
	driveapi.HandlerFromMux(srv, r)

	// Create HTTP server.
	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Info("shutting down HTTP server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	log.Info("HTTP server listening", "port", cfg.Port)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("listen: %w", err)
	}
	return nil
}

func newLogger(env, level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}
	if env == "development" {
		return slog.New(slog.NewTextHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, opts))
}
