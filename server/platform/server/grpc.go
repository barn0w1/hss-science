package server

import (
	"context"
	"log/slog"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// newGRPCServer creates a gRPC server with standard interceptors.
// It enforces logging and panic recovery for all services.
func newGRPCServer() *grpc.Server {
	// Logger adapter to bridge slog with grpc-middleware
	loggerOpts := []logging.Option{
		logging.WithLogOnEvents(logging.StartCall, logging.FinishCall),
	}

	opts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(
			recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(recoveryHandler)),
			logging.UnaryServerInterceptor(interceptorLogger(slog.Default()), loggerOpts...),
		),
	}

	s := grpc.NewServer(opts...)

	// Enable Server Reflection for tools like grpcurl/Postman
	reflection.Register(s)

	return s
}

// interceptorLogger adapts slog to the logging interceptor interface.
func interceptorLogger(l *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		l.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}

// recoveryHandler ensures the server returns a proper error code instead of crashing on panic.
func recoveryHandler(p any) (err error) {
	slog.Error("Recovered from panic", "panic", p)
	return status.Errorf(codes.Internal, "internal server error")
}
