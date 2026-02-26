package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds all configuration for the myaccount-bff service.
type Config struct {
	Port     string
	LogLevel string
	DevMode  bool

	// OIDC RP configuration.
	OIDCIssuer       string
	OIDCClientID     string
	OIDCClientSecret string
	OIDCRedirectURI  string

	// Session configuration.
	RedisURL      string
	SessionSecret [32]byte
	SessionMaxAge time.Duration

	// gRPC backend.
	AccountsGRPCAddr string

	// SPA origin for CORS.
	SPAOrigin string
}

// Load reads configuration from environment variables and validates required fields.
func Load() (*Config, error) {
	cfg := &Config{
		Port:     envOrDefault("PORT", "8081"),
		LogLevel: envOrDefault("LOG_LEVEL", "info"),
		DevMode:  envOrDefault("DEV_MODE", "false") == "true",
	}

	var missing []string

	cfg.OIDCIssuer = os.Getenv("OIDC_ISSUER")
	if cfg.OIDCIssuer == "" {
		missing = append(missing, "OIDC_ISSUER")
	}

	cfg.OIDCClientID = os.Getenv("OIDC_CLIENT_ID")
	if cfg.OIDCClientID == "" {
		missing = append(missing, "OIDC_CLIENT_ID")
	}

	cfg.OIDCClientSecret = os.Getenv("OIDC_CLIENT_SECRET")
	if cfg.OIDCClientSecret == "" {
		missing = append(missing, "OIDC_CLIENT_SECRET")
	}

	cfg.OIDCRedirectURI = os.Getenv("OIDC_REDIRECT_URI")
	if cfg.OIDCRedirectURI == "" {
		missing = append(missing, "OIDC_REDIRECT_URI")
	}

	cfg.RedisURL = os.Getenv("REDIS_URL")
	if cfg.RedisURL == "" {
		missing = append(missing, "REDIS_URL")
	}

	sessionSecretB64 := os.Getenv("SESSION_SECRET")
	if sessionSecretB64 == "" {
		missing = append(missing, "SESSION_SECRET")
	} else {
		decoded, err := base64.StdEncoding.DecodeString(sessionSecretB64)
		if err != nil {
			return nil, fmt.Errorf("SESSION_SECRET: invalid base64: %w", err)
		}
		if len(decoded) != 32 {
			return nil, fmt.Errorf("SESSION_SECRET: expected 32 bytes, got %d", len(decoded))
		}
		copy(cfg.SessionSecret[:], decoded)
	}

	cfg.AccountsGRPCAddr = os.Getenv("ACCOUNTS_GRPC_ADDR")
	if cfg.AccountsGRPCAddr == "" {
		missing = append(missing, "ACCOUNTS_GRPC_ADDR")
	}

	cfg.SPAOrigin = os.Getenv("SPA_ORIGIN")
	if cfg.SPAOrigin == "" {
		missing = append(missing, "SPA_ORIGIN")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %s", strings.Join(missing, ", "))
	}

	cfg.SessionMaxAge = envOrDefaultDuration("SESSION_MAX_AGE", 24*time.Hour)

	return cfg, nil
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func envOrDefaultDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}
