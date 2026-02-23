package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// resetPathFlags resets configPathCmd flags to defaults and registers
// cleanup to restore them after the test, preventing inter-test leakage.
func resetPathFlags(t *testing.T) {
	t.Helper()
	_ = configPathCmd.Flags().Set("cache", "false")
	_ = configPathCmd.Flags().Set("credentials", "false")
	t.Cleanup(func() {
		_ = configPathCmd.Flags().Set("cache", "false")
		_ = configPathCmd.Flags().Set("credentials", "false")
	})
}

// Root command tests

func TestRootCmd_HasExpectedSubcommands(t *testing.T) {
	expected := []string{"auth", "status", "config", "cache", "key", "init"}
	for _, name := range expected {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rootCmd missing expected subcommand %q", name)
		}
	}
}

func TestRootCmd_HasProviderSubcommands(t *testing.T) {
	providers := []string{"claude", "codex", "copilot", "cursor", "gemini"}
	for _, name := range providers {
		found := false
		for _, cmd := range rootCmd.Commands() {
			if cmd.Name() == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("rootCmd missing provider subcommand %q", name)
		}
	}
}

func TestRootCmd_HasPersistentFlags(t *testing.T) {
	flags := []string{"json", "no-color", "verbose", "quiet"}
	for _, name := range flags {
		f := rootCmd.PersistentFlags().Lookup(name)
		if f == nil {
			t.Errorf("rootCmd missing persistent flag %q", name)
		}
	}
}

func TestRootCmd_VersionFlag(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	// Set up temp dir to avoid first-run check
	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)
	reloadConfig()

	rootCmd.SetArgs([]string{"--version"})
	defer rootCmd.SetArgs(nil)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("root --version error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "vibeusage") {
		t.Errorf("expected 'vibeusage' in version output, got: %q", output)
	}
	if !strings.Contains(output, version) {
		t.Errorf("expected version %q in output, got: %q", version, output)
	}
}

// Config show tests

func TestConfigShow_DefaultOutput(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)
	reloadConfig()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	if err := configShowCmd.RunE(configShowCmd, nil); err != nil {
		t.Fatalf("config show error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Config:") {
		t.Errorf("expected 'Config:' prefix, got: %q", output)
	}
	// Should contain TOML output
	if !strings.Contains(output, "timeout") {
		t.Errorf("expected config fields in output, got: %q", output)
	}
}

func TestConfigShow_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)
	reloadConfig()

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	if err := configShowCmd.RunE(configShowCmd, nil); err != nil {
		t.Fatalf("config show error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	// Quiet mode should only show the path
	if !strings.Contains(output, "config.toml") {
		t.Errorf("quiet mode should show config path, got: %q", output)
	}
}

func TestConfigShow_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)
	reloadConfig()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	if err := configShowCmd.RunE(configShowCmd, nil); err != nil {
		t.Fatalf("config show --json error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("config show --json output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}

	if _, ok := parsed["fetch"]; !ok {
		t.Error("JSON output missing 'fetch' key")
	}
	if _, ok := parsed["display"]; !ok {
		t.Error("JSON output missing 'display' key")
	}
	if _, ok := parsed["credentials"]; !ok {
		t.Error("JSON output missing 'credentials' key")
	}
	if _, ok := parsed["path"]; !ok {
		t.Error("JSON output missing 'path' key")
	}
}

// Config path tests

func TestConfigPath_DefaultOutput(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)
	t.Setenv("VIBEUSAGE_CACHE_DIR", tmpDir)
	reloadConfig()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	resetPathFlags(t)

	if err := configPathCmd.RunE(configPathCmd, nil); err != nil {
		t.Fatalf("config path error: %v", err)
	}

	output := buf.String()
	for _, expected := range []string{"Config dir:", "Config file:", "Cache dir:", "Credentials:"} {
		if !strings.Contains(output, expected) {
			t.Errorf("expected %q in output, got: %q", expected, output)
		}
	}
}

func TestConfigPath_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)
	reloadConfig()

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	resetPathFlags(t)

	if err := configPathCmd.RunE(configPathCmd, nil); err != nil {
		t.Fatalf("config path error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != tmpDir {
		t.Errorf("quiet mode should output just the dir, got: %q", output)
	}
}

func TestConfigPath_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CONFIG_DIR", tmpDir)
	t.Setenv("VIBEUSAGE_CACHE_DIR", tmpDir)
	reloadConfig()

	oldJSON := jsonOutput
	jsonOutput = true
	defer func() { jsonOutput = oldJSON }()

	resetPathFlags(t)

	if err := configPathCmd.RunE(configPathCmd, nil); err != nil {
		t.Fatalf("config path --json error: %v", err)
	}

	var parsed map[string]string
	if err := json.Unmarshal(buf.Bytes(), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}

	for _, key := range []string{"config_dir", "config_file", "cache_dir", "credentials_dir"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("JSON output missing %q key", key)
		}
	}
}

func TestConfigPath_CacheFlag(t *testing.T) {
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	tmpDir := t.TempDir()
	t.Setenv("VIBEUSAGE_CACHE_DIR", tmpDir)
	reloadConfig()

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	resetPathFlags(t)
	_ = configPathCmd.Flags().Set("cache", "true")

	if err := configPathCmd.RunE(configPathCmd, nil); err != nil {
		t.Fatalf("config path --cache error: %v", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != tmpDir {
		t.Errorf("expected cache dir %q, got %q", tmpDir, output)
	}
}
