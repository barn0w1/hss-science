package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"

	gooidc "github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/session"
)

// AuthHandler handles OIDC RP authentication flows.
type AuthHandler struct {
	oauth2Config *oauth2.Config
	verifier     *gooidc.IDTokenVerifier
	sessionStore *session.Store
	devMode      bool
	spaOrigin    string
	logger       *slog.Logger
}

// NewAuthHandler creates a new auth handler for OIDC RP operations.
func NewAuthHandler(
	oauth2Config *oauth2.Config,
	verifier *gooidc.IDTokenVerifier,
	sessionStore *session.Store,
	devMode bool,
	spaOrigin string,
	logger *slog.Logger,
) *AuthHandler {
	return &AuthHandler{
		oauth2Config: oauth2Config,
		verifier:     verifier,
		sessionStore: sessionStore,
		devMode:      devMode,
		spaOrigin:    spaOrigin,
		logger:       logger,
	}
}

// Login initiates the OIDC authorization code flow.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	// Generate state parameter for CSRF protection.
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "failed to generate state")
		return
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	// Generate nonce for ID token verification.
	nonceBytes := make([]byte, 32)
	if _, err := rand.Read(nonceBytes); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "failed to generate nonce")
		return
	}
	nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)

	// Store state + nonce + return_to in a short-lived cookie.
	returnTo := r.URL.Query().Get("return_to")
	if returnTo == "" {
		returnTo = "/"
	}

	cookieValue := state + "|" + nonce + "|" + returnTo
	http.SetCookie(w, &http.Cookie{
		Name:     "myaccount_oidc_state",
		Value:    cookieValue,
		Path:     "/auth/callback",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   !h.devMode,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to the OP's authorization endpoint.
	authURL := h.oauth2Config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("nonce", nonce),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// Callback handles the OIDC authorization code callback.
func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	// Check for error from OP.
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		desc := r.URL.Query().Get("error_description")
		h.logger.Error("oidc callback error", "error", errParam, "description", desc)
		writeError(w, http.StatusBadRequest, errParam, desc)
		return
	}

	// Validate state cookie.
	stateCookie, err := r.Cookie("myaccount_oidc_state")
	if err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing state cookie")
		return
	}

	// Parse cookie: state|nonce|return_to
	parts := splitN(stateCookie.Value, "|", 3)
	if len(parts) != 3 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid state cookie")
		return
	}
	expectedState, expectedNonce, returnTo := parts[0], parts[1], parts[2]

	// Clear state cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "myaccount_oidc_state",
		Value:    "",
		Path:     "/auth/callback",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !h.devMode,
		SameSite: http.SameSiteLaxMode,
	})

	// Verify state matches.
	actualState := r.URL.Query().Get("state")
	if actualState != expectedState {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "state mismatch")
		return
	}

	// Exchange authorization code for tokens.
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing code parameter")
		return
	}

	oauth2Token, err := h.oauth2Config.Exchange(r.Context(), code)
	if err != nil {
		h.logger.Error("token exchange failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "token exchange failed")
		return
	}

	// Extract and verify the ID token.
	rawIDToken, ok := oauth2Token.Extra("id_token").(string)
	if !ok {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "no id_token in response")
		return
	}

	idToken, err := h.verifier.Verify(r.Context(), rawIDToken)
	if err != nil {
		h.logger.Error("id token verification failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "id_token verification failed")
		return
	}

	// Verify nonce.
	if idToken.Nonce != expectedNonce {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "nonce mismatch")
		return
	}

	// Extract claims from ID token.
	var claims struct {
		Sub        string `json:"sub"`
		Email      string `json:"email"`
		GivenName  string `json:"given_name"`
		FamilyName string `json:"family_name"`
		Picture    string `json:"picture"`
	}
	if err := idToken.Claims(&claims); err != nil {
		h.logger.Error("failed to extract claims", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "failed to extract claims")
		return
	}

	// Create session in Redis.
	sd := &session.SessionData{
		AccessToken:  oauth2Token.AccessToken,
		RefreshToken: oauth2Token.RefreshToken,
		IDToken:      rawIDToken,
		UserID:       claims.Sub,
		Email:        claims.Email,
		GivenName:    claims.GivenName,
		FamilyName:   claims.FamilyName,
		Picture:      claims.Picture,
		ExpiresAt:    oauth2Token.Expiry,
	}

	sessionID, err := h.sessionStore.Create(r.Context(), sd)
	if err != nil {
		h.logger.Error("session creation failed", "error", err)
		writeError(w, http.StatusInternalServerError, "INTERNAL", "session creation failed")
		return
	}

	// Set session cookie.
	cookieName := session.CookieName
	if h.devMode {
		cookieName = "myaccount_session"
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400, // 24 hours
		HttpOnly: true,
		Secure:   !h.devMode,
		SameSite: http.SameSiteLaxMode,
	})

	h.logger.Info("user authenticated", "user_id", claims.Sub, "email", claims.Email)

	// Redirect to the SPA.
	http.Redirect(w, r, returnTo, http.StatusFound)
}

// Logout destroys the session and clears the cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sd, ok := session.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "no session")
		return
	}

	// Determine the cookie name used.
	cookieName := session.CookieName
	if h.devMode {
		cookieName = "myaccount_session"
	}

	// Get session ID from cookie.
	cookie, err := r.Cookie(cookieName)
	if err == nil {
		_ = h.sessionStore.Delete(r.Context(), cookie.Value)
	}

	// Clear cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   !h.devMode,
		SameSite: http.SameSiteLaxMode,
	})

	h.logger.Info("user logged out", "user_id", sd.UserID)

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

// Session returns current session info for the SPA.
func (h *AuthHandler) Session(w http.ResponseWriter, r *http.Request) {
	sd, ok := session.FromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "no session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"user_id":       sd.UserID,
		"email":         sd.Email,
		"given_name":    sd.GivenName,
		"family_name":   sd.FamilyName,
		"picture":       sd.Picture,
	})
}

func splitN(s, sep string, n int) []string {
	result := make([]string, 0, n)
	for i := 0; i < n-1; i++ {
		idx := indexOf(s, sep)
		if idx < 0 {
			break
		}
		result = append(result, s[:idx])
		s = s[idx+len(sep):]
	}
	result = append(result, s)
	return result
}

func indexOf(s, sep string) int {
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}

// Ensure json import is used via the claims struct unmarshaling.
var _ = json.Marshal
