package grpctransport

import (
	"context"

	internalpb "github.com/barn0w1/hss-science/server/gen/internal_/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type InternalHandler struct {
	internalpb.UnimplementedAccountsInternalServiceServer
	usecase *usecase.AuthUsecase
}

func NewInternalHandler(usecase *usecase.AuthUsecase) *InternalHandler {
	return &InternalHandler{usecase: usecase}
}

// VerifyAuthCode validates and consumes an auth code for internal services.
func (h *InternalHandler) VerifyAuthCode(ctx context.Context, req *internalpb.VerifyAuthCodeRequest) (*internalpb.VerifyAuthCodeResponse, error) {
	if req.GetAuthCode() == "" || req.GetAudience() == "" {
		return nil, status.Error(codes.InvalidArgument, "auth_code and audience are required")
	}

	result, err := h.usecase.VerifyAuthCode(ctx, req.GetAuthCode(), req.GetAudience())
	if err != nil {
		return nil, status.Errorf(codes.Unauthenticated, "invalid auth code: %v", err)
	}

	return &internalpb.VerifyAuthCodeResponse{
		UserId: result.UserID.String(),
		Role:   string(result.Role),
		Claims: map[string]string{},
	}, nil
}
