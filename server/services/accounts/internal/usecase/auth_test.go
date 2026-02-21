package usecase_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"
)

// --- In-memory fakes ---

type fakeUserRepo struct {
	mu    sync.Mutex
	users map[string]*domain.User
	// provider+providerID -> userID
	identities map[string]string
	nextID     int
}

func newFakeUserRepo() *fakeUserRepo {
	return &fakeUserRepo{
		users:      make(map[string]*domain.User),
		identities: make(map[string]string),
	}
}

func (r *fakeUserRepo) UpsertByIdentity(_ context.Context, provider string, info *domain.ProviderUserInfo, _, _ string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := provider + ":" + info.ProviderID
	now := time.Now()

	if userID, ok := r.identities[key]; ok {
		u := r.users[userID]
		u.DisplayName = info.DisplayName
		u.AvatarURL = info.AvatarURL
		u.UpdatedAt = now
		return u, nil
	}

	r.nextID++
	id := fmt.Sprintf("user-%d", r.nextID)
	u := &domain.User{
		ID:          id,
		DisplayName: info.DisplayName,
		AvatarURL:   info.AvatarURL,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	r.users[id] = u
	r.identities[key] = id
	return u, nil
}

func (r *fakeUserRepo) GetByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	u, ok := r.users[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return u, nil
}

type fakeAuthCodeRepo struct {
	mu    sync.Mutex
	codes map[string]*domain.AuthCode
}

func newFakeAuthCodeRepo() *fakeAuthCodeRepo {
	return &fakeAuthCodeRepo{codes: make(map[string]*domain.AuthCode)}
}

func (r *fakeAuthCodeRepo) Create(_ context.Context, code *domain.AuthCode) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.codes[code.Code] = code
	return nil
}

func (r *fakeAuthCodeRepo) Consume(_ context.Context, code string) (*domain.AuthCode, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ac, ok := r.codes[code]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if ac.Used {
		return nil, domain.ErrAlreadyUsed
	}
	if time.Now().After(ac.ExpiresAt) {
		return nil, domain.ErrExpired
	}
	ac.Used = true
	return ac, nil
}

type fakeStateRepo struct {
	mu     sync.Mutex
	states map[string]*domain.OAuthState
}

func newFakeStateRepo() *fakeStateRepo {
	return &fakeStateRepo{states: make(map[string]*domain.OAuthState)}
}

func (r *fakeStateRepo) Create(_ context.Context, state *domain.OAuthState) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.states[state.State] = state
	return nil
}

func (r *fakeStateRepo) Consume(_ context.Context, state string) (*domain.OAuthState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.states[state]
	if !ok {
		return nil, domain.ErrNotFound
	}
	if time.Now().After(s.ExpiresAt) {
		delete(r.states, state)
		return nil, domain.ErrExpired
	}
	delete(r.states, state)
	return s, nil
}

type fakeOAuthProvider struct {
	name     string
	userInfo *domain.ProviderUserInfo
	err      error
}

func (p *fakeOAuthProvider) Name() string { return p.name }

func (p *fakeOAuthProvider) AuthURL(state string) string {
	return "https://discord.com/oauth2/authorize?state=" + state
}

func (p *fakeOAuthProvider) Exchange(_ context.Context, _ string) (*domain.ProviderUserInfo, string, string, error) {
	if p.err != nil {
		return nil, "", "", p.err
	}
	return p.userInfo, "access-token", "refresh-token", nil
}

// --- Tests ---

func newTestUsecase(provider *fakeOAuthProvider) (*usecase.AuthUsecase, *fakeUserRepo, *fakeAuthCodeRepo, *fakeStateRepo) {
	users := newFakeUserRepo()
	authCodes := newFakeAuthCodeRepo()
	states := newFakeStateRepo()
	uc := usecase.NewAuthUsecase(users, authCodes, states, []domain.OAuthProvider{provider})
	return uc, users, authCodes, states
}

