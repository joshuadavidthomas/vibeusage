package device

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPollContext_ParentCancellation(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	ctx, cancelPoll := PollContext(parent)
	defer cancelPoll()

	cancelParent()

	select {
	case <-ctx.Done():
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatalf("PollContext error = %v, want context.Canceled", ctx.Err())
		}
	case <-time.After(time.Second):
		t.Fatal("PollContext was not cancelled with its parent")
	}
}

func TestEnterWaiter_PreCanceledContextDoesNotStartRead(t *testing.T) {
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()
	waiter := enterWaiter{reader: reader}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waiter.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %v, want context.Canceled", err)
	}
	waiter.mu.Lock()
	defer waiter.mu.Unlock()
	if waiter.done != nil {
		t.Fatal("pre-canceled wait started a stdin read")
	}
}

func TestEnterWaiter_ReusesPendingReadAfterCancellation(t *testing.T) {
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close() }()
	defer func() { _ = writer.Close() }()
	waiter := enterWaiter{reader: reader}

	pending := waiter.nextRead()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waiter.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("wait error = %v, want context.Canceled", err)
	}
	if reused := waiter.nextRead(); reused != pending {
		t.Fatal("waiter started a competing read after cancellation")
	}

	if _, err := writer.Write([]byte("\n")); err != nil {
		t.Fatalf("write newline: %v", err)
	}
	select {
	case <-pending:
	case <-time.After(time.Second):
		t.Fatal("pending read did not finish")
	}
}

func TestRun_ParentCancellationInterruptsInitialRequest(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-requestStarted
		cancel()
	}()

	_, err := Run(ctx, io.Discard, true, Config{
		DeviceCodeURL: server.URL,
		HTTPTimeout:   30,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run error = %v, want wrapped context.Canceled", err)
	}
	if err == context.Canceled {
		t.Fatal("Run should add device-code request context to the cancellation error")
	}
}

func TestDeviceCodeResponse_Unmarshal(t *testing.T) {
	raw := `{
		"device_code": "dc-123",
		"user_code": "ABCD1234",
		"verification_uri": "https://github.com/login/device",
		"interval": 5
	}`

	var resp deviceCodeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.DeviceCode != "dc-123" {
		t.Errorf("device_code = %q, want %q", resp.DeviceCode, "dc-123")
	}
	if resp.UserCode != "ABCD1234" {
		t.Errorf("user_code = %q, want %q", resp.UserCode, "ABCD1234")
	}
	if resp.VerificationURI != "https://github.com/login/device" {
		t.Errorf("verification_uri = %q, want %q", resp.VerificationURI, "https://github.com/login/device")
	}
	if resp.Interval != 5 {
		t.Errorf("interval = %v, want 5", resp.Interval)
	}
}

func TestDeviceCodeResponse_UnmarshalDefaultInterval(t *testing.T) {
	raw := `{"device_code": "dc", "user_code": "UC", "verification_uri": "https://example.com"}`

	var resp deviceCodeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.Interval != 0 {
		t.Errorf("interval = %v, want 0 (default)", resp.Interval)
	}
}

func TestDeviceCodeResponse_UnmarshalComplete(t *testing.T) {
	raw := `{
		"user_code": "ABCD-1234",
		"device_code": "dc-123",
		"verification_uri_complete": "https://auth.kimi.com/device?code=ABCD-1234",
		"interval": 5,
		"expires_in": 600
	}`

	var resp deviceCodeResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.VerificationURIComplete != "https://auth.kimi.com/device?code=ABCD-1234" {
		t.Errorf("verification_uri_complete = %q", resp.VerificationURIComplete)
	}
	if resp.ExpiresIn != 600 {
		t.Errorf("expires_in = %d, want 600", resp.ExpiresIn)
	}
}

func TestTokenResponse_UnmarshalSuccess(t *testing.T) {
	raw := `{
		"access_token": "gho_xxxx",
		"token_type": "bearer",
		"scope": "read:user"
	}`

	var resp tokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "gho_xxxx" {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "gho_xxxx")
	}
	if resp.RefreshToken != "" {
		t.Errorf("refresh_token = %q, want empty", resp.RefreshToken)
	}
	if resp.ExpiresIn != 0 {
		t.Errorf("expires_in = %v, want 0", resp.ExpiresIn)
	}
	if resp.Error != "" {
		t.Errorf("error = %q, want empty", resp.Error)
	}
}

func TestTokenResponse_UnmarshalWithRefresh(t *testing.T) {
	raw := `{
		"access_token": "ghu_xxxx",
		"refresh_token": "ghr_xxxx",
		"expires_in": 28800,
		"refresh_token_expires_in": 15897600,
		"token_type": "bearer",
		"scope": "read:user"
	}`

	var resp tokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "ghu_xxxx" {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "ghu_xxxx")
	}
	if resp.RefreshToken != "ghr_xxxx" {
		t.Errorf("refresh_token = %q, want %q", resp.RefreshToken, "ghr_xxxx")
	}
	if resp.ExpiresIn != 28800 {
		t.Errorf("expires_in = %v, want 28800", resp.ExpiresIn)
	}
}

func TestTokenResponse_UnmarshalError(t *testing.T) {
	raw := `{
		"error": "authorization_pending",
		"error_description": "The authorization request is still pending."
	}`

	var resp tokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "" {
		t.Errorf("access_token = %q, want empty", resp.AccessToken)
	}
	if resp.Error != "authorization_pending" {
		t.Errorf("error = %q, want %q", resp.Error, "authorization_pending")
	}
	if resp.ErrorDesc != "The authorization request is still pending." {
		t.Errorf("error_description = %q, want %q", resp.ErrorDesc, "The authorization request is still pending.")
	}
}

func TestTokenResponse_UnmarshalIntegerExpiresIn(t *testing.T) {
	// KimiCode returns expires_in as an integer; ensure float64 handles it.
	raw := `{
		"access_token": "eyJhbG...",
		"refresh_token": "rt-xxx",
		"expires_in": 900
	}`

	var resp tokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.ExpiresIn != 900 {
		t.Errorf("expires_in = %v, want 900", resp.ExpiresIn)
	}
}

func TestSaveCredentials_WriteFailure(t *testing.T) {
	dataPath := filepath.Join(t.TempDir(), "data")
	if err := os.WriteFile(dataPath, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("VIBEUSAGE_DATA_DIR", dataPath)

	err := saveCredentials(Config{ProviderID: "test-provider", CredType: "oauth"}, tokenResponse{AccessToken: "token"})
	if err == nil {
		t.Fatal("saveCredentials() should return a credential write error")
	}
	if !strings.Contains(err.Error(), "save test-provider credentials") {
		t.Errorf("error should identify the failed credential save, got: %v", err)
	}
}

func TestOAuthCredentials_Roundtrip(t *testing.T) {
	original := oauthCredentials{
		AccessToken:  "ghu_xxxx",
		RefreshToken: "ghr_xxxx",
		ExpiresAt:    "2025-02-20T06:00:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded oauthCredentials
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}
