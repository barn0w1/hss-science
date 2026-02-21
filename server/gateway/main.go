package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	gwconfig "github.com/barn0w1/hss-science/server/gateway/config"
	"github.com/barn0w1/hss-science/server/gateway/handler"
	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("gateway: %v", err)
	}
}

func run() error {
	cfg, err := gwconfig.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Connect to the Accounts gRPC service.
	accountsConn, err := grpc.NewClient(
		cfg.AccountsGRPCAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connect to accounts service: %w", err)
	}
	defer accountsConn.Close()

	accountsClient := accountsv1.NewAccountsServiceClient(accountsConn)

	// HTTP Router
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Auth routes
	authHandler := handler.NewAuthHandler(accountsClient)
	authHandler.RegisterRoutes(mux)

	server := &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		log.Printf("gateway HTTP server listening on :%s", cfg.HTTPPort)
		errCh <- server.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Printf("received signal %v, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
