package identity

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

type mockRepo struct {
	getByIDFn                       func(ctx context.Context, id string) (*User, error)
	findByFederatedIdentityFn       func(ctx context.Context, provider, providerSubject string) (*User, error)
	createWithFederatedIdentityFn   func(ctx context.Context, user *User, fi *FederatedIdentity) error
	updateUserFromClaimsFn          func(ctx context.Context, userID string, claims FederatedClaims, updatedAt time.Time) error
	updateFederatedIdentityClaimsFn func(ctx context.Context, provider, providerSubject string, claims FederatedClaims, lastLoginAt time.Time) error
	listFederatedIdentitiesFn       func(ctx context.Context, userID string) ([]*FederatedIdentity, error)
	deleteFederatedIdentityFn       func(ctx context.Context, id, userID string) error
	updateLocalProfileFn            func(ctx context.Context, userID string, name, picture *string, updatedAt time.Time) error
}

func (m *mockRepo) GetByID(ctx context.Context, id string) (*User, error) {
	return m.getByIDFn(ctx, id)
}
func (m *mockRepo) FindByFederatedIdentity(ctx context.Context, provider, providerSubject string) (*User, error) {
	return m.findByFederatedIdentityFn(ctx, provider, providerSubject)
}
func (m *mockRepo) CreateWithFederatedIdentity(ctx context.Context, user *User, fi *FederatedIdentity) error {
	return m.createWithFederatedIdentityFn(ctx, user, fi)
}
func (m *mockRepo) UpdateUserFromClaims(ctx context.Context, userID string, claims FederatedClaims, updatedAt time.Time) error {
	return m.updateUserFromClaimsFn(ctx, userID, claims, updatedAt)
}
func (m *mockRepo) UpdateFederatedIdentityClaims(ctx context.Context, provider, providerSubject string, claims FederatedClaims, lastLoginAt time.Time) error {
	return m.updateFederatedIdentityClaimsFn(ctx, provider, providerSubject, claims, lastLoginAt)
}
func (m *mockRepo) ListFederatedIdentities(ctx context.Context, userID string) ([]*FederatedIdentity, error) {
	if m.listFederatedIdentitiesFn != nil {
		return m.listFederatedIdentitiesFn(ctx, userID)
	}
	return nil, nil
}
func (m *mockRepo) DeleteFederatedIdentity(ctx context.Context, id, userID string) error {
	if m.deleteFederatedIdentityFn != nil {
		return m.deleteFederatedIdentityFn(ctx, id, userID)
	}
	return nil
}
func (m *mockRepo) UpdateLocalProfile(ctx context.Context, userID string, name, picture *string, updatedAt time.Time) error {
	if m.updateLocalProfileFn != nil {
		return m.updateLocalProfileFn(ctx, userID, name, picture, updatedAt)
	}
	return nil
}

func TestGetUser_Found(t *testing.T) {
	want := &User{ID: "u1", Email: "a@b.com", Name: "Alice"}
	repo := &mockRepo{
		getByIDFn: func(_ context.Context, id string) (*User, error) {
			if id == "u1" {
				return want, nil
			}
			return nil, domerr.ErrNotFound
		},
	}
	svc := NewService(repo)
	got, err := svc.GetUser(context.Background(), "u1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("expected ID %s, got %s", want.ID, got.ID)
	}
}

