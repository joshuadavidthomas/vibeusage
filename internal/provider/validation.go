package provider

import (
	"errors"
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
