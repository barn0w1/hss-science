package drive

import (
	"encoding/base64"
	"fmt"
	"os"
)

// Config holds the environment configuration for the Drive BFF.
type Config struct {
	Env                string
	Port               string
	LogLevel           string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	CookieHashKey      []byte
	CookieBlockKey     []byte
	CookieDomain       string
	PostLoginRedirect  string
	AccountsGRPCAddr   string
}

// ParseConfig reads required environment variables and returns a Config.
// It fails fast if any required variable is missing or invalid.
func ParseConfig() (*Config, error) {
	cfg := &Config{
		Env:                getEnvDefault("ENV", "development"),
		Port:               getEnvDefault("PORT", "8080"),
		LogLevel:           getEnvDefault("LOG_LEVEL", "info"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		CookieDomain:       os.Getenv("COOKIE_DOMAIN"),
		PostLoginRedirect:  os.Getenv("POST_LOGIN_REDIRECT"),
		AccountsGRPCAddr:   os.Getenv("ACCOUNTS_GRPC_ADDR"),
	}

	required := map[string]string{
		"GOOGLE_CLIENT_ID":     cfg.GoogleClientID,
		"GOOGLE_CLIENT_SECRET": cfg.GoogleClientSecret,
		"GOOGLE_REDIRECT_URL":  cfg.GoogleRedirectURL,
		"POST_LOGIN_REDIRECT":  cfg.PostLoginRedirect,
		"ACCOUNTS_GRPC_ADDR":   cfg.AccountsGRPCAddr,
	}
	for k, v := range required {
		if v == "" {
			return nil, fmt.Errorf("%s is required", k)
		}
	}

	var err error
	cfg.CookieHashKey, err = decodeBase64Env("COOKIE_HASH_KEY", 32)
	if err != nil {
		return nil, err
	}
	cfg.CookieBlockKey, err = decodeBase64Env("COOKIE_BLOCK_KEY", 32)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

// IsProduction returns true for production and staging environments.
func (c *Config) IsProduction() bool {
	return c.Env == "production" || c.Env == "staging"
}

func decodeBase64Env(key string, expectedLen int) ([]byte, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return nil, fmt.Errorf("%s is required", key)
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid base64: %w", key, err)
	}
	if len(decoded) != expectedLen {
		return nil, fmt.Errorf("%s: expected %d bytes, got %d", key, expectedLen, len(decoded))
	}
	return decoded, nil
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
