# vibeusage Implementation Plan

A CLI application to track usage stats from all LLM providers to understand session usage windows and costs.

**Target**: Python 3.14+
**Core Dependencies**: httpx, typer, rich, msgspec, platformdirs, tomli-w

---

## Current Status

**Implementation State**: Phase 0-4 complete (Claude MVP functional), Phase 5-6 in progress

**Completed** (100% functional):
- ✓ Phase 0: Project setup (dependencies, structure, entry points)
- ✓ Phase 1: Data models and core types (models.py)
- ✓ Phase 2: Core infrastructure (config, orchestration, retry, gate, fetch, http)
- ✓ Phase 3: Claude provider (OAuth, Web, CLI strategies, status polling)
- ✓ Phase 4: CLI framework (ATyper, app, usage/status/config/key/cache commands)
- ✓ Display module (rich.py, json.py - formatters and utilities)
- ✓ Error classification (types.py, classify.py, http.py, network.py)
- ✓ Error display (cli/display.py with Rich renderables)
- ✓ Error messages (errors/messages.py with provider templates)
- ✓ Provider registry and base protocol
- ✓ Configuration system (paths, settings, credentials, cache, keyring)
- ✓ Test suite (303 passing tests, 44% coverage)

**Recent Fixes** (v0.0.1):
- Created missing `errors/classify.py` module
- Fixed Claude OAuth period mapping bug: `seven_day` → `WEEKLY`
- Implemented `display/` module with Rich formatters and JSON output
- Implemented `cli/display.py` with UsageDisplay, ProviderPanel, ErrorDisplay
- Implemented `errors/messages.py` with AUTH_ERROR_TEMPLATES for 5 providers
- Implemented `errors/http.py` with handle_http_request() retry logic
- Implemented `errors/network.py` with network error classification
- Implemented `tests/` suite with pytest infrastructure
  - 303 passing tests covering models, errors, config, display, core, CLI, and providers
  - 44% code coverage with pytest-cov
  - Comprehensive fixtures in conftest.py
  - Tests organized by module (unit, integration, CLI, error scenarios)

**Partially Implemented**:
- ⚠️ auth/base.py: Base classes only - concrete strategies implemented in provider modules

**NOT Implemented** (blocking full release):
- ❌ providers/codex/ - Entire provider (OAuth strategy)
- ❌ providers/copilot/ - Entire provider (device flow strategy)
- ❌ providers/cursor/ - Entire provider (web strategy)
- ❌ providers/gemini/ - Entire provider (OAuth strategy)

**Minor Issues** (non-blocking):
- ProviderStatus factory methods have wrong return type hints (returns `type[ProviderStatus]` instead of `ProviderStatus`)
- `pace_to_color()` in models.py instead of display/colors.py (functional but location differs from spec)

---

## Remaining Work (Prioritized by Dependencies & Value)

### Priority 1: Complete Claude Provider Experience (MVP++)
**Goal**: Make Claude provider fully functional with auth management

- [x] **Implement auth command** (cli/commands/auth.py)
  - [x] `vibeusage auth <provider>` - trigger provider-specific auth flow
  - [x] `vibeusage auth --status` - show auth status for all providers
  - [x] Claude: session key prompt with validation
  - [x] Integrate with ClaudeWebStrategy to save session credentials
  - [x] Rich table output with auth status indicators

- [x] **Implement error display utilities** (cli/display.py errors/)
  - [x] `UsageDisplay` class - __rich_console__ renderable for single provider
  - [x] `ProviderPanel` class - provider wrapped in panel for multi-view
  - [x] `show_error()` - formatted error panel with remediation
  - [x] `show_partial_failures()` - summary of failed providers
  - [x] `show_stale_warning()` - cached data indicator with age

- [x] **Add error message templates** (errors/messages.py)
  - [x] `AUTH_ERROR_TEMPLATES` dict per provider (Claude, Codex, Copilot, Cursor, Gemini)
  - [x] `get_auth_error_message(provider_id, error)` function
  - [x] Include remediation steps for each auth error type
  - [x] Map to VibeusageError.remediation field
  - [x] Additional modules: errors/http.py, errors/network.py

