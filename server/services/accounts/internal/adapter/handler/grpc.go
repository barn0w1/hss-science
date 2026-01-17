package handler

import (
	"context"

	pb "github.com/barn0w1/hss-science/server/gen/public/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"
)

type AuthHandler struct {
	pb.UnimplementedAccountsServiceServer
	usecase *usecase.AuthUsecase
}

func NewAuthHandler(uc *usecase.AuthUsecase) *AuthHandler {
	return &AuthHandler{usecase: uc}
}

func (h *AuthHandler) SignUp(ctx context.Context, req *pb.SignUpRequest) (*pb.SignUpResponse, error) {
	user, err := h.usecase.SignUp(ctx, req.Email, req.Password)
	if err != nil {
		return nil, err // Convert logic error to gRPC status code
	}
	return &pb.SignUpResponse{UserId: user.ID}, nil
}
