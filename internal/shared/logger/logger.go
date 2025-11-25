package logger

import (
	"log/slog"
	"os"
	"strings"
)

type Logger interface {
	Debug(msg string, kv ...any)
	Info(msg string, kv ...any)
	Warn(msg string, kv ...any)
	Error(msg string, kv ...any)

	With(kv ...any) Logger
}

type slogLogger struct {
	inner *slog.Logger
}

func (l *slogLogger) Debug(msg string, kv ...any) {
	l.inner.Debug(msg, kv...)
}

func (l *slogLogger) Info(msg string, kv ...any) {
	l.inner.Info(msg, kv...)
}

func (l *slogLogger) Warn(msg string, kv ...any) {
	l.inner.Warn(msg, kv...)
}

func (l *slogLogger) Error(msg string, kv ...any) {
	l.inner.Error(msg, kv...)
}

func (l *slogLogger) With(kv ...any) Logger {
	return &slogLogger{
		inner: l.inner.With(kv...),
	}
}

// New создаёт JSON-логгер на stdout с уровнем level и полем "service".
func New(level, service string) Logger {
	level = strings.ToLower(strings.TrimSpace(level))

	var slogLevel slog.Level
	switch level {
	case "debug":
		slogLevel = slog.LevelDebug
	case "warn", "warning":
		slogLevel = slog.LevelWarn
	case "error":
		slogLevel = slog.LevelError
	case "info", "":
		fallthrough
	default:
		slogLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slogLevel,
	})

	base := slog.New(handler)
	if service != "" {
		base = base.With("service", service)
	}

	return &slogLogger{
		inner: base,
	}
}

type nopLogger struct{}

func (n *nopLogger) Debug(string, ...any) {}
func (n *nopLogger) Info(string, ...any)  {}
func (n *nopLogger) Warn(string, ...any)  {}
func (n *nopLogger) Error(string, ...any) {}
func (n *nopLogger) With(...any) Logger   { return n }