- [ ] **Fix minor type issues**
  - [ ] Fix ProviderStatus factory method return type hints
  - [x] Export `classify_exception` from errors/__init__.py
  - [ ] Consider moving `pace_to_color()` to display/colors.py for consistency

**Value**: Completes end-to-end Claude experience with proper auth flow and error handling

---

### Priority 2: Codex/OpenAI Provider
**Goal**: Add second most valuable provider after Claude

- [ ] **Create provider module** (providers/codex/)
  - [ ] `__init__.py` - CodexProvider with metadata (status_url, dashboard_url)
  - [ ] `oauth.py` - CodexOAuthStrategy implementation
    - [ ] Credential sources: `~/.codex/auth.json`, vibeusage storage
    - [ ] Client ID: `app_EMoamEEZ73f0CkXaXp7hrann`
    - [ ] Token refresh: `POST https://auth.openai.com/oauth/token`
    - [ ] Usage endpoint: `GET https://chatgpt.com/backend-api/wham/usage`
    - [ ] Check config.toml for `usage_url` override
    - [ ] Parse response: rate_limits.primary/secondary, credits
    - [ ] Map to UsageSnapshot with appropriate periods

- [ ] **Register provider** (providers/__init__.py)
  - [ ] Add CodexProvider to registry
  - [ ] Verify CLI commands discover provider
  - [ ] Test `vibeusage codex` command

- [ ] **Add auth support** (cli/commands/auth.py)
  - [ ] Codex auth flow implementation
  - [ ] OAuth flow with browser redirect
  - [ ] Credential storage integration

**Value**: High - ChatGPT/Claude are the two most requested providers

**Dependencies**: Priority 1 (auth command infrastructure)

---

### Priority 3: Copilot Provider
**Goal**: Add GitHub Copilot support for developers

- [ ] **Create provider module** (providers/copilot/)
  - [ ] `__init__.py` - CopilotProvider with metadata
    - [ ] status_url="https://www.githubstatus.com"
    - [ ] dashboard_url="https://github.com/settings/copilot"
  - [ ] `device_flow.py` - CopilotDeviceFlowStrategy
    - [ ] Client ID: `Iv1.b507a08c87ecfe98` (VS Code client ID)
    - [ ] Scope: `read:user`
    - [ ] Device code endpoint + token polling
    - [ ] Usage endpoint: `GET https://api.github.com/copilot_internal/user`
    - [ ] Parse: premium_interactions, chat quotas (MONTHLY periods)
    - [ ] Map to UsageSnapshot

- [ ] **Add auth support**
  - [ ] GitHub device flow in auth command
  - [ ] Spinner/polling UI during auth
  - [ ] Credential storage

- [ ] **Register and test**
  - [ ] Add to provider registry
  - [ ] Test `vibeusage copilot` command
  - [ ] Verify auth flow completes

**Value**: Medium-High - Popular developer tool

**Dependencies**: Priority 1 (auth command infrastructure)

---

### Priority 4: Cursor Provider
**Goal**: Add Cursor IDE usage tracking

- [ ] **Create provider module** (providers/cursor/)
  - [ ] `__init__.py` - CursorProvider with metadata
    - [ ] status_url="https://status.cursor.com"
    - [ ] dashboard_url="https://cursor.com/settings/usage"
  - [ ] `web.py` - CursorWebStrategy
    - [ ] Cookie names: `WorkosCursorSessionToken`, `__Secure-next-auth.session-token`, `next-auth.session-token`
    - [ ] Domains: `cursor.com`, `cursor.sh`
    - [ ] Usage: `POST https://www.cursor.com/api/usage-summary`
    - [ ] User info: `GET https://www.cursor.com/api/auth/me`
    - [ ] Parse: premium_requests, billing_cycle, on_demand_spend (overage)

- [ ] **Add auth support**
  - [ ] Session key extraction from browser cookies
  - [ ] Manual session key entry fallback
  - [ ] Cookie file management

- [ ] **Register and test**
  - [ ] Add to provider registry
  - [ ] Test `vibeusage cursor` command

**Value**: Medium - Growing user base among AI developers

