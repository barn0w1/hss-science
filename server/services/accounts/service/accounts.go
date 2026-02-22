// Package service implements the core business logic for the accounts service.
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/barn0w1/hss-science/server/services/accounts/domain"
	"github.com/barn0w1/hss-science/server/services/accounts/provider"
)

// AccountsService contains the core business logic for authentication.
type AccountsService struct {
	users     domain.UserRepository
	states    domain.StateRepository
	authCodes domain.AuthCodeRepository
	providers map[string]provider.OAuthProvider
	stateTTL  time.Duration
	codeTTL   time.Duration
}

// New creates a new AccountsService.
func New(
	users domain.UserRepository,
	states domain.StateRepository,
	authCodes domain.AuthCodeRepository,
	providers map[string]provider.OAuthProvider,
	stateTTL, codeTTL time.Duration,
) *AccountsService {
	return &AccountsService{
		users:     users,
		states:    states,
		authCodes: authCodes,
		providers: providers,
		stateTTL:  stateTTL,
		codeTTL:   codeTTL,
	}
}

// GetAuthURL generates an OAuth authorization URL and stores the state for later verification.
func (s *AccountsService) GetAuthURL(ctx context.Context, providerName, redirectURI, clientState string) (authURL, state string, err error) {
	p, err := s.getProvider(providerName)
	if err != nil {
		return "", "", err
	}

	state, err = generateRandomString(32)
	if err != nil {
		return "", "", fmt.Errorf("generate state: %w", err)
	}

	oauthState := &domain.OAuthState{
		State:       state,
		Provider:    providerName,
		RedirectURI: redirectURI,
		ClientState: clientState,
		ExpiresAt:   time.Now().Add(s.stateTTL),
	}
	if err := s.states.Create(ctx, oauthState); err != nil {
		return "", "", fmt.Errorf("store state: %w", err)
	}

	authURL = p.AuthCodeURL(state)
	return authURL, state, nil
}

// HandleProviderCallback processes the OAuth callback: validates state, exchanges
// the code for tokens, fetches user info, upserts the user, and issues an auth code.
func (s *AccountsService) HandleProviderCallback(ctx context.Context, providerName, code, stateValue string) (authCode, redirectURI, clientState string, user *domain.User, err error) {
	// Validate and consume the state.
	oauthState, err := s.states.Consume(ctx, stateValue)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("consume state: %w", err)
	}

	// Verify provider matches.
	if oauthState.Provider != providerName {
		return "", "", "", nil, fmt.Errorf("provider mismatch: expected %s, got %s", oauthState.Provider, providerName)
	}

	p, err := s.getProvider(providerName)
	if err != nil {
		return "", "", "", nil, err
	}

	// Exchange code for token.
	token, err := p.Exchange(ctx, code)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("exchange code: %w", err)
	}

	// Fetch user info from provider.
	userInfo, err := p.FetchUserInfo(ctx, token)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("fetch user info: %w", err)
	}

	// Upsert user and external account.
	extAccount := &domain.ExternalAccount{
		Provider:         providerName,
		ProviderUserID:   userInfo.ProviderUserID,
		ProviderUsername: userInfo.Username,
		ProviderEmail:    userInfo.Email,
		AccessToken:      token.AccessToken,
		RefreshToken:     stringPtr(token.RefreshToken),
		TokenExpiry:      timePtr(token.Expiry),
	}

	user, err = s.users.UpsertByProvider(ctx, extAccount, userInfo.DisplayName, userInfo.AvatarURL)
	if err != nil {
		return "", "", "", nil, fmt.Errorf("upsert user: %w", err)
	}

	// Issue internal auth code.
	authCode, err = s.issueAuthCode(ctx, user.ID)
	if err != nil {
		return "", "", "", nil, err
	}

	return authCode, oauthState.RedirectURI, oauthState.ClientState, user, nil
}

// IssueAuthCode creates a new internal authorization code for an already-authenticated user.
func (s *AccountsService) IssueAuthCode(ctx context.Context, userID uuid.UUID) (string, error) {
	// Verify user exists.
	if _, err := s.users.GetByID(ctx, userID); err != nil {
		return "", fmt.Errorf("verify user: %w", err)
	}
	return s.issueAuthCode(ctx, userID)
}

// ExchangeToken validates and consumes an internal auth code, returning the associated user.
func (s *AccountsService) ExchangeToken(ctx context.Context, codeValue string) (*domain.User, error) {
	ac, err := s.authCodes.Consume(ctx, codeValue)
	if err != nil {
		return nil, fmt.Errorf("consume auth code: %w", err)
	}

	user, err := s.users.GetByID(ctx, ac.UserID)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	return user, nil
}

// GetUser retrieves a user by their internal ID.
func (s *AccountsService) GetUser(ctx context.Context, userID uuid.UUID) (*domain.User, error) {
	return s.users.GetByID(ctx, userID)
}

func (s *AccountsService) getProvider(name string) (provider.OAuthProvider, error) {
	p, ok := s.providers[name]
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
	return p, nil
}

func (s *AccountsService) issueAuthCode(ctx context.Context, userID uuid.UUID) (string, error) {
	code, err := generateRandomString(32)
	if err != nil {
		return "", fmt.Errorf("generate auth code: %w", err)
	}

	ac := &domain.AuthCode{
		Code:      code,
		UserID:    userID,
		ExpiresAt: time.Now().Add(s.codeTTL),
	}
	if err := s.authCodes.Create(ctx, ac); err != nil {
		return "", fmt.Errorf("store auth code: %w", err)
	}

	return code, nil
}

func generateRandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
