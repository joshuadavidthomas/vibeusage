# vibeusage Implementation Plan

A CLI application to track usage stats from all LLM providers to understand session usage windows and costs.

**Target**: Python 3.14+
**Core Dependencies**: httpx, typer, rich, msgspec, platformdirs, tomli-w

---

## Current Status

**Implementation State**: Greenfield - only placeholder code exists

**Existing Code**:
- `src/vibeusage/__init__.py`: placeholder `main()` function (prints "Hello from vibeusage!")
- `pyproject.toml`: skeleton with no dependencies, placeholder description

**All phases below are incomplete and require implementation.**

---

## Spec Inconsistencies to Resolve

> These items differ between specifications and need resolution before/during implementation:

1. **Exit Codes**: Spec 05 defines codes 0-4; Spec 07 defines codes 0-5 (adds CONFIG_ERROR, PARTIAL_FAILURE). **Use Spec 07's definition.**

2. **JSON Output Modules**: Plan lists both `cli/json.py` and `display/json.py`. **Consolidate to `display/json.py` only**, with CLI importing from there.

3. **Browser Cookie Dependency**: Spec 03 mentions `browser_cookie3` or `pycookiecheat` but neither is in dependencies. **Add to pyproject.toml if browser cookie extraction is implemented** (currently stubbed).

4. **Cache Module Location**: Spec 01 shows `cache/` as separate top-level package with `snapshots.py` and `org_ids.py`; Spec 06 and this plan show `config/cache.py`. **Use `config/cache.py` as single module.**

5. **FetchPipelineResult vs FetchOutcome**: Spec 07 defines `FetchPipelineResult`; Spec 01 and this plan use `FetchOutcome`. **Standardize on `FetchOutcome`.**

6. **Exit Code 4 Naming**: Spec 05 calls it `INVALID_INPUT`; Spec 07 calls it `CONFIG_ERROR`. **Use `CONFIG_ERROR` (Spec 07).**

7. **Display Module Split**: Both `cli/display.py` and `display/rich.py` exist. **Clarify: `cli/display.py` for Rich renderables (UsageDisplay, ProviderPanel), `display/rich.py` for formatting utilities.**

8. **pace_to_color Location**: Spec 02 defines in data models section; this plan places in `display/colors.py`. **Keep in `display/colors.py` (presentation concern).**

9. **format_reset_time vs format_reset_countdown**: Different names for same function. **Use `format_reset_time()` in this plan.**

---

## Implementation Notes

> Recommendations based on spec analysis:

1. **Early Testing Recommended**: Consider adding basic unit tests after Phase 1 (data models) rather than deferring all testing to Phase 6. Model validation and error classification are easily testable.

2. **HTTP Client Dependency**: `core/http.py` is required by fetch, retry, and all provider strategies. Ensure it's implemented early in Phase 2.

3. **End-to-End Testing Gap**: The provider protocol (Phase 2) cannot be integration tested until Phase 3 (Claude provider). Plan for this gap.

4. **Display Module Clarity**: `cli/display.py` contains Rich renderables (UsageDisplay, ProviderPanel); `display/rich.py` contains utility functions. Keep this separation clear.

5. **msgspec Version**: Requires `msgspec>=0.18` for `indent` parameter in JSON encoding. Verify version in pyproject.toml.

6. **Use ty for type-checking**: Use `ty`, a new blazingly-fast Python type checker from Astral the team behind `uv` and `ruff`. Do not use `mypy` or `pyright` -- if either are found in `pyproject.toml` or elsewhere in the codebase, they should be replaced with `ty`.

---

## Phase 0: Project Setup

> Foundation - must complete before any implementation

- [ ] **Update pyproject.toml**
  - [ ] Add dependencies: `httpx>=0.27`, `typer>=0.12`, `rich>=13.0`, `msgspec>=0.18`, `platformdirs>=4.0`, `tomli-w>=1.0`
  - [ ] Replace placeholder description with: "Track usage across agentic LLM providers"
  - [ ] Add entry point: `vibeusage = "vibeusage.__main__:main"`

