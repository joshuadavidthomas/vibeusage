package display

import (
	"strings"
	"testing"
)

func TestNewTable_RendersHeadersAndRows(t *testing.T) {
	result := NewTable(
		[]string{"Name", "Age", "City"},
		[][]string{
			{"Alice", "30", "NYC"},
			{"Bob", "25", "LA"},
		},
	)

	if result == "" {
		t.Fatal("expected non-empty table output")
	}

	for _, want := range []string{"Name", "Age", "City", "Alice", "30", "NYC", "Bob", "25", "LA"} {
		if !strings.Contains(result, want) {
			t.Errorf("table output missing %q", want)
		}
	}
}

func TestNewTable_EmptyRows(t *testing.T) {
	result := NewTable(
		[]string{"Provider", "Status"},
		nil,
	)

	// Should still render headers even with no rows
	if !strings.Contains(result, "Provider") {
		t.Error("expected headers in empty table")
	}
}

func TestNewTable_HasBorders(t *testing.T) {
	result := NewTable(
		[]string{"A", "B"},
		[][]string{{"1", "2"}},
	)

	// lipgloss/table with rounded borders uses these characters
	if !strings.Contains(result, "╭") {
		t.Error("expected rounded border top-left corner")
	}
	if !strings.Contains(result, "╰") {
		t.Error("expected rounded border bottom-left corner")
	}
}

func TestNewTable_NoColor(t *testing.T) {
	// When noColor is true, table should still render but without ANSI codes
	result := NewTableWithOptions(
		[]string{"Name", "Status"},
		[][]string{{"test", "ok"}},
		TableOptions{NoColor: true},
	)

	if result == "" {
		t.Fatal("expected non-empty table output")
	}
	if !strings.Contains(result, "Name") {
		t.Error("expected headers in no-color table")
	}
	if !strings.Contains(result, "test") {
		t.Error("expected row data in no-color table")
	}
}

func TestNewTable_Title(t *testing.T) {
	result := NewTableWithOptions(
		[]string{"Col1", "Col2"},
		[][]string{{"a", "b"}},
		TableOptions{Title: "My Table"},
	)

	if !strings.Contains(result, "My Table") {
		t.Error("expected title in output")
	}
}

func TestNewTable_NoTitle(t *testing.T) {
	result := NewTableWithOptions(
		[]string{"Col1", "Col2"},
		[][]string{{"a", "b"}},
		TableOptions{},
	)

	// Should just have the table, no extra title line
	if !strings.Contains(result, "Col1") {
		t.Error("expected headers without title")
	}
}
