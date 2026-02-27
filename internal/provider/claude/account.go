package claude

import (
	"strconv"
	"strings"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

// enrichWithAccount enriches a usage snapshot with data from the OAuth
// account endpoint (/api/oauth/account). It populates email, organization
// name, and plan from the chat-capable membership (the one with usage
// limits), falling back to the first membership.
func enrichWithAccount(snapshot *models.UsageSnapshot, account *OAuthAccountResponse) {
	if account == nil {
		return
	}

	if snapshot.Identity == nil {
		snapshot.Identity = &models.ProviderIdentity{}
	}

	if account.EmailAddress != "" {
		snapshot.Identity.Email = account.EmailAddress
	}

	org := findChatOrg(account.Memberships)
	if org == nil {
		return
	}

	if org.Name != "" {
		snapshot.Identity.Organization = org.Name
	}
	if plan := parsePlanFromTier(org.RateLimitTier); plan != "" {
		snapshot.Identity.Plan = plan
	}
}

// findChatOrg returns the organization from the membership list that has
// the "chat" capability (the consumer subscription org). Falls back to
// the first membership if none have "chat".
func findChatOrg(memberships []OAuthAccountMembership) *OAuthAccountOrganization {
	for i := range memberships {
		if memberships[i].Organization.HasCapability("chat") {
			return &memberships[i].Organization
		}
	}
	if len(memberships) > 0 {
		return &memberships[0].Organization
	}
	return nil
}

// parsePlanFromTier converts a rate_limit_tier string to a human-readable
// plan name. Known patterns:
//
//   - "default_claude_max_20x" → "Max 20x"
//   - "default_claude_max_5x"  → "Max 5x"
//   - "default_claude_pro"     → "Pro"
//   - "default_claude_free"    → "Free"
func parsePlanFromTier(tier string) string {
	if tier == "" {
		return ""
	}

	lower := strings.ToLower(tier)
	parts := strings.Split(lower, "_")

	// Look for max plan with multiplier (e.g. "max_20x")
	if strings.Contains(lower, "max") {
		for _, p := range parts {
			if strings.HasSuffix(p, "x") && len(p) > 1 {
				num := strings.TrimSuffix(p, "x")
				if _, err := strconv.Atoi(num); err == nil {
					return "Max " + num + "x"
				}
			}
		}
		return "Max"
	}

	// Check for other known plan types
	for _, p := range parts {
		switch p {
		case "pro":
			return "Pro"
		case "free":
			return "Free"
		case "team":
			return "Team"
		case "enterprise":
			return "Enterprise"
		}
	}

	return tier
}
