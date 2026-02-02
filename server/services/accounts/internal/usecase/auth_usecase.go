package usecase

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/config"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/repository"
	"github.com/google/uuid"
)

// ErrInvalidState is returned when OAuth state validation fails.
var ErrInvalidState = errors.New("invalid state")

type AuthUsecase struct {
	cfg           *config.Config
	userRepo      repository.UserRepository
	sessionRepo   repository.SessionRepository
	authCodeRepo  repository.AuthCodeRepository
	oauthProvider repository.OAuthProvider
}

func NewAuthUsecase(
	cfg *config.Config,
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	authCodeRepo repository.AuthCodeRepository,
	oauthProvider repository.OAuthProvider,
) *AuthUsecase {
	return &AuthUsecase{
		cfg:           cfg,
		userRepo:      userRepo,
		sessionRepo:   sessionRepo,
		authCodeRepo:  authCodeRepo,
		oauthProvider: oauthProvider,
	}
}

type OAuthCallbackResult struct {
	Session      *model.Session
	SessionToken string
	Audience     string
	RedirectURI  string
	State        string
}

type VerifyAuthCodeResult struct {
	UserID uuid.UUID
	Role   model.GlobalRole
}

// Authorize handles the /authorize request flow.
func (u *AuthUsecase) Authorize(ctx context.Context, audience, redirectURI, state, sessionID string) (string, error) {
	if audience == "" || redirectURI == "" {
		return "", fmt.Errorf("audience and redirect_uri are required")
	}

	if sessionID != "" {
		session, err := u.sessionRepo.GetByTokenHash(ctx, model.HashToken(sessionID))
		if err == nil && session.IsValid(time.Now()) {
			code, raw, err := model.NewAuthCode(session.UserID, audience, redirectURI, time.Duration(u.cfg.AuthCodeTTLSeconds)*time.Second)
			if err != nil {
				return "", fmt.Errorf("failed to create auth code: %w", err)
			}
			if err := u.authCodeRepo.Create(ctx, code); err != nil {
				return "", fmt.Errorf("failed to create auth code: %w", err)
			}
			return buildRedirectURL(redirectURI, raw, state), nil
		}
	}

	encodedState, err := u.encodeState(oauthState{
		Audience:    audience,
		RedirectURI: redirectURI,
		State:       state,
		IssuedAt:    time.Now().Unix(),
		Nonce:       randomNonce(),
	})
	if err != nil {
		return "", fmt.Errorf("failed to encode state: %w", err)
	}

	return u.oauthProvider.GetAuthURL(u.cfg.DiscordRedirectURL, encodedState), nil
}

// OAuthCallback handles Discord callback and session creation.
func (u *AuthUsecase) OAuthCallback(ctx context.Context, code, state, ip, userAgent string) (*OAuthCallbackResult, error) {
	if code == "" || state == "" {
		return nil, fmt.Errorf("code and state are required")
	}

	decoded, err := u.decodeState(state)
	if err != nil {
		return nil, err
	}

	if time.Since(time.Unix(decoded.IssuedAt, 0)) > time.Duration(u.cfg.OAuthStateTTLSeconds)*time.Second {
		return nil, ErrInvalidState
	}

	discordUser, err := u.oauthProvider.GetUserInfo(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to get discord user info: %w", err)
	}

	user, err := u.userRepo.GetByDiscordID(ctx, discordUser.DiscordID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			user = nil
		} else {
			return nil, fmt.Errorf("database error during user lookup: %w", err)
		}
	}

	if user == nil {
		user = model.NewUser(discordUser.DiscordID, discordUser.Name, discordUser.AvatarURL)
		if err := u.userRepo.Create(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to create user: %w", err)
		}
	} else {
		user.UpdateProfile(discordUser.Name, discordUser.AvatarURL)
		if err := u.userRepo.Update(ctx, user); err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
	}

	session, sessionToken, err := model.NewSession(user.ID, time.Duration(u.cfg.SessionTTLHours)*time.Hour, userAgent, ip)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	if err := u.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &OAuthCallbackResult{
		Session:      session,
		SessionToken: sessionToken,
		Audience:     decoded.Audience,
		RedirectURI:  decoded.RedirectURI,
		State:        decoded.State,
	}, nil
}

// VerifyAuthCode validates and consumes an auth code.
func (u *AuthUsecase) VerifyAuthCode(ctx context.Context, code, audience string) (*VerifyAuthCodeResult, error) {
	if code == "" || audience == "" {
		return nil, fmt.Errorf("auth_code and audience are required")
	}

	codeHash := model.HashToken(code)
	authCode, err := u.authCodeRepo.GetByCodeHash(ctx, codeHash)
	if err != nil {
		return nil, err
	}

	if authCode.Audience != audience {
		return nil, repository.ErrNotFound
	}

	if authCode.IsExpired(time.Now()) || authCode.IsConsumed() {
		return nil, repository.ErrNotFound
	}

	if err := u.authCodeRepo.Consume(ctx, codeHash, time.Now()); err != nil {
		return nil, err
	}

	user, err := u.userRepo.GetByID(ctx, authCode.UserID)
	if err != nil {
		return nil, err
	}

	return &VerifyAuthCodeResult{
		UserID: user.ID,
		Role:   user.Role,
	}, nil
}

// --- Helper Functions ---

type oauthState struct {
	Audience    string `json:"audience"`
	RedirectURI string `json:"redirect_uri"`
	State       string `json:"state"`
	IssuedAt    int64  `json:"iat"`
	Nonce       string `json:"nonce"`
}

func (u *AuthUsecase) encodeState(s oauthState) (string, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return "", err
	}

	sig := signState(payload, []byte(u.cfg.StateSecret))
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)
	encodedSig := base64.RawURLEncoding.EncodeToString(sig)

	return fmt.Sprintf("%s.%s", encodedPayload, encodedSig), nil
}

func (u *AuthUsecase) decodeState(encoded string) (*oauthState, error) {
	parts := strings.SplitN(encoded, ".", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidState
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidState
	}

	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidState
	}

	expected := signState(payload, []byte(u.cfg.StateSecret))
	if !hmac.Equal(sig, expected) {
		return nil, ErrInvalidState
	}

	var s oauthState
	if err := json.Unmarshal(payload, &s); err != nil {
		return nil, ErrInvalidState
	}

	if s.Audience == "" || s.RedirectURI == "" {
		return nil, ErrInvalidState
	}

	return &s, nil
}

func signState(payload, secret []byte) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write(payload)
	return mac.Sum(nil)
}

func randomNonce() string {
	b := make([]byte, 12)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func buildRedirectURL(redirectURI, code, state string) string {
	query := url.Values{}
	query.Set("code", code)
	if state != "" {
		query.Set("state", state)
	}
	sep := "?"
	if strings.Contains(redirectURI, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%s%s", redirectURI, sep, query.Encode())
}
