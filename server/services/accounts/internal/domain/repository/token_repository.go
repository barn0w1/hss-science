package repository

import (
	"context"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
)

// TokenRepository defines the interface for persisting Refresh Tokens.
type TokenRepository interface {
	// Save stores a new refresh token.
	Save(ctx context.Context, token *model.RefreshToken) error

	// Get retrieves a refresh token by its hash.
	Get(ctx context.Context, tokenHash string) (*model.RefreshToken, error)

	// Revoke marks a refresh token as revoked (Logout).
	// Instead of physical deletion, we update the revoked_at timestamp.
	Revoke(ctx context.Context, tokenHash string) error

	// RevokeByUserID marks all tokens for a user as revoked (Revoke all sessions).
	RevokeByUserID(ctx context.Context, userID string) error

	// CleanupExpired deletes tokens that are expired AND older than retention period.
	// This should be called by a background worker/cron.
	CleanupExpired(ctx context.Context, cutoff time.Time) error
}
