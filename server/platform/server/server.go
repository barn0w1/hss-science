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

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config holds the server startup configuration.
type Config struct {
	GRPCPort int
	HTTPPort int  // If 0, the HTTP Gateway will not start.
	IsDev    bool // Controls CORS and logging verbosity.
}

// Run starts the gRPC server and optional HTTP Gateway.
// It blocks until a signal (SIGINT/SIGTERM) is received.
func Run(
	ctx context.Context,
	cfg Config,
	registerGRPC func(*grpc.Server),
	registerGateway func(context.Context, *runtime.ServeMux, *grpc.ClientConn) error,
) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 1. Initialize gRPC Server
	grpcServer := newGRPCServer()
	registerGRPC(grpcServer)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen on grpc port %d: %w", cfg.GRPCPort, err)
	}

	// 2. Orchestration
	g, ctx := errgroup.WithContext(ctx)

	// --- Start gRPC ---
	g.Go(func() error {
		slog.Info("🚀 gRPC Server started", "port", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			return err
		}
		return nil
	})

	// --- Start Gateway (Optional) ---
	if cfg.HTTPPort > 0 && registerGateway != nil {
		// Create a client connection to the local gRPC server
		conn, err := grpc.NewClient(
			fmt.Sprintf("localhost:%d", cfg.GRPCPort),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return fmt.Errorf("failed to dial internal grpc: %w", err)
		}
		defer conn.Close()

		mux := newGatewayMux()
		if err := registerGateway(ctx, mux, conn); err != nil {
			return fmt.Errorf("failed to register gateway handler: %w", err)
		}

		httpServer := &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
			Handler: withCORS(mux, cfg.IsDev),
		}

		// Gateway Listener
		g.Go(func() error {
			slog.Info("🌍 HTTP Gateway started", "port", cfg.HTTPPort)
			if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				return err
			}
			return nil
		})

		// Gateway Shutdown Handler
		g.Go(func() error {
			<-ctx.Done()
			slog.Info("Shutting down HTTP Gateway...")
			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancelShutdown()
			return httpServer.Shutdown(shutdownCtx)
		})
	}

	// 3. Signal Listener (Graceful Shutdown)
	g.Go(func() error {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

		select {
		case s := <-quit:
			slog.Info("Signal received, initiating shutdown...", "signal", s)
			cancel()
		case <-ctx.Done():
		}

		slog.Info("Stopping gRPC Server...")
		grpcServer.GracefulStop()
		return nil
	})

	return g.Wait()
}
