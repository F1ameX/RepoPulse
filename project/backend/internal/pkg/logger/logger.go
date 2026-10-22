// Package logger carries structured loggers through context.Context.
package logger

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

// With returns a child context containing l without changing its parent.
func With(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// From returns the context logger, or slog.Default when it is absent or nil.
// Missing logging setup must not prevent a request from being served.
func From(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}
