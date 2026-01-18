package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pb "github.com/barn0w1/hss-science/server/gen/public/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"
	"google.golang.org/grpc"
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

// GetAuthUrl generates the OAuth authorization URL.
func (h *AuthHandler) GetAuthUrl(ctx context.Context, req *pb.GetAuthUrlRequest) (*pb.GetAuthUrlResponse, error) {
	if req.RedirectUrl == "" {
		return nil, status.Error(codes.InvalidArgument, "redirect_url is required")
	}

	url, err := h.usecase.GetAuthURL(ctx, req.RedirectUrl)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to generate url: %v", err)
	}

	return &pb.GetAuthUrlResponse{Url: url}, nil
}

// Login handles the OAuth login request.
func (h *AuthHandler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.Code == "" {
		return nil, status.Error(codes.InvalidArgument, "auth code is required")
	}

	ip := getClientIP(ctx)
	ua := getUserAgent(ctx)

	result, err := h.usecase.Login(ctx, req.Code, ip, ua)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "login failed: %v", err)
	}

	// Refresh TokenをHttpOnly Cookieに設定
	setRefreshTokenCookie(ctx, result.RefreshToken)

	return &pb.LoginResponse{
		AccessToken:  result.AccessToken,
		ExpiresIn:    int32(result.ExpiresIn),
		RefreshToken: "", // Cookieに入れたのでBodyには含めない
	}, nil
}

// RefreshToken handles token refresh request using Cookie.
func (h *AuthHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	// 1. Cookieから取得を試みる
	refreshToken := getRefreshTokenFromCookie(ctx)

	// 2. なければBodyから取得 (Fallback)
	if refreshToken == "" {
		refreshToken = req.RefreshToken
	}

	if refreshToken == "" {
		return nil, status.Error(codes.Unauthenticated, "refresh token is missing")
	}

	ip := getClientIP(ctx)
	ua := getUserAgent(ctx)

	result, err := h.usecase.RefreshTokens(ctx, refreshToken, ip, ua)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidToken) {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired refresh token")
		}
		return nil, status.Errorf(codes.Internal, "refresh failed: %v", err)
	}

	// 新しいRefresh TokenをCookieに再設定 (Rotation)
	setRefreshTokenCookie(ctx, result.RefreshToken)

	return &pb.RefreshTokenResponse{
		AccessToken:  result.AccessToken,
		ExpiresIn:    int32(result.ExpiresIn),
		RefreshToken: "", // Cookieに入れたのでBodyには含めない
	}, nil
}

// Logout handles logout request.
func (h *AuthHandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	// Cookieから取得
	refreshToken := getRefreshTokenFromCookie(ctx)
	if refreshToken == "" {
		refreshToken = req.RefreshToken
	}

	if refreshToken == "" {
		// すでにない場合は成功として返す
		return &pb.LogoutResponse{}, nil
	}

	if err := h.usecase.Logout(ctx, refreshToken); err != nil {
		return nil, status.Errorf(codes.Internal, "logout failed: %v", err)
	}

	// Cookieを削除 (Max-Age=0)
	clearRefreshTokenCookie(ctx)

	return &pb.LogoutResponse{}, nil
}

// GetMe returns the current user info.
func (h *AuthHandler) GetMe(ctx context.Context, req *pb.GetMeRequest) (*pb.User, error) {
	// Middlewareですでに検証済みだが、UserIDを抽出するために再度検証
	// (本来はContextから取り出すのが良いが、簡略化のためここで再Parse)
	userID, err := h.extractUserID(ctx)
	if err != nil {
		return nil, err
	}

	user, err := h.usecase.GetMe(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get user: %v", err)
	}
	if user == nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	return &pb.User{
		Id:        user.ID,
		Name:      user.Name,
		AvatarUrl: user.AvatarURL,
		Role:      string(user.Role),
	}, nil
}

// --- Helpers ---

func setRefreshTokenCookie(ctx context.Context, token string) {
	// Pathを /v1/auth に限定し、リフレッシュ時のみ送信されるようにする
	// Secure; SameSite=Lax; HttpOnly
	// 30 days = 2592000 seconds
	cookie := fmt.Sprintf("refresh_token=%s; Path=/v1/auth; HttpOnly; Secure; SameSite=Lax; Max-Age=%d",
		token, 30*24*60*60)

	// Gatewayがこれを Set-Cookie ヘッダーに変換する
	grpc.SendHeader(ctx, metadata.Pairs("Set-Cookie", cookie))
}

func clearRefreshTokenCookie(ctx context.Context) {
	cookie := "refresh_token=; Path=/v1/auth; HttpOnly; Secure; SameSite=Lax; Max-Age=0"
	grpc.SendHeader(ctx, metadata.Pairs("Set-Cookie", cookie))
}

func getRefreshTokenFromCookie(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	// "cookie" key comes from gateway matcher
	cookies := md.Get("cookie")
	for _, c := range cookies {
		// Simple parsing: "key=value; key2=value2"
		parts := strings.Split(c, ";")
		for _, part := range parts {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) == 2 && kv[0] == "refresh_token" {
				return kv[1]
			}
		}
	}
	return ""
}

func (h *AuthHandler) extractUserID(ctx context.Context) (string, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return "", status.Error(codes.Unauthenticated, "no auth header")
	}
	token := strings.TrimPrefix(authHeaders[0], "Bearer ")
	return h.usecase.VerifyAccessToken(token)
}

func getClientIP(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if forwarded := md.Get("x-forwarded-for"); len(forwarded) > 0 {
			ips := strings.Split(forwarded[0], ",")
			return strings.TrimSpace(ips[0])
		}
	}
	if p, ok := peer.FromContext(ctx); ok {
		return p.Addr.String()
	}
	return "unknown"
}

func getUserAgent(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if ua := md.Get("grpcgateway-user-agent"); len(ua) > 0 {
			return ua[0]
		}
		if ua := md.Get("user-agent"); len(ua) > 0 {
			return ua[0]
		}
	}
	return "unknown"
}
