package model

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
)

const DefaultTokenBytes = 32

// GenerateToken returns a base64url-encoded, cryptographically secure token.
func GenerateToken(bytes int) (string, error) {
	if bytes <= 0 {
		return "", errors.New("token length must be positive")
	}
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken returns the SHA-256 hex digest of the raw token.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
