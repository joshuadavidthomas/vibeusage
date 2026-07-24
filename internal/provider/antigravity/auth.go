package antigravity

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/auth/device"
	"github.com/joshuadavidthomas/vibeusage/internal/auth/google"
	"github.com/joshuadavidthomas/vibeusage/internal/auth/oauth"
	"github.com/joshuadavidthomas/vibeusage/internal/config"
	"github.com/joshuadavidthomas/vibeusage/internal/httpclient"
)

// Scopes needed for the Antigravity quota and user-info APIs.
const oauthScopes = "openid https://www.googleapis.com/auth/cloud-platform https://www.googleapis.com/auth/userinfo.email"

type callbackResult struct {
	code string
	err  error
}

// RunAuthFlow runs an interactive localhost-redirect OAuth flow to obtain
// a refresh token that vibeusage can use independently of the Antigravity IDE.
// Output is written to w, allowing callers to control where messages go.
func RunAuthFlow(parent context.Context, w io.Writer, quiet bool) (bool, error) {
	if err := parent.Err(); err != nil {
		return false, fmt.Errorf("starting authorization flow: %w", err)
	}

	// Start a local HTTP server on a random port to receive the redirect.
	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return false, fmt.Errorf("failed to start local server: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://localhost:%d", port)

	// Channel to receive the authorization code from the callback.
	resultCh := make(chan callbackResult, 1)

	mux := http.NewServeMux()
	mux.Handle("/", newOAuthCallbackHandler(resultCh))

	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	// Build the authorization URL.
	authURL := fmt.Sprintf(
		"https://accounts.google.com/o/oauth2/v2/auth?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&access_type=offline&prompt=consent",
		antigravityClientID, redirectURI, oauthScopes,
	)

	if !quiet {
		device.WriteOpening(w, authURL)
		device.WriteWaiting(w)
	} else {
		_, _ = fmt.Fprintln(w, authURL)
	}

	device.OpenBrowser(authURL)

	pollCtx, cancel := device.PollContext(parent)
	defer cancel()

	// Wait for the callback or timeout/interrupt.
	select {
	case result := <-resultCh:
		if parent.Err() != nil {
			return false, fmt.Errorf("waiting for authorization callback: %w", parent.Err())
		}
		if result.err != nil {
			if !quiet {
				_, _ = fmt.Fprintf(w, "\n  ✗ %s\n", result.err)
			}
			return false, nil
		}
		return exchangeCode(parent, w, result.code, redirectURI, quiet)
	case <-pollCtx.Done():
		if parent.Err() != nil {
			return false, fmt.Errorf("waiting for authorization callback: %w", parent.Err())
		}
		if !quiet {
			device.WriteTimeout(w)
		}
		return false, nil
	}
}

func newOAuthCallbackHandler(resultCh chan<- callbackResult) http.Handler {
	var resultOnce sync.Once
	complete := func(result callbackResult) {
		resultOnce.Do(func() {
			resultCh <- result
		})
	}

	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		errParam := r.URL.Query().Get("error")

		if errParam != "" {
			desc := r.URL.Query().Get("error_description")
			if desc == "" {
				desc = errParam
			}
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprintf(rw, "<html><body><h2>Authorization failed</h2><p>%s</p><p>You can close this tab.</p></body></html>", desc)
			complete(callbackResult{err: fmt.Errorf("authorization failed: %s", desc)})
			return
		}

		if code == "" {
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = fmt.Fprint(rw, "<html><body><h2>Missing authorization code</h2><p>You can close this tab.</p></body></html>")
			complete(callbackResult{err: fmt.Errorf("no authorization code in redirect")})
			return
		}

		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(rw, "<html><body><h2>✓ Authorization successful!</h2><p>You can close this tab and return to the terminal.</p></body></html>")
		complete(callbackResult{code: code})
	})
}

// exchangeCode exchanges the authorization code for tokens and saves them.
func exchangeCode(ctx context.Context, w io.Writer, code, redirectURI string, quiet bool) (bool, error) {
	client := httpclient.NewFromConfig(config.Get().Fetch.Timeout)

	var tokenResp google.TokenResponse
	resp, err := client.PostFormCtx(ctx, google.TokenURL,
		map[string]string{
			"grant_type":    "authorization_code",
			"code":          code,
			"redirect_uri":  redirectURI,
			"client_id":     antigravityClientID,
			"client_secret": antigravityClientSecret,
		},
		&tokenResp,
	)
	if err != nil {
		if ctx.Err() != nil {
			return false, fmt.Errorf("token exchange failed: %w", ctx.Err())
		}
		return false, fmt.Errorf("token exchange failed: %w", err)
	}
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("token exchange failed: HTTP %d: %s", resp.StatusCode, google.ExtractAPIError(resp.Body))
	}
	if resp.JSONErr != nil {
		return false, fmt.Errorf("invalid token response: %w", resp.JSONErr)
	}
	if tokenResp.AccessToken == "" {
		return false, fmt.Errorf("token exchange returned empty access token")
	}

	creds := &oauth.Credentials{
		AccessToken:  tokenResp.AccessToken,
		RefreshToken: tokenResp.RefreshToken,
	}
	if tokenResp.ExpiresIn > 0 {
		creds.ExpiresAt = time.Now().UTC().Add(time.Duration(tokenResp.ExpiresIn) * time.Second).Format(time.RFC3339)
	}

	if err := saveAntigravityCredentials(creds); err != nil {
		return false, fmt.Errorf("failed to save credentials: %w", err)
	}

	if !quiet {
		device.WriteSuccess(w)
		if tokenResp.RefreshToken != "" {
			_, _ = fmt.Fprintln(w, "  Token will refresh automatically — no need to open the IDE.")
		}
	}

	return true, nil
}
