package model

import (
	"time"

	"github.com/google/uuid"
)

// Session represents an accounts-level login state.
// It proves that a user exists and has authenticated via OAuth.
type Session struct {
	ID     uuid.UUID // session id (cookie value)
	UserID uuid.UUID

	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time

	// Optional (security / audit)
	UserAgent string
	IPAddress string
}

// NewSession creates a new session for a user.
func NewSession(
	userID uuid.UUID,
	ttl time.Duration,
	userAgent string,
	ipAddress string,
) *Session {
	now := time.Now()

	return &Session{
		ID:        uuid.New(),
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
		RevokedAt: nil,
		UserAgent: userAgent,
		IPAddress: ipAddress,
	}
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
