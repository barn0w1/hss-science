package repository

import (
	"context"
)

// OAuthUserInfo represents the user data retrieved from the OAuth provider.
type OAuthUserInfo struct {
	DiscordID string
	Name      string
	AvatarURL string
}

// OAuthProvider defines the interface for interacting with the OAuth service (Discord).
type OAuthProvider interface {
	// GetUserInfo exchanges the auth code for an access token and retrieves user info.
	GetUserInfo(ctx context.Context, code string) (*OAuthUserInfo, error)
	// GetAuthURL constructs the authorization URL for initiating the OAuth flow.
	GetAuthURL(redirectURL, state string) string
}
