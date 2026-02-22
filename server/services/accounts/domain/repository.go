package domain

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrStateNotFound    = errors.New("oauth state not found or expired")
	ErrAuthCodeNotFound = errors.New("auth code not found, expired, or already used")
)

// UserRepository manages user persistence and external account linking.
type UserRepository interface {
	// UpsertByProvider creates or updates a user based on their external account.
	// If an external account with the same (provider, provider_user_id) exists,
	// the user's profile and tokens are updated. Otherwise, a new user and
	// external account are created within a single transaction.
	// displayName and avatarURL are used for the user-level profile.
	UpsertByProvider(ctx context.Context, account *ExternalAccount, displayName, avatarURL string) (*User, error)

	// GetByID retrieves a user by their internal UUID.
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
}

// StateRepository manages temporary OAuth state for CSRF protection.
type StateRepository interface {
	// Create stores a new OAuth state entry.
	Create(ctx context.Context, state *OAuthState) error

	// Consume atomically retrieves and deletes a state by its value.
	// Returns ErrStateNotFound if the state does not exist or is expired.
	Consume(ctx context.Context, stateValue string) (*OAuthState, error)

	// DeleteExpired removes states that have passed their expiration time.
	DeleteExpired(ctx context.Context) (int64, error)
}

// AuthCodeRepository manages short-lived internal authorization codes.
type AuthCodeRepository interface {
	// Create stores a new auth code.
	Create(ctx context.Context, code *AuthCode) error

	// Consume atomically retrieves and marks a code as used.
	// Returns ErrAuthCodeNotFound if the code does not exist, is expired,
	// or has already been used.
	Consume(ctx context.Context, codeValue string) (*AuthCode, error)

	// DeleteExpired removes codes that have passed their expiration time.
	DeleteExpired(ctx context.Context) (int64, error)
}
