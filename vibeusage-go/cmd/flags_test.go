package cmd

import (
	"bytes"
	"os"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/prompt"
)

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
