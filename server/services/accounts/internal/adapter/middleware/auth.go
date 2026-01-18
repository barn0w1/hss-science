package middleware

import (
	"context"
	"strings"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AuthMiddleware struct {
	usecase *usecase.AuthUsecase
}

func NewAuthMiddleware(u *usecase.AuthUsecase) *AuthMiddleware {
	return &AuthMiddleware{usecase: u}
}

// UnaryServerInterceptor returns the gRPC interceptor.
func (m *AuthMiddleware) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// 1. Skip authentication for public endpoints
		if isPublicEndpoint(info.FullMethod) {
			return handler(ctx, req)
		}

		// 2. Authenticate
		userID, err := m.authenticate(ctx)
		if err != nil {
			return nil, err
		}

		// 3. Inject UserID into Context (optional, but good practice)
		// type contextKey string
		// const userIDKey contextKey = "user_id"
		// ctx = context.WithValue(ctx, userIDKey, userID)

		return handler(ctx, req)
	}
}

func (m *AuthMiddleware) authenticate(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "metadata missing")
	}

	// Gateway passes "Authorization" header
	values := md.Get("authorization")
	if len(values) == 0 {
		return "", status.Error(codes.Unauthenticated, "authorization required")
	}

	// Format: "Bearer <token>"
	token := strings.TrimPrefix(values[0], "Bearer ")
	userID, err := m.usecase.VerifyAccessToken(token)
	if err != nil {
		return "", status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
	}

	return userID, nil
}

func isPublicEndpoint(method string) bool {
	// Proto package name: hss_science.accounts.v1
	// Service name: AccountsService
	prefix := "/hss_science.accounts.v1.AccountsService/"

	publicMethods := map[string]bool{
		prefix + "GetAuthUrl":   true,
		prefix + "Login":        true,
		prefix + "RefreshToken": true, // RefreshToken logic handles its own cookie validation
		prefix + "Logout":       true,
	}
	return publicMethods[method]
}
