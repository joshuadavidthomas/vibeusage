package updater

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestIsReleaseVersion(t *testing.T) {
	if !IsReleaseVersion("v1.2.3") {
		t.Fatal("expected tagged semver release to be valid")
	}
	if IsReleaseVersion("dev") {
		t.Fatal("expected dev version to be rejected")
	}
}

func TestAvailableUpdateForVersion_UpdateAvailable(t *testing.T) {
	update, ok := AvailableUpdateForVersion("v1.0.0", LatestReleaseInfo{
		CheckedAt:     time.Now(),
		AttemptedAt:   time.Now(),
		LatestVersion: "v1.1.0",
		ReleaseURL:    "https://example.com/release",
	})
	if !ok {
		t.Fatal("expected available update")
	}
	if update.CurrentVersion != "v1.0.0" {
		t.Fatalf("current version = %q, want %q", update.CurrentVersion, "v1.0.0")
	}
	if update.LatestVersion != "v1.1.0" {
		t.Fatalf("latest version = %q, want %q", update.LatestVersion, "v1.1.0")
	}
}

func TestAvailableUpdateForVersion_NoUpdateForInvalidCurrentVersion(t *testing.T) {
	if _, ok := AvailableUpdateForVersion("dev", LatestReleaseInfo{LatestVersion: "v1.1.0"}); ok {
		t.Fatal("expected no update for dev builds")
	}
}

func TestLatestReleaseInfoAttemptExpired(t *testing.T) {
	entry := LatestReleaseInfo{AttemptedAt: time.Now().Add(-2 * time.Hour)}
	if !entry.AttemptExpired(time.Hour) {
		t.Fatal("expected attempt to be expired")
	}
	if entry.AttemptExpired(3 * time.Hour) {
		t.Fatal("expected attempt to be fresh")
	}
	if !(LatestReleaseInfo{}).AttemptExpired(time.Hour) {
		t.Fatal("expected zero AttemptedAt to be expired")
	}
}

func TestRefreshLatestReleaseCache_PreservesCachedDataOnError(t *testing.T) {
	cached := &LatestReleaseInfo{
		CheckedAt:     time.Now().Add(-48 * time.Hour),
		AttemptedAt:   time.Now().Add(-48 * time.Hour),
		LatestVersion: "v1.1.0",
		ReleaseURL:    "https://example.com/release",
	}
	client := &Client{HTTP: roundTripperClient(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	})}

	updated, err := RefreshLatestReleaseCache(context.Background(), client, cached)
	if err == nil {
		t.Fatal("expected refresh error")
	}
	if updated.LatestVersion != cached.LatestVersion {
		t.Fatalf("latest version = %q, want preserved %q", updated.LatestVersion, cached.LatestVersion)
	}
	if !updated.AttemptedAt.After(cached.AttemptedAt) {
		t.Fatal("expected AttemptedAt to advance on error")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func roundTripperClient(fn roundTripperFunc) *http.Client {
	return &http.Client{Transport: fn}
}
