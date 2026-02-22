package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/barn0w1/hss-science/server/services/accounts/domain"
	"github.com/barn0w1/hss-science/server/services/accounts/provider"
)

// --- Mock Repositories ---

type mockUserRepo struct {
	users map[uuid.UUID]*domain.User
	// Track calls for upsert
	lastUpsertAccount     *domain.ExternalAccount
	lastUpsertDisplayName string
	lastUpsertAvatarURL   string
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{users: make(map[uuid.UUID]*domain.User)}
}

func (m *mockUserRepo) UpsertByProvider(_ context.Context, account *domain.ExternalAccount, displayName, avatarURL string) (*domain.User, error) {
	m.lastUpsertAccount = account
	m.lastUpsertDisplayName = displayName
	m.lastUpsertAvatarURL = avatarURL

	// Simulate: always create a new user for simplicity.
	id := uuid.New()
	now := time.Now()
	user := &domain.User{
		ID:          id,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.users[id] = user
	return user, nil
}

func (m *mockUserRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, domain.ErrUserNotFound
	}
	return u, nil
}

type mockStateRepo struct {
	states map[string]*domain.OAuthState
}

func newMockStateRepo() *mockStateRepo {
	return &mockStateRepo{states: make(map[string]*domain.OAuthState)}
}

func (m *mockStateRepo) Create(_ context.Context, state *domain.OAuthState) error {
	m.states[state.State] = state
	return nil
}

func (m *mockStateRepo) Consume(_ context.Context, stateValue string) (*domain.OAuthState, error) {
	st, ok := m.states[stateValue]
	if !ok {
		return nil, domain.ErrStateNotFound
	}
	if time.Now().After(st.ExpiresAt) {
		delete(m.states, stateValue)
		return nil, domain.ErrStateNotFound
	}
	delete(m.states, stateValue)
	return st, nil
}

func (m *mockStateRepo) DeleteExpired(_ context.Context) (int64, error) {
	return 0, nil
}

type mockAuthCodeRepo struct {
	codes map[string]*domain.AuthCode
}

func newMockAuthCodeRepo() *mockAuthCodeRepo {
	return &mockAuthCodeRepo{codes: make(map[string]*domain.AuthCode)}
}

func (m *mockAuthCodeRepo) Create(_ context.Context, code *domain.AuthCode) error {
	m.codes[code.Code] = code
	return nil
}

func (m *mockAuthCodeRepo) Consume(_ context.Context, codeValue string) (*domain.AuthCode, error) {
	ac, ok := m.codes[codeValue]
	if !ok || ac.Used || time.Now().After(ac.ExpiresAt) {
		return nil, domain.ErrAuthCodeNotFound
	}
	ac.Used = true
	return ac, nil
}

func (m *mockAuthCodeRepo) DeleteExpired(_ context.Context) (int64, error) {
	return 0, nil
}

// --- Mock Provider ---

type mockProvider struct {
	name     string
	userInfo *provider.UserInfo
	token    *oauth2.Token
	err      error
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) AuthCodeURL(state string) string {
	return "https://mock-provider.com/authorize?state=" + state
}

func (m *mockProvider) Exchange(_ context.Context, _ string) (*oauth2.Token, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.token, nil
}

func (m *mockProvider) FetchUserInfo(_ context.Context, _ *oauth2.Token) (*provider.UserInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.userInfo, nil
}

// --- Helper ---

func newTestService(userRepo *mockUserRepo, stateRepo *mockStateRepo, codeRepo *mockAuthCodeRepo, providers map[string]provider.OAuthProvider) *AccountsService {
	return New(userRepo, stateRepo, codeRepo, providers, 10*time.Minute, 5*time.Minute)
}

// --- Tests ---

