package openrouter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseCreditsSnapshot_Success(t *testing.T) {
	resp := CreditsResponse{
		Data: CreditsData{TotalCredits: 100, TotalUsage: 25},
	}

	snapshot, err := parseCreditsSnapshot(resp)
	if err != nil {
		t.Fatalf("parseCreditsSnapshot() error = %v", err)
	}
	if snapshot.Provider != "openrouter" {
		t.Errorf("provider = %q, want %q", snapshot.Provider, "openrouter")
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("period count = %d, want 1", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Utilization != 25 {
		t.Errorf("utilization = %d, want 25", snapshot.Periods[0].Utilization)
	}
	if snapshot.Overage == nil {
		t.Fatal("expected overage")
	}
	if snapshot.Overage.Used != 25 || snapshot.Overage.Limit != 100 {
		t.Errorf("overage = %+v, want used=25 limit=100", snapshot.Overage)
	}
}

func TestParseCreditsSnapshot_ZeroCredits(t *testing.T) {
	resp := CreditsResponse{Data: CreditsData{TotalCredits: 0, TotalUsage: 10}}

	snapshot, err := parseCreditsSnapshot(resp)
	if err != nil {
		t.Fatalf("parseCreditsSnapshot() error = %v", err)
	}
	if snapshot.Periods[0].Utilization != 0 {
		t.Errorf("utilization = %d, want 0", snapshot.Periods[0].Utilization)
	}
}

func TestFetchCredits_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	oldURL := creditsURL
	creditsURL = srv.URL
	defer func() { creditsURL = oldURL }()

	result, err := fetchCredits(context.Background(), "or-test", 5)
	if err != nil {
		t.Fatalf("fetchCredits() err = %v", err)
	}
	if result.ShouldFallback {
		t.Fatal("auth failure should be fatal")
	}
	if !strings.Contains(strings.ToLower(result.Error), "openrouter") {
		t.Errorf("error = %q, want auth hint mentioning openrouter", result.Error)
	}
}

func TestFetchCredits_InvalidJSONIncludesUnderlyingError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":`))
	}))
	defer srv.Close()

	oldURL := creditsURL
	creditsURL = srv.URL
	defer func() { creditsURL = oldURL }()

	result, err := fetchCredits(context.Background(), "or-test", 5)
	if err != nil {
		t.Fatalf("fetchCredits() err = %v", err)
	}
	if result.Success {
		t.Fatal("expected parse failure")
	}
	if !strings.Contains(strings.ToLower(result.Error), "invalid") {
		t.Errorf("error = %q, want invalid response message", result.Error)
	}
}
