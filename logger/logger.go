package logger

import (
	"context"
	"log/slog"
)

var logger *slog.Logger = slog.New(&noopHandler{})

// SetLogger sets the Depot logger to the given logger.
func SetLogger(l *slog.Logger) {
	logger = l
}

// GetLogger returns the Depot logger.
func GetLogger() *slog.Logger {
	return logger
}

// With calls [(slog.Logger).With] on the default logger.
func With(args ...any) *slog.Logger {
	if logger == nil {
		return nil
	}
	return logger.With(args...)
}

// Enabled reports whether the logger emits log records at the given context and level.
func Enabled(ctx context.Context, level slog.Level) bool {
	if logger == nil {
		return false
	}
	return logger.Enabled(ctx, level)
}

// Debug calls [(slog.Logger).Debug] on the Depot logger, if configured.
func Debug(msg string, args ...any) {
	logger.Debug(msg, args...)
}

// DebugContext calls [(slog.Logger).DebugContext] on the Depot logger, if configured.
func DebugContext(ctx context.Context, msg string, args ...any) {
	logger.DebugContext(ctx, msg, args...)
}

// Info calls [(slog.Logger).Info] on the Depot logger, if configured.
func Info(msg string, args ...any) {
	logger.Info(msg, args...)
}

// InfoContext calls [(slog.Logger).InfoContext] on the Depot logger, if configured.
func InfoContext(ctx context.Context, msg string, args ...any) {
	logger.InfoContext(ctx, msg, args...)
}

// Warn calls [(slog.Logger).Warn] on the Depot logger, if configured.
func Warn(msg string, args ...any) {
	logger.Warn(msg, args...)
}

// WarnContext calls [(slog.Logger).WarnContext] on the Depot logger, if configured.
func WarnContext(ctx context.Context, msg string, args ...any) {
	logger.WarnContext(ctx, msg, args...)
}

// Error calls [(slog.Logger).Error] on the Depot logger, if configured.
func Error(msg string, args ...any) {
	logger.Error(msg, args...)
}

// ErrorContext calls [(slog.Logger).ErrorContext] on the Depot logger, if configured.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	logger.ErrorContext(ctx, msg, args...)
}

// Log calls [(slog.Logger).Log] on the Depot logger, if configured.
func Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	logger.Log(ctx, level, msg, args...)
}

// LogAttrs calls [(slog.Logger).LogAttrs] on the Depot logger, if configured.
func LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	logger.LogAttrs(ctx, level, msg, attrs...)
}

type noopHandler struct{}

func (h *noopHandler) Enabled(context.Context, slog.Level) bool  { return false }
func (h *noopHandler) Handle(context.Context, slog.Record) error { return nil }
func (h *noopHandler) WithAttrs(attrs []slog.Attr) slog.Handler  { return h }
func (h *noopHandler) WithGroup(name string) slog.Handler        { return h }
