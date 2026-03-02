package warp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestParseUsageSnapshot_HappyPath(t *testing.T) {
	resp := GraphQLResponse{
		Data: &GraphQLData{
			User: &UserUnion{
				TypeName: "UserOutput",
				User: &UserFields{
					RequestLimitInfo: &RequestLimitInfo{
						RequestLimit:                 75,
						RequestsUsedSinceLastRefresh: 19,
						NextRefreshTime:              "2026-03-27T15:55:15.607496Z",
						RequestLimitRefreshDuration:  "MONTHLY",
						RequestLimitPooling:          "USER",
					},
				},
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
	if snapshot.Periods[0].PeriodType != models.PeriodMonthly {
		t.Errorf("period type = %v, want monthly", snapshot.Periods[0].PeriodType)
	}
	if snapshot.Identity == nil {
		t.Fatal("identity should not be nil")
	}
	if snapshot.Identity.Plan != "Free" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "Free")
	}
}

func TestParseUsageSnapshot_UnlimitedPlan(t *testing.T) {
	resp := GraphQLResponse{
		Data: &GraphQLData{
			User: &UserUnion{
				TypeName: "UserOutput",
				User: &UserFields{
					RequestLimitInfo: &RequestLimitInfo{
						IsUnlimited:                  true,
						RequestsUsedSinceLastRefresh: 700,
						RequestLimitPooling:          "TEAM",
					},
				},
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
	if snapshot.Identity.Plan != "Unlimited (TEAM)" {
		t.Errorf("plan = %q, want %q", snapshot.Identity.Plan, "Unlimited (TEAM)")
	}
}

func TestParseUsageSnapshot_MissingFields(t *testing.T) {
	_, err := parseUsageSnapshot(GraphQLResponse{
		Data: &GraphQLData{
			User: &UserUnion{
				TypeName: "UserOutput",
				User:     &UserFields{},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for missing requestLimitInfo")
	}
}

func TestParseUsageSnapshot_UnexpectedUserType(t *testing.T) {
	_, err := parseUsageSnapshot(GraphQLResponse{
		Data: &GraphQLData{
			User: &UserUnion{
				TypeName: "AnonymousUser",
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for unexpected user type")
	}
	if !strings.Contains(err.Error(), "AnonymousUser") {
		t.Errorf("error = %q, want mention of AnonymousUser", err)
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

func TestParseUsageSnapshot_WithBonusGrants(t *testing.T) {
	resp := GraphQLResponse{
		Data: &GraphQLData{
			User: &UserUnion{
				TypeName: "UserOutput",
				User: &UserFields{
					RequestLimitInfo: &RequestLimitInfo{
						RequestLimit:                 75,
						RequestsUsedSinceLastRefresh: 10,
						NextRefreshTime:              "2026-03-27T15:55:15.607496Z",
						RequestLimitRefreshDuration:  "MONTHLY",
						RequestLimitPooling:          "USER",
					},
					BonusGrants: []BonusGrant{
						{
							RequestCreditsGranted:   100,
							RequestCreditsRemaining: 75,
							Expiration:              "2026-04-01T00:00:00Z",
						},
					},
				},
			},
		},
	}

	snapshot, err := parseUsageSnapshot(resp)
	if err != nil {
		t.Fatalf("parseUsageSnapshot() error = %v", err)
	}
	if len(snapshot.Periods) != 2 {
		t.Fatalf("period count = %d, want 2", len(snapshot.Periods))
	}
	if snapshot.Periods[1].Name != "Bonus Credits" {
		t.Errorf("period name = %q, want %q", snapshot.Periods[1].Name, "Bonus Credits")
	}
	// 25 used of 100 total = 25%
	if snapshot.Periods[1].Utilization != 25 {
		t.Errorf("bonus utilization = %d, want 25", snapshot.Periods[1].Utilization)
	}
}

func TestParseUsageSnapshot_CombinedBonusGrants(t *testing.T) {
	resp := GraphQLResponse{
		Data: &GraphQLData{
			User: &UserUnion{
				TypeName: "UserOutput",
				User: &UserFields{
					RequestLimitInfo: &RequestLimitInfo{
						RequestLimit:                 75,
						RequestsUsedSinceLastRefresh: 0,
						RequestLimitRefreshDuration:  "MONTHLY",
						RequestLimitPooling:          "USER",
					},
					BonusGrants: []BonusGrant{
						{RequestCreditsGranted: 50, RequestCreditsRemaining: 30},
					},
					Workspaces: []Workspace{
						{
							Name: "My Team",
							BonusGrantsInfo: &BonusGrantsInfo{
								Grants: []BonusGrant{
									{RequestCreditsGranted: 100, RequestCreditsRemaining: 60},
								},
							},
						},
					},
				},
			},
		},
	}

	snapshot, err := parseUsageSnapshot(resp)
	if err != nil {
		t.Fatalf("parseUsageSnapshot() error = %v", err)
	}
	if len(snapshot.Periods) != 2 {
		t.Fatalf("period count = %d, want 2", len(snapshot.Periods))
	}
	// Combined: 150 total, 90 remaining, 60 used = 40%
	if snapshot.Periods[1].Utilization != 40 {
		t.Errorf("combined bonus utilization = %d, want 40", snapshot.Periods[1].Utilization)
	}
}

func TestParseUsageSnapshot_EmptyBonusGrants(t *testing.T) {
	resp := GraphQLResponse{
		Data: &GraphQLData{
			User: &UserUnion{
				TypeName: "UserOutput",
				User: &UserFields{
					RequestLimitInfo: &RequestLimitInfo{
						RequestLimit:                 75,
						RequestsUsedSinceLastRefresh: 0,
						RequestLimitRefreshDuration:  "MONTHLY",
						RequestLimitPooling:          "USER",
					},
					BonusGrants: []BonusGrant{},
					Workspaces: []Workspace{
						{
							Name: "Placeholder Workspace",
							BonusGrantsInfo: &BonusGrantsInfo{
								Grants: []BonusGrant{},
							},
						},
					},
				},
			},
		},
	}

	snapshot, err := parseUsageSnapshot(resp)
	if err != nil {
		t.Fatalf("parseUsageSnapshot() error = %v", err)
	}
	if len(snapshot.Periods) != 1 {
		t.Fatalf("period count = %d, want 1 (no bonus)", len(snapshot.Periods))
	}
}

func TestParseUsageSnapshot_AllRequestLimitFields(t *testing.T) {
	resp := GraphQLResponse{
		Data: &GraphQLData{
			User: &UserUnion{
				TypeName: "UserOutput",
				User: &UserFields{
					RequestLimitInfo: &RequestLimitInfo{
						IsUnlimited:                       false,
						NextRefreshTime:                   "2026-03-27T15:55:15.607496Z",
						RequestLimit:                      75,
						RequestsUsedSinceLastRefresh:      0,
						RequestLimitRefreshDuration:       "MONTHLY",
						IsUnlimitedVoice:                  false,
						VoiceRequestLimit:                 10000,
						VoiceTokenLimit:                   30000,
						VoiceRequestsUsedSinceLastRefresh: 0,
						VoiceTokensUsedSinceLastRefresh:   0,
						IsUnlimitedCodebaseIndices:        false,
						MaxCodebaseIndices:                3,
						MaxFilesPerRepo:                   5000,
						EmbeddingGenerationBatchSize:      100,
						RequestLimitPooling:               "USER",
					},
				},
			},
		},
	}

	snapshot, err := parseUsageSnapshot(resp)
	if err != nil {
		t.Fatalf("parseUsageSnapshot() error = %v", err)
	}
	if snapshot.Periods[0].Name != "Monthly Credits" {
		t.Errorf("period name = %q, want %q", snapshot.Periods[0].Name, "Monthly Credits")
	}
	if snapshot.Periods[0].PeriodType != models.PeriodMonthly {
		t.Errorf("period type = %v, want monthly", snapshot.Periods[0].PeriodType)
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
