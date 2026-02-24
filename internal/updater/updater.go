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
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

const (
	defaultOwner       = "joshuadavidthomas"
	defaultRepo        = "vibeusage"
	defaultAPIBaseURL  = "https://api.github.com"
	defaultUserAgent   = "vibeusage-updater"
	projectName        = "vibeusage"
	checksumsAssetName = "checksums.txt"
)

// Service is the interface used by the CLI update command.
type Service interface {
	Check(ctx context.Context, req CheckRequest) (CheckResult, error)
	Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error)
}

// Client checks GitHub releases and applies binary updates.
type Client struct {
	Owner      string
	Repo       string
	APIBaseURL string
	Token      string
	HTTP       *http.Client
}

// CheckRequest configures update-check behavior.
type CheckRequest struct {
	CurrentVersion string
	TargetVersion  string
	OS             string
	Arch           string
}

// CheckResult describes update availability for this platform.
type CheckResult struct {
	CurrentVersion  string
	LatestVersion   string
	TargetVersion   string
	UpdateAvailable bool
	IsDowngrade     bool
	ReleaseName     string
	ReleaseNotes    string
	ReleaseURL      string
	AssetName       string
	AssetURL        string
	ChecksumsURL    string
	OS              string
	Arch            string
}

// ApplyRequest controls installation of a previously checked update.
type ApplyRequest struct {
	Check          CheckResult
	BinaryPath     string
	AllowDowngrade bool
}

// ApplyResult is the result of applying an update.
type ApplyResult struct {
	Updated    bool
	Pending    bool
	OldVersion string
	NewVersion string
	BinaryPath string
}

type githubRelease struct {
	TagName     string               `json:"tag_name"`
	Name        string               `json:"name"`
	Body        string               `json:"body"`
	HTMLURL     string               `json:"html_url"`
	PublishedAt time.Time            `json:"published_at"`
	Assets      []githubReleaseAsset `json:"assets"`
}

type githubReleaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// NewClient creates a GitHub-backed updater client.
func NewClient() *Client {
	return &Client{
		Owner:      defaultOwner,
		Repo:       defaultRepo,
		APIBaseURL: defaultAPIBaseURL,
		Token:      strings.TrimSpace(os.Getenv("VIBEUSAGE_UPDATE_GITHUB_TOKEN")),
		HTTP:       &http.Client{Timeout: 60 * time.Second},
	}
}

// Check checks GitHub releases and returns whether an update is available.
func (c *Client) Check(ctx context.Context, req CheckRequest) (CheckResult, error) {
	osName := req.OS
	if osName == "" {
		osName = runtime.GOOS
	}
	arch := req.Arch
	if arch == "" {
		arch = runtime.GOARCH
	}

	release, err := c.fetchRelease(ctx, req.TargetVersion)
	if err != nil {
		return CheckResult{}, err
	}

	assetName, err := expectedAssetName(osName, arch)
	if err != nil {
		return CheckResult{}, err
	}

	asset, ok := findReleaseAsset(release.Assets, assetName, osName, arch)
	if !ok {
		return CheckResult{}, fmt.Errorf("release %s does not include an asset for %s/%s", release.TagName, osName, arch)
	}

	checksums, ok := findChecksumsAsset(release.Assets)
	if !ok {
		return CheckResult{}, fmt.Errorf("release %s does not include %s", release.TagName, checksumsAssetName)
	}

	targetVersion := normalizeVersion(release.TagName)
	currentVersion := normalizeVersion(req.CurrentVersion)
	updateAvailable := false
	isDowngrade := false

	if req.TargetVersion == "" {
		if cmp, comparable := compareVersions(currentVersion, targetVersion); comparable {
			updateAvailable = cmp < 0
		} else {
			updateAvailable = currentVersion == "" || currentVersion != targetVersion
		}
	} else {
		if cmp, comparable := compareVersions(currentVersion, targetVersion); comparable {
			updateAvailable = cmp != 0
			isDowngrade = cmp > 0
		} else {
			updateAvailable = currentVersion == "" || currentVersion != targetVersion
		}
	}

	return CheckResult{
		CurrentVersion:  req.CurrentVersion,
		LatestVersion:   release.TagName,
		TargetVersion:   release.TagName,
		UpdateAvailable: updateAvailable,
		IsDowngrade:     isDowngrade,
		ReleaseName:     release.Name,
		ReleaseNotes:    release.Body,
		ReleaseURL:      release.HTMLURL,
		AssetName:       asset.Name,
		AssetURL:        asset.BrowserDownloadURL,
		ChecksumsURL:    checksums.BrowserDownloadURL,
		OS:              osName,
		Arch:            arch,
	}, nil
}

