package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
)

func TestConfigReset_UsesConfirm(t *testing.T) {
	mock := &prompt.Mock{
		ConfirmFunc: func(cfg prompt.ConfirmConfig) (bool, error) {
			return true, nil
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)

	// Create a config file to reset
	cfgDir := tmpDir
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("[display]\n"), 0o644)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	reloadConfig()

	err := configResetCmd.RunE(configResetCmd, nil)
	if err != nil {
		t.Fatalf("config reset error: %v", err)
	}

	if len(mock.ConfirmCalls) != 1 {
		t.Fatalf("expected 1 Confirm call, got %d", len(mock.ConfirmCalls))
	}
}

func TestConfigReset_UserDeclinesConfirm(t *testing.T) {
	mock := &prompt.Mock{
		ConfirmFunc: func(cfg prompt.ConfirmConfig) (bool, error) {
			return false, nil // user says no
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)

	// Create a config file
	_ = os.WriteFile(filepath.Join(tmpDir, "config.toml"), []byte("[display]\n"), 0o644)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	reloadConfig()

	cmd := configResetCmd
	err := cmd.RunE(cmd, nil)
	if err != nil {
		t.Fatalf("config reset error: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("cancelled")) {
		t.Error("expected 'cancelled' message when user declines")
	}

	// File should still exist
	if _, err := os.Stat(filepath.Join(tmpDir, "config.toml")); os.IsNotExist(err) {
		t.Error("config file should not have been removed")
	}
}
