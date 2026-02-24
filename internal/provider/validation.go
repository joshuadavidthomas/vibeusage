package provider

import (
	"errors"
	"fmt"
	"strings"
)

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
