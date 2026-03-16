package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/accounts"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/middleware"
)

type ProvidersHandler struct {
	accounts *accounts.Client
}

func NewProviders(ac *accounts.Client) *ProvidersHandler {
	return &ProvidersHandler{accounts: ac}
}

func (h *ProvidersHandler) List(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFromContext(r.Context())
	providers, err := h.accounts.ListLinkedProviders(r.Context(), sess.AccessToken)
	if err != nil {
		httpStatus, code := grpcToHTTP(err)
		writeError(w, httpStatus, code, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(providers)
}

func (h *ProvidersHandler) Unlink(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFromContext(r.Context())
	identityID := chi.URLParam(r, "identityID")
	if identityID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "identityID is required")
		return
	}

	if err := h.accounts.UnlinkProvider(r.Context(), sess.AccessToken, identityID); err != nil {
		httpStatus, code := grpcToHTTP(err)
		writeError(w, httpStatus, code, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
