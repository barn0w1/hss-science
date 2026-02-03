package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"time"

	"github.com/barn0w1/hss-science/server/platform/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// customInterceptors allows services to inject their own middleware (e.g. Auth).
func newGRPCServer(cfg config.AppConfig, customInterceptors ...grpc.UnaryServerInterceptor) *grpc.Server {
	// 1. 標準インターセプター (実行順序: Recovery -> Logging -> Custom...)
	interceptors := []grpc.UnaryServerInterceptor{
		recoveryInterceptor, // 最優先: パニックキャッチ
		loggingInterceptor,  // 次点: ログ記録
	}

	// 2. サービス固有のインターセプターを追加 (Authなど)
	interceptors = append(interceptors, customInterceptors...)

	// 3. Chainを使って一括登録
	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(interceptors...),
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

// --- Interceptors ---

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

// recoveryInterceptor catches panics and converts them to gRPC Internal error.
func recoveryInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("gRPC Panic Recovered",
				"method", info.FullMethod,
				"panic", r,
				"stack", string(debug.Stack()),
			)
			err = status.Errorf(codes.Internal, "internal server error")
		}
	}()
	return handler(ctx, req)
}
