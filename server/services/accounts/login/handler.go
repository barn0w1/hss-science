package login

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/barn0w1/hss-science/server/services/accounts/model"
	"github.com/barn0w1/hss-science/server/services/accounts/repo"
)

type Handler struct {
	providers   []*UpstreamProvider
	providerMap map[string]*UpstreamProvider
	userRepo    *repo.UserRepository
	authReqRepo *repo.AuthRequestRepository
	cryptoKey   [32]byte
	callbackURL func(context.Context, string) string
	tmpl        *template.Template
	logger      *slog.Logger
}

func NewHandler(
	providers []*UpstreamProvider,
	userRepo *repo.UserRepository,
	authReqRepo *repo.AuthRequestRepository,
	cryptoKey [32]byte,
	callbackURL func(context.Context, string) string,
	logger *slog.Logger,
) *Handler {
	pm := make(map[string]*UpstreamProvider, len(providers))
	for _, p := range providers {
		pm[p.Name] = p
	}

	tmpl := template.Must(template.New("select_provider").Parse(selectProviderHTML))

	return &Handler{
		providers:   providers,
		providerMap: pm,
		userRepo:    userRepo,
		authReqRepo: authReqRepo,
		cryptoKey:   cryptoKey,
		callbackURL: callbackURL,
		tmpl:        tmpl,
		logger:      logger,
	}
}

type selectProviderData struct {
	AuthRequestID string
	Providers     []*UpstreamProvider
}

func (h *Handler) SelectProvider(w http.ResponseWriter, r *http.Request) {
	authRequestID := r.URL.Query().Get("authRequestID")
	if authRequestID == "" {
		http.Error(w, "missing authRequestID", http.StatusBadRequest)
		return
	}

	_, err := h.authReqRepo.GetByID(r.Context(), authRequestID)
	if err != nil {
		h.logger.Error("auth request not found", "id", authRequestID, "error", err)
		http.Error(w, "invalid auth request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.Execute(w, selectProviderData{
		AuthRequestID: authRequestID,
		Providers:     h.providers,
	}); err != nil {
		h.logger.Error("template execution failed", "error", err)
	}
}

type federatedState struct {
	AuthRequestID string `json:"a"`
	Provider      string `json:"p"`
	Nonce         string `json:"n"`
}

func (h *Handler) FederatedRedirect(w http.ResponseWriter, r *http.Request) {
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

	claims, err := provider.UserInfoFunc(r.Context(), token)
	if err != nil {
		h.logger.Error("user info retrieval failed", "provider", state.Provider, "error", err)
		http.Error(w, "authentication failed", http.StatusInternalServerError)
		return
	}

	user, err := h.findOrCreateUser(r.Context(), state.Provider, claims)
	if err != nil {
		h.logger.Error("find or create user failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	now := time.Now().UTC()
	if err := h.authReqRepo.CompleteLogin(r.Context(), state.AuthRequestID, user.ID, now, []string{"federated"}); err != nil {
		h.logger.Error("complete login failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	callbackURL := h.callbackURL(r.Context(), state.AuthRequestID)
	http.Redirect(w, r, callbackURL, http.StatusFound)
}

func (h *Handler) findOrCreateUser(ctx context.Context, provider string, claims *UpstreamClaims) (*model.User, error) {
	existing, err := h.userRepo.FindByFederatedIdentity(ctx, provider, claims.Subject)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	userID := uuid.New().String()
	user := &model.User{
		ID:            userID,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		GivenName:     claims.GivenName,
		FamilyName:    claims.FamilyName,
		Picture:       claims.Picture,
	}
	fi := &model.FederatedIdentity{
		ID:              uuid.New().String(),
		UserID:          userID,
		Provider:        provider,
		ProviderSubject: claims.Subject,
	}
	if err := h.userRepo.CreateWithFederatedIdentity(ctx, user, fi); err != nil {
		return nil, err
	}
	return user, nil
}

func (h *Handler) encryptState(state federatedState) (string, error) {
	plaintext, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(h.cryptoKey[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func (h *Handler) decryptState(encrypted string) (federatedState, error) {
	var state federatedState
	ciphertext, err := base64.URLEncoding.DecodeString(encrypted)
	if err != nil {
		return state, fmt.Errorf("base64 decode: %w", err)
	}
	block, err := aes.NewCipher(h.cryptoKey[:])
	if err != nil {
		return state, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return state, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return state, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return state, fmt.Errorf("decrypt: %w", err)
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
