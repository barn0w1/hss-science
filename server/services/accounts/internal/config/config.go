package config

import (
	"strings"

	platform "github.com/barn0w1/hss-science/server/platform/config"
)

type Config struct {
	platform.AppConfig // Embed common config

	// Accounts Service Specific
	DiscordClientID     string
	DiscordClientSecret string
	DiscordRedirectURL  string

	// Security
	StateSecret          string // HMAC secret for OAuth state
	SessionTTLHours      int    // Accounts session TTL (hours)
	AuthCodeTTLSeconds   int    // Auth code TTL (seconds)
	OAuthStateTTLSeconds int    // OAuth state TTL (seconds)
	OAuthHTTPTimeoutSec  int    // Timeout for OAuth HTTP requests

	// Cookie Settings (For SSO)
	SessionCookieName string // e.g. "accounts_session"
	CookieDomain      string // e.g. ".hss-science.org" or ""
	CookieSecure      bool   // true for prod, false for dev
	CookieSameSite    string // "Lax", "None", or "Strict"

	// HTTP Server
	HTTPReadTimeoutSec       int
	HTTPWriteTimeoutSec      int
	HTTPIdleTimeoutSec       int
	HTTPReadHeaderTimeoutSec int
	HTTPShutdownTimeoutSec   int

	// Database Pooling
	DBConnectTimeoutSec  int
	DBMaxOpenConns       int
	DBMaxIdleConns       int
	DBConnMaxLifetimeMin int
	DBConnMaxIdleTimeMin int
}

func Load() *Config {
	base := platform.LoadBase() // Load common logic

	// Determine defaults based on environment
	isDev := base.Env == "dev"

	// Default: Production settings
	cookieSecure := !isDev
	cookieSameSite := "Lax" // Subdomain sharing works with Lax
	cookieNameDefault := "accounts_session"
	if !isDev {
		cookieNameDefault = "__Secure-accounts_session"
	}

	if isDev {
		// Dev settings (localhost)
		cookieSecure = false
		cookieSameSite = "Lax"
	}

	return &Config{
		AppConfig:            base,
		DiscordClientID:      platform.GetEnvRequired("DISCORD_CLIENT_ID"),
		DiscordClientSecret:  platform.GetEnvRequired("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURL:   platform.GetEnvRequired("DISCORD_REDIRECT_URL"),
		StateSecret:          platform.GetEnvRequired("STATE_SECRET"),
		SessionTTLHours:      platform.GetEnvAsInt("SESSION_TTL_HOURS", 168),
		AuthCodeTTLSeconds:   platform.GetEnvAsInt("AUTH_CODE_TTL_SECONDS", 60),
		OAuthStateTTLSeconds: platform.GetEnvAsInt("OAUTH_STATE_TTL_SECONDS", 300),
		OAuthHTTPTimeoutSec:  platform.GetEnvAsInt("OAUTH_HTTP_TIMEOUT_SECONDS", 10),

		// Cookie Config
		SessionCookieName: platform.GetEnv("SESSION_COOKIE_NAME", cookieNameDefault),
		CookieDomain:      platform.GetEnv("COOKIE_DOMAIN", ""),
		CookieSecure:      getEnvAsBool("COOKIE_SECURE", cookieSecure),
		CookieSameSite:    platform.GetEnv("COOKIE_SAMESITE", cookieSameSite),

		// HTTP Server
		HTTPReadTimeoutSec:       platform.GetEnvAsInt("HTTP_READ_TIMEOUT_SECONDS", 10),
		HTTPWriteTimeoutSec:      platform.GetEnvAsInt("HTTP_WRITE_TIMEOUT_SECONDS", 10),
		HTTPIdleTimeoutSec:       platform.GetEnvAsInt("HTTP_IDLE_TIMEOUT_SECONDS", 120),
		HTTPReadHeaderTimeoutSec: platform.GetEnvAsInt("HTTP_READ_HEADER_TIMEOUT_SECONDS", 5),
		HTTPShutdownTimeoutSec:   platform.GetEnvAsInt("HTTP_SHUTDOWN_TIMEOUT_SECONDS", 5),

		// Database Pooling
		DBConnectTimeoutSec:  platform.GetEnvAsInt("DB_CONNECT_TIMEOUT_SECONDS", 5),
		DBMaxOpenConns:       platform.GetEnvAsInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:       platform.GetEnvAsInt("DB_MAX_IDLE_CONNS", 25),
		DBConnMaxLifetimeMin: platform.GetEnvAsInt("DB_CONN_MAX_LIFETIME_MINUTES", 5),
		DBConnMaxIdleTimeMin: platform.GetEnvAsInt("DB_CONN_MAX_IDLE_TIME_MINUTES", 5),
	}
}

func getEnvAsBool(key string, defaultVal bool) bool {
	val := strings.TrimSpace(strings.ToLower(platform.GetEnv(key, "")))
	if val == "" {
		return defaultVal
	}
	switch val {
	case "1", "true", "t", "yes", "y":
		return true
	case "0", "false", "f", "no", "n":
		return false
	default:
		return defaultVal
	}
}
