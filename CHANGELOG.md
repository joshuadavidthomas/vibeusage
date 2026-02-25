# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project attempts to adhere to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

<!--
## [${version}]

### Added - for new features
### Changed - for changes in existing functionality
### Deprecated - for soon-to-be removed features
### Removed - for now removed features
### Fixed - for any bug fixes
### Security - in case of vulnerabilities

[${version}]: https://github.com/joshuadavidthomas/vibeusage/releases/tag/${tag}
-->

## [Unreleased]

### Fixed

- Updated default-role seeding to read config directly from disk so existing config values are preserved when in-memory config is stale.
- Improved fetch pipeline final error reporting so trailing "not configured" strategies don’t mask the real preceding failure reason.

## [0.2.0]

### Added

- Added Homebrew installation support.

### Changed

- Limited `vibeusage update` installs to script-managed installations.
- Switched path management to `github.com/adrg/xdg`, with an intentional macOS preference for `~/.config/vibeusage/config.toml` when `XDG_CONFIG_HOME` is unset.
- Moved vibeusage-managed credentials to the XDG data directory (`DataHome/vibeusage/credentials`) and added `VIBEUSAGE_DATA_DIR` override support.
- Added temporary dual-write compatibility for config and credentials (new + legacy paths) to ease upgrades; planned for removal in v0.3.0.

## [0.1.1]

### Fixed

- Fixed CLI panic when running `vibeusage init` due to a shorthand flag collision between `init --quick` and global `--quiet`.
- Fixed CLI panic when running `vibeusage config path` due to a shorthand flag collision between `config path --credentials` and global `--refresh`.
- Added regression tests to catch Cobra flag-merge panics across the full command tree.

## [0.1.0]

### Added

- Created `vibeusage` CLI for tracking usage quotas across AI coding tool providers from the terminal.
- Added support for 12 providers: Amp, Antigravity (Google AI IDE), Claude Code, Codex (OpenAI), Copilot (GitHub), Cursor, Gemini, Kimi Code, Minimax, OpenRouter, Warp, and Z.ai.
- Added automatic credential detection for providers with existing CLI tools (Claude Code, Codex CLI, Copilot, Gemini CLI, Kimi CLI, Amp CLI, and the Antigravity IDE), with token refresh where supported.
- Added interactive authentication flows: OAuth device flow for Copilot and Kimi Code, browser session key extraction for Claude and Cursor, and API key entry for Amp, Gemini, Minimax, OpenRouter, Warp, and Z.ai.
- Added `vibeusage route <model>` command for smart model routing that picks the best provider for a model based on real usage headroom across connected accounts.
- Added role-based routing groups (`vibeusage route --role <name>`) configurable via `config.toml` under `[roles.<name>]`.
- Added dynamic model registry sourced from models.dev for routing lookups, with cost multiplier awareness.
- Added pace-colored progress bars (green, yellow, red) based on usage rate relative to the reset period.
- Added `vibeusage init` first-run setup wizard for guided provider configuration.
- Added `vibeusage update` command for self-updating from GitHub releases, with `--check` for checking without installing.
- Added `--json` flag for machine-readable output across all commands.
- Added `--refresh` global flag to bypass cache fallback and require fresh data.
- Added cache system with stale data fallback when provider APIs are unavailable but credentials are configured.
- Added configuration via TOML file with settings for credentials, display, fetch behavior, and routing roles.
- Added shell completion support via `vibeusage completion`.

### New Contributors

- Josh Thomas <josh@joshthomas.dev> (maintainer)

[unreleased]: https://github.com/joshuadavidthomas/vibeusage/compare/v0.2.0...HEAD
[0.1.0]: https://github.com/joshuadavidthomas/vibeusage/releases/tag/v0.1.0
[0.1.1]: https://github.com/joshuadavidthomas/vibeusage/releases/tag/v0.1.1
[0.2.0]: https://github.com/joshuadavidthomas/vibeusage/releases/tag/v0.2.0
