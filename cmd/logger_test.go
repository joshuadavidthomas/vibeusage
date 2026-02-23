package cmd

import (
	"bytes"
	"testing"

	"github.com/charmbracelet/log"
	"github.com/joshuadavidthomas/vibeusage/internal/logging"
)

func TestLogger_InitializedExplicitly(t *testing.T) {
	if logger == nil {
		t.Fatal("expected logger to be non-nil")
	}
}

func TestLogger_DefaultLevelIsWarn(t *testing.T) {
	// Create a fresh logger like the package var does
	var buf bytes.Buffer
	l := logging.NewLogger(&buf)
	if l.GetLevel() != log.WarnLevel {
		t.Errorf("expected default level WarnLevel, got %v", l.GetLevel())
	}
}

func TestConfigureLogger_UsesPackageLogger(t *testing.T) {
	var buf bytes.Buffer
	old := logger
	logger = logging.NewLogger(&buf)
	defer func() { logger = old }()

	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	configureLogger()

	if logger.GetLevel() != log.DebugLevel {
		t.Errorf("expected configureLogger to set DebugLevel on cmd logger, got %v", logger.GetLevel())
	}
}

func TestConfigureLogger_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	old := logger
	logger = logging.NewLogger(&buf)
	defer func() { logger = old }()

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	configureLogger()

	if logger.GetLevel() != log.ErrorLevel {
		t.Errorf("expected configureLogger to set ErrorLevel for quiet, got %v", logger.GetLevel())
	}
}
