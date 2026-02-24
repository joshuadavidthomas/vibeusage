package provider

import "testing"

func TestValidatePrefix(t *testing.T) {
	validate := ValidatePrefix("sk-ant-sid01-")

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid key passes", "sk-ant-sid01-abc123", false},
		{"prefix only passes", "sk-ant-sid01-", false},
		{"wrong prefix fails", "some-random-key", true},
		{"empty fails", "", true},
		{"whitespace only fails", "   ", true},
		{"whitespace around valid key passes", "  sk-ant-sid01-abc  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePrefix(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateNotEmpty(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"non-empty string passes", "hello", false},
		{"empty string fails", "", true},
		{"whitespace-only fails", "   ", true},
		{"tab-only fails", "\t", true},
		{"value with spaces passes", "  hello  ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateNotEmpty(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNotEmpty(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}
