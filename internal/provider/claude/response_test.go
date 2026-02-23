package claude

import (
	"encoding/json"
	"testing"
)

func TestOAuthUsageResponse_UnmarshalFullResponse(t *testing.T) {
	raw := `{
		"five_hour": {"utilization": 42.0, "resets_at": "2025-02-19T22:00:00Z"},
		"seven_day": {"utilization": 75.0, "resets_at": "2025-02-26T00:00:00Z"},
		"monthly": {"utilization": 30.0, "resets_at": "2025-03-01T00:00:00Z"},
		"seven_day_sonnet": {"utilization": 60.0, "resets_at": "2025-02-26T00:00:00Z"},
		"seven_day_opus": {"utilization": 10.0, "resets_at": "2025-02-26T00:00:00Z"},
		"seven_day_haiku": {"utilization": 90.0, "resets_at": "2025-02-26T00:00:00Z"},
		"extra_usage": {"is_enabled": true, "used_credits": 550, "monthly_limit": 10000}
	}`

	var resp OAuthUsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Standard periods
	if resp.FiveHour == nil {
		t.Fatal("expected five_hour to be present")
	}
	if resp.FiveHour.Utilization != 42.0 {
		t.Errorf("five_hour utilization = %v, want 42.0", resp.FiveHour.Utilization)
	}
	if resp.FiveHour.ResetsAt != "2025-02-19T22:00:00Z" {
		t.Errorf("five_hour resets_at = %q, want %q", resp.FiveHour.ResetsAt, "2025-02-19T22:00:00Z")
	}

	if resp.SevenDay == nil {
		t.Fatal("expected seven_day to be present")
	}
	if resp.SevenDay.Utilization != 75.0 {
		t.Errorf("seven_day utilization = %v, want 75.0", resp.SevenDay.Utilization)
	}

	if resp.Monthly == nil {
		t.Fatal("expected monthly to be present")
	}
	if resp.Monthly.Utilization != 30.0 {
		t.Errorf("monthly utilization = %v, want 30.0", resp.Monthly.Utilization)
	}

	// Model-specific periods
	if resp.SevenDaySonnet == nil {
		t.Fatal("expected seven_day_sonnet to be present")
	}
	if resp.SevenDaySonnet.Utilization != 60.0 {
		t.Errorf("seven_day_sonnet utilization = %v, want 60.0", resp.SevenDaySonnet.Utilization)
	}

	if resp.SevenDayOpus == nil {
		t.Fatal("expected seven_day_opus to be present")
	}
	if resp.SevenDayOpus.Utilization != 10.0 {
		t.Errorf("seven_day_opus utilization = %v, want 10.0", resp.SevenDayOpus.Utilization)
	}

	if resp.SevenDayHaiku == nil {
		t.Fatal("expected seven_day_haiku to be present")
	}
	if resp.SevenDayHaiku.Utilization != 90.0 {
		t.Errorf("seven_day_haiku utilization = %v, want 90.0", resp.SevenDayHaiku.Utilization)
	}

	// Extra usage
	if resp.ExtraUsage == nil {
		t.Fatal("expected extra_usage to be present")
	}
	if !resp.ExtraUsage.IsEnabled {
		t.Error("expected extra_usage.is_enabled to be true")
	}
	if resp.ExtraUsage.UsedCredits != 550 {
		t.Errorf("extra_usage.used_credits = %v, want 550", resp.ExtraUsage.UsedCredits)
	}
	if resp.ExtraUsage.MonthlyLimit != 10000 {
		t.Errorf("extra_usage.monthly_limit = %v, want 10000", resp.ExtraUsage.MonthlyLimit)
	}
}

