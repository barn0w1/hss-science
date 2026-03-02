package crypto

import (
	"crypto/rand"
	"testing"
)

func testKey(t *testing.T) [32]byte {
	t.Helper()
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		t.Fatal(err)
	}
	return key
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := testKey(t)
	plaintext := []byte("hello world")

	encoded, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if encoded == "" {
		t.Fatal("expected non-empty encoded string")
	}

	got, err := Decrypt(key, encoded)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Errorf("expected %q, got %q", plaintext, got)
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := testKey(t)
	key2 := testKey(t)

	encoded, err := Encrypt(key1, []byte("secret"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = Decrypt(key2, encoded)
	if err == nil {
		t.Fatal("expected error when decrypting with wrong key")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	key := testKey(t)
	_, err := Decrypt(key, "not-valid-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestDecrypt_TooShort(t *testing.T) {
	key := testKey(t)
	_, err := Decrypt(key, "AQID")
	if err == nil {
		t.Fatal("expected error for ciphertext too short")
	}
}
