package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
	"github.com/joshuadavidthomas/vibeusage/internal/provider"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
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
			// Verify validation accepts supported keys
			sessionKey := "sk-ant-" + "sid01-" + "abc123"
			if err := cfg.Validate(sessionKey); err != nil {
				t.Errorf("validation should accept valid session key: %v", err)
			}
			// API keys are no longer accepted for Claude auth
			apiKey := "sk-ant-" + "api03-" + "abc123"
			if err := cfg.Validate(apiKey); err == nil {
				t.Error("validation should reject api keys")
			}
			return "sk-ant-" + "sid01-" + "test123", nil
		},
		ConfirmFunc: func(cfg prompt.ConfirmConfig) (bool, error) {
			return false, nil // decline detected creds, enter new
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	// Use temp dir for credentials; disable provider CLI reuse to avoid
	// detecting real Claude CLI credentials on the host.
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	t.Setenv("ANTHROPIC_API_KEY", "")
	config.Override(t, config.DefaultConfig())

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	p, _ := provider.Get("claude")
	err := authProvider("claude", p)
	if err != nil {
		t.Fatalf("authProvider(claude) error: %v", err)
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
	testenv.ApplySameDir(t.Setenv, tmpDir)
	t.Setenv("CURSOR_API_KEY", "")
	config.Override(t, config.DefaultConfig())

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	p, _ := provider.Get("cursor")
	err := authProvider("cursor", p)
	if err != nil {
		t.Fatalf("authProvider(cursor) error: %v", err)
	}

	if len(mock.InputCalls) != 1 {
		t.Fatalf("expected 1 Input call, got %d", len(mock.InputCalls))
	}
}

func TestAuthStatusCommand_HasTableBorders(t *testing.T) {
	tmp := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmp)
	config.Override(t, config.DefaultConfig())

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

	_ = authStatusCommand()

	output := buf.String()

	if !strings.Contains(output, "╭") {
		t.Errorf("expected lipgloss rounded border in auth status, got:\n%s", output)
	}
}

func TestAuthStatusCommand_ContainsHeaders(t *testing.T) {
	tmp := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmp)
	config.Override(t, config.DefaultConfig())

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

	_ = authStatusCommand()

	output := buf.String()
	for _, header := range []string{"Provider", "Status", "Source"} {
		if !strings.Contains(output, header) {
			t.Errorf("output missing header %q\n\nGot:\n%s", header, output)
		}
	}
}

func TestAuthStatusCommand_QuietMode(t *testing.T) {
	tmp := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmp)
	config.Override(t, config.DefaultConfig())

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	_ = authStatusCommand()

	output := buf.String()
	if strings.Contains(output, "╭") {
		t.Error("quiet mode should not use table borders")
	}
}

