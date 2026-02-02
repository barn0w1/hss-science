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

	// GetByCodeHash retrieves an auth code by its hash.
	GetByCodeHash(ctx context.Context, codeHash string) (*model.AuthCode, error)

	// Consume marks an auth code as used.
	Consume(ctx context.Context, codeHash string, consumedAt time.Time) error

	// Delete removes an auth code.
	Delete(ctx context.Context, codeHash string) error

	// CleanupExpired deletes expired auth codes.
	CleanupExpired(ctx context.Context, cutoff time.Time) error
}
