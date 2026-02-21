package cmd

import (
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/spinner"
)

func TestOutcomeToCompletion_Success(t *testing.T) {
	o := fetch.FetchOutcome{
		ProviderID: "claude",
		Success:    true,
		Source:     "oauth",
		Attempts: []fetch.FetchAttempt{
			{Strategy: "oauth", Success: true, DurationMs: 342},
		},
	}

	got := outcomeToCompletion(o)

	want := spinner.CompletionInfo{
		ProviderID: "claude",
		Source:     "oauth",
		DurationMs: 342,
		Success:    true,
	}

	if got != want {
		t.Errorf("outcomeToCompletion() = %+v, want %+v", got, want)
	}
}

func TestOutcomeToCompletion_Failure(t *testing.T) {
	o := fetch.FetchOutcome{
		ProviderID: "cursor",
		Success:    false,
		Error:      "auth failed",
		Attempts: []fetch.FetchAttempt{
			{Strategy: "api_key", Success: false, DurationMs: 50, Error: "not available"},
			{Strategy: "web", Success: false, DurationMs: 100, Error: "auth failed"},
		},
	}

	got := outcomeToCompletion(o)

	want := spinner.CompletionInfo{
		ProviderID: "cursor",
		DurationMs: 150, // sum of all attempts
		Success:    false,
		Error:      "auth failed",
	}

	if got != want {
		t.Errorf("outcomeToCompletion() = %+v, want %+v", got, want)
	}
}

func TestOutcomeToCompletion_Fallback(t *testing.T) {
	o := fetch.FetchOutcome{
		ProviderID: "gemini",
		Success:    true,
		Source:     "code_assist",
		Attempts: []fetch.FetchAttempt{
			{Strategy: "oauth", Success: false, DurationMs: 200, Error: "token expired"},
			{Strategy: "code_assist", Success: true, DurationMs: 800},
		},
	}

	got := outcomeToCompletion(o)

	if got.DurationMs != 1000 {
		t.Errorf("expected total duration 1000ms, got %d", got.DurationMs)
	}
	if got.Source != "code_assist" {
		t.Errorf("expected source code_assist, got %s", got.Source)
	}
	if !got.Success {
		t.Error("expected success=true")
	}
}
