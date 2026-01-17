package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/barn0w1/hss-science/server/platform/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// 引数に cfg を追加
func newGRPCServer(cfg config.AppConfig) *grpc.Server {
	// 将来的なInterceptor（Auth, Logging, Recovery）用
	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(loggingInterceptor),
	}

	s := grpc.NewServer(opts...)

	// ENVが "dev" の場合のみReflectionを有効化
	if cfg.Env == "dev" {
		reflection.Register(s)
		slog.Info("gRPC Reflection enabled (env=dev)")
	}

	return s
}

func (s *Server) runGRPC(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.cfg.GRPCPort)

	lc := net.ListenConfig{}
	lis, err := lc.Listen(ctx, "tcp", addr)
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
