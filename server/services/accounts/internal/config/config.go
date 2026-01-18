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
	JWTSecret       string // For signing Access Tokens
	RefreshTokenTTL int    // Days

	// Cookie Settings (For SSO)
	CookieDomain   string // e.g. ".hss-science.org" or ""
	CookieSecure   bool   // true for prod, false for dev
	CookieSameSite string // "Lax", "None", or "Strict"
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
		AppConfig:           base,
		DiscordClientID:     platform.GetEnvRequired("DISCORD_CLIENT_ID"),
		DiscordClientSecret: platform.GetEnvRequired("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURL:  platform.GetEnvRequired("DISCORD_REDIRECT_URL"),
		JWTSecret:           platform.GetEnvRequired("JWT_SECRET"),
		RefreshTokenTTL:     platform.GetEnvAsInt("REFRESH_TOKEN_TTL_DAYS", 30),

		// Cookie Config
		CookieDomain:   platform.GetEnv("COOKIE_DOMAIN", ""),
		CookieSecure:   cookieSecure,
		CookieSameSite: cookieSameSite,
	}
}
