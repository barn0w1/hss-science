package web

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/zitadel/oidc/v3/pkg/op"
	"golang.org/x/crypto/hkdf"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/authn"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/storage"
)

const (
	csrfCookieName = "oidc_csrf"
	queryAuthReqID = "authRequestID"
)

// providerLink holds the display label and destination URL for one identity provider
// on the login selection page.
type providerLink struct {
	Label string
	URL   string
}

// Login handles the authentication UI and upstream provider callbacks.
type Login struct {
	storage     *storage.PostgresStorage
	provider    authn.AuthnProvider
	opCallback  func(context.Context, string) string
	interceptor *op.IssuerInterceptor
	logger      *slog.Logger
	router      chi.Router
	stateKey    []byte
	devMode     bool
}

// deriveStateKey derives a dedicated HMAC key from the encryption key using HKDF-SHA256.
func deriveStateKey(encryptionKey [32]byte) ([]byte, error) {
	hkdfReader := hkdf.New(sha256.New, encryptionKey[:], nil, []byte("oidc-state-v1"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdfReader, key); err != nil {
		return nil, fmt.Errorf("derive state key: %w", err)
	}
	return key, nil
}

// NewLogin creates the login handler and configures routes.
func NewLogin(
	store *storage.PostgresStorage,
	provider authn.AuthnProvider,
	opCallback func(context.Context, string) string,
	interceptor *op.IssuerInterceptor,
	encryptionKey [32]byte,
	devMode bool,
	logger *slog.Logger,
) (*Login, error) {
	stateKey, err := deriveStateKey(encryptionKey)
	if err != nil {
		return nil, err
	}

	l := &Login{
		storage:     store,
		provider:    provider,
		opCallback:  opCallback,
		interceptor: interceptor,
		logger:      logger,
		stateKey:    stateKey,
		devMode:     devMode,
	}

	l.router = chi.NewRouter()
	l.router.Get("/select", l.selectHandler)
	l.router.Get("/google", l.googleLoginHandler)
	l.router.Get("/callback", interceptor.HandlerFunc(l.callbackHandler))
	return l, nil
}

// Router returns the chi router for mounting.
func (l *Login) Router() chi.Router {
	return l.router
}

// selectHandler renders the login provider selection page.
// It builds a list of available providers and passes them to the template so
// each provider can be added without touching HTML.
func (l *Login) selectHandler(w http.ResponseWriter, r *http.Request) {
	authRequestID := r.URL.Query().Get(queryAuthReqID)
	if authRequestID == "" {
		l.renderError(w, "missing auth request ID", http.StatusBadRequest)
		return
	}
	if _, err := uuid.Parse(authRequestID); err != nil {
		l.renderError(w, "invalid auth request ID", http.StatusBadRequest)
		return
	}

	data := struct {
		Providers []providerLink
	}{
		Providers: []providerLink{
			{
				Label: "Sign in with " + l.provider.Name(),
				URL:   "/login/google?authRequestID=" + authRequestID,
			},
		},
	}

	if err := templates.ExecuteTemplate(w, "login.html", data); err != nil {
		l.logger.Error("failed to render login template", "error", err)
		l.renderError(w, "internal error", http.StatusInternalServerError)
	}
}

// googleLoginHandler initiates the upstream Google OIDC flow.
// It encodes the auth request ID into the OAuth2 state parameter
// and sets a CSRF cookie for validation on callback.
func (l *Login) googleLoginHandler(w http.ResponseWriter, r *http.Request) {
	authRequestID := r.URL.Query().Get(queryAuthReqID)
	if authRequestID == "" {
		l.renderError(w, "missing auth request ID", http.StatusBadRequest)
		return
	}
	if _, err := uuid.Parse(authRequestID); err != nil {
		l.renderError(w, "invalid auth request ID", http.StatusBadRequest)
		return
	}

	// Generate CSRF token
	csrfToken, err := generateRandomString(32)
	if err != nil {
		l.renderError(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Encode authRequestID and CSRF token into HMAC-authenticated OAuth2 state
	state := encodeState(l.stateKey, authRequestID, csrfToken)

	// Set CSRF cookie
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/login/callback",
		MaxAge:   600,
		HttpOnly: true,
		Secure:   !l.devMode,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to Google
	http.Redirect(w, r, l.provider.AuthURL(state), http.StatusFound)
}

// callbackHandler processes the callback from Google after authentication.
func (l *Login) callbackHandler(w http.ResponseWriter, r *http.Request) {
	// Decode and verify HMAC-authenticated state
	state := r.URL.Query().Get("state")
	authRequestID, csrfToken, err := decodeState(l.stateKey, state)
	if err != nil {
		l.logger.Error("invalid state parameter", "error", err)
		l.renderError(w, "invalid state parameter", http.StatusBadRequest)
		return
	}

	// Validate CSRF
	cookie, err := r.Cookie(csrfCookieName)
	if err != nil || cookie.Value != csrfToken {
		l.logger.Error("CSRF validation failed")
		l.renderError(w, "CSRF validation failed", http.StatusForbidden)
		return
	}

	// Clear CSRF cookie
	http.SetCookie(w, &http.Cookie{
		Name:   csrfCookieName,
		Path:   "/login/callback",
		MaxAge: -1,
	})

	// Process upstream callback
	identity, err := l.provider.HandleCallback(r.Context(), r)
	if err != nil {
		l.logger.Error("upstream callback failed", "error", err)
		l.renderError(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	// Find or create internal user
	user, err := l.storage.FindOrCreateUser(r.Context(), identity.Provider, identity.ExternalSub, storage.UserProfile{
		Email:         identity.Email,
		EmailVerified: identity.EmailVerified,
		GivenName:     identity.GivenName,
		FamilyName:    identity.FamilyName,
		Picture:       identity.Picture,
		Locale:        identity.Locale,
	})
	if err != nil {
		l.logger.Error("user provisioning failed", "error", err)
		l.renderError(w, "user provisioning failed", http.StatusInternalServerError)
		return
	}

	// Complete the original auth request
	if err := l.storage.CompleteAuthRequest(r.Context(), authRequestID, user.ID); err != nil {
		l.logger.Error("complete auth request failed", "error", err, "authRequestID", authRequestID)
		l.renderError(w, "authorization failed", http.StatusInternalServerError)
		return
	}

	l.logger.Info("user authenticated",
		"user_id", user.ID,
		"email", user.Email,
		"provider", identity.Provider,
		"auth_request_id", authRequestID,
	)

	// Redirect back to the OP to finish the OIDC flow
	http.Redirect(w, r, l.opCallback(r.Context(), authRequestID), http.StatusFound)
}

func (l *Login) renderError(w http.ResponseWriter, msg string, status int) {
	w.WriteHeader(status)
	if err := templates.ExecuteTemplate(w, "error.html", struct{ Error string }{Error: msg}); err != nil {
		l.logger.Error("failed to render error template", "status", status, "error", err)
		if _, writeErr := w.Write([]byte(http.StatusText(status))); writeErr != nil {
			l.logger.Error("failed to write fallback error response", "status", status, "error", writeErr)
		}
	}
}

// encodeState combines authRequestID and csrfToken into an HMAC-SHA256
// authenticated base64url string. The MAC prevents state forgery.
func encodeState(key []byte, authRequestID, csrfToken string) string {
	payload := authRequestID + ":" + csrfToken
	mac := computeHMAC(key, []byte(payload))
	signed := payload + ":" + hex.EncodeToString(mac)
	return base64.RawURLEncoding.EncodeToString([]byte(signed))
}

// decodeState verifies the HMAC and splits the state back into authRequestID and csrfToken.
func decodeState(key []byte, state string) (authRequestID, csrfToken string, err error) {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", "", fmt.Errorf("decode state: %w", err)
	}

	// Split into payload parts and MAC: "authRequestID:csrfToken:hexMAC"
	lastColon := strings.LastIndex(string(raw), ":")
	if lastColon < 0 {
		return "", "", fmt.Errorf("malformed state")
	}
	payload := string(raw[:lastColon])
	macHex := string(raw[lastColon+1:])

	// Verify HMAC
	expectedMAC, err := hex.DecodeString(macHex)
	if err != nil {
		return "", "", fmt.Errorf("malformed state MAC")
	}
	if !hmac.Equal(computeHMAC(key, []byte(payload)), expectedMAC) {
		return "", "", fmt.Errorf("state HMAC verification failed")
	}

	// Split payload into authRequestID and csrfToken
	parts := strings.SplitN(payload, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("malformed state payload")
	}
	return parts[0], parts[1], nil
}

func computeHMAC(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func generateRandomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
