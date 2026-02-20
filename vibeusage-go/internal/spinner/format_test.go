package spinner

import "testing"

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		ms   int
		want string
	}{
		{"zero", 0, "0ms"},
		{"sub-second", 189, "189ms"},
		{"one second", 1000, "1.0s"},
		{"fractional seconds", 1200, "1.2s"},
		{"multi second", 3456, "3.5s"},
		{"ten seconds", 10000, "10.0s"},
		{"exact half", 1500, "1.5s"},
		{"just under one second", 999, "999ms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.ms)
			if got != tt.want {
				t.Errorf("FormatDuration(%d) = %q, want %q", tt.ms, got, tt.want)
			}
		})
	}
}

func TestFormatTitle(t *testing.T) {
	tests := []struct {
		name     string
		inflight []string
		want     string
	}{
		{"single", []string{"claude"}, "Fetching claude..."},
		{"two", []string{"claude", "copilot"}, "Fetching claude, copilot..."},
		{"three", []string{"claude", "copilot", "gemini"}, "Fetching claude, copilot, gemini..."},
		{"empty", []string{}, "Fetching..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTitle(tt.inflight)
			if got != tt.want {
				t.Errorf("FormatTitle(%v) = %q, want %q", tt.inflight, got, tt.want)
			}
		})
	}
}

func TestFormatCompletionText(t *testing.T) {
	tests := []struct {
		name string
		info CompletionInfo
		want string
	}{
		{
			"success",
			CompletionInfo{ProviderID: "copilot", Source: "device_flow", DurationMs: 189, Success: true},
			"copilot (device_flow, 189ms)",
		},
		{
			"failure",
			CompletionInfo{ProviderID: "claude", Success: false, Error: "auth failed"},
			"claude (auth failed)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCompletionText(tt.info)
			if got != tt.want {
				t.Errorf("FormatCompletionText(%+v) = %q, want %q", tt.info, got, tt.want)
			}
		})
	}
}
