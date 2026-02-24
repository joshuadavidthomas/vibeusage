package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/routing"
)

func TestDisplayRecommendation_NoBest(t *testing.T) {
	rec := routing.Recommendation{
		ModelName:   "claude-sonnet-4-5",
		Unavailable: []string{"cursor", "copilot"},
	}

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	if err := displayRecommendation(rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "No usage data available") {
		t.Errorf("expected no-data message, got:\n%s", got)
	}
	if !strings.Contains(got, "claude-sonnet-4-5") {
		t.Errorf("expected model name in output, got:\n%s", got)
	}
	if !strings.Contains(got, "cursor") || !strings.Contains(got, "copilot") {
		t.Errorf("expected unavailable providers in output, got:\n%s", got)
	}
}

func TestDisplayRecommendation_WithCandidates(t *testing.T) {
	best := &routing.Candidate{
		ProviderID:        "claude",
		Utilization:       40,
		Headroom:          60,
		EffectiveHeadroom: 60,
		PeriodType:        models.PeriodMonthly,
	}
	rec := routing.Recommendation{
		ModelID:    "claude-sonnet-4-5",
		ModelName:  "Claude Sonnet 4.5",
		Best:       best,
		Candidates: []routing.Candidate{*best},
	}

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldNoColor := noColor
	noColor = true
	defer func() { noColor = oldNoColor }()

	if err := displayRecommendation(rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "Route: Claude Sonnet 4.5") {
		t.Errorf("expected route header, got:\n%s", got)
	}
	// Table headers may be truncated at narrow terminal widths; check prefix.
	for _, header := range []string{"Provid", "Usage", "Head"} {
		if !strings.Contains(got, header) {
			t.Errorf("expected table header prefix %q in output, got:\n%s", header, got)
		}
	}
}

func TestDisplayRoleRecommendation_NoBest(t *testing.T) {
	rec := routing.RoleRecommendation{
		Role: "thinking",
		Unavailable: []routing.RoleUnavailable{
			{ModelID: "claude-opus-4", ProviderID: "claude"},
		},
	}

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	if err := displayRoleRecommendation(rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "No usage data available") {
		t.Errorf("expected no-data message, got:\n%s", got)
	}
	if !strings.Contains(got, "thinking") {
		t.Errorf("expected role name in output, got:\n%s", got)
	}
	if !strings.Contains(got, "claude-opus-4") {
		t.Errorf("expected unavailable model in output, got:\n%s", got)
	}
}

func TestDisplayRoleRecommendation_WithCandidates(t *testing.T) {
	bestRole := &routing.RoleCandidate{
		Candidate: routing.Candidate{
			ProviderID:        "cursor",
			Utilization:       30,
			Headroom:          70,
			EffectiveHeadroom: 70,
			PeriodType:        models.PeriodMonthly,
		},
		ModelID:   "claude-sonnet-4-5",
		ModelName: "Claude Sonnet 4.5",
	}
	rec := routing.RoleRecommendation{
		Role:       "thinking",
		Best:       bestRole,
		Candidates: []routing.RoleCandidate{*bestRole},
	}

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldNoColor := noColor
	noColor = true
	defer func() { noColor = oldNoColor }()

	if err := displayRoleRecommendation(rec); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "Route: thinking (role)") {
		t.Errorf("expected role route header, got:\n%s", got)
	}
	// Table headers may be truncated at narrow terminal widths; check prefix.
	for _, header := range []string{"Model", "Usage", "Head"} {
		if !strings.Contains(got, header) {
			t.Errorf("expected table header prefix %q in output, got:\n%s", header, got)
		}
	}
}

func TestRenderRouteTable_RendersHeaders(t *testing.T) {
	ft := routing.FormattedTable{
		Headers: []string{"Provider", "Usage", "Headroom"},
		Rows:    [][]string{{"Claude", "40%", "60%"}},
		Styles:  []routing.RowStyle{routing.RowBold},
	}

	var buf bytes.Buffer
	outWriter = &buf
	defer func() { outWriter = os.Stdout }()

	oldNoColor := noColor
	noColor = true
	defer func() { noColor = oldNoColor }()

	renderRouteTable(ft)

	got := buf.String()
	for _, header := range ft.Headers {
		if !strings.Contains(got, header) {
			t.Errorf("expected header %q in table output, got:\n%s", header, got)
		}
	}
	if !strings.Contains(got, "Claude") {
		t.Errorf("expected row data in table output, got:\n%s", got)
	}
}
