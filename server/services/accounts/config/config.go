package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the Accounts service.
type Config struct {
	GRPCPort string

	DatabaseURL string

	DiscordClientID     string
	DiscordClientSecret string
	DiscordRedirectURL  string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		GRPCPort:            envOrDefault("GRPC_PORT", "50051"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		DiscordClientID:     os.Getenv("DISCORD_CLIENT_ID"),
		DiscordClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURL:  os.Getenv("DISCORD_REDIRECT_URL"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.DiscordClientID == "" {
		return nil, fmt.Errorf("DISCORD_CLIENT_ID is required")
	}
	if cfg.DiscordClientSecret == "" {
		return nil, fmt.Errorf("DISCORD_CLIENT_SECRET is required")
	}
	if cfg.DiscordRedirectURL == "" {
		return nil, fmt.Errorf("DISCORD_REDIRECT_URL is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
