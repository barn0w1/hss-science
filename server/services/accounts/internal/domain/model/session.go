package model

import (
	"time"

	"github.com/google/uuid"
)

// Session represents an accounts-level login state.
// Only the token hash is persisted; raw token is returned at creation time.
type Session struct {
	TokenHash string
	UserID    uuid.UUID

	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time

	// Optional (security / audit)
	UserAgent string
	IPAddress string
}

// NewSession creates a new session and returns the raw token.
func NewSession(
	userID uuid.UUID,
	ttl time.Duration,
	userAgent string,
	ipAddress string,
) (*Session, string, error) {
	now := time.Now()

	raw, err := GenerateToken(DefaultTokenBytes)
	if err != nil {
		return nil, "", err
	}

	return &Session{
		TokenHash: HashToken(raw),
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		RevokedAt: nil,
		UserAgent: userAgent,
		IPAddress: ipAddress,
	}, raw, nil
}

// IsExpired checks whether the session is expired.
func (s *Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// IsRevoked checks whether the session is revoked.
func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

// IsValid checks whether the session is active.
func (s *Session) IsValid(now time.Time) bool {
	return !s.IsExpired(now) && !s.IsRevoked()
}

// Revoke marks the session as revoked.
func (s *Session) Revoke(now time.Time) {
	s.RevokedAt = &now
}
