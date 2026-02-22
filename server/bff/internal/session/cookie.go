package session

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/securecookie"
)

// Data holds the session payload stored in the encrypted cookie.
type Data struct {
	UserID   string `json:"uid"`
	IssuedAt int64  `json:"iat"`
}

// Manager handles encrypted cookie-based session management.
type Manager struct {
	sc     *securecookie.SecureCookie
	name   string
	maxAge int
	secure bool
}

// New creates a session Manager with the given encryption keys.
// hashKey should be 32 or 64 bytes for HMAC-SHA256.
// blockKey should be 16, 24, or 32 bytes for AES encryption.
func New(hashKey, blockKey []byte, maxAge int, secure bool) *Manager {
	sc := securecookie.New(hashKey, blockKey)
	sc.MaxAge(maxAge)
	return &Manager{
		sc:     sc,
		name:   "accounts_session",
		maxAge: maxAge,
		secure: secure,
	}
}

// Encode returns the Set-Cookie header string for the given session data.
func (m *Manager) Encode(data *Data) (string, error) {
	encoded, err := m.sc.Encode(m.name, data)
	if err != nil {
		return "", fmt.Errorf("encode session: %w", err)
	}

	cookie := &http.Cookie{
		Name:     m.name,
		Value:    encoded,
		Path:     "/",
		MaxAge:   m.maxAge,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	}
	return cookie.String(), nil
}

// Decode reads the session data from the request cookie.
// Returns nil (no error) if no session cookie is present.
func (m *Manager) Decode(r *http.Request) (*Data, error) {
	cookie, err := r.Cookie(m.name)
	if err != nil {
		return nil, nil // No cookie present — not an error.
	}

	var data Data
	if err := m.sc.Decode(m.name, cookie.Value, &data); err != nil {
		return nil, fmt.Errorf("decode session: %w", err)
	}

	return &data, nil
}

// ClearCookie returns a Set-Cookie string that clears the session.
func (m *Manager) ClearCookie() string {
	cookie := &http.Cookie{
		Name:     m.name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.secure,
		SameSite: http.SameSiteLaxMode,
	}
	return cookie.String()
}

// IsValid checks if the session data has not expired based on the maxAge.
func (m *Manager) IsValid(data *Data) bool {
	if data == nil {
		return false
	}
	issuedAt := time.Unix(data.IssuedAt, 0)
	return time.Since(issuedAt) < time.Duration(m.maxAge)*time.Second
}
