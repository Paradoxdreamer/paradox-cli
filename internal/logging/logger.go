package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
)

// Level represents a logging level.
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Format is the output format of the logger.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Options configures the logger.
type Options struct {
	Level  Level
	Format Format
	Output io.Writer // defaults to os.Stderr
}

var (
	mu     sync.RWMutex
	global *slog.Logger
)

func init() {
	// Sensible default until Configure is called
	global = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: LevelInfo,
	}))
}

// Configure sets up the global logger.
// Safe to call multiple times (e.g. from PersistentPreRun).
func Configure(opts Options) {
	if opts.Output == nil {
		opts.Output = os.Stderr
	}
	if opts.Format == "" {
		opts.Format = FormatText
	}

	var handler slog.Handler
	handlerOpts := &slog.HandlerOptions{
		Level: opts.Level,
		// Add source for debug builds later if desired
	}

	switch opts.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(opts.Output, handlerOpts)
	default:
		handler = slog.NewTextHandler(opts.Output, handlerOpts)
	}

	mu.Lock()
	global = slog.New(handler)
	mu.Unlock()
}

// L returns the global logger.
func L() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return global
}

// With returns a logger with the given attributes.
func With(args ...any) *slog.Logger {
	return L().With(args...)
}

// Debug logs at debug level.
func Debug(msg string, args ...any) {
	L().Debug(msg, args...)
}

// Info logs at info level.
func Info(msg string, args ...any) {
	L().Info(msg, args...)
}

// Warn logs at warn level.
func Warn(msg string, args ...any) {
	L().Warn(msg, args...)
}

// Error logs at error level.
func Error(msg string, args ...any) {
	L().Error(msg, args...)
}

// DebugContext, InfoContext, etc. for context-aware logging.
func DebugContext(ctx context.Context, msg string, args ...any) {
	L().DebugContext(ctx, msg, args...)
}

func InfoContext(ctx context.Context, msg string, args ...any) {
	L().InfoContext(ctx, msg, args...)
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	L().WarnContext(ctx, msg, args...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	L().ErrorContext(ctx, msg, args...)
}

// ParseLevel converts a string level to slog.Level.
// Accepts: debug, info, warn, error (case-insensitive).
func ParseLevel(s string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug, nil
	case "info", "":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("invalid log level %q (want debug|info|warn|error)", s)
	}
}

// ParseFormat converts a string to Format.
func ParseFormat(s string) (Format, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "text", "":
		return FormatText, nil
	case "json":
		return FormatJSON, nil
	default:
		return FormatText, fmt.Errorf("invalid log format %q (want text|json)", s)
	}
}
