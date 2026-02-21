package spinner

import (
	"strings"
)

// CompletionInfo describes a completed provider fetch.
type CompletionInfo struct {
	ProviderID string
	Success    bool
	Error      string
}

// FormatTitle formats the spinner title showing in-flight providers.
func FormatTitle(inflight []string) string {
	if len(inflight) == 0 {
		return "Fetching..."
	}
	return "Fetching " + strings.Join(inflight, ", ") + "..."
}

// FormatCompletionText formats the text portion of a completion line (without symbol).
func FormatCompletionText(info CompletionInfo) string {
	return info.ProviderID
}
