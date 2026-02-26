package handler

import (
	"net/http"

	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/grpcclient"
	"github.com/barn0w1/hss-science/server/bff/myaccount/internal/session"
	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
)

// AccountHandler handles account-level API endpoints.
type AccountHandler struct {
	grpc *grpcclient.Client
}

// NewAccountHandler creates a new account handler.
func NewAccountHandler(grpc *grpcclient.Client) *AccountHandler {
	return &AccountHandler{grpc: grpc}
}

// Delete handles DELETE /api/v1/account.
func (h *AccountHandler) Delete(w http.ResponseWriter, r *http.Request) {
	sd, _ := session.FromContext(r.Context())
	ctx := grpcclient.WithToken(r.Context(), sd.AccessToken)

	_, err := h.grpc.Service().DeleteAccount(ctx, &pb.DeleteAccountRequest{})
	if err != nil {
		writeGRPCError(w, err)
		return
	}

	writeNoContent(w)
}
