package repository

import (
	"context"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/domain/model"
)

// UserRepository defines the interface for persisting User data.
type UserRepository interface {
	// Create saves a new user to the storage.
	Create(ctx context.Context, user *model.User) error

	// Update updates an existing user's profile
	Update(ctx context.Context, user *model.User) error

	// GetByID retrieves a user by their internal UUID.
	GetByID(ctx context.Context, id string) (*model.User, error)

	// GetByDiscordID retrieves a user by their Discord Snowflake ID.
	GetByDiscordID(ctx context.Context, discordID string) (*model.User, error)
}
