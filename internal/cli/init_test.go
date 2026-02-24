package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
)

func TestInteractiveWizard_UsesMultiSelect(t *testing.T) {
	mock := &prompt.Mock{
		MultiSelectFunc: func(cfg prompt.MultiSelectConfig) ([]string, error) {
			if cfg.Title == "" {
				t.Error("MultiSelect title should not be empty")
			}
			if len(cfg.Options) == 0 {
				t.Error("MultiSelect should have provider options")
			}
			return []string{"claude", "gemini"}, nil
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
	if !bytes.Contains([]byte(output), []byte("vibeusage auth")) {
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
