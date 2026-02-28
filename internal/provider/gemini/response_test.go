package gemini

import (
	"encoding/json"
	"testing"
	"time"
)

func TestQuotaResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"buckets": [
			{
				"resetTime": "2025-02-20T00:00:00Z",
				"tokenType": "REQUESTS",
				"modelId": "gemini-2.0-flash",
				"remainingFraction": 0.75
			},
			{
				"resetTime": "2025-02-20T00:00:00Z",
				"tokenType": "REQUESTS",
				"modelId": "gemini-1.5-pro",
				"remainingFraction": 0.5
			}
		]
	}`

	var resp QuotaResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(resp.Buckets) != 2 {
		t.Fatalf("len(buckets) = %d, want 2", len(resp.Buckets))
	}

	b := resp.Buckets[0]
	if b.ModelID != "gemini-2.0-flash" {
		t.Errorf("modelId = %q, want %q", b.ModelID, "gemini-2.0-flash")
	}
	if b.RemainingFraction == nil || *b.RemainingFraction != 0.75 {
		t.Errorf("remainingFraction = %v, want 0.75", b.RemainingFraction)
	}
	if b.ResetTime != "2025-02-20T00:00:00Z" {
		t.Errorf("resetTime = %q, want %q", b.ResetTime, "2025-02-20T00:00:00Z")
	}
	if b.TokenType != "REQUESTS" {
		t.Errorf("tokenType = %q, want %q", b.TokenType, "REQUESTS")
	}
}

func TestQuotaResponse_UnmarshalLiveAPIShape(t *testing.T) {
	// Matches the exact shape returned by the live API
	raw := `{
		"buckets": [
			{
				"resetTime": "2026-03-01T04:56:03Z",
				"tokenType": "REQUESTS",
				"modelId": "gemini-2.0-flash",
				"remainingFraction": 1
			},
			{
				"resetTime": "2026-03-01T04:56:03Z",
				"tokenType": "REQUESTS",
				"modelId": "gemini-2.0-flash_vertex",
				"remainingFraction": 1
			},
			{
				"resetTime": "2026-03-01T04:56:03Z",
				"tokenType": "REQUESTS",
				"modelId": "gemini-2.5-flash",
				"remainingFraction": 1
			},
			{
				"resetTime": "2026-03-01T04:56:03Z",
				"tokenType": "REQUESTS",
				"modelId": "gemini-2.5-pro",
				"remainingFraction": 1
			}
		]
	}`

	var resp QuotaResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(resp.Buckets) != 4 {
		t.Fatalf("len(buckets) = %d, want 4", len(resp.Buckets))
	}

	// Verify all fields parsed correctly
	for i, b := range resp.Buckets {
		if b.TokenType != "REQUESTS" {
			t.Errorf("buckets[%d].tokenType = %q, want %q", i, b.TokenType, "REQUESTS")
		}
		if b.RemainingFraction == nil {
			t.Errorf("buckets[%d].remainingFraction is nil", i)
		} else if *b.RemainingFraction != 1 {
			t.Errorf("buckets[%d].remainingFraction = %v, want 1", i, *b.RemainingFraction)
		}
		if b.ResetTime == "" {
			t.Errorf("buckets[%d].resetTime is empty", i)
		}
		if b.ModelID == "" {
			t.Errorf("buckets[%d].modelId is empty", i)
		}
	}

	// Check specific model IDs
	expectedModels := []string{"gemini-2.0-flash", "gemini-2.0-flash_vertex", "gemini-2.5-flash", "gemini-2.5-pro"}
	for i, want := range expectedModels {
		if resp.Buckets[i].ModelID != want {
			t.Errorf("buckets[%d].modelId = %q, want %q", i, resp.Buckets[i].ModelID, want)
		}
	}
}

func TestQuotaResponse_UnmarshalEmpty(t *testing.T) {
	raw := `{}`

	var resp QuotaResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Buckets != nil {
		t.Errorf("expected nil buckets, got %v", resp.Buckets)
	}
}

func TestQuotaBucket_Utilization(t *testing.T) {
	tests := []struct {
		name              string
		remainingFraction *float64
		want              int
	}{
		{"75% remaining", ptrFloat64(0.75), 25},
		{"0% remaining", ptrFloat64(0.0), 100},
		{"100% remaining", ptrFloat64(1.0), 0},
		{"50% remaining", ptrFloat64(0.5), 50},
		{"nil defaults to 0% used", nil, 0},
		{"remaining > 1.0 clamped to 0", ptrFloat64(1.5), 0},
		{"negative remaining clamped to 100", ptrFloat64(-0.5), 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := QuotaBucket{RemainingFraction: tt.remainingFraction}
			got := b.Utilization()
			if got != tt.want {
				t.Errorf("Utilization() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestQuotaBucket_ResetTimeUTC(t *testing.T) {
	tests := []struct {
		name      string
		resetTime string
		wantNil   bool
		wantTime  time.Time
	}{
		{
			name:      "valid time",
			resetTime: "2025-02-20T00:00:00Z",
			wantNil:   false,
			wantTime:  time.Date(2025, 2, 20, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "empty",
			wantNil: true,
		},
		{
			name:      "invalid",
			resetTime: "garbage",
			wantNil:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := QuotaBucket{ResetTime: tt.resetTime}
			got := b.ResetTimeUTC()
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

func TestCodeAssistResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"currentTier": {
			"id": "standard-tier",
			"name": "Gemini Code Assist",
			"description": "Unlimited coding assistant with the most powerful Gemini models",
			"userDefinedCloudaicompanionProject": true,
			"privacyNotice": {},
			"usesGcpTos": true
		},
		"allowedTiers": [
			{
				"id": "standard-tier",
				"name": "Gemini Code Assist",
				"description": "Unlimited coding assistant with the most powerful Gemini models",
				"userDefinedCloudaicompanionProject": true,
				"privacyNotice": {},
				"isDefault": true,
				"usesGcpTos": true
			}
		],
		"cloudaicompanionProject": "helpful-perigee-2nnd9",
		"gcpManaged": false,
		"manageSubscriptionUri": "https://accounts.google.com/AccountChooser?Email=test%40gmail.com&continue=https%3A%2F%2Fone.google.com%2Fsettings",
		"paidTier": {
			"id": "g1-pro-tier",
			"name": "Gemini Code Assist in Google One AI Pro",
			"description": "Google One AI Pro tier for Gemini Code Assist",
			"privacyNotice": {}
		}
	}`

	var resp CodeAssistResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.CurrentTier == nil {
		t.Fatal("expected non-nil currentTier")
	}
	if resp.CurrentTier.ID != "standard-tier" {
		t.Errorf("currentTier.id = %q, want %q", resp.CurrentTier.ID, "standard-tier")
	}
	if resp.CurrentTier.Name != "Gemini Code Assist" {
		t.Errorf("currentTier.name = %q, want %q", resp.CurrentTier.Name, "Gemini Code Assist")
	}
	if resp.CurrentTier.Description != "Unlimited coding assistant with the most powerful Gemini models" {
		t.Errorf("currentTier.description = %q", resp.CurrentTier.Description)
	}
	if !resp.CurrentTier.UserDefinedCloudAICompanionProject {
		t.Error("expected currentTier.userDefinedCloudaicompanionProject to be true")
	}
	if !resp.CurrentTier.UsesGCPTos {
		t.Error("expected currentTier.usesGcpTos to be true")
	}

	if len(resp.AllowedTiers) != 1 {
		t.Fatalf("len(allowedTiers) = %d, want 1", len(resp.AllowedTiers))
	}
	if !resp.AllowedTiers[0].IsDefault {
		t.Error("expected allowedTiers[0].isDefault to be true")
	}

	if resp.CloudAICompanionProject != "helpful-perigee-2nnd9" {
		t.Errorf("cloudaicompanionProject = %q, want %q", resp.CloudAICompanionProject, "helpful-perigee-2nnd9")
	}
	if resp.GCPManaged {
		t.Error("expected gcpManaged to be false")
	}
	if resp.ManageSubscriptionURI == "" {
		t.Error("expected non-empty manageSubscriptionUri")
	}

	if resp.PaidTier == nil {
		t.Fatal("expected non-nil paidTier")
	}
	if resp.PaidTier.ID != "g1-pro-tier" {
		t.Errorf("paidTier.id = %q, want %q", resp.PaidTier.ID, "g1-pro-tier")
	}
	if resp.PaidTier.Name != "Gemini Code Assist in Google One AI Pro" {
		t.Errorf("paidTier.name = %q, want %q", resp.PaidTier.Name, "Gemini Code Assist in Google One AI Pro")
	}
}

