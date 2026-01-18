package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	pb "github.com/barn0w1/hss-science/server/gen/public/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/config" // Configのimport
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
	cfg     *config.Config // Configフィールド追加
}

// Configを引数に追加
func NewAuthHandler(usecase *usecase.AuthUsecase, cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		usecase: usecase,
		cfg:     cfg,
	}
}

// GetAuthUrl generates the OAuth authorization URL.
func (h *AuthHandler) GetAuthUrl(ctx context.Context, req *pb.GetAuthUrlRequest) (*pb.GetAuthUrlResponse, error) {
	url, err := h.usecase.GetAuthURL(ctx)
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

	// メソッド呼び出しに変更 (h.setRefreshTokenCookie)
	h.setRefreshTokenCookie(ctx, result.RefreshToken)

	return &pb.LoginResponse{
		AccessToken:  result.AccessToken,
		ExpiresIn:    int32(result.ExpiresIn),
		RefreshToken: "", // Cookieに入れたのでBodyには含めない
	}, nil
}

// RefreshToken handles token refresh request using Cookie.
func (h *AuthHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	refreshToken := getRefreshTokenFromCookie(ctx)

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

	// メソッド呼び出しに変更
	h.setRefreshTokenCookie(ctx, result.RefreshToken)

	return &pb.RefreshTokenResponse{
		AccessToken:  result.AccessToken,
		ExpiresIn:    int32(result.ExpiresIn),
		RefreshToken: "",
	}, nil
}

// Logout handles logout request.
func (h *AuthHandler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	refreshToken := getRefreshTokenFromCookie(ctx)
	if refreshToken == "" {
		refreshToken = req.RefreshToken
	}

	if refreshToken == "" {
		return &pb.LogoutResponse{}, nil
	}

	if err := h.usecase.Logout(ctx, refreshToken); err != nil {
		return nil, status.Errorf(codes.Internal, "logout failed: %v", err)
	}

	// メソッド呼び出しに変更
	h.clearRefreshTokenCookie(ctx)

	return &pb.LogoutResponse{}, nil
}

// GetMe returns the current user info.
func (h *AuthHandler) GetMe(ctx context.Context, req *pb.GetMeRequest) (*pb.User, error) {
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

func (h *AuthHandler) setRefreshTokenCookie(ctx context.Context, token string) {
	// 修正ポイント1: Path を "/" に変更 (API全体でCookieを有効にするため)
	// 修正ポイント2: 属性の順番やフォーマットを標準に合わせる
	cookie := fmt.Sprintf("refresh_token=%s; Path=/; HttpOnly; SameSite=%s; Max-Age=%d",
		token, h.cfg.CookieSameSite, 30*24*60*60) // 30 days

	// Secure属性 (HTTPS環境では必須)
	if h.cfg.CookieSecure {
		cookie += "; Secure"
	}

	// Domain属性
	// .hss-science.org のように指定されていれば、サブドメイン間(drive等)で共有可能
	if h.cfg.CookieDomain != "" {
		cookie += fmt.Sprintf("; Domain=%s", h.cfg.CookieDomain)
	}

	grpc.SendHeader(ctx, metadata.Pairs("Set-Cookie", cookie))
}

func (h *AuthHandler) clearRefreshTokenCookie(ctx context.Context) {
	// 修正ポイント: ここも Path を "/" に合わせる（そうしないと削除できない）
	cookie := fmt.Sprintf("refresh_token=; Path=/; HttpOnly; SameSite=%s; Max-Age=0", h.cfg.CookieSameSite)

	if h.cfg.CookieSecure {
		cookie += "; Secure"
	}
	if h.cfg.CookieDomain != "" {
		cookie += fmt.Sprintf("; Domain=%s", h.cfg.CookieDomain)
	}

	grpc.SendHeader(ctx, metadata.Pairs("Set-Cookie", cookie))
}

// 以下は変更なし（関数として利用）
func getRefreshTokenFromCookie(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	cookies := md.Get("cookie")
	for _, c := range cookies {
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