// Apply downloads, verifies, and replaces the current binary.
func (c *Client) Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error) {
	check := req.Check
	if !check.UpdateAvailable {
		return ApplyResult{
			Updated:    false,
			OldVersion: check.CurrentVersion,
			NewVersion: check.TargetVersion,
		}, nil
	}
	if check.IsDowngrade && !req.AllowDowngrade {
		return ApplyResult{}, fmt.Errorf("refusing downgrade from %s to %s without explicit approval", check.CurrentVersion, check.TargetVersion)
	}

	checksumsBody, err := c.download(ctx, check.ChecksumsURL)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("failed to download checksums: %w", err)
	}

	expectedChecksum, ok := checksumForAsset(string(checksumsBody), check.AssetName)
	if !ok {
		return ApplyResult{}, fmt.Errorf("checksums file does not contain %s", check.AssetName)
	}

	archiveBody, err := c.download(ctx, check.AssetURL)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("failed to download release asset: %w", err)
	}
	if err := verifySHA256(archiveBody, expectedChecksum); err != nil {
		return ApplyResult{}, err
	}

	binaryBody, err := extractBinaryFromArchive(check.AssetName, archiveBody, binaryNameForOS(check.OS))
	if err != nil {
		return ApplyResult{}, fmt.Errorf("failed to extract binary from archive: %w", err)
	}

	targetPath, err := resolveBinaryPath(req.BinaryPath)
	if err != nil {
		return ApplyResult{}, err
	}

	pending, err := replaceBinary(targetPath, binaryBody)
	if err != nil {
		return ApplyResult{}, err
	}

	return ApplyResult{
		Updated:    true,
		Pending:    pending,
		OldVersion: check.CurrentVersion,
		NewVersion: check.TargetVersion,
		BinaryPath: targetPath,
	}, nil
}

func (c *Client) fetchRelease(ctx context.Context, targetVersion string) (*githubRelease, error) {
	apiBase := strings.TrimSuffix(strings.TrimSpace(c.APIBaseURL), "/")
	if apiBase == "" {
		apiBase = defaultAPIBaseURL
	}
	owner := strings.TrimSpace(c.Owner)
	if owner == "" {
		owner = defaultOwner
	}
	repo := strings.TrimSpace(c.Repo)
	if repo == "" {
		repo = defaultRepo
	}

	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases/latest", apiBase, owner, repo)
	if targetVersion != "" {
		tag := targetVersion
		if !strings.HasPrefix(tag, "v") {
			tag = "v" + tag
		}
		endpoint = fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", apiBase, owner, repo, url.PathEscape(tag))
	}

	body, err := c.getJSON(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, fmt.Errorf("failed to parse release metadata: %w", err)
	}
	if rel.TagName == "" {
		return nil, fmt.Errorf("release metadata missing tag_name")
	}
	return &rel, nil
}

func (c *Client) getJSON(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", defaultUserAgent)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("GitHub API request failed (%d): %s", resp.StatusCode, msg)
	}

	return body, nil
}

func (c *Client) download(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", defaultUserAgent)
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("download failed (%d): %s", resp.StatusCode, msg)
	}

	return body, nil
}

func expectedAssetName(osName, arch string) (string, error) {
	switch arch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture for self-update: %s", arch)
	}

	switch osName {
	case "linux", "darwin":
		return fmt.Sprintf("%s_%s_%s.tar.gz", projectName, osName, arch), nil
	case "windows":
		return fmt.Sprintf("%s_windows_%s.zip", projectName, arch), nil
	default:
		return "", fmt.Errorf("unsupported OS for self-update: %s", osName)
	}
}