func TestGetAuthURL(t *testing.T) {
	userRepo := newMockUserRepo()
	stateRepo := newMockStateRepo()
	codeRepo := newMockAuthCodeRepo()
	mp := &mockProvider{name: "discord"}
	svc := newTestService(userRepo, stateRepo, codeRepo, map[string]provider.OAuthProvider{"discord": mp})

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		authURL, state, err := svc.GetAuthURL(ctx, "discord", "https://app.example.com/callback", "client-state-123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if authURL == "" {
			t.Fatal("expected non-empty auth URL")
		}
		if state == "" {
			t.Fatal("expected non-empty state")
		}

		// State should be stored.
		if len(stateRepo.states) != 1 {
			t.Fatalf("expected 1 stored state, got %d", len(stateRepo.states))
		}
		storedState := stateRepo.states[state]
		if storedState.RedirectURI != "https://app.example.com/callback" {
			t.Fatalf("expected redirect_uri stored, got %s", storedState.RedirectURI)
		}
		if storedState.ClientState != "client-state-123" {
			t.Fatalf("expected client_state stored, got %s", storedState.ClientState)
		}
	})

	t.Run("unknown provider", func(t *testing.T) {
		_, _, err := svc.GetAuthURL(ctx, "unknown", "https://example.com", "")
		if err == nil {
			t.Fatal("expected error for unknown provider")
		}
	})
}

func TestHandleProviderCallback(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		userRepo := newMockUserRepo()
		stateRepo := newMockStateRepo()
		codeRepo := newMockAuthCodeRepo()

		mp := &mockProvider{
			name: "discord",
			token: &oauth2.Token{
				AccessToken:  "access-token-123",
				RefreshToken: "refresh-token-456",
				Expiry:       time.Now().Add(time.Hour),
			},
			userInfo: &provider.UserInfo{
				ProviderUserID: "discord-user-123",
				Username:       "testuser",
				DisplayName:    "Test User",
				AvatarURL:      "https://cdn.discordapp.com/avatars/123/abc.png",
			},
		}

		svc := newTestService(userRepo, stateRepo, codeRepo, map[string]provider.OAuthProvider{"discord": mp})

		// Set up a valid state.
		stateRepo.states["valid-state"] = &domain.OAuthState{
			State:       "valid-state",
			Provider:    "discord",
			RedirectURI: "https://app.example.com/callback",
			ClientState: "csrf-token",
			ExpiresAt:   time.Now().Add(10 * time.Minute),
		}

		authCode, redirectURI, clientState, user, err := svc.HandleProviderCallback(ctx, "discord", "auth-code", "valid-state")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if authCode == "" {
			t.Fatal("expected non-empty auth code")
		}
		if redirectURI != "https://app.example.com/callback" {
			t.Fatalf("expected redirect_uri, got %s", redirectURI)
		}
		if clientState != "csrf-token" {
			t.Fatalf("expected client_state, got %s", clientState)
		}
		if user == nil {
			t.Fatal("expected non-nil user")
		}
		if user.DisplayName != "Test User" {
			t.Fatalf("expected display name 'Test User', got %s", user.DisplayName)
		}

		// State should be consumed.
		if len(stateRepo.states) != 0 {
			t.Fatal("expected state to be consumed")
		}

		// Auth code should be stored.
		if len(codeRepo.codes) != 1 {
			t.Fatalf("expected 1 auth code, got %d", len(codeRepo.codes))
		}
	})

	t.Run("invalid state", func(t *testing.T) {
		userRepo := newMockUserRepo()
		stateRepo := newMockStateRepo()
		codeRepo := newMockAuthCodeRepo()
		mp := &mockProvider{name: "discord"}
		svc := newTestService(userRepo, stateRepo, codeRepo, map[string]provider.OAuthProvider{"discord": mp})

		_, _, _, _, err := svc.HandleProviderCallback(ctx, "discord", "code", "nonexistent-state")
		if err == nil {
			t.Fatal("expected error for invalid state")
		}
		if !errors.Is(err, domain.ErrStateNotFound) {
			t.Fatalf("expected ErrStateNotFound, got: %v", err)
		}
	})

	t.Run("provider mismatch", func(t *testing.T) {
		userRepo := newMockUserRepo()
		stateRepo := newMockStateRepo()
		codeRepo := newMockAuthCodeRepo()
		mp := &mockProvider{name: "discord"}
		svc := newTestService(userRepo, stateRepo, codeRepo, map[string]provider.OAuthProvider{"discord": mp})

		stateRepo.states["state-123"] = &domain.OAuthState{
			State:     "state-123",
			Provider:  "google", // Mismatch!
			ExpiresAt: time.Now().Add(10 * time.Minute),
		}

		_, _, _, _, err := svc.HandleProviderCallback(ctx, "discord", "code", "state-123")
		if err == nil {
			t.Fatal("expected error for provider mismatch")
		}
	})
}

