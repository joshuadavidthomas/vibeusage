package logging

import (
	"bytes"
	"context"
	"testing"

	"github.com/charmbracelet/log"
)

func TestWithLogger_StoresLoggerInContext(t *testing.T) {
	l := NewLogger(&bytes.Buffer{})
	ctx := WithLogger(context.Background(), l)

	got := FromContext(ctx)
	if got != l {
		t.Error("expected FromContext to return the logger stored by WithLogger")
	}
}

func TestFromContext_ReturnsDefaultWhenMissing(t *testing.T) {
	ctx := context.Background()

	got := FromContext(ctx)
	if got == nil {
		t.Fatal("expected FromContext to return a non-nil default logger")
	}
	// Default logger should be at WarnLevel (same as NewLogger)
	if got.GetLevel() != log.WarnLevel {
		t.Errorf("expected default logger at WarnLevel, got %v", got.GetLevel())
	}
}

func TestFromContext_ReturnsStoredLogger_NotDefault(t *testing.T) {
	var buf bytes.Buffer
	custom := NewLogger(&buf)
	Configure(custom, Flags{Verbose: true})

	ctx := WithLogger(context.Background(), custom)
	got := FromContext(ctx)

	if got.GetLevel() != log.DebugLevel {
		t.Errorf("expected stored logger at DebugLevel, got %v", got.GetLevel())
	}
}
