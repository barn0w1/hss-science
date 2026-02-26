package authz

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type contextKey string

const userIDKey contextKey = "user_id"

// UserIDFromContext extracts the authenticated user's ID from the context.
func UserIDFromContext(ctx context.Context) (string, error) {
	v, ok := ctx.Value(userIDKey).(string)
	if !ok || v == "" {
		return "", status.Error(codes.Unauthenticated, "no authenticated user in context")
	}
	return v, nil
}

// UnaryJWTInterceptor returns a gRPC unary server interceptor that validates
// the JWT access token from the "authorization" metadata key and injects the
// user's subject into the context.
func UnaryJWTInterceptor(verifier TokenVerifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing metadata")
		}

		authHeader := md.Get("authorization")
		if len(authHeader) == 0 {
			return nil, status.Error(codes.Unauthenticated, "missing authorization header")
		}

		rawToken := strings.TrimPrefix(authHeader[0], "Bearer ")
		if rawToken == authHeader[0] {
			return nil, status.Error(codes.Unauthenticated, "invalid authorization format")
		}

		claims, err := verifier.Verify(ctx, rawToken)
		if err != nil {
			return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
		}

		ctx = context.WithValue(ctx, userIDKey, claims.Subject)
		return handler(ctx, req)
	}
}
