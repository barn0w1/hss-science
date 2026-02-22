package transport

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/domain"
	"github.com/barn0w1/hss-science/server/services/accounts/service"
)

// Server implements the gRPC AccountsServiceServer interface.
type Server struct {
	accountsv1.UnimplementedAccountsServiceServer
	svc *service.AccountsService
}

// NewServer creates a new gRPC server wrapping the accounts service.
func NewServer(svc *service.AccountsService) *Server {
	return &Server{svc: svc}
}

func (s *Server) GetAuthURL(ctx context.Context, req *accountsv1.GetAuthURLRequest) (*accountsv1.GetAuthURLResponse, error) {
	if req.GetProvider() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}
	if req.GetRedirectUri() == "" {
		return nil, status.Error(codes.InvalidArgument, "redirect_uri is required")
	}

	authURL, state, err := s.svc.GetAuthURL(ctx, req.GetProvider(), req.GetRedirectUri(), req.GetClientState())
	if err != nil {
		return nil, mapError(err)
	}

	return &accountsv1.GetAuthURLResponse{
		AuthUrl: authURL,
		State:   state,
	}, nil
}

func (s *Server) HandleProviderCallback(ctx context.Context, req *accountsv1.HandleProviderCallbackRequest) (*accountsv1.HandleProviderCallbackResponse, error) {
	if req.GetProvider() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}
	if req.GetCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}
	if req.GetState() == "" {
		return nil, status.Error(codes.InvalidArgument, "state is required")
	}

	authCode, redirectURI, clientState, user, err := s.svc.HandleProviderCallback(ctx, req.GetProvider(), req.GetCode(), req.GetState())
	if err != nil {
		return nil, mapError(err)
	}

	return &accountsv1.HandleProviderCallbackResponse{
		AuthCode:    authCode,
		RedirectUri: redirectURI,
		ClientState: clientState,
		User:        domainUserToProto(user),
	}, nil
}

func (s *Server) IssueAuthCode(ctx context.Context, req *accountsv1.IssueAuthCodeRequest) (*accountsv1.IssueAuthCodeResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	authCode, err := s.svc.IssueAuthCode(ctx, userID)
	if err != nil {
		return nil, mapError(err)
	}

	return &accountsv1.IssueAuthCodeResponse{
		AuthCode: authCode,
	}, nil
}

func (s *Server) ExchangeToken(ctx context.Context, req *accountsv1.ExchangeTokenRequest) (*accountsv1.ExchangeTokenResponse, error) {
	if req.GetAuthCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "auth_code is required")
	}

	user, err := s.svc.ExchangeToken(ctx, req.GetAuthCode())
	if err != nil {
		return nil, mapError(err)
	}

	return &accountsv1.ExchangeTokenResponse{
		User: domainUserToProto(user),
	}, nil
}

func (s *Server) GetUser(ctx context.Context, req *accountsv1.GetUserRequest) (*accountsv1.GetUserResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	userID, err := uuid.Parse(req.GetUserId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user_id format")
	}

	user, err := s.svc.GetUser(ctx, userID)
	if err != nil {
		return nil, mapError(err)
	}

	return &accountsv1.GetUserResponse{
		User: domainUserToProto(user),
	}, nil
}

func domainUserToProto(u *domain.User) *accountsv1.User {
	if u == nil {
		return nil
	}
	return &accountsv1.User{
		Id:          u.ID.String(),
		DisplayName: u.DisplayName,
		AvatarUrl:   u.AvatarURL,
		CreatedAt:   timestamppb.New(u.CreatedAt),
		UpdatedAt:   timestamppb.New(u.UpdatedAt),
	}
}

// mapError converts domain errors to appropriate gRPC status codes.
func mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrStateNotFound):
		return status.Error(codes.InvalidArgument, "invalid or expired state")
	case errors.Is(err, domain.ErrAuthCodeNotFound):
		return status.Error(codes.InvalidArgument, "invalid, expired, or already used auth code")
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
