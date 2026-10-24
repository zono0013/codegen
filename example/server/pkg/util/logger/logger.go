package logger

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"example.com/blog/server/pkg/util/env"
)

type ctxKey struct{}

var (
	mu            sync.RWMutex
	currentLogger = slog.Default()
)

// Init reads env.AppEnv() / env.LogLevel() and sets up the package-level logger.
// Call this once at the very top of cmd/api/main.go before serving.
func Init() {
	var level slog.Level
	if env.LogLevel() == "debug" {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}

	var base slog.Handler
	switch env.AppEnv() {
	case "local", "dev":
		base = slog.NewTextHandler(os.Stderr, opts)
	default:
		base = slog.NewJSONHandler(os.Stdout, opts)
	}

	l := slog.New(&handlerWithCtxAttrs{base: base})

	mu.Lock()
	currentLogger = l
	mu.Unlock()

	slog.SetDefault(l)
}

// L returns the current *slog.Logger. Use this in tests to swap the logger
// without touching slog.SetDefault (avoids parallel-test races).
func L() *slog.Logger {
	mu.RLock()
	defer mu.RUnlock()
	return currentLogger
}

// Debug logs at DEBUG level using the current logger.
func Debug(ctx context.Context, msg string, args ...any) {
	L().DebugContext(ctx, msg, args...)
}

// Info logs at INFO level using the current logger.
func Info(ctx context.Context, msg string, args ...any) {
	L().InfoContext(ctx, msg, args...)
}

// Warn logs at WARN level using the current logger.
func Warn(ctx context.Context, msg string, args ...any) {
	L().WarnContext(ctx, msg, args...)
}

// Error logs at ERROR level using the current logger.
func Error(ctx context.Context, msg string, args ...any) {
	L().ErrorContext(ctx, msg, args...)
}

// WithAttrs injects slog.Attr values into ctx. Handlers created by Init will
// automatically expand these attrs on every log record that carries this ctx.
// Existing attrs in ctx are preserved and new ones are appended.
func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	existing, _ := ctx.Value(ctxKey{}).([]slog.Attr)
	merged := make([]slog.Attr, len(existing)+len(attrs))
	copy(merged, existing)
	copy(merged[len(existing):], attrs)
	return context.WithValue(ctx, ctxKey{}, merged)
}

// handlerWithCtxAttrs wraps a slog.Handler and automatically adds any []slog.Attr
// stored in the context to every log record before delegating to the base handler.
type handlerWithCtxAttrs struct {
	base slog.Handler
}

func (h *handlerWithCtxAttrs) Enabled(ctx context.Context, level slog.Level) bool {
	return h.base.Enabled(ctx, level)
}

func (h *handlerWithCtxAttrs) Handle(ctx context.Context, r slog.Record) error {
	if attrs, ok := ctx.Value(ctxKey{}).([]slog.Attr); ok && len(attrs) > 0 {
		r.AddAttrs(attrs...)
	}
	return h.base.Handle(ctx, r)
}

func (h *handlerWithCtxAttrs) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &handlerWithCtxAttrs{base: h.base.WithAttrs(attrs)}
}

func (h *handlerWithCtxAttrs) WithGroup(name string) slog.Handler {
	return &handlerWithCtxAttrs{base: h.base.WithGroup(name)}
}