**Dependencies**: Priority 1 (auth command infrastructure + cookie handling)

---

### Priority 5: Gemini Provider
**Goal**: Add Google Gemini Studio support

- [ ] **Create provider module** (providers/gemini/)
  - [ ] `__init__.py` - GeminiProvider with metadata
    - [ ] dashboard_url="https://aistudio.google.com/app/usage"
    - [ ] status_url=None (uses Google Workspace status)
  - [ ] `oauth.py` - GeminiOAuthStrategy
    - [ ] Credential sources: `~/.gemini/oauth_creds.json`, vibeusage storage
    - [ ] Token refresh: `POST https://oauth2.googleapis.com/token`
    - [ ] Quota endpoint: `POST https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota`
    - [ ] Parse: quota_buckets with per-model DAILY periods
    - [ ] User tier from loadCodeAssist endpoint

- [ ] **Add auth support**
  - [ ] Google OAuth flow
  - [ ] Credential storage for OAuth tokens

- [ ] **Register and test**
  - [ ] Add to provider registry
  - [ ] Test `vibeusage gemini` command

**Value**: Medium - Fourth most requested provider

**Dependencies**: Priority 1 (auth command infrastructure)

---

### Priority 6: Polish & Robustness
**Goal**: Production-ready error handling and UX

#### Error Handling Enhancement
- [ ] **HTTP error classification** (errors/http.py)
  - [ ] `handle_http_request()` - async function with automatic retry
  - [ ] `classify_http_status_error()` - map status codes to ErrorCategory
  - [ ] Integrate with retry logic in core/retry.py

- [ ] **Network error handling** (errors/network.py)
  - [ ] `classify_network_error()` - httpx-specific error handling
  - [ ] Map httpx.ConnectTimeout, ReadTimeout, NetworkError to categories
  - [ ] Provide retry recommendations for transient failures

- [ ] **Helper utilities**
  - [ ] `format_timedelta()` - format timedelta for gate messages
  - [ ] `calculate_age_minutes()` - calculate snapshot age for staleness

#### User Experience Improvements
- [ ] **Progress indicators**
  - [ ] Rich Status spinner during slow fetches
  - [ ] Progress bar for multi-provider fetch
  - [ ] Show which provider is currently being fetched

- [ ] **First-run experience**
  - [ ] Detect missing config and create defaults
  - [ ] Interactive setup wizard on first run
  - [ ] Prompt for initial provider auth

- [ ] **Verbose diagnostics** (`--verbose/-v` flag)
  - [ ] `show_diagnostic_info()` - display version, platform, Python version
  - [ ] Show config/cache directory paths
  - [ ] List credential status per provider
  - [ ] Show failure gate status
  - [ ] Display recent fetch attempt history with timing

- [ ] **Stale data display**
  - [ ] `display_with_staleness()` - yellow warning banner for stale data
  - [ ] `display_multi_provider_result()` - show partial results with failures
  - [ ] Age indicator in provider panels

#### JSON Output Enhancement
- [ ] **Error response struct**
  - [ ] `ErrorResponse` msgspec struct for JSON error output
  - [ ] Include error category, severity, remediation
- [ ] **Multi-provider response**
  - [ ] `MultiProviderResponse` struct with successes, failures, stale
- [ ] **Formatter functions**
  - [ ] `format_json_error()` - convert VibeusageError to JSON
  - [ ] `format_json_result()` - convert AggregatedResult to JSON

#### Reliability Features
- [ ] **Offline mode**
  - [ ] Detect network unavailability
  - [ ] Serve cached data when offline
  - [ ] Display "offline mode" indicator
- [ ] **Graceful degradation**
  - [ ] Continue with partial results if some providers fail
  - [ ] Clear separation of successful/failed providers in output
- [ ] **Timeout handling**
  - [ ] Configurable per-provider timeouts in config.toml
  - [ ] Respect timeout settings in fetch pipeline
- [ ] **Rate limit handling**
  - [ ] Parse Retry-After headers from HTTP responses
  - [ ] Wait before retry when rate limited
  - [ ] Display rate limit message to user
