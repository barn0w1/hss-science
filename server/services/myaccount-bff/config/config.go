package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type ConfigSource interface {
	Get(key string) string
}

type OSEnvSource struct{}

func (OSEnvSource) Get(key string) string { return os.Getenv(key) }

type MapSource map[string]string

func (m MapSource) Get(key string) string { return m[key] }

type Config struct {
	Port           string
	OIDCIssuer     string
	ClientID       string
	ClientSecret   string
	RedirectURL    string
	AccountsGRPC   string
	RedisURL       string
	SessionKey     [32]byte
	SessionIdleTTL time.Duration
	SessionHardTTL time.Duration
	CORSOrigins    []string
}

func Load() (*Config, error) {
	return LoadFrom(OSEnvSource{})
}

func LoadFrom(src ConfigSource) (*Config, error) {
	cfg := &Config{
		Port:         getFrom(src, "PORT", "8080"),
		OIDCIssuer:   src.Get("OIDC_ISSUER"),
		ClientID:     src.Get("CLIENT_ID"),
		ClientSecret: src.Get("CLIENT_SECRET"),
		RedirectURL:  src.Get("REDIRECT_URL"),
		AccountsGRPC: src.Get("ACCOUNTS_GRPC_ADDR"),
		RedisURL:     src.Get("REDIS_URL"),
	}

	if cfg.OIDCIssuer == "" {
		return nil, fmt.Errorf("OIDC_ISSUER is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("CLIENT_ID is required")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("CLIENT_SECRET is required")
	}
	if cfg.RedirectURL == "" {
		return nil, fmt.Errorf("REDIRECT_URL is required")
	}
	if cfg.AccountsGRPC == "" {
		return nil, fmt.Errorf("ACCOUNTS_GRPC_ADDR is required")
	}
	if cfg.RedisURL == "" {
		return nil, fmt.Errorf("REDIS_URL is required")
	}

	sessionKeyHex := src.Get("SESSION_KEY")
	if sessionKeyHex == "" {
		return nil, fmt.Errorf("SESSION_KEY is required")
	}
	keyBytes, err := hex.DecodeString(sessionKeyHex)
	if err != nil {
		return nil, fmt.Errorf("SESSION_KEY must be hex-encoded: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("SESSION_KEY must be exactly 32 bytes (64 hex chars), got %d bytes", len(keyBytes))
	}
	copy(cfg.SessionKey[:], keyBytes)

	idleMinutes, err := loadBoundedInt(src, "SESSION_IDLE_TTL_MINUTES", 120, 5, 1440)
	if err != nil {
		return nil, err
	}
	cfg.SessionIdleTTL = time.Duration(idleMinutes) * time.Minute

	hardDays, err := loadBoundedInt(src, "SESSION_HARD_TTL_DAYS", 7, 1, 90)
	if err != nil {
		return nil, err
	}
	cfg.SessionHardTTL = time.Duration(hardDays) * 24 * time.Hour

	corsRaw := src.Get("CORS_ALLOWED_ORIGINS")
	if corsRaw == "" {
		return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS is required")
	}
	for _, o := range strings.Split(corsRaw, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			cfg.CORSOrigins = append(cfg.CORSOrigins, trimmed)
		}
	}
	if len(cfg.CORSOrigins) == 0 {
		return nil, fmt.Errorf("CORS_ALLOWED_ORIGINS must have at least one entry")
	}

	return cfg, nil
}

func getFrom(src ConfigSource, key, fallback string) string {
	if v := src.Get(key); v != "" {
		return v
	}
	return fallback
}

func loadBoundedInt(src ConfigSource, key string, defaultVal, min, max int) (int, error) {
	raw := src.Get(key)
	val := defaultVal
	if raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return 0, fmt.Errorf("%s must be numeric: %w", key, err)
		}
		val = n
	}
	if val == 0 {
		val = defaultVal
	}
	if val < min || val > max {
		return 0, fmt.Errorf("%s must be 0 (default) or %d-%d, got %d", key, min, max, val)
	}
	return val, nil
}
