package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/joshuadavidthomas/vibeusage/internal/models"
)

func writeJSON(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		panic("writeJSON: " + err.Error())
	}
}

func TestFetchStatuspageStatus_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"status": map[string]any{
				"indicator":   "none",
				"description": "All Systems Operational",
			},
		})
	}))
	defer srv.Close()

	status := FetchStatuspageStatus(context.Background(), srv.URL)

	if status.Level != models.StatusOperational {
		t.Errorf("expected StatusOperational, got %v", status.Level)
	}
	if status.Description != "All Systems Operational" {
		t.Errorf("expected description 'All Systems Operational', got %q", status.Description)
	}
	if status.UpdatedAt == nil {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestFetchStatuspageStatus_Indicators(t *testing.T) {
	tests := []struct {
		indicator string
		want      models.StatusLevel
	}{
		{"none", models.StatusOperational},
		{"minor", models.StatusDegraded},
		{"major", models.StatusPartialOutage},
		{"critical", models.StatusMajorOutage},
		{"bogus", models.StatusUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.indicator, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				writeJSON(w, map[string]any{
					"status": map[string]any{
						"indicator":   tt.indicator,
						"description": "test",
					},
				})
			}))
			defer srv.Close()

			status := FetchStatuspageStatus(context.Background(), srv.URL)
			if status.Level != tt.want {
				t.Errorf("indicator %q: got %v, want %v", tt.indicator, status.Level, tt.want)
			}
		})
	}
}

func TestFetchStatuspageStatus_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	status := FetchStatuspageStatus(ctx, srv.URL)
	if status.Level != models.StatusUnknown {
		t.Errorf("expected StatusUnknown on cancelled context, got %v", status.Level)
	}
}

func TestFetchStatuspageStatus_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	status := FetchStatuspageStatus(context.Background(), srv.URL)
	if status.Level != models.StatusUnknown {
		t.Errorf("expected StatusUnknown on server error, got %v", status.Level)
	}
}

func TestFetchGoogleAppsStatus_NoActiveIncidents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []googleIncident{
			{Title: "Gemini outage", Severity: "high", EndTime: "2025-01-01T00:00:00Z"},
		})
	}))
	defer srv.Close()

	status := fetchGoogleAppsStatusFromURL(context.Background(), srv.URL, []string{"gemini"})
	if status.Level != models.StatusOperational {
		t.Errorf("expected StatusOperational for resolved incident, got %v", status.Level)
	}
}

func TestFetchGoogleAppsStatus_ActiveIncidentMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []googleIncident{
			{Title: "Gemini API latency", Severity: "high", EndTime: ""},
		})
	}))
	defer srv.Close()

	status := fetchGoogleAppsStatusFromURL(context.Background(), srv.URL, []string{"gemini"})
	if status.Level != models.StatusPartialOutage {
		t.Errorf("expected StatusPartialOutage for high severity, got %v", status.Level)
	}
	if status.Description != "Gemini API latency" {
		t.Errorf("expected description, got %q", status.Description)
	}
}

func TestFetchGoogleAppsStatus_NoKeywordMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, []googleIncident{
			{Title: "Gmail outage", Severity: "critical", EndTime: ""},
		})
	}))
	defer srv.Close()

	status := fetchGoogleAppsStatusFromURL(context.Background(), srv.URL, []string{"gemini"})
	if status.Level != models.StatusOperational {
		t.Errorf("expected StatusOperational when no keyword match, got %v", status.Level)
	}
}

func TestFetchGoogleAppsStatus_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	status := fetchGoogleAppsStatusFromURL(ctx, srv.URL, []string{"gemini"})
	if status.Level != models.StatusUnknown {
		t.Errorf("expected StatusUnknown on cancelled context, got %v", status.Level)
	}
}

func TestFetchOnlineOrNotStatus_NoIncidents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Status</title>
  </channel>
