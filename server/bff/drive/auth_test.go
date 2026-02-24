package drive_test

import (
	"context"
	"encoding/base64"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	drive "github.com/barn0w1/hss-science/server/bff/drive"
	driveapi "github.com/barn0w1/hss-science/server/bff/gen/drive/v1"
	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/gorilla/securecookie"
	"google.golang.org/grpc"
)

// mockTokenExchanger implements drive.TokenExchanger for testing.
type mockTokenExchanger struct {
	authURLFn  func(state, codeChallenge string) string
	exchangeFn func(ctx context.Context, code, codeVerifier string) (*drive.IDClaims, error)
}

func (m *mockTokenExchanger) AuthURL(state, codeChallenge string) string {
	if m.authURLFn != nil {
		return m.authURLFn(state, codeChallenge)
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?state=" + state
}

func (m *mockTokenExchanger) Exchange(ctx context.Context, code, codeVerifier string) (*drive.IDClaims, error) {
	if m.exchangeFn != nil {
		return m.exchangeFn(ctx, code, codeVerifier)
	}
	return &drive.IDClaims{
		Sub:     "google-sub-123",
		Email:   "test@example.com",
		Name:    "Test User",
		Picture: "https://example.com/photo.jpg",
	}, nil
}

// mockAccountsClient implements accountsv1.AccountsServiceClient for testing.
type mockAccountsClient struct {
	loginUserFn func(ctx context.Context, in *accountsv1.LoginUserRequest, opts ...grpc.CallOption) (*accountsv1.LoginUserResponse, error)
}

func (m *mockAccountsClient) LoginUser(ctx context.Context, in *accountsv1.LoginUserRequest, opts ...grpc.CallOption) (*accountsv1.LoginUserResponse, error) {
	if m.loginUserFn != nil {
		return m.loginUserFn(ctx, in, opts...)
	}
	return &accountsv1.LoginUserResponse{
		UserId:       "usr_test-123",
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
	}, nil
}

func testConfig() *drive.Config {
	hashKey := make([]byte, 32)
	blockKey := make([]byte, 32)
	for i := range hashKey {
		hashKey[i] = byte(i)
		blockKey[i] = byte(i + 32)
	}
	return &drive.Config{
		Env:                "development",
		Port:               "8080",
		GoogleClientID:     "test-client-id",
		GoogleClientSecret: "test-client-secret",
		GoogleRedirectURL:  "http://localhost:8080/api/v1/auth/callback",
		CookieHashKey:      hashKey,
		CookieBlockKey:     blockKey,
		CookieDomain:       "",
		PostLoginRedirect:  "http://localhost:5173/",
		AccountsGRPCAddr:   "localhost:50051",
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

func TestAuthLogin_RedirectsToGoogle(t *testing.T) {
	cfg := testConfig()
	mock := &mockTokenExchanger{}
	srv := drive.NewServer(cfg, testLogger(), mock, &mockAccountsClient{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
	w := httptest.NewRecorder()

	srv.AuthLogin(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if !strings.Contains(location, "accounts.google.com") {
		t.Errorf("expected redirect to Google, got: %s", location)
	}

	// Verify PKCE cookie was set.
	var pkceCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "__Secure-oauth_pkce" {
			pkceCookie = c
			break
		}
	}
	if pkceCookie == nil {
		t.Fatal("__Secure-oauth_pkce cookie not set")
	}
	if !pkceCookie.HttpOnly {
		t.Error("PKCE cookie should be HttpOnly")
	}
	if !pkceCookie.Secure {
		t.Error("PKCE cookie should be Secure")
	}
	if pkceCookie.Path != "/api/v1/auth/callback" {
		t.Errorf("PKCE cookie path = %s, want /api/v1/auth/callback", pkceCookie.Path)
	}
}

func TestAuthCallback_StateMismatch(t *testing.T) {
	cfg := testConfig()
	sc := securecookie.New(cfg.CookieHashKey, cfg.CookieBlockKey)

	encoded, err := sc.Encode("__Secure-oauth_pkce", drive.PKCEState{
		State:        "correct-state",
		CodeVerifier: "test-verifier",
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := drive.NewServer(cfg, testLogger(), &mockTokenExchanger{}, &mockAccountsClient{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback?code=testcode&state=wrong-state", nil)
	req.AddCookie(&http.Cookie{Name: "__Secure-oauth_pkce", Value: encoded})
	w := httptest.NewRecorder()

	params := driveapi.AuthCallbackParams{Code: "testcode", State: "wrong-state"}
	srv.AuthCallback(w, req, params)

	resp := w.Result()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuthCallback_Success(t *testing.T) {
	cfg := testConfig()
	sc := securecookie.New(cfg.CookieHashKey, cfg.CookieBlockKey)

	encoded, err := sc.Encode("__Secure-oauth_pkce", drive.PKCEState{
		State:        "valid-state",
		CodeVerifier: "test-verifier",
	})
	if err != nil {
		t.Fatal(err)
	}

	mock := &mockTokenExchanger{}
	acctsMock := &mockAccountsClient{}

	srv := drive.NewServer(cfg, testLogger(), mock, acctsMock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/callback?code=testcode&state=valid-state", nil)
	req.AddCookie(&http.Cookie{Name: "__Secure-oauth_pkce", Value: encoded})
	w := httptest.NewRecorder()

	params := driveapi.AuthCallbackParams{Code: "testcode", State: "valid-state"}
	srv.AuthCallback(w, req, params)

	resp := w.Result()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}

	location := resp.Header.Get("Location")
	if location != cfg.PostLoginRedirect {
		t.Errorf("redirect location = %s, want %s", location, cfg.PostLoginRedirect)
	}

	// Verify cookies.
	cookies := resp.Cookies()
	cookieMap := make(map[string]*http.Cookie)
	for _, c := range cookies {
		cookieMap[c.Name] = c
	}

	accessCookie, ok := cookieMap["__Secure-access_token"]
	if !ok {
		t.Fatal("__Secure-access_token cookie not set")
	}
	if accessCookie.Value != "test-access-token" {
		t.Errorf("access token = %s, want test-access-token", accessCookie.Value)
	}
	if accessCookie.Path != "/api/v1" {
		t.Errorf("access cookie path = %s, want /api/v1", accessCookie.Path)
	}
	if !accessCookie.HttpOnly || !accessCookie.Secure {
		t.Error("access cookie should be HttpOnly and Secure")
	}

	refreshCookie, ok := cookieMap["__Secure-refresh_token"]
	if !ok {
		t.Fatal("__Secure-refresh_token cookie not set")
	}
	if refreshCookie.Value != "test-refresh-token" {
		t.Errorf("refresh token = %s, want test-refresh-token", refreshCookie.Value)
	}
	if refreshCookie.Path != "/api/v1/auth" {
		t.Errorf("refresh cookie path = %s, want /api/v1/auth", refreshCookie.Path)
	}

	// Verify PKCE cookie was deleted.
	pkceCookie, ok := cookieMap["__Secure-oauth_pkce"]
	if !ok {
		t.Fatal("__Secure-oauth_pkce cookie deletion not set")
	}
	if pkceCookie.MaxAge != -1 {
		t.Errorf("PKCE cookie MaxAge = %d, want -1", pkceCookie.MaxAge)
	}
}

func TestGeneratePKCE(t *testing.T) {
	verifier, challenge, err := drive.GeneratePKCE()
	if err != nil {
		t.Fatal(err)
	}
	if verifier == "" || challenge == "" {
		t.Error("PKCE values should not be empty")
	}
	if verifier == challenge {
		t.Error("verifier and challenge should differ")
	}

	// Verify base64url encoding (no padding).
	if _, err := base64.RawURLEncoding.DecodeString(verifier); err != nil {
		t.Errorf("verifier is not valid base64url: %v", err)
	}
	if _, err := base64.RawURLEncoding.DecodeString(challenge); err != nil {
		t.Errorf("challenge is not valid base64url: %v", err)
	}
}

func TestGenerateState(t *testing.T) {
	state1, err := drive.GenerateState()
	if err != nil {
		t.Fatal(err)
	}
	state2, err := drive.GenerateState()
	if err != nil {
		t.Fatal(err)
	}
	if state1 == state2 {
		t.Error("consecutive states should differ")
	}
}
