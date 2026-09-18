package antigravity

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/logging"
)

const (
	testClientID     = "test-client-id"
	testClientSecret = "test-client-secret"
)

// setTestClientCredentials configures the OAuth client that the auth flow reads
// from the environment, and returns the credentials the flow should use.
func setTestClientCredentials(t *testing.T) oauthClientCredentials {
	t.Helper()

	t.Setenv(clientIDEnvVar, testClientID)
	t.Setenv(clientSecretEnvVar, testClientSecret)
	return oauthClientCredentials{ID: testClientID, Secret: testClientSecret}
}

func TestRunAuthFlow_ReturnsParentCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RunAuthFlow(ctx, io.Discard, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunAuthFlow error = %v, want wrapped context.Canceled", err)
	}
	if err == context.Canceled {
		t.Fatal("RunAuthFlow should add authorization flow context to the cancellation error")
	}
}

func TestOAuthCallbackHandler_FirstResultWinsWithoutBlockingRepeats(t *testing.T) {
	resultCh := make(chan callbackResult, 1)
	handler := newOAuthCallbackHandler("expected-state", resultCh)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/?state=expected-state&code=first", nil))

	secondDone := make(chan struct{})
	go func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/?state=expected-state&code=second", nil))
		close(secondDone)
	}()

	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("repeated callback blocked while the first result was queued")
	}

	result := <-resultCh
	if result.err != nil {
		t.Fatalf("first callback returned error: %v", result.err)
	}
	if result.code != "first" {
		t.Errorf("callback code = %q, want first callback code", result.code)
	}

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/?state=expected-state&code=third", nil))
	select {
	case extra := <-resultCh:
		t.Fatalf("later callback produced an extra result: %+v", extra)
	default:
	}
}

func TestOAuthCallbackHandlerRejectsInvalidStateWithoutCompleting(t *testing.T) {
	resultCh := make(chan callbackResult, 1)
	handler := newOAuthCallbackHandler("expected-state", resultCh)

	for _, requestURL := range []string{"/?code=wrong", "/?state=wrong-state&code=wrong"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest("GET", requestURL, nil))

		if recorder.Code != http.StatusNotFound {
			t.Errorf("callback %q status = %d, want %d", requestURL, recorder.Code, http.StatusNotFound)
		}
		if recorder.Body.Len() != 0 {
			t.Errorf("callback %q returned a response body", requestURL)
		}
		select {
		case result := <-resultCh:
			t.Fatalf("callback %q completed with %+v", requestURL, result)
		default:
		}
	}

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/?state=expected-state&code=correct", nil))
	result := receiveCallbackResult(t, resultCh)
	if result.err != nil || result.code != "correct" {
		t.Errorf("correct-state callback result = %+v, want code correct", result)
	}
}

func TestOAuthCallbackHandlerCompletesWithProviderError(t *testing.T) {
	resultCh := make(chan callbackResult, 1)
	handler := newOAuthCallbackHandler("expected-state", resultCh)

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/?state=expected-state&error=access_denied", nil))

	result := receiveCallbackResult(t, resultCh)
	if result.err == nil {
		t.Fatal("callback error = nil, want authorization error")
	}
	if result.err.Error() != "authorization failed: access_denied" {
		t.Errorf("callback error = %q, want authorization failed: access_denied", result.err)
	}
}

func TestGenerateOAuthMaterialCreatesS256Challenge(t *testing.T) {
	state, verifier, challenge, err := generateOAuthMaterial()
	if err != nil {
		t.Fatalf("generateOAuthMaterial() error = %v", err)
	}

	for name, value := range map[string]string{"state": state, "verifier": verifier} {
		decoded, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			t.Errorf("%s is not base64url: %v", name, err)
		}
		if len(decoded) != 32 {
			t.Errorf("%s decoded length = %d, want 32", name, len(decoded))
		}
		if strings.Contains(value, "=") {
			t.Errorf("%s has base64 padding", name)
		}
	}

	sum := sha256.Sum256([]byte(verifier))
	wantChallenge := base64.RawURLEncoding.EncodeToString(sum[:])
	if challenge != wantChallenge {
		t.Errorf("challenge = %q, want %q", challenge, wantChallenge)
	}
	if strings.Contains(challenge, "=") {
		t.Error("challenge has base64 padding")
	}
}

func TestTokenExchangeFormIncludesVerifier(t *testing.T) {
	oauthClient := setTestClientCredentials(t)
	form := tokenExchangeForm("authorization-code", "http://localhost:8080", "code-verifier", oauthClient)

	for key, want := range map[string]string{
		"grant_type":    "authorization_code",
		"code":          "authorization-code",
		"redirect_uri":  "http://localhost:8080",
		"client_id":     oauthClient.ID,
		"client_secret": oauthClient.Secret,
		"code_verifier": "code-verifier",
	} {
		if got := form[key]; got != want {
			t.Errorf("token form %s = %q, want %q", key, got, want)
		}
	}
	if _, ok := form["state"]; ok {
		t.Error("token form includes state")
	}
	if _, ok := form["code_challenge"]; ok {
		t.Error("token form includes code_challenge")
	}
}

