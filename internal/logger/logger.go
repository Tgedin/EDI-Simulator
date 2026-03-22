package logger

import (
	"log/slog"
	"os"
	"strings"
)

// New returns a JSON slog logger with the service name pre-attached as a group
// attribute. It also sets the returned logger as the slog default so any
// stdlib log calls from imported packages use the same structured output.
func New(service, level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	l := slog.New(h).With("service", service)
	slog.SetDefault(l)
	return l
}
