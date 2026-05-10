// Package logger constructs the application's slog.Logger. JSON in production
// for log aggregators (Datadog, Loki, GCP Logging); text in development for
// human readability.
package logger

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// New returns a configured slog.Logger.
//
//   - When isProduction is true, output is JSON (stable schema for parsing).
//   - Otherwise output is text (human-friendly).
//
// level may be "debug", "info", "warn", or "error" (case-insensitive). Unknown
// levels default to info — never panic on a config typo.
func New(isProduction bool, level string) *slog.Logger {
	return NewWithWriter(os.Stdout, isProduction, level)
}

// NewWithWriter is like New but writes to w. Used in tests to capture output.
func NewWithWriter(w io.Writer, isProduction bool, level string) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(level)}

	var h slog.Handler
	switch {
	case isProduction:
		h = slog.NewJSONHandler(w, opts)
	default:
		h = slog.NewTextHandler(w, opts)
	}
	return slog.New(h)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
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
