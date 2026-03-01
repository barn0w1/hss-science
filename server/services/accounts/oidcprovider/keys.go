package oidcprovider

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"

	jose "github.com/go-jose/go-jose/v4"
)

// SigningKeyWithID wraps a jose.SigningKey with a derived key ID to satisfy the op.SigningKey interface.
type SigningKeyWithID struct {
	signingKey jose.SigningKey
	id         string
}

// NewSigningKey creates a signing key with a derived ID from the RSA private key.
func NewSigningKey(key *rsa.PrivateKey) *SigningKeyWithID {
	return &SigningKeyWithID{
		signingKey: jose.SigningKey{
			Algorithm: jose.RS256,
			Key:       key,
		},
		id: deriveKeyID(&key.PublicKey),
	}
}

func (k *SigningKeyWithID) SignatureAlgorithm() jose.SignatureAlgorithm {
	return k.signingKey.Algorithm
}

func (k *SigningKeyWithID) Key() any {
	return k.signingKey.Key
}

func (k *SigningKeyWithID) ID() string {
	return k.id
}

// PublicKeyWithID wraps a jose.JSONWebKey with a derived key ID to satisfy the op.Key interface.
type PublicKeyWithID struct {
	jwk jose.JSONWebKey
	id  string
}

// NewPublicKey creates a public key with a derived ID from the RSA private key's public part.
func NewPublicKey(key *rsa.PrivateKey) *PublicKeyWithID {
	return &PublicKeyWithID{
		jwk: jose.JSONWebKey{
			Key:       &key.PublicKey,
			Algorithm: string(jose.RS256),
			Use:       "sig",
		},
		id: deriveKeyID(&key.PublicKey),
	}
}

func (k *PublicKeyWithID) ID() string {
	return k.id
}

func (k *PublicKeyWithID) Algorithm() jose.SignatureAlgorithm {
	return jose.RS256
}

func (k *PublicKeyWithID) Use() string {
	return "sig"
}

func (k *PublicKeyWithID) Key() any {
	return k.jwk.Key
}

func deriveKeyID(pub *rsa.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		panic("failed to marshal public key: " + err.Error())
	}
	h := sha256.Sum256(der)
	return hex.EncodeToString(h[:8])
}
