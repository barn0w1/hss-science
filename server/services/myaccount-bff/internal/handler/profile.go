package handler

import (
	"encoding/json"
	"net/http"

	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/accounts"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/middleware"
)

type ProfileHandler struct {
	accounts *accounts.Client
}

func NewProfile(ac *accounts.Client) *ProfileHandler {
	return &ProfileHandler{accounts: ac}
}

func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFromContext(r.Context())
	profile, err := h.accounts.GetMyProfile(r.Context(), sess.AccessToken)
	if err != nil {
		httpStatus, code := grpcToHTTP(err)
		writeError(w, httpStatus, code, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

type updateProfileRequest struct {
	Name    *string `json:"name"`
	Picture *string `json:"picture"`
}

func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	sess := middleware.SessionFromContext(r.Context())

	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid JSON body")
		return
	}

	profile, err := h.accounts.UpdateMyProfile(r.Context(), sess.AccessToken, req.Name, req.Picture)
	if err != nil {
		httpStatus, code := grpcToHTTP(err)
		writeError(w, httpStatus, code, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}
