package device

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
)

// Config describes the parameters for a standard OAuth device code flow.
// Providers supply a Config and the deviceflow package handles the entire
// lifecycle: request code → display → wait → poll → save credentials.
type Config struct {
	// DeviceCodeURL is the endpoint that returns a device code and user code.
	DeviceCodeURL string
	// DeviceCodeParams are sent as form fields when requesting the device code.
	DeviceCodeParams map[string]string
	// TokenURL is the endpoint polled for an access token.
	TokenURL string
	// TokenParams are sent as form fields when polling for a token.
	// The device_code field is added automatically.
	TokenParams map[string]string
	// HTTPOptions are applied to every HTTP request (e.g. Accept headers).
	HTTPOptions []httpclient.RequestOption
	// HTTPTimeout is the HTTP client timeout in seconds.
	HTTPTimeout float64

	// ManualCodeEntry means the user must copy the code and press Enter
	// before the browser opens (e.g. GitHub requires manual code entry).
	// When false, the verification URI already contains the code and the
	// browser opens automatically.
	ManualCodeEntry bool
	// FormatCode optionally reformats the user code for display
	// (e.g. adding a dash: "36F71B5E" → "36F7-1B5E").
	FormatCode func(code string) string

	// CredentialPath is where the OAuth credentials are saved.
	CredentialPath string
	// ShowRefreshHint prints "Token will refresh automatically." on success
	// when a refresh token is present.
	ShowRefreshHint bool
}

// deviceCodeResponse is the common shape returned by device code endpoints.
type deviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval,omitempty"`
	ExpiresIn               int    `json:"expires_in,omitempty"`
}

// tokenResponse is the common shape returned by token polling endpoints.
type tokenResponse struct {
	AccessToken  string  `json:"access_token,omitempty"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	ExpiresIn    float64 `json:"expires_in,omitempty"`
	TokenType    string  `json:"token_type,omitempty"`
	Scope        string  `json:"scope,omitempty"`
	Error        string  `json:"error,omitempty"`
	ErrorDesc    string  `json:"error_description,omitempty"`
}

// oauthCredentials is the credential format saved to disk.
// Compatible with oauth.Credentials.
type oauthCredentials struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

// Run executes a standard OAuth device code flow.
// Returns true on success, false on user cancellation or timeout.
func Run(w io.Writer, quiet bool, cfg Config) (bool, error) {
	client := httpclient.NewFromConfig(cfg.HTTPTimeout)

	// Request device code.
	var dcResp deviceCodeResponse
	resp, err := client.PostForm(cfg.DeviceCodeURL, cfg.DeviceCodeParams, &dcResp, cfg.HTTPOptions...)
	if err != nil {
		return false, fmt.Errorf("failed to request device code: %w", err)
	}
	if resp.JSONErr != nil {
		return false, fmt.Errorf("invalid device code response: %w", resp.JSONErr)
	}

	deviceCode := dcResp.DeviceCode
	userCode := dcResp.UserCode
	interval := dcResp.Interval
	if interval == 0 {
		interval = 5
	}

	// Pick the right verification URI.
	verificationURI := dcResp.VerificationURIComplete
	if verificationURI == "" {
		verificationURI = dcResp.VerificationURI
	}
	// For manual code entry, use the base URI (user enters code on the page).
	openURI := verificationURI
	if cfg.ManualCodeEntry && dcResp.VerificationURI != "" {
		openURI = dcResp.VerificationURI
	}

	// Format user code for display.
	displayCode := userCode
	if cfg.FormatCode != nil {
		displayCode = cfg.FormatCode(userCode)
	}

	// Display instructions.
	if !quiet {
		if cfg.ManualCodeEntry {
			_, _ = fmt.Fprintf(w, "Copy this code: %s\n", displayCode)
			_, _ = fmt.Fprintf(w, "Press Enter to open %s", openURI)
			if err := WaitForEnter(); err != nil {
				return false, nil
			}
			WriteOpening(w, openURI)
		} else {
			WriteOpening(w, openURI)
		}
		WriteWaiting(w)
	} else {
		_, _ = fmt.Fprintln(w, openURI)
		_, _ = fmt.Fprintf(w, "Code: %s\n", displayCode)
		OpenBrowser(openURI)
	}

	// Build token params with device code.
	tokenParams := make(map[string]string, len(cfg.TokenParams)+1)
	for k, v := range cfg.TokenParams {
		tokenParams[k] = v
	}
	tokenParams["device_code"] = deviceCode

	ctx, cancel := PollContext()
	defer cancel()

	// Poll for token.
	first := true
	for {
		if !first {
			if !PollWait(ctx, interval) {
				break
			}
		}
		first = false

		var tokenResp tokenResponse
		pollResp, err := client.PostForm(cfg.TokenURL, tokenParams, &tokenResp, cfg.HTTPOptions...)
		if err != nil {
			continue
		}
		if pollResp.JSONErr != nil {
			continue
		}

		if tokenResp.AccessToken != "" {
			creds := oauthCredentials{
				AccessToken:  tokenResp.AccessToken,
				RefreshToken: tokenResp.RefreshToken,
			}
			if tokenResp.ExpiresIn > 0 {
				creds.ExpiresAt = time.Now().UTC().Add(
					time.Duration(tokenResp.ExpiresIn) * time.Second,
				).Format(time.RFC3339)
			}
			content, _ := json.Marshal(creds)
			_ = config.WriteCredential(cfg.CredentialPath, content)

			if !quiet {
				WriteSuccess(w)
				if cfg.ShowRefreshHint && tokenResp.RefreshToken != "" {
					_, _ = fmt.Fprintln(w, "  Token will refresh automatically.")
				}
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
				WriteExpired(w)
			}
			return false, nil
		case "access_denied":
			if !quiet {
				WriteDenied(w)
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
		WriteTimeout(w)
	}
	return false, nil
}
