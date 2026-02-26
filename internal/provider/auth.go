package provider

import (
	"errors"
	"fmt"
	"io"
	"strings"
)

// AuthFlow describes how a provider authenticates.
// Providers return nil from Auth() if they don't support interactive auth.
type AuthFlow interface {
	// Authenticate runs the auth flow, writing output to w.
	// Returns true on success, false on user cancellation.
	Authenticate(w io.Writer, quiet bool) (bool, error)
}

// DeviceAuthFlow wraps an OAuth/device-code flow provided by the
// provider package (e.g. copilot.RunDeviceFlow, kimicode.RunDeviceFlow).
type DeviceAuthFlow struct {
	RunFlow func(w io.Writer, quiet bool) (bool, error)
}

// Authenticate delegates to the provider's flow function.
func (d DeviceAuthFlow) Authenticate(w io.Writer, quiet bool) (bool, error) {
	return d.RunFlow(w, quiet)
}

// ManualKeyAuthFlow describes an auth flow where the user manually
// provides a credential (session key, API key, etc.).
type ManualKeyAuthFlow struct {
	// Instructions is the text shown to the user explaining how to get the key.
	Instructions string
	// Placeholder is shown in the input prompt (e.g. "sk-ant-sid01-...").
	Placeholder string
	// Validate checks the user's input before saving.
	Validate func(string) error
	// CredPath is the credential file path suffix (e.g. "claude/session").
	CredPath string
	// JSONKey is the key name used in the JSON credential file (e.g. "session_key").
	JSONKey string
	// Save optionally overrides how credentials are persisted. If nil, the CLI
	// writes {JSONKey: value} to CredPath.
	Save func(value string) error
}

// Authenticate is not directly called — the cmd layer uses the fields
// to build the interactive prompt. This satisfies the interface for type safety.
func (m ManualKeyAuthFlow) Authenticate(w io.Writer, quiet bool) (bool, error) {
	// Manual key flows are driven by the cmd layer using the fields above.
	// This method exists to satisfy the AuthFlow interface.
	return false, nil
}

// Authenticator is an optional interface that providers can implement
// to declare their auth flow. Providers that don't implement this
// get a generic credential prompt fallback.
type Authenticator interface {
	Auth() AuthFlow
}

// ValidateNotEmpty returns an error if the string is empty or whitespace-only.
// Use this as the Validate field in ManualKeyAuthFlow for providers that need
// only basic non-empty checking.
func ValidateNotEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("value cannot be empty")
	}
	return nil
}

// ValidatePrefix returns a validator that rejects empty values and values that
// don't start with the given prefix after trimming whitespace. Use this as the
// Validate field in ManualKeyAuthFlow for providers whose keys have a known
// format (e.g. "sk-ant-sid01-", "sk-cp-").
func ValidatePrefix(prefix string) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return errors.New("value cannot be empty")
		}
		if !strings.HasPrefix(s, prefix) {
			return fmt.Errorf("must start with %s", prefix)
		}
		return nil
	}
}

// ValidateAnyPrefix returns a validator that rejects empty values and accepts
// values that start with any one of the provided prefixes.
func ValidateAnyPrefix(prefixes ...string) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return errors.New("value cannot be empty")
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(s, prefix) {
				return nil
			}
		}
		if len(prefixes) == 0 {
			return errors.New("no valid prefixes configured")
		}
		return fmt.Errorf("must start with one of: %s", strings.Join(prefixes, ", "))
	}
}
