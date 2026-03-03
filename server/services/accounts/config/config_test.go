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

func requiredEnv(keyPEM string) MapSource {
	return MapSource{
		"ISSUER":               "https://accounts.example.com",
		"DATABASE_URL":         "postgres://localhost/test",
		"CRYPTO_KEY":           validCryptoKeyHex(),
		"SIGNING_KEY_PEM":      keyPEM,
		"GOOGLE_CLIENT_ID":     "gid",
		"GOOGLE_CLIENT_SECRET": "gsecret",
	}
}

func TestLoadFrom_Success(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)

	cfg, err := LoadFrom(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %s", cfg.Port)
	}
	if cfg.Issuer != "https://accounts.example.com" {
		t.Errorf("expected issuer https://accounts.example.com, got %s", cfg.Issuer)
	}
	if cfg.SigningKeys.Current == nil {
		t.Fatal("expected signing key to be set")
	}
}

func TestLoadFrom_CustomPort(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	src["PORT"] = "9090"

	cfg, err := LoadFrom(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %s", cfg.Port)
	}
}

func TestLoadFrom_PKCS8Key(t *testing.T) {
	pemKey := generatePKCS8Key(t)
	src := requiredEnv(pemKey)

	cfg, err := LoadFrom(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SigningKeys.Current == nil {
		t.Fatal("expected signing key to be set")
	}
}

func TestLoadFrom_MissingIssuer(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	delete(src, "ISSUER")

	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for missing ISSUER")
	}
}

func TestLoadFrom_MissingDatabaseURL(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	delete(src, "DATABASE_URL")

	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL")
	}
}

func TestLoadFrom_MissingCryptoKey(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	delete(src, "CRYPTO_KEY")

	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for missing CRYPTO_KEY")
	}
}

func TestLoadFrom_InvalidCryptoKeyHex(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	src["CRYPTO_KEY"] = "not-hex"

	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for invalid hex CRYPTO_KEY")
	}
}

func TestLoadFrom_CryptoKeyWrongLength(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	src["CRYPTO_KEY"] = hex.EncodeToString(make([]byte, 16))

	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for wrong-length CRYPTO_KEY")
	}
}

func TestLoadFrom_MissingSigningKey(t *testing.T) {
	src := requiredEnv("")
	delete(src, "SIGNING_KEY_PEM")

	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for missing SIGNING_KEY_PEM")
	}
}

func TestLoadFrom_InvalidSigningKey(t *testing.T) {
	src := requiredEnv("not-a-pem")

	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for invalid SIGNING_KEY_PEM")
	}
}

func TestLoadFrom_NoUpstreamIdP(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	delete(src, "GOOGLE_CLIENT_ID")

	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error when no upstream IdP is configured")
	}
}

func TestLoadFrom_GitHubOnly(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	delete(src, "GOOGLE_CLIENT_ID")
	delete(src, "GOOGLE_CLIENT_SECRET")
	src["GITHUB_CLIENT_ID"] = "ghid"
	src["GITHUB_CLIENT_SECRET"] = "ghsecret"

	cfg, err := LoadFrom(src)
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

func TestLoadFrom_TokenLifetimeDefaults(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)

	cfg, err := LoadFrom(src)
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

func TestLoadFrom_TokenLifetimeCustom(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	src["ACCESS_TOKEN_LIFETIME_MINUTES"] = "30"
	src["REFRESH_TOKEN_LIFETIME_DAYS"] = "14"

	cfg, err := LoadFrom(src)
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

func TestLoadFrom_TokenLifetimeZeroFallsBackToDefault(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	src["ACCESS_TOKEN_LIFETIME_MINUTES"] = "0"
	src["REFRESH_TOKEN_LIFETIME_DAYS"] = "0"

	cfg, err := LoadFrom(src)
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

func TestLoadFrom_TokenLifetimeOutOfRange(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	src["ACCESS_TOKEN_LIFETIME_MINUTES"] = "120"

	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for out-of-range ACCESS_TOKEN_LIFETIME_MINUTES")
	}
}

func TestLoadFrom_RefreshTokenLifetimeOutOfRange(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	src["REFRESH_TOKEN_LIFETIME_DAYS"] = "365"

	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for out-of-range REFRESH_TOKEN_LIFETIME_DAYS")
	}
}

func TestLoadFrom_AuthRequestTTLDefault(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)

	cfg, err := LoadFrom(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthRequestTTLMinutes != 30 {
		t.Errorf("expected default 30, got %d", cfg.AuthRequestTTLMinutes)
	}
}

func TestLoadFrom_AuthRequestTTLCustom(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	src["AUTH_REQUEST_TTL_MINUTES"] = "10"

	cfg, err := LoadFrom(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthRequestTTLMinutes != 10 {
		t.Errorf("expected 10, got %d", cfg.AuthRequestTTLMinutes)
	}
}

func TestLoadFrom_AuthRequestTTLZeroFallsBackToDefault(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	src["AUTH_REQUEST_TTL_MINUTES"] = "0"

	cfg, err := LoadFrom(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.AuthRequestTTLMinutes != 30 {
		t.Errorf("expected default 30 for zero, got %d", cfg.AuthRequestTTLMinutes)
	}
}

func TestLoadFrom_AuthRequestTTLOutOfRange(t *testing.T) {
	pemKey := generateTestKey(t)
	src := requiredEnv(pemKey)
	src["AUTH_REQUEST_TTL_MINUTES"] = "120"

	_, err := LoadFrom(src)
	if err == nil {
		t.Fatal("expected error for out-of-range AUTH_REQUEST_TTL_MINUTES")
	}
}
