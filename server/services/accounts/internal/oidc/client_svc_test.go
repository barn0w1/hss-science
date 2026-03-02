package oidc

import (
	"context"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/domerr"
)

type mockClientRepo struct {
	client *Client
	err    error
}

func (m *mockClientRepo) GetByID(_ context.Context, _ string) (*Client, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.client, nil
}

func hashPassword(t *testing.T, password string) string {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func TestClientService_ClientCredentials_Success(t *testing.T) {
	c := &Client{ID: "client-1", SecretHash: hashPassword(t, "secret")}
	svc := NewClientService(&mockClientRepo{client: c})
	got, err := svc.ClientCredentials(context.Background(), "client-1", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "client-1" {
		t.Errorf("expected client-1, got %s", got.ID)
	}
}

func TestClientService_ClientCredentials_WrongSecret(t *testing.T) {
	c := &Client{ID: "client-1", SecretHash: hashPassword(t, "secret")}
	svc := NewClientService(&mockClientRepo{client: c})
	_, err := svc.ClientCredentials(context.Background(), "client-1", "wrong")
	if !domerr.Is(err, domerr.ErrUnauthorized) {
		t.Errorf("expected domerr.ErrUnauthorized, got %v", err)
	}
}

func TestClientService_ClientCredentials_NotFound(t *testing.T) {
	svc := NewClientService(&mockClientRepo{err: domerr.ErrNotFound})
	_, err := svc.ClientCredentials(context.Background(), "nonexistent", "secret")
	if !domerr.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected domerr.ErrNotFound, got %v", err)
	}
}

func TestClientService_AuthorizeSecret_Success(t *testing.T) {
	c := &Client{ID: "client-1", SecretHash: hashPassword(t, "secret")}
	svc := NewClientService(&mockClientRepo{client: c})
	if err := svc.AuthorizeSecret(context.Background(), "client-1", "secret"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientService_AuthorizeSecret_WrongSecret(t *testing.T) {
	c := &Client{ID: "client-1", SecretHash: hashPassword(t, "secret")}
	svc := NewClientService(&mockClientRepo{client: c})
	err := svc.AuthorizeSecret(context.Background(), "client-1", "wrong")
	if !domerr.Is(err, domerr.ErrUnauthorized) {
		t.Errorf("expected domerr.ErrUnauthorized, got %v", err)
	}
}

func TestClientService_AuthorizeSecret_NotFound(t *testing.T) {
	svc := NewClientService(&mockClientRepo{err: domerr.ErrNotFound})
	err := svc.AuthorizeSecret(context.Background(), "nonexistent", "secret")
	if !domerr.Is(err, domerr.ErrNotFound) {
		t.Errorf("expected domerr.ErrNotFound, got %v", err)
	}
}

func TestClientService_GetByID(t *testing.T) {
	c := &Client{ID: "client-1"}
	svc := NewClientService(&mockClientRepo{client: c})
	got, err := svc.GetByID(context.Background(), "client-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "client-1" {
		t.Errorf("expected client-1, got %s", got.ID)
	}
}
