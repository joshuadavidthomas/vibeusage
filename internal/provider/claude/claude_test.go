package claude

import (
	"testing"
)

func TestValidateSessionKey(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid prefix passes", "sk-ant-sid01-abc123", false},
		{"wrong prefix fails", "some-random-key", true},
		{"empty fails", "", true},
		{"prefix only passes", "sk-ant-sid01-", false},
		{"whitespace only fails", "   ", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSessionKey(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSessionKey(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
