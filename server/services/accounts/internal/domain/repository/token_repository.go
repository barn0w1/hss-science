package repository

import (
	"context"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
)

// TokenRepository defines the interface for persisting Refresh Tokens.
type TokenRepository interface {
	// Save stores a new refresh token.
	Save(ctx context.Context, token *model.RefreshToken) error

	// Get retrieves a refresh token by its hash.
	Get(ctx context.Context, tokenHash string) (*model.RefreshToken, error)

	// Delete removes a refresh token (Logout).
	Delete(ctx context.Context, tokenHash string) error

	// DeleteByUserID removes all tokens for a user (Revoke all sessions).
	DeleteByUserID(ctx context.Context, userID string) error
}
