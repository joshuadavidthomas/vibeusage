package spinner

// CompletionInfo describes a completed provider fetch.
type CompletionInfo struct {
	ProviderID string
	Success    bool
	Error      string
}

// FormatCompletionText formats the text portion of a completion line (without symbol).
func FormatCompletionText(info CompletionInfo) string {
	return info.ProviderID
}
