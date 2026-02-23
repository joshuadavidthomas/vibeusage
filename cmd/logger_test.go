package cmd

import (
	"bytes"
	"context"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/joshuadavidthomas/vibeusage/internal/logging"
)

func TestNewConfiguredLogger_Verbose(t *testing.T) {
	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	l := newConfiguredLogger()

	if l.GetLevel() != log.DebugLevel {
		t.Errorf("expected DebugLevel for verbose, got %v", l.GetLevel())
	}
}

func TestNewConfiguredLogger_Quiet(t *testing.T) {
	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	l := newConfiguredLogger()

	if l.GetLevel() != log.ErrorLevel {
		t.Errorf("expected ErrorLevel for quiet, got %v", l.GetLevel())
	}
}

func TestNewConfiguredLogger_Default(t *testing.T) {
	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	l := newConfiguredLogger()

	if l.GetLevel() != log.WarnLevel {
		t.Errorf("expected WarnLevel for default, got %v", l.GetLevel())
	}
}

func TestLoggerFromContext_ReturnsInjectedLogger(t *testing.T) {
	var buf bytes.Buffer
	l := logging.NewLogger(&buf)
	logging.Configure(l, logging.Flags{Verbose: true})

	ctx := logging.WithLogger(context.Background(), l)
	got := logging.FromContext(ctx)

	if got != l {
		t.Error("expected FromContext to return the injected logger")
	}
}

func TestLoggerFromContext_FallsBackToDiscard(t *testing.T) {
	ctx := context.Background()
	got := logging.FromContext(ctx)

	if got == nil {
		t.Fatal("expected non-nil fallback logger")
	}
	// Fallback should be at WarnLevel and write to discard
	if got.GetLevel() != log.WarnLevel {
		t.Errorf("expected fallback at WarnLevel, got %v", got.GetLevel())
	}
}
