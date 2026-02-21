package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain"
)

const (
	stateLength    = 32 // bytes → 64 hex chars
	authCodeLength = 32
	stateTTL       = 10 * time.Minute
	authCodeTTL    = 5 * time.Minute
)

// AuthUsecase encapsulates the core SSO authentication business logic.
// It is the single orchestrator for all auth flows and is completely
// independent of transport (gRPC/HTTP) concerns.
type AuthUsecase struct {
	users     domain.UserRepository
	authCodes domain.AuthCodeRepository
	states    domain.OAuthStateRepository
	providers map[string]domain.OAuthProvider
}

// NewAuthUsecase constructs an AuthUsecase with the given dependencies.
func NewAuthUsecase(
	users domain.UserRepository,
	authCodes domain.AuthCodeRepository,
	states domain.OAuthStateRepository,
	providers []domain.OAuthProvider,
) *AuthUsecase {
	pm := make(map[string]domain.OAuthProvider, len(providers))
	for _, p := range providers {
		pm[p.Name()] = p
	}
	return &AuthUsecase{
		users:     users,
		authCodes: authCodes,
		states:    states,
		providers: pm,
	}
}

// GetAuthURL generates an authorization URL for the requested OAuth provider
// and creates a state entry in the database for later verification.
func (uc *AuthUsecase) GetAuthURL(ctx context.Context, providerName, redirectURI, clientState string) (authURL, state string, err error) {
	provider, ok := uc.providers[providerName]
	if !ok {
		return "", "", fmt.Errorf("unsupported provider: %s", providerName)
	}

	state, err = generateRandomHex(stateLength)
	if err != nil {
		return "", "", fmt.Errorf("generate state: %w", err)
	}

	now := time.Now()
	oauthState := &domain.OAuthState{
		State:       state,
		Provider:    providerName,
		RedirectURI: redirectURI,
		ClientState: clientState,
		ExpiresAt:   now.Add(stateTTL),
		CreatedAt:   now,
	}
	if err := uc.states.Create(ctx, oauthState); err != nil {
		return "", "", fmt.Errorf("save state: %w", err)
	}

	authURL = provider.AuthURL(state)
	return authURL, state, nil
}

// HandleProviderCallback processes the OAuth callback: validates the state,
// exchanges the authorization code with the external provider, upserts the
// user, and issues an internal auth code.
func (uc *AuthUsecase) HandleProviderCallback(ctx context.Context, providerName, code, state string) (authCode, redirectURI, clientState string, user *domain.User, err error) {
	// 1. Validate and consume the OAuth state.
	oauthState, err := uc.states.Consume(ctx, state)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("invalid or expired state: %w", err)
	}

	if oauthState.Provider != providerName {
		return "", "", "", nil, fmt.Errorf("state provider mismatch: expected %s, got %s", oauthState.Provider, providerName)
	}

	// 2. Exchange the code with the external provider.
	provider, ok := uc.providers[providerName]
	if !ok {
		return "", "", "", nil, fmt.Errorf("unsupported provider: %s", providerName)
	}

	userInfo, accessToken, refreshToken, err := provider.Exchange(ctx, code)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("provider exchange: %w", err)
	}

	// 3. Upsert the user in the database.
	user, err = uc.users.UpsertByIdentity(ctx, providerName, userInfo, accessToken, refreshToken)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("upsert user: %w", err)
	}

	// 4. Issue an internal auth code.
	authCode, err = generateRandomHex(authCodeLength)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("generate auth code: %w", err)
	}

	now := time.Now()
	ac := &domain.AuthCode{
		Code:        authCode,
		UserID:      user.ID,
		RedirectURI: oauthState.RedirectURI,
		ClientState: oauthState.ClientState,
		ExpiresAt:   now.Add(authCodeTTL),
		CreatedAt:   now,
	}
	if err := uc.authCodes.Create(ctx, ac); err != nil {
		return "", "", "", nil, fmt.Errorf("save auth code: %w", err)
	}

	return authCode, oauthState.RedirectURI, oauthState.ClientState, user, nil
}

// ExchangeToken validates and consumes an internal auth code, returning
// the associated user.
func (uc *AuthUsecase) ExchangeToken(ctx context.Context, code string) (*domain.User, error) {
	ac, err := uc.authCodes.Consume(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("invalid auth code: %w", err)
	}

	user, err := uc.users.GetByID(ctx, ac.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}

// GetUser retrieves a user by their internal ID.
func (uc *AuthUsecase) GetUser(ctx context.Context, userID string) (*domain.User, error) {
	return uc.users.GetByID(ctx, userID)
}

func generateRandomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
