package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// Config holds all configuration for the accounts-idp service.
// All values are loaded from environment variables following 12-Factor App principles.
type Config struct {
	Port               string
	Issuer             string
	DatabaseURL        string
	EncryptionKey      [32]byte
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string
	LogLevel           string
	DevMode            bool
}

// Load reads configuration from environment variables and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		Port:     envOrDefault("PORT", "8080"),
		LogLevel: envOrDefault("LOG_LEVEL", "info"),
		DevMode:  envOrDefault("DEV_MODE", "false") == "true",
	}

	var missing []string

	cfg.Issuer = os.Getenv("ISSUER")
	if cfg.Issuer == "" {
		missing = append(missing, "ISSUER")
	}

	cfg.DatabaseURL = os.Getenv("DATABASE_URL")
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}

	encKeyB64 := os.Getenv("ENCRYPTION_KEY")
	if encKeyB64 == "" {
		missing = append(missing, "ENCRYPTION_KEY")
	} else {
		decoded, err := base64.StdEncoding.DecodeString(encKeyB64)
		if err != nil {
			return nil, fmt.Errorf("ENCRYPTION_KEY: invalid base64: %w", err)
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("ENCRYPTION_KEY: expected 32 bytes, got %d", len(decoded))
		}
		copy(cfg.EncryptionKey[:], decoded)
	}

	cfg.GoogleClientID = os.Getenv("GOOGLE_CLIENT_ID")
	if cfg.GoogleClientID == "" {
		missing = append(missing, "GOOGLE_CLIENT_ID")
	}

	cfg.GoogleClientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	if cfg.GoogleClientSecret == "" {
		missing = append(missing, "GOOGLE_CLIENT_SECRET")
	}

	cfg.GoogleRedirectURI = os.Getenv("GOOGLE_REDIRECT_URI")
	if cfg.GoogleRedirectURI == "" {
		missing = append(missing, "GOOGLE_REDIRECT_URI")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
