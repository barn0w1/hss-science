package oidcprovider

import (
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"

	jose "github.com/go-jose/go-jose/v4"
)

type signingKey struct {
	id         string
	algorithm  jose.SignatureAlgorithm
	privateKey *rsa.PrivateKey
}

func NewSigningKey(key *rsa.PrivateKey) *signingKey {
	return &signingKey{
		id:         deriveKeyID(&key.PublicKey),
		algorithm:  jose.RS256,
		privateKey: key,
	}
}

func (k *signingKey) SignatureAlgorithm() jose.SignatureAlgorithm { return k.algorithm }
func (k *signingKey) Key() any                                    { return k.privateKey }
func (k *signingKey) ID() string                                  { return k.id }

type publicKey struct {
	id        string
	algorithm jose.SignatureAlgorithm
	key       *rsa.PublicKey
}

func NewPublicKey(key *rsa.PrivateKey) *publicKey {
	return &publicKey{
		id:        deriveKeyID(&key.PublicKey),
		algorithm: jose.RS256,
		key:       &key.PublicKey,
	}
}

func (k *publicKey) ID() string                         { return k.id }
func (k *publicKey) Algorithm() jose.SignatureAlgorithm { return k.algorithm }
func (k *publicKey) Use() string                        { return "sig" }
func (k *publicKey) Key() any                           { return k.key }

func deriveKeyID(pub *rsa.PublicKey) string {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		panic("failed to marshal public key: " + err.Error())
	}
	h := sha256.Sum256(der)
	return hex.EncodeToString(h[:8])
}