- [ ] **Create package structure**
  ```
  vibeusage/
  ├── __init__.py              # Package version
  ├── __main__.py              # Entry point: python -m vibeusage
  ├── models.py                # Data models (spec 02)
  ├── cli/
  │   ├── __init__.py          # CLI app and callback
  │   ├── atyper.py            # ATyper async wrapper for Typer
  │   ├── commands/
  │   │   ├── __init__.py
  │   │   ├── usage.py         # Main usage display commands
  │   │   ├── auth.py          # Authentication commands
  │   │   ├── status.py        # Provider status command
  │   │   ├── config.py        # Config subcommands
  │   │   ├── key.py           # Credential management
  │   │   └── cache.py         # Cache management
  │   ├── display.py           # Rich renderables (UsageDisplay, ProviderPanel)
  │   └── formatters.py        # Bar rendering, time formatting
  ├── auth/
  │   ├── __init__.py
  │   ├── base.py              # AuthStrategy, AuthResult, AuthCredentials
  │   ├── credentials.py       # OAuth2Credentials, SessionCredentials, etc.
  │   ├── oauth.py             # OAuth2Strategy
  │   ├── session.py           # ManualSessionStrategy
  │   ├── cookies.py           # BrowserCookieStrategy
  │   ├── apikey.py            # APIKeyStrategy
  │   ├── cli.py               # CLISessionStrategy
  │   ├── device_flow.py       # GitHubDeviceFlowStrategy
  │   └── local_process.py     # LocalProcessStrategy
  ├── core/
  │   ├── __init__.py
  │   ├── http.py              # HTTP client with connection pooling
  │   ├── fetch.py             # execute_fetch_pipeline
  │   ├── orchestrator.py      # fetch_all_providers, fetch_single_provider
  │   ├── aggregate.py         # AggregatedResult
  │   ├── gate.py              # FailureGate
  │   └── retry.py             # RetryConfig, with_retry
  ├── providers/
  │   ├── __init__.py          # Provider registry
  │   └── base.py              # Provider protocol, ProviderMetadata
  ├── strategies/
  │   ├── __init__.py
  │   ├── base.py              # FetchStrategy, FetchResult
  │   └── status.py            # Status page fetching utilities
  ├── config/
  │   ├── __init__.py
  │   ├── paths.py             # Platform-specific paths
  │   ├── settings.py          # Config structs and load/save
  │   ├── credentials.py       # Credential file management
  │   ├── cache.py             # Snapshot and org ID caching
  │   └── keyring.py           # Optional keyring integration
  ├── errors/
  │   ├── __init__.py
  │   ├── types.py             # ErrorCategory, ErrorSeverity, VibeusageError
  │   ├── http.py              # HTTP error classification
  │   ├── messages.py          # Provider-specific error messages
  │   └── classify.py          # Exception classification
  └── display/
      ├── __init__.py
      ├── rich.py              # Rich-based rendering
      ├── json.py              # JSON output
      └── colors.py            # Pace-based coloring
  ```

---

## Phase 1: Data Models & Core Types

> Define the contract - all data structures from spec 02, 03, 07

### Data Models (`vibeusage/models.py`)
- [ ] **PeriodType enum** - `session`, `daily`, `weekly`, `monthly` with `hours` property
- [ ] **UsagePeriod struct** - name, utilization, period_type, resets_at, model
  - [ ] `remaining()` method - return 100 - utilization
  - [ ] `elapsed_ratio()` method - calculate time elapsed in period
  - [ ] `pace_ratio()` method - calculate usage pace for coloring
  - [ ] `time_until_reset()` method - return timedelta to reset
- [ ] **OverageUsage struct** - used, limit, currency, is_enabled (Decimal for precision)
  - [ ] `remaining()` method
  - [ ] `utilization()` method
- [ ] **ProviderIdentity struct** - email, organization, plan (all optional)
- [ ] **StatusLevel enum** - operational, degraded, partial_outage, major_outage, unknown
- [ ] **ProviderStatus struct** - level, description, updated_at
  - [ ] `operational()` factory method
  - [ ] `unknown()` factory method
