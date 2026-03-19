package config

import (
	"fmt"
	"os"
	"strconv"
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
	GRPCListenAddr          string
	DatabaseURL             string
	OIDCIssuerURL           string
	R2Endpoint              string
	R2Bucket                string
	R2AccessKeyID           string
	R2SecretAccessKey       string
	PresignPutTTL           time.Duration
	PresignGetMaxTTL        time.Duration
	MultipartThresholdBytes int64

	DBMaxOpenConns        int
	DBMaxIdleConns        int
	DBConnMaxLifetimeSecs int
	DBConnMaxIdleTimeSecs int
}

func Load() (*Config, error) {
	return LoadFrom(OSEnvSource{})
}

func LoadFrom(src ConfigSource) (*Config, error) {
	cfg := &Config{
		GRPCListenAddr:    getFrom(src, "GRPC_LISTEN_ADDR", ":50052"),
		DatabaseURL:       src.Get("DATABASE_URL"),
		OIDCIssuerURL:     src.Get("OIDC_ISSUER_URL"),
		R2Endpoint:        src.Get("R2_ENDPOINT"),
		R2Bucket:          src.Get("R2_BUCKET"),
		R2AccessKeyID:     src.Get("R2_ACCESS_KEY_ID"),
		R2SecretAccessKey: src.Get("R2_SECRET_ACCESS_KEY"),
	}

	required := map[string]string{
		"DATABASE_URL":         cfg.DatabaseURL,
		"OIDC_ISSUER_URL":      cfg.OIDCIssuerURL,
		"R2_ENDPOINT":          cfg.R2Endpoint,
		"R2_BUCKET":            cfg.R2Bucket,
		"R2_ACCESS_KEY_ID":     cfg.R2AccessKeyID,
		"R2_SECRET_ACCESS_KEY": cfg.R2SecretAccessKey,
	}
	var missing []string
	for k, v := range required {
		if v == "" {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required environment variables: %v", missing)
	}

	var err error
	putTTL, err := loadInt(src, "PRESIGN_PUT_TTL_SECONDS", 900)
	if err != nil {
		return nil, err
	}
	cfg.PresignPutTTL = time.Duration(putTTL) * time.Second

	getTTL, err := loadInt(src, "PRESIGN_GET_TTL_MAX_SECONDS", 3600)
	if err != nil {
		return nil, err
	}
	cfg.PresignGetMaxTTL = time.Duration(getTTL) * time.Second

	threshold, err := loadInt64(src, "MULTIPART_THRESHOLD_BYTES", 10*1024*1024)
	if err != nil {
		return nil, err
	}
	cfg.MultipartThresholdBytes = threshold

	cfg.DBMaxOpenConns, err = loadInt(src, "DB_MAX_OPEN_CONNS", 25)
	if err != nil {
		return nil, err
	}
	cfg.DBMaxIdleConns, err = loadInt(src, "DB_MAX_IDLE_CONNS", 5)
	if err != nil {
		return nil, err
	}
	cfg.DBConnMaxLifetimeSecs, err = loadInt(src, "DB_CONN_MAX_LIFETIME_SECONDS", 300)
	if err != nil {
		return nil, err
	}
	cfg.DBConnMaxIdleTimeSecs, err = loadInt(src, "DB_CONN_MAX_IDLE_TIME_SECONDS", 180)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func getFrom(src ConfigSource, key, fallback string) string {
	if v := src.Get(key); v != "" {
		return v
	}
	return fallback
}

func loadInt(src ConfigSource, key string, defaultVal int) (int, error) {
	raw := src.Get(key)
	if raw == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be numeric: %w", key, err)
	}
	return n, nil
}

func loadInt64(src ConfigSource, key string, defaultVal int64) (int64, error) {
	raw := src.Get(key)
	if raw == "" {
		return defaultVal, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be numeric: %w", key, err)
	}
	return n, nil
}
