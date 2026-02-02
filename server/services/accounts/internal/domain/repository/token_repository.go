package repository

import (
	"context"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
	"github.com/google/uuid"
)

// TokenRepository defines the interface for persisting refresh tokens.
type TokenRepository interface {
	// Save stores a new refresh token.
	Save(ctx context.Context, token *model.RefreshToken) error

	// Get retrieves a refresh token by its hash.
	Get(ctx context.Context, tokenHash string) (*model.RefreshToken, error)

	// Revoke marks a refresh token as revoked.
	Revoke(ctx context.Context, tokenHash string) error

	// RevokeByUserID revokes all refresh tokens for a user.
	RevokeByUserID(ctx context.Context, userID uuid.UUID) error

	// CleanupExpired deletes expired refresh tokens.
	CleanupExpired(ctx context.Context, cutoff time.Time) error
}
