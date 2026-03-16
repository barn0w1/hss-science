package oidc

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockTokenRepo struct {
	token        *Token
	refreshToken *RefreshToken
	userID       string
	tokenID      string
	err          error

	lastAccess              *Token
	lastRefresh             *RefreshToken
	lastCurrentRefreshToken string
}

func (m *mockTokenRepo) CreateAccess(_ context.Context, access *Token) error {
	m.lastAccess = access
	return m.err
}
func (m *mockTokenRepo) CreateAccessAndRefresh(_ context.Context, access *Token, refresh *RefreshToken, currentRefreshToken string) error {
	m.lastAccess = access
	m.lastRefresh = refresh
	m.lastCurrentRefreshToken = currentRefreshToken
	return m.err
}
func (m *mockTokenRepo) GetByID(_ context.Context, _ string) (*Token, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.token, nil
}
func (m *mockTokenRepo) GetRefreshToken(_ context.Context, _ string) (*RefreshToken, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.refreshToken, nil
}
func (m *mockTokenRepo) GetRefreshInfo(_ context.Context, _ string) (string, string, error) {
	return m.userID, m.tokenID, m.err
}
func (m *mockTokenRepo) DeleteByUserAndClient(_ context.Context, _, _ string) error { return m.err }
func (m *mockTokenRepo) Revoke(_ context.Context, _, _ string) error                { return m.err }
func (m *mockTokenRepo) RevokeRefreshToken(_ context.Context, _, _ string) error    { return m.err }
func (m *mockTokenRepo) DeleteExpired(_ context.Context, _ time.Time) (int64, int64, error) {
	return 0, 0, m.err
}
func (m *mockTokenRepo) GetLatestDeviceSessionID(_ context.Context, _, _ string) (string, error) {
	return "", m.err
}

func TestTokenService_GetLatestDeviceSessionID(t *testing.T) {
	t.Run("delegates to repo", func(t *testing.T) {
		svc := NewTokenService(&mockTokenRepo{})
		dsid, err := svc.GetLatestDeviceSessionID(context.Background(), "user-1", "client-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if dsid != "" {
			t.Errorf("expected empty string, got %q", dsid)
		}
	})

	t.Run("propagates repo error", func(t *testing.T) {
		svc := NewTokenService(&mockTokenRepo{err: errors.New("db error")})
		_, err := svc.GetLatestDeviceSessionID(context.Background(), "u", "c")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestTokenService_CreateAccess(t *testing.T) {
	repo := &mockTokenRepo{}
	svc := NewTokenService(repo)
	exp := time.Now().UTC().Add(15 * time.Minute)
	id, err := svc.CreateAccess(context.Background(), "client-1", "user-1", []string{"client-1"}, []string{"openid"}, exp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty token ID")
	}
	if repo.lastAccess == nil {
		t.Fatal("expected repo.CreateAccess to be called")
	}
	if repo.lastAccess.ID != id {
		t.Errorf("expected token ID %s in repo, got %s", id, repo.lastAccess.ID)
	}
}

func TestTokenService_CreateAccessAndRefresh(t *testing.T) {
	repo := &mockTokenRepo{}
	svc := NewTokenService(repo)
	accessExp := time.Now().UTC().Add(15 * time.Minute)
	refreshExp := time.Now().UTC().Add(7 * 24 * time.Hour)
	authTime := time.Now().UTC()

	accessID, refreshToken, err := svc.CreateAccessAndRefresh(context.Background(),
		"client-1", "user-1", []string{"client-1"}, []string{"openid"},
		accessExp, refreshExp, authTime, []string{"fed"}, "old-token", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if accessID == "" || refreshToken == "" {
		t.Fatal("expected non-empty IDs")
	}
	if repo.lastAccess == nil || repo.lastRefresh == nil {
		t.Fatal("expected repo.CreateAccessAndRefresh to be called")
	}
	if repo.lastAccess.RefreshTokenID != repo.lastRefresh.ID {
		t.Error("expected access.RefreshTokenID == refresh.ID")
	}
	if repo.lastRefresh.AccessTokenID != repo.lastAccess.ID {
		t.Error("expected refresh.AccessTokenID == access.ID")
	}
	if repo.lastRefresh.Token == refreshToken {
		t.Error("expected stored token to be a hash, not the raw value")
	}
	if repo.lastRefresh.Token != hashRefreshToken(refreshToken) {
		t.Error("stored token is not the expected SHA-256 hash")
	}
	if repo.lastCurrentRefreshToken != hashRefreshToken("old-token") {
		t.Errorf("expected hashed currentRefreshToken, got %s", repo.lastCurrentRefreshToken)
	}
}

func TestTokenService_CreateAccess_Error(t *testing.T) {
	repo := &mockTokenRepo{err: errors.New("db error")}
	svc := NewTokenService(repo)
	_, err := svc.CreateAccess(context.Background(), "c", "u", nil, nil, time.Now())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestTokenService_GetByID(t *testing.T) {
	tok := &Token{ID: "t-1", ClientID: "c-1"}
	svc := NewTokenService(&mockTokenRepo{token: tok})
	got, err := svc.GetByID(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "t-1" {
		t.Errorf("expected t-1, got %s", got.ID)
	}
}

func TestTokenService_GetRefreshToken(t *testing.T) {
	rt := &RefreshToken{ID: "rt-1", Token: "tok"}
	svc := NewTokenService(&mockTokenRepo{refreshToken: rt})
	got, err := svc.GetRefreshToken(context.Background(), "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "rt-1" {
		t.Errorf("expected rt-1, got %s", got.ID)
	}
}

func TestTokenService_GetRefreshInfo(t *testing.T) {
	svc := NewTokenService(&mockTokenRepo{userID: "u-1", tokenID: "t-1"})
	uid, tid, err := svc.GetRefreshInfo(context.Background(), "tok")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uid != "u-1" {
		t.Errorf("expected u-1, got %s", uid)
	}
	if tid != "t-1" {
		t.Errorf("expected t-1, got %s", tid)
	}
}
