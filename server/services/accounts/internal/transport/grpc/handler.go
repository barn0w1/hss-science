package grpc

import (
	"context"
	"errors"

	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Handler implements the AccountsService gRPC interface.
type Handler struct {
	accountsv1.UnimplementedAccountsServiceServer
	auth *usecase.AuthUsecase
}

// NewHandler creates a new gRPC handler with the given auth usecase.
func NewHandler(auth *usecase.AuthUsecase) *Handler {
	return &Handler{auth: auth}
}

func (h *Handler) GetAuthURL(ctx context.Context, req *accountsv1.GetAuthURLRequest) (*accountsv1.GetAuthURLResponse, error) {
	if req.GetProvider() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}
	if req.GetRedirectUri() == "" {
		return nil, status.Error(codes.InvalidArgument, "redirect_uri is required")
	}

	authURL, state, err := h.auth.GetAuthURL(ctx, req.GetProvider(), req.GetRedirectUri(), req.GetClientState())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get auth url: %v", err)
	}

	return &accountsv1.GetAuthURLResponse{
		AuthUrl: authURL,
		State:   state,
	}, nil
}

func (h *Handler) HandleProviderCallback(ctx context.Context, req *accountsv1.HandleProviderCallbackRequest) (*accountsv1.HandleProviderCallbackResponse, error) {
	if req.GetProvider() == "" {
		return nil, status.Error(codes.InvalidArgument, "provider is required")
	}
	if req.GetCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "code is required")
	}
	if req.GetState() == "" {
		return nil, status.Error(codes.InvalidArgument, "state is required")
	}

	authCode, redirectURI, clientState, user, err := h.auth.HandleProviderCallback(ctx, req.GetProvider(), req.GetCode(), req.GetState())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrExpired) || errors.Is(err, domain.ErrInvalidState) {
			return nil, status.Errorf(codes.Unauthenticated, "callback failed: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "callback: %v", err)
	}

	return &accountsv1.HandleProviderCallbackResponse{
		AuthCode:    authCode,
		RedirectUri: redirectURI,
		ClientState: clientState,
		User:        domainUserToProto(user),
	}, nil
}

func (h *Handler) ExchangeToken(ctx context.Context, req *accountsv1.ExchangeTokenRequest) (*accountsv1.ExchangeTokenResponse, error) {
	if req.GetAuthCode() == "" {
		return nil, status.Error(codes.InvalidArgument, "auth_code is required")
	}

	user, err := h.auth.ExchangeToken(ctx, req.GetAuthCode())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrAlreadyUsed) || errors.Is(err, domain.ErrExpired) {
			return nil, status.Errorf(codes.Unauthenticated, "exchange failed: %v", err)
		}
		return nil, status.Errorf(codes.Internal, "exchange: %v", err)
	}

	return &accountsv1.ExchangeTokenResponse{
		User: domainUserToProto(user),
	}, nil
}

func (h *Handler) GetUser(ctx context.Context, req *accountsv1.GetUserRequest) (*accountsv1.GetUserResponse, error) {
	if req.GetUserId() == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	user, err := h.auth.GetUser(ctx, req.GetUserId())
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "get user: %v", err)
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
		Id:          u.ID,
		DisplayName: u.DisplayName,
		AvatarUrl:   u.AvatarURL,
		CreatedAt:   timestamppb.New(u.CreatedAt),
		UpdatedAt:   timestamppb.New(u.UpdatedAt),
	}
}
