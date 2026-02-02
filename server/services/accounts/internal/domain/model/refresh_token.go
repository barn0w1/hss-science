package model

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken represents a long-lived token used to obtain new access tokens.
type RefreshToken struct {
	TokenHash string
	UserID    uuid.UUID

	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time

	UserAgent string
	IPAddress string
}

// IsExpired checks whether the refresh token is expired.
func (t *RefreshToken) IsExpired(now time.Time) bool {
	return now.After(t.ExpiresAt)
}

// IsRevoked checks whether the refresh token has been revoked.
func (t *RefreshToken) IsRevoked() bool {
	return t.RevokedAt != nil
}

// IsValid checks whether the token is not expired and not revoked.
func (t *RefreshToken) IsValid() bool {
	now := time.Now()
	return !t.IsExpired(now) && !t.IsRevoked()
}

// Revoke marks the refresh token as revoked.
func (t *RefreshToken) Revoke(now time.Time) {
	t.RevokedAt = &now
}
