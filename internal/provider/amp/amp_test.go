package amp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseDisplayBalance_FreeTier(t *testing.T) {
	result := balanceResult{DisplayText: "Free quota: $6.00 / $10.00 daily. Replenishes at $1.00/hour."}

	snapshot, err := parseDisplayBalance(result, "provider_cli")
	if err != nil {
		t.Fatalf("parseDisplayBalance() error = %v", err)
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("period count = %d, want 1", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Utilization != 40 {
		t.Errorf("utilization = %d, want 40", snapshot.Periods[0].Utilization)
	}
	if snapshot.Periods[0].ResetsAt == nil {
		t.Error("expected resetsAt estimate from replenish rate")
	}
}

func TestParseDisplayBalance_CreditsOnly(t *testing.T) {
	result := balanceResult{DisplayText: "Credits: $12.34"}

	snapshot, err := parseDisplayBalance(result, "api_key")
	if err != nil {
		t.Fatalf("parseDisplayBalance() error = %v", err)
	}
	if snapshot.Overage == nil {
		t.Fatal("expected overage info for credits")
	}
	if snapshot.Overage.Limit != 12.34 {
		t.Errorf("credit limit = %.2f, want 12.34", snapshot.Overage.Limit)
	}
}

func TestParseDisplayBalance_BonusText(t *testing.T) {
	result := balanceResult{DisplayText: "Free quota: $2.50 / $10.00 daily. Bonus credits: $8.00"}

	snapshot, err := parseDisplayBalance(result, "provider_cli")
	if err != nil {
		t.Fatalf("parseDisplayBalance() error = %v", err)
	}
	if snapshot.Periods[0].Utilization != 75 {
		t.Errorf("utilization = %d, want 75", snapshot.Periods[0].Utilization)
	}
}

func TestParseDisplayBalance_MalformedText(t *testing.T) {
	_, err := parseDisplayBalance(balanceResult{DisplayText: "nonsense"}, "provider_cli")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestFetchBalance_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"unauthorized"}}`))
	}))
	defer srv.Close()

	oldURL := internalRPCURL
	internalRPCURL = srv.URL
	defer func() { internalRPCURL = oldURL }()

	result, err := fetchBalance(context.Background(), "amp-token", "provider_cli", 5)
	if err != nil {
		t.Fatalf("fetchBalance() err = %v", err)
	}
	if result.ShouldFallback {
		t.Fatal("auth failure should be fatal")
	}
	if !strings.Contains(strings.ToLower(result.Error), "amp") {
		t.Errorf("error = %q, want amp auth hint", result.Error)
	}
}
