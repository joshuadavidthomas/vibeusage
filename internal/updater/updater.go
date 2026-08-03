package updater

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	selfupdate "github.com/joshuadavidthomas/go-selfupdate"
	"golang.org/x/mod/semver"
)

const (
	defaultOwner      = "joshuadavidthomas"
	defaultRepo       = "vibeusage"
	defaultAPIBaseURL = "https://api.github.com"
	defaultUserAgent  = "vibeusage-updater"
	projectName       = "vibeusage"
)

// Service is the interface used by the CLI update command.
type Service interface {
	Check(ctx context.Context, req CheckRequest) (CheckResult, error)
	Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error)
}

// Client adapts go-selfupdate to the CLI and performs lightweight release
// checks used by the update-available header.
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

	updater *selfupdate.Updater
	plan    *selfupdate.Plan
}

// ApplyRequest controls installation of a previously checked update.
type ApplyRequest struct {
	Check          CheckResult
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
	targetVersion := normalizeTargetVersion(req.TargetVersion)
	engine, err := selfupdate.New(selfupdate.Config{
		Repository:     c.repository(),
		Command:        projectName,
		CurrentVersion: req.CurrentVersion,
		AllowDowngrade: targetVersion != "",
		HTTPClient:     c.HTTP,
		GitHubToken:    c.Token,
	})
	if err != nil {
		return CheckResult{}, err
	}

	var plan *selfupdate.Plan
	if targetVersion == "" {
		plan, err = engine.Check(ctx)
	} else {
		plan, err = engine.CheckVersion(ctx, targetVersion)
	}
	if err != nil {
		return CheckResult{}, err
	}

	release := plan.Release()
	updateAvailable := plan.UpdateAvailable()
	isDowngrade := false
	if targetVersion != "" && plan.VersionsComparable() {
		cmp := semver.Compare(plan.CurrentVersion(), plan.AvailableVersion())
		updateAvailable = cmp != 0
		isDowngrade = cmp > 0
	}

	return CheckResult{
		CurrentVersion:  plan.CurrentVersion(),
		LatestVersion:   release.Version,
		TargetVersion:   plan.AvailableVersion(),
		UpdateAvailable: updateAvailable,
		IsDowngrade:     isDowngrade,
		ReleaseName:     release.Name,
		ReleaseNotes:    release.Notes,
		ReleaseURL:      release.URL,
		AssetName:       plan.AssetName(),
		updater:         engine,
		plan:            plan,
	}, nil
}

// Apply downloads, verifies, and replaces the current binary.
func (c *Client) Apply(ctx context.Context, req ApplyRequest) (ApplyResult, error) {
	check := req.Check
	if !check.UpdateAvailable {
		return ApplyResult{OldVersion: check.CurrentVersion, NewVersion: check.TargetVersion}, nil
	}
	if check.IsDowngrade && !req.AllowDowngrade {
		return ApplyResult{}, fmt.Errorf("refusing downgrade from %s to %s without explicit approval", check.CurrentVersion, check.TargetVersion)
	}
	if check.updater == nil || check.plan == nil {
		return ApplyResult{}, fmt.Errorf("update check has no installable plan")
	}

	result, err := check.updater.Apply(ctx, check.plan)
	return ApplyResult{
		Updated:    result.Committed,
		Pending:    result.CleanupPending,
		OldVersion: result.PreviousVersion,
		NewVersion: result.Version,
		BinaryPath: result.Executable,
	}, err
}

func (c *Client) repository() string {
	owner := strings.TrimSpace(c.Owner)
	if owner == "" {
		owner = defaultOwner
	}
	repo := strings.TrimSpace(c.Repo)
	if repo == "" {
		repo = defaultRepo
	}
	return owner + "/" + repo
}

func normalizeTargetVersion(version string) string {
	version = strings.TrimSpace(version)
	if version != "" && !strings.HasPrefix(version, "v") {
		return "v" + version
	}
	return version
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

func normalizeVersion(version string) string {
	return strings.TrimPrefix(strings.TrimSpace(version), "v")
}

func compareVersions(current, target string) (int, bool) {
	current = "v" + normalizeVersion(current)
	target = "v" + normalizeVersion(target)
	if !semver.IsValid(current) || !semver.IsValid(target) {
		return 0, false
	}
	return semver.Compare(current, target), true
}
