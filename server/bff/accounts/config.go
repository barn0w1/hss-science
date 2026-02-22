// Package accounts implements the HTTP BFF handler and configuration for the accounts service.
package accounts

import (
	"fmt"
	"os"
	"strings"
)

// Config holds the BFF configuration.
type Config struct {
	// Env is the runtime environment ("development" or "production").
	Env string

	// Port is the HTTP port number (without colon prefix).
	Port string

	// LogLevel is the structured log level (DEBUG, INFO, WARN, ERROR).
	LogLevel string

	// GRPCAddr is the address of the Accounts gRPC service.
	GRPCAddr string

	// Provider is the OAuth provider name (e.g., "discord").
	Provider string

	// SessionHashKey is the 32-byte key for HMAC signing.
	SessionHashKey []byte

	// SessionBlockKey is the 16-byte key for AES encryption.
	SessionBlockKey []byte

	// SessionMaxAge is the session duration in seconds.
	SessionMaxAge int

	// SessionSecure controls the Secure flag on cookies.
	SessionSecure bool

	// Audiences maps audience identifiers to their allowed redirect URI prefixes.
	Audiences map[string][]string
}

// LoadConfig loads configuration from environment variables.
func LoadConfig() *Config {
	audiences := parseAudiences(os.Getenv("ALLOWED_AUDIENCES"))

	return &Config{
		Env:             envOrDefault("ENV", "production"),
		Port:            envOrDefault("PORT", "8080"),
		LogLevel:        envOrDefault("LOG_LEVEL", "INFO"),
		GRPCAddr:        envOrDefault("ACCOUNTS_GRPC_ADDR", "localhost:50051"),
		Provider:        envOrDefault("OAUTH_PROVIDER", "discord"),
		SessionHashKey:  []byte(requiredEnv("SESSION_HASH_KEY")),
		SessionBlockKey: []byte(requiredEnv("SESSION_BLOCK_KEY")),
		SessionMaxAge:   2592000, // 30 days
		SessionSecure:   os.Getenv("SESSION_SECURE") != "false",
		Audiences:       audiences,
	}
}

// ValidateAudience checks if the audience is known.
func (c *Config) ValidateAudience(audience string) bool {
	_, ok := c.Audiences[audience]
	return ok
}

// ValidateRedirectURI checks if the redirect_uri is allowed for the given audience.
func (c *Config) ValidateRedirectURI(audience, redirectURI string) bool {
	prefixes, ok := c.Audiences[audience]
	if !ok {
		return false
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(redirectURI, prefix) {
			return true
		}
	}
	return false
}

// parseAudiences parses the ALLOWED_AUDIENCES env var.
// Format: "audience1=https://prefix1,https://prefix2;audience2=https://prefix3"
func parseAudiences(raw string) map[string][]string {
	result := make(map[string][]string)
	if raw == "" {
		return result
	}
	for _, entry := range strings.Split(raw, ";") {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) != 2 {
			continue
		}
		audience := strings.TrimSpace(parts[0])
		prefixes := strings.Split(parts[1], ",")
		for i := range prefixes {
			prefixes[i] = strings.TrimSpace(prefixes[i])
		}
		result[audience] = prefixes
	}
	return result
}

func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func requiredEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		fmt.Fprintf(os.Stderr, "required environment variable %s is not set\n", key)
		os.Exit(1)
	}
	return v
}
