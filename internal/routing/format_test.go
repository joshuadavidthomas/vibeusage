package routing

import "testing"

func TestFormatMultiplier(t *testing.T) {
	tests := []struct {
		name       string
		multiplier *float64
		want       string
	}{
		{"nil multiplier", nil, "—"},
		{"zero (free)", floatPtr(0), "free"},
		{"integer 1x", floatPtr(1), "1x"},
		{"integer 3x", floatPtr(3), "3x"},
		{"integer 10x", floatPtr(10), "10x"},
		{"fractional 0.33x", floatPtr(0.33), "0.33x"},
		{"fractional 1.5x", floatPtr(1.5), "1.5x"},
		{"fractional 0.5x", floatPtr(0.5), "0.5x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatMultiplier(tt.multiplier)
			if got != tt.want {
				t.Errorf("FormatMultiplier(%v) = %q, want %q", tt.multiplier, got, tt.want)
			}
		})
	}
}
