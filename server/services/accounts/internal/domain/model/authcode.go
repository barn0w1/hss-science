package model

import (
	"time"

	"github.com/google/uuid"
)

// AuthCode represents a one-time authorization code issued by accounts service.
// Only the hash is persisted; raw code is returned at creation time.
type AuthCode struct {
	CodeHash    string    // SHA-256 hex digest of the raw code
	UserID      uuid.UUID // subject
	Audience    string    // target service (e.g. "drive")
	RedirectURI string    // validated redirect destination

	CreatedAt  time.Time
	ExpiresAt  time.Time
	ConsumedAt *time.Time
}

// NewAuthCode creates a new one-time auth code and returns the raw code.
func NewAuthCode(
	userID uuid.UUID,
	audience string,
	redirectURI string,
	ttl time.Duration,
) (*AuthCode, string, error) {
	now := time.Now()

	raw, err := GenerateToken(DefaultTokenBytes)
	if err != nil {
		return nil, "", err
	}

	return &AuthCode{
		CodeHash:    HashToken(raw),
		UserID:      userID,
		Audience:    audience,
		RedirectURI: redirectURI,
		CreatedAt:   now,
		ExpiresAt:   now.Add(ttl),
	}, raw, nil
}

// IsExpired checks whether the auth code is expired.
func (c *AuthCode) IsExpired(now time.Time) bool {
	return now.After(c.ExpiresAt)
}

// IsConsumed checks whether the auth code was already used.
func (c *AuthCode) IsConsumed() bool {
	return c.ConsumedAt != nil
}

// Consume marks the auth code as used.
func (c *AuthCode) Consume(now time.Time) {
	c.ConsumedAt = &now
}
