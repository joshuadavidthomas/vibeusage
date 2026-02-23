package provider

import "io"

// AuthFlow describes how a provider authenticates.
// Providers return nil from Auth() if they don't support interactive auth.
type AuthFlow interface {
	// Authenticate runs the auth flow, writing output to w.
	// Returns true on success, false on user cancellation.
	Authenticate(w io.Writer, quiet bool) (bool, error)
}

// DeviceAuthFlow wraps an OAuth/device-code flow provided by the
// provider package (e.g. copilot.RunDeviceFlow, kimi.RunDeviceFlow).
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
// get the generic fallback (pointing to `vibeusage key <provider> set`).
type Authenticator interface {
	Auth() AuthFlow
}
