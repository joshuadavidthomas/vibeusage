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

## How to cut a release

1. Ensure CI is green on `main`.
2. Tag the commit:

```bash
git tag v0.1.0
git push origin v0.1.0
```

3. The `release` workflow runs automatically and creates a GitHub Release.

## Version injection

The runtime `vibeusage --version` value is injected from the tag by GoReleaser:

- `internal/cli.version` is set via `-ldflags`.

Local builds default to `dev`.
