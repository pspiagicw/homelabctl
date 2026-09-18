package logging

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
)

type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Color codes (8-bit ANSI, see tint.Attr docs)
const (
	colorComponent = 4 // cyan
	colorAddress   = 5
	colorStatus    = 1
	colorNode      = 3 // yellow
	colorErr       = 9 // bright red
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

func Setup(cfg Config) {
	level := parseLevel(cfg.Level)

	out := cfg.Output
	if out == nil {
		out = os.Stderr
	}

	addSource := level == slog.LevelDebug

	switch parseFormat(cfg.Format) {
	case FormatJSON:
		handler := slog.NewJSONHandler(out, &slog.HandlerOptions{
			Level:     level,
			AddSource: addSource,
		})
		logger := slog.New(handler)
		slog.SetDefault(logger)
	default:
		handler := tint.NewTextHandler(out, &tint.Options{
			Level:       level,
			AddSource:   addSource,
			TimeFormat:  time.Kitchen,
			ReplaceAttr: replaceAttr,
		})
		logger := slog.New(handler)
		slog.SetDefault(logger)
	}
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

// replaceAttr applies global, key-based coloring rules so call sites
// don't need to wrap every attr in tint.Attr() manually.
func replaceAttr(groups []string, a slog.Attr) slog.Attr {
	// Any attr whose value is an error → always red, regardless of key.
	if a.Value.Kind() == slog.KindAny {
		if _, ok := a.Value.Any().(error); ok {
			return tint.Attr(colorErr, a)
		}
	}

	if a.Value.Kind() == slog.KindBool {
		if a.Value.Bool() {
			return tint.Attr(2, a) // green
		}
		return tint.Attr(1, a) // red
	}

	if a.Value.Kind() == slog.KindString {
		switch a.Value.String() {
		case "on", "On", "Normal":
			return tint.Attr(2, a) // green
		case "off", "Off", "OutageDetected", "OutageConfirmed":
			return tint.Attr(1, a) // red
		case "booting", "Booting", "Restoring", "Restored":
			return tint.Attr(3, a) // yellow
		case "unreachable", "Unreachable":
			return tint.Attr(9, a) // bright red
		}
	}

	// Key-based rules.
	switch a.Key {
	case "component":
		return tint.Attr(colorComponent, a)
	case "node":
		return tint.Attr(colorNode, a)
	case "address":
		return tint.Attr(colorAddress, a)
	case "status":
		return tint.Attr(colorStatus, a)
	}

	return a
}

// WithComponent returns a logger scoped to a named subsystem
// (e.g. "sentinel", "scheduler", "override", "api").
// Every log line through it carries component=<name>, colorized per replaceAttr.
func WithComponent(name string) *slog.Logger {
	return slog.Default().With("component", name)
}
