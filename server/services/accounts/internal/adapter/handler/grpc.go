package handler

import (
	"context"
	"errors"
	"strings"

	pb "github.com/barn0w1/hss-science/server/gen/public/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type AuthHandler struct {
	pb.UnimplementedAccountsServiceServer
	usecase *usecase.AuthUsecase
}

func NewAuthHandler(usecase *usecase.AuthUsecase) *AuthHandler {
	return &AuthHandler{
		usecase: usecase,
	}
}

// Login handles the OAuth login request.
func (h *AuthHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	// Validate request
	if req.Code == "" {
		return nil, status.Error(codes.InvalidArgument, "auth code is required")
	}

	// Extract Metadata (IP, User-Agent)
	ip := getClientIP(ctx)
	ua := getUserAgent(ctx)

	// Call UseCase
	result, err := h.usecase.Login(ctx, req.Code, ip, ua)
	if err != nil {
		// エラーログはInterceptorが出すので、ここでは適切なステータスコードへの変換に集中する
		return nil, status.Errorf(codes.Internal, "login failed: %v", err)
	}

	return &pb.LoginResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    int32(result.ExpiresIn),
	}, nil
}

// Refresh handles token refresh request.
func (h *AuthHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	ip := getClientIP(ctx)
	ua := getUserAgent(ctx)

	result, err := h.usecase.RefreshTokens(ctx, req.RefreshToken, ip, ua)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidToken) {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
		}
		return nil, status.Errorf(codes.Internal, "refresh failed: %v", err)
	}

	return &pb.RefreshTokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresIn:    int32(result.ExpiresIn),
	}, nil
}

// Logout handles logout request.
func (h *AuthHandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh token is required")
	}

	if err := h.usecase.Logout(ctx, req.RefreshToken); err != nil {
		// ログアウト失敗はクライアントには成功として返しても良いが、一応エラーを返す
		return nil, status.Errorf(codes.Internal, "logout failed: %v", err)
	}

	return &pb.LogoutResponse{}, nil
}

// --- Helper Functions to extract metadata ---

func getClientIP(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		// 1. Check X-Forwarded-For (From Gateway/Cloudflare)
		if forwarded := md.Get("x-forwarded-for"); len(forwarded) > 0 {
			// Cloudflare sends comma separated IPs, take the first one
			ips := strings.Split(forwarded[0], ",")
			return strings.TrimSpace(ips[0])
		}
	}
	// 2. Fallback to Peer IP (Direct connection)
	if p, ok := peer.FromContext(ctx); ok {
		return p.Addr.String()
	}
	return "unknown"
}

func getUserAgent(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		// gRPC-Gateway maps "User-Agent" header to "grpcgateway-user-agent"
		if ua := md.Get("grpcgateway-user-agent"); len(ua) > 0 {
			return ua[0]
		}
		// Fallback for direct gRPC clients
		if ua := md.Get("user-agent"); len(ua) > 0 {
			return ua[0]
		}
	}
	return "unknown"
}
