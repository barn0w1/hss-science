package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/config"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/repository"
	"github.com/golang-jwt/jwt/v5"
)

// ErrUserNotFound should be defined in domain/repository or here to handle "not found" robustly.
// For simplicity, we assume repository returns a specific error or nil.
var ErrInvalidToken = errors.New("invalid or expired token")

type AuthUsecase struct {
	cfg           *config.Config
	userRepo      repository.UserRepository
	tokenRepo     repository.TokenRepository
	oauthProvider repository.OAuthProvider
}

func NewAuthUsecase(
	cfg *config.Config,
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	oauthProvider repository.OAuthProvider,
) *AuthUsecase {
	return &AuthUsecase{
		cfg:           cfg,
		userRepo:      userRepo,
		tokenRepo:     tokenRepo,
		oauthProvider: oauthProvider,
	}
}

// LoginResult contains the tokens returned after a successful login.
type LoginResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int // Seconds
}

// Login handles the OAuth callback, user creation/update, and token issuance.
func (u *AuthUsecase) Login(ctx context.Context, code string, ip, userAgent string) (*LoginResult, error) {
	// 1. Exchange code for user info from Discord
	discordUser, err := u.oauthProvider.GetUserInfo(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to get discord user info: %w", err)
	}

	// 2. Find or Create User
	user, err := u.userRepo.GetByDiscordID(ctx, discordUser.DiscordID)

	if err != nil {
		return nil, fmt.Errorf("database error during user lookup: %w", err)
	}

	if user == nil {
		// New User: Create
		user = model.NewUser(discordUser.DiscordID, discordUser.Name, discordUser.AvatarURL)
		if err := u.userRepo.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else {
		// Existing User: Update Profile
		// UpdateProfile internally checks if changes are needed
		user.UpdateProfile(discordUser.Name, discordUser.AvatarURL)
		if err := u.userRepo.Update(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
	}

	// 3. Issue Tokens
	return u.issueTokens(ctx, user, ip, userAgent)
}

// RefreshTokens handles the token renewal using a valid refresh token.
func (u *AuthUsecase) RefreshTokens(ctx context.Context, rawRefreshToken, ip, userAgent string) (*LoginResult, error) {
	// 1. Hash the raw token to look it up
	tokenHash := hashToken(rawRefreshToken)

	refreshToken, err := u.tokenRepo.Get(ctx, tokenHash)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// 2. Validate (Expired? Revoked?)
	if !refreshToken.IsValid() {
		// Token Rotation Policy:
		// もしRevoked済みなのに使おうとした場合（Replay Attackの可能性）、
		// 本来はここでアラートを上げたり、Userの全セッションを無効化するなどの対策も考えられる。
		return nil, ErrInvalidToken
	}

	// 3. Get User to ensure latest role/status
	user, err := u.userRepo.GetByID(ctx, refreshToken.UserID)
	if err != nil {
		return nil, errors.New("user account not found or deleted")
	}

	// 4. Revoke the OLD refresh token (Token Rotation)
	if err := u.tokenRepo.Revoke(ctx, tokenHash); err != nil {
		return nil, fmt.Errorf("failed to revoke old token: %w", err)
	}

	// 5. Issue NEW tokens
	return u.issueTokens(ctx, user, ip, userAgent)
}

// Logout revokes the refresh token.
func (u *AuthUsecase) Logout(ctx context.Context, rawRefreshToken string) error {
	tokenHash := hashToken(rawRefreshToken)
	return u.tokenRepo.Revoke(ctx, tokenHash)
}

// --- Helper Functions ---

func (u *AuthUsecase) issueTokens(ctx context.Context, user *model.User, ip, userAgent string) (*LoginResult, error) {
	// A. Generate Access Token (JWT)
	// Claims: Sub(UserID), Role, Iss, Exp
	accessTokenClaims := jwt.MapClaims{
		"sub":  user.ID,
		"role": user.Role, // Global Role (e.g. "admin", "user")
		"iss":  "accounts.hss-science.org",
		"exp":  time.Now().Add(15 * time.Minute).Unix(), // 15 min expiry (Standard)
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessTokenClaims)
	signedAccessToken, err := accessToken.SignedString([]byte(u.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// B. Generate Refresh Token (Random String)
	// 32 bytes = 256 bits entropy is sufficient
	rawRefreshToken, err := generateRandomString(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Create RefreshToken Model
	refreshTokenModel := &model.RefreshToken{
		TokenHash: hashToken(rawRefreshToken),
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(time.Duration(u.cfg.RefreshTokenTTL) * 24 * time.Hour),
		CreatedAt: time.Now(),
		RevokedAt: nil, // Explicitly valid
		UserAgent: userAgent,
		IPAddress: ip,
	}

	// Save to DB
	if err := u.tokenRepo.Save(ctx, refreshTokenModel); err != nil {
		return nil, fmt.Errorf("failed to save refresh token: %w", err)
	}

	return &LoginResult{
		AccessToken:  signedAccessToken,
		RefreshToken: rawRefreshToken, // Return RAW token to client
		ExpiresIn:    15 * 60,
	}, nil
}

// generateRandomString generates a URL-safe random string of length n bytes.
func generateRandomString(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// hashToken calculates SHA-256 hash of the token string.
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
