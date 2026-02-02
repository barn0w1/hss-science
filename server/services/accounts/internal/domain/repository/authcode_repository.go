package repository

import (
	"context"
	"time"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
)

// AuthCodeRepository defines the interface for persisting auth codes.
type AuthCodeRepository interface {
	// Create stores a new auth code.
	Create(ctx context.Context, code *model.AuthCode) error

	// GetByCode retrieves an auth code by its code value.
	GetByCode(ctx context.Context, code string) (*model.AuthCode, error)

	// Consume marks an auth code as used.
	Consume(ctx context.Context, code string, consumedAt time.Time) error

	// Delete removes an auth code.
	Delete(ctx context.Context, code string) error

	// CleanupExpired deletes expired auth codes.
	CleanupExpired(ctx context.Context, cutoff time.Time) error
}
