package warp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseUsageSnapshot_HappyPath(t *testing.T) {
	resp := GraphQLResponse{
		Data: &GraphQLData{
			RequestLimitInfo: &RequestLimitInfo{
				RequestLimit:    1000,
				RequestsUsed:    250,
				NextRefreshTime: "2026-03-01T00:00:00Z",
			},
		},
	}

	snapshot, err := parseUsageSnapshot(resp)
	if err != nil {
		t.Fatalf("parseUsageSnapshot() error = %v", err)
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("period count = %d, want 1", len(snapshot.Periods))
	}
	if snapshot.Periods[0].Utilization != 25 {
		t.Errorf("utilization = %d, want 25", snapshot.Periods[0].Utilization)
	}
}

func TestParseUsageSnapshot_UnlimitedPlan(t *testing.T) {
	resp := GraphQLResponse{
		Data: &GraphQLData{
			RequestLimitInfo: &RequestLimitInfo{
				IsUnlimited:  true,
				RequestsUsed: 700,
			},
		},
	}

	snapshot, err := parseUsageSnapshot(resp)
	if err != nil {
		t.Fatalf("parseUsageSnapshot() error = %v", err)
	}
	if snapshot.Periods[0].Utilization != 0 {
		t.Errorf("utilization = %d, want 0 for unlimited", snapshot.Periods[0].Utilization)
	}
}

func TestParseUsageSnapshot_MissingFields(t *testing.T) {
	_, err := parseUsageSnapshot(GraphQLResponse{Data: &GraphQLData{}})
	if err == nil {
		t.Fatal("expected error for missing requestLimitInfo")
	}
}

func TestGraphQLErrorArrayParsing(t *testing.T) {
	_, err := parseUsageSnapshot(GraphQLResponse{Errors: []GraphQLError{{Message: "backend unavailable"}}})
	if err == nil {
		t.Fatal("expected graphql error")
	}
	if !strings.Contains(err.Error(), "backend unavailable") {
		t.Errorf("error = %q, want graphql message", err)
	}
}

func TestFetchUsage_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"message":"forbidden"}]}`))
	}))
	defer srv.Close()

	oldURL := requestLimitURL
	requestLimitURL = srv.URL
	defer func() { requestLimitURL = oldURL }()

	result, err := fetchUsage(context.Background(), "wk-test", 5)
	if err != nil {
		t.Fatalf("fetchUsage() err = %v", err)
	}
	if result.ShouldFallback {
		t.Fatal("auth failure should be fatal")
	}
	if !strings.Contains(strings.ToLower(result.Error), "warp") {
		t.Errorf("error = %q, want warp auth hint", result.Error)
	}
}