func TestGetAuthURL(t *testing.T) {
	provider := &fakeOAuthProvider{name: "discord"}
	uc, _, _, states := newTestUsecase(provider)

	authURL, state, err := uc.GetAuthURL(context.Background(), "discord", "https://drive.example.com/callback", "client-csrf")
	if err != nil {
		t.Fatalf("GetAuthURL: %v", err)
	}

	if authURL == "" {
		t.Fatal("expected non-empty auth URL")
	}
	if state == "" {
		t.Fatal("expected non-empty state")
	}

	// Verify the state was persisted.
	states.mu.Lock()
	saved, ok := states.states[state]
	states.mu.Unlock()
	if !ok {
		t.Fatal("state was not persisted")
	}
	if saved.Provider != "discord" {
		t.Errorf("state provider = %q, want %q", saved.Provider, "discord")
	}
	if saved.RedirectURI != "https://drive.example.com/callback" {
		t.Errorf("state redirect_uri = %q, want %q", saved.RedirectURI, "https://drive.example.com/callback")
	}
	if saved.ClientState != "client-csrf" {
		t.Errorf("state client_state = %q, want %q", saved.ClientState, "client-csrf")
	}
}

func TestGetAuthURL_UnsupportedProvider(t *testing.T) {
	provider := &fakeOAuthProvider{name: "discord"}
	uc, _, _, _ := newTestUsecase(provider)

	_, _, err := uc.GetAuthURL(context.Background(), "github", "https://example.com", "")
	if err == nil {
		t.Fatal("expected error for unsupported provider")
	}
}

func TestHandleProviderCallback_Success(t *testing.T) {
	provider := &fakeOAuthProvider{
		name: "discord",
		userInfo: &domain.ProviderUserInfo{
			ProviderID:  "12345",
			Email:       "user@example.com",
			DisplayName: "TestUser",
			AvatarURL:   "https://cdn.discordapp.com/avatars/12345/abc.png",
		},
	}
	uc, _, _, _ := newTestUsecase(provider)
	ctx := context.Background()

	// First, get an auth URL to create a state entry.
	_, state, err := uc.GetAuthURL(ctx, "discord", "https://drive.example.com/callback", "my-state")
	if err != nil {
		t.Fatalf("GetAuthURL: %v", err)
	}

	// Now handle the callback.
	authCode, redirectURI, clientState, user, err := uc.HandleProviderCallback(ctx, "discord", "discord-auth-code", state)
	if err != nil {
		t.Fatalf("HandleProviderCallback: %v", err)
	}

	if authCode == "" {
		t.Fatal("expected non-empty auth code")
	}
	if redirectURI != "https://drive.example.com/callback" {
		t.Errorf("redirect_uri = %q, want %q", redirectURI, "https://drive.example.com/callback")
	}
	if clientState != "my-state" {
		t.Errorf("client_state = %q, want %q", clientState, "my-state")
	}
	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.DisplayName != "TestUser" {
		t.Errorf("user.DisplayName = %q, want %q", user.DisplayName, "TestUser")
	}
}

func TestHandleProviderCallback_InvalidState(t *testing.T) {
	provider := &fakeOAuthProvider{name: "discord", userInfo: &domain.ProviderUserInfo{ProviderID: "1"}}
	uc, _, _, _ := newTestUsecase(provider)

	_, _, _, _, err := uc.HandleProviderCallback(context.Background(), "discord", "code", "nonexistent-state")
	if err == nil {
		t.Fatal("expected error for invalid state")
	}
}

func TestHandleProviderCallback_ProviderExchangeError(t *testing.T) {
	provider := &fakeOAuthProvider{
		name: "discord",
		err:  fmt.Errorf("discord is down"),
	}
	uc, _, _, _ := newTestUsecase(provider)
	ctx := context.Background()

	_, state, _ := uc.GetAuthURL(ctx, "discord", "https://example.com", "")

	_, _, _, _, err := uc.HandleProviderCallback(ctx, "discord", "code", state)
	if err == nil {
		t.Fatal("expected error when provider exchange fails")
	}
}

