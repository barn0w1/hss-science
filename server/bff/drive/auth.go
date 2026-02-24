package drive

import (
	"encoding/json"
	"net/http"

	driveapi "github.com/barn0w1/hss-science/server/bff/gen/drive/v1"
	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

const (
	pkceCookieName         = "__Secure-oauth_pkce"
	accessTokenCookieName  = "__Secure-access_token"
	refreshTokenCookieName = "__Secure-refresh_token"

	pkceCookieMaxAge   = 600    // 10 minutes
	accessTokenMaxAge  = 900    // 15 minutes
	refreshTokenMaxAge = 604800 // 7 days
)

// AuthLogin initiates the Google OIDC login flow.
// GET /api/v1/auth/login
func (s *Server) AuthLogin(w http.ResponseWriter, r *http.Request) {
	codeVerifier, codeChallenge, err := GeneratePKCE()
	if err != nil {
		s.log.ErrorContext(r.Context(), "failed to generate PKCE", "error", err)
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	state, err := GenerateState()
	if err != nil {
		s.log.ErrorContext(r.Context(), "failed to generate state", "error", err)
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	encoded, err := s.secureCookie.Encode(pkceCookieName, PKCEState{
		State:        state,
		CodeVerifier: codeVerifier,
	})
	if err != nil {
		s.log.ErrorContext(r.Context(), "failed to encode PKCE cookie", "error", err)
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	http.SetCookie(w, s.newCookie(pkceCookieName, encoded, "/api/v1/auth/callback", pkceCookieMaxAge))

	authURL := s.oidc.AuthURL(state, codeChallenge)
	s.log.InfoContext(r.Context(), "oidc login initiated")
	http.Redirect(w, r, authURL, http.StatusFound)
}

// AuthCallback handles the Google OIDC callback.
// GET /api/v1/auth/callback?code=xxx&state=yyy
func (s *Server) AuthCallback(w http.ResponseWriter, r *http.Request, params driveapi.AuthCallbackParams) {
	// Read and decode PKCE cookie.
	cookie, err := r.Cookie(pkceCookieName)
	if err != nil {
		s.log.WarnContext(r.Context(), "missing PKCE cookie", "error", err)
		respondError(w, http.StatusBadRequest, "missing or expired PKCE cookie")
		return
	}

	var pkceState PKCEState
	if err := s.secureCookie.Decode(pkceCookieName, cookie.Value, &pkceState); err != nil {
		s.log.WarnContext(r.Context(), "invalid PKCE cookie", "error", err)
		respondError(w, http.StatusBadRequest, "invalid PKCE cookie")
		return
	}

	// Validate state.
	if params.State != pkceState.State {
		s.log.WarnContext(r.Context(), "state mismatch")
		respondError(w, http.StatusBadRequest, "state mismatch")
		return
	}

	// Exchange code for ID token.
	claims, err := s.oidc.Exchange(r.Context(), params.Code, pkceState.CodeVerifier)
	if err != nil {
		s.log.ErrorContext(r.Context(), "token exchange failed", "error", err)
		respondError(w, http.StatusInternalServerError, "token exchange failed")
		return
	}

	// Call AccountsService.LoginUser via gRPC.
	resp, err := s.accountsClient.LoginUser(r.Context(), &accountsv1.LoginUserRequest{
		GoogleId: claims.Sub,
		Email:    claims.Email,
		Name:     claims.Name,
		Picture:  claims.Picture,
		DeviceInfo: &accountsv1.DeviceInfo{
			IpAddress: r.RemoteAddr,
			UserAgent: r.UserAgent(),
		},
	})
	if err != nil {
		s.log.ErrorContext(r.Context(), "login RPC failed", "error", err)
		respondError(w, http.StatusInternalServerError, "login failed")
		return
	}

	// Delete PKCE cookie.
	http.SetCookie(w, s.newCookie(pkceCookieName, "", "/api/v1/auth/callback", -1))

	// Set access_token cookie.
	http.SetCookie(w, s.newCookie(accessTokenCookieName, resp.AccessToken, "/api/v1", accessTokenMaxAge))

	// Set refresh_token cookie.
	http.SetCookie(w, s.newCookie(refreshTokenCookieName, resp.RefreshToken, "/api/v1/auth", refreshTokenMaxAge))

	s.log.InfoContext(r.Context(), "user authenticated", "user_id", resp.UserId)
	http.Redirect(w, r, s.cfg.PostLoginRedirect, http.StatusFound)
}

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(driveapi.ErrorResponse{Error: msg})
}