func TestAuthCopilot_UsesConfirmForReauth(t *testing.T) {
	// Set up credentials so the "already authenticated" path is hit
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)

	credDir := filepath.Join(tmpDir, "credentials", "copilot")
	_ = os.MkdirAll(credDir, 0o755)
	_ = os.WriteFile(filepath.Join(credDir, "oauth.json"), []byte(`{"access_token":"test"}`), 0o600)

	// Stub verify so it doesn't make real network calls
	oldVerify := verifyCredentialsFn
	verifyCredentialsFn = func(string) bool { return true }
	defer func() { verifyCredentialsFn = oldVerify }()

	mock := &prompt.Mock{
		ConfirmFunc: func(cfg prompt.ConfirmConfig) (bool, error) {
			return true, nil // user says yes to use existing creds
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	// Force config reload to pick up new env
	config.Override(t, config.DefaultConfig())

	p, _ := provider.Get("copilot")
	err := authProvider("copilot", p)
	if err != nil {
		t.Fatalf("authProvider(copilot) error: %v", err)
	}

	if len(mock.ConfirmCalls) != 1 {
		t.Fatalf("expected 1 Confirm call, got %d", len(mock.ConfirmCalls))
	}
}

// --delete flag tests

func TestAuthDelete_RemovesCredentialsAndDisables(t *testing.T) {
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	// Create credentials
	credDir := filepath.Join(tmpDir, "credentials", "claude")
	_ = os.MkdirAll(credDir, 0o755)
	_ = os.WriteFile(filepath.Join(credDir, "session.json"), []byte(`{"session_key":"test"}`), 0o600)

	// Enable the provider
	_ = config.WriteEnabledProviders([]string{"claude", "copilot"})

	mock := &prompt.Mock{
		ConfirmFunc: func(cfg prompt.ConfirmConfig) (bool, error) {
			if !strings.Contains(cfg.Title, "Claude") {
				t.Errorf("expected Claude in confirm title, got %q", cfg.Title)
			}
			return true, nil
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	err := authDeleteProvider("claude")
	if err != nil {
		t.Fatalf("authDeleteProvider error: %v", err)
	}

	// Credential should be gone
	if _, err := os.Stat(filepath.Join(credDir, "session.json")); !os.IsNotExist(err) {
		t.Error("credential file should have been deleted")
	}

	// Should be removed from enabled list
	enabled := config.ReadEnabledProviders()
	for _, id := range enabled {
		if id == "claude" {
			t.Error("claude should have been removed from enabled providers")
		}
	}
	if len(enabled) != 1 || enabled[0] != "copilot" {
		t.Errorf("expected [copilot], got %v", enabled)
	}

	if len(mock.ConfirmCalls) != 1 {
		t.Fatalf("expected 1 Confirm call, got %d", len(mock.ConfirmCalls))
	}
}

func TestAuthDelete_UserDeclinesConfirm(t *testing.T) {
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	credDir := filepath.Join(tmpDir, "credentials", "claude")
	_ = os.MkdirAll(credDir, 0o755)
	credPath := filepath.Join(credDir, "session.json")
	_ = os.WriteFile(credPath, []byte(`{"session_key":"test"}`), 0o600)

	mock := &prompt.Mock{
		ConfirmFunc: func(cfg prompt.ConfirmConfig) (bool, error) {
			return false, nil
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	err := authDeleteProvider("claude")
	if err != nil {
		t.Fatalf("authDeleteProvider error: %v", err)
	}

	// Credential should still exist
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		t.Error("credential file should not have been deleted")
	}
}

// --token flag tests

func TestAuthSetToken_SavesCredentialAndEnables(t *testing.T) {
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	t.Setenv("CURSOR_API_KEY", "")
	config.Override(t, config.DefaultConfig())

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	p, _ := provider.Get("cursor")
	err := authSetToken("cursor", p, "my-session-token")
	if err != nil {
		t.Fatalf("authSetToken error: %v", err)
	}

	// Credential should be saved
	credPath := filepath.Join(tmpDir, "credentials", "cursor", "session.json")
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		t.Error("expected credential file to be created")
	}

	// Should be enabled
	enabled := config.ReadEnabledProviders()
	found := false
	for _, id := range enabled {
		if id == "cursor" {
			found = true
		}
	}
	if !found {
		t.Error("cursor should be in enabled providers")
	}
}

func TestAuthSetToken_ValidatesInput(t *testing.T) {
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	t.Setenv("ANTHROPIC_API_KEY", "")
	config.Override(t, config.DefaultConfig())

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	p, _ := provider.Get("claude")

	// Claude requires sk-ant-sid01- prefix
	err := authSetToken("claude", p, "bad-key")
	if err == nil {
		t.Error("expected validation error for bad key")
	}
}

func TestAuthSetToken_RejectsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	p, _ := provider.Get("cursor")
	err := authSetToken("cursor", p, "  ")
	if err == nil {
		t.Error("expected error for empty/whitespace token")
	}
}

// JSON output tests

func TestAuthStatusJSON_UsesTypedStruct(t *testing.T) {
	tmp := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmp)
	config.Override(t, config.DefaultConfig())

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
