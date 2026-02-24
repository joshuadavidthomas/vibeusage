package cmd

import (
	"testing"

	"github.com/charmbracelet/log"
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
