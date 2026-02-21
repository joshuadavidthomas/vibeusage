package antigravity

import (
	"encoding/json"
	"testing"
	"time"
)

func ptrFloat64(f float64) *float64 { return &f }

func TestFetchAvailableModelsResponse_Unmarshal(t *testing.T) {
	raw := `{
		"models": {
			"gemini-2.5-pro": {
				"displayName": "Gemini 2.5 Pro",
				"quotaInfo": {
					"remainingFraction": 0.75,
					"resetTime": "2026-02-20T05:00:00Z"
				},
				"recommended": true
			},
			"gemini-3-flash": {
				"displayName": "Gemini 3 Flash",
				"quotaInfo": {
					"remainingFraction": 0.5,
					"resetTime": "2026-02-20T04:00:00Z"
				}
			}
		}
	}`

	var resp FetchAvailableModelsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(resp.Models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(resp.Models))
	}

	pro := resp.Models["gemini-2.5-pro"]
	if pro.DisplayName != "Gemini 2.5 Pro" {
		t.Errorf("displayName = %q, want %q", pro.DisplayName, "Gemini 2.5 Pro")
	}
	if pro.QuotaInfo == nil {
		t.Fatal("expected quotaInfo")
	}
	if pro.QuotaInfo.RemainingFraction == nil || *pro.QuotaInfo.RemainingFraction != 0.75 {
		t.Errorf("remainingFraction = %v, want 0.75", pro.QuotaInfo.RemainingFraction)
	}
	if !pro.Recommended {
		t.Error("expected recommended = true")
	}
}

func TestFetchAvailableModelsResponse_UnmarshalEmpty(t *testing.T) {
	raw := `{}`

	var resp FetchAvailableModelsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Models != nil {
		t.Errorf("expected nil models, got %v", resp.Models)
	}
}

func TestQuotaInfo_Utilization(t *testing.T) {
	tests := []struct {
		name string
		qi   *QuotaInfo
		want int
	}{
		{"75% remaining", &QuotaInfo{RemainingFraction: ptrFloat64(0.75)}, 25},
		{"0% remaining", &QuotaInfo{RemainingFraction: ptrFloat64(0.0)}, 100},
		{"100% remaining", &QuotaInfo{RemainingFraction: ptrFloat64(1.0)}, 0},
		{"50% remaining", &QuotaInfo{RemainingFraction: ptrFloat64(0.5)}, 50},
		{"nil fraction defaults to 0% used", &QuotaInfo{}, 0},
		{"nil quotaInfo", nil, 0},
		{"remaining > 1.0 clamped to 0", &QuotaInfo{RemainingFraction: ptrFloat64(1.5)}, 0},
		{"negative remaining clamped to 100", &QuotaInfo{RemainingFraction: ptrFloat64(-0.5)}, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.qi.Utilization()
			if got != tt.want {
				t.Errorf("Utilization() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestQuotaInfo_ResetTimeUTC(t *testing.T) {
	tests := []struct {
		name     string
		qi       *QuotaInfo
		wantNil  bool
		wantTime time.Time
	}{
		{
			name:     "valid time",
			qi:       &QuotaInfo{ResetTime: "2026-02-20T05:00:00Z"},
			wantNil:  false,
			wantTime: time.Date(2026, 2, 20, 5, 0, 0, 0, time.UTC),
		},
		{
			name:    "empty",
			qi:      &QuotaInfo{},
			wantNil: true,
		},
		{
			name:    "nil quotaInfo",
			qi:      nil,
			wantNil: true,
		},
		{
			name:    "invalid",
			qi:      &QuotaInfo{ResetTime: "garbage"},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.qi.ResetTimeUTC()
			if tt.wantNil {
				if got != nil {
					t.Errorf("ResetTimeUTC() = %v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("ResetTimeUTC() = nil, want non-nil")
			}
			if !got.Equal(tt.wantTime) {
				t.Errorf("ResetTimeUTC() = %v, want %v", got, tt.wantTime)
			}
		})
	}
}

func TestCodeAssistResponse_EffectiveTier(t *testing.T) {
	tests := []struct {
		name string
		resp *CodeAssistResponse
		want string
	}{
		{
			name: "tier name from currentTier",
			resp: &CodeAssistResponse{CurrentTier: &TierInfo{ID: "pro-tier", Name: "Google AI Pro"}},
			want: "Google AI Pro",
		},
		{
			name: "tier id fallback",
			resp: &CodeAssistResponse{CurrentTier: &TierInfo{ID: "free-tier"}},
			want: "free-tier",
		},
		{
			name: "user_tier fallback",
			resp: &CodeAssistResponse{UserTier: "premium"},
			want: "premium",
		},
		{
			name: "nil response",
			resp: nil,
			want: "",
		},
		{
			name: "empty response",
			resp: &CodeAssistResponse{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.EffectiveTier()
			if got != tt.want {
				t.Errorf("EffectiveTier() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCodeAssistRequest_Marshal(t *testing.T) {
	req := CodeAssistRequest{
		Metadata: &CodeAssistRequestMetadata{
			IDEType:    "ANTIGRAVITY",
			Platform:   "PLATFORM_UNSPECIFIED",
			PluginType: "GEMINI",
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	want := `{"metadata":{"ideType":"ANTIGRAVITY","platform":"PLATFORM_UNSPECIFIED","pluginType":"GEMINI"}}`
	if string(data) != want {
		t.Errorf("marshal = %s, want %s", string(data), want)
	}
}

func TestAntigravityCredentials_AccessTokenFormat(t *testing.T) {
	raw := `{
		"access_token": "at-val",
		"refresh_token": "ref-val",
		"expires_at": "2026-02-20T00:00:00Z"
	}`

	var agCreds AntigravityCredentials
	if err := json.Unmarshal([]byte(raw), &agCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	creds := agCreds.ToOAuthCredentials()
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if creds.AccessToken != "at-val" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "at-val")
	}
}

func TestAntigravityCredentials_Empty(t *testing.T) {
	raw := `{}`

	var agCreds AntigravityCredentials
	if err := json.Unmarshal([]byte(raw), &agCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	creds := agCreds.ToOAuthCredentials()
	if creds != nil {
		t.Errorf("expected nil credentials, got %+v", creds)
	}
}

func TestVscdbAuthStatus_Unmarshal(t *testing.T) {
	raw := `{
		"name": "Test User",
		"apiKey": "ya29.test-token",
		"email": "test@example.com"
	}`

	var status VscdbAuthStatus
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if status.Name != "Test User" {
		t.Errorf("name = %q, want %q", status.Name, "Test User")
	}
	if status.APIKey != "ya29.test-token" {
		t.Errorf("apiKey = %q, want %q", status.APIKey, "ya29.test-token")
	}
	if status.Email != "test@example.com" {
		t.Errorf("email = %q, want %q", status.Email, "test@example.com")
	}
}