- [ ] **UsageSnapshot struct** - provider, fetched_at, periods, overage, identity, status, source
  - [ ] `primary_period()` method - return shortest period
  - [ ] `secondary_period()` method - return next period (not model-specific)
  - [ ] `model_periods()` method - return model-specific periods
  - [ ] `is_stale()` method - check if older than threshold
- [ ] **Validation functions** - `validate_usage_period()`, `validate_snapshot()`

### Auth Types (`vibeusage/auth/`)
- [ ] **AuthCredentials protocol** - `is_expired()`, `to_headers()`
- [ ] **OAuth2Credentials struct** - access_token, refresh_token, expires_at, token_type, scope
  - [ ] `can_refresh()` method
- [ ] **SessionCredentials struct** - session_key, cookie_name, expires_at
- [ ] **APIKeyCredentials struct** - api_key, header_name, prefix
- [ ] **CLICredentials struct** - command (marker for CLI delegation)
- [ ] **LocalProcessCredentials struct** - csrf_token, port, host
- [ ] **AuthResult struct** - success, credentials, error, source with factory methods

### Error Types (`vibeusage/errors/`)
- [ ] **ErrorCategory enum** - authentication, authorization, rate_limited, network, provider, parse, configuration, not_found, unknown
- [ ] **ErrorSeverity enum** - fatal, recoverable, transient, warning
- [ ] **VibeusageError struct** - message, category, severity, provider, remediation, details, timestamp
- [ ] **HTTPErrorMapping struct** - category, severity, should_retry, should_fallback
- [ ] **HTTP_ERROR_MAPPINGS dict** - mapping for 401, 403, 404, 429, 500-504
- [ ] **classify_http_error()** function

### Strategy Types (`vibeusage/strategies/base.py`)
- [ ] **FetchResult struct** - success, snapshot, error, should_fallback
  - [ ] `ok()`, `fail()`, `fatal()` factory methods
- [ ] **FetchStrategy ABC** - name property, `is_available()`, `fetch()`
- [ ] **FetchAttempt struct** - strategy, success, error, duration_ms
- [ ] **FetchOutcome struct** - provider_id, success, snapshot, source, attempts, error

---

## Phase 2: Core Infrastructure

> Enable provider work - config, auth base, orchestration

### Configuration System (`vibeusage/config/`)

- [ ] **Platform paths** (`paths.py`)
  - [ ] `config_dir()` - user config directory (respects `VIBEUSAGE_CONFIG_DIR`)
  - [ ] `cache_dir()` - user cache directory (respects `VIBEUSAGE_CACHE_DIR`)
  - [ ] `credentials_dir()` - credentials subdirectory
  - [ ] `snapshots_dir()` - cached snapshots
  - [ ] `org_ids_dir()` - cached org IDs
  - [ ] `config_file()` - main config.toml path

- [ ] **Config structs** (`settings.py`)
  - [ ] `DisplayConfig` - show_remaining, pace_colors, reset_format
  - [ ] `FetchConfig` - timeout, max_concurrent, stale_threshold
  - [ ] `CredentialsConfig` - use_keyring, reuse_provider_credentials
  - [ ] `ProviderConfig` - auth_source, preferred_browser, enabled
  - [ ] `Config` - enabled_providers, display, fetch, credentials, providers dict
    - [ ] `get_provider_config(provider_id)` method - returns config with defaults
    - [ ] `is_provider_enabled(provider_id)` method - checks if provider enabled
  - [ ] `Config.load()` - load from TOML with msgspec.convert()
  - [ ] `Config.save()` - save to TOML with msgspec.to_builtins()
  - [ ] `get_config()` singleton
  - [ ] `reload_config()` function
  - [ ] **Environment variable support**:
    - [ ] `VIBEUSAGE_ENABLED_PROVIDERS` - comma-separated provider list override
    - [ ] `VIBEUSAGE_NO_COLOR` - disable colored output

- [ ] **Credential management** (`credentials.py`)
  - [ ] `credential_path()` - get path for provider/credential_type
  - [ ] `PROVIDER_CREDENTIAL_PATHS` - dict of provider CLI credential locations
  - [ ] `find_provider_credential()` - check vibeusage then provider CLIs
  - [ ] `write_credential()` - secure write with 0o600 permissions
  - [ ] `read_credential()` - read credential file
  - [ ] `delete_credential()` - delete credential file
  - [ ] `check_credential_permissions()` - verify secure permissions
  - [ ] `check_provider_credentials(provider_id)` - returns (status, source) tuple

