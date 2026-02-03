package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AppConfig holds the foundational configuration required by all backend services.
// It is designed to be embedded into service-specific config structs.
type AppConfig struct {
	Env         string // "dev", "prod"
	ServiceName string // e.g. "accounts", "drive" (Crucial for logging/tracing)

	// Server Ports
	// Using separate ports for HTTP (Gateway) and gRPC is recommended for clarity and avoiding cmux complexity.
	HTTPPort int // Port for REST API (gRPC-Gateway) & Health checks. Default: 8080
	GRPCPort int // Port for gRPC Service (Internal communication). Default: 50051

	// CORS Configuration (Essential for SPA)
	AllowedOrigins []string // List of allowed origins for CORS

	// Logging
	LogLevel  string
	LogFormat string

	// Database Config
	DB DBConfig
}

type DBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string // "disable", "require", "verify-full"
}

// LoadBase loads the common configuration from environment variables.
// Services should embed AppConfig into their own Config struct and call this.
func LoadBase() AppConfig {
	return AppConfig{
		Env:         GetEnv("ENV", "dev"),
		ServiceName: GetEnv("SERVICE_NAME", "unknown-service"),

		// Default ports: 8080 for HTTP/Gateway, 50051 for gRPC
		HTTPPort: GetEnvAsInt("HTTP_PORT", 8080),
		GRPCPort: GetEnvAsInt("GRPC_PORT", 50051),

		// Default to allow all ("*") in dev, but should be explicit in prod
		AllowedOrigins: GetEnvAsSlice("CORS_ALLOWED_ORIGINS", []string{"*"}),

		LogLevel:  GetEnv("LOG_LEVEL", "info"),
		LogFormat: GetEnv("LOG_FORMAT", "json"),

		DB: DBConfig{
			Host:     GetEnvRequired("DB_HOST"),
			Port:     GetEnvAsInt("DB_PORT", 5432),
			User:     GetEnvRequired("DB_USER"),
			Password: GetEnvRequired("DB_PASSWORD"),
			Name:     GetEnvRequired("DB_NAME"),
			SSLMode:  GetEnv("DB_SSLMODE", "disable"),
		},
	}
}

// DSN returns the PostgreSQL Data Source Name.
func (c *DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.Name, c.SSLMode,
	)
}

// --- Helpers ---

func GetEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

// GetEnvRequired panics if the key is missing.
// This enforces the "Fail Fast" principle for critical infrastructure config.
func GetEnvRequired(key string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		panic(fmt.Sprintf("FATAL: Environment variable %s is required but not set.", key))
	}
	return value
}

func GetEnvAsInt(key string, defaultVal int) int {
	valueStr := GetEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultVal
}

// GetEnvAsSlice splits a comma-separated environment variable into a slice.
func GetEnvAsSlice(key string, defaultVal []string) []string {
	valueStr := GetEnv(key, "")
	if valueStr == "" {
		return defaultVal
	}
	// Split by comma and trim spaces
	parts := strings.Split(valueStr, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
