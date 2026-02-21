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

// ValidateClaudeSessionKey validates that the input looks like a Claude session key.
func ValidateClaudeSessionKey(s string) error {
	if err := ValidateNotEmpty(s); err != nil {
		return err
	}
	if !strings.HasPrefix(s, "sk-ant-sid01-") {
		return errors.New("session key should start with sk-ant-sid01-")
	}
	return nil
}
