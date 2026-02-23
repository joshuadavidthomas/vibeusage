package provider

import (
	"bytes"
	"io"
	"testing"
)

// Verify the AuthFlow interface is satisfied by each flow type.

func TestDeviceAuthFlow_ImplementsAuthFlow(t *testing.T) {
	var flow AuthFlow = DeviceAuthFlow{
		RunFlow: func(w io.Writer, quiet bool) (bool, error) {
			return true, nil
		},
	}
	_ = flow
}

func TestManualKeyAuthFlow_ImplementsAuthFlow(t *testing.T) {
	var flow AuthFlow = ManualKeyAuthFlow{
		Instructions: "Get your key from example.com",
		Placeholder:  "sk-...",
		Validate:     func(s string) error { return nil },
		CredPath:     "test/session",
		JSONKey:      "session_key",
	}
	_ = flow
}

func TestDeviceAuthFlow_Authenticate(t *testing.T) {
	called := false
	flow := DeviceAuthFlow{
		RunFlow: func(w io.Writer, quiet bool) (bool, error) {
			called = true
			return true, nil
		},
	}

	var buf bytes.Buffer
	success, err := flow.Authenticate(&buf, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !success {
		t.Error("expected success")
	}
	if !called {
		t.Error("expected RunFlow to be called")
	}
}

func TestManualKeyAuthFlow_FieldAccess(t *testing.T) {
	flow := ManualKeyAuthFlow{
		Instructions: "Test instructions",
		Placeholder:  "test-placeholder",
		JSONKey:      "api_key",
		CredPath:     "test/apikey",
	}

	if flow.Instructions != "Test instructions" {
		t.Errorf("Instructions = %q, want %q", flow.Instructions, "Test instructions")
	}
	if flow.Placeholder != "test-placeholder" {
		t.Errorf("Placeholder = %q, want %q", flow.Placeholder, "test-placeholder")
	}
}
