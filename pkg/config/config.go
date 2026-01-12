package config

import (
	"os"
	"strconv"
)

type AppConfig struct {
	Env       string // "dev" or "prod"
	Port      int
	LogLevel  string // "debug", "info", "warn", "error"
	LogFormat string // "json", "text"
}

// Load は環境変数から設定を読み込みます
func Load() *AppConfig {
	return &AppConfig{
		Env:       getEnv("ENV", "dev"), // デフォルトはdev
		Port:      getEnvAsInt("PORT", 8080),
		LogLevel:  getEnv("LOG_LEVEL", "info"),
		LogFormat: getEnv("LOG_FORMAT", "json"), // デフォルトはJSON
	}
}

// ヘルパー関数: 環境変数を取得、なければデフォルト値を返す
func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultVal
}