func TestOAuthUsageResponse_UnmarshalMinimalResponse(t *testing.T) {
	raw := `{
		"five_hour": {"utilization": 10.0, "resets_at": "2025-02-19T22:00:00Z"}
	}`

	var resp OAuthUsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.FiveHour == nil {
		t.Fatal("expected five_hour to be present")
	}
	if resp.FiveHour.Utilization != 10.0 {
		t.Errorf("five_hour utilization = %v, want 10.0", resp.FiveHour.Utilization)
	}

	// Other fields should be nil
	if resp.SevenDay != nil {
		t.Error("expected seven_day to be nil")
	}
	if resp.Monthly != nil {
		t.Error("expected monthly to be nil")
	}
	if resp.SevenDaySonnet != nil {
		t.Error("expected seven_day_sonnet to be nil")
	}
	if resp.ExtraUsage != nil {
		t.Error("expected extra_usage to be nil")
	}
}

func TestOAuthUsageResponse_UnmarshalEmptyResponse(t *testing.T) {
	raw := `{}`

	var resp OAuthUsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.FiveHour != nil {
		t.Error("expected five_hour to be nil")
	}
	if resp.SevenDay != nil {
		t.Error("expected seven_day to be nil")
	}
	if resp.Monthly != nil {
		t.Error("expected monthly to be nil")
	}
}

func TestOAuthUsageResponse_PeriodWithoutResetsAt(t *testing.T) {
	raw := `{
		"five_hour": {"utilization": 50.0}
	}`

	var resp OAuthUsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.FiveHour == nil {
		t.Fatal("expected five_hour to be present")
	}
	if resp.FiveHour.Utilization != 50.0 {
		t.Errorf("utilization = %v, want 50.0", resp.FiveHour.Utilization)
	}
	if resp.FiveHour.ResetsAt != "" {
		t.Errorf("resets_at = %q, want empty", resp.FiveHour.ResetsAt)
	}
}

func TestExtraUsageResponse_Disabled(t *testing.T) {
	raw := `{"is_enabled": false, "used_credits": 0, "monthly_limit": 0}`

	var resp ExtraUsageResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.IsEnabled {
		t.Error("expected is_enabled to be false")
	}
	if resp.UsedCredits != 0 {
		t.Errorf("used_credits = %v, want 0", resp.UsedCredits)
	}
}

func TestOAuthTokenResponse_Unmarshal(t *testing.T) {
	raw := `{
		"access_token": "new-access-token",
		"refresh_token": "new-refresh-token",
		"token_type": "Bearer",
		"expires_in": 3600
	}`

	var resp OAuthTokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "new-access-token" {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "new-access-token")
	}
	if resp.RefreshToken != "new-refresh-token" {
		t.Errorf("refresh_token = %q, want %q", resp.RefreshToken, "new-refresh-token")
	}
	if resp.TokenType != "Bearer" {
		t.Errorf("token_type = %q, want %q", resp.TokenType, "Bearer")
	}
	if resp.ExpiresIn != 3600 {
		t.Errorf("expires_in = %v, want 3600", resp.ExpiresIn)
	}
}

func TestOAuthTokenResponse_UnmarshalMinimal(t *testing.T) {
	raw := `{"access_token": "tok"}`

	var resp OAuthTokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "tok" {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "tok")
	}
	if resp.ExpiresIn != 0 {
		t.Errorf("expires_in = %v, want 0", resp.ExpiresIn)
	}
}

func TestOAuthCredentials_Unmarshal(t *testing.T) {
	raw := `{
		"access_token": "my-access-token",
		"refresh_token": "my-refresh-token",
		"expires_at": "2025-02-19T22:00:00Z"
	}`

	var creds OAuthCredentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if creds.AccessToken != "my-access-token" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "my-access-token")
	}
	if creds.RefreshToken != "my-refresh-token" {
		t.Errorf("refresh_token = %q, want %q", creds.RefreshToken, "my-refresh-token")
	}
	if creds.ExpiresAt != "2025-02-19T22:00:00Z" {
		t.Errorf("expires_at = %q, want %q", creds.ExpiresAt, "2025-02-19T22:00:00Z")
	}
}

