package grpcserver

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/identity-service/internal/identity"
	oidcdom "github.com/barn0w1/hss-science/server/services/identity-service/internal/oidc"
)

var _ pb.AccountManagementServiceServer = (*Handler)(nil)

type Handler struct {
	pb.UnimplementedAccountManagementServiceServer
	identitySvc      identity.Service
	deviceSessionSvc oidcdom.DeviceSessionService
}

func (h *Handler) GetMyProfile(ctx context.Context, _ *pb.GetMyProfileRequest) (*pb.Profile, error) {
	userID := UserIDFromContext(ctx)
	user, err := h.identitySvc.GetUser(ctx, userID)
	if err != nil {
		return nil, domainStatus(err)
	}
	return userToProto(user), nil
}

func (h *Handler) UpdateMyProfile(
	ctx context.Context, req *pb.UpdateMyProfileRequest,
) (*pb.Profile, error) {
	userID := UserIDFromContext(ctx)
	user, err := h.identitySvc.UpdateProfile(ctx, userID, req.Name, req.Picture)
	if err != nil {
		return nil, domainStatus(err)
	}
	return userToProto(user), nil
}

func (h *Handler) ListLinkedProviders(
	ctx context.Context, _ *pb.ListLinkedProvidersRequest,
) (*pb.ListLinkedProvidersResponse, error) {
	userID := UserIDFromContext(ctx)
	fis, err := h.identitySvc.ListLinkedProviders(ctx, userID)
	if err != nil {
		return nil, domainStatus(err)
	}
	providers := make([]*pb.FederatedProviderInfo, len(fis))
	for i, fi := range fis {
		providers[i] = &pb.FederatedProviderInfo{
			IdentityId:    fi.ID,
			Provider:      fi.Provider,
			ProviderEmail: fi.ProviderEmail,
			LastLoginAt:   timestamppb.New(fi.LastLoginAt),
		}
	}
	return &pb.ListLinkedProvidersResponse{Providers: providers}, nil
}

func (h *Handler) UnlinkProvider(
	ctx context.Context, req *pb.UnlinkProviderRequest,
) (*emptypb.Empty, error) {
	if req.IdentityId == "" {
		return nil, status.Error(codes.InvalidArgument, "identity_id is required")
	}
	userID := UserIDFromContext(ctx)
	if err := h.identitySvc.UnlinkProvider(ctx, userID, req.IdentityId); err != nil {
		return nil, domainStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) ListActiveSessions(
	ctx context.Context, _ *pb.ListActiveSessionsRequest,
) (*pb.ListActiveSessionsResponse, error) {
	userID := UserIDFromContext(ctx)
	sessions, err := h.deviceSessionSvc.ListActiveByUserID(ctx, userID)
	if err != nil {
		return nil, domainStatus(err)
	}
	pbSessions := make([]*pb.Session, len(sessions))
	for i, s := range sessions {
		pbSessions[i] = &pb.Session{
			SessionId:  s.ID,
			DeviceName: s.DeviceName,
			IpAddress:  s.IPAddress,
			CreatedAt:  timestamppb.New(s.CreatedAt),
			LastUsedAt: timestamppb.New(s.LastUsedAt),
		}
	}
	return &pb.ListActiveSessionsResponse{Sessions: pbSessions}, nil
}

func (h *Handler) RevokeSession(
	ctx context.Context, req *pb.RevokeSessionRequest,
) (*emptypb.Empty, error) {
	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}
	userID := UserIDFromContext(ctx)
	if err := h.deviceSessionSvc.RevokeByID(ctx, req.SessionId, userID); err != nil {
		return nil, domainStatus(err)
	}
	return &emptypb.Empty{}, nil
}

func (h *Handler) RevokeAllOtherSessions(
	ctx context.Context, req *pb.RevokeAllOtherSessionsRequest,
) (*emptypb.Empty, error) {
	userID := UserIDFromContext(ctx)
	sessions, err := h.deviceSessionSvc.ListActiveByUserID(ctx, userID)
	if err != nil {
		return nil, domainStatus(err)
	}
	for _, s := range sessions {
		if s.ID == req.CurrentSessionId {
			continue
		}
		_ = h.deviceSessionSvc.RevokeByID(ctx, s.ID, userID)
	}
	return &emptypb.Empty{}, nil
}

func userToProto(u *identity.User) *pb.Profile {
	return &pb.Profile{
		UserId:         u.ID,
		Email:          u.Email,
		EmailVerified:  u.EmailVerified,
		Name:           u.Name,
		GivenName:      u.GivenName,
		FamilyName:     u.FamilyName,
		Picture:        u.Picture,
		NameIsLocal:    u.LocalName != nil && *u.LocalName != "",
		PictureIsLocal: u.LocalPicture != nil && *u.LocalPicture != "",
		CreatedAt:      timestamppb.New(u.CreatedAt),
		UpdatedAt:      timestamppb.New(u.UpdatedAt),
	}
}
