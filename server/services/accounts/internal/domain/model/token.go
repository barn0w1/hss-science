package model

import (
	"time"
)

// RefreshToken represents a user session for renewing access tokens.
type RefreshToken struct {
	TokenHash string // Hashed token
	UserID    string
	ExpiresAt time.Time
	CreatedAt time.Time

	// Audit info (Optional but useful)
	UserAgent string
	IPAddress string
}

// IsExpired checks if the token is past its expiration time.
func (t *RefreshToken) IsExpired() bool {
	return time.Now().After(t.ExpiresAt)
}
