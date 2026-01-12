package config

import (
	"os"
	"strconv"
)

// AppConfig holds process-level configuration sourced from environment variables.
// It is intentionally flat to keep startup simple and predictable.
type AppConfig struct {
	Env       string // Runtime environment selector (e.g. "dev", "prod")
	Port      int
	LogLevel  string // Logging verbosity contract with the logger
	LogFormat string // Logger output format contract
}

// Load constructs AppConfig from environment variables.
// This is meant to be called once at startup; it does not support reloading.
func Load() *AppConfig {
	return &AppConfig{
		Env:       getEnv("ENV", "dev"),
		Port:      getEnvAsInt("PORT", 8080),
		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "json"),
	}
}

// getEnv returns the environment variable value if set, otherwise a default.
// This centralizes fallback behavior to keep config semantics consistent.
func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

// getEnvAsInt parses an environment variable as int with a safe default.
// Invalid or unset values intentionally degrade to default to avoid hard startup failures.
func getEnvAsInt(key string, defaultVal int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultVal
}
