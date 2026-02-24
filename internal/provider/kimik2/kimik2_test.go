package kimik2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseCreditsSnapshot_VariantTopLevel(t *testing.T) {
	resp := CreditsResponse{Consumed: 40, Remaining: 60}

	snapshot, err := parseCreditsSnapshot(resp, nil)
	if err != nil {
		t.Fatalf("parseCreditsSnapshot() error = %v", err)
	}
	if snapshot.Periods[0].Utilization != 40 {
		t.Errorf("utilization = %d, want 40", snapshot.Periods[0].Utilization)
	}
}

func TestParseCreditsSnapshot_VariantNestedStringFields(t *testing.T) {
	resp := CreditsResponse{Data: &CreditsData{Used: "25", Balance: "75"}}

	snapshot, err := parseCreditsSnapshot(resp, nil)
	if err != nil {
		t.Fatalf("parseCreditsSnapshot() error = %v", err)
	}
	if snapshot.Periods[0].Utilization != 25 {
		t.Errorf("utilization = %d, want 25", snapshot.Periods[0].Utilization)
	}
}

func TestParseCreditsSnapshot_HeaderFallbackForRemaining(t *testing.T) {
	resp := CreditsResponse{Consumed: 30}
	headers := http.Header{"X-Remaining-Credits": []string{"70"}}

	snapshot, err := parseCreditsSnapshot(resp, headers)
	if err != nil {
		t.Fatalf("parseCreditsSnapshot() error = %v", err)
	}
	if snapshot.Periods[0].Utilization != 30 {
		t.Errorf("utilization = %d, want 30", snapshot.Periods[0].Utilization)
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

	result, err := fetchCredits(context.Background(), "k2-test", 5)
	if err != nil {
		t.Fatalf("fetchCredits() err = %v", err)
	}
	if result.ShouldFallback {
		t.Fatal("auth failure should be fatal")
	}
	if !strings.Contains(strings.ToLower(result.Error), "kimik2") {
		t.Errorf("error = %q, want kimik2 auth hint", result.Error)
	}
}
