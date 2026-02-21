package logging

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
)

func TestNewLogger_DefaultLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)

	if l.GetLevel() != log.WarnLevel {
		t.Errorf("expected default level WarnLevel, got %v", l.GetLevel())
	}
}

func TestNewLogger_WritesToProvidedWriter(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	l.SetLevel(log.DebugLevel)

	l.Debug("test message")

	if !strings.Contains(buf.String(), "test message") {
		t.Errorf("expected output to contain 'test message', got %q", buf.String())
	}
}

func TestConfigure_VerboseSetsDebugLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)

	Configure(l, Flags{Verbose: true})

	if l.GetLevel() != log.DebugLevel {
		t.Errorf("expected DebugLevel for verbose, got %v", l.GetLevel())
	}
}

func TestConfigure_QuietSetsErrorLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)

	Configure(l, Flags{Quiet: true})

	if l.GetLevel() != log.ErrorLevel {
		t.Errorf("expected ErrorLevel for quiet, got %v", l.GetLevel())
	}
}

func TestConfigure_QuietTakesPrecedenceOverVerbose(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)

	Configure(l, Flags{Verbose: true, Quiet: true})

	if l.GetLevel() != log.ErrorLevel {
		t.Errorf("expected ErrorLevel when both quiet and verbose set, got %v", l.GetLevel())
	}
}

func TestConfigure_DefaultSetsWarnLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)

	Configure(l, Flags{})

	if l.GetLevel() != log.WarnLevel {
		t.Errorf("expected WarnLevel for default, got %v", l.GetLevel())
	}
}

func TestConfigure_NoColorDisablesColors(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	l.SetLevel(log.DebugLevel)

	Configure(l, Flags{Verbose: true, NoColor: true})

	l.Debug("no color test")

	output := buf.String()
	// With Ascii profile, output should have no ANSI escape sequences
	if strings.Contains(output, "\033[") {
		t.Errorf("expected no ANSI escape codes with NoColor, got %q", output)
	}
}

func TestConfigure_JSONSetsJSONFormatter(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	l.SetLevel(log.DebugLevel)

	Configure(l, Flags{Verbose: true, JSON: true})

	l.Debug("json test", "key", "value")

	output := buf.String()
	// JSON output should contain typical JSON structure markers
	if !strings.Contains(output, `"msg"`) || !strings.Contains(output, `"key"`) {
		t.Errorf("expected JSON formatted output, got %q", output)
	}
}

func TestDebugMessages_SuppressedAtDefaultLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)

	l.Debug("should not appear")

	if buf.Len() != 0 {
		t.Errorf("expected no output at default level, got %q", buf.String())
	}
}

func TestDebugMessages_ShownWhenVerbose(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	Configure(l, Flags{Verbose: true})

	l.Debug("should appear")

	if !strings.Contains(buf.String(), "should appear") {
		t.Errorf("expected debug message in verbose mode, got %q", buf.String())
	}
}

func TestStructuredKeyValues(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	Configure(l, Flags{Verbose: true, JSON: true})

	l.Debug("fetch complete", "provider", "claude", "duration_ms", 342)

	output := buf.String()
	if !strings.Contains(output, "claude") {
		t.Errorf("expected 'claude' in output, got %q", output)
	}
	if !strings.Contains(output, "342") {
		t.Errorf("expected '342' in output, got %q", output)
	}
}

func TestWarnMessages_ShownAtDefaultLevel(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)

	l.Warn("a warning")

	if !strings.Contains(buf.String(), "a warning") {
		t.Errorf("expected warning to appear at default level, got %q", buf.String())
	}
}

func TestErrorMessages_ShownInQuietMode(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	Configure(l, Flags{Quiet: true})

	l.Error("an error")

	if !strings.Contains(buf.String(), "an error") {
		t.Errorf("expected error to appear in quiet mode, got %q", buf.String())
	}
}

func TestWarnMessages_SuppressedInQuietMode(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	Configure(l, Flags{Quiet: true})

	l.Warn("quiet warning")

	if buf.Len() != 0 {
		t.Errorf("expected no output for warn in quiet mode, got %q", buf.String())
	}
}
