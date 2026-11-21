// Package logger хранит slog-логгер в context, чтобы request_id попадал в каждый лог.
package logger

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

// WithContext кладёт логгер в контекст.
func WithContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext достаёт логгер из контекста (с уже прикреплённым request_id).
// Если логгера нет — возвращает slog.Default().
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}
