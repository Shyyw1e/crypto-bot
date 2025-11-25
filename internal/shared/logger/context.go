package logger

import "context"

type ctxKey struct{}

var loggerKey ctxKey

var defaultLogger Logger = &nopLogger{}

func IntoContext(ctx context.Context, l Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, loggerKey, l)
}

func FromContext(ctx context.Context) Logger {
	if ctx == nil {
		return defaultLogger
	}
	if l, ok := ctx.Value(loggerKey).(Logger); ok && l != nil {
		return l
	}
	return defaultLogger
}