func TestBuildAuthorizationURLIncludesStateAndPKCE(t *testing.T) {
	oauthClient := setTestClientCredentials(t)
	authURL, err := url.Parse(buildAuthorizationURL("http://localhost:8080", "state-value", "challenge-value", oauthClient))
	if err != nil {
		t.Fatalf("parse authorization URL: %v", err)
	}

	query := authURL.Query()
	for key, want := range map[string]string{
		"client_id":             oauthClient.ID,
		"redirect_uri":          "http://localhost:8080",
		"response_type":         "code",
		"scope":                 oauthScopes,
		"state":                 "state-value",
		"code_challenge":        "challenge-value",
		"code_challenge_method": "S256",
	} {
		if got := query.Get(key); got != want {
			t.Errorf("authorization URL %s = %q, want %q", key, got, want)
		}
	}
	if strings.Contains(authURL.RawQuery, " ") {
		t.Error("authorization URL query contains unescaped spaces")
	}
	if query.Get("code_verifier") != "" {
		t.Error("authorization URL exposes the code verifier")
	}
}

func receiveCallbackResult(t *testing.T, resultCh <-chan callbackResult) callbackResult {
	t.Helper()

	select {
	case result := <-resultCh:
		return result
	case <-time.After(time.Second):
		t.Fatal("callback did not complete")
		return callbackResult{}
	}
}

func TestClientCredentials_RequiresBothValues(t *testing.T) {
	tests := []struct {
		name         string
		clientID     string
		clientSecret string
		wantNamed    []string
		wantAbsent   []string
	}{
		{name: "neither configured", wantNamed: []string{clientIDEnvVar, clientSecretEnvVar}},
		{name: "only client id", clientID: "client-id", wantNamed: []string{clientSecretEnvVar}, wantAbsent: []string{clientIDEnvVar}},
		{name: "only client secret", clientSecret: "client-secret", wantNamed: []string{clientIDEnvVar}, wantAbsent: []string{clientSecretEnvVar}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(clientIDEnvVar, tt.clientID)
			t.Setenv(clientSecretEnvVar, tt.clientSecret)

			_, err := clientCredentials()
			if err == nil {
				t.Fatal("clientCredentials() error = nil, want an error for incomplete configuration")
			}
			for _, envVar := range tt.wantNamed {
				if !strings.Contains(err.Error(), envVar) {
					t.Errorf("clientCredentials() error = %q, want it to name missing %s", err, envVar)
				}
			}
			for _, envVar := range tt.wantAbsent {
				if strings.Contains(err.Error(), envVar) {
					t.Errorf("clientCredentials() error = %q, want it to omit configured %s", err, envVar)
				}
			}
		})
	}
}

func TestClientCredentials_TrimsWhitespace(t *testing.T) {
	t.Setenv(clientIDEnvVar, "  client-id\n")
	t.Setenv(clientSecretEnvVar, "\tclient-secret ")

	oauthClient, err := clientCredentials()
	if err != nil {
		t.Fatalf("clientCredentials() error = %v", err)
	}
	if oauthClient.ID != "client-id" {
		t.Errorf("client id = %q, want %q", oauthClient.ID, "client-id")
	}
	if oauthClient.Secret != "client-secret" {
		t.Errorf("client secret = %q, want %q", oauthClient.Secret, "client-secret")
	}
}

func TestRunAuthFlow_FailsBeforeReachingGoogleWithoutClientCredentials(t *testing.T) {
	t.Setenv(clientIDEnvVar, "")
	t.Setenv(clientSecretEnvVar, "")

	var out bytes.Buffer
	done, err := RunAuthFlow(context.Background(), &out, false)
	if err == nil {
		t.Fatal("RunAuthFlow() error = nil, want a missing-credential error")
	}
	for _, envVar := range []string{clientIDEnvVar, clientSecretEnvVar} {
		if !strings.Contains(err.Error(), envVar) {
			t.Errorf("RunAuthFlow() error = %q, want it to name %s", err, envVar)
		}
	}
	if done {
		t.Error("RunAuthFlow() reported success without client credentials")
	}
	if out.Len() != 0 {
		t.Errorf("RunAuthFlow() wrote %q, want no output before the credential check", out.String())
	}
}

func TestRunAuthFlow_LogsBlockedConfiguration(t *testing.T) {
	t.Setenv(clientIDEnvVar, "")
	t.Setenv(clientSecretEnvVar, "")

	ctx, logs := logging.NewTestContext(logging.Flags{Verbose: true})
	done, err := RunAuthFlow(ctx, io.Discard, true)
	if err == nil {
		t.Fatal("RunAuthFlow() error = nil, want a missing-credential error")
	}
	if done {
		t.Error("RunAuthFlow() reported success without client credentials")
	}
	for _, want := range []string{
		"antigravity auth flow blocked by OAuth client configuration",
		"antigravity",
		clientIDEnvVar,
		clientSecretEnvVar,
	} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("log output %q does not contain %q", logs.String(), want)
		}
	}
}
