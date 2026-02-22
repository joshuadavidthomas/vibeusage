package spinner

import "testing"

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