func TestCodeAssistResponse_UnmarshalEmpty(t *testing.T) {
	raw := `{}`

	var resp CodeAssistResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.CurrentTier != nil {
		t.Errorf("expected nil currentTier, got %+v", resp.CurrentTier)
	}
	if resp.AllowedTiers != nil {
		t.Errorf("expected nil allowedTiers, got %v", resp.AllowedTiers)
	}
	if resp.PaidTier != nil {
		t.Errorf("expected nil paidTier, got %+v", resp.PaidTier)
	}
}

func TestCodeAssistResponse_UserTier(t *testing.T) {
	tests := []struct {
		name string
		resp *CodeAssistResponse
		want string
	}{
		{
			name: "with current tier",
			resp: &CodeAssistResponse{
				CurrentTier: &CodeAssistTier{Name: "Gemini Code Assist"},
			},
			want: "Gemini Code Assist",
		},
		{
			name: "nil response",
			resp: nil,
			want: "",
		},
		{
			name: "nil current tier",
			resp: &CodeAssistResponse{},
			want: "",
		},
		{
			name: "empty tier name",
			resp: &CodeAssistResponse{
				CurrentTier: &CodeAssistTier{ID: "standard-tier"},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.resp.UserTier()
			if got != tt.want {
				t.Errorf("UserTier() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGeminiCLICredentials_InstalledFormat(t *testing.T) {
	raw := `{
		"installed": {
			"token": "installed-tok",
			"refresh_token": "installed-ref",
			"expiry_date": 1740000000000
		}
	}`

	var cliCreds GeminiCLICredentials
	if err := json.Unmarshal([]byte(raw), &cliCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	creds := cliCreds.EffectiveCredentials()
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if creds.AccessToken != "installed-tok" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "installed-tok")
	}
	if creds.RefreshToken != "installed-ref" {
		t.Errorf("refresh_token = %q, want %q", creds.RefreshToken, "installed-ref")
	}
	if creds.ExpiresAt == "" {
		t.Error("expected expires_at to be set from expiry_date")
	}
}

func TestGeminiCLICredentials_TokenFormat(t *testing.T) {
	raw := `{
		"token": "tok-val",
		"refresh_token": "ref-val",
		"expiry_date": "2025-02-20T00:00:00Z"
	}`

	var cliCreds GeminiCLICredentials
	if err := json.Unmarshal([]byte(raw), &cliCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	creds := cliCreds.EffectiveCredentials()
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if creds.AccessToken != "tok-val" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "tok-val")
	}
	if creds.ExpiresAt != "2025-02-20T00:00:00Z" {
		t.Errorf("expires_at = %q, want %q", creds.ExpiresAt, "2025-02-20T00:00:00Z")
	}
}

func TestGeminiCLICredentials_AccessTokenFormat(t *testing.T) {
	raw := `{
		"access_token": "at-val",
		"refresh_token": "ref-val",
		"expires_at": "2025-02-20T00:00:00Z"
	}`

	var cliCreds GeminiCLICredentials
	if err := json.Unmarshal([]byte(raw), &cliCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	creds := cliCreds.EffectiveCredentials()
	if creds == nil {
		t.Fatal("expected non-nil credentials")
	}
	if creds.AccessToken != "at-val" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "at-val")
	}
	if creds.ExpiresAt != "2025-02-20T00:00:00Z" {
		t.Errorf("expires_at = %q, want %q", creds.ExpiresAt, "2025-02-20T00:00:00Z")
	}
}

func TestGeminiCLICredentials_Empty(t *testing.T) {
	raw := `{}`

	var cliCreds GeminiCLICredentials
	if err := json.Unmarshal([]byte(raw), &cliCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	creds := cliCreds.EffectiveCredentials()
	if creds != nil {
		t.Errorf("expected nil credentials, got %+v", creds)
	}
}

func TestModelsResponse_Unmarshal(t *testing.T) {
	raw := `{
		"models": [
			{"name": "models/gemini-2.0-flash"},
			{"name": "models/gemini-1.5-pro"}
		]
	}`

	var resp ModelsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(resp.Models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(resp.Models))
	}
	if resp.Models[0].Name != "models/gemini-2.0-flash" {
		t.Errorf("models[0].name = %q, want %q", resp.Models[0].Name, "models/gemini-2.0-flash")
	}
}

func TestModelsResponse_UnmarshalEmpty(t *testing.T) {
	raw := `{}`

	var resp ModelsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Models != nil {
		t.Error("expected nil models")
	}
}