- [ ] **Cache management** (`cache.py`)
  - [ ] `snapshot_path()` - path for provider snapshot
  - [ ] `cache_snapshot()` - save snapshot with msgspec
  - [ ] `load_cached_snapshot()` - load cached snapshot
  - [ ] `is_snapshot_fresh()` - check against stale threshold
  - [ ] `org_id_path()`, `cache_org_id()`, `load_cached_org_id()`
  - [ ] `clear_org_id_cache()`, `clear_provider_cache()`, `clear_all_cache()`

- [ ] **Optional keyring** (`keyring.py`)
  - [ ] `use_keyring()` - check if enabled and available
  - [ ] `keyring_key(provider_id, credential_type)` - generate keyring key
  - [ ] `store_in_keyring()`, `get_from_keyring()`, `delete_from_keyring()`

### Authentication Layer (`vibeusage/auth/`)

- [ ] **AuthStrategy ABC** (`base.py`)
  - [ ] `name` property
  - [ ] `is_available()` - fast check, no network
  - [ ] `authenticate()` -> AuthResult
  - [ ] `refresh()` - default re-authenticates

- [ ] **ProviderAuthConfig** (`base.py`)
  - [ ] Coordinates auth strategy chain per provider
  - [ ] `strategies: list[AuthStrategy]` - ordered strategy list
  - [ ] `authenticate()` - iterate strategies, return first success
  - [ ] Aggregate errors from all failed strategies

- [ ] **OAuth2Strategy** (`oauth.py`)
  - [ ] `OAuth2Config` struct - token_endpoint, client_id, client_secret, scope, credentials_file
  - [ ] Load credentials from file
  - [ ] Token refresh with httpx
  - [ ] Save refreshed credentials

- [ ] **BrowserCookieStrategy** (`cookies.py`)
  - [ ] `CookieConfig` struct - cookie_names, domains, stored_session_file
  - [ ] Try stored session first
  - [ ] Browser priority: Safari, Chrome, Firefox, Arc, Brave, Edge
  - [ ] `_try_browser()`, `_extract_cookie()` (stub for later implementation)
  - [ ] Save successful session

- [ ] **ManualSessionStrategy** (`session.py`)
  - [ ] Load session key from file

- [ ] **APIKeyStrategy** (`apikey.py`)
  - [ ] Try environment variable first, then file

- [ ] **CLISessionStrategy** (`cli.py`)
  - [ ] `CLIConfig` struct - command, usage_args, version_args
  - [ ] Check if CLI in PATH
  - [ ] Verify CLI works with version check
  - [ ] Return CLICredentials marker

- [ ] **GitHubDeviceFlowStrategy** (`device_flow.py`)
  - [ ] `DeviceFlowConfig` struct - client_id, device_code_url, token_url, scope
  - [ ] Request device code
  - [ ] Show user code and verification URL
  - [ ] Poll for token with interval
  - [ ] Handle error responses (authorization_pending, slow_down, expired_token, access_denied)

- [ ] **LocalProcessStrategy** (`local_process.py`)
  - [ ] `LocalProcessConfig` struct - executable_name, port_range, csrf_endpoint
  - [ ] Detect running local process (e.g., Antigravity server)
  - [ ] Extract CSRF token and port from process
  - [ ] Return LocalProcessCredentials with csrf_token, port, host

### Fetch & Orchestration (`vibeusage/core/`)

- [ ] **Provider protocol** (`providers/base.py`)
  - [ ] `ProviderMetadata` struct - id, name, description, homepage, status_url, dashboard_url
  - [ ] `Provider` ABC - metadata (ClassVar), `fetch_strategies()`, `fetch_status()`, `is_enabled()`

- [ ] **Provider registry** (`providers/__init__.py`)
  - [ ] `_PROVIDERS` dict
  - [ ] `register_provider()` decorator
  - [ ] `get_provider()`, `get_all_providers()`, `list_provider_ids()`

