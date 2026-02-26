package kimicode

import (
	"context"
	"encoding/json"
	"os"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/oauth"
)

const (
	oauthHost       = "https://auth.kimi.com"
	deviceCodePath  = "/api/oauth/device_authorization"
	tokenPath       = "/api/oauth/token"
	clientID        = "17e5f671-d194-4dfb-9706-5516cb48c098"
	deviceFlowGrant = "urn:ietf:params:oauth:grant-type:device_code"
	refreshGrant    = "refresh_token"
)

// oauthBaseURL returns the OAuth host, respecting the KIMI_CODE_OAUTH_HOST override.
func oauthBaseURL() string {
	if host := os.Getenv("KIMI_CODE_OAUTH_HOST"); host != "" {
		return host
	}
	return oauthHost
}

// DeviceFlowStrategy fetches Kimi usage using OAuth device flow credentials.
type DeviceFlowStrategy struct {
	HTTPTimeout float64
}

func (s *DeviceFlowStrategy) IsAvailable() bool {
	for _, p := range s.credentialPaths() {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func (s *DeviceFlowStrategy) credentialPaths() []string {
	return []string{config.CredentialPath("kimicode", "oauth")}
}

func (s *DeviceFlowStrategy) Fetch(ctx context.Context) (fetch.FetchResult, error) {
	creds := s.loadCredentials()
	if creds == nil {
		return fetch.ResultFail("No OAuth credentials found. Run `vibeusage auth kimicode` to authenticate."), nil
	}

	if creds.AccessToken == "" {
		return fetch.ResultFail("Invalid credentials: missing access_token"), nil
	}

	if creds.NeedsRefresh() {
		refreshed := s.refreshToken(ctx, creds)
		if refreshed == nil {
			return fetch.ResultFail("Failed to refresh token. Run `vibeusage auth kimicode` to re-authenticate."), nil
		}
		creds = refreshed
	}

	return fetchUsage(ctx, creds.AccessToken, "device_flow", s.HTTPTimeout)
}

func (s *DeviceFlowStrategy) loadCredentials() *OAuthCredentials {
	for _, path := range s.credentialPaths() {
		data, err := config.ReadCredential(path)
		if err != nil || data == nil {
			continue
		}

		// Try current RFC3339 format first
		var creds OAuthCredentials
		if err := json.Unmarshal(data, &creds); err == nil && creds.AccessToken != "" {
			// If ExpiresAt looks like a number string, it might be a
			// partially-migrated legacy credential; skip to migration.
			if _, parseErr := time.Parse(time.RFC3339, creds.ExpiresAt); creds.ExpiresAt == "" || parseErr == nil {
				return &creds
			}
		}

		// Try legacy float64 timestamp format and migrate
		if migrated := migrateCredentials(data); migrated != nil {
			// Write back in the new format so future loads are fast
			if content, err := json.Marshal(migrated); err == nil {
				_ = config.WriteCredential(path, content)
			}
			return migrated
		}
	}
	return nil
}

func (s *DeviceFlowStrategy) refreshToken(ctx context.Context, creds *OAuthCredentials) *OAuthCredentials {
	return oauth.Refresh(ctx, creds.RefreshToken, oauth.RefreshConfig{
		TokenURL:    oauthBaseURL() + tokenPath,
		FormFields:  map[string]string{"client_id": clientID},
		Headers:     commonHeaders(),
		ProviderID:  "kimicode",
		HTTPTimeout: s.HTTPTimeout,
	})
}
