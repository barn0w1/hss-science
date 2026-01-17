package model

import (
	"time"
)

// RefreshToken represents a user session for renewing access tokens.
type RefreshToken struct {
	TokenHash string // Primary Key
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time

	RevokedAt *time.Time

	// Audit info
	UserAgent string
	IPAddress string
}

// IsExpired checks if the token is past its expiration time.
func (t *RefreshToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}

// IsRevoked checks if the token has been explicitly revoked.
func (t *RefreshToken) IsRevoked() bool {
	return t.RevokedAt != nil
}

// IsValid checks if the token can be used.
func (t *RefreshToken) IsValid() bool {
	return !t.IsExpired() && !t.IsRevoked()
}