- [ ] **Fetch pipeline** (`core/fetch.py`)
  - [ ] `execute_fetch_pipeline()` - try strategies in order
  - [ ] Handle timeouts with asyncio.wait_for
  - [ ] Track attempts for debugging
  - [ ] Return FetchOutcome

- [ ] **Orchestrator** (`core/orchestrator.py`)
  - [ ] `fetch_all_providers()` - concurrent fetch with semaphore (max_concurrent=5)
  - [ ] `fetch_single_provider()` - single provider fetch
  - [ ] `on_complete` callback for progress

- [ ] **Result aggregation** (`core/aggregate.py`)
  - [ ] `AggregatedResult` struct - snapshots dict, errors dict, fetched_at
    - [ ] `successful_providers()` method - returns list of successful provider IDs
    - [ ] `failed_providers()` method - returns list of failed provider IDs
  - [ ] `aggregate_results()` function

- [ ] **Retry logic** (`core/retry.py`)
  - [ ] `RetryConfig` struct - max_attempts, base_delay, max_delay, exponential_base, jitter
  - [ ] `calculate_retry_delay()` with exponential backoff
  - [ ] `with_retry()` async decorator
  - [ ] `should_retry_exception()` classifier

- [ ] **Failure gate** (`core/gate.py`)
  - [ ] `FailureRecord` struct - timestamp, error_category, message
  - [ ] `FailureGate` class with constants:
    - [ ] `MAX_CONSECUTIVE_FAILURES = 3`
    - [ ] `GATE_DURATION = timedelta(minutes=5)`
    - [ ] `WINDOW_DURATION = timedelta(minutes=10)`
  - [ ] `record_failure()`, `record_success()`, `is_gated()`, `gate_remaining()`
  - [ ] `recent_failures()` - returns recent failures for diagnostics
  - [ ] `get_failure_gate()` - global singleton accessor

- [ ] **HTTP client** (`core/http.py`)
  - [ ] `get_http_client()` context manager with connection pooling
  - [ ] `cleanup()` - close client on shutdown
  - [ ] `get_timeout_config()` from settings

- [ ] **Cache-aware fetch** (`core/fetch.py`)
  - [ ] `FetchResultWithFallback` struct - snapshot, is_fresh, error, stale_age_minutes
  - [ ] `fetch_with_cache_fallback()` - fetch or return cached
  - [ ] `fetch_with_gate()` - respect failure gate

- [ ] **Multi-provider fetch** (`core/orchestrator.py`)
  - [ ] `MultiProviderResult` struct - successes, failures, stale dicts
    - [ ] `has_any_data` property - checks if any data available
    - [ ] `all_failed` property - checks if all providers failed
  - [ ] `fetch_all_with_partial_failure()`

---

## Phase 3: Claude Provider (MVP)

> First end-to-end provider implementation

### Provider Module (`vibeusage/providers/claude/`)

- [ ] **ClaudeProvider** (`__init__.py`)
  - [ ] ProviderMetadata: id="claude", name="Claude", homepage="https://claude.ai"
  - [ ] status_url="https://status.anthropic.com"
  - [ ] dashboard_url="https://claude.ai/settings/usage"
  - [ ] `fetch_strategies()` returns [OAuth, Web, CLI]
  - [ ] `fetch_status()` - query Statuspage.io

- [ ] **ClaudeOAuthStrategy** (`oauth.py`)
  - [ ] Credential sources: `~/.claude/.credentials.json`, vibeusage storage
  - [ ] Token refresh: `POST https://api.anthropic.com/oauth/token`
  - [ ] Usage endpoint: `GET https://api.anthropic.com/api/oauth/usage`
  - [ ] Header: `anthropic-beta: oauth-2025-04-20`
  - [ ] `parse_oauth_response()` - extract five_hour, seven_day, opus, sonnet periods

- [ ] **ClaudeWebStrategy** (`web.py`)
  - [ ] Cookie: `sessionKey`
  - [ ] Get organizations: `GET https://claude.ai/api/organizations`
  - [ ] Select org with "chat" capability (Claude Max)
  - [ ] Cache org_id to `~/.cache/vibeusage/org-ids/claude`
  - [ ] Get usage: `GET https://claude.ai/api/organizations/{org_id}/usage`
  - [ ] Get overage: `GET https://claude.ai/api/organizations/{org_id}/overage_spend_limit`
  - [ ] `parse_overage()` - extract OverageUsage

