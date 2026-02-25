package httpclient

import "strings"

// SummarizeBody returns a short summary of an HTTP response body suitable for
// error messages. Empty bodies return "empty body"; bodies longer than 120
// characters are truncated with "...".
func SummarizeBody(body []byte) string {
	s := strings.TrimSpace(string(body))
	if s == "" {
		return "empty body"
	}
	if len(s) > 120 {
		return s[:120] + "..."
	}
	return s
}
