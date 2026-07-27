package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/device"
)

// AuthFlow is a marker interface for provider auth flow types.
// Use a type switch to determine the concrete type:
//   - DeviceAuthFlow: standard OAuth device code flow (configured via device.Config)
//   - ManualKeyAuthFlow: user pastes a credential (API key, session token, etc.)
//   - CustomAuthFlow: provider-specific flow that doesn't fit the standard patterns
type AuthFlow interface {
	authFlow()
}

// DeviceAuthFlow describes an OAuth device code flow.
// The deviceflow package handles the entire lifecycle using the Config.
type DeviceAuthFlow struct {
	Config device.Config
}

func (DeviceAuthFlow) authFlow() {}

// CustomAuthFlow wraps a provider-specific auth function that doesn't fit
// the standard device code or manual key patterns (e.g. localhost OAuth redirect).
type CustomAuthFlow struct {
	RunFlow func(ctx context.Context, w io.Writer, quiet bool) (bool, error)
}

func (CustomAuthFlow) authFlow() {}

// ManualKeyAuthFlow describes an auth flow where the user manually
// provides a credential (session key, API key, etc.).
type ManualKeyAuthFlow struct {
	// Instructions is the text shown to the user explaining how to get the key.
	Instructions string
	// Placeholder is shown in the input prompt (e.g. "sk-ant-sid01-...").
	Placeholder string
	// Validate checks the user's input before saving.
	Validate func(string) error
	// ProviderID is the provider identifier for credential storage.
	ProviderID string
	// CredType is the credential type (e.g. "apikey", "session", "oauth").
	CredType string
	// JSONKey is the key name used in the JSON credential file (e.g. "session_key").
	JSONKey string
	// CookieNames marks the credential as a browser cookie value and lists the
	// cookie names users may copy it from. Cookie headers and assignments are
	// rejected so only the value is stored.
	CookieNames []string
	// Save optionally overrides how credentials are persisted. If nil, the CLI
	// writes {JSONKey: value} to the consolidated credentials file.
	Save func(value string) error
}

func (ManualKeyAuthFlow) authFlow() {}

// Authenticator is an optional interface that providers can implement
// to declare their auth flow. Providers that don't implement this
// get a generic credential prompt fallback.
type Authenticator interface {
	Auth() AuthFlow
}

// CredentialAcceptor is an optional interface for providers whose primary
// Auth() flow isn't a ManualKeyAuthFlow but still accepts a credential from
// stdin (e.g. KimiCode advertises a device flow for OAuth but also supports a
// stored API key). Providers that do not implement this interface reject
// --token so vibeusage doesn't write a credential its fetch strategies will
// silently ignore.
type CredentialAcceptor interface {
	AcceptCredential(credential string) error
}

// PrepareCredential trims surrounding whitespace and rejects empty values and
// embedded control characters before a credential reaches provider storage.
func PrepareCredential(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("credential cannot be empty")
	}
	if strings.ContainsFunc(value, unicode.IsControl) {
		return "", errors.New("credential cannot contain control characters")
	}
	return value, nil
}

// Prepare validates and normalizes a value according to the manual flow.
func (f ManualKeyAuthFlow) Prepare(value string) (string, error) {
	value, err := PrepareCredential(value)
	if err != nil {
		return "", err
	}
	if len(f.CookieNames) > 0 {
		if field, _, found := strings.Cut(value, ":"); found {
			field = strings.TrimSpace(field)
			if strings.EqualFold(field, "cookie") || strings.EqualFold(field, "set-cookie") {
				return "", errors.New("paste only the cookie value, without a Cookie or Set-Cookie header")
			}
		}
		if field, _, found := strings.Cut(value, "="); found {
			field = strings.TrimSpace(field)
			for _, name := range f.CookieNames {
				if strings.EqualFold(field, name) {
					return "", fmt.Errorf("paste only the %s cookie value, without %s=", name, name)
				}
			}
		}
		if err := (&http.Cookie{Name: "credential", Value: value}).Valid(); err != nil {
			return "", errors.New("paste only the cookie value, without a Cookie header or other cookies")
		}
	}
	if f.Validate != nil {
		if err := f.Validate(value); err != nil {
			return "", err
		}
	}
	return value, nil
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
