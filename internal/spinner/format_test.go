package spinner

import "testing"

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
			CompletionInfo{ProviderID: "copilot", Success: true},
			"copilot",
		},
		{
			"failure",
			CompletionInfo{ProviderID: "claude", Success: false, Error: "auth failed"},
			"claude",
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
