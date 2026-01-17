package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newGRPCServer() *grpc.Server {
	// ここに将来的にInterceptor（Auth, Logging, Recovery）を追加する
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(loggingInterceptor),
	}
	return grpc.NewServer(opts...)
}

func (s *Server) runGRPC(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.cfg.GRPCPort)
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on gRPC port %s: %w", addr, err)
	}

	slog.Info("Starting gRPC server", "addr", addr)

	// Serve blocks until Stop() is called or error occurs
	if err := s.grpcServer.Serve(lis); err != nil {
		return err
	}
	return nil
}

// Simple logging interceptor using slog
func loggingInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	duration := time.Since(start)

	code := codes.OK
	if err != nil {
		code = status.Code(err)
	}

	// Structured logging
	slog.Info("gRPC Request",
		"method", info.FullMethod,
		"code", code.String(),
		"duration", duration,
		"error", err,
	)

	return resp, err
}