- [ ] **Failure gate persistence**
  - [ ] Save gate state to cache file
  - [ ] Restore gate state on startup
  - [ ] Clear stale gate entries on startup

**Value**: High - Makes tool production-ready and robust

**Dependencies**: All provider implementations (Priority 1-5)

---

### Priority 7: Test Suite
**Goal**: Ensure reliability and prevent regressions

#### Status: IN PROGRESS (44% coverage, 303 passing tests)

**Completed**:
- [x] **Test infrastructure** (pytest, pytest-asyncio, pytest-cov, pytest-mock)
- [x] **Fixtures** (conftest.py with comprehensive test fixtures)

#### Unit Tests
- [x] **Model validation tests** (tests/test_models.py)
  - [x] UsageSnapshot validation with all period types
  - [x] ProviderStatus factory methods
  - [x] Edge cases: negative utilization, future reset times, etc.
- [x] **Error classification tests** (tests/test_errors/)
  - [x] classify_exception() for all exception types
  - [x] HTTP status code mappings
  - [x] Network error mappings
- [x] **Config system tests** (tests/test_config/)
  - [x] Config.load() and Config.save()
  - [x] Provider config merging with defaults
  - [x] Environment variable overrides
- [x] **Credential management tests**
  - [x] Secure file permissions (0o600)
  - [x] Credential path resolution
  - [x] Provider credential discovery

#### Integration Tests
- [x] **Provider fetch tests** (tests/providers/test_claude.py)
  - [x] Claude OAuth strategy with mocked token endpoint
  - [x] Claude Web strategy with mocked usage endpoint
  - [x] Claude CLI strategy with mocked command output
- [x] **Fetch pipeline tests** (tests/test_core/test_fetch.py)
  - [x] Strategy fallback behavior
  - [x] Timeout handling
  - [x] Retry with exponential backoff
  - [x] Cache fallback behavior
- [x] **Orchestrator tests** (tests/test_core/test_orchestration.py)
  - [x] Concurrent fetch with semaphore
  - [x] Partial failure handling
  - [x] Result aggregation

#### CLI Tests
- [x] **Command behavior tests** (tests/test_cli/)
  - [x] `vibeusage` default command output
  - [x] `vibeusage <provider>` provider-specific commands
  - [x] `vibeusage auth` auth flow
  - [x] `vibeusage status` status table
  - [x] `vibeusage config show/path/edit`
  - [x] `vibeusage key` credential management
  - [x] `vibeusage cache show/clear`
- [x] **Output format tests**
  - [x] Rich output format validation
  - [x] JSON output structure validation
  - [x] --json flag behavior
- [x] **Exit code tests**
  - [x] ExitCode.SUCCESS for successful fetch
  - [x] ExitCode.AUTH_ERROR for auth failures
  - [x] ExitCode.NETWORK_ERROR for network issues
  - [x] ExitCode.CONFIG_ERROR for config problems
  - [x] ExitCode.PARTIAL_FAILURE for some providers failed

#### Error Scenario Tests
- [x] **Auth failure scenarios**
  - [x] Invalid credentials
  - [x] Expired tokens
  - [x] Missing credentials file
- [x] **Network failure scenarios**
  - [x] Timeout
  - [x] Connection refused
  - [x] DNS failure
- [x] **Provider failure scenarios**
  - [x] Provider API down
  - [x] Rate limiting
  - [x] Malformed API response
- [x] **Config error scenarios**
  - [x] Invalid TOML
  - [x] Missing required fields
  - [x] Invalid provider IDs

**Remaining Work**:
- [ ] Increase code coverage from 44% to 80%+
- [ ] Add tests for display module (rich.py, json.py)
- [ ] Add tests for CLI display utilities (cli/display.py)
- [ ] Add integration tests for unimplemented providers (Codex, Copilot, Cursor, Gemini)

**Value**: High - Essential for production reliability

**Dependencies**: Priority 6 (most features implemented)

---

### Priority 8: Documentation
**Goal**: Enable users to install, configure, and use vibeusage effectively

- [ ] **README.md**
  - [ ] Installation instructions (pip install, uv, etc.)
  - [ ] Quick start guide (first run, basic usage)
  - [ ] Supported providers list
  - [ ] Example output screenshots
  - [ ] Configuration examples
  - [ ] Troubleshooting section