func TestIssueAuthCode(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		userRepo := newMockUserRepo()
		stateRepo := newMockStateRepo()
		codeRepo := newMockAuthCodeRepo()
		svc := newTestService(userRepo, stateRepo, codeRepo, nil)

		// Create a user.
		userID := uuid.New()
		userRepo.users[userID] = &domain.User{ID: userID, DisplayName: "Test User"}

		code, err := svc.IssueAuthCode(ctx, userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code == "" {
			t.Fatal("expected non-empty auth code")
		}
		if len(codeRepo.codes) != 1 {
			t.Fatalf("expected 1 auth code, got %d", len(codeRepo.codes))
		}
	})

	t.Run("user not found", func(t *testing.T) {
		userRepo := newMockUserRepo()
		stateRepo := newMockStateRepo()
		codeRepo := newMockAuthCodeRepo()
		svc := newTestService(userRepo, stateRepo, codeRepo, nil)

		_, err := svc.IssueAuthCode(ctx, uuid.New())
		if err == nil {
			t.Fatal("expected error for non-existent user")
		}
	})
}

func TestExchangeToken(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		userRepo := newMockUserRepo()
		stateRepo := newMockStateRepo()
		codeRepo := newMockAuthCodeRepo()
		svc := newTestService(userRepo, stateRepo, codeRepo, nil)

		userID := uuid.New()
		userRepo.users[userID] = &domain.User{ID: userID, DisplayName: "Test User"}

		codeRepo.codes["test-code"] = &domain.AuthCode{
			Code:      "test-code",
			UserID:    userID,
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}

		user, err := svc.ExchangeToken(ctx, "test-code")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.ID != userID {
			t.Fatalf("expected user ID %s, got %s", userID, user.ID)
		}
	})

	t.Run("invalid code", func(t *testing.T) {
		userRepo := newMockUserRepo()
		stateRepo := newMockStateRepo()
		codeRepo := newMockAuthCodeRepo()
		svc := newTestService(userRepo, stateRepo, codeRepo, nil)

		_, err := svc.ExchangeToken(ctx, "nonexistent-code")
		if err == nil {
			t.Fatal("expected error for invalid code")
		}
	})

	t.Run("already used code", func(t *testing.T) {
		userRepo := newMockUserRepo()
		stateRepo := newMockStateRepo()
		codeRepo := newMockAuthCodeRepo()
		svc := newTestService(userRepo, stateRepo, codeRepo, nil)

		userID := uuid.New()
		userRepo.users[userID] = &domain.User{ID: userID}

		codeRepo.codes["used-code"] = &domain.AuthCode{
			Code:      "used-code",
			UserID:    userID,
			Used:      true,
			ExpiresAt: time.Now().Add(5 * time.Minute),
		}

		_, err := svc.ExchangeToken(ctx, "used-code")
		if err == nil {
			t.Fatal("expected error for already used code")
		}
	})
}

func TestGetUser(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		userRepo := newMockUserRepo()
		svc := newTestService(userRepo, newMockStateRepo(), newMockAuthCodeRepo(), nil)

		userID := uuid.New()
		userRepo.users[userID] = &domain.User{ID: userID, DisplayName: "Test User"}

		user, err := svc.GetUser(ctx, userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.DisplayName != "Test User" {
			t.Fatalf("expected 'Test User', got %s", user.DisplayName)
		}
	})

	t.Run("not found", func(t *testing.T) {
		userRepo := newMockUserRepo()
		svc := newTestService(userRepo, newMockStateRepo(), newMockAuthCodeRepo(), nil)

		_, err := svc.GetUser(ctx, uuid.New())
		if err == nil {
			t.Fatal("expected error for non-existent user")
		}
	})
}
