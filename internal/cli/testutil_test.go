package cli

import (
	"bytes"
	"context"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/logging"
)

// reloadConfig forces a config reload. Used by tests that modify
// VIBEUSAGE_CONFIG_DIR via t.Setenv before exercising commands.
func reloadConfig() {
	_, _ = config.Reload()
}

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
