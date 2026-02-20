package httpclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew_DefaultTimeout(t *testing.T) {
	c := New()
	if c.http.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", c.http.Timeout)
	}
}

func TestNewWithTimeout(t *testing.T) {
	c := NewWithTimeout(10 * time.Second)
	if c.http.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", c.http.Timeout)
	}
}

func TestGetJSON_Success(t *testing.T) {
	type resp struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp{Name: "test", Value: 42})
	}))
	defer srv.Close()

	c := New()
	var out resp
	httpResp, err := c.GetJSON(srv.URL, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpResp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", httpResp.StatusCode)
	}
	if out.Name != "test" || out.Value != 42 {
		t.Errorf("unexpected response: %+v", out)
	}
}

func TestGetJSON_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"unauthorized"}`))
	}))
	defer srv.Close()

	c := New()
	var out map[string]string
	httpResp, err := c.GetJSON(srv.URL, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpResp.StatusCode != 401 {
		t.Errorf("expected status 401, got %d", httpResp.StatusCode)
	}
	// Body should still be readable from Response
	if len(httpResp.Body) == 0 {
		t.Error("expected body to be captured even on non-200")
	}
}

func TestGetJSON_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	c := New()
	var out map[string]string
	httpResp, err := c.GetJSON(srv.URL, &out)
	if err != nil {
		t.Fatalf("unexpected network error: %v", err)
	}
	if httpResp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", httpResp.StatusCode)
	}
	if httpResp.JSONErr == nil {
		t.Error("expected JSONErr to be set for invalid JSON")
	}
}

func TestGetJSON_NilOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"key":"val"}`))
	}))
	defer srv.Close()

	c := New()
	httpResp, err := c.GetJSON(srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpResp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", httpResp.StatusCode)
	}
}

func TestGetJSON_NetworkError(t *testing.T) {
	c := New()
	_, err := c.GetJSON("http://127.0.0.1:1", nil)
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

func TestPostJSON_WithBody(t *testing.T) {
	type reqBody struct {
		Input string `json:"input"`
	}
	type respBody struct {
		Output string `json:"output"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		var in reqBody
		json.NewDecoder(r.Body).Decode(&in)
		json.NewEncoder(w).Encode(respBody{Output: "echo:" + in.Input})
	}))
	defer srv.Close()

	c := New()
	var out respBody
	httpResp, err := c.PostJSON(srv.URL, reqBody{Input: "hello"}, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpResp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", httpResp.StatusCode)
	}
	if out.Output != "echo:hello" {
		t.Errorf("unexpected output: %s", out.Output)
	}
}

func TestPostJSON_NilBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		// Body should be empty when nil is passed
		body := make([]byte, 1)
		n, _ := r.Body.Read(body)
		if n != 0 {
			t.Errorf("expected empty body, got %d bytes", n)
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	c := New()
	var out map[string]string
	httpResp, err := c.PostJSON(srv.URL, nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpResp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", httpResp.StatusCode)
	}
	if out["status"] != "ok" {
		t.Errorf("unexpected response: %v", out)
	}
}

func TestPostForm(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		ct := r.Header.Get("Content-Type")
		if ct != "application/x-www-form-urlencoded" {
			t.Errorf("expected form Content-Type, got %s", ct)
		}
		r.ParseForm()
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("unexpected grant_type: %s", r.FormValue("grant_type"))
		}
		json.NewEncoder(w).Encode(map[string]string{"access_token": "new-token"})
	}))
	defer srv.Close()

	c := New()
	var out map[string]string
	form := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": "old-token",
	}
	httpResp, err := c.PostForm(srv.URL, form, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpResp.StatusCode != 200 {
		t.Errorf("expected 200, got %d", httpResp.StatusCode)
	}
	if out["access_token"] != "new-token" {
		t.Errorf("unexpected token: %s", out["access_token"])
	}
}

func TestNewFromConfig(t *testing.T) {
	c := NewFromConfig(15.0)
	if c.http.Timeout != 15*time.Second {
		t.Errorf("expected timeout 15s, got %v", c.http.Timeout)
	}
}

func TestNewFromConfig_Zero(t *testing.T) {
	c := NewFromConfig(0)
	if c.http.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s for zero config, got %v", c.http.Timeout)
	}
}

func TestDo_CustomMethod(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH, got %s", r.Method)
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()

	c := New()
	httpResp, err := c.Do(http.MethodPatch, srv.URL, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if httpResp.StatusCode != 204 {
		t.Errorf("expected 204, got %d", httpResp.StatusCode)
	}
}
