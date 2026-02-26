package grpcserver

import (
	"context"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/grpcserver/authz"
	"github.com/barn0w1/hss-science/server/services/accounts/internal/repository"
)

// Compile-time check.
var _ pb.AccountsServiceServer = (*Server)(nil)

// Server implements the AccountsService gRPC server.
type Server struct {
	pb.UnimplementedAccountsServiceServer
	repo   repository.AccountRepository
	logger *slog.Logger
}

// NewServer creates a new gRPC accounts server.
func NewServer(repo repository.AccountRepository, logger *slog.Logger) *Server {
	return &Server{repo: repo, logger: logger}
}

func (s *Server) GetProfile(ctx context.Context, _ *pb.GetProfileRequest) (*pb.GetProfileResponse, error) {
	userID, err := authz.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "get profile failed", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Internal, "get profile: %v", err)
	}

	return &pb.GetProfileResponse{Profile: userToProto(user)}, nil
}

// allowedMaskPaths defines which FieldMask paths are valid for UpdateProfile.
var allowedMaskPaths = map[string]string{
	"given_name":  "given_name",
	"family_name": "family_name",
	"picture":     "picture",
	"locale":      "locale",
}

func (s *Server) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UpdateProfileResponse, error) {
	userID, err := authz.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if req.Profile == nil {
		return nil, status.Error(codes.InvalidArgument, "profile is required")
	}
	if req.UpdateMask == nil || len(req.UpdateMask.Paths) == 0 {
		return nil, status.Error(codes.InvalidArgument, "update_mask is required")
	}

	fields := make(map[string]any, len(req.UpdateMask.Paths))
	for _, path := range req.UpdateMask.Paths {
		col, ok := allowedMaskPaths[path]
		if !ok {
			return nil, status.Errorf(codes.InvalidArgument, "field %q is not updatable", path)
		}
		switch path {
		case "given_name":
			fields[col] = req.Profile.GivenName
		case "family_name":
			fields[col] = req.Profile.FamilyName
		case "picture":
			fields[col] = req.Profile.Picture
		case "locale":
			fields[col] = req.Profile.Locale
		}
	}

	user, err := s.repo.UpdateUser(ctx, userID, fields)
	if err != nil {
		s.logger.ErrorContext(ctx, "update profile failed", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Internal, "update profile: %v", err)
	}

	return &pb.UpdateProfileResponse{Profile: userToProto(user)}, nil
}

func (s *Server) ListLinkedAccounts(ctx context.Context, _ *pb.ListLinkedAccountsRequest) (*pb.ListLinkedAccountsResponse, error) {
	userID, err := authz.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	identities, err := s.repo.ListFederatedIdentities(ctx, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "list linked accounts failed", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Internal, "list linked accounts: %v", err)
	}

	accounts := make([]*pb.LinkedAccount, len(identities))
	for i := range identities {
		accounts[i] = linkedAccountToProto(&identities[i])
	}

	return &pb.ListLinkedAccountsResponse{LinkedAccounts: accounts}, nil
}

func (s *Server) UnlinkAccount(ctx context.Context, req *pb.UnlinkAccountRequest) (*pb.UnlinkAccountResponse, error) {
	userID, err := authz.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if req.LinkedAccountId == "" {
		return nil, status.Error(codes.InvalidArgument, "linked_account_id is required")
	}

	// Enforce minimum-one-identity constraint.
	count, err := s.repo.CountFederatedIdentities(ctx, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "count identities failed", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Internal, "count identities: %v", err)
	}
	if count <= 1 {
		return nil, status.Error(codes.FailedPrecondition, "cannot unlink the only remaining identity")
	}

	if err := s.repo.DeleteFederatedIdentity(ctx, userID, req.LinkedAccountId); err != nil {
		s.logger.ErrorContext(ctx, "unlink account failed", "user_id", userID, "identity_id", req.LinkedAccountId, "error", err)
		return nil, status.Errorf(codes.Internal, "unlink account: %v", err)
	}

	return &pb.UnlinkAccountResponse{}, nil
}

func (s *Server) ListActiveSessions(ctx context.Context, _ *pb.ListActiveSessionsRequest) (*pb.ListActiveSessionsResponse, error) {
	userID, err := authz.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	tokens, err := s.repo.ListRefreshTokens(ctx, userID)
	if err != nil {
		s.logger.ErrorContext(ctx, "list sessions failed", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Internal, "list sessions: %v", err)
	}

	sessions := make([]*pb.Session, len(tokens))
	for i := range tokens {
		sessions[i] = sessionToProto(&tokens[i])
	}

	return &pb.ListActiveSessionsResponse{Sessions: sessions}, nil
}

func (s *Server) RevokeSession(ctx context.Context, req *pb.RevokeSessionRequest) (*pb.RevokeSessionResponse, error) {
	userID, err := authz.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if req.SessionId == "" {
		return nil, status.Error(codes.InvalidArgument, "session_id is required")
	}

	if err := s.repo.DeleteRefreshToken(ctx, userID, req.SessionId); err != nil {
		s.logger.ErrorContext(ctx, "revoke session failed", "user_id", userID, "session_id", req.SessionId, "error", err)
		return nil, status.Errorf(codes.NotFound, "session not found")
	}

	return &pb.RevokeSessionResponse{}, nil
}

func (s *Server) DeleteAccount(ctx context.Context, _ *pb.DeleteAccountRequest) (*pb.DeleteAccountResponse, error) {
	userID, err := authz.UserIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	if err := s.repo.DeleteUser(ctx, userID); err != nil {
		s.logger.ErrorContext(ctx, "delete account failed", "user_id", userID, "error", err)
		return nil, status.Errorf(codes.Internal, "delete account: %v", err)
	}

	s.logger.InfoContext(ctx, "account deleted", "user_id", userID)
	return &pb.DeleteAccountResponse{}, nil
}
