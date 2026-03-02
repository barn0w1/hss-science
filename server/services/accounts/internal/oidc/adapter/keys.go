package adapter

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"

	jose "github.com/go-jose/go-jose/v4"
)

type SigningKeyWithID struct {
	signingKey jose.SigningKey
	id         string
}

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

type PublicKeyWithID struct {
	jwk jose.JSONWebKey
	id  string
}

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
