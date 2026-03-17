package handler

import (
	"encoding/json"
	"net/http"
	"time"

	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/accounts"
	"github.com/barn0w1/hss-science/server/services/myaccount-bff/internal/middleware"
)

type profileResponse struct {
	UserID         string `json:"user_id"`
	Email          string `json:"email"`
	EmailVerified  bool   `json:"email_verified"`
	Name           string `json:"name"`
	GivenName      string `json:"given_name"`
	FamilyName     string `json:"family_name"`
	Picture        string `json:"picture"`
	NameIsLocal    bool   `json:"name_is_local"`
	PictureIsLocal bool   `json:"picture_is_local"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
}

func toProfileResponse(p *pb.Profile) profileResponse {
	resp := profileResponse{
		UserID:         p.GetUserId(),
		Email:          p.GetEmail(),
		EmailVerified:  p.GetEmailVerified(),
		Name:           p.GetName(),
		GivenName:      p.GetGivenName(),
		FamilyName:     p.GetFamilyName(),
		Picture:        p.GetPicture(),
		NameIsLocal:    p.GetNameIsLocal(),
		PictureIsLocal: p.GetPictureIsLocal(),
	}
	if p.GetCreatedAt() != nil {
		resp.CreatedAt = p.GetCreatedAt().AsTime().Format(time.RFC3339)
	}
	if p.GetUpdatedAt() != nil {
		resp.UpdatedAt = p.GetUpdatedAt().AsTime().Format(time.RFC3339)
	}
	return resp
}

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
	_ = json.NewEncoder(w).Encode(toProfileResponse(profile))
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
	_ = json.NewEncoder(w).Encode(toProfileResponse(profile))
}
