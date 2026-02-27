package antigravity

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

func ptrFloat64(f float64) *float64 { return &f }

func TestFetchAvailableModelsResponse_Unmarshal(t *testing.T) {
	raw := `{
		"models": {
			"gemini-2.5-pro": {
				"displayName": "Gemini 2.5 Pro",
				"supportsImages": true,
				"supportsThinking": true,
				"thinkingBudget": 1024,
				"minThinkingBudget": 128,
				"recommended": true,
				"maxTokens": 1048576,
				"maxOutputTokens": 65535,
				"tokenizerType": "LLAMA_WITH_SPECIAL",
				"quotaInfo": {
					"remainingFraction": 0.75,
					"resetTime": "2026-02-20T05:00:00Z"
				},
				"model": "MODEL_GOOGLE_GEMINI_2_5_PRO",
				"apiProvider": "API_PROVIDER_GOOGLE_GEMINI",
				"modelProvider": "MODEL_PROVIDER_GOOGLE",
				"supportedMimeTypes": {
					"image/png": true,
					"application/pdf": true
				},
				"requiresImageOutputOutsideFunctionResponses": true
			},
			"gemini-3-flash": {
				"displayName": "Gemini 3 Flash",
				"supportsVideo": true,
				"tagTitle": "New",
				"quotaInfo": {
					"remainingFraction": 0.5,
					"resetTime": "2026-02-20T04:00:00Z"
				},
				"model": "MODEL_PLACEHOLDER_M18",
				"apiProvider": "API_PROVIDER_GOOGLE_GEMINI",
				"modelProvider": "MODEL_PROVIDER_GOOGLE"
			},
			"chat_20706": {
				"maxTokens": 16384,
				"tokenizerType": "QWEN2",
				"quotaInfo": {"remainingFraction": 1},
				"model": "MODEL_CHAT_20706",
				"apiProvider": "API_PROVIDER_INTERNAL",
				"isInternal": true,
				"supportsCumulativeContext": true,
				"supportsEstimateTokenCounter": true,
				"toolFormatterType": "TOOL_FORMATTER_TYPE_XML",
				"promptTemplaterType": "PROMPT_TEMPLATER_TYPE_CHATML",
				"requiresLeadInGeneration": true,
				"addCursorToFindReplaceTarget": true,
				"tabJumpPrintLineRange": true,
				"requiresNoXmlToolExamples": true
			}
		},
		"defaultAgentModelId": "gemini-2.5-pro",
		"agentModelSorts": [
			{
				"displayName": "Recommended",
				"groups": [{"modelIds": ["gemini-2.5-pro", "gemini-3-flash"]}]
			}
		],
		"commandModelIds": ["gemini-3-flash"],
		"tabModelIds": ["chat_20706"],
		"imageGenerationModelIds": ["gemini-3.1-flash-image"],
		"mqueryModelIds": ["gemini-2.5-flash-lite"],
		"webSearchModelIds": ["gemini-2.5-flash"],
		"commitMessageModelIds": ["gemini-2.5-flash"],
		"deprecatedModelIds": {
			"claude-opus-4-5-thinking": {
				"newModelId": "claude-opus-4-6-thinking",
				"oldModelEnum": "MODEL_PLACEHOLDER_M12",
				"newModelEnum": "MODEL_PLACEHOLDER_M26"
			}
		}
	}`

	var resp FetchAvailableModelsResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(resp.Models) != 3 {
		t.Fatalf("len(models) = %d, want 3", len(resp.Models))
	}

	// Top-level fields
	if resp.DefaultAgentModelID != "gemini-2.5-pro" {
		t.Errorf("defaultAgentModelId = %q, want %q", resp.DefaultAgentModelID, "gemini-2.5-pro")
	}
	if len(resp.AgentModelSorts) != 1 {
		t.Fatalf("len(agentModelSorts) = %d, want 1", len(resp.AgentModelSorts))
	}
	if resp.AgentModelSorts[0].DisplayName != "Recommended" {
		t.Errorf("agentModelSorts[0].displayName = %q, want %q", resp.AgentModelSorts[0].DisplayName, "Recommended")
	}
	if len(resp.AgentModelSorts[0].Groups) != 1 || len(resp.AgentModelSorts[0].Groups[0].ModelIDs) != 2 {
		t.Errorf("agentModelSorts[0].groups unexpected structure")
	}
	if len(resp.CommandModelIDs) != 1 || resp.CommandModelIDs[0] != "gemini-3-flash" {
		t.Errorf("commandModelIds = %v", resp.CommandModelIDs)
	}
	if len(resp.TabModelIDs) != 1 || resp.TabModelIDs[0] != "chat_20706" {
		t.Errorf("tabModelIds = %v", resp.TabModelIDs)
	}
	if len(resp.ImageGenerationModelIDs) != 1 {
		t.Errorf("imageGenerationModelIds = %v", resp.ImageGenerationModelIDs)
	}
	if len(resp.MqueryModelIDs) != 1 {
		t.Errorf("mqueryModelIds = %v", resp.MqueryModelIDs)
	}
	if len(resp.WebSearchModelIDs) != 1 {
		t.Errorf("webSearchModelIds = %v", resp.WebSearchModelIDs)
	}
	if len(resp.CommitMessageModelIDs) != 1 {
		t.Errorf("commitMessageModelIds = %v", resp.CommitMessageModelIDs)
	}
	if len(resp.DeprecatedModelIDs) != 1 {
		t.Fatalf("len(deprecatedModelIds) = %d, want 1", len(resp.DeprecatedModelIDs))
	}
	dep := resp.DeprecatedModelIDs["claude-opus-4-5-thinking"]
	if dep.NewModelID != "claude-opus-4-6-thinking" {
		t.Errorf("deprecated newModelId = %q", dep.NewModelID)
	}

	// Model with full capabilities
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
	if !pro.SupportsImages {
		t.Error("expected supportsImages = true")
	}
	if !pro.SupportsThinking {
		t.Error("expected supportsThinking = true")
	}
	if pro.ThinkingBudget != 1024 {
		t.Errorf("thinkingBudget = %d, want 1024", pro.ThinkingBudget)
	}
	if pro.MinThinkingBudget != 128 {
		t.Errorf("minThinkingBudget = %d, want 128", pro.MinThinkingBudget)
	}
	if pro.MaxTokens != 1048576 {
		t.Errorf("maxTokens = %d, want 1048576", pro.MaxTokens)
	}
	if pro.MaxOutputTokens != 65535 {
		t.Errorf("maxOutputTokens = %d, want 65535", pro.MaxOutputTokens)
	}
	if pro.TokenizerType != "LLAMA_WITH_SPECIAL" {
		t.Errorf("tokenizerType = %q", pro.TokenizerType)
	}
	if pro.Model != "MODEL_GOOGLE_GEMINI_2_5_PRO" {
		t.Errorf("model = %q", pro.Model)
	}
	if pro.APIProvider != "API_PROVIDER_GOOGLE_GEMINI" {
		t.Errorf("apiProvider = %q", pro.APIProvider)
	}
	if pro.ModelProvider != "MODEL_PROVIDER_GOOGLE" {
		t.Errorf("modelProvider = %q", pro.ModelProvider)
	}
	if len(pro.SupportedMimeTypes) != 2 {
		t.Errorf("supportedMimeTypes len = %d, want 2", len(pro.SupportedMimeTypes))
	}
	if !pro.RequiresImageOutputOutsideFunctionResponses {
		t.Error("expected requiresImageOutputOutsideFunctionResponses = true")
	}

	// Model with tag and video
	flash := resp.Models["gemini-3-flash"]
	if flash.TagTitle != "New" {
		t.Errorf("tagTitle = %q, want %q", flash.TagTitle, "New")
	}
	if !flash.SupportsVideo {
		t.Error("expected supportsVideo = true")
	}

	// Internal model with IDE-specific fields
	chat := resp.Models["chat_20706"]
	if !chat.IsInternal {
		t.Error("expected isInternal = true")
	}
	if !chat.SupportsCumulativeContext {
		t.Error("expected supportsCumulativeContext = true")
	}
	if !chat.SupportsEstimateTokenCounter {
		t.Error("expected supportsEstimateTokenCounter = true")
	}
	if chat.ToolFormatterType != "TOOL_FORMATTER_TYPE_XML" {
		t.Errorf("toolFormatterType = %q", chat.ToolFormatterType)
	}
	if chat.PromptTemplaterType != "PROMPT_TEMPLATER_TYPE_CHATML" {
		t.Errorf("promptTemplaterType = %q", chat.PromptTemplaterType)
	}
	if !chat.RequiresLeadInGeneration {
		t.Error("expected requiresLeadInGeneration = true")
	}
	if !chat.RequiresNoXmlToolExamples {
		t.Error("expected requiresNoXmlToolExamples = true")
	}
	if !chat.AddCursorToFindReplaceTarget {
		t.Error("expected addCursorToFindReplaceTarget = true")
	}
	if !chat.TabJumpPrintLineRange {
		t.Error("expected tabJumpPrintLineRange = true")
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

func TestCodeAssistResponse_Unmarshal(t *testing.T) {
	raw := `{
		"currentTier": {
			"id": "free-tier",
			"name": "Antigravity",
			"description": "Gemini-powered code suggestions and chat in multiple IDEs",
			"privacyNotice": {
				"showNotice": true,
				"noticeText": "Privacy notice text here."
			},
			"upgradeSubscriptionUri": "https://one.google.com/ai",
			"upgradeSubscriptionText": "Upgrade to get more requests.",
			"upgradeSubscriptionType": "GOOGLE_ONE_HELIUM"
		},
		"allowedTiers": [
			{
				"id": "free-tier",
				"name": "Antigravity",
				"description": "Free tier",
				"isDefault": true,
				"privacyNotice": {"showNotice": true, "noticeText": "Notice."}
			},
			{
				"id": "standard-tier",
				"name": "Antigravity",
				"description": "Standard tier",
				"userDefinedCloudaicompanionProject": true,
				"usesGcpTos": true,
				"privacyNotice": {}
			}
		],
		"cloudaicompanionProject": "helpful-perigee-2nnd9",
		"gcpManaged": false,
		"upgradeSubscriptionUri": "https://codeassist.google.com/upgrade",
		"paidTier": {
			"id": "g1-pro-tier",
			"name": "Google AI Pro",
			"description": "Google AI Pro",
			"upgradeSubscriptionUri": "https://antigravity.google/g1-upgrade",
			"upgradeSubscriptionText": "Upgrade to Google AI Ultra."
		}
	}`

	var resp CodeAssistResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// currentTier
	if resp.CurrentTier == nil {
		t.Fatal("expected currentTier")
	}
	if resp.CurrentTier.ID != "free-tier" {
		t.Errorf("currentTier.id = %q", resp.CurrentTier.ID)
	}
	if resp.CurrentTier.Name != "Antigravity" {
		t.Errorf("currentTier.name = %q", resp.CurrentTier.Name)
	}
	if resp.CurrentTier.Description != "Gemini-powered code suggestions and chat in multiple IDEs" {
		t.Errorf("currentTier.description = %q", resp.CurrentTier.Description)
	}
	if resp.CurrentTier.PrivacyNotice == nil || !resp.CurrentTier.PrivacyNotice.ShowNotice {
		t.Error("expected currentTier.privacyNotice.showNotice = true")
	}
	if resp.CurrentTier.UpgradeSubscriptionURI != "https://one.google.com/ai" {
		t.Errorf("currentTier.upgradeSubscriptionUri = %q", resp.CurrentTier.UpgradeSubscriptionURI)
	}
	if resp.CurrentTier.UpgradeSubscriptionText != "Upgrade to get more requests." {
		t.Errorf("currentTier.upgradeSubscriptionText = %q", resp.CurrentTier.UpgradeSubscriptionText)
	}
	if resp.CurrentTier.UpgradeSubscriptionType != "GOOGLE_ONE_HELIUM" {
		t.Errorf("currentTier.upgradeSubscriptionType = %q", resp.CurrentTier.UpgradeSubscriptionType)
	}

	// allowedTiers
	if len(resp.AllowedTiers) != 2 {
		t.Fatalf("len(allowedTiers) = %d, want 2", len(resp.AllowedTiers))
	}
	if !resp.AllowedTiers[0].IsDefault {
		t.Error("expected allowedTiers[0].isDefault = true")
	}
	if !resp.AllowedTiers[1].UserDefinedCloudAICompanionProject {
		t.Error("expected allowedTiers[1].userDefinedCloudaicompanionProject = true")
	}
	if !resp.AllowedTiers[1].UsesGCPTOS {
		t.Error("expected allowedTiers[1].usesGcpTos = true")
	}

	// Top-level fields
	if resp.CloudAICompanionProject != "helpful-perigee-2nnd9" {
		t.Errorf("cloudaicompanionProject = %q", resp.CloudAICompanionProject)
	}
	if resp.GCPManaged {
		t.Error("expected gcpManaged = false")
	}
	if resp.UpgradeSubscriptionURI != "https://codeassist.google.com/upgrade" {
		t.Errorf("upgradeSubscriptionUri = %q", resp.UpgradeSubscriptionURI)
	}

	// paidTier
	if resp.PaidTier == nil {
		t.Fatal("expected paidTier")
	}
	if resp.PaidTier.ID != "g1-pro-tier" {
		t.Errorf("paidTier.id = %q", resp.PaidTier.ID)
	}
	if resp.PaidTier.Name != "Google AI Pro" {
		t.Errorf("paidTier.name = %q", resp.PaidTier.Name)
	}
	if resp.PaidTier.UpgradeSubscriptionURI != "https://antigravity.google/g1-upgrade" {
		t.Errorf("paidTier.upgradeSubscriptionUri = %q", resp.PaidTier.UpgradeSubscriptionURI)
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

func TestParseSubscriptionFromProto_ProTier(t *testing.T) {
	inner := protoBytes(1, []byte("g1-pro-tier"))
	inner = append(inner, protoBytes(2, []byte("Google AI Pro"))...)
	proto := protoBytes(36, inner)

	info := parseSubscriptionFromProto(base64.StdEncoding.EncodeToString(proto))

	if info == nil {
		t.Fatal("expected non-nil subscription info")
	}
	if info.TierID != "g1-pro-tier" {
		t.Errorf("tierID = %q, want %q", info.TierID, "g1-pro-tier")
	}
	if info.TierName != "Google AI Pro" {
		t.Errorf("tierName = %q, want %q", info.TierName, "Google AI Pro")
	}
}

func TestParseSubscriptionFromProto_WithOtherFields(t *testing.T) {
	// Simulate a real proto with other fields before field 36
	var proto []byte
	proto = append(proto, protoBytes(3, []byte("Joshua Thomas"))...)
	proto = append(proto, protoBytes(7, []byte("user@example.com"))...)

	inner := protoBytes(1, []byte("g1-ultra-tier"))
	inner = append(inner, protoBytes(2, []byte("Google AI Ultra"))...)
	proto = append(proto, protoBytes(36, inner)...)

	info := parseSubscriptionFromProto(base64.StdEncoding.EncodeToString(proto))

	if info == nil {
		t.Fatal("expected non-nil subscription info")
	}
	if info.TierName != "Google AI Ultra" {
		t.Errorf("tierName = %q, want %q", info.TierName, "Google AI Ultra")
	}
}

func TestParseSubscriptionFromProto_NoField36(t *testing.T) {
	proto := protoBytes(3, []byte("Test User"))

	info := parseSubscriptionFromProto(base64.StdEncoding.EncodeToString(proto))
	if info != nil {
		t.Errorf("expected nil, got %+v", info)
	}
}

func TestParseSubscriptionFromProto_Empty(t *testing.T) {
	if info := parseSubscriptionFromProto(""); info != nil {
		t.Errorf("expected nil for empty input, got %+v", info)
	}
}

func TestParseSubscriptionFromProto_InvalidBase64(t *testing.T) {
	if info := parseSubscriptionFromProto("not-valid-base64!!!"); info != nil {
		t.Errorf("expected nil for invalid base64, got %+v", info)
	}
}

// protoBytes encodes a length-delimited protobuf field.
func protoBytes(num protowire.Number, val []byte) []byte {
	b := protowire.AppendTag(nil, num, protowire.BytesType)
	b = protowire.AppendBytes(b, val)
	return b
}
