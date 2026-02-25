package cli

import (
	"bytes"
	"context"

	"github.com/joshuadavidthomas/vibeusage/internal/logging"
)

func newVerboseContext(logBuf *bytes.Buffer) context.Context {
	l := logging.NewLogger(logBuf)
	logging.Configure(l, logging.Flags{Verbose: true})
	return logging.WithLogger(context.Background(), l)
}

func newQuietContext(logBuf *bytes.Buffer) context.Context {
	l := logging.NewLogger(logBuf)
	logging.Configure(l, logging.Flags{Quiet: true})
	return logging.WithLogger(context.Background(), l)
}

func newDefaultContext(logBuf *bytes.Buffer) context.Context {
	l := logging.NewLogger(logBuf)
	logging.Configure(l, logging.Flags{})
	return logging.WithLogger(context.Background(), l)
}