</rss>`))
	}))
	defer srv.Close()

	status := fetchOnlineOrNotStatusFromURL(context.Background(), srv.URL)
	if status.Level != models.StatusOperational {
		t.Errorf("expected StatusOperational for empty feed, got %v", status.Level)
	}
	if status.Description != "All systems operational" {
		t.Errorf("expected 'All systems operational', got %q", status.Description)
	}
}

func TestFetchOnlineOrNotStatus_ResolvedIncidents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Test Status</title>
    <item>
      <title>API Outage</title>
      <description><![CDATA[<strong>RESOLVED</strong> - This incident has been resolved.]]></description>
      <pubDate>Thu, 26 Feb 2026 10:00:00 GMT</pubDate>
    </item>
  </channel>
</rss>`))
	}))
	defer srv.Close()

	status := fetchOnlineOrNotStatusFromURL(context.Background(), srv.URL)
	if status.Level != models.StatusOperational {
		t.Errorf("expected StatusOperational for resolved incidents, got %v", status.Level)
	}
}

func TestFetchOnlineOrNotStatus_UnresolvedIncident(t *testing.T) {
	// Use parseOnlineOrNotFeed directly to avoid timing issues with RSS dates
	feed := rssFeed{
		Channel: rssChannel{
			Items: []rssItem{
				{
					Title:       "API Degradation",
					Description: "<strong>Investigating</strong> - We are investigating an issue.",
					PubDate:     time.Now().UTC().Format(time.RFC1123),
				},
			},
		},
	}

	status := parseOnlineOrNotFeed(feed)
	if status.Level != models.StatusDegraded {
		t.Errorf("expected StatusDegraded for unresolved incident, got %v", status.Level)
	}
	if status.Description != "API Degradation" {
		t.Errorf("expected description 'API Degradation', got %q", status.Description)
	}
}

func TestFetchOnlineOrNotStatus_OldIncidentIgnored(t *testing.T) {
	feed := rssFeed{
		Channel: rssChannel{
			Items: []rssItem{
				{
					Title:       "Old Outage",
					Description: "<strong>Investigating</strong> - We are investigating.",
					PubDate:     time.Now().UTC().Add(-48 * time.Hour).Format(time.RFC1123),
				},
			},
		},
	}

	status := parseOnlineOrNotFeed(feed)
	if status.Level != models.StatusOperational {
		t.Errorf("expected StatusOperational for old incident, got %v", status.Level)
	}
}

func TestFetchOnlineOrNotStatus_CaseInsensitiveResolved(t *testing.T) {
	feed := rssFeed{
		Channel: rssChannel{
			Items: []rssItem{
				{
					Title:       "Resolved Issue",
					Description: "<strong>resolved</strong> - This has been resolved.",
					PubDate:     time.Now().UTC().Format(time.RFC1123),
				},
			},
		},
	}

	status := parseOnlineOrNotFeed(feed)
	if status.Level != models.StatusOperational {
		t.Errorf("expected StatusOperational for resolved (lowercase) incident, got %v", status.Level)
	}
}

func TestFetchOnlineOrNotStatus_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	status := fetchOnlineOrNotStatusFromURL(context.Background(), srv.URL)
	if status.Level != models.StatusUnknown {
		t.Errorf("expected StatusUnknown on server error, got %v", status.Level)
	}
}

func TestFetchOnlineOrNotStatus_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	status := fetchOnlineOrNotStatusFromURL(ctx, srv.URL)
	if status.Level != models.StatusUnknown {
		t.Errorf("expected StatusUnknown on cancelled context, got %v", status.Level)
	}
}

func TestParseRSSDate(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"Thu, 26 Feb 2026 10:00:00 GMT", false},
		{"Mon, 01 Jan 2024 00:00:00 UTC", false},
		{"invalid date", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := parseRSSDate(tt.input)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for %q", tt.input)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for %q: %v", tt.input, err)
			}
		})
	}
}
