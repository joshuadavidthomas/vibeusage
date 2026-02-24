package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name     string
		current  string
		target   string
		wantCmp  int
		wantBool bool
	}{
		{name: "upgrade", current: "0.1.0", target: "0.2.0", wantCmp: -1, wantBool: true},
		{name: "equal", current: "v1.2.3", target: "1.2.3", wantCmp: 0, wantBool: true},
		{name: "downgrade", current: "2.0.0", target: "1.9.9", wantCmp: 1, wantBool: true},
		{name: "invalid current", current: "dev", target: "1.0.0", wantCmp: 0, wantBool: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmp, ok := compareVersions(tt.current, tt.target)
			if ok != tt.wantBool {
				t.Fatalf("compareVersions ok = %v, want %v", ok, tt.wantBool)
			}
			if ok && cmp != tt.wantCmp {
				t.Fatalf("compareVersions cmp = %d, want %d", cmp, tt.wantCmp)
			}
		})
	}
}

func TestChecksumForAsset(t *testing.T) {
	checksums := "" +
		"aabbccdd  vibeusage_linux_amd64.tar.gz\n" +
		"11223344 *vibeusage_windows_amd64.zip\n"

	hash, ok := checksumForAsset(checksums, "vibeusage_windows_amd64.zip")
	if !ok {
		t.Fatal("checksumForAsset returned ok=false")
	}
	if hash != "11223344" {
		t.Fatalf("checksumForAsset hash = %q, want %q", hash, "11223344")
	}
}

func TestExtractBinaryFromTarGz(t *testing.T) {
	archive := makeTarGzArchive(t, "vibeusage", []byte("binary-data"))
	body, err := extractBinaryFromArchive("vibeusage_linux_amd64.tar.gz", archive, "vibeusage")
	if err != nil {
		t.Fatalf("extractBinaryFromArchive(tar.gz) error: %v", err)
	}
	if string(body) != "binary-data" {
		t.Fatalf("extracted body = %q, want %q", string(body), "binary-data")
	}
}

func TestExtractBinaryFromZip(t *testing.T) {
	archive := makeZipArchive(t, "vibeusage.exe", []byte("windows-binary"))
	body, err := extractBinaryFromArchive("vibeusage_windows_amd64.zip", archive, "vibeusage.exe")
	if err != nil {
		t.Fatalf("extractBinaryFromArchive(zip) error: %v", err)
	}
	if string(body) != "windows-binary" {
		t.Fatalf("extracted body = %q, want %q", string(body), "windows-binary")
	}
}

func TestClientCheckAndApply(t *testing.T) {
	archive := makeTarGzArchive(t, "vibeusage", []byte("new-binary"))
	sum := sha256.Sum256(archive)
	checksum := hex.EncodeToString(sum[:])
	checksums := fmt.Sprintf("%s  vibeusage_linux_amd64.tar.gz\n", checksum)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()

	release := githubRelease{
		TagName: "v1.2.3",
		Name:    "v1.2.3",
		Assets: []githubReleaseAsset{
			{Name: "vibeusage_linux_amd64.tar.gz", BrowserDownloadURL: server.URL + "/asset"},
			{Name: "checksums.txt", BrowserDownloadURL: server.URL + "/checksums"},
		},
	}

	mux.HandleFunc("/repos/joshuadavidthomas/vibeusage/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(release)
	})
	mux.HandleFunc("/asset", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive)
	})
	mux.HandleFunc("/checksums", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(checksums))
	})

	client := NewClient()
	client.APIBaseURL = server.URL
	client.HTTP = server.Client()

	check, err := client.Check(context.Background(), CheckRequest{
		CurrentVersion: "v1.0.0",
		OS:             "linux",
		Arch:           "amd64",
	})
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}
	if !check.UpdateAvailable {
		t.Fatal("expected update to be available")
	}
	if check.TargetVersion != "v1.2.3" {
		t.Fatalf("target version = %q, want %q", check.TargetVersion, "v1.2.3")
	}

	targetPath := filepath.Join(t.TempDir(), "vibeusage")
	if err := os.WriteFile(targetPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("failed to seed target binary: %v", err)
	}

	apply, err := client.Apply(context.Background(), ApplyRequest{Check: check, BinaryPath: targetPath})
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if !apply.Updated {
		t.Fatal("expected Updated=true")
	}
	if apply.Pending {
		t.Fatal("expected Pending=false on non-windows")
	}

	updatedBody, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed to read updated binary: %v", err)
	}
	if string(updatedBody) != "new-binary" {
		t.Fatalf("updated binary body = %q, want %q", string(updatedBody), "new-binary")
	}
}

func TestClientApplyRefusesDowngrade(t *testing.T) {
	client := NewClient()
	_, err := client.Apply(context.Background(), ApplyRequest{Check: CheckResult{
		CurrentVersion:  "v2.0.0",
		TargetVersion:   "v1.0.0",
		UpdateAvailable: true,
		IsDowngrade:     true,
	}})
	if err == nil {
		t.Fatal("expected downgrade refusal error")
	}
	if !strings.Contains(err.Error(), "refusing downgrade") {
		t.Fatalf("error = %q, want downgrade refusal", err.Error())
	}
}

func makeTarGzArchive(t *testing.T, filename string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tarWriter := tar.NewWriter(gz)

	header := &tar.Header{Name: filename, Mode: 0o755, Size: int64(len(body))}
	if err := tarWriter.WriteHeader(header); err != nil {
		t.Fatalf("WriteHeader error: %v", err)
	}
	if _, err := tarWriter.Write(body); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tar close error: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close error: %v", err)
	}
	return buf.Bytes()
}

func makeZipArchive(t *testing.T, filename string, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	f, err := zipWriter.Create(filename)
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if _, err := f.Write(body); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if err := zipWriter.Close(); err != nil {
		t.Fatalf("zip close error: %v", err)
	}
	return buf.Bytes()
}