func findReleaseAsset(assets []githubReleaseAsset, expectedName, osName, arch string) (githubReleaseAsset, bool) {
	for _, asset := range assets {
		if asset.Name == expectedName {
			return asset, true
		}
	}

	marker := "_" + osName + "_" + arch
	var fallback githubReleaseAsset
	for _, asset := range assets {
		if !strings.HasPrefix(asset.Name, projectName+"_") {
			continue
		}
		if !strings.Contains(asset.Name, marker) {
			continue
		}
		if strings.HasSuffix(asset.Name, ".tar.gz") || strings.HasSuffix(asset.Name, ".zip") {
			if fallback.Name == "" {
				fallback = asset
			}
		}
	}
	if fallback.Name != "" {
		return fallback, true
	}

	return githubReleaseAsset{}, false
}

func findChecksumsAsset(assets []githubReleaseAsset) (githubReleaseAsset, bool) {
	for _, asset := range assets {
		if asset.Name == checksumsAssetName {
			return asset, true
		}
	}
	for _, asset := range assets {
		if strings.HasSuffix(asset.Name, "_checksums.txt") {
			return asset, true
		}
	}
	return githubReleaseAsset{}, false
}

func checksumForAsset(checksumsContent, assetName string) (string, bool) {
	for _, line := range strings.Split(checksumsContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := strings.TrimSpace(fields[0])
		name := strings.TrimPrefix(strings.TrimSpace(fields[1]), "*")
		if name == assetName {
			return hash, true
		}
	}
	return "", false
}

func verifySHA256(content []byte, expectedHex string) error {
	expected := strings.ToLower(strings.TrimSpace(expectedHex))
	sum := sha256.Sum256(content)
	actual := hex.EncodeToString(sum[:])
	if expected != actual {
		return fmt.Errorf("checksum mismatch for release asset: expected %s, got %s", expected, actual)
	}
	return nil
}

func extractBinaryFromArchive(assetName string, archiveBody []byte, binaryName string) ([]byte, error) {
	switch {
	case strings.HasSuffix(assetName, ".tar.gz"):
		return extractBinaryFromTarGz(archiveBody, binaryName)
	case strings.HasSuffix(assetName, ".zip"):
		return extractBinaryFromZip(archiveBody, binaryName)
	default:
		return nil, fmt.Errorf("unsupported archive format for asset %s", assetName)
	}
}

func extractBinaryFromTarGz(archiveBody []byte, binaryName string) ([]byte, error) {
	gzReader, err := gzip.NewReader(bytes.NewReader(archiveBody))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(header.Name) != binaryName {
			continue
		}
		binaryBody, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, err
		}
		if len(binaryBody) == 0 {
			return nil, fmt.Errorf("archive entry %s is empty", header.Name)
		}
		return binaryBody, nil
	}
	return nil, fmt.Errorf("binary %s not found in tar.gz archive", binaryName)
}

func extractBinaryFromZip(archiveBody []byte, binaryName string) ([]byte, error) {
	readerAt := bytes.NewReader(archiveBody)
	zipReader, err := zip.NewReader(readerAt, int64(len(archiveBody)))
	if err != nil {
		return nil, err
	}
	for _, file := range zipReader.File {
		if filepath.Base(file.Name) != binaryName {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		binaryBody, readErr := io.ReadAll(rc)
		_ = rc.Close()
		if readErr != nil {
			return nil, readErr
		}
		if len(binaryBody) == 0 {
			return nil, fmt.Errorf("archive entry %s is empty", file.Name)
		}
		return binaryBody, nil
	}
	return nil, fmt.Errorf("binary %s not found in zip archive", binaryName)
}

func resolveBinaryPath(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return override, nil
	}
	path, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to locate current executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path, nil
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	return v
}

func compareVersions(current, target string) (int, bool) {
	current = normalizeVersion(current)
	target = normalizeVersion(target)
	if current == "" || target == "" {
		return 0, false
	}
	currentSemver := "v" + current
	targetSemver := "v" + target
	if !semver.IsValid(currentSemver) || !semver.IsValid(targetSemver) {
		return 0, false
	}
	return semver.Compare(currentSemver, targetSemver), true
}

func binaryNameForOS(osName string) string {
	if osName == "windows" {
		return projectName + ".exe"
	}
	return projectName
}