- [ ] **ClaudeCLIStrategy** (`cli.py`)
  - [ ] Check for `claude` binary in PATH
  - [ ] Execute `claude /usage`
  - [ ] `parse_cli_output()` - strip ANSI, extract percentages with regex

- [ ] **Status polling**
  - [ ] `fetch_statuspage_status()` in `strategies/status.py`
  - [ ] Parse Statuspage.io JSON response
  - [ ] Map indicator to StatusLevel

---

## Phase 4: CLI & Display

> User-facing commands and output

### CLI Framework (`vibeusage/cli/`)

- [ ] **ATyper wrapper** (`atyper.py`)
  - [ ] Extend Typer to support async commands
  - [ ] Override `callback()` and `command()` methods
  - [ ] `async_me_maybe()` - wrap coroutines with asyncio.run

- [ ] **Main app** (`__init__.py`)
  - [ ] Create ATyper app with name="vibeusage"
  - [ ] Global options: --json/-j, --no-color, --verbose/-v, --quiet/-q, --version
  - [ ] Callback: invoke_without_command=True for default behavior
  - [ ] Register subcommand typers: auth, config, key, cache, status

- [ ] **Usage commands** (`commands/usage.py`)
  - [ ] Default command: show all enabled providers
  - [ ] Provider-specific commands: claude, codex, copilot, cursor, gemini
  - [ ] --refresh flag to bypass cache
  - [ ] JSON output mode with msgspec

- [ ] **Auth commands** (`commands/auth.py`)
  - [ ] `vibeusage auth <provider>` - trigger auth flow
  - [ ] `vibeusage auth --status` - show auth status for all providers
  - [ ] Claude: session key prompt with validation
  - [ ] Copilot: device flow with spinner
  - [ ] Cursor: session key prompt

- [ ] **Status command** (`commands/status.py`)
  - [ ] Fetch provider health statuses
  - [ ] Display table with symbols (●, ◐, ◑, ○, ?)
  - [ ] JSON output mode

- [ ] **Config commands** (`commands/config.py`)
  - [ ] `vibeusage config show` - display current settings
  - [ ] `vibeusage config path` - show directory paths
  - [ ] `vibeusage config reset` - reset to defaults (with confirmation)
  - [ ] `vibeusage config edit` - open in $EDITOR

- [ ] **Key commands** (`commands/key.py`)
  - [ ] `vibeusage key` - show credential status for all providers
  - [ ] `vibeusage key <provider>` - show credential status
  - [ ] `vibeusage key <provider> set` - set credential (interactive)
  - [ ] `vibeusage key <provider> delete` - delete credential

- [ ] **Cache commands** (`commands/cache.py`)
  - [ ] `vibeusage cache show` - show cache status per provider
  - [ ] `vibeusage cache clear [provider]` - clear cache

- [ ] **Entry point** (`__main__.py`)
  - [ ] `main()` function calling `app()`
  - [ ] Handle KeyboardInterrupt gracefully

### Display Components (`vibeusage/cli/`)

- [ ] **Formatters** (`formatters.py`)
  - [ ] `render_usage_bar()` - Unicode block chars (█░)
  - [ ] `format_reset_time()` - countdown string (4d 12h, 3h 45m, <1m)
  - [ ] `get_usage_style()` - Rich Style from pace ratio
  - [ ] `format_usage_line()` - Rich Text with bar + percentage

- [ ] **Display classes** (`display.py`)
  - [ ] `UsageDisplay` - __rich_console__ renderable for single provider
  - [ ] `ProviderPanel` - provider wrapped in panel for multi-view
  - [ ] `StatusDisplay` - status table renderable

- [ ] **Error display** (integrate with `errors/`)
  - [ ] `show_error()` - formatted error panel
  - [ ] `show_partial_failures()` - summary of failed providers
  - [ ] `show_stale_warning()` - cached data indicator

