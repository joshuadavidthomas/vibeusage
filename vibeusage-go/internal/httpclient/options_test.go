package httpclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Custom")
		if got != "my-value" {
			t.Errorf("expected X-Custom=my-value, got %s", got)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New()
	_, err := c.Do(http.MethodGet, srv.URL, nil, WithHeader("X-Custom", "my-value"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestWithBearer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("Authorization")
		if got != "Bearer my-token" {
			t.Errorf("expected Bearer my-token, got %s", got)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New()
	_, err := c.Do(http.MethodGet, srv.URL, nil, WithBearer("my-token"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestWithCookie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("sessionKey")
		if err != nil {
			t.Errorf("expected sessionKey cookie: %v", err)
			return
		}
		if cookie.Value != "abc123" {
			t.Errorf("expected cookie value abc123, got %s", cookie.Value)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New()
	_, err := c.Do(http.MethodGet, srv.URL, nil, WithCookie("sessionKey", "abc123"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestMultipleOptions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer tok" {
			t.Errorf("expected Bearer tok, got %s", auth)
		}
		beta := r.Header.Get("anthropic-beta")
		if beta != "oauth-2025-04-20" {
			t.Errorf("expected anthropic-beta header, got %s", beta)
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := New()
	var out map[string]string
	_, err := c.GetJSON(srv.URL, &out,
		WithBearer("tok"),
		WithHeader("anthropic-beta", "oauth-2025-04-20"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if out["ok"] != "true" {
		t.Errorf("unexpected response: %v", out)
	}
}
