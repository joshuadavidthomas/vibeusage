package antigravity

import (
	"encoding/base64"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
	"github.com/joshuadavidthomas/vibeusage/internal/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/provider/googleauth"
	"google.golang.org/protobuf/encoding/protowire"
)

// FetchAvailableModelsResponse represents the response from the
// cloudcode-pa.googleapis.com/v1internal:fetchAvailableModels endpoint.
type FetchAvailableModelsResponse struct {
	Models map[string]ModelInfo `json:"models,omitempty"`
}

// ModelInfo represents a single model's info from the fetchAvailableModels response.
type ModelInfo struct {
	DisplayName string     `json:"displayName,omitempty"`
	QuotaInfo   *QuotaInfo `json:"quotaInfo,omitempty"`
	Recommended bool       `json:"recommended,omitempty"`
}

// QuotaInfo contains the remaining fraction and reset time for a model.
type QuotaInfo struct {
	RemainingFraction *float64 `json:"remainingFraction,omitempty"`
	ResetTime         string   `json:"resetTime,omitempty"`
}

// Utilization returns the usage percentage, clamped to [0, 100].
// If remainingFraction is absent, assumes full quota remaining (0% used).
func (q *QuotaInfo) Utilization() int {
	if q == nil {
		return 0
	}
	rf := 1.0
	if q.RemainingFraction != nil {
		rf = *q.RemainingFraction
	}
	pct := int((1 - rf) * 100)
	return max(0, min(pct, 100))
}

// ResetTimeUTC parses the resetTime as a time.Time.
func (q *QuotaInfo) ResetTimeUTC() *time.Time {
	if q == nil {
		return nil
	}
	return models.ParseRFC3339Ptr(q.ResetTime)
}

// CodeAssistResponse represents the response from the
// cloudcode-pa.googleapis.com/v1internal:loadCodeAssist endpoint.
type CodeAssistResponse struct {
	CurrentTier *TierInfo `json:"currentTier,omitempty"`
	UserTier    string    `json:"user_tier,omitempty"` // fallback field
}

// TierInfo represents subscription tier information.
type TierInfo struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

// EffectiveTier returns the user's tier name from whichever field is present.
func (c *CodeAssistResponse) EffectiveTier() string {
	if c == nil {
		return ""
	}
	if c.CurrentTier != nil && c.CurrentTier.Name != "" {
		return c.CurrentTier.Name
	}
	if c.CurrentTier != nil && c.CurrentTier.ID != "" {
		return c.CurrentTier.ID
	}
	return c.UserTier
}

// CodeAssistRequest represents the request body for the loadCodeAssist endpoint.
type CodeAssistRequest struct {
	Metadata *CodeAssistRequestMetadata `json:"metadata,omitempty"`
}

// CodeAssistRequestMetadata identifies the requesting IDE.
type CodeAssistRequestMetadata struct {
	IDEType    string `json:"ideType"`
	Platform   string `json:"platform"`
	PluginType string `json:"pluginType"`
}

// AntigravityCredentials represents a JSON credential file format.
type AntigravityCredentials struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	ExpiryDate   any    `json:"expiry_date,omitempty"`
	Token        string `json:"token,omitempty"`
}

// ToOAuthCredentials converts the Antigravity credential format to OAuthCredentials.
func (a *AntigravityCredentials) ToOAuthCredentials() *oauth.Credentials {
	accessToken := a.AccessToken
	if accessToken == "" {
		accessToken = a.Token
	}
	if accessToken == "" {
		return nil
	}
	creds := &oauth.Credentials{
		AccessToken:  accessToken,
		RefreshToken: a.RefreshToken,
		ExpiresAt:    a.ExpiresAt,
	}
	if creds.ExpiresAt == "" {
		creds.ExpiresAt = googleauth.ParseExpiryDate(a.ExpiryDate)
	}
	return creds
}

// VscdbAuthStatus represents the JSON blob stored in Antigravity's VS Code
// state database under the "antigravityAuthStatus" key.
type VscdbAuthStatus struct {
	Name                        string `json:"name,omitempty"`
	APIKey                      string `json:"apiKey,omitempty"`
	Email                       string `json:"email,omitempty"`
	UserStatusProtoBinaryBase64 string `json:"userStatusProtoBinaryBase64,omitempty"`
}

// SubscriptionInfo holds the subscription tier parsed from the protobuf
// blob in the Antigravity vscdb auth status.
type SubscriptionInfo struct {
	TierID   string // e.g. "g1-pro-tier"
	TierName string // e.g. "Google AI Pro"
}

// parseSubscriptionFromProto extracts subscription info from the
// userStatusProtoBinaryBase64 field in the vscdb auth status.
//
// The protobuf structure (reverse-engineered):
//
//	Top-level message:
//	  field 3  (string): user name
//	  field 7  (string): user email
//	  field 13 (message): PlanStatus (contains PlanInfo with plan name "Pro")
//	  field 36 (message): SubscriptionInfo
//	    field 1 (string): tier ID (e.g. "g1-pro-tier")
//	    field 2 (string): tier name (e.g. "Google AI Pro")
func parseSubscriptionFromProto(base64Value string) *SubscriptionInfo {
	if base64Value == "" {
		return nil
	}
	data, err := base64.StdEncoding.DecodeString(base64Value)
	if err != nil {
		return nil
	}

	// Walk top-level fields looking for field 36 (subscription info)
	subscriptionBytes := findField(data, 36)
	if subscriptionBytes == nil {
		return nil
	}

	info := &SubscriptionInfo{
		TierID:   string(findField(subscriptionBytes, 1)),
		TierName: string(findField(subscriptionBytes, 2)),
	}

	if info.TierID == "" && info.TierName == "" {
		return nil
	}
	return info
}

// findField walks a protobuf message and returns the raw bytes of the
// first length-delimited field matching the given field number.
func findField(b []byte, target protowire.Number) []byte {
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return nil
		}
		b = b[n:]

		switch typ {
		case protowire.VarintType:
			_, n = protowire.ConsumeVarint(b)
		case protowire.Fixed32Type:
			_, n = protowire.ConsumeFixed32(b)
		case protowire.Fixed64Type:
			_, n = protowire.ConsumeFixed64(b)
		case protowire.BytesType:
			val, vn := protowire.ConsumeBytes(b)
			if vn < 0 {
				return nil
			}
			if num == target {
				return val
			}
			n = vn
		case protowire.StartGroupType:
			_, n = protowire.ConsumeGroup(num, b)
		default:
			return nil
		}

		if n < 0 {
			return nil
		}
		b = b[n:]
	}
	return nil
}
