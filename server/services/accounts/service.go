package accounts

import (
	"context"
	"log/slog"
	"time"

	accountsv1 "github.com/barn0w1/hss-science/server/gen/accounts/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UserRepository defines the data access contract consumed by the service.
// Defined at the consumer site per Go convention.
type UserRepository interface {
	UpsertUser(ctx context.Context, googleID, email, name, picture string) (string, error)
	CreateSession(ctx context.Context, userID, deviceIP, deviceUA string, expiresAt time.Time) (string, error)
}

// Service implements the AccountsServiceServer gRPC interface.
type Service struct {
	accountsv1.UnimplementedAccountsServiceServer
	repo UserRepository
	jwt  *JWTMinter
	log  *slog.Logger
}

// NewService creates a new Accounts gRPC service.
func NewService(repo UserRepository, jwt *JWTMinter, log *slog.Logger) *Service {
	return &Service{repo: repo, jwt: jwt, log: log}
}

// LoginUser performs JIT user provisioning, creates a session, and mints JWTs.
func (s *Service) LoginUser(ctx context.Context, req *accountsv1.LoginUserRequest) (*accountsv1.LoginUserResponse, error) {
	if req.GetGoogleId() == "" {
		return nil, status.Error(codes.InvalidArgument, "google_id is required")
	}
	if req.GetEmail() == "" {
		return nil, status.Error(codes.InvalidArgument, "email is required")
	}

	userID, err := s.repo.UpsertUser(ctx, req.GetGoogleId(), req.GetEmail(), req.GetName(), req.GetPicture())
	if err != nil {
		s.log.ErrorContext(ctx, "failed to upsert user", "error", err)
		return nil, status.Error(codes.Internal, "failed to provision user")
	}

	var deviceIP, deviceUA string
	if di := req.GetDeviceInfo(); di != nil {
		deviceIP = di.GetIpAddress()
		deviceUA = di.GetUserAgent()
	}

	sessionID, err := s.repo.CreateSession(ctx, userID, deviceIP, deviceUA, time.Now().Add(s.jwt.RefreshTTL()))
	if err != nil {
		s.log.ErrorContext(ctx, "failed to create session", "error", err, "user_id", userID)
		return nil, status.Error(codes.Internal, "failed to create session")
	}

	accessToken, err := s.jwt.MintAccessToken(userID, req.GetEmail(), req.GetName(), req.GetPicture())
	if err != nil {
		s.log.ErrorContext(ctx, "failed to mint access token", "error", err, "user_id", userID)
		return nil, status.Error(codes.Internal, "failed to mint tokens")
	}

	refreshToken, err := s.jwt.MintRefreshToken(userID, sessionID)
	if err != nil {
		s.log.ErrorContext(ctx, "failed to mint refresh token", "error", err, "user_id", userID)
		return nil, status.Error(codes.Internal, "failed to mint tokens")
	}

	s.log.InfoContext(ctx, "user logged in", "user_id", userID, "session_id", sessionID)

	return &accountsv1.LoginUserResponse{
		UserId:       userID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}
