package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

type Config struct {
	// Level is one of "debug", "info", "warn", "error" (case-insensitive).
	// Defaults to "info" if empty or unrecognized.
	Level string

	// Format is "text" or "json". Defaults to "text" if empty or
	// unrecognized. homelabctld's systemd unit should set this to "json".
	Format string

	// AddSource includes the source file:line of each log call. Useful in
	// debug, noisy/expensive-looking in production - defaults to true only
	// when Level is "debug".
	AddSource bool

	// Output overrides the destination writer. Defaults to os.Stderr.
	// Exposed mainly for tests.
	Output *os.File
}

func Setup(cfg Config) *slog.Logger {
	level := parseLevel(cfg.Level)

	out := cfg.Output
	if out == nil {
		out = os.Stderr
	}

	addSource := cfg.AddSource || level == slog.LevelDebug

	handlerOpts := slog.HandlerOptions{
		Level:     level,
		AddSource: addSource,
	}

	var handler slog.Handler
	switch parseFormat(cfg.Format) {
	case FormatJSON:
		handler = slog.NewJSONHandler(out, &handlerOpts)
	default:
		handler = slog.NewTextHandler(out, &handlerOpts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}

// parseLevel converts a case-insensitive level string to a slog.Level,
// falling back to Info for empty/unrecognized input rather than erroring -
// a bad --log-level flag shouldn't prevent the daemon from starting.
func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "", "info":
		return slog.LevelInfo
	default:
		// Unrecognized level: warn on stderr directly (logger isn't built
		// yet) and fall back to Info.
		fmt.Fprintf(os.Stderr, "logging: unrecognized level %q, defaulting to info\n", s)
		return slog.LevelInfo
	}
}

func parseFormat(s string) Format {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "json":
		return FormatJSON
	default:
		return FormatText
	}
}

func Fatal(msg string, args ...any) {
	slog.Error(msg, args...)
	os.Exit(1)
}