- [ ] **Exit codes** (`cli/__init__.py`)
  - [ ] `ExitCode` class: SUCCESS=0, GENERAL_ERROR=1, AUTH_ERROR=2, NETWORK_ERROR=3, CONFIG_ERROR=4, PARTIAL_FAILURE=5
  - [ ] `exit_code_for_error()`, `exit_code_for_result()`

- [ ] **Shell completions**
  - [ ] `complete_provider()` - autocomplete provider names
  - [ ] Enable Typer's built-in completion

### Display Module (`vibeusage/display/`)

- [ ] **Pace-based coloring** (`colors.py`)
  - [ ] `pace_to_color()` - return color name from pace ratio
  - [ ] Pace thresholds: ≤1.15 green, 1.15-1.30 yellow, >1.30 red
  - [ ] Fallback thresholds: <50% green, 50-80% yellow, >80% red

- [ ] **Rich output** (`rich.py`)
  - [ ] Provider-specific Rich formatting utilities

- [ ] **JSON output** (`json.py`)
  - [ ] `output_json()` - print msgspec.json.encode to stdout
  - [ ] `output_json_pretty()` - with indent=2
  - [ ] Helper for CLI commands to import

---

## Phase 5: Additional Providers

> Expand provider coverage

### Phase 5a: Codex/OpenAI
- [ ] **CodexProvider** (`providers/codex/__init__.py`)
  - [ ] ProviderMetadata with ChatGPT URLs
  - [ ] status_url="https://status.openai.com"
- [ ] **CodexOAuthStrategy** (`providers/codex/oauth.py`)
  - [ ] Credential sources: `~/.codex/auth.json`, vibeusage storage
  - [ ] Client ID: `app_EMoamEEZ73f0CkXaXp7hrann`
  - [ ] Token refresh: `POST https://auth.openai.com/oauth/token`
  - [ ] Usage endpoint: `GET https://chatgpt.com/backend-api/wham/usage`
  - [ ] Check config.toml for `usage_url` override
  - [ ] Parse rate_limits.primary/secondary, credits

### Phase 5b: Copilot
- [ ] **CopilotProvider** (`providers/copilot/__init__.py`)
  - [ ] status_url="https://www.githubstatus.com"
  - [ ] dashboard_url="https://github.com/settings/copilot"
- [ ] **CopilotDeviceFlowStrategy** (`providers/copilot/device_flow.py`)
  - [ ] Client ID: `Iv1.b507a08c87ecfe98` (VS Code client ID)
  - [ ] Scope: `read:user`
  - [ ] Usage endpoint: `GET https://api.github.com/copilot_internal/user`
  - [ ] Parse premium_interactions, chat quotas (MONTHLY periods)

### Phase 5c: Cursor
- [ ] **CursorProvider** (`providers/cursor/__init__.py`)
  - [ ] status_url="https://status.cursor.com"
  - [ ] dashboard_url="https://cursor.com/settings/usage"
- [ ] **CursorWebStrategy** (`providers/cursor/web.py`)
  - [ ] Cookie names: `WorkosCursorSessionToken`, `__Secure-next-auth.session-token`, `next-auth.session-token`
  - [ ] Domains: `cursor.com`, `cursor.sh`
  - [ ] Usage: `POST https://www.cursor.com/api/usage-summary`
  - [ ] User info: `GET https://www.cursor.com/api/auth/me`
  - [ ] Parse premium_requests, billing_cycle, on_demand_spend (overage)

### Phase 5d: Gemini
- [ ] **GeminiProvider** (`providers/gemini/__init__.py`)
  - [ ] dashboard_url="https://aistudio.google.com/app/usage"
  - [ ] status_url=None (uses Google Workspace)
- [ ] **GeminiOAuthStrategy** (`providers/gemini/oauth.py`)
  - [ ] Credential sources: `~/.gemini/oauth_creds.json`, vibeusage storage
  - [ ] Token refresh: `POST https://oauth2.googleapis.com/token`
  - [ ] Quota endpoint: `POST https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota`
  - [ ] Parse quota_buckets with per-model DAILY periods
  - [ ] User tier from loadCodeAssist endpoint

### Phase 5e: Future Providers (Post-MVP)

> Not included in initial release, but documented in specs for later