- [ ] **Provider setup docs** (docs/providers/*.md)
  - [ ] Claude auth setup (OAuth, web session, CLI)
  - [ ] Codex auth setup (OAuth)
  - [ ] Copilot auth setup (device flow)
  - [ ] Cursor auth setup (browser cookies)
  - [ ] Gemini auth setup (OAuth)
- [ ] **Config reference** (docs/config.md)
  - [ ] All settings documented
  - [ ] Default values listed
  - [ ] Environment variables documented
  - [ ] Example config.toml
- [ ] **AGENTS.md** (update existing)
  - [ ] Add test commands: `uv run pytest`
  - [ ] Add lint commands: `uv run ruff check`
  - [ ] Add typecheck commands: `uv run ty`
  - [ ] Update build/release instructions

**Value**: Medium - Important for adoption but not blocking MVP

**Dependencies**: Priority 7 (features stable)

---

## Implementation Order Summary

### Immediate (Complete MVP)
1. **Priority 1**: Complete Claude provider experience (auth command, error display, messages)
2. **Priority 2**: Codex/OpenAI provider
3. **Priority 3**: Copilot provider

### Short-term (Expand Provider Coverage)
4. **Priority 4**: Cursor provider
5. **Priority 5**: Gemini provider

### Medium-term (Production Readiness)
6. **Priority 6**: Polish & robustness (UX, error handling, reliability)
7. **Priority 7**: Test suite (in progress - 303 tests passing, 44% coverage)

### Long-term (Documentation & Release)
8. **Priority 8**: Documentation (README, provider guides, config reference)

---

## Milestones

### MVP Milestone (Current State)
**Status**: Functional but incomplete
- ✓ Claude provider works (OAuth, Web, CLI strategies)
- ✓ Basic CLI commands (usage, status, config, key, cache)
- ✓ Display module with Rich and JSON output
- ⚠️ Missing auth command (manual credential management required)
- ⚠️ Basic error handling (no provider-specific messages)

### MVP++ Milestone (Priority 1 Complete)
**Goal**: Complete Claude experience
- Auth command for interactive credential setup
- Provider-specific error messages with remediation
- Rich error display panels
- Stale data warnings
- Complete user experience for Claude provider

### Multi-Provider Milestone (Priorities 1-5 Complete)
**Goal**: Support top 5 AI providers
- Claude (✓), Codex, Copilot, Cursor, Gemini
- Auth flows for all providers
- Consistent error handling across providers
- Unified display formatting

### Production Release Milestone (Priorities 1-7 Complete)
**Goal**: Production-ready tool
- All 5 providers fully implemented
- Comprehensive error handling
- Offline mode and graceful degradation
- Full test coverage (in progress: 303 tests, 44%)
- Robust retry and failure gate mechanisms

### Full Release Milestone (All Priorities Complete)
**Goal**: Complete, documented, maintained tool
- All providers implemented
- 100% test coverage
- Complete documentation
- Examples and troubleshooting guides
- Ready for public release

---

## Spec Inconsistencies (Resolved)

> These items were noted in earlier analysis but are now resolved:

1. ✓ **Exit Codes**: Using Spec 07's definition (0-5 with CONFIG_ERROR, PARTIAL_FAILURE)
2. ✓ **JSON Output Modules**: Consolidated to `display/json.py` only
3. ✓ **Cache Module Location**: Using `config/cache.py` as single module
4. ✓ **FetchPipelineResult vs FetchOutcome**: Standardized on `FetchOutcome`
5. ✓ **Exit Code 4 Naming**: Using `CONFIG_ERROR` (Spec 07)
6. ✓ **Display Module Split**: `cli/display.py` for Rich renderables, `display/rich.py` for utilities
7. ✓ **format_reset_time vs format_reset_countdown**: Using `format_reset_time()`

**Still Outstanding**:
- Browser cookie extraction: `browser_cookie3` or `pycookiecheat` not in dependencies (deferred - not needed for MVP)
- `pace_to_color` location: In models.py instead of display/colors.py (functional but inconsistent with spec)
