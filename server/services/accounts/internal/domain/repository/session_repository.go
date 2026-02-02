package repository

import (
	"context"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
	"github.com/google/uuid"
)

// SessionRepository defines the interface for persisting sessions.
type SessionRepository interface {
	// Create saves a new session.
	Create(ctx context.Context, session *model.Session) error

	// GetByTokenHash retrieves a session by its token hash.
	GetByTokenHash(ctx context.Context, tokenHash string) (*model.Session, error)

	// Delete deletes a session by its token hash.
	Delete(ctx context.Context, tokenHash string) error

	// Revoke marks a session as revoked.
	Revoke(ctx context.Context, tokenHash string, revokedAt time.Time) error

	// DeleteByUserID deletes all sessions for a user.
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error

	// CleanupExpired removes expired sessions.
	CleanupExpired(ctx context.Context, cutoff time.Time) error
}
