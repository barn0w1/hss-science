package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/grpcclient"
	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/session"
	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

// LinkedAccountsHandler handles linked accounts API endpoints.
type LinkedAccountsHandler struct {
	grpc *grpcclient.Client
}

// NewLinkedAccountsHandler creates a new linked accounts handler.
func NewLinkedAccountsHandler(grpc *grpcclient.Client) *LinkedAccountsHandler {
	return &LinkedAccountsHandler{grpc: grpc}
}

// List handles GET /api/v1/linked-accounts.
func (h *LinkedAccountsHandler) List(w http.ResponseWriter, r *http.Request) {
	sd, _ := session.FromContext(r.Context())
	ctx := grpcclient.WithToken(r.Context(), sd.AccessToken)

	resp, err := h.grpc.Service().ListLinkedAccounts(ctx, &pb.ListLinkedAccountsRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	accounts := make([]*LinkedAccountResponse, len(resp.LinkedAccounts))
	for i, la := range resp.LinkedAccounts {
		accounts[i] = linkedAccountToREST(la)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"linked_accounts": accounts,
	})
}

// Unlink handles DELETE /api/v1/linked-accounts/{id}.
func (h *LinkedAccountsHandler) Unlink(w http.ResponseWriter, r *http.Request) {
	sd, _ := session.FromContext(r.Context())
	ctx := grpcclient.WithToken(r.Context(), sd.AccessToken)

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "BAD_REQUEST", "missing id parameter")
		return
	}

	_, err := h.grpc.Service().UnlinkAccount(ctx, &pb.UnlinkAccountRequest{
		LinkedAccountId: id,
	})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeNoContent(w)
}
