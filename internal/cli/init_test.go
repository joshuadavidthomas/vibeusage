package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/display"
	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
	"github.com/joshuadavidthomas/vibeusage/internal/testenv"
)

func TestInteractiveWizard_UsesMultiSelect(t *testing.T) {
	// Use temp dir so credential writes and config saves don't affect the host.
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	mock := &prompt.Mock{
		MultiSelectFunc: func(cfg prompt.MultiSelectConfig) ([]string, error) {
			if cfg.Title == "" {
				t.Error("MultiSelect title should not be empty")
			}
			if len(cfg.Options) == 0 {
				t.Error("MultiSelect should have provider options")
			}
			// Select providers that use manual key auth so the wizard
			// can drive them inline via the Input prompt.
			return []string{"gemini", "openrouter"}, nil
		},
		InputFunc: func(cfg prompt.InputConfig) (string, error) {
			// Return a dummy credential for each provider auth prompt.
			return "test-key-value", nil
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	// Capture output
	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	// Run wizard (not the cobra command, the extracted function)
	err := interactiveWizard()
	if err != nil {
		t.Fatalf("interactiveWizard() error: %v", err)
	}

	if len(mock.MultiSelectCalls) != 1 {
		t.Fatalf("expected 1 MultiSelect call, got %d", len(mock.MultiSelectCalls))
	}

	// Should have options for all known providers
	opts := mock.MultiSelectCalls[0].Options
	if len(opts) < 3 {
		t.Errorf("expected at least 3 provider options, got %d", len(opts))
	}

	// Auth should have been called inline for each selected provider.
	if len(mock.InputCalls) != 2 {
		t.Errorf("expected 2 Input calls (one per selected provider), got %d", len(mock.InputCalls))
	}

	// Summary should report success.
	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("Authenticated")) {
		t.Errorf("expected success summary in output, got:\n%s", output)
	}
}

func TestInteractiveWizard_NoSelection(t *testing.T) {
	mock := &prompt.Mock{
		MultiSelectFunc: func(cfg prompt.MultiSelectConfig) ([]string, error) {
			return nil, nil // user selected nothing
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	err := interactiveWizard()
	if err != nil {
		t.Fatalf("interactiveWizard() error: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("vibeusage init")) {
		t.Error("expected fallback instructions when no providers selected")
	}
}

func TestQuickSetup_NoPrompt(t *testing.T) {
	mock := &prompt.Mock{}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	err := quickSetup()
	if err != nil {
		t.Fatalf("quickSetup() error: %v", err)
	}

	// quickSetup should not call any prompts
	if len(mock.InputCalls) != 0 || len(mock.ConfirmCalls) != 0 || len(mock.MultiSelectCalls) != 0 {
		t.Error("quickSetup should not use any prompts")
	}
}

// Flag behavior tests

func TestInitWizard_QuietSkipsPrompt(t *testing.T) {
	mock := &prompt.Mock{}
	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	err := interactiveWizard()
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(mock.MultiSelectCalls) != 0 {
		t.Error("quiet mode should skip MultiSelect prompt")
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("vibeusage auth")) {
		t.Error("quiet mode should show fallback instructions")
	}
}

func TestQuickSetup_QuietMode(t *testing.T) {
	mock := &prompt.Mock{}
	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	err := quickSetup()
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(mock.InputCalls)+len(mock.ConfirmCalls)+len(mock.MultiSelectCalls) != 0 {
		t.Error("quiet mode should not invoke any prompts")
	}
}

func TestInitCommand_QuickFlag_DoesNotPanicWhenConfigMissing(t *testing.T) {
	tmp := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmp)
	config.Override(t, config.DefaultConfig())

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldJSON := jsonOutput
	jsonOutput = false
	defer func() { jsonOutput = oldJSON }()

	rootCmd.SetArgs([]string{"init", "--quick"})
	defer rootCmd.SetArgs(nil)

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("init --quick should execute without panic or error: %v", err)
	}
}

func TestInteractiveWizard_SavesEnabledProviders(t *testing.T) {
	tmpDir := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmpDir)
	config.Override(t, config.DefaultConfig())

	mock := &prompt.Mock{
		MultiSelectFunc: func(cfg prompt.MultiSelectConfig) ([]string, error) {
			return []string{"gemini", "openrouter"}, nil
		},
		InputFunc: func(cfg prompt.InputConfig) (string, error) {
			return "test-key-value", nil
		},
	}

	old := prompt.Default
	prompt.SetDefault(mock)
	defer prompt.SetDefault(old)

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	if err := interactiveWizard(); err != nil {
		t.Fatalf("interactiveWizard() error: %v", err)
	}

	// The selected providers should now be the enabled set.
	cfg := config.Get()
	if len(cfg.EnabledProviders) != 2 {
		t.Fatalf("expected 2 enabled providers, got %d", len(cfg.EnabledProviders))
	}
	// Providers are sorted before saving.
	if cfg.EnabledProviders[0] != "gemini" || cfg.EnabledProviders[1] != "openrouter" {
		t.Errorf("expected [gemini openrouter], got %v", cfg.EnabledProviders)
	}
}

// JSON output tests

func TestInitJSON_UsesTypedStruct(t *testing.T) {
	tmp := t.TempDir()
	testenv.ApplySameDir(t.Setenv, tmp)
	config.Override(t, config.DefaultConfig())

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
