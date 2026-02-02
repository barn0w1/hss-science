package handler

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/config"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"
)

const (
	AuthorizePath     = "/v1/authorize"
	OAuthCallbackPath = "/v1/oauth/callback"
)

type PublicHandler struct {
	usecase *usecase.AuthUsecase
	cfg     *config.Config
}

func NewPublicHandler(usecase *usecase.AuthUsecase, cfg *config.Config) *PublicHandler {
	return &PublicHandler{usecase: usecase, cfg: cfg}
}

// Authorize handles GET /v1/authorize
func (h *PublicHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	audience := q.Get("audience")
	redirectURI := q.Get("redirect_uri")
	state := q.Get("state")

	sessionID := h.getSessionIDFromCookie(r)
	redirectURL, err := h.usecase.Authorize(r.Context(), audience, redirectURI, state, sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// OAuthCallback handles GET /v1/oauth/callback
func (h *PublicHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	code := q.Get("code")
	state := q.Get("state")

	ip := clientIP(r)
	ua := r.UserAgent()

	result, err := h.usecase.OAuthCallback(r.Context(), code, state, ip, ua)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.setSessionCookie(w, result.SessionToken, result.Session.ExpiresAt)

	redirectURL := buildAuthorizeURL(result.Audience, result.RedirectURI, result.State)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func (h *PublicHandler) getSessionIDFromCookie(r *http.Request) string {
	cookie, err := r.Cookie(h.cfg.SessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func (h *PublicHandler) setSessionCookie(w http.ResponseWriter, sessionID string, expiresAt time.Time) {
	maxAge := int(time.Until(expiresAt).Seconds())
	if maxAge < 0 {
		maxAge = 0
	}
	cookie := &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: parseSameSite(h.cfg.CookieSameSite),
		Expires:  expiresAt,
		MaxAge:   maxAge,
	}

	if h.cfg.CookieDomain != "" {
		cookie.Domain = h.cfg.CookieDomain
	}

	http.SetCookie(w, cookie)
}

func parseSameSite(value string) http.SameSite {
	switch value {
	case "None":
		return http.SameSiteNoneMode
	case "Strict":
		return http.SameSiteStrictMode
	default:
		return http.SameSiteLaxMode
	}
}

func buildAuthorizeURL(audience, redirectURI, state string) string {
	query := url.Values{}
	query.Set("audience", audience)
	query.Set("redirect_uri", redirectURI)
	if state != "" {
		query.Set("state", state)
	}
	return AuthorizePath + "?" + query.Encode()
}

func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
