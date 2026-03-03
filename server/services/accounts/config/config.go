package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

type ConfigSource interface {
	Get(key string) string
}

type OSEnvSource struct{}

func (OSEnvSource) Get(key string) string { return os.Getenv(key) }

type MapSource map[string]string

func (m MapSource) Get(key string) string { return m[key] }

type SigningKeySet struct {
	Current  *rsa.PrivateKey
	Previous []*rsa.PrivateKey
}

type Config struct {
	Port        string
	Issuer      string
	DatabaseURL string
	CryptoKey   [32]byte
	SigningKeys SigningKeySet

	AccessTokenLifetimeMinutes int
	RefreshTokenLifetimeDays   int
	AuthRequestTTLMinutes      int

	DBMaxOpenConns        int
	DBMaxIdleConns        int
	DBConnMaxLifetimeSecs int
	DBConnMaxIdleTimeSecs int

	RateLimitEnabled   bool
	RateLimitLoginRPM  int
	RateLimitTokenRPM  int
	RateLimitGlobalRPM int

	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string
}

func Load() (*Config, error) {
	return LoadFrom(OSEnvSource{})
}

func LoadFrom(src ConfigSource) (*Config, error) {
	cfg := &Config{
		Port:               getFrom(src, "PORT", "8080"),
		Issuer:             src.Get("ISSUER"),
		DatabaseURL:        src.Get("DATABASE_URL"),
		GoogleClientID:     src.Get("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: src.Get("GOOGLE_CLIENT_SECRET"),
		GitHubClientID:     src.Get("GITHUB_CLIENT_ID"),
		GitHubClientSecret: src.Get("GITHUB_CLIENT_SECRET"),
	}

	if cfg.Issuer == "" {
		return nil, fmt.Errorf("ISSUER is required")
	}
	if u, err := url.Parse(cfg.Issuer); err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("ISSUER must be a valid URL with scheme and host, got %q", cfg.Issuer)
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	cryptoKeyHex := src.Get("CRYPTO_KEY")
	if cryptoKeyHex == "" {
		return nil, fmt.Errorf("CRYPTO_KEY is required")
	}
	keyBytes, err := hex.DecodeString(cryptoKeyHex)
	if err != nil {
		return nil, fmt.Errorf("CRYPTO_KEY must be hex-encoded: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("CRYPTO_KEY must be exactly 32 bytes (64 hex chars), got %d bytes", len(keyBytes))
	}
	copy(cfg.CryptoKey[:], keyBytes)

	signingKeyPEM := src.Get("SIGNING_KEY_PEM")
	if signingKeyPEM == "" {
		return nil, fmt.Errorf("SIGNING_KEY_PEM is required")
	}
	cfg.SigningKeys.Current, err = parseRSAPrivateKey(signingKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("SIGNING_KEY_PEM: %w", err)
	}

	if prev := src.Get("SIGNING_KEY_PREVIOUS_PEM"); prev != "" {
		for _, pemStr := range strings.Split(prev, "---NEXT---") {
			trimmed := strings.TrimSpace(pemStr)
			if trimmed == "" {
				continue
			}
			key, err := parseRSAPrivateKey(trimmed)
			if err != nil {
				return nil, fmt.Errorf("SIGNING_KEY_PREVIOUS_PEM: %w", err)
			}
			cfg.SigningKeys.Previous = append(cfg.SigningKeys.Previous, key)
		}
	}

	if cfg.GoogleClientID == "" && cfg.GitHubClientID == "" {
		return nil, fmt.Errorf("at least one upstream IdP must be configured (GOOGLE_CLIENT_ID or GITHUB_CLIENT_ID)")
	}

	cfg.AccessTokenLifetimeMinutes, err = loadBoundedInt(src, "ACCESS_TOKEN_LIFETIME_MINUTES", 15, 1, 60)
	if err != nil {
		return nil, err
	}
	cfg.RefreshTokenLifetimeDays, err = loadBoundedInt(src, "REFRESH_TOKEN_LIFETIME_DAYS", 7, 1, 90)
	if err != nil {
		return nil, err
	}
	cfg.AuthRequestTTLMinutes, err = loadBoundedInt(src, "AUTH_REQUEST_TTL_MINUTES", 30, 1, 60)
	if err != nil {
		return nil, err
	}

	cfg.DBMaxOpenConns, err = loadBoundedInt(src, "DB_MAX_OPEN_CONNS", 25, 1, 500)
	if err != nil {
		return nil, err
	}
	cfg.DBMaxIdleConns, err = loadBoundedInt(src, "DB_MAX_IDLE_CONNS", 10, 1, 200)
	if err != nil {
		return nil, err
	}
	cfg.DBConnMaxLifetimeSecs, err = loadBoundedInt(src, "DB_CONN_MAX_LIFETIME_SECONDS", 300, 10, 3600)
	if err != nil {
		return nil, err
	}
	cfg.DBConnMaxIdleTimeSecs, err = loadBoundedInt(src, "DB_CONN_MAX_IDLE_TIME_SECONDS", 180, 10, 1800)
	if err != nil {
		return nil, err
	}

	cfg.RateLimitEnabled = src.Get("RATE_LIMIT_ENABLED") != "false"
	cfg.RateLimitLoginRPM, err = loadBoundedInt(src, "RATE_LIMIT_LOGIN_RPM", 20, 1, 600)
	if err != nil {
		return nil, err
	}
	cfg.RateLimitTokenRPM, err = loadBoundedInt(src, "RATE_LIMIT_TOKEN_RPM", 60, 1, 6000)
	if err != nil {
		return nil, err
	}
	cfg.RateLimitGlobalRPM, err = loadBoundedInt(src, "RATE_LIMIT_GLOBAL_RPM", 120, 1, 6000)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	var rsaKey *rsa.PrivateKey
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		rsaKey = key
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		var ok bool
		rsaKey, ok = key.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("PKCS#8 key is not RSA")
		}
	default:
		return nil, fmt.Errorf("unsupported PEM block type: %s", block.Type)
	}

	if rsaKey.N.BitLen() < 2048 {
		return nil, fmt.Errorf("RSA signing key must be >= 2048 bits, got %d", rsaKey.N.BitLen())
	}

	return rsaKey, nil
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
