package logging

import (
	"context"
	"io"

	"github.com/charmbracelet/log"
)

type contextKey struct{}

// WithLogger returns a new context with the given logger attached.
func WithLogger(ctx context.Context, l *log.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}

// FromContext retrieves the logger from the context.
// If no logger is stored, it returns a default WarnLevel logger
// that discards output.
func FromContext(ctx context.Context) *log.Logger {
	if l, ok := ctx.Value(contextKey{}).(*log.Logger); ok {
		return l
	}
	return NewLogger(io.Discard)
}
