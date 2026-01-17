package handler

import (
	"context"
	"fmt"

	// Protoの生成コード
	pb "github.com/barn0w1/hss-science/server/gen/public/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/service"
)

type GrpcServer struct {
	pb.UnimplementedAccountsServiceServer
	svc *service.AuthService
}

func NewGrpcServer(svc *service.AuthService) *GrpcServer {
	return &GrpcServer{svc: svc}
}

func (h *GrpcServer) GetLoginUrl(ctx context.Context, req *pb.GetLoginUrlRequest) (*pb.GetLoginUrlResponse, error) {
	// TODO: Replace with actual Discord Client ID and logic from the service layer.
	mockURL := fmt.Sprintf(
		"https://discord.com/oauth2/authorize?client_id=FAKE_CLIENT_ID&redirect_uri=%s&response_type=code&scope=identify",
		req.RedirectTo,
	)

	return &pb.GetLoginUrlResponse{
		Url: mockURL,
	}, nil
}

func (h *GrpcServer) GetSession(ctx context.Context, req *pb.GetSessionRequest) (*pb.GetSessionResponse, error) {
	// 1. ContextからUserIDを取り出す (Middlewareが入れてくれているはず)
	userID := "..."

	// 2. Serviceを呼ぶ
	user, err := h.svc.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 3. Domain型 -> Proto型 への変換
	return &pb.GetSessionResponse{
		Session: &pb.Session{
			UserId:      user.ID,
			DisplayName: user.Username,
			AvatarUrl:   user.AvatarURL,
		},
	}, nil
}
