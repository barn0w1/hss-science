package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/config"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/discord"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/postgres"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain"
	grpctransport "github.com/barn0w1/hss-science/server/services/accounts/internal/transport/grpc"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("accounts service: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Database
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	// Repositories
	userRepo := postgres.NewUserRepo(db)
	authCodeRepo := postgres.NewAuthCodeRepo(db)
	stateRepo := postgres.NewOAuthStateRepo(db)

	// OAuth Providers
	discordProvider := discord.NewProvider(
		cfg.DiscordClientID,
		cfg.DiscordClientSecret,
		cfg.DiscordRedirectURL,
	)

	// Usecase
	authUsecase := usecase.NewAuthUsecase(
		userRepo,
		authCodeRepo,
		stateRepo,
		[]domain.OAuthProvider{discordProvider},
	)

	// gRPC Server
	grpcServer := grpc.NewServer()
	handler := grpctransport.NewHandler(authUsecase)
	accountsv1.RegisterAccountsServiceServer(grpcServer, handler)
	reflection.Register(grpcServer)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	// Graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		log.Printf("accounts gRPC server listening on :%s", cfg.GRPCPort)
		errCh <- grpcServer.Serve(lis)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("received signal %v, shutting down", sig)
		grpcServer.GracefulStop()
		return nil
	case err := <-errCh:
		return err
	}
}
