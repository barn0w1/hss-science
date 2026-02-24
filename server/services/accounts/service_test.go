package accounts_test

import (
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	accounts "github.com/barn0w1/hss-science/server/services/accounts"
)

// mockRepository implements accounts.UserRepository for testing.
type mockRepository struct {
	upsertUserFn    func(ctx context.Context, googleID, email, name, picture string) (string, error)
	createSessionFn func(ctx context.Context, userID, deviceIP, deviceUA string, expiresAt time.Time) (string, error)
}

func (m *mockRepository) UpsertUser(ctx context.Context, googleID, email, name, picture string) (string, error) {
	return m.upsertUserFn(ctx, googleID, email, name, picture)
}

func (m *mockRepository) CreateSession(ctx context.Context, userID, deviceIP, deviceUA string, expiresAt time.Time) (string, error) {
	return m.createSessionFn(ctx, userID, deviceIP, deviceUA, expiresAt)
}

func testJWTMinter(t *testing.T) (*accounts.JWTMinter, ed25519.PublicKey) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}

	pemBlock := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	tmpFile, err := os.CreateTemp(t.TempDir(), "test_key_*.pem")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmpFile.Write(pemBlock); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	minter, err := accounts.NewJWTMinter(tmpFile.Name(), "test-issuer", 15*time.Minute, 168*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	return minter, pub
}

func TestLoginUser_Success(t *testing.T) {
	minter, pubKey := testJWTMinter(t)
	repo := &mockRepository{
		upsertUserFn: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "usr_test-123", nil
		},
		createSessionFn: func(_ context.Context, _, _, _ string, _ time.Time) (string, error) {
			return "sess_test-456", nil
		},
	}

	svc := accounts.NewService(repo, minter, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	resp, err := svc.LoginUser(context.Background(), &accountsv1.LoginUserRequest{
		GoogleId: "google-sub-123",
		Email:    "test@example.com",
		Name:     "Test User",
		Picture:  "https://example.com/photo.jpg",
		DeviceInfo: &accountsv1.DeviceInfo{
			IpAddress: "127.0.0.1",
			UserAgent: "TestAgent/1.0",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.UserId != "usr_test-123" {
		t.Errorf("expected user_id usr_test-123, got %s", resp.UserId)
	}

	// Verify access token is a valid JWT.
	accessToken, err := jwt.ParseWithClaims(resp.AccessToken, &accounts.AccessTokenClaims{}, func(token *jwt.Token) (any, error) {
		return pubKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse access token: %v", err)
	}
	accessClaims := accessToken.Claims.(*accounts.AccessTokenClaims)
	if accessClaims.Subject != "usr_test-123" {
		t.Errorf("access token sub = %s, want usr_test-123", accessClaims.Subject)
	}
	if accessClaims.Email != "test@example.com" {
		t.Errorf("access token email = %s, want test@example.com", accessClaims.Email)
	}

	// Verify refresh token is a valid JWT.
	refreshToken, err := jwt.ParseWithClaims(resp.RefreshToken, &accounts.RefreshTokenClaims{}, func(token *jwt.Token) (any, error) {
		return pubKey, nil
	})
	if err != nil {
		t.Fatalf("failed to parse refresh token: %v", err)
	}
	refreshClaims := refreshToken.Claims.(*accounts.RefreshTokenClaims)
	if refreshClaims.Subject != "usr_test-123" {
		t.Errorf("refresh token sub = %s, want usr_test-123", refreshClaims.Subject)
	}
	if refreshClaims.SessionID != "sess_test-456" {
		t.Errorf("refresh token sid = %s, want sess_test-456", refreshClaims.SessionID)
	}
}

func TestLoginUser_MissingGoogleID(t *testing.T) {
	minter, _ := testJWTMinter(t)
	repo := &mockRepository{}
	svc := accounts.NewService(repo, minter, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	_, err := svc.LoginUser(context.Background(), &accountsv1.LoginUserRequest{
		Email: "test@example.com",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", st.Code())
	}
}

func TestLoginUser_MissingEmail(t *testing.T) {
	minter, _ := testJWTMinter(t)
	repo := &mockRepository{}
	svc := accounts.NewService(repo, minter, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	_, err := svc.LoginUser(context.Background(), &accountsv1.LoginUserRequest{
		GoogleId: "google-sub-123",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("expected InvalidArgument, got %s", st.Code())
	}
}

func TestLoginUser_DBError(t *testing.T) {
	minter, _ := testJWTMinter(t)
	repo := &mockRepository{
		upsertUserFn: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "", fmt.Errorf("connection refused")
		},
	}
	svc := accounts.NewService(repo, minter, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	_, err := svc.LoginUser(context.Background(), &accountsv1.LoginUserRequest{
		GoogleId: "google-sub-123",
		Email:    "test@example.com",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %v", err)
	}
	if st.Code() != codes.Internal {
		t.Errorf("expected Internal, got %s", st.Code())
	}
}

func TestLoginUser_UserIDPrefix(t *testing.T) {
	minter, _ := testJWTMinter(t)

	var capturedUserID string
	repo := &mockRepository{
		upsertUserFn: func(_ context.Context, _, _, _, _ string) (string, error) {
			return "usr_abc-def", nil
		},
		createSessionFn: func(_ context.Context, userID, _, _ string, _ time.Time) (string, error) {
			capturedUserID = userID
			return "sess_xyz", nil
		},
	}
	svc := accounts.NewService(repo, minter, slog.New(slog.NewTextHandler(os.Stderr, nil)))

	resp, err := svc.LoginUser(context.Background(), &accountsv1.LoginUserRequest{
		GoogleId: "g123",
		Email:    "u@x.com",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(resp.UserId, "usr_") {
		t.Errorf("user_id %q does not have usr_ prefix", resp.UserId)
	}
	if capturedUserID != "usr_abc-def" {
		t.Errorf("session created for wrong user: %s", capturedUserID)
	}
}