func TestGetUser_NotFound(t *testing.T) {
	repo := &mockRepo{
		getByIDFn: func(_ context.Context, _ string) (*User, error) {
			return nil, domerr.ErrNotFound
		},
	}
	svc := NewService(repo)
	_, err := svc.GetUser(context.Background(), "nonexistent")
	if !errors.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFindOrCreate_ExistingUser(t *testing.T) {
	existing := &User{ID: "u1", Email: "a@b.com"}
	var updateUserCalled bool
	var updateClaimsCalled bool
	repo := &mockRepo{
		findByFederatedIdentityFn: func(_ context.Context, _, _ string) (*User, error) {
			return existing, nil
		},
		updateUserFromClaimsFn: func(_ context.Context, _ string, _ FederatedClaims, _ time.Time) error {
			updateUserCalled = true
			return nil
		},
		updateFederatedIdentityClaimsFn: func(_ context.Context, _, _ string, _ FederatedClaims, _ time.Time) error {
			updateClaimsCalled = true
			return nil
		},
		createWithFederatedIdentityFn: func(_ context.Context, _ *User, _ *FederatedIdentity) error {
			t.Fatal("CreateWithFederatedIdentity should not be called for existing user")
			return nil
		},
	}

	svc := NewService(repo)
	claims := FederatedClaims{Subject: "sub1", Email: "new@b.com"}
	got, err := svc.FindOrCreateByFederatedLogin(context.Background(), "google", claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != existing.ID {
		t.Errorf("expected ID %s, got %s", existing.ID, got.ID)
	}
	if got.Email != "new@b.com" {
		t.Errorf("expected email new@b.com, got %s", got.Email)
	}
	if !updateUserCalled {
		t.Error("expected UpdateUserFromClaims to be called")
	}
	if !updateClaimsCalled {
		t.Error("expected UpdateFederatedIdentityClaims to be called")
	}
}

func TestFindOrCreate_NewUser(t *testing.T) {
	var createdUser *User
	var createdFI *FederatedIdentity
	repo := &mockRepo{
		findByFederatedIdentityFn: func(_ context.Context, _, _ string) (*User, error) {
			return nil, nil
		},
		createWithFederatedIdentityFn: func(_ context.Context, u *User, fi *FederatedIdentity) error {
			createdUser = u
			createdFI = fi
			return nil
		},
		updateFederatedIdentityClaimsFn: func(_ context.Context, _, _ string, _ FederatedClaims, _ time.Time) error {
			t.Fatal("UpdateFederatedIdentityClaims should not be called for new user")
			return nil
		},
	}

	svc := NewService(repo)
	claims := FederatedClaims{
		Subject:       "sub1",
		Email:         "alice@example.com",
		EmailVerified: true,
		Name:          "Alice Smith",
		GivenName:     "Alice",
		FamilyName:    "Smith",
		Picture:       "https://example.com/pic.jpg",
	}
	got, err := svc.FindOrCreateByFederatedLogin(context.Background(), "google", claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID == "" {
		t.Fatal("expected non-empty user ID")
	}
	if got.Email != "alice@example.com" {
		t.Errorf("expected email alice@example.com, got %s", got.Email)
	}
	if got.Name != "Alice Smith" {
		t.Errorf("expected name Alice Smith, got %s", got.Name)
	}
	if createdUser == nil {
		t.Fatal("expected user to be created")
	}
	if createdFI == nil {
		t.Fatal("expected federated identity to be created")
	}
	if createdFI.UserID != createdUser.ID {
		t.Errorf("FI.UserID %s != User.ID %s", createdFI.UserID, createdUser.ID)
	}
	if createdFI.Provider != "google" {
		t.Errorf("expected provider google, got %s", createdFI.Provider)
	}
	if createdFI.ProviderEmail != "alice@example.com" {
		t.Errorf("expected ProviderEmail alice@example.com, got %s", createdFI.ProviderEmail)
	}
	if createdFI.ProviderDisplayName != "Alice Smith" {
		t.Errorf("expected ProviderDisplayName Alice Smith, got %s", createdFI.ProviderDisplayName)
	}
}

func TestFindOrCreate_LookupError(t *testing.T) {
	repo := &mockRepo{
		findByFederatedIdentityFn: func(_ context.Context, _, _ string) (*User, error) {
			return nil, fmt.Errorf("db down")
		},
	}
	svc := NewService(repo)
	_, err := svc.FindOrCreateByFederatedLogin(context.Background(), "google", FederatedClaims{Subject: "s"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFindOrCreate_CreateError(t *testing.T) {
	repo := &mockRepo{
		findByFederatedIdentityFn: func(_ context.Context, _, _ string) (*User, error) {
			return nil, nil
		},
		createWithFederatedIdentityFn: func(_ context.Context, _ *User, _ *FederatedIdentity) error {
			return fmt.Errorf("unique constraint violation")
		},
	}
	svc := NewService(repo)
	_, err := svc.FindOrCreateByFederatedLogin(context.Background(), "google", FederatedClaims{Subject: "s"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestFindOrCreate_UpdateClaimsError(t *testing.T) {
	existing := &User{ID: "u1"}
	repo := &mockRepo{
		findByFederatedIdentityFn: func(_ context.Context, _, _ string) (*User, error) {
			return existing, nil
		},
		updateUserFromClaimsFn: func(_ context.Context, _ string, _ FederatedClaims, _ time.Time) error {
			return nil
		},
		updateFederatedIdentityClaimsFn: func(_ context.Context, _, _ string, _ FederatedClaims, _ time.Time) error {
			return fmt.Errorf("db error")
		},
	}
	svc := NewService(repo)
	_, err := svc.FindOrCreateByFederatedLogin(context.Background(), "google", FederatedClaims{Subject: "s"})
	if err == nil {
		t.Fatal("expected error")
	}
}
