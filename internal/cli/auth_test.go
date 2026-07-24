package cli

import (
	"bytes"
	"encoding/json"
	"errors"
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

	// Verify credential was saved in consolidated store
	data, _ := config.ReadCredential("claude", "session")
	if data == nil {
		t.Error("expected credential to be saved")
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

	_ = config.WriteCredential("copilot", "oauth", []byte(`{"access_token":"test"}`))

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

func TestAuthSetup_DisablesAndReenablesExternalCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	t.Setenv("HOME", tmpDir)
	for _, envVar := range []string{
		"ANTIGRAVITY_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY",
		"GEMINI_API_KEY", "GITHUB_TOKEN", "CURSOR_API_KEY",
		"KIMI_CODE_API_KEY", "MINIMAX_API_KEY", "ZAI_API_KEY",
	} {
		t.Setenv(envVar, "")
	}
	config.Override(t, config.DefaultConfig())

	credentialPath := filepath.Join(tmpDir, ".gemini", "oauth_creds.json")
	if err := os.MkdirAll(filepath.Dir(credentialPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(credentialPath, []byte(`{"access_token":"external"}`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	call := 0
	mock := &prompt.Mock{
		MultiSelectFunc: func(cfg prompt.MultiSelectConfig) ([]string, error) {
			call++
			var geminiSelected bool
			for _, option := range cfg.Options {
				if option.Value == "gemini" {
					geminiSelected = option.Selected
					break
				}
			}
			if call == 1 {
				if !geminiSelected {
					t.Error("externally detected Gemini should start selected")
				}
				if cfg.Validate != nil {
					t.Error("provider selection should allow removing the last provider")
				}
				return nil, nil
			}
			if geminiSelected {
				t.Error("disabled Gemini should start unselected")
			}
			return []string{"gemini"}, nil
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	if err := authSetup(); err != nil {
		t.Fatalf("authSetup remove error: %v", err)
	}
	if config.Get().IsProviderEnabled("gemini") {
		t.Fatal("Gemini should be disabled after deselection")
	}
	if _, err := os.Stat(credentialPath); err != nil {
		t.Fatalf("external credential should remain untouched: %v", err)
	}
	if hasCreds, _ := provider.CheckCredentials("gemini"); !hasCreds {
		t.Fatal("external Gemini credential should still be detectable")
	}
	if ids := provider.AvailableIDs(config.Get()); slicesContain(ids, "gemini") {
		t.Fatalf("disabled Gemini should not be available, got %v", ids)
	}

	buf.Reset()
	if err := authSetup(); err != nil {
		t.Fatalf("authSetup re-enable error: %v", err)
	}
	if !config.Get().IsProviderEnabled("gemini") {
		t.Fatal("Gemini should be enabled after reselection")
	}
	if ids := provider.AvailableIDs(config.Get()); !slicesContain(ids, "gemini") {
		t.Fatalf("re-enabled Gemini should be available, got %v", ids)
	}
	if len(mock.InputCalls) != 0 || len(mock.ConfirmCalls) != 0 {
		t.Fatal("re-enabling detected external credentials should not run auth")
	}
}

func TestAuthSetup_FailedAuthKeepsProviderDisabled(t *testing.T) {
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	t.Setenv("HOME", tmpDir)
	t.Setenv("CURSOR_API_KEY", "")

	cfg := config.DefaultConfig()
	disabled := false
	cfg.Providers["cursor"] = config.ProviderConfig{Enabled: &disabled}
	config.Override(t, cfg)

	mock := &prompt.Mock{
		MultiSelectFunc: func(prompt.MultiSelectConfig) ([]string, error) {
			return []string{"cursor"}, nil
		},
		InputFunc: func(prompt.InputConfig) (string, error) {
			return "", errors.New("test auth failure")
		},
	}
	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	if err := authSetup(); err != nil {
		t.Fatalf("authSetup error: %v", err)
	}
	if config.Get().IsProviderEnabled("cursor") {
		t.Fatal("failed auth should leave Cursor disabled")
	}
	if len(mock.InputCalls) != 1 {
		t.Fatalf("expected one auth attempt, got %d", len(mock.InputCalls))
	}
}

func slicesContain(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// --delete flag tests

func TestAuthDelete_RemovesCredentialsAndDisables(t *testing.T) {
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	// Create credentials via the consolidated store
	_ = config.WriteCredential("claude", "session", []byte(`{"session_key":"test"}`))

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
	data, _ := config.ReadCredential("claude", "session")
	if data != nil {
		t.Error("credential should have been deleted")
	}
	if config.Get().IsProviderEnabled("claude") {
		t.Error("provider should have been disabled")
	}
	persisted, loadErr := config.Load("")
	if loadErr != nil {
		t.Fatalf("Load config: %v", loadErr)
	}
	if persisted.IsProviderEnabled("claude") {
		t.Error("provider disablement should be persisted")
	}

	if len(mock.ConfirmCalls) != 1 {
		t.Fatalf("expected 1 Confirm call, got %d", len(mock.ConfirmCalls))
	}
}

func TestAuthDelete_UserDeclinesConfirm(t *testing.T) {
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	_ = config.WriteCredential("claude", "session", []byte(`{"session_key":"test"}`))

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
	data, _ := config.ReadCredential("claude", "session")
	if data == nil {
		t.Error("credential should not have been deleted")
	}
}

// --token flag tests

func TestAuthSetToken_SavesCredentialAndEnables(t *testing.T) {
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	t.Setenv("CURSOR_API_KEY", "")
	cfg := config.DefaultConfig()
	disabled := false
	cfg.Providers["cursor"] = config.ProviderConfig{Enabled: &disabled}
	config.Override(t, cfg)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	p, _ := provider.Get("cursor")
	err := authSetToken("cursor", p, "my-session-token")
	if err != nil {
		t.Fatalf("authSetToken error: %v", err)
	}

	// Credential should be saved in consolidated file
	data, readErr := config.ReadCredential("cursor", "session")
	if readErr != nil {
		t.Fatalf("ReadCredential error: %v", readErr)
	}
	if data == nil {
		t.Error("expected credential to be saved")
	}

	// Provider should be configured
	hasCreds, _ := provider.CheckCredentials("cursor")
	if !hasCreds {
		t.Error("cursor should have credentials after auth")
	}
	if !config.Get().IsProviderEnabled("cursor") {
		t.Error("cursor should be enabled after auth")
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

func TestAuthSetToken_RejectsCustomAuthFlowProvider(t *testing.T) {
	// Codex now uses a CustomAuthFlow that defers to the Codex CLI; --token
	// has no usable destination and previously fell through to a generic
	// apikey slot that fetch ignored. authSetToken must refuse instead.
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	p, _ := provider.Get("codex")
	err := authSetToken("codex", p, "some-token")
	if err == nil {
		t.Fatal("expected --token to be rejected for codex (CustomAuthFlow)")
	}
	if config.HasCredential("codex", "apikey") {
		t.Error("codex/apikey must not be written for a CustomAuthFlow provider")
	}
	if config.HasCredential("codex", "oauth") {
		t.Error("codex/oauth must not be written by --token")
	}
}

func TestAuthSetToken_AcceptsKimiCodeViaTokenAcceptor(t *testing.T) {
	// KimiCode advertises a DeviceAuthFlow but also supports a stored API
	// key via APIKeyStrategy; it implements provider.TokenAcceptor so
	// --token routes to the apikey slot instead of being rejected.
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	t.Setenv("KIMI_CODE_API_KEY", "")
	config.Override(t, config.DefaultConfig())

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	p, _ := provider.Get("kimicode")
	if err := authSetToken("kimicode", p, "my-api-key"); err != nil {
		t.Fatalf("authSetToken error: %v", err)
	}
	data, err := config.ReadCredential("kimicode", "apikey")
	if err != nil || data == nil {
		t.Fatalf("kimicode/apikey not written: data=%q err=%v", data, err)
	}
	var payload map[string]string
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if payload["api_key"] != "my-api-key" {
		t.Errorf("api_key = %q, want my-api-key", payload["api_key"])
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
