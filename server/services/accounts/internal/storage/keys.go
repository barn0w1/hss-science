package storage

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/zitadel/oidc/v3/pkg/op"
)

// signingKey wraps an RSA private key and implements the op.SigningKey interface.
type signingKey struct {
	id        string
	algorithm jose.SignatureAlgorithm
	key       *rsa.PrivateKey
}

func (s *signingKey) SignatureAlgorithm() jose.SignatureAlgorithm {
	return s.algorithm
}

func (s *signingKey) Key() any {
	return s.key
}

func (s *signingKey) ID() string {
	return s.id
}

// publicKey wraps the public half of a signing key and implements the op.Key interface.
type publicKey struct {
	signingKey
}

func (p *publicKey) Algorithm() jose.SignatureAlgorithm {
	return p.algorithm
}

func (p *publicKey) Use() string {
	return "sig"
}

func (p *publicKey) Key() any {
	return &p.key.PublicKey
}

func (p *publicKey) ID() string {
	return p.id
}

// signingKeyRow matches the signing_keys database table.
type signingKeyRow struct {
	ID            string `db:"id"`
	Algorithm     string `db:"algorithm"`
	PrivateKeyPEM []byte `db:"private_key_pem"`
	PublicKeyPEM  []byte `db:"public_key_pem"`
	Active        bool   `db:"active"`
}

// LoadOrGenerateSigningKey loads the active signing key from the database.
// If no active key exists, it generates a new RSA 2048 key pair and stores it.
func LoadOrGenerateSigningKey(db *sqlx.DB) (*signingKey, error) {
	var row signingKeyRow
	err := db.Get(&row, `SELECT id, algorithm, private_key_pem, public_key_pem, active
		FROM signing_keys WHERE active = true ORDER BY created_at DESC LIMIT 1`)
	if err == nil {
		return parseSigningKeyRow(&row)
	}

	// No active key found; generate a new one.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA key: %w", err)
	}

	privPEM := encodePrivateKey(privateKey)
	pubPEM := encodePublicKey(&privateKey.PublicKey)
	keyID := uuid.NewString()

	_, err = db.Exec(`INSERT INTO signing_keys (id, algorithm, private_key_pem, public_key_pem, active)
		VALUES ($1, $2, $3, $4, true)
		ON CONFLICT DO NOTHING`,
		keyID, "RS256", privPEM, pubPEM)
	if err != nil {
		return nil, fmt.Errorf("insert signing key: %w", err)
	}

	// Re-read to handle the race case where another instance inserted first.
	err = db.Get(&row, `SELECT id, algorithm, private_key_pem, public_key_pem, active
		FROM signing_keys WHERE active = true ORDER BY created_at DESC LIMIT 1`)
	if err != nil {
		return nil, fmt.Errorf("load signing key after insert: %w", err)
	}

	return parseSigningKeyRow(&row)
}

// LoadAllPublicKeys returns all non-expired signing keys for the JWKS endpoint.
func LoadAllPublicKeys(db *sqlx.DB) ([]op.Key, error) {
	var rows []signingKeyRow
	err := db.Select(&rows, `SELECT id, algorithm, private_key_pem, public_key_pem, active
		FROM signing_keys WHERE expires_at IS NULL OR expires_at > now()`)
	if err != nil {
		return nil, fmt.Errorf("load public keys: %w", err)
	}

	keys := make([]op.Key, len(rows))
	for i := range rows {
		sk, err := parseSigningKeyRow(&rows[i])
		if err != nil {
			return nil, err
		}
		keys[i] = &publicKey{*sk}
	}
	return keys, nil
}

func parseSigningKeyRow(row *signingKeyRow) (*signingKey, error) {
	privKey, err := decodePrivateKey(row.PrivateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("decode private key %s: %w", row.ID, err)
	}
	return &signingKey{
		id:        row.ID,
		algorithm: jose.SignatureAlgorithm(row.Algorithm),
		key:       privKey,
	}, nil
}

func encodePrivateKey(key *rsa.PrivateKey) []byte {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
}

func encodePublicKey(key *rsa.PublicKey) []byte {
	pubBytes, err := x509.MarshalPKIXPublicKey(key)
	if err != nil {
		return nil
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})
}

func decodePrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}
