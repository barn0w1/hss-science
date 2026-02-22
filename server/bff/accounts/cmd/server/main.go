package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	oapi "github.com/barn0w1/hss-science/server/bff/gen/accounts/v1"

	"github.com/barn0w1/hss-science/server/bff/accounts"
	"github.com/barn0w1/hss-science/server/bff/internal/session"

	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Load configuration.
	cfg := accounts.LoadConfig()

	// Connect to gRPC service.
	conn, err := grpc.NewClient(cfg.GRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	grpcClient := accountsv1.NewAccountsServiceClient(conn)

	// Session manager.
	sess := session.New(cfg.SessionHashKey, cfg.SessionBlockKey, cfg.SessionMaxAge, cfg.SessionSecure)

	// BFF handler.
	handler := accounts.NewHandler(grpcClient, sess, cfg)

	// Wire up the chi router with oapi-codegen strict server.
	strictHandler := oapi.NewStrictHandler(handler, []oapi.StrictMiddlewareFunc{
		accounts.RequestInjector,
	})

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	oapi.HandlerFromMux(strictHandler, r)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		log.Printf("BFF listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-gctx.Done()
		log.Println("shutting down BFF...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})

	return g.Wait()
}
