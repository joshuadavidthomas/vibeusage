package models

import (
	"strings"
	"time"
)

// ParseRFC3339Ptr parses an RFC 3339 timestamp and returns a pointer to the
// resulting time. Returns nil if the input is empty, whitespace-only, or
// not a valid RFC 3339 string.
func ParseRFC3339Ptr(raw string) *time.Time {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	return &t
}

// ClampPct clamps an integer percentage to the range [0, 100].
func ClampPct(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
