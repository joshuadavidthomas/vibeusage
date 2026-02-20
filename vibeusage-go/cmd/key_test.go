package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
)

func TestKeySet_UsesInputPrompt(t *testing.T) {
	mock := &prompt.Mock{
		InputFunc: func(cfg prompt.InputConfig) (string, error) {
			if cfg.Validate == nil {
				t.Error("key set should have validation")
			}
			// Verify it rejects empty
			if err := cfg.Validate(""); err == nil {
				t.Error("validation should reject empty input")
			}
			return "my-credential-value", nil
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

	reloadConfig()

	// Get the claude subcommand, then its "set" subcommand
	claudeCmd := findSubcommand(keyCmd, "claude")
	if claudeCmd == nil {
		t.Fatal("expected 'claude' subcommand under 'key'")
	}
	setCmd := findSubcommand(claudeCmd, "set")
	if setCmd == nil {
		t.Fatal("expected 'set' subcommand under 'key claude'")
	}

	err := setCmd.RunE(setCmd, nil) // no args = prompt for value
	if err != nil {
		t.Fatalf("key set error: %v", err)
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

func TestKeyDelete_UsesConfirmPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)

	// Create a credential to delete
	credDir := filepath.Join(tmpDir, "credentials", "claude")
	os.MkdirAll(credDir, 0o755)
	credPath := filepath.Join(credDir, "session.json")
	os.WriteFile(credPath, []byte(`{"key":"test"}`), 0o600)

	mock := &prompt.Mock{
		ConfirmFunc: func(cfg prompt.ConfirmConfig) (bool, error) {
			return true, nil // user confirms deletion
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	reloadConfig()

	claudeCmd := findSubcommand(keyCmd, "claude")
	if claudeCmd == nil {
		t.Fatal("expected 'claude' subcommand under 'key'")
	}
	deleteCmd := findSubcommand(claudeCmd, "delete")
	if deleteCmd == nil {
		t.Fatal("expected 'delete' subcommand under 'key claude'")
	}

	err := deleteCmd.RunE(deleteCmd, nil)
	if err != nil {
		t.Fatalf("key delete error: %v", err)
	}

	if len(mock.ConfirmCalls) != 1 {
		t.Fatalf("expected 1 Confirm call, got %d", len(mock.ConfirmCalls))
	}
}

func TestKeyDelete_UserDeclinesConfirm(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)

	credDir := filepath.Join(tmpDir, "credentials", "claude")
	os.MkdirAll(credDir, 0o755)
	credPath := filepath.Join(credDir, "session.json")
	os.WriteFile(credPath, []byte(`{"key":"test"}`), 0o600)

	mock := &prompt.Mock{
		ConfirmFunc: func(cfg prompt.ConfirmConfig) (bool, error) {
			return false, nil // user says no
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	reloadConfig()

	claudeCmd := findSubcommand(keyCmd, "claude")
	deleteCmd := findSubcommand(claudeCmd, "delete")

	err := deleteCmd.RunE(deleteCmd, nil)
	if err != nil {
		t.Fatalf("key delete error: %v", err)
	}

	// File should still exist
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		t.Error("credential file should not have been deleted")
	}
}

// findSubcommand finds a subcommand by name.
func findSubcommand(parent *cobra.Command, name string) *cobra.Command {
	for _, c := range parent.Commands() {
		if c.Name() == name {
			return c
		}
	}
	return nil
}
