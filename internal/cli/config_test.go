package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
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
	testenv.ApplySameDir(t.Setenv, tmpDir)

	// Create a config file to reset
	cfgDir := tmpDir
	_ = os.MkdirAll(cfgDir, 0o755)
	_ = os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte("[display]\n"), 0o644)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	config.Override(t, config.DefaultConfig())

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
	testenv.ApplySameDir(t.Setenv, tmpDir)

	// Create a config file
	_ = os.WriteFile(filepath.Join(tmpDir, "config.toml"), []byte("[display]\n"), 0o644)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	config.Override(t, config.DefaultConfig())

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

// JSON output tests

func TestConfigResetJSON_UsesTypedStruct(t *testing.T) {
	tmp := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmp)
	config.Override(t, config.DefaultConfig())

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	_ = configResetCmd.Flags().Set("confirm", "true")
	defer func() { _ = configResetCmd.Flags().Set("confirm", "false") }()

	if err := configResetCmd.RunE(configResetCmd, nil); err != nil {
		t.Fatalf("config reset error: %v", err)
	}

	var result display.ActionResultJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("config reset JSON should unmarshal into ActionResultJSON: %v\nOutput: %s", err, buf.String())
	}

	if !result.Success {
		t.Error("success should be true")
	}
	if !result.Reset {
		t.Error("reset should be true")
	}
	if result.Message == "" {
		t.Error("message should not be empty")
	}
}
