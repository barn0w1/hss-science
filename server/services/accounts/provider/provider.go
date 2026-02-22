package provider

import (
	"context"

	"golang.org/x/oauth2"
)

// UserInfo holds user profile data fetched from an external OAuth provider.
type UserInfo struct {
	ProviderUserID string
	Username       string
	Email          *string
	DisplayName    string
	AvatarURL      string
}

// OAuthProvider abstracts the OAuth2 flow for a specific external identity provider.
type OAuthProvider interface {
	// Name returns the provider identifier (e.g., "discord").
	Name() string

	// AuthCodeURL returns the URL to redirect the user to for authorization.
	AuthCodeURL(state string) string

	// Exchange trades an authorization code for an OAuth2 token.
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)

	// FetchUserInfo retrieves user profile data using the given token.
	FetchUserInfo(ctx context.Context, token *oauth2.Token) (*UserInfo, error)
}