- [ ] **Augment** - Browser cookies + manual session
- [ ] **Factory (Droid)** - Browser cookies + manual session
- [ ] **VertexAI** - OAuth via gcloud ADC (`~/.config/gcloud/application_default_credentials.json`)
- [ ] **MiniMax** - Browser cookies + bearer token
- [ ] **Antigravity** - LocalProcessStrategy
- [ ] **Kiro** - CLI-based
- [ ] **Zai** - API key (env var → file)

---

## Phase 6: Polish & Robustness

> Error handling refinement, caching, UX improvements

### Error Handling Refinement
- [ ] **Exception classification** (`errors/classify.py`)
  - [ ] `classify_exception()` - any exception to VibeusageError
  - [ ] `classify_network_error()` - httpx-specific error handling
  - [ ] Handle httpx, JSON, file errors
- [ ] **HTTP error handling** (`errors/http.py`)
  - [ ] `handle_http_request()` - async function with automatic retry
  - [ ] `classify_http_status_error()`
- [ ] **Provider error messages** (`errors/messages.py`)
  - [ ] `AUTH_ERROR_TEMPLATES` dict per provider
  - [ ] `get_auth_error_message()` function
- [ ] **Helper utilities** (`errors/` or `display/`)
  - [ ] `format_timedelta()` - format timedelta for gate messages
  - [ ] `calculate_age_minutes()` - calculate snapshot age

### Reliability
- [ ] **Offline mode** - serve cached data when network unavailable
- [ ] **Graceful degradation** - continue with partial results
- [ ] **Timeout handling** - configurable per-provider timeouts
- [ ] **Rate limit handling** - respect Retry-After headers
- [ ] **Failure gate persistence** - persist gate state to cache file

### User Experience
- [ ] **Progress indicators** - Rich Status/Progress for slow fetches
- [ ] **Helpful error messages** - all errors include remediation
- [ ] **First-run experience** - create default config, guide setup
- [ ] **Verbose diagnostics** - `show_verbose_error()`, `show_diagnostic_info()`
  - [ ] Display version, platform, Python version
  - [ ] Show config/cache directory paths
  - [ ] List credential status per provider
  - [ ] Show failure gate status
  - [ ] Display recent fetch attempt history
- [ ] **Stale data display**
  - [ ] `display_with_staleness()` - display with yellow staleness warning
  - [ ] `display_multi_provider_result()` - display partial results with failures
- [ ] **Legacy migration** - `migrate_legacy_config()` for ccusage users

### JSON Output
- [ ] **Error response struct** - `ErrorResponse` for JSON errors
- [ ] **Multi-provider response** - `MultiProviderResponse` struct
- [ ] **format_json_error()**, **format_json_result()** functions

### Testing
- [ ] **Unit tests** - models, parsing, validation, error classification
- [ ] **Integration tests** - provider fetch with mocked APIs
- [ ] **CLI tests** - command behavior, output format, exit codes
- [ ] **Error scenario tests** - all error paths tested

### Documentation
- [ ] **README.md** - installation, quick start, examples
- [ ] **Provider setup docs** - per-provider auth instructions
- [ ] **Config reference** - all settings documented
- [ ] **AGENTS.md** - update with build/test commands

---

## Implementation Order Summary

1. **Phase 0** - Project setup (dependencies, structure)
2. **Phase 1** - Data models and core types (contracts first)
3. **Phase 2** - Infrastructure (config, auth, orchestration)
4. **Phase 3** - Claude provider (end-to-end MVP)
5. **Phase 4** - CLI and display (user-facing)
6. **Phase 5** - Additional providers (iterate)
7. **Phase 6** - Polish and robustness (production-ready)

### MVP Milestone (Phases 0-4)
After completing phases 0-4, vibeusage should:
- Fetch Claude usage via OAuth, web, or CLI
- Display usage with pace-based coloring
- Handle errors gracefully with cached fallback
- Support --json output for scripting

### Full Release Milestone (All Phases)
After all phases:
- Support 5 providers: Claude, Codex, Copilot, Cursor, Gemini
- Complete CLI with auth, config, cache management
- Robust error handling with retry and failure gates
- Full test coverage
