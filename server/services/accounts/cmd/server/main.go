package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	accounts "github.com/barn0w1/hss-science/server/services/accounts"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"google.golang.org/grpc"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "accounts: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := accounts.ParseConfig()
	if err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	log := newLogger(cfg.Env, cfg.LogLevel)
	log = log.With("service", "accounts")

	db, err := sqlx.Connect("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer db.Close()

	jwtMinter, err := accounts.NewJWTMinter(cfg.JWTPrivateKeyPath, cfg.JWTIssuer, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	if err != nil {
		return fmt.Errorf("init jwt minter: %w", err)
	}

	repo := accounts.NewPGRepository(db)
	svc := accounts.NewService(repo, jwtMinter, log)

	grpcServer := grpc.NewServer()
	accountsv1.RegisterAccountsServiceServer(grpcServer, svc)

	lis, err := net.Listen("tcp", ":"+cfg.Port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	// Graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Info("shutting down gRPC server")
		grpcServer.GracefulStop()
	}()

	log.Info("gRPC server listening", "port", cfg.Port)
	return grpcServer.Serve(lis)
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
