package main

import (
	"context"
	"log"
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
	"github.com/barn0w1/hss-science/server/services/accounts/provider"
	"github.com/barn0w1/hss-science/server/services/accounts/service"
	"github.com/barn0w1/hss-science/server/services/accounts/store"
	"github.com/barn0w1/hss-science/server/services/accounts/transport"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Configuration from environment.
	grpcAddr := envOrDefault("GRPC_ADDR", ":50051")
	databaseURL := requiredEnv("DATABASE_URL")
	discordClientID := requiredEnv("DISCORD_CLIENT_ID")
	discordClientSecret := requiredEnv("DISCORD_CLIENT_SECRET")
	discordRedirectURL := requiredEnv("DISCORD_REDIRECT_URL")

	// Database connection pool.
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return err
	}
	log.Println("connected to database")

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

	// gRPC server.
	grpcServer := grpc.NewServer()
	accountsv1.RegisterAccountsServiceServer(grpcServer, transport.NewServer(svc))
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return err
	}

	g, gctx := errgroup.WithContext(ctx)

	// Start gRPC server.
	g.Go(func() error {
		log.Printf("gRPC server listening on %s", grpcAddr)
		return grpcServer.Serve(lis)
	})

	// Graceful shutdown.
	g.Go(func() error {
		<-gctx.Done()
		log.Println("shutting down gRPC server...")
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
					log.Printf("cleanup expired states: %v", err)
				} else if n > 0 {
					log.Printf("cleaned up %d expired states", n)
				}
				if n, err := authCodeStore.DeleteExpired(context.Background()); err != nil {
					log.Printf("cleanup expired auth codes: %v", err)
				} else if n > 0 {
					log.Printf("cleaned up %d expired auth codes", n)
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
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}
