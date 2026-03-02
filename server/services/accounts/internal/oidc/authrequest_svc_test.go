package oidc

import (
	"context"
	"testing"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

type mockAuthRequestRepo struct {
	ar  *AuthRequest
	err error
}

func (m *mockAuthRequestRepo) Create(_ context.Context, ar *AuthRequest) error {
	m.ar = ar
	return m.err
}
func (m *mockAuthRequestRepo) GetByID(_ context.Context, _ string) (*AuthRequest, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.ar, nil
}
func (m *mockAuthRequestRepo) GetByCode(_ context.Context, _ string) (*AuthRequest, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.ar, nil
}
func (m *mockAuthRequestRepo) SaveCode(_ context.Context, _, _ string) error { return m.err }
func (m *mockAuthRequestRepo) CompleteLogin(_ context.Context, _, _ string, _ time.Time, _ []string) error {
	return m.err
}
func (m *mockAuthRequestRepo) Delete(_ context.Context, _ string) error { return m.err }

func TestAuthRequestService_GetByID_Valid(t *testing.T) {
	ar := &AuthRequest{
		ID:        "ar-1",
		CreatedAt: time.Now().UTC().Add(-10 * time.Minute),
	}
	svc := NewAuthRequestService(&mockAuthRequestRepo{ar: ar}, 30*time.Minute)
	got, err := svc.GetByID(context.Background(), "ar-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "ar-1" {
		t.Errorf("expected ar-1, got %s", got.ID)
	}
}

func TestAuthRequestService_GetByID_Expired(t *testing.T) {
	ar := &AuthRequest{
		ID:        "ar-1",
		CreatedAt: time.Now().UTC().Add(-31 * time.Minute),
	}
	svc := NewAuthRequestService(&mockAuthRequestRepo{ar: ar}, 30*time.Minute)
	_, err := svc.GetByID(context.Background(), "ar-1")
	if !domerr.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected domerr.ErrNotFound for expired request, got %v", err)
	}
}

func TestAuthRequestService_GetByCode_Valid(t *testing.T) {
	ar := &AuthRequest{
		ID:        "ar-1",
		Code:      "code-1",
		CreatedAt: time.Now().UTC().Add(-5 * time.Minute),
	}
	svc := NewAuthRequestService(&mockAuthRequestRepo{ar: ar}, 30*time.Minute)
	got, err := svc.GetByCode(context.Background(), "code-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "ar-1" {
		t.Errorf("expected ar-1, got %s", got.ID)
	}
}

func TestAuthRequestService_GetByCode_Expired(t *testing.T) {
	ar := &AuthRequest{
		ID:        "ar-1",
		Code:      "code-1",
		CreatedAt: time.Now().UTC().Add(-31 * time.Minute),
	}
	svc := NewAuthRequestService(&mockAuthRequestRepo{ar: ar}, 30*time.Minute)
	_, err := svc.GetByCode(context.Background(), "code-1")
	if !domerr.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected domerr.ErrNotFound for expired request, got %v", err)
	}
}

func TestAuthRequestService_GetByID_NotFound(t *testing.T) {
	svc := NewAuthRequestService(&mockAuthRequestRepo{err: domerr.ErrNotFound}, 30*time.Minute)
	_, err := svc.GetByID(context.Background(), "nonexistent")
	if !domerr.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected domerr.ErrNotFound, got %v", err)
	}
}

func TestAuthRequestService_Create(t *testing.T) {
	repo := &mockAuthRequestRepo{}
	svc := NewAuthRequestService(repo, 30*time.Minute)
	ar := &AuthRequest{ID: "ar-1", ClientID: "client-1"}
	if err := svc.Create(context.Background(), ar); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.ar.ID != "ar-1" {
		t.Errorf("expected repo to receive ar-1, got %s", repo.ar.ID)
	}
}

func TestAuthRequestService_CompleteLogin(t *testing.T) {
	svc := NewAuthRequestService(&mockAuthRequestRepo{}, 30*time.Minute)
	err := svc.CompleteLogin(context.Background(), "ar-1", "user-1", time.Now().UTC(), []string{"federated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
