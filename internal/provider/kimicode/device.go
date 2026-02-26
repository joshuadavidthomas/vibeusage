package kimicode

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/fetch"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
	"github.com/joshuadavidthomas/vibeusage/internal/provider/deviceflow"
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
		var creds OAuthCredentials
		if err := json.Unmarshal(data, &creds); err != nil {
			continue
		}
		if creds.AccessToken != "" {
			return &creds
		}
	}
	return nil
}

func (s *DeviceFlowStrategy) refreshToken(ctx context.Context, creds *OAuthCredentials) *OAuthCredentials {
	if creds.RefreshToken == "" {
		return nil
	}

	client := httpclient.NewFromConfig(s.HTTPTimeout)
	opts := commonHeaders()

	var tokenResp TokenResponse
	resp, err := client.PostFormCtx(ctx, oauthBaseURL()+tokenPath,
		map[string]string{
			"client_id":     clientID,
			"grant_type":    refreshGrant,
			"refresh_token": creds.RefreshToken,
		},
		&tokenResp,
		opts...,
	)
	if err != nil {
		return nil
	}
	if resp.StatusCode != 200 || resp.JSONErr != nil {
		return nil
	}
	if tokenResp.AccessToken == "" {
		return nil
	}

	updated := &OAuthCredentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
	}
	if tokenResp.ExpiresIn > 0 {
		updated.ExpiresAt = float64(time.Now().UTC().Unix() + int64(tokenResp.ExpiresIn))
	}
	if updated.RefreshToken == "" {
		updated.RefreshToken = creds.RefreshToken
	}

	content, _ := json.Marshal(updated)
	_ = config.WriteCredential(config.CredentialPath("kimicode", "oauth"), content)

	return updated
}

// RunDeviceFlow runs the interactive Kimi device flow for auth.
// Output is written to w, allowing callers to control where messages go.
func RunDeviceFlow(w io.Writer, quiet bool) (bool, error) {
	client := httpclient.NewFromConfig(config.Get().Fetch.Timeout)
	opts := commonHeaders()

	// Request device code
	var dcResp DeviceCodeResponse
	resp, err := client.PostFormCtx(context.Background(), oauthBaseURL()+deviceCodePath,
		map[string]string{
			"client_id": clientID,
		},
		&dcResp,
		opts...,
	)
	if err != nil {
		return false, fmt.Errorf("failed to request device code: %w", err)
	}
	if resp.JSONErr != nil {
		return false, fmt.Errorf("invalid device code response: %w", resp.JSONErr)
	}

	deviceCode := dcResp.DeviceCode
	userCode := dcResp.UserCode
	verificationURI := dcResp.VerificationURIComplete
	interval := dcResp.Interval
	if interval == 0 {
		interval = 5
	}

	// Display instructions
	if !quiet {
		deviceflow.WriteOpening(w, verificationURI)
		deviceflow.WriteWaiting(w)
	} else {
		_, _ = fmt.Fprintln(w, verificationURI)
		_, _ = fmt.Fprintf(w, "Code: %s\n", userCode)
	}

	// Try to open browser
	deviceflow.OpenBrowser(verificationURI)

	// Poll for token
	for attempt := 0; attempt < 120; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(interval) * time.Second)
		}

		var tokenResp TokenResponse
		pollResp, err := client.PostFormCtx(context.Background(), oauthBaseURL()+tokenPath,
			map[string]string{
				"client_id":   clientID,
				"device_code": deviceCode,
				"grant_type":  deviceFlowGrant,
			},
			&tokenResp,
			opts...,
		)
		if err != nil {
			if attempt < 3 {
				continue
			}
			return false, fmt.Errorf("network error: %w", err)
		}
		if pollResp.JSONErr != nil {
			continue
		}

		if tokenResp.AccessToken != "" {
			creds := OAuthCredentials{
				AccessToken:  tokenResp.AccessToken,
				RefreshToken: tokenResp.RefreshToken,
			}
			if tokenResp.ExpiresIn > 0 {
				creds.ExpiresAt = float64(time.Now().UTC().Unix() + int64(tokenResp.ExpiresIn))
			}
			content, _ := json.Marshal(creds)
			_ = config.WriteCredential(config.CredentialPath("kimicode", "oauth"), content)
			if !quiet {
				deviceflow.WriteSuccess(w)
			}
			return true, nil
		}

		switch tokenResp.Error {
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5
			continue
		case "expired_token":
			if !quiet {
				deviceflow.WriteExpired(w)
			}
			return false, nil
		case "access_denied":
			if !quiet {
				deviceflow.WriteDenied(w)
			}
			return false, nil
		default:
			desc := tokenResp.ErrorDesc
			if desc == "" {
				desc = tokenResp.Error
			}
			if desc != "" {
				return false, fmt.Errorf("authentication error: %s", desc)
			}
		}
	}

	if !quiet {
		deviceflow.WriteTimeout(w)
	}
	return false, nil
}

