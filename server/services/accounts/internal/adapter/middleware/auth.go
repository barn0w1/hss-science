package middleware

import (
	"context"

	"google.golang.org/grpc"
)

// AuthMiddleware is a no-op for accounts service.
// Internal gRPC is protected by network boundaries, and public endpoints use native HTTP.
type AuthMiddleware struct{}

func NewAuthMiddleware() *AuthMiddleware {
	return &AuthMiddleware{}
}

// UnaryServerInterceptor returns a pass-through interceptor.
func (m *AuthMiddleware) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(ctx, req)
	}
}
