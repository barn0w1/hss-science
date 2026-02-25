package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
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

	// Database connection pool settings.
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	// Token lifetime settings.
	AccessTokenLifetime  time.Duration
	RefreshTokenLifetime time.Duration
	IDTokenLifetime      time.Duration
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

	// Database pool configuration (optional, with defaults).
	var err error
	if cfg.DBMaxOpenConns, err = envOrDefaultInt("DB_MAX_OPEN_CONNS", 25); err != nil {
		return nil, err
	}
	if cfg.DBMaxIdleConns, err = envOrDefaultInt("DB_MAX_IDLE_CONNS", 5); err != nil {
		return nil, err
	}
	if cfg.DBConnMaxLifetime, err = envOrDefaultDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute); err != nil {
		return nil, err
	}

	// Token lifetime configuration (optional, with defaults).
	if cfg.AccessTokenLifetime, err = envOrDefaultDuration("ACCESS_TOKEN_LIFETIME", 5*time.Minute); err != nil {
		return nil, err
	}
	if cfg.RefreshTokenLifetime, err = envOrDefaultDuration("REFRESH_TOKEN_LIFETIME", 5*time.Hour); err != nil {
		return nil, err
	}
	if cfg.IDTokenLifetime, err = envOrDefaultDuration("ID_TOKEN_LIFETIME", 1*time.Hour); err != nil {
		return nil, err
	}

	return cfg, nil
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envOrDefaultInt(key string, defaultVal int) (int, error) {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("%s: invalid integer value %q: %w", key, v, err)
		}
		return n, nil
	}
	return defaultVal, nil
}

func envOrDefaultDuration(key string, defaultVal time.Duration) (time.Duration, error) {
	if v := os.Getenv(key); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0, fmt.Errorf("%s: invalid duration value %q: %w", key, v, err)
		}
		return d, nil
	}
	return defaultVal, nil
}
