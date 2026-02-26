package authz

import (
	"context"
	"crypto/rsa"
	"fmt"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/jmoiron/sqlx"

	"github.com/barn0w1/hss-science/server/services/accounts/internal/storage"
)

// LocalJWTVerifier verifies JWT access tokens using the OP's signing keys
// loaded directly from the shared database.
type LocalJWTVerifier struct {
	db     *sqlx.DB
	issuer string
}

// NewLocalJWTVerifier creates a verifier that validates JWTs against
// the signing keys stored in the same database as the OP.
func NewLocalJWTVerifier(db *sqlx.DB, issuer string) *LocalJWTVerifier {
	return &LocalJWTVerifier{db: db, issuer: issuer}
}

func (v *LocalJWTVerifier) Verify(_ context.Context, rawToken string) (*Claims, error) {
	tok, err := jwt.ParseSigned(rawToken, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return nil, fmt.Errorf("parse JWT: %w", err)
	}

	// Load public keys from the database.
	opKeys, err := storage.LoadAllPublicKeys(v.db)
	if err != nil {
		return nil, fmt.Errorf("load public keys: %w", err)
	}

	// Try each key to find a match.
	var claims jwt.Claims
	var verified bool
	for _, k := range opKeys {
		pubKey, ok := k.Key().(*rsa.PublicKey)
		if !ok {
			continue
		}
		if err := tok.Claims(pubKey, &claims); err == nil {
			verified = true
			break
		}
	}
	if !verified {
		return nil, fmt.Errorf("no matching signing key")
	}

	// Validate standard claims (issuer, expiry, not-before).
	expected := jwt.Expected{
		Issuer: v.issuer,
		Time:   time.Now(),
	}
	if err := claims.ValidateWithLeeway(expected, 10*time.Second); err != nil {
		return nil, fmt.Errorf("validate claims: %w", err)
	}

	if claims.Subject == "" {
		return nil, fmt.Errorf("missing subject claim")
	}

	return &Claims{
		Subject:  claims.Subject,
		Audience: claims.Audience,
	}, nil
}
