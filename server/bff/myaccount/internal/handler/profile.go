package handler

import (
	"encoding/json"
	"net/http"

	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/grpcclient"
	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/session"
	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// ProfileHandler handles profile API endpoints.
type ProfileHandler struct {
	grpc *grpcclient.Client
}

// NewProfileHandler creates a new profile handler.
func NewProfileHandler(grpc *grpcclient.Client) *ProfileHandler {
	return &ProfileHandler{grpc: grpc}
}

// Get handles GET /api/v1/profile.
func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	sd, _ := session.FromContext(r.Context())
	ctx := grpcclient.WithToken(r.Context(), sd.AccessToken)

	resp, err := h.grpc.Service().GetProfile(ctx, &pb.GetProfileRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, profileToREST(resp.Profile))
}

// UpdateProfileBody is the JSON request body for PATCH /api/v1/profile.
type UpdateProfileBody struct {
	GivenName  *string `json:"given_name,omitempty"`
	FamilyName *string `json:"family_name,omitempty"`
	Picture    *string `json:"picture,omitempty"`
	Locale     *string `json:"locale,omitempty"`
}

// Update handles PATCH /api/v1/profile.
func (h *ProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	sd, _ := session.FromContext(r.Context())
	ctx := grpcclient.WithToken(r.Context(), sd.AccessToken)

	var body UpdateProfileBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "invalid JSON body")
		return
	}

	// Build field mask and profile from non-nil fields.
	paths := make([]string, 0, 4)
	profile := &pb.Profile{}

	if body.GivenName != nil {
		profile.GivenName = *body.GivenName
		paths = append(paths, "given_name")
	}
	if body.FamilyName != nil {
		profile.FamilyName = *body.FamilyName
		paths = append(paths, "family_name")
	}
	if body.Picture != nil {
		profile.Picture = *body.Picture
		paths = append(paths, "picture")
	}
	if body.Locale != nil {
		profile.Locale = *body.Locale
		paths = append(paths, "locale")
	}

	if len(paths) == 0 {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "no fields to update")
		return
	}

	resp, err := h.grpc.Service().UpdateProfile(ctx, &pb.UpdateProfileRequest{
		Profile:    profile,
		UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, profileToREST(resp.Profile))
}
