package config

import (
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

	// Cookie Settings (For SSO)
	SessionCookieName string // e.g. "accounts_session"
	CookieDomain      string // e.g. ".hss-science.org" or ""
	CookieSecure      bool   // true for prod, false for dev
	CookieSameSite    string // "Lax", "None", or "Strict"
}

func Load() *Config {
	base := platform.LoadBase() // Load common logic

	// Determine defaults based on environment
	isDev := base.Env == "dev"

	// Default: Production settings
	cookieSecure := true
	cookieSameSite := "Lax" // Subdomain sharing works with Lax

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

		// Cookie Config
		SessionCookieName: platform.GetEnv("SESSION_COOKIE_NAME", "accounts_session"),
		CookieDomain:      platform.GetEnv("COOKIE_DOMAIN", ""),
		CookieSecure:      cookieSecure,
		CookieSameSite:    cookieSameSite,
	}
}
