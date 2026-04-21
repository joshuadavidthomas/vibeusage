package claude

import (
	"net/http"
	"strconv"
	"time"
)

// defaultRetryAfter is used when a 429 response has no Retry-After header.
// Matches the freshSnapshotReuseTTL cadence of ~60s; conservative enough to
// be a real cooldown while not punishing transient blips.
const defaultRetryAfter = 60 * time.Second

// parseRetryAfter returns the absolute time the client should wait until
// before retrying. RFC 7231 allows Retry-After as either delta-seconds
// (integer) or an HTTP-date; we accept both, falling back to now+default
// when the header is absent or malformed.
func parseRetryAfter(h http.Header, now time.Time) time.Time {
	v := h.Get("Retry-After")
	if v == "" {
		return now.Add(defaultRetryAfter)
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return now.Add(time.Duration(secs) * time.Second)
	}
	if t, err := http.ParseTime(v); err == nil {
		return t
	}
	return now.Add(defaultRetryAfter)
}
