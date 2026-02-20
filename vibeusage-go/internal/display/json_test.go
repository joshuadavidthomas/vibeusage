package display

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestOutputJSON_WritesToWriter(t *testing.T) {
	var buf bytes.Buffer
	OutputJSON(&buf, map[string]string{"key": "value"})

	output := buf.String()
	if !strings.Contains(output, `"key"`) {
		t.Errorf("expected key in output, got: %s", output)
	}
	if !strings.Contains(output, `"value"`) {
		t.Errorf("expected value in output, got: %s", output)
	}
}

func TestOutputJSON_PrettyPrints(t *testing.T) {
	var buf bytes.Buffer
	OutputJSON(&buf, map[string]string{"a": "1"})

	output := buf.String()
	if !strings.Contains(output, "  ") {
		t.Errorf("expected indented output, got: %s", output)
	}
}

func TestOutputStatusJSON_WritesToWriter(t *testing.T) {
	var buf bytes.Buffer
	statuses := map[string]models.ProviderStatus{
		"claude": {
			Level:       models.StatusOperational,
			Description: "All systems normal",
		},
	}

	OutputStatusJSON(&buf, statuses)

	output := buf.String()
	if !strings.Contains(output, "claude") {
		t.Errorf("expected provider in output, got: %s", output)
	}
	if !strings.Contains(output, "operational") {
		t.Errorf("expected level in output, got: %s", output)
	}
}

func TestOutputMultiProviderJSON_WritesToWriter(t *testing.T) {
	var buf bytes.Buffer

	now := time.Now()
	outcomes := map[string]fetch.FetchOutcome{
		"claude": {
			ProviderID: "claude",
			Success:    true,
			Snapshot: &models.UsageSnapshot{
				Provider:  "claude",
				FetchedAt: now,
			},
		},
	}

	OutputMultiProviderJSON(&buf, outcomes)

	output := buf.String()
	if !strings.Contains(output, "claude") {
		t.Errorf("expected provider in output, got: %s", output)
	}
	if !strings.Contains(output, "providers") {
		t.Errorf("expected providers key in output, got: %s", output)
	}
}

func TestOutputMultiProviderJSON_IncludesErrors(t *testing.T) {
	var buf bytes.Buffer

	outcomes := map[string]fetch.FetchOutcome{
		"cursor": {
			ProviderID: "cursor",
			Success:    false,
			Error:      "auth failed",
		},
	}

	OutputMultiProviderJSON(&buf, outcomes)

	output := buf.String()
	if !strings.Contains(output, "errors") {
		t.Errorf("expected errors key in output, got: %s", output)
	}
	if !strings.Contains(output, "auth failed") {
		t.Errorf("expected error message in output, got: %s", output)
	}
}
