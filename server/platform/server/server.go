package server

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/barn0w1/hss-science/server/platform/config"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

// Server manages both the gRPC server and the HTTP Gateway server.
type Server struct {
	cfg        config.AppConfig
	grpcServer *grpc.Server
	// Gateway registration function
	gatewayReg func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error
}

// New creates a new Server manager.
// interceptors: Optional list of gRPC middleware (e.g. Auth).
func New(cfg config.AppConfig, interceptors ...grpc.UnaryServerInterceptor) *Server {
	return &Server{
		cfg:        cfg,
		grpcServer: newGRPCServer(cfg, interceptors...),
	}
}

// GrpcServer returns the internal gRPC server to register services.
func (s *Server) GrpcServer() *grpc.Server {
	return s.grpcServer
}

// RegisterGateway sets the registration function for the HTTP Gateway.
// The callback usually calls pb.RegisterServiceHandlerFromEndpoint.
func (s *Server) RegisterGateway(fn func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error) {
	s.gatewayReg = fn
}

// Run starts both servers and waits for a termination signal.
func (s *Server) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	// 1. Start gRPC Server
	g.Go(func() error {
		return s.runGRPC(ctx) // defined in grpc.go
	})

	// 2. Start HTTP Gateway (depends on gRPC)
	g.Go(func() error {
		// Wait a bit for gRPC to start (optional but safer in local dev)
		time.Sleep(100 * time.Millisecond)
		return s.runGateway(ctx) // defined in gateway.go
	})

	// 3. Wait for signal and shutdown
	g.Go(func() error {
		<-ctx.Done()
		slog.Info("Shutdown signal received")

		// Shutdown Gracefully
		s.grpcServer.GracefulStop()
		// HTTP shutdown is handled in runGateway via server.Shutdown(ctx)

		return nil
	})

	return g.Wait()
}
