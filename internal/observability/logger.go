// Package observability provides structured logging and metrics for दूतसभा.
// All logs go to stderr — stdout is reserved for data output.
package observability

import (
	"io"
	"log/slog"
	"os"
)

// NewLogger creates a slog.Logger that writes to stderr.
//
// format: "json" for machine-parseable, "text" for human-readable.
// level: controls minimum verbosity.
// w: output writer (typically os.Stderr).
func NewLogger(level slog.Level, format string, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Enable source location at Debug level with explicit request (level < Debug means -vvv).
	if level < slog.LevelDebug {
		opts.Level = slog.LevelDebug
		opts.AddSource = true
	}

	var handler slog.Handler
	if format == "json" {
		handler = slog.NewJSONHandler(w, opts)
	} else {
		handler = slog.NewTextHandler(w, opts)
	}

	return slog.New(handler)
}

// VerbosityLevel maps -v flag count to slog.Level.
//
//	0 = Warn (default)
//	1 = Info (-v)
//	2 = Debug (-vv)
//	3+ = Debug with source (-vvv)
func VerbosityLevel(count int) slog.Level {
	switch {
	case count >= 3:
		return slog.LevelDebug - 1 // triggers AddSource
	case count == 2:
		return slog.LevelDebug
	case count == 1:
		return slog.LevelInfo
	default:
		return slog.LevelWarn
	}
}

// SetupDefaultLogger configures the global slog default logger for the session.
//
// When jsonMode is true and no explicit verbosity (-v) is set, the log level
// is raised to Error to suppress WARN-level log lines on stderr. This ensures
// --json mode produces clean stdout JSON with no stderr noise (GitHub issue #4).
// Users can opt back in with --json -v.
func SetupDefaultLogger(verbosity int, jsonMode bool) *slog.Logger {
	level := VerbosityLevel(verbosity)
	// In JSON mode without explicit verbosity, suppress WARN to keep stderr clean.
	if jsonMode && verbosity == 0 {
		level = slog.LevelError
	}
	format := "text"
	if jsonMode {
		format = "json"
	}
	logger := NewLogger(level, format, os.Stderr)
	slog.SetDefault(logger)
	return logger
}
