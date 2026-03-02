package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port        string
	Issuer      string
	DatabaseURL string
	CryptoKey   [32]byte
	SigningKey  *rsa.PrivateKey

	AccessTokenLifetimeMinutes int
	RefreshTokenLifetimeDays   int
	AuthRequestTTLMinutes      int

	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		Issuer:             os.Getenv("ISSUER"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
	}

	if cfg.Issuer == "" {
		return nil, fmt.Errorf("ISSUER is required")
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	cryptoKeyHex := os.Getenv("CRYPTO_KEY")
	if cryptoKeyHex == "" {
		return nil, fmt.Errorf("CRYPTO_KEY is required")
	}
	keyBytes, err := hex.DecodeString(cryptoKeyHex)
	if err != nil {
		return nil, fmt.Errorf("CRYPTO_KEY must be hex-encoded: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("CRYPTO_KEY must be exactly 32 bytes (64 hex chars), got %d bytes", len(keyBytes))
	}
	copy(cfg.CryptoKey[:], keyBytes)

	signingKeyPEM := os.Getenv("SIGNING_KEY_PEM")
	if signingKeyPEM == "" {
		return nil, fmt.Errorf("SIGNING_KEY_PEM is required")
	}
	cfg.SigningKey, err = parseRSAPrivateKey(signingKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("SIGNING_KEY_PEM: %w", err)
	}

	if cfg.GoogleClientID == "" && cfg.GitHubClientID == "" {
		return nil, fmt.Errorf("at least one upstream IdP must be configured (GOOGLE_CLIENT_ID or GITHUB_CLIENT_ID)")
	}

	cfg.AccessTokenLifetimeMinutes = getEnvInt("ACCESS_TOKEN_LIFETIME_MINUTES", 15)
	if cfg.AccessTokenLifetimeMinutes == 0 {
		cfg.AccessTokenLifetimeMinutes = 15
	}
	if cfg.AccessTokenLifetimeMinutes < 1 || cfg.AccessTokenLifetimeMinutes > 60 {
		return nil, fmt.Errorf("ACCESS_TOKEN_LIFETIME_MINUTES must be 0 (default) or 1-60, got %d", cfg.AccessTokenLifetimeMinutes)
	}

	cfg.RefreshTokenLifetimeDays = getEnvInt("REFRESH_TOKEN_LIFETIME_DAYS", 7)
	if cfg.RefreshTokenLifetimeDays == 0 {
		cfg.RefreshTokenLifetimeDays = 7
	}
	if cfg.RefreshTokenLifetimeDays < 1 || cfg.RefreshTokenLifetimeDays > 90 {
		return nil, fmt.Errorf("REFRESH_TOKEN_LIFETIME_DAYS must be 0 (default) or 1-90, got %d", cfg.RefreshTokenLifetimeDays)
	}

	cfg.AuthRequestTTLMinutes = getEnvInt("AUTH_REQUEST_TTL_MINUTES", 30)
	if cfg.AuthRequestTTLMinutes == 0 {
		cfg.AuthRequestTTLMinutes = 30
	}
	if cfg.AuthRequestTTLMinutes < 1 || cfg.AuthRequestTTLMinutes > 60 {
		return nil, fmt.Errorf("AUTH_REQUEST_TTL_MINUTES must be 0 (default) or 1-60, got %d", cfg.AuthRequestTTLMinutes)
	}

	return cfg, nil
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key is not RSA")
		}
		return rsaKey, nil
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, defaultVal int) int {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}
