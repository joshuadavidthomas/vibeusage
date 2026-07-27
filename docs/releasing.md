# Releasing vibeusage

This project publishes release binaries with GoReleaser via GitHub Actions.

## What gets published

Each release creates archives for:

- linux/amd64
- linux/arm64
- darwin/amd64
- darwin/arm64
- windows/amd64
- windows/arm64

Assets include:

- `vibeusage_<os>_<arch>.tar.gz` (linux/darwin)
- `vibeusage_windows_<arch>.zip` (windows)
- `checksums.txt`

GitHub stores a signed provenance attestation for every file listed in `checksums.txt`. GoReleaser also updates the Homebrew tap repository with `Formula/vibeusage.rb`.

## Prerequisites

A `release` GitHub environment with the following secrets:

- `HOMEBREW_TAP_GITHUB_TOKEN` - a classic PAT (or fine-grained token) with write access to `joshuadavidthomas/homebrew-homebrew`, so GoReleaser can update the tap repo

## How to cut a release

1. Ensure CI is green on `main`.
2. Tag the commit:

```bash
git tag v0.1.0
git push origin v0.1.0
```

3. The `release` workflow runs race-enabled tests on Linux, macOS, and Windows.
4. The release job runs `go vet` and a module-tidiness check.
5. GoReleaser creates the GitHub Release and updates the Homebrew formula.
6. GitHub signs and stores provenance attestations for the release artifacts.

## Recovery

Start by inspecting the workflow run and the release:

```bash
gh run list --workflow release.yml
gh release view v0.1.0
```

If a transient service or network error stopped the workflow, rerun only its failed jobs:

```bash
gh run rerun <run-id> --failed
```

If the workflow failed before GoReleaser published anything, fix the cause and cut a new version tag. Do not move a tag that has reached GitHub.

If the GitHub Release exists, inspect its archives and `checksums.txt` before rerunning the release job. That job rebuilds and republishes the release. Confirm that existing assets came from the same tagged commit, and check the Homebrew tap separately because GoReleaser may have published the GitHub assets before the tap update failed.

Attestation runs in its own job against the exact archives preserved by the release job for seven days. If only attestation failed within that window, `gh run rerun <run-id> --failed` reruns that job without invoking GoReleaser. Download an archive and verify it with a concrete filename:

```bash
gh attestation verify vibeusage_linux_amd64.tar.gz --repo joshuadavidthomas/vibeusage
```

If a published build or tag is wrong, leave that tag unchanged and issue a new patch release. Consumers may already have downloaded the old assets.

## Version injection

The runtime `vibeusage --version` value is injected from the tag by GoReleaser:

- `internal/cli.version` is set via `-ldflags`.

Local builds default to `dev`.
