package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/barn0w1/hss-science/server/platform/server/gateway"
	platform_grpc "github.com/barn0w1/hss-science/server/platform/server/grpc"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

// Config holds the configuration for starting the server set.
type Config struct {
	GRPCPort int
	HTTPPort int // Optional: If 0, Gateway is disabled (gRPC only mode).
}

// Run starts the gRPC server and optional HTTP gateway.
// It manages lifecycle, listener, and graceful shutdown.
func Run(
	ctx context.Context,
	cfg Config,
	registerGRPC func(*grpc.Server),
	// registerGateway registers handlers to the mux.
	// Users should dial the local gRPC server within this function if needed,
	// or use generated Register...HandlerFromEndpoint with DialOptions.
	registerGateway func(context.Context, *runtime.ServeMux) error,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// ---------------------------
	// 1. Setup gRPC Server
	// ---------------------------
	grpcServer := platform_grpc.New()
	registerGRPC(grpcServer)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen on grpc port %d: %w", cfg.GRPCPort, err)
	}

	// ---------------------------
	// 2. Orchestration (errgroup)
	// ---------------------------
	g, ctx := errgroup.WithContext(ctx)

	// A. Start gRPC Server
	g.Go(func() error {
		slog.Info("Starting gRPC server", "port", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			return err
		}
		return nil
	})

	// B. Start Gateway Server (If configured)
	if cfg.HTTPPort > 0 && registerGateway != nil {
		mux := gateway.NewMux()

		// ユーザー登録関数を実行 (ここで Mux に Handler を登録する)
		if err := registerGateway(ctx, mux); err != nil {
			return fmt.Errorf("failed to register gateway: %w", err)
		}

		httpServer := &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
			Handler: gateway.WithCORS(mux),
		}

		g.Go(func() error {
			slog.Info("Starting HTTP gateway", "port", cfg.HTTPPort)
			if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		})

		// HTTP Shutdown Handler
		g.Go(func() error {
			<-ctx.Done() // Wait for cancel
			slog.Info("Shutting down HTTP gateway...")
			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelShutdown()
			return httpServer.Shutdown(shutdownCtx)
		})
	}

	// C. Signal Listener (Graceful Shutdown)
	g.Go(func() error {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

		select {
		case <-quit:
			slog.Info("Signal received, initiating graceful shutdown...")
			cancel() // 他のすべてのgoroutineをキャンセル
		case <-ctx.Done():
		}

		slog.Info("Stopping gRPC server...")
		grpcServer.GracefulStop()
		return nil
	})

	return g.Wait()
}
