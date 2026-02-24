package accounts

import (
	"fmt"
	"os"
	"time"
)

// Config holds the environment configuration for the Accounts gRPC service.
type Config struct {
	Env               string
	Port              string
	LogLevel          string
	DatabaseURL       string
	JWTPrivateKeyPath string
	JWTIssuer         string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
}

// ParseConfig reads required environment variables and returns a Config.
// It fails fast if any required variable is missing or invalid.
func ParseConfig() (*Config, error) {
	cfg := &Config{
		Env:               getEnvDefault("ENV", "development"),
		Port:              getEnvDefault("PORT", "50051"),
		LogLevel:          getEnvDefault("LOG_LEVEL", "info"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		JWTPrivateKeyPath: os.Getenv("JWT_PRIVATE_KEY_PATH"),
		JWTIssuer:         os.Getenv("JWT_ISSUER"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTPrivateKeyPath == "" {
		return nil, fmt.Errorf("JWT_PRIVATE_KEY_PATH is required")
	}
	if cfg.JWTIssuer == "" {
		return nil, fmt.Errorf("JWT_ISSUER is required")
	}

	var err error
	cfg.AccessTokenTTL, err = time.ParseDuration(getEnvDefault("ACCESS_TOKEN_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid ACCESS_TOKEN_TTL: %w", err)
	}
	cfg.RefreshTokenTTL, err = time.ParseDuration(getEnvDefault("REFRESH_TOKEN_TTL", "168h"))
	if err != nil {
		return nil, fmt.Errorf("invalid REFRESH_TOKEN_TTL: %w", err)
	}

	return cfg, nil
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