func TestExchangeToken_Success(t *testing.T) {
	provider := &fakeOAuthProvider{
		name: "discord",
		userInfo: &domain.ProviderUserInfo{
			ProviderID:  "99",
			DisplayName: "ExchangeUser",
		},
	}
	uc, _, _, _ := newTestUsecase(provider)
	ctx := context.Background()

	// Full flow: get URL -> callback -> exchange.
	_, state, _ := uc.GetAuthURL(ctx, "discord", "https://example.com", "")
	authCode, _, _, _, err := uc.HandleProviderCallback(ctx, "discord", "code", state)
	if err != nil {
		t.Fatalf("HandleProviderCallback: %v", err)
	}

	user, err := uc.ExchangeToken(ctx, authCode)
	if err != nil {
		t.Fatalf("ExchangeToken: %v", err)
	}
	if user.DisplayName != "ExchangeUser" {
		t.Errorf("user.DisplayName = %q, want %q", user.DisplayName, "ExchangeUser")
	}
}

func TestExchangeToken_CodeAlreadyUsed(t *testing.T) {
	provider := &fakeOAuthProvider{
		name:     "discord",
		userInfo: &domain.ProviderUserInfo{ProviderID: "1", DisplayName: "u"},
	}
	uc, _, _, _ := newTestUsecase(provider)
	ctx := context.Background()

	_, state, _ := uc.GetAuthURL(ctx, "discord", "https://example.com", "")
	authCode, _, _, _, _ := uc.HandleProviderCallback(ctx, "discord", "code", state)

	// First exchange should succeed.
	_, err := uc.ExchangeToken(ctx, authCode)
	if err != nil {
		t.Fatalf("first ExchangeToken: %v", err)
	}

	// Second exchange should fail (code already used).
	_, err = uc.ExchangeToken(ctx, authCode)
	if err == nil {
		t.Fatal("expected error for already-used auth code")
	}
}

func TestExchangeToken_InvalidCode(t *testing.T) {
	provider := &fakeOAuthProvider{name: "discord"}
	uc, _, _, _ := newTestUsecase(provider)

	_, err := uc.ExchangeToken(context.Background(), "nonexistent-code")
	if err == nil {
		t.Fatal("expected error for invalid auth code")
	}
}

func TestGetUser_Success(t *testing.T) {
	provider := &fakeOAuthProvider{
		name: "discord",
		userInfo: &domain.ProviderUserInfo{
			ProviderID:  "42",
			DisplayName: "GetUserTest",
		},
	}
	uc, _, _, _ := newTestUsecase(provider)
	ctx := context.Background()

	// Create a user via the callback flow.
	_, state, _ := uc.GetAuthURL(ctx, "discord", "https://example.com", "")
	_, _, _, user, _ := uc.HandleProviderCallback(ctx, "discord", "code", state)

	found, err := uc.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if found.DisplayName != "GetUserTest" {
		t.Errorf("user.DisplayName = %q, want %q", found.DisplayName, "GetUserTest")
	}
}

func TestGetUser_NotFound(t *testing.T) {
	provider := &fakeOAuthProvider{name: "discord"}
	uc, _, _, _ := newTestUsecase(provider)

	_, err := uc.GetUser(context.Background(), "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
}

func TestUpsertByIdentity_UpdateExistingUser(t *testing.T) {
	provider := &fakeOAuthProvider{
		name: "discord",
		userInfo: &domain.ProviderUserInfo{
			ProviderID:  "100",
			DisplayName: "OriginalName",
			AvatarURL:   "https://cdn.example.com/old.png",
		},
	}
	uc, _, _, _ := newTestUsecase(provider)
	ctx := context.Background()

	// First login creates the user.
	_, state1, _ := uc.GetAuthURL(ctx, "discord", "https://example.com", "")
	_, _, _, user1, _ := uc.HandleProviderCallback(ctx, "discord", "code1", state1)

	// Second login with updated profile.
	provider.userInfo = &domain.ProviderUserInfo{
		ProviderID:  "100",
		DisplayName: "UpdatedName",
		AvatarURL:   "https://cdn.example.com/new.png",
	}

	_, state2, _ := uc.GetAuthURL(ctx, "discord", "https://example.com", "")
	_, _, _, user2, _ := uc.HandleProviderCallback(ctx, "discord", "code2", state2)

	// Should be the same internal user with updated profile.
	if user2.ID != user1.ID {
		t.Errorf("expected same user ID, got %q and %q", user1.ID, user2.ID)
	}
	if user2.DisplayName != "UpdatedName" {
		t.Errorf("user.DisplayName = %q, want %q", user2.DisplayName, "UpdatedName")
	}
}
