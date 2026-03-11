package authn

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/identity"
	oidcdom "github.com/barn0w1/hss-science/server/services/accounts/internal/oidc"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/pkg/crypto"
)

type Handler struct {
	providers   []*Provider
	providerMap map[string]*Provider
	loginUC     *CompleteFederatedLogin
	cipher      crypto.Cipher
	callbackURL func(context.Context, string) string
	tmpl        *template.Template
	logger      *slog.Logger
}

func NewHandler(
	providers []*Provider,
	identitySvc identity.Service,
	loginCompleter oidcdom.LoginCompleter,
	cipher crypto.Cipher,
	callbackURL func(context.Context, string) string,
	logger *slog.Logger,
) *Handler {
	pm := make(map[string]*Provider, len(providers))
	for _, p := range providers {
		pm[p.Name] = p
	}
	tmpl := template.Must(template.New("select_provider").Parse(selectProviderHTML))
	return &Handler{
		providers:   providers,
		providerMap: pm,
		loginUC:     NewCompleteFederatedLogin(identitySvc, loginCompleter),
		cipher:      cipher,
		callbackURL: callbackURL,
		tmpl:        tmpl,
		logger:      logger,
	}
}

type selectProviderData struct {
	AuthRequestID string
	Providers     []*Provider
}

func (h *Handler) SelectProvider(w http.ResponseWriter, r *http.Request) {
	authRequestID := r.URL.Query().Get("authRequestID")
	if authRequestID == "" {
		http.Error(w, "missing authRequestID", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var buf bytes.Buffer
	if err := h.tmpl.Execute(&buf, selectProviderData{
		AuthRequestID: authRequestID,
		Providers:     h.providers,
	}); err != nil {
		h.logger.Error("template execution failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_, _ = buf.WriteTo(w)
}

type federatedState struct {
	AuthRequestID string `json:"a"`
	Provider      string `json:"p"`
	Nonce         string `json:"n"`
}

func (h *Handler) FederatedRedirect(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	authRequestID := r.FormValue("authRequestID")
	providerName := r.FormValue("provider")

	if authRequestID == "" || providerName == "" {
		http.Error(w, "missing authRequestID or provider", http.StatusBadRequest)
		return
	}

	provider, ok := h.providerMap[providerName]
	if !ok {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}

	state := federatedState{
		AuthRequestID: authRequestID,
		Provider:      providerName,
		Nonce:         uuid.New().String(),
	}
	encryptedState, err := h.encryptState(state)
	if err != nil {
		h.logger.Error("failed to encrypt state", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	url := provider.OAuth2Config.AuthCodeURL(encryptedState, oauth2.AccessTypeOffline)
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *Handler) FederatedCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	stateParam := r.URL.Query().Get("state")
	if code == "" || stateParam == "" {
		http.Error(w, "missing code or state", http.StatusBadRequest)
		return
	}

	state, err := h.decryptState(stateParam)
	if err != nil {
		h.logger.Error("failed to decrypt state", "error", err)
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	provider, ok := h.providerMap[state.Provider]
	if !ok {
		http.Error(w, "unknown provider in state", http.StatusBadRequest)
		return
	}

	token, err := provider.OAuth2Config.Exchange(r.Context(), code)
	if err != nil {
		h.logger.Error("code exchange failed", "provider", state.Provider, "error", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	claims, err := provider.Claims.FetchClaims(r.Context(), token)
	if err != nil {
		h.logger.Error("user info retrieval failed", "provider", state.Provider, "error", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	if _, err := h.loginUC.Execute(r.Context(), state.Provider, *claims, state.AuthRequestID); err != nil {
		h.logger.Error("login completion failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	callbackURL := h.callbackURL(r.Context(), state.AuthRequestID)
	http.Redirect(w, r, callbackURL, http.StatusFound)
}

func (h *Handler) encryptState(state federatedState) (string, error) {
	plaintext, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return h.cipher.Encrypt(plaintext)
}

func (h *Handler) decryptState(encoded string) (federatedState, error) {
	var state federatedState
	plaintext, err := h.cipher.Decrypt(encoded)
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(plaintext, &state); err != nil {
		return state, fmt.Errorf("unmarshal state: %w", err)
	}
	return state, nil
}

const selectProviderHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Sign In</title>
</head>
<body>
    <h1>Sign In</h1>
    <p>Choose your sign-in method:</p>
    {{range .Providers}}
    <form method="POST" action="/login/select">
        <input type="hidden" name="authRequestID" value="{{$.AuthRequestID}}">
        <input type="hidden" name="provider" value="{{.Name}}">
        <button type="submit">{{.DisplayName}}</button>
    </form>
    {{end}}
</body>
</html>`
