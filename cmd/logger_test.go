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

func TestConfigureLogger_DefaultMode(t *testing.T) {
	var buf bytes.Buffer
	old := logger
	logger = logging.NewLogger(&buf)
	defer func() { logger = old }()

	oldVerbose := verbose
	verbose = false
	defer func() { verbose = oldVerbose }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	configureLogger()

	if logger.GetLevel() != log.WarnLevel {
		t.Errorf("expected configureLogger to set WarnLevel for default, got %v", logger.GetLevel())
	}
}
