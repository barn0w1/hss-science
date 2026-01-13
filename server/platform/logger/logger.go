package logger

import (
	"log/slog"
	"os"
	"strings"
)

// Config holds the configuration for the logger.
type Config struct {
	LogLevel  string // "debug", "info", "warn", "error"
	LogFormat string // "json", "text"
}

// Setup initializes the global logger based on the provided config.
func Setup(cfg Config) {
	var handler slog.Handler

	// Parse level string to slog.Level
	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Select handler based on format
	switch strings.ToLower(cfg.LogFormat) {
	case "text":
		handler = slog.NewTextHandler(os.Stdout, opts)
	default:
		// Default to JSON for production/cloud environments
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}
