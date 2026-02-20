package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestDisplayStatusTable_ContainsProviderData(t *testing.T) {
	now := time.Now()
	statuses := map[string]models.ProviderStatus{
		"claude": {
			Level:       models.StatusOperational,
			Description: "All systems normal",
			UpdatedAt:   &now,
		},
		"cursor": {
			Level:       models.StatusDegraded,
			Description: "Slow responses",
			UpdatedAt:   &now,
		},
	}

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldNoColor := noColor
	noColor = true
	defer func() { noColor = oldNoColor }()

	displayStatusTable(statuses, 100)

	output := buf.String()

	for _, want := range []string{"claude", "cursor", "All systems normal", "Slow responses"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q\n\nGot:\n%s", want, output)
		}
	}
}

func TestDisplayStatusTable_HasTableBorders(t *testing.T) {
	statuses := map[string]models.ProviderStatus{
		"claude": {
			Level:       models.StatusOperational,
			Description: "OK",
		},
	}

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldNoColor := noColor
	noColor = false
	defer func() { noColor = oldNoColor }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	displayStatusTable(statuses, 0)

	output := buf.String()

	// Should use lipgloss table borders (rounded)
	if !strings.Contains(output, "╭") {
		t.Errorf("expected lipgloss rounded border, got:\n%s", output)
	}
}

func TestDisplayStatusTable_QuietMode(t *testing.T) {
	now := time.Now()
	statuses := map[string]models.ProviderStatus{
		"claude": {
			Level:       models.StatusOperational,
			Description: "OK",
			UpdatedAt:   &now,
		},
	}

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldQuiet := quiet
	quiet = true
	defer func() { quiet = oldQuiet }()

	displayStatusTable(statuses, 0)

	output := buf.String()

	// Quiet mode should not use table borders
	if strings.Contains(output, "╭") {
		t.Error("quiet mode should not use table borders")
	}
	if !strings.Contains(output, "claude") {
		t.Error("quiet mode should still show provider names")
	}
}

func TestDisplayStatusTable_VerboseShowsDuration(t *testing.T) {
	statuses := map[string]models.ProviderStatus{
		"claude": {Level: models.StatusOperational, Description: "OK"},
	}

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	oldVerbose := verbose
	verbose = true
	defer func() { verbose = oldVerbose }()

	displayStatusTable(statuses, 250)

	output := buf.String()
	if !strings.Contains(output, "250ms") {
		t.Errorf("verbose mode should show duration, got:\n%s", output)
	}
}

func TestDisplayStatusTable_Headers(t *testing.T) {
	statuses := map[string]models.ProviderStatus{
		"claude": {Level: models.StatusOperational, Description: "OK"},
	}

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldNoColor := noColor
	noColor = true
	defer func() { noColor = oldNoColor }()

	oldQuiet := quiet
	quiet = false
	defer func() { quiet = oldQuiet }()

	displayStatusTable(statuses, 0)

	output := buf.String()
	for _, header := range []string{"Provider", "Status", "Description", "Updated"} {
		if !strings.Contains(output, header) {
			t.Errorf("output missing header %q\n\nGot:\n%s", header, output)
		}
	}
}
