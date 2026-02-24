package provider

import "testing"

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