func TestOAuthCredentials_UnmarshalMinimal(t *testing.T) {
	raw := `{"access_token": "tok"}`

	var creds OAuthCredentials
	if err := json.Unmarshal([]byte(raw), &creds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if creds.AccessToken != "tok" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "tok")
	}
	if creds.RefreshToken != "" {
		t.Errorf("refresh_token = %q, want empty", creds.RefreshToken)
	}
	if creds.ExpiresAt != "" {
		t.Errorf("expires_at = %q, want empty", creds.ExpiresAt)
	}
}

func TestClaudeCLICredentials_Unmarshal(t *testing.T) {
	raw := `{
		"claudeAiOauth": {
			"accessToken": "cli-access-token",
			"refreshToken": "cli-refresh-token",
			"expiresAt": 1740000000000
		}
	}`

	var cliCreds ClaudeCLICredentials
	if err := json.Unmarshal([]byte(raw), &cliCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if cliCreds.ClaudeAiOauth == nil {
		t.Fatal("expected claudeAiOauth to be present")
	}
	if cliCreds.ClaudeAiOauth.AccessToken != "cli-access-token" {
		t.Errorf("access_token = %q, want %q", cliCreds.ClaudeAiOauth.AccessToken, "cli-access-token")
	}
	if cliCreds.ClaudeAiOauth.RefreshToken != "cli-refresh-token" {
		t.Errorf("refresh_token = %q, want %q", cliCreds.ClaudeAiOauth.RefreshToken, "cli-refresh-token")
	}
	if cliCreds.ClaudeAiOauth.ExpiresAt != 1740000000000 {
		t.Errorf("expires_at = %v, want 1740000000000", cliCreds.ClaudeAiOauth.ExpiresAt)
	}
}

func TestClaudeCLICredentials_ToOAuthCredentials(t *testing.T) {
	cliCreds := ClaudeCLICredentials{
		ClaudeAiOauth: &ClaudeCLIOAuth{
			AccessToken:  "cli-access-token",
			RefreshToken: "cli-refresh-token",
			ExpiresAt:    1740000000000, // ms timestamp
		},
	}

	creds := cliCreds.ClaudeAiOauth.ToOAuthCredentials()

	if creds.AccessToken != "cli-access-token" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "cli-access-token")
	}
	if creds.RefreshToken != "cli-refresh-token" {
		t.Errorf("refresh_token = %q, want %q", creds.RefreshToken, "cli-refresh-token")
	}
	// ExpiresAt should be converted from ms timestamp to RFC3339
	if creds.ExpiresAt == "" {
		t.Error("expected expires_at to be set")
	}
	// 1740000000000ms = 2025-02-19T21:20:00Z
	want := "2025-02-19T21:20:00Z"
	if creds.ExpiresAt != want {
		t.Errorf("expires_at = %q, want %q", creds.ExpiresAt, want)
	}
}

func TestClaudeCLIOAuth_ToOAuthCredentials_ZeroExpiresAt(t *testing.T) {
	cliOAuth := ClaudeCLIOAuth{
		AccessToken:  "tok",
		RefreshToken: "ref",
		ExpiresAt:    0,
	}

	creds := cliOAuth.ToOAuthCredentials()

	if creds.AccessToken != "tok" {
		t.Errorf("access_token = %q, want %q", creds.AccessToken, "tok")
	}
	if creds.ExpiresAt != "" {
		t.Errorf("expires_at = %q, want empty for zero timestamp", creds.ExpiresAt)
	}
}

func TestOAuthCredentials_Roundtrip(t *testing.T) {
	original := OAuthCredentials{
		AccessToken:  "my-token",
		RefreshToken: "my-refresh",
		ExpiresAt:    "2025-02-19T22:00:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded OAuthCredentials
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestClaudeCLICredentials_NilOAuth(t *testing.T) {
	raw := `{}`

	var cliCreds ClaudeCLICredentials
	if err := json.Unmarshal([]byte(raw), &cliCreds); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if cliCreds.ClaudeAiOauth != nil {
		t.Error("expected claudeAiOauth to be nil")
	}
}
