package claude

import (
	"net/http"
	"testing"
	"time"
)

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		header string
		want   time.Time
	}{
		{
			name:   "delta-seconds",
			header: "30",
			want:   now.Add(30 * time.Second),
		},
		{
			name:   "zero delta-seconds",
			header: "0",
			want:   now,
		},
		{
			name:   "http-date",
			header: "Tue, 21 Apr 2026 12:05:00 GMT",
			want:   time.Date(2026, 4, 21, 12, 5, 0, 0, time.UTC),
		},
		{
			name:   "missing header uses default",
			header: "",
			want:   now.Add(defaultRetryAfter),
		},
		{
			name:   "malformed header uses default",
			header: "not-a-duration",
			want:   now.Add(defaultRetryAfter),
		},
		{
			name:   "negative delta-seconds is malformed, uses default",
			header: "-5",
			want:   now.Add(defaultRetryAfter),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := http.Header{}
			if tt.header != "" {
				h.Set("Retry-After", tt.header)
			}
			got := parseRetryAfter(h, now)
			if !got.Equal(tt.want) {
				t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.header, got, tt.want)
			}
		})
	}
}
