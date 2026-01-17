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

	JWTSecret       string // For signing Access Tokens
	RefreshTokenTTL int    // Days
}

func Load() *Config {
	base := platform.LoadBase() // Load common logic

	return &Config{
		AppConfig:           base,
		DiscordClientID:     platform.GetEnvRequired("DISCORD_CLIENT_ID"),
		DiscordClientSecret: platform.GetEnvRequired("DISCORD_CLIENT_SECRET"),
		DiscordRedirectURL:  platform.GetEnvRequired("DISCORD_REDIRECT_URL"),
		JWTSecret:           platform.GetEnvRequired("JWT_SECRET"),
		RefreshTokenTTL:     platform.GetEnvAsInt("REFRESH_TOKEN_TTL_DAYS", 30), // Default 30 days
	}
}
