// Command server starts the Accounts gRPC service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/barn0w1/hss-science/server/internal/logging"
	"github.com/barn0w1/hss-science/server/services/accounts/provider"
	"github.com/barn0w1/hss-science/server/services/accounts/service"
	"github.com/barn0w1/hss-science/server/services/accounts/store"
	"github.com/barn0w1/hss-science/server/services/accounts/transport"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Configuration from environment (fail-fast on missing required vars).
	env := envOrDefault("ENV", "production")
	port := envOrDefault("PORT", "50051")
	logLevel := envOrDefault("LOG_LEVEL", "INFO")
	databaseURL := requiredEnv("DATABASE_URL")
	discordClientID := requiredEnv("DISCORD_CLIENT_ID")
	discordClientSecret := requiredEnv("DISCORD_CLIENT_SECRET")
	discordRedirectURL := requiredEnv("DISCORD_REDIRECT_URL")

	// Structured logging.
	logger := logging.Setup(env, logLevel, "accounts-grpc")

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Database connection pool.
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	logger.Info("connected to database")

	// Repositories.
	userStore := store.NewUserStore(pool)
	stateStore := store.NewStateStore(pool)
	authCodeStore := store.NewAuthCodeStore(pool)

	// OAuth providers.
	discord := provider.NewDiscord(provider.DiscordConfig{
		ClientID:     discordClientID,
		ClientSecret: discordClientSecret,
		RedirectURL:  discordRedirectURL,
	})
	providers := map[string]provider.OAuthProvider{
		discord.Name(): discord,
	}

	// Service layer.
	svc := service.New(userStore, stateStore, authCodeStore, providers, 10*time.Minute, 5*time.Minute)

	// gRPC server with logging interceptor.
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(logging.UnaryServerInterceptor(logger)),
	)
	accountsv1.RegisterAccountsServiceServer(grpcServer, transport.NewServer(svc))
	reflection.Register(grpcServer)

	addr := ":" + port
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}

	g, gctx := errgroup.WithContext(ctx)

	// Start gRPC server.
	g.Go(func() error {
		logger.Info("gRPC server started", "addr", addr)
		return grpcServer.Serve(lis)
	})

	// Graceful shutdown.
	g.Go(func() error {
		<-gctx.Done()
		logger.Info("shutting down gRPC server")
		grpcServer.GracefulStop()
		return nil
	})

	// Periodic cleanup of expired states and auth codes.
	g.Go(func() error {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-gctx.Done():
				return nil
			case <-ticker.C:
				if n, err := stateStore.DeleteExpired(context.Background()); err != nil {
					logger.Error("cleanup expired states", "error", err)
				} else if n > 0 {
					logger.Info("cleaned up expired states", "count", n)
				}
				if n, err := authCodeStore.DeleteExpired(context.Background()); err != nil {
					logger.Error("cleanup expired auth codes", "error", err)
				} else if n > 0 {
					logger.Info("cleaned up expired auth codes", "count", n)
				}
			}
		}
	})

	return g.Wait()
}

func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func requiredEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "required environment variable %s is not set\n", key)
		os.Exit(1)
	}
	return v
}
