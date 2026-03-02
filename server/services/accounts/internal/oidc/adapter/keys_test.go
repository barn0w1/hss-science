package adapter

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"

	jose "github.com/go-jose/go-jose/v4"
)

func testRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestSigningKey_Interface(t *testing.T) {
	key := testRSAKey(t)
	sk := NewSigningKey(key)

	if sk.SignatureAlgorithm() != jose.RS256 {
		t.Errorf("expected RS256, got %s", sk.SignatureAlgorithm())
	}
	if sk.ID() == "" {
		t.Error("expected non-empty key ID")
	}
	if sk.Key() == nil {
		t.Error("expected non-nil key")
	}
	if _, ok := sk.Key().(*rsa.PrivateKey); !ok {
		t.Error("expected key to be *rsa.PrivateKey")
	}
}

func TestPublicKey_Interface(t *testing.T) {
	key := testRSAKey(t)
	pk := NewPublicKey(key)

	if pk.ID() == "" {
		t.Error("expected non-empty key ID")
	}
	if pk.Algorithm() != jose.RS256 {
		t.Errorf("expected RS256, got %s", pk.Algorithm())
	}
	if pk.Use() != "sig" {
		t.Errorf("expected use=sig, got %s", pk.Use())
	}
	if pk.Key() == nil {
		t.Error("expected non-nil key")
	}
	if _, ok := pk.Key().(*rsa.PublicKey); !ok {
		t.Error("expected key to be *rsa.PublicKey")
	}
}

func TestDeriveKeyID_Deterministic(t *testing.T) {
	key := testRSAKey(t)
	id1 := deriveKeyID(&key.PublicKey)
	id2 := deriveKeyID(&key.PublicKey)
	if id1 != id2 {
		t.Errorf("expected deterministic key ID, got %s and %s", id1, id2)
	}
	if len(id1) != 16 {
		t.Errorf("expected 16-char hex key ID, got %d chars", len(id1))
	}
}

func TestDeriveKeyID_DifferentKeys(t *testing.T) {
	key1 := testRSAKey(t)
	key2 := testRSAKey(t)
	id1 := deriveKeyID(&key1.PublicKey)
	id2 := deriveKeyID(&key2.PublicKey)
	if id1 == id2 {
		t.Error("expected different key IDs for different keys")
	}
}

func TestSigningAndPublicKey_SameID(t *testing.T) {
	key := testRSAKey(t)
	sk := NewSigningKey(key)
	pk := NewPublicKey(key)
	if sk.ID() != pk.ID() {
		t.Errorf("expected same ID for signing and public key, got %s and %s", sk.ID(), pk.ID())
	}
}
