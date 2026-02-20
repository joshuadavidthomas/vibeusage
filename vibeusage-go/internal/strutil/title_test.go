package strutil

import "testing"

func TestTitleCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"claude", "Claude"},
		{"copilot", "Copilot"},
		{"gemini", "Gemini"},
		{"hello world", "Hello World"},
		{"gemini 2 5 flash", "Gemini 2 5 Flash"},
		{"already Title", "Already Title"},
		{"ALL CAPS", "ALL CAPS"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := TitleCase(tt.input)
			if got != tt.want {
				t.Errorf("TitleCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
