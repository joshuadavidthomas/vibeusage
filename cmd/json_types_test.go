package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/display"
)

func TestAuthStatusJSON_UsesTypedStruct(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	_ = authStatusCommand()

	var result map[string]display.AuthStatusEntryJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("auth status JSON should unmarshal into map[string]AuthStatusEntryJSON: %v\nOutput: %s", err, buf.String())
	}

	// Should have at least one provider entry
	if len(result) == 0 {
		t.Error("expected at least one provider in auth status")
	}
}

func TestConfigShowJSON_UsesTypedStruct(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	if err := configShowCmd.RunE(configShowCmd, nil); err != nil {
		t.Fatalf("config show error: %v", err)
	}

	var result display.ConfigShowJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("config show JSON should unmarshal into ConfigShowJSON: %v\nOutput: %s", err, buf.String())
	}

	if result.Path == "" {
		t.Error("path should not be empty")
	}
}

func TestConfigResetJSON_UsesTypedStruct(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

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

func TestKeyStatusJSON_UsesTypedStruct(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	_ = displayAllCredentialStatus()

	var result map[string]display.KeyStatusEntryJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("key status JSON should unmarshal into map[string]KeyStatusEntryJSON: %v\nOutput: %s", err, buf.String())
	}
}

func TestInitJSON_UsesTypedStruct(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("init error: %v", err)
	}

	var result display.InitStatusJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("init JSON should unmarshal into InitStatusJSON: %v\nOutput: %s", err, buf.String())
	}

	if len(result.AvailableProviders) == 0 {
		t.Error("available_providers should not be empty")
	}
}

func TestCacheClearJSON_UsesTypedStruct(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmp)
	reloadConfig()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	if err := cacheClearCmd.RunE(cacheClearCmd, nil); err != nil {
		t.Fatalf("cache clear error: %v", err)
	}

	var result display.ActionResultJSON
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("cache clear JSON should unmarshal into ActionResultJSON: %v\nOutput: %s", err, buf.String())
	}

	if !result.Success {
		t.Error("success should be true")
	}
	if result.Message == "" {
		t.Error("message should not be empty")
	}
}
