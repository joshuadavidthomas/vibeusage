package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
)

func TestAuthClaude_UsesInputWithValidation(t *testing.T) {
	mock := &prompt.Mock{
		InputFunc: func(cfg prompt.InputConfig) (string, error) {
			if cfg.Title == "" {
				t.Error("Input title should not be empty")
			}
			if cfg.Validate == nil {
				t.Error("Claude auth should have a validation function")
			}
			// Verify validation rejects bad keys
			if err := cfg.Validate("bad-key"); err == nil {
				t.Error("validation should reject non-prefixed keys")
			}
			// Verify validation accepts good keys
			if err := cfg.Validate("sk-ant-sid01-abc123"); err != nil {
				t.Errorf("validation should accept valid keys: %v", err)
			}
			return "sk-ant-sid01-test123", nil
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	// Use temp dir for credentials
	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	err := authClaude()
	if err != nil {
		t.Fatalf("authClaude() error: %v", err)
	}

	if len(mock.InputCalls) != 1 {
		t.Fatalf("expected 1 Input call, got %d", len(mock.InputCalls))
	}

	// Verify credential was saved
	credPath := filepath.Join(tmpDir, "credentials", "claude", "session.json")
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		t.Error("expected credential file to be created")
	}
}

func TestAuthCursor_UsesInputWithValidation(t *testing.T) {
	mock := &prompt.Mock{
		InputFunc: func(cfg prompt.InputConfig) (string, error) {
			if cfg.Validate == nil {
				t.Error("Cursor auth should have a validation function")
			}
			// Verify it rejects empty
			if err := cfg.Validate(""); err == nil {
				t.Error("validation should reject empty input")
			}
			return "test-session-token", nil
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	err := authCursor()
	if err != nil {
		t.Fatalf("authCursor() error: %v", err)
	}

	if len(mock.InputCalls) != 1 {
		t.Fatalf("expected 1 Input call, got %d", len(mock.InputCalls))
	}
}

func TestAuthStatusCommand_HasTableBorders(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldNoColor := noColor
	noColor = false
	defer func() { noColor = oldNoColor }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	authStatusCommand()

	output := buf.String()

	if !strings.Contains(output, "╭") {
		t.Errorf("expected lipgloss rounded border in auth status, got:\n%s", output)
	}
}

func TestAuthStatusCommand_ContainsHeaders(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldNoColor := noColor
	noColor = true
	defer func() { noColor = oldNoColor }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	authStatusCommand()

	output := buf.String()
	for _, header := range []string{"Provider", "Status", "Source"} {
		if !strings.Contains(output, header) {
			t.Errorf("output missing header %q\n\nGot:\n%s", header, output)
		}
	}
}

func TestAuthStatusCommand_QuietMode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	authStatusCommand()

	output := buf.String()
	if strings.Contains(output, "╭") {
		t.Error("quiet mode should not use table borders")
	}
}

func TestAuthCopilot_UsesConfirmForReauth(t *testing.T) {
	// Set up credentials so the "already authenticated" path is hit
	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)

	credDir := filepath.Join(tmpDir, "credentials", "copilot")
	os.MkdirAll(credDir, 0o755)
	os.WriteFile(filepath.Join(credDir, "oauth.json"), []byte(`{"access_token":"test"}`), 0o600)

	mock := &prompt.Mock{
		ConfirmFunc: func(cfg prompt.ConfirmConfig) (bool, error) {
			return false, nil // user says no to re-auth
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	// Force config reload to pick up new env
	reloadConfig()

	err := authCopilot()
	if err != nil {
		t.Fatalf("authCopilot() error: %v", err)
	}

	if len(mock.ConfirmCalls) != 1 {
		t.Fatalf("expected 1 Confirm call, got %d", len(mock.ConfirmCalls))
	}
}
