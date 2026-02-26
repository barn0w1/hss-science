package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/grpcclient"
	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/session"
	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

// SessionsHandler handles session management API endpoints.
type SessionsHandler struct {
	grpc *grpcclient.Client
}

// NewSessionsHandler creates a new sessions handler.
func NewSessionsHandler(grpc *grpcclient.Client) *SessionsHandler {
	return &SessionsHandler{grpc: grpc}
}

// List handles GET /api/v1/sessions.
func (h *SessionsHandler) List(w http.ResponseWriter, r *http.Request) {
	sd, _ := session.FromContext(r.Context())
	ctx := grpcclient.WithToken(r.Context(), sd.AccessToken)

	resp, err := h.grpc.Service().ListActiveSessions(ctx, &pb.ListActiveSessionsRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	sessions := make([]*SessionResponse, len(resp.Sessions))
	for i, s := range resp.Sessions {
		sessions[i] = sessionToREST(s)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": sessions,
	})
}

// Revoke handles DELETE /api/v1/sessions/{id}.
func (h *SessionsHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	sd, _ := session.FromContext(r.Context())
	ctx := grpcclient.WithToken(r.Context(), sd.AccessToken)

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing id parameter")
		return
	}

	_, err := h.grpc.Service().RevokeSession(ctx, &pb.RevokeSessionRequest{
		SessionId: id,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeNoContent(w)
}
