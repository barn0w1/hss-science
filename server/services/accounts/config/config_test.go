package config

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"testing"
)

func generateTestKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	return string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der}))
}

func generatePKCS8Key(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func validCryptoKeyHex() string {
	return hex.EncodeToString(make([]byte, 32))
}

func setRequiredEnv(t *testing.T, keyPEM string) {
	t.Helper()
	t.Setenv("ISSUER", "https://accounts.example.com")
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("CRYPTO_KEY", validCryptoKeyHex())
	t.Setenv("SIGNING_KEY_PEM", keyPEM)
	t.Setenv("GOOGLE_CLIENT_ID", "gid")
	t.Setenv("GOOGLE_CLIENT_SECRET", "gsecret")
}

func TestLoad_Success(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.Issuer != "https://accounts.example.com" {
		t.Errorf("expected issuer https://accounts.example.com, got %s", cfg.Issuer)
	}
	if cfg.SigningKey == nil {
		t.Fatal("expected signing key to be set")
	}
}

func TestLoad_CustomPort(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}
}

func TestLoad_PKCS8Key(t *testing.T) {
	pemKey := generatePKCS8Key(t)
	setRequiredEnv(t, pemKey)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SigningKey == nil {
		t.Fatal("expected signing key to be set")
	}
}

func TestLoad_MissingIssuer(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("ISSUER", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing ISSUER")
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL")
	}
}

func TestLoad_MissingCryptoKey(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("CRYPTO_KEY", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing CRYPTO_KEY")
	}
}

func TestLoad_InvalidCryptoKeyHex(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("CRYPTO_KEY", "not-hex")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid hex CRYPTO_KEY")
	}
}

func TestLoad_CryptoKeyWrongLength(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("CRYPTO_KEY", hex.EncodeToString(make([]byte, 16)))

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for wrong-length CRYPTO_KEY")
	}
}

func TestLoad_MissingSigningKey(t *testing.T) {
	setRequiredEnv(t, "")
	t.Setenv("SIGNING_KEY_PEM", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for missing SIGNING_KEY_PEM")
	}
}

func TestLoad_InvalidSigningKey(t *testing.T) {
	setRequiredEnv(t, "not-a-pem")
	t.Setenv("SIGNING_KEY_PEM", "not-a-pem")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid SIGNING_KEY_PEM")
	}
}

func TestLoad_NoUpstreamIdP(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GITHUB_CLIENT_ID", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when no upstream IdP is configured")
	}
}

func TestLoad_GitHubOnly(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	t.Setenv("GITHUB_CLIENT_ID", "ghid")
	t.Setenv("GITHUB_CLIENT_SECRET", "ghsecret")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.GitHubClientID != "ghid" {
		t.Errorf("expected GitHub client ID ghid, got %s", cfg.GitHubClientID)
	}
}

func TestParseRSAPrivateKey_UnsupportedBlockType(t *testing.T) {
	block := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: []byte("fake")})
	_, err := parseRSAPrivateKey(string(block))
	if err == nil {
		t.Fatal("expected error for unsupported PEM block type")
	}
}

func TestLoad_TokenLifetimeDefaults(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AccessTokenLifetimeMinutes != 15 {
		t.Errorf("expected default 15, got %d", cfg.AccessTokenLifetimeMinutes)
	}
	if cfg.RefreshTokenLifetimeDays != 7 {
		t.Errorf("expected default 7, got %d", cfg.RefreshTokenLifetimeDays)
	}
	if cfg.AuthRequestTTLMinutes != 30 {
		t.Errorf("expected default 30, got %d", cfg.AuthRequestTTLMinutes)
	}
}

func TestLoad_TokenLifetimeCustom(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("ACCESS_TOKEN_LIFETIME_MINUTES", "30")
	t.Setenv("REFRESH_TOKEN_LIFETIME_DAYS", "14")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AccessTokenLifetimeMinutes != 30 {
		t.Errorf("expected 30, got %d", cfg.AccessTokenLifetimeMinutes)
	}
	if cfg.RefreshTokenLifetimeDays != 14 {
		t.Errorf("expected 14, got %d", cfg.RefreshTokenLifetimeDays)
	}
}

func TestLoad_TokenLifetimeZeroFallsBackToDefault(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("ACCESS_TOKEN_LIFETIME_MINUTES", "0")
	t.Setenv("REFRESH_TOKEN_LIFETIME_DAYS", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AccessTokenLifetimeMinutes != 15 {
		t.Errorf("expected default 15 for zero, got %d", cfg.AccessTokenLifetimeMinutes)
	}
	if cfg.RefreshTokenLifetimeDays != 7 {
		t.Errorf("expected default 7 for zero, got %d", cfg.RefreshTokenLifetimeDays)
	}
}

func TestLoad_TokenLifetimeOutOfRange(t *testing.T) {
	pemKey := generateTestKey(t)

	setRequiredEnv(t, pemKey)
	t.Setenv("ACCESS_TOKEN_LIFETIME_MINUTES", "120")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for out-of-range ACCESS_TOKEN_LIFETIME_MINUTES")
	}
}

func TestLoad_RefreshTokenLifetimeOutOfRange(t *testing.T) {
	pemKey := generateTestKey(t)

	setRequiredEnv(t, pemKey)
	t.Setenv("REFRESH_TOKEN_LIFETIME_DAYS", "365")
	_, err := Load()
	if err == nil {
		t.Fatal("expected error for out-of-range REFRESH_TOKEN_LIFETIME_DAYS")
	}
}

func TestLoad_AuthRequestTTLDefault(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthRequestTTLMinutes != 30 {
		t.Errorf("expected default 30, got %d", cfg.AuthRequestTTLMinutes)
	}
}

func TestLoad_AuthRequestTTLCustom(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("AUTH_REQUEST_TTL_MINUTES", "10")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthRequestTTLMinutes != 10 {
		t.Errorf("expected 10, got %d", cfg.AuthRequestTTLMinutes)
	}
}

func TestLoad_AuthRequestTTLZeroFallsBackToDefault(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("AUTH_REQUEST_TTL_MINUTES", "0")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthRequestTTLMinutes != 30 {
		t.Errorf("expected default 30 for zero, got %d", cfg.AuthRequestTTLMinutes)
	}
}

func TestLoad_AuthRequestTTLOutOfRange(t *testing.T) {
	pemKey := generateTestKey(t)
	setRequiredEnv(t, pemKey)
	t.Setenv("AUTH_REQUEST_TTL_MINUTES", "120")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for out-of-range AUTH_REQUEST_TTL_MINUTES")
	}
}
