package observability

import (
	"log/slog"
	"os"
	"strings"

	"novpn/internal/config"
)

func NewLogger(cfg config.ObservabilityConfig) *slog.Logger {
	level := parseLevel(cfg.LogLevel)
	options := &slog.HandlerOptions{Level: level}

	if cfg.JSONLogs {
		return slog.New(slog.NewJSONHandler(os.Stdout, options))
	}

	return slog.New(slog.NewTextHandler(os.Stdout, options))
}

func parseLevel(raw string) slog.Level {
	switch strings.ToLower(raw) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
