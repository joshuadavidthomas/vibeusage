package prompt

import (
	"errors"
	"testing"
)

func TestValidateNotEmpty(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"non-empty string passes", "hello", false},
		{"empty string fails", "", true},
		{"whitespace-only fails", "   ", true},
		{"tab-only fails", "\t", true},
		{"value with spaces passes", "  hello  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNotEmpty(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNotEmpty(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestMockPrompter_Input(t *testing.T) {
	m := &Mock{
		InputFunc: func(cfg InputConfig) (string, error) {
			return "test-value", nil
		},
	}

	result, err := m.Input(InputConfig{Title: "Enter something"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "test-value" {
		t.Errorf("got %q, want %q", result, "test-value")
	}
}

func TestMockPrompter_InputError(t *testing.T) {
	m := &Mock{
		InputFunc: func(cfg InputConfig) (string, error) {
			return "", errors.New("user cancelled")
		},
	}

	_, err := m.Input(InputConfig{Title: "Enter something"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestMockPrompter_Confirm(t *testing.T) {
	m := &Mock{
		ConfirmFunc: func(cfg ConfirmConfig) (bool, error) {
			return true, nil
		},
	}

	result, err := m.Confirm(ConfirmConfig{Title: "Are you sure?"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("got false, want true")
	}
}

func TestMockPrompter_MultiSelect(t *testing.T) {
	m := &Mock{
		MultiSelectFunc: func(cfg MultiSelectConfig) ([]string, error) {
			return []string{"claude", "gemini"}, nil
		},
	}

	result, err := m.MultiSelect(MultiSelectConfig{
		Title: "Choose providers",
		Options: []SelectOption{
			{Label: "Claude", Value: "claude"},
			{Label: "Gemini", Value: "gemini"},
			{Label: "Copilot", Value: "copilot"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("got %d results, want 2", len(result))
	}
	if result[0] != "claude" || result[1] != "gemini" {
		t.Errorf("got %v, want [claude gemini]", result)
	}
}

func TestDefaultPrompter_IsSet(t *testing.T) {
	if Default == nil {
		t.Fatal("Default prompter should not be nil")
	}
}

func TestSetDefault_Restores(t *testing.T) {
	original := Default

	mock := &Mock{}
	SetDefault(mock)
	if Default != mock {
		t.Fatal("SetDefault did not set the mock")
	}

	SetDefault(original)
	if Default != original {
		t.Fatal("SetDefault did not restore original")
	}
}
