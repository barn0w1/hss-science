package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents an internal system user, independent of any external IdP.
type User struct {
	ID          uuid.UUID
	DisplayName string
	AvatarURL   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ExternalAccount represents a linked identity from an external OAuth provider.
type ExternalAccount struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	Provider         string
	ProviderUserID   string
	ProviderUsername string
	ProviderEmail    *string
	AccessToken      string
	RefreshToken     *string
	TokenExpiry      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// OAuthState holds temporary state for an in-progress OAuth authorization flow.
type OAuthState struct {
	State       string
	Provider    string
	RedirectURI string
	ClientState string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}

// AuthCode is a short-lived, single-use internal authorization code issued
// after successful authentication. Downstream BFFs exchange this for user info.
type AuthCode struct {
	Code      string
	UserID    uuid.UUID
	Used      bool
	ExpiresAt time.Time
	CreatedAt time.Time
}
