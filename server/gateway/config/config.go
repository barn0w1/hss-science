package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the Gateway (BFF).
type Config struct {
	HTTPPort         string
	AccountsGRPCAddr string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPPort:         envOrDefault("HTTP_PORT", "8080"),
		AccountsGRPCAddr: os.Getenv("ACCOUNTS_GRPC_ADDR"),
	}

	if cfg.AccountsGRPCAddr == "" {
		return nil, fmt.Errorf("ACCOUNTS_GRPC_ADDR is required")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
