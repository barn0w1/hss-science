package handler

import (
	"log"
	"net/http"
	"net/url"

	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AuthHandler handles HTTP endpoints for the SSO authentication flow.
// It is the BFF's responsibility to deal with HTTP redirects, cookies,
// and query parameters — the gRPC layer never sees these concerns.
type AuthHandler struct {
	accounts accountsv1.AccountsServiceClient
}

// NewAuthHandler creates a new AuthHandler backed by the given gRPC client.
func NewAuthHandler(accounts accountsv1.AccountsServiceClient) *AuthHandler {
	return &AuthHandler{accounts: accounts}
}

// RegisterRoutes registers the auth-related HTTP routes on the given mux.
func (h *AuthHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/authorize", h.handleAuthorize)
	mux.HandleFunc("GET /api/v1/oauth/callback", h.handleOAuthCallback)
}

// handleAuthorize starts the SSO flow.
// Query params: provider (default "discord"), redirect_uri, state (optional client CSRF).
// It calls the gRPC GetAuthURL and redirects the user to the external IdP.
func (h *AuthHandler) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	redirectURI := q.Get("redirect_uri")
	if redirectURI == "" {
		http.Error(w, "redirect_uri is required", http.StatusBadRequest)
		return
	}

	provider := q.Get("provider")
	if provider == "" {
		provider = "discord"
	}

	clientState := q.Get("state")

	resp, err := h.accounts.GetAuthURL(r.Context(), &accountsv1.GetAuthURLRequest{
		Provider:    provider,
		RedirectUri: redirectURI,
		ClientState: clientState,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Store the state in a short-lived cookie so we can verify it on callback.
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    resp.GetState(),
		Path:     "/api/v1/oauth",
		MaxAge:   600, // 10 minutes
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, resp.GetAuthUrl(), http.StatusFound)
}

// handleOAuthCallback handles the redirect back from the external IdP.
// Query params: code, state.
// It validates the state cookie, calls the gRPC HandleProviderCallback,
// and redirects the user back to the requesting service with an internal auth code.
func (h *AuthHandler) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	code := q.Get("code")
	state := q.Get("state")

	if code == "" || state == "" {
		http.Error(w, "code and state are required", http.StatusBadRequest)
		return
	}

	// Verify state matches the cookie we set.
	cookie, err := r.Cookie("oauth_state")
	if err != nil || cookie.Value != state {
		http.Error(w, "state mismatch", http.StatusUnauthorized)
		return
	}

	// Clear the state cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    "",
		Path:     "/api/v1/oauth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	// Delegate the heavy lifting to the gRPC service.
	resp, err := h.accounts.HandleProviderCallback(r.Context(), &accountsv1.HandleProviderCallbackRequest{
		Provider: "discord",
		Code:     code,
		State:    state,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	// Build the redirect URL with the internal auth code.
	redirectURL, err := url.Parse(resp.GetRedirectUri())
	if err != nil {
		http.Error(w, "invalid redirect_uri", http.StatusInternalServerError)
		return
	}

	rq := redirectURL.Query()
	rq.Set("code", resp.GetAuthCode())
	if resp.GetClientState() != "" {
		rq.Set("state", resp.GetClientState())
	}
	redirectURL.RawQuery = rq.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// writeGRPCError translates a gRPC error to an HTTP response.
func writeGRPCError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	httpCode := http.StatusInternalServerError
	switch st.Code() {
	case codes.InvalidArgument:
		httpCode = http.StatusBadRequest
	case codes.NotFound:
		httpCode = http.StatusNotFound
	case codes.Unauthenticated:
		httpCode = http.StatusUnauthorized
	case codes.PermissionDenied:
		httpCode = http.StatusForbidden
	}

	log.Printf("gRPC error: %v", st.Message())
	http.Error(w, st.Message(), httpCode)
}
