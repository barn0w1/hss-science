package domain

import (
	"context"
)

// UserRepository manages User persistence.
type UserRepository interface {
	// UpsertByIdentity creates or updates a user based on the external provider
	// identity. Returns the internal user. If a user_identity for the given
	// (provider, provider_id) already exists, it updates the linked user's
	// profile. Otherwise, it creates both a new user and identity row.
	UpsertByIdentity(ctx context.Context, provider string, info *ProviderUserInfo, accessToken, refreshToken string) (*User, error)

	// GetByID retrieves a user by internal ID.
	GetByID(ctx context.Context, id string) (*User, error)
}

// AuthCodeRepository manages short-lived authorization codes.
type AuthCodeRepository interface {
	// Create persists a new authorization code.
	Create(ctx context.Context, code *AuthCode) error

	// Consume looks up an authorization code, marks it as used, and returns it.
	// Returns an error if the code does not exist, is expired, or was already used.
	Consume(ctx context.Context, code string) (*AuthCode, error)
}

// OAuthStateRepository manages temporary OAuth state values.
type OAuthStateRepository interface {
	// Create persists a new OAuth state.
	Create(ctx context.Context, state *OAuthState) error

	// Consume looks up an OAuth state, deletes it, and returns it.
	// Returns an error if the state does not exist or is expired.
	Consume(ctx context.Context, state string) (*OAuthState, error)
}

// OAuthProvider abstracts communication with an external OAuth provider.
type OAuthProvider interface {
	// Name returns the provider identifier (e.g., "discord").
	Name() string

	// AuthURL returns the authorization URL for the given state parameter.
	AuthURL(state string) string

	// Exchange exchanges an authorization code for provider user info and tokens.
	Exchange(ctx context.Context, code string) (*ProviderUserInfo, string, string, error)
}
