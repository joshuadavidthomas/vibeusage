package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCredentials_NeedsRefresh(t *testing.T) {
	tests := []struct {
		name  string
		creds Credentials
		want  bool
	}{
		{
			name:  "no expiry",
			creds: Credentials{AccessToken: "tok"},
			want:  false,
		},
		{
			name: "far future",
			creds: Credentials{
				AccessToken: "tok",
				ExpiresAt:   "2099-01-01T00:00:00Z",
			},
			want: false,
		},
		{
			name: "expired",
			creds: Credentials{
				AccessToken: "tok",
				ExpiresAt:   "2020-01-01T00:00:00Z",
			},
			want: true,
		},
		{
			name: "invalid date",
			creds: Credentials{
				AccessToken: "tok",
				ExpiresAt:   "garbage",
			},
			want: true,
		},
		{
			name: "within buffer",
			creds: Credentials{
				AccessToken: "tok",
				ExpiresAt:   time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339),
			},
			want: true,
		},
		{
			name: "outside buffer",
			creds: Credentials{
				AccessToken: "tok",
				ExpiresAt:   time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.creds.NeedsRefresh()
			if got != tt.want {
				t.Errorf("NeedsRefresh() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCredentials_Roundtrip(t *testing.T) {
	original := Credentials{
		AccessToken:  "my-token",
		RefreshToken: "my-refresh",
		ExpiresAt:    "2025-02-20T00:00:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Credentials
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded != original {
		t.Errorf("roundtrip mismatch: got %+v, want %+v", decoded, original)
	}
}

func TestTokenResponse_Unmarshal(t *testing.T) {
	raw := `{
		"access_token": "new-token",
		"refresh_token": "new-refresh",
		"expires_in": 3600
	}`

	var resp TokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "new-token" {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "new-token")
	}
	if resp.RefreshToken != "new-refresh" {
		t.Errorf("refresh_token = %q, want %q", resp.RefreshToken, "new-refresh")
	}
	if resp.ExpiresIn != 3600 {
		t.Errorf("expires_in = %v, want 3600", resp.ExpiresIn)
	}
}

func TestTokenResponse_UnmarshalMinimal(t *testing.T) {
	raw := `{"access_token": "tok"}`

	var resp TokenResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if resp.AccessToken != "tok" {
		t.Errorf("access_token = %q, want %q", resp.AccessToken, "tok")
	}
	if resp.ExpiresIn != 0 {
		t.Errorf("expires_in = %v, want 0", resp.ExpiresIn)
	}
}

func TestRefresh_EmptyRefreshToken(t *testing.T) {
	saveCalled := false
	got := Refresh(context.Background(), "", RefreshConfig{
		TokenURL: "http://unused.invalid",
		Save:     func(*Credentials) error { saveCalled = true; return nil },
	})
	if got != nil {
		t.Errorf("Refresh() = %+v, want nil", got)
	}
	if saveCalled {
		t.Error("Save should not be called when refresh token is empty")
	}
}

func TestRefresh_SaveInvokedWithNewCreds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q, want refresh_token", r.FormValue("grant_type"))
		}
		if r.FormValue("refresh_token") != "old-refresh" {
			t.Errorf("refresh_token = %q, want old-refresh", r.FormValue("refresh_token"))
		}
		if r.FormValue("client_id") != "cid" {
			t.Errorf("client_id = %q, want cid", r.FormValue("client_id"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "new-access",
			"refresh_token": "new-refresh",
			"expires_in":    3600,
		})
	}))
	defer srv.Close()

	var saved *Credentials
	got := Refresh(context.Background(), "old-refresh", RefreshConfig{
		TokenURL:   srv.URL,
		FormFields: map[string]string{"client_id": "cid"},
		Save:       func(c *Credentials) error { saved = c; return nil },
	})

	if got == nil {
		t.Fatal("Refresh() = nil, want creds")
	}
	if got.AccessToken != "new-access" {
		t.Errorf("access_token = %q, want new-access", got.AccessToken)
	}
	if got.RefreshToken != "new-refresh" {
		t.Errorf("refresh_token = %q, want new-refresh", got.RefreshToken)
	}
	if got.ExpiresAt == "" {
		t.Error("expires_at should be set when expires_in > 0")
	}
	if saved == nil {
		t.Fatal("Save was not invoked")
	}
	if saved != got {
		t.Error("Save received a different *Credentials than was returned")
	}
}

func TestRefresh_NilSaveIsNoop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	got := Refresh(context.Background(), "old-refresh", RefreshConfig{
		TokenURL: srv.URL,
		Save:     nil,
	})

	if got == nil {
		t.Fatal("Refresh() = nil, want creds")
	}
	if got.AccessToken != "new-access" {
		t.Errorf("access_token = %q, want new-access", got.AccessToken)
	}
}

func TestRefresh_PreservesOldRefreshTokenWhenServerOmitsIt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	var saved *Credentials
	got := Refresh(context.Background(), "old-refresh", RefreshConfig{
		TokenURL: srv.URL,
		Save:     func(c *Credentials) error { saved = c; return nil },
	})

	if got == nil || got.RefreshToken != "old-refresh" {
		t.Errorf("refresh_token = %q, want old-refresh (preserved)", got.RefreshToken)
	}
	if saved == nil || saved.RefreshToken != "old-refresh" {
		t.Error("Save should receive creds with the preserved old refresh token")
	}
}

func TestRefresh_ServerErrorDoesNotInvokeSave(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	saveCalled := false
	got := Refresh(context.Background(), "old-refresh", RefreshConfig{
		TokenURL: srv.URL,
		Save:     func(*Credentials) error { saveCalled = true; return nil },
	})

	if got != nil {
		t.Errorf("Refresh() = %+v, want nil on server error", got)
	}
	if saveCalled {
		t.Error("Save must not be called when refresh fails")
	}
}

func TestRefresh_SaveErrorDoesNotInvalidateReturnedCreds(t *testing.T) {
	// Save errors mean persistence failed; the freshly-refreshed creds are
	// still valid in memory and must be returned so the current request
	// succeeds (the next invocation will refresh again).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "new-access",
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	got := Refresh(context.Background(), "old-refresh", RefreshConfig{
		TokenURL: srv.URL,
		Save: func(*Credentials) error {
			return errEphemeralPersistFail
		},
	})

	if got == nil {
		t.Fatal("Refresh() = nil, want creds even when Save fails")
	}
	if got.AccessToken != "new-access" {
		t.Errorf("access_token = %q, want new-access", got.AccessToken)
	}
}

var errEphemeralPersistFail = &persistErr{msg: "persist failed"}

type persistErr struct{ msg string }

func (e *persistErr) Error() string { return e.msg }

func TestRefresh_EmptyAccessTokenDoesNotInvokeSave(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"refresh_token": "r"})
	}))
	defer srv.Close()

	saveCalled := false
	got := Refresh(context.Background(), "old-refresh", RefreshConfig{
		TokenURL: srv.URL,
		Save:     func(*Credentials) error { saveCalled = true; return nil },
	})

	if got != nil {
		t.Errorf("Refresh() = %+v, want nil when access_token missing", got)
	}
	if saveCalled {
		t.Error("Save must not be called when access_token is empty")
	}
}
