package grpctransport

import (
	"context"

	publicpb "github.com/barn0w1/hss-science/server/gen/public/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/adapter/transport"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// PublicGRPCHandler is intentionally minimal because accounts uses native HTTP for public endpoints.
type PublicGRPCHandler struct {
	publicpb.UnimplementedAccountsServiceServer
}

func NewPublicGRPCHandler() *PublicGRPCHandler {
	return &PublicGRPCHandler{}
}

func (h *PublicGRPCHandler) Authorize(context.Context, *publicpb.AuthorizeRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "use native HTTP "+transport.AuthorizePath)
}

func (h *PublicGRPCHandler) OAuthCallback(context.Context, *publicpb.OAuthCallbackRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "use native HTTP "+transport.OAuthCallbackPath)
}
