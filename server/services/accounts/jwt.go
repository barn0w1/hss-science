package accounts

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTMinter signs JWTs using an Ed25519 private key.
type JWTMinter struct {
	privateKey ed25519.PrivateKey
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewJWTMinter loads an Ed25519 private key from the given PEM file path
// and returns a minter configured with the specified parameters.
func NewJWTMinter(privateKeyPath, issuer string, accessTTL, refreshTTL time.Duration) (*JWTMinter, error) {
	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", privateKeyPath)
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	privKey, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not Ed25519: got %T", parsed)
	}

	return &JWTMinter{
		privateKey: privKey,
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}, nil
}

// AccessTokenClaims extends RegisteredClaims with user profile fields.
type AccessTokenClaims struct {
	jwt.RegisteredClaims
	Email   string `json:"email"`
	Name    string `json:"name"`
	Picture string `json:"picture"`
}

// RefreshTokenClaims extends RegisteredClaims with a session reference.
type RefreshTokenClaims struct {
	jwt.RegisteredClaims
	SessionID string `json:"sid"`
}

// MintAccessToken creates a short-lived JWT containing user profile claims.
func (m *JWTMinter) MintAccessToken(userID, email, name, picture string) (string, error) {
	now := time.Now()
	claims := AccessTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
		Email:   email,
		Name:    name,
		Picture: picture,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return token.SignedString(m.privateKey)
}

// MintRefreshToken creates a long-lived JWT containing only the session reference.
func (m *JWTMinter) MintRefreshToken(userID, sessionID string) (string, error) {
	now := time.Now()
	claims := RefreshTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			ID:        uuid.NewString(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.refreshTTL)),
		},
		SessionID: sessionID,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return token.SignedString(m.privateKey)
}

// RefreshTTL returns the refresh token time-to-live, used to set session expiry.
func (m *JWTMinter) RefreshTTL() time.Duration {
	return m.refreshTTL
}
