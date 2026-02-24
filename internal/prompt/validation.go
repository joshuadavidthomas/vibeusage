package prompt

import (
	"errors"
	"strings"
)

// ValidateNotEmpty returns an error if the string is empty or whitespace-only.
func ValidateNotEmpty(s string) error {
	if strings.TrimSpace(s) == "" {
		return errors.New("value cannot be empty")
	}
	return nil
}
