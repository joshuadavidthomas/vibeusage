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

### Added

- Added `vibeusage statusline` command for condensed usage output suitable for status bars.
- Added provider status, identity metadata, and auth source to single-provider detail view.
- Added CLI fallback for Codex OAuth token refresh. When the standard refresh fails, vibeusage now spawns the Codex CLI in the background to refresh credentials automatically, matching the existing Claude Code behavior.

### Changed

- Polished single-provider detail view to match the multi-provider overview panel style.
- Overage with no spend limit now shows "(Unlimited)" instead of "$0.00".
- Improved Codex error messages to suggest `codex login` when token refresh fails or credentials are invalid.

### Removed

- Removed unreliable Claude plan tier inference.

### Fixed

- Fixed Codex OAuth tokens silently expiring without automatic refresh. The Codex CLI stores credentials without `expires_at`, so vibeusage couldn't detect expiry upfront. Now retries token refresh (both standard OAuth and CLI fallback) when the API returns 401, instead of immediately failing.
- Fixed inconsistent panel widths in the dashboard view where providers without reset times (e.g., Amp) rendered narrower boxes than providers with reset countdowns.
- Fixed confusing overage display when the API returns a null monthly spend limit.

## [0.3.0]

### Added

- Added `vibeusage usage` command to group provider-specific usage commands.
- Added `vibeusage auth <provider> --delete` to remove credentials and disable a provider.
- Added `vibeusage auth <provider> --token <value>` for non-interactive credential setup (useful for scripting and CI).

### Changed

- **Breaking:** Provider-specific usage commands moved from top-level (`vibeusage claude`, `vibeusage codex`, etc.) to `vibeusage usage <provider>`.
- **Breaking:** `--json` output now serializes usage snapshots directly from the model types. Removed `remaining`, `cached` fields; added `fetched_at`, `is_enabled`, `source` fields. `resets_at` uses Go's default time format. Identity fields with empty values are now omitted.
- Reordered Claude fetch strategy flow to prefer OAuth first and keep web session usage as the last-resort fallback.
- Providers without a dedicated auth flow now prompt for credentials inline instead of telling users to run a separate command.

### Deprecated

- Top-level provider commands (`vibeusage claude`, `vibeusage codex`, etc.) are deprecated and will be removed in v0.4.0. Deprecated stubs currently remain with error messages pointing to `vibeusage usage <provider>`. Use `vibeusage usage <provider>` instead.

### Removed

- **Breaking:** Removed `vibeusage cache` command, `fetch.stale_threshold_minutes` config option, and `--refresh` flag. Cache is now only used for resilience when APIs fail. Use `--no-cache` to fail fast instead of falling back to cached data.
- **Breaking:** Removed v0.2.0 legacy migration compatibility layer. Config and credentials are no longer read from or dual-written to legacy paths. Ensure you've run v0.2.0+ at least once before upgrading to ensure data is migrated.
- **Breaking:** Removed `vibeusage key` command. All credential management is now handled through `vibeusage auth`. Use `vibeusage auth --status` instead of `vibeusage key` for credential status, `vibeusage auth <provider> --token <value>` instead of `vibeusage key <provider> set`, and `vibeusage auth <provider> --delete` instead of `vibeusage key <provider> delete`.
- Removed Anthropic API key (`sk-ant-api...` / `sk-ant-admin-...`) support from Claude provider. Regular API keys cannot access consumer plan usage data — they live in a separate billing system with no access to Pro/Max rate limit information. Future Admin API key support tracked in [#97](https://github.com/joshuadavidthomas/vibeusage/issues/97). `vibeusage auth claude` now only accepts `sessionKey` cookies (`sk-ant-sid01-...`).
- Removed `ANTHROPIC_API_KEY` environment variable support from the Claude provider.

### Fixed

- Fixed Claude web session strategy silently returning empty usage data. The `claude.ai` usage endpoint returns the same response format as the OAuth endpoint (`five_hour`/`seven_day`/per-model breakdowns), but the web strategy was parsing a different format (`usage_amount`/`usage_limit`).
- Fixed provider credential leakage when `credentials.reuse_provider_credentials = false` by making provider strategy availability/loading consistently honor the setting.
- Fixed credential path coupling by removing `DataDir()` fallback to `VIBEUSAGE_CONFIG_DIR`; credentials now use the data path unless `VIBEUSAGE_DATA_DIR` is explicitly set.
- Fixed no-data output to show provider-specific fetch errors when providers are configured but all strategies fail.
- Fixed credential auto-detection for macOS users authenticated with Claude Code and Codex CLI by adding macOS Keychain credential lookup alongside file-based paths.
- Fixed `vibeusage auth --status` / `init` setup detection for providers that are available via strategy-specific credential sources (for example, keychain-backed OAuth credentials).
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

[unreleased]: https://github.com/joshuadavidthomas/vibeusage/compare/v0.3.0...HEAD
[0.1.0]: https://github.com/joshuadavidthomas/vibeusage/releases/tag/v0.1.0
[0.1.1]: https://github.com/joshuadavidthomas/vibeusage/releases/tag/v0.1.1
[0.2.0]: https://github.com/joshuadavidthomas/vibeusage/releases/tag/v0.2.0
[0.3.0]: https://github.com/joshuadavidthomas/vibeusage/releases/tag/v0.3.0
