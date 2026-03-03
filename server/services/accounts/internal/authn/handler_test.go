package authn

import (
	"context"
	"crypto/rand"
	"html/template"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"golang.org/x/oauth2"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/crypto"
)

func testHandler(t *testing.T) *Handler {
	t.Helper()
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatal(err)
	}

	providers := []*Provider{
		{
			Name:        "test",
			DisplayName: "Test Provider",
			OAuth2Config: &oauth2.Config{
				ClientID:     "cid",
				ClientSecret: "csecret",
				RedirectURL:  "http://localhost/login/callback",
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://idp.example.com/authorize",
					TokenURL: "https://idp.example.com/token",
				},
				Scopes: []string{"openid"},
			},
		},
	}

	pm := make(map[string]*Provider, len(providers))
	for _, p := range providers {
		pm[p.Name] = p
	}

	tmpl := template.Must(template.New("select_provider").Parse(selectProviderHTML))
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	return &Handler{
		providers:   providers,
		providerMap: pm,
		loginUC:     nil,
		cipher:      crypto.NewAESCipher(key),
		callbackURL: func(_ context.Context, id string) string {
			return "http://localhost/authorize/callback?id=" + id
		},
		tmpl:   tmpl,
		logger: logger,
	}
}

func TestEncryptDecryptState_RoundTrip(t *testing.T) {
	h := testHandler(t)
	state := federatedState{
		AuthRequestID: "ar-123",
		Provider:      "google",
		Nonce:         "n0nc3",
	}

	encrypted, err := h.encryptState(state)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted == "" {
		t.Fatal("expected non-empty encrypted state")
	}

	decrypted, err := h.decryptState(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if decrypted.AuthRequestID != state.AuthRequestID {
		t.Errorf("expected auth request ID %s, got %s", state.AuthRequestID, decrypted.AuthRequestID)
	}
	if decrypted.Provider != state.Provider {
		t.Errorf("expected provider %s, got %s", state.Provider, decrypted.Provider)
	}
	if decrypted.Nonce != state.Nonce {
		t.Errorf("expected nonce %s, got %s", state.Nonce, decrypted.Nonce)
	}
}

func TestDecryptState_InvalidBase64(t *testing.T) {
	h := testHandler(t)
	_, err := h.decryptState("not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecryptState_TooShort(t *testing.T) {
	h := testHandler(t)
	_, err := h.decryptState("AQID")
	if err == nil {
		t.Fatal("expected error for ciphertext too short")
	}
}

func TestDecryptState_WrongKey(t *testing.T) {
	h := testHandler(t)
	state := federatedState{
		AuthRequestID: "ar-123",
		Provider:      "google",
		Nonce:         "n0nc3",
	}
	encrypted, err := h.encryptState(state)
	if err != nil {
		t.Fatal(err)
	}

	var otherKey [32]byte
	if _, err := rand.Read(otherKey[:]); err != nil {
		t.Fatal(err)
	}
	h.cipher = crypto.NewAESCipher(otherKey)
	_, err = h.decryptState(encrypted)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestFederatedRedirect_Success(t *testing.T) {
	h := testHandler(t)

	form := url.Values{}
	form.Set("authRequestID", "ar-123")
	form.Set("provider", "test")

	req := httptest.NewRequest(http.MethodPost, "/login/select", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.FederatedRedirect(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "https://idp.example.com/authorize") {
		t.Errorf("expected redirect to idp, got %s", loc)
	}
	if !strings.Contains(loc, "state=") {
		t.Error("expected state parameter in redirect URL")
	}
}

func TestFederatedRedirect_MissingParams(t *testing.T) {
	h := testHandler(t)

	tests := []struct {
		name   string
		values url.Values
	}{
		{"missing both", url.Values{}},
		{"missing provider", url.Values{"authRequestID": {"ar-123"}}},
		{"missing authRequestID", url.Values{"provider": {"test"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/login/select", strings.NewReader(tt.values.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()

			h.FederatedRedirect(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", rec.Code)
			}
		})
	}
}

func TestFederatedRedirect_UnknownProvider(t *testing.T) {
	h := testHandler(t)

	form := url.Values{}
	form.Set("authRequestID", "ar-123")
	form.Set("provider", "nonexistent")

	req := httptest.NewRequest(http.MethodPost, "/login/select", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	h.FederatedRedirect(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestFederatedCallback_MissingParams(t *testing.T) {
	h := testHandler(t)

	tests := []struct {
		name  string
		query string
	}{
		{"missing both", ""},
		{"missing state", "code=abc"},
		{"missing code", "state=xyz"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/login/callback?"+tt.query, nil)
			rec := httptest.NewRecorder()

			h.FederatedCallback(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", rec.Code)
			}
		})
	}
}

func TestFederatedCallback_InvalidState(t *testing.T) {
	h := testHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/login/callback?code=abc&state=invalid", nil)
	rec := httptest.NewRecorder()

	h.FederatedCallback(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestSelectProvider_MissingAuthRequestID(t *testing.T) {
	h := testHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()

	h.SelectProvider(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}
