package logging

import (
	"bytes"
	"context"
)

// NewTestContext creates a context with a logger configured per the given flags,
// writing to a new buffer. It returns both the context and the buffer so tests
// can inspect log output.
func NewTestContext(flags Flags) (context.Context, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	l := NewLogger(buf)
	Configure(l, flags)
	return WithLogger(context.Background(), l), buf
}
