package spinner

import (
	"fmt"
	"strings"
)

// CompletionInfo describes a completed provider fetch.
type CompletionInfo struct {
	ProviderID string
	Source     string
	DurationMs int
	Success    bool
	Error      string
}

// FormatDuration formats milliseconds as a human-readable duration.
func FormatDuration(ms int) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
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
	if !info.Success {
		return fmt.Sprintf("%s (%s)", info.ProviderID, info.Error)
	}
	return fmt.Sprintf("%s (%s, %s)", info.ProviderID, info.Source, FormatDuration(info.DurationMs))
}
