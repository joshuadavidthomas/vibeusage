package claude

import (
	"testing"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func TestParsePlanFromTier(t *testing.T) {
	tests := []struct {
		tier string
		want string
	}{
		{"default_claude_max_20x", "Max 20x"},
		{"default_claude_max_5x", "Max 5x"},
		{"default_claude_max", "Max"},
		{"default_claude_pro", "Pro"},
		{"default_claude_free", "Free"},
		{"default_claude_team", "Team"},
		{"default_claude_enterprise", "Enterprise"},
		{"", ""},
		{"unknown_tier", "unknown_tier"},
		{"DEFAULT_CLAUDE_MAX_20X", "Max 20x"},
		{"max_5x", "Max 5x"},
	}

	for _, tt := range tests {
		t.Run(tt.tier, func(t *testing.T) {
			got := parsePlanFromTier(tt.tier)
			if got != tt.want {
				t.Errorf("parsePlanFromTier(%q) = %q, want %q", tt.tier, got, tt.want)
			}
		})
	}
}

func TestEnrichWithAccount(t *testing.T) {
	t.Run("populates email and org from chat membership", func(t *testing.T) {
		snapshot := &models.UsageSnapshot{
			Provider: "claude",
			Identity: &models.ProviderIdentity{Plan: "pro"},
		}
		account := &OAuthAccountResponse{
			EmailAddress: "user@example.com",
			Memberships: []OAuthAccountMembership{
				{
					Organization: OAuthAccountOrganization{
						Name:          "Personal",
						RateLimitTier: "default_claude_max_20x",
						Capabilities:  []string{"chat", "claude_max"},
						BillingType:   "stripe_subscription",
					},
				},
			},
		}

		enrichWithAccount(snapshot, account)

		if snapshot.Identity.Email != "user@example.com" {
			t.Errorf("Email = %q, want %q", snapshot.Identity.Email, "user@example.com")
		}
		if snapshot.Identity.Organization != "Personal" {
			t.Errorf("Organization = %q, want %q", snapshot.Identity.Organization, "Personal")
		}
		if snapshot.Identity.Plan != "Max 20x" {
			t.Errorf("Plan = %q, want %q", snapshot.Identity.Plan, "Max 20x")
		}
	})

	t.Run("prefers chat org over api org", func(t *testing.T) {
		snapshot := &models.UsageSnapshot{
			Provider: "claude",
			Identity: &models.ProviderIdentity{},
		}
		account := &OAuthAccountResponse{
			EmailAddress: "user@example.com",
			Memberships: []OAuthAccountMembership{
				{
					Organization: OAuthAccountOrganization{
						Name:          "Work Corp",
						RateLimitTier: "default_after_self_serve_aisa",
						Capabilities:  []string{"api"},
						BillingType:   "prepaid",
					},
				},
				{
					Organization: OAuthAccountOrganization{
						Name:          "user@example.com's Organization",
						RateLimitTier: "default_claude_max_20x",
						Capabilities:  []string{"chat", "has_chats_from_console", "claude_max"},
						BillingType:   "stripe_subscription",
					},
				},
			},
		}

		enrichWithAccount(snapshot, account)

		if snapshot.Identity.Organization != "user@example.com's Organization" {
			t.Errorf("Organization = %q, want chat org", snapshot.Identity.Organization)
		}
		if snapshot.Identity.Plan != "Max 20x" {
			t.Errorf("Plan = %q, want %q", snapshot.Identity.Plan, "Max 20x")
		}
	})

	t.Run("falls back to first org when none have chat", func(t *testing.T) {
		snapshot := &models.UsageSnapshot{
			Provider: "claude",
			Identity: &models.ProviderIdentity{},
		}
		account := &OAuthAccountResponse{
			EmailAddress: "user@example.com",
			Memberships: []OAuthAccountMembership{
				{
					Organization: OAuthAccountOrganization{
						Name:          "API Org",
						RateLimitTier: "default_after_self_serve_aisa",
						Capabilities:  []string{"api"},
					},
				},
			},
		}

		enrichWithAccount(snapshot, account)

		if snapshot.Identity.Organization != "API Org" {
			t.Errorf("Organization = %q, want fallback to first org", snapshot.Identity.Organization)
		}
	})

	t.Run("creates identity when nil", func(t *testing.T) {
		snapshot := &models.UsageSnapshot{Provider: "claude"}
		account := &OAuthAccountResponse{
			EmailAddress: "user@example.com",
		}

		enrichWithAccount(snapshot, account)

		if snapshot.Identity == nil {
			t.Fatal("expected Identity to be created")
		}
		if snapshot.Identity.Email != "user@example.com" {
			t.Errorf("Email = %q, want %q", snapshot.Identity.Email, "user@example.com")
		}
	})

	t.Run("nil account is no-op", func(t *testing.T) {
		snapshot := &models.UsageSnapshot{
			Provider: "claude",
			Identity: &models.ProviderIdentity{Plan: "pro"},
		}

		enrichWithAccount(snapshot, nil)

		if snapshot.Identity.Plan != "pro" {
			t.Errorf("Plan = %q, want %q", snapshot.Identity.Plan, "pro")
		}
	})

	t.Run("empty memberships preserves email only", func(t *testing.T) {
		snapshot := &models.UsageSnapshot{
			Provider: "claude",
			Identity: &models.ProviderIdentity{Plan: "pro"},
		}
		account := &OAuthAccountResponse{
			EmailAddress: "user@example.com",
			Memberships:  []OAuthAccountMembership{},
		}

		enrichWithAccount(snapshot, account)

		if snapshot.Identity.Email != "user@example.com" {
			t.Errorf("Email = %q, want %q", snapshot.Identity.Email, "user@example.com")
		}
		if snapshot.Identity.Plan != "pro" {
			t.Errorf("Plan should be preserved as %q, got %q", "pro", snapshot.Identity.Plan)
		}
	})

	t.Run("empty tier preserves existing plan", func(t *testing.T) {
		snapshot := &models.UsageSnapshot{
			Provider: "claude",
			Identity: &models.ProviderIdentity{Plan: "pro"},
		}
		account := &OAuthAccountResponse{
			Memberships: []OAuthAccountMembership{
				{
					Organization: OAuthAccountOrganization{
						Name:          "Work",
						RateLimitTier: "",
					},
				},
			},
		}

		enrichWithAccount(snapshot, account)

		if snapshot.Identity.Plan != "pro" {
			t.Errorf("Plan = %q, want %q", snapshot.Identity.Plan, "pro")
		}
		if snapshot.Identity.Organization != "Work" {
			t.Errorf("Organization = %q, want %q", snapshot.Identity.Organization, "Work")
		}
	})
}

func TestFindChatOrg(t *testing.T) {
	t.Run("nil for empty memberships", func(t *testing.T) {
		if got := findChatOrg(nil); got != nil {
			t.Errorf("findChatOrg(nil) = %+v, want nil", got)
		}
		if got := findChatOrg([]OAuthAccountMembership{}); got != nil {
			t.Errorf("findChatOrg([]) = %+v, want nil", got)
		}
	})

	t.Run("returns chat org when present", func(t *testing.T) {
		memberships := []OAuthAccountMembership{
			{Organization: OAuthAccountOrganization{Name: "API", Capabilities: []string{"api"}}},
			{Organization: OAuthAccountOrganization{Name: "Chat", Capabilities: []string{"chat", "claude_max"}}},
		}
		got := findChatOrg(memberships)
		if got == nil || got.Name != "Chat" {
			t.Errorf("findChatOrg() = %+v, want Chat org", got)
		}
	})

	t.Run("falls back to first when no chat", func(t *testing.T) {
		memberships := []OAuthAccountMembership{
			{Organization: OAuthAccountOrganization{Name: "API", Capabilities: []string{"api"}}},
		}
		got := findChatOrg(memberships)
		if got == nil || got.Name != "API" {
			t.Errorf("findChatOrg() = %+v, want API org as fallback", got)
		}
	})
}
