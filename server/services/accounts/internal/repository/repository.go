package repository

import (
	"context"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/storage"
)

// AccountRepository defines the data access operations needed
// by the gRPC account management service.
type AccountRepository interface {
	GetUser(ctx context.Context, userID string) (*storage.User, error)
	UpdateUser(ctx context.Context, userID string, fields map[string]any) (*storage.User, error)
	ListFederatedIdentities(ctx context.Context, userID string) ([]storage.FederatedIdentity, error)
	CountFederatedIdentities(ctx context.Context, userID string) (int, error)
	DeleteFederatedIdentity(ctx context.Context, userID, identityID string) error
	ListRefreshTokens(ctx context.Context, userID string) ([]storage.RefreshToken, error)
	DeleteRefreshToken(ctx context.Context, userID, tokenID string) error
	DeleteUser(ctx context.Context, userID string) error
}
