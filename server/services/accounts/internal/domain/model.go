// Package domain defines the core business entities and repository contracts
// for the Accounts service. It has zero dependencies on infrastructure.
package domain

import "time"

// User is the canonical internal user record.
type User struct {
	ID          string
	DisplayName string
	AvatarURL   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// UserIdentity links an external provider identity to an internal user.
type UserIdentity struct {
	ID           string
	UserID       string
	Provider     string
	ProviderID   string
	Email        string
	DisplayName  string
	AvatarURL    string
	AccessToken  string
	RefreshToken string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AuthCode is a short-lived internal authorization code for the SSO flow.
type AuthCode struct {
	Code        string
	UserID      string
	RedirectURI string
	ClientState string
	Used        bool
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// OAuthState stores the temporary state for an in-flight OAuth flow.
type OAuthState struct {
	State       string
	Provider    string
	RedirectURI string
	ClientState string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// ProviderUserInfo is the normalized user information returned by an OAuth
// provider (e.g., Discord).
type ProviderUserInfo struct {
	ProviderID  string
	Email       string
	DisplayName string
	AvatarURL   string
}
