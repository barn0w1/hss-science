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

	// GetByID retrieves a session by its ID.
	GetByID(ctx context.Context, id uuid.UUID) (*model.Session, error)

	// Delete deletes a session by its ID.
	Delete(ctx context.Context, id uuid.UUID) error

	// Revoke marks a session as revoked.
	Revoke(ctx context.Context, id uuid.UUID, revokedAt time.Time) error

	// DeleteByUserID deletes all sessions for a user.
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error

	// CleanupExpired removes expired sessions.
	CleanupExpired(ctx context.Context, cutoff time.Time) error
}
