package display

import (
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestStatusSymbol_NoColor_NoANSI(t *testing.T) {
	levels := []models.StatusLevel{
		models.StatusOperational,
		models.StatusDegraded,
		models.StatusPartialOutage,
		models.StatusMajorOutage,
		"unknown",
	}

	for _, level := range levels {
		result := StatusSymbol(level, true)
		if strings.Contains(result, "\x1b[") {
			t.Errorf("StatusSymbol(%q, true) should not contain ANSI codes, got: %q", level, result)
		}
		if result == "" {
			t.Errorf("StatusSymbol(%q, true) should not be empty", level)
		}
	}
}

func TestStatusSymbol_NoColor_ReturnsCorrectSymbols(t *testing.T) {
	tests := []struct {
		level models.StatusLevel
		want  string
	}{
		{models.StatusOperational, "●"},
		{models.StatusDegraded, "◐"},
		{models.StatusPartialOutage, "◑"},
		{models.StatusMajorOutage, "○"},
		{"unknown", "?"},
	}

	for _, tt := range tests {
		result := StatusSymbol(tt.level, true)
		if result != tt.want {
			t.Errorf("StatusSymbol(%q, true) = %q, want %q", tt.level, result, tt.want)
		}
	}
}

func TestStatusSymbol_WithColor_ReturnsNonEmpty(t *testing.T) {
	levels := []models.StatusLevel{
		models.StatusOperational,
		models.StatusDegraded,
		models.StatusPartialOutage,
		models.StatusMajorOutage,
	}

	for _, level := range levels {
		result := StatusSymbol(level, false)
		if result == "" {
			t.Errorf("StatusSymbol(%q, false) should not be empty", level)
		}
	}
}
