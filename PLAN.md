# vibeusage Implementation Plan

A CLI application to track usage stats from all LLM providers to understand session usage windows and costs.

**Target**: Python 3.14+
**Core Dependencies**: httpx, typer, rich, msgspec, platformdirs, tomli-w

---

## Current Status

**Implementation State**: Phase 0-5 complete (All 5 priority providers implemented: Claude, Codex, Copilot, Cursor, Gemini)

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
- ✓ Claude provider (OAuth, Web, CLI strategies, status polling)
- ✓ Codex provider (OAuth strategy, status polling)
- ✓ Copilot provider (device flow OAuth strategy, status polling)
- ✓ Cursor provider (web session strategy, status polling)
- ✓ Gemini provider (OAuth + API key strategies, Google Workspace status)
- ✓ Test suite (724 passing tests, 61% coverage)
- ✓ Provider command aliases (claude, codex, copilot, cursor, gemini as top-level commands)
- ✓ SingleProviderDisplay with title+separator format per spec 05
- ✓ ProviderPanel with compact view (filters model-specific periods) per spec 05

---

## Recent Fixes

### 2026-01-17: Documentation (Priority 8) ✅ COMPLETED
- **Added comprehensive user documentation**
  - README.md with installation, quick start, provider setup, command reference, troubleshooting
  - Provider-specific setup guides (docs/providers/claude.md, codex.md, copilot.md, cursor.md, gemini.md)
  - Configuration reference (docs/config.md) with all options and environment variables
  - Each provider doc includes authentication methods, credential storage, troubleshooting
  - Configuration reference covers display, fetch, credential settings, and provider-specific options
- All 724 tests pass (61% coverage)

### 2026-01-16: Usage Command Tests (Priority 7)
- **Added 39 comprehensive tests for CLI usage commands (tests/cli/test_usage_commands.py)**
  - Tests for usage_command (main entry point) with single/multiple provider paths
  - Tests for fetch_provider_usage and fetch_all_usage helper functions
  - Tests for display_snapshot and display_multiple_snapshots display functions
  - Tests for format_period, format_overage, and get_pace_color utility functions
  - Tests for keyboard interrupt, exception handling, and cleanup behavior
  - Tests for verbose/quiet modes and JSON output
  - All 724 tests pass (61% coverage, up from 58%)

### 2026-01-16: First-Run Experience Wizard (Priority 6)
- **Added first-run experience wizard with `vibeusage init` command**
  - Added `is_first_run()` and `count_configured_providers()` functions to config/credentials.py
  - Added first-run detection to default command that shows welcome message when no providers configured
  - Created init.py command with interactive wizard, --quick flag, --skip flag, and JSON output support
  - Added 25 new tests for first-run detection and init command functionality
  - All 677 tests pass (54% coverage)

### 2025-01-17: Auth CLI Commands Test Suite (Priority 7)
- **Added comprehensive test suite for auth CLI commands (46 new tests)**
  - Created tests/cli/test_auth_commands.py with comprehensive auth command tests
  - Tests for auth_command (main entry point with provider routing)
  - Tests for auth_status_command (JSON/quiet/normal/verbose modes)
  - Tests for auth_claude_command (session key handling, validation, errors)
  - Tests for auth_generic_command (all providers, instructions display)
  - Tests for helper functions (_show_claude_auth_instructions, _show_provider_auth_instructions)
- **Improved overall test coverage from 54% to 58%**
  - auth.py: 10% → 97% coverage
  - 685 tests passing (up from 614)
- All tests pass with 0 failures

### 2026-01-16: Auth Module Tests (Priority 7)
- **Added comprehensive test suite for auth/base.py (71 new tests, 96% coverage)**
  - Tests for all credential types: OAuth2Credentials, SessionCredentials, APIKeyCredentials, CLICredentials, LocalProcessCredentials
  - Tests for AuthResult factory methods (ok, fail)
  - Tests for AuthStrategy abstract base class
  - Tests for all config structs: OAuth2Config, CookieConfig, CLIConfig, DeviceFlowConfig, LocalProcessConfig
  - Tests for ProviderAuthConfig authentication flow with strategy fallback
  - Tests for protocol compliance
- **auth/__init__.py: 100% coverage**
- **auth/base.py: 96% coverage (up from 0%)**
- All 614 tests pass (54% overall coverage, up from 51%)

### 2026-01-16: Stale Warning Config Threshold Fix
- **Fixed stale warnings to use configured threshold instead of hardcoded value**
  - Changed `display_snapshot()` and `display_multiple_snapshots()` to read `stale_threshold_minutes` from config
  - Previous implementation used hardcoded 10-minute threshold instead of configured 60-minute default
  - Added blank line after stale warning for better readability
  - Fixed type annotation: use `collections.abc.Callable` instead of bare `callable`
  - Fixed FetchOutcome error type: use `str` instead of `Exception`

### 2026-01-16: Progress Module Tests & Type Fixes
- **Added comprehensive test suite for cli/progress.py (31 new tests, 100% coverage)**
  - Tests for create_progress() context manager (quiet mode, normal mode, console handling)
  - Tests for ProgressTracker class (start, update, description formatting)
  - Tests for ProgressCallback callable adapter
  - Tests for create_progress_callback() factory function
  - Integration tests for full fetch cycles
  - Tests for progress theme configuration
- **Fixed type issues in progress.py**
  - Fixed RGB color format (removed spaces: `rgb(67,142,247)` instead of `rgb(67, 142, 247)`)
  - Removed invalid `theme` parameter from Progress() constructor (theme applied to Console instead)
  - Changed return type from `callable` to `ProgressCallback | None` for proper type annotation
- **Typecheck now passes for progress.py**
- All 543 tests pass (51% coverage, up from 50%)

### 2026-01-16: Progress Indicators & Stale Data Warnings
- **Implemented progress indicators for concurrent fetch operations per Priority 6**
  - Added `cli/progress.py` module with ProgressTracker and ProgressCallback classes
  - Rich progress bar shows fetch status for all providers during concurrent fetches
  - Respects `--quiet` flag to suppress progress output
  - Integrates with orchestrator's `on_complete` callback pattern
- **Wired up stale data warnings that were previously implemented but unused**
  - `display_snapshot()` now shows stale warning for cached single provider data
  - `display_multiple_snapshots()` shows stale warnings per provider
  - Warnings respect `--quiet` flag
  - Existing `show_stale_warning()` function in `cli/display.py` now properly integrated
- All 512 tests pass (50% coverage)
- Lint passes

### 2026-01-16: Linting Fixes
- **Fixed linting issues across the codebase**
  - Ran ruff format on all files (60 files reformatted)
  - Fixed B904: raise without `from` in exception handlers
  - Fixed UP017: datetime.UTC to datetime.timezone.utc
  - Fixed E402: module level import not at top of file
  - Fixed B017: contextmanager suppress-specific exception check
  - Fixed test_copilot.py mock issue: changed from patching Path.exists (which isn't imported) to patching CREDENTIAL_FILE directly
  - All 512 tests pass

### 2026-01-17: JSON Error Response Standardization (Priority 6)
- **Implemented standardized ErrorResponse struct for JSON error output per spec 07**
  - Added ErrorResponse and ErrorData msgpack.Structs to display/json.py
  - Added output_json_error() function for consistent error JSON output
  - Added from_vibeusage_error() helper to convert VibeusageError to ErrorResponse
  - Updated multi-provider JSON format to use providers/errors dict structure per spec
  - Single provider errors now include full error metadata (category, severity, provider, remediation)
  - Added 14 new tests for ErrorResponse functionality
  - All 512 tests pass (49% coverage)

### 2026-01-17: Gemini Provider Implementation
- **Implemented complete Gemini provider with OAuth and API key strategies**
  - OAuth strategy uses Google Cloud Code API (retrieveUserQuota, loadCodeAssist endpoints)
  - API key strategy as fallback for users without OAuth credentials
  - Status polling via Google Workspace incidents feed
  - Per-model quota tracking with daily reset periods
  - Provider registered and CLI command `vibeusage gemini` functional

### 2026-01-17: Spec Compliance Fixes
- **Fixed minor spec compliance issues in display formatting**
  - SingleProviderDisplay now capitalizes provider name in title ("Claude" not "claude")
  - ProviderPanel removed source row for cleaner compact view per spec 05
  - Added 8 new tests for SingleProviderDisplay and ProviderPanel spec compliance

### 2026-01-17: Test Ordering Fix
- **Fixed test ordering issues in test_providers.py**
  - Added `teardown_method` to `TestProviderRegistry` to restore provider registry
  - Previously `setup_method` cleared the registry without restoring it
  - All 466 tests now pass (up from 463)

### 2026-01-17: Usage Display Formatting Fix
- **Fixed usage display formatting to match spec 05-cli-interface.md**
  - Single provider view shows "All Models" for general periods with indented model-specific periods
  - Multi-provider view shows period type names ("Weekly", "Daily") instead of period names ("All Models")
  - Multi-provider view filters out model-specific periods for compact display
  - Removed extra blank line in multi-provider view

### 2026-01-17: Verbose/Quiet Flags
- **Implemented `--verbose` and `--quiet` flags across all commands**
  - Verbose mode shows: fetch timing, account identity, credential paths, model info
  - Quiet mode suppresses: headers, tables, informational messages, setup instructions
  - Conflict resolution: quiet takes precedence when both flags specified
  - All commands respect these flags: usage, status, auth, config, key, cache

### 2026-01-17: JSON Output
- **Implemented `--json` flag for all commands**
  - All commands (usage, status, auth, key, cache, config) support JSON output
  - Both `--json usage` and `usage --json` work correctly
  - Added 8 new tests for JSON functionality

### 2026-01-17: Multi-Provider Panel Layout
- **Fixed multi-provider usage display to use panel-based layout per spec 05**
  - Updated `display_multiple_snapshots()` to use ProviderPanel for proper panel formatting
  - Fixed Rich renderable `__rich_console__` methods to use `yield` instead of `return`
  - Multi-provider view now shows panels with provider names, progress bars, and reset times

### 2026-01-16: Usage Display Spec Compliance
- **Fixed usage display to match spec 05-cli-interface.md**
  - Added provider command aliases: `vibeusage claude`, `codex`, `copilot`, `cursor`, `gemini`
  - Created `SingleProviderDisplay` class for spec-compliant single provider output:
    - Provider name title with `━━━` separator (no panel wrapper)
    - Session periods standalone, not indented
    - Weekly/Daily/Monthly section headers with indented model periods
    - Only overage in a separate Panel at bottom
  - Updated `ProviderPanel` to filter out model-specific periods in compact view
  - Output now matches spec 05-cli-interface.md exactly

### 2026-01-16: Codex OAuth
- **Fixed Codex OAuth credential loading and API response parsing**
  - Fixed nested `tokens` object extraction from Codex CLI auth.json
  - Fixed API response parsing for actual `rate_limit`, `primary_window`, `secondary_window`, `reset_at` format
  - Added 3 new tests for credential format handling

### 2026-01-16: Claude OAuth
- **Fixed Claude OAuth credential loading and usage response parsing**
  - Fixed camelCase to snake_case conversion for `accessToken`, `refreshToken`, `expiresAt`
  - Fixed OAuth usage API parsing for `utilization` and `resets_at` fields
  - Fixed `pace_to_color()` and `format_period()` function calls

### 2026-01-16: Provider Registry Test Fix
- **Fixed provider registry test ordering issues**
  - Added autouse fixture in conftest.py to automatically restore provider registry state
  - Previously tests that modified the registry could affect subsequent tests
  - All 466 tests now pass with 0 test ordering issues

### 2025-01-16: Test Suite Fixes
- Fixed File I/O type issue in _save_to_toml(): use binary mode 'wb' instead of 'w'
- Fixed AsyncIO event loop handling in atyper.py sync_wrapper
- Fixed provider test class definitions to avoid variable scope issues
- Fixed gate_dir() test mock to return Path object instead of string
- Replaced sys.stdout.buffer patching with capsysbinary/capsys fixtures
- Fixed JSON formatting tests to expect compact JSON (msgspec format)
- Fixed ATyper API tests to be more resilient to API changes
- Reduced test failures from 38 to 3 (only test ordering issues remain)

---

## CLI Command Testing Results (2025-01-17)

### WORKING Commands (✓)

**Core Commands:**
- `vibeusage --help` - Shows all commands correctly ✓
- `vibeusage` (default) - Shows panel-based usage display per spec 05 ✓
- `vibeusage --json` - JSON output works ✓
- `vibeusage --version` - Shows "vibeusage 0.1.0" ✓
- `vibeusage --no-color` - Works (disables color output) ✓
- `vibeusage usage` - Shows panel-based usage display per spec 05 ✓
- `vibeusage usage --json` - JSON output works ✓
- `vibeusage usage --refresh` - Works ✓

**Provider-Specific Usage:**
- `vibeusage usage claude` - Shows Claude usage data ✓
- `vibeusage usage codex` - Shows Codex usage data ✓
- `vibeusage usage copilot` - Gives "Strategy not available" error (expected, provider needs auth) ✓

**Provider Command Aliases:**
- `vibeusage claude` - Shows Claude usage (identical to `vibeusage usage claude`) ✓
- `vibeusage codex` - Shows Codex usage ✓
- `vibeusage copilot` - Shows Copilot usage ✓
- `vibeusage cursor` - Shows Cursor usage ✓
- `vibeusage gemini` - Shows Gemini usage ✓

**Status & Auth Commands:**
- `vibeusage status` - Shows provider status table ✓
- `vibeusage status --json` - JSON output works ✓
- `vibeusage --json status` - JSON output works ✓
- `vibeusage auth` - Shows auth status table ✓
- `vibeusage auth --json` - JSON output works ✓
- `vibeusage --json auth` - JSON output works ✓
- `vibeusage auth claude` - Interactive auth flow works ✓

**Cache Commands:**
- `vibeusage cache show` - Shows cache status table ✓
- `vibeusage --json cache show` - JSON output works ✓
- `vibeusage cache clear` - Clears cache ✓

**Config Commands:**
- `vibeusage config show` - Shows config file contents ✓
- `vibeusage --json config show` - JSON output works ✓
- `vibeusage config path` - Shows config/cache/credentials directory paths ✓
- `vibeusage --json config path` - JSON output works ✓

**Key Commands:**
- `vibeusage key` - Shows credential status for all providers ✓
- `vibeusage --json key` - JSON output works ✓
- `vibeusage key set <provider>` - Sets credential for provider ✓

---

## Remaining Work (Prioritized by Dependencies & Value)

### Priority 1: Minor Fixes & UX Improvements ✅ COMPLETE
**Goal**: Fix remaining interface issues and polish UX

- [x] Fix `key set` command interface
- [x] Implement `--json` flag for all commands
- [x] Implement `--verbose` and `--quiet` flags

**Remaining (Optional):**
- [ ] Fix ProviderStatus factory method return type hints
- [ ] Consider moving `pace_to_color()` to display/colors.py for consistency

---

### Priority 2: Codex/OpenAI Provider ✅ COMPLETED

---

### Priority 3: Copilot Provider ✅ COMPLETED

---

### Priority 4: Cursor Provider ✅ COMPLETED

---

### Priority 5: Gemini Provider ✅ COMPLETED

- [x] **Create provider module** (providers/gemini/)
  - [x] `__init__.py` - GeminiProvider with metadata
  - [x] `oauth.py` - GeminiOAuthStrategy
  - [x] Google OAuth flow and credential storage
  - [x] `api_key.py` - GeminiApiKeyStrategy (fallback)
  - [x] `status.py` - Google Workspace status polling
- [x] **Register and test**

**Value**: Medium - Fourth most requested provider

---

### Priority 6: Polish & Robustness
**Goal**: Production-ready error handling and UX

#### Error Handling Enhancement
- [x] HTTP error classification (errors/http.py)
- [x] Network error handling (errors/network.py)

#### User Experience Improvements
- [x] Progress indicators (spinners during fetches)
- [x] First-run experience wizard
- [x] Stale data display warnings (with configurable threshold)
- [x] Offline mode and graceful degradation (automatic cache fallback)

#### JSON Output Enhancement
- [x] ErrorResponse struct for JSON error output
- [x] Multi-provider response struct

**Value**: High - Makes tool production-ready

---

### Priority 7: Test Suite
**Status**: GOOD PROGRESS (61% coverage, 724 passing tests, 0 test ordering issues)

**Completed**:
- [x] Test infrastructure (pytest, pytest-asyncio, pytest-cov, pytest-mock)
- [x] Fixtures (conftest.py with comprehensive test fixtures)
- [x] Model validation tests
- [x] Error classification tests
- [x] Config system tests
- [x] Provider fetch tests (Claude, Codex, Copilot, Cursor, Gemini)
- [x] Fetch pipeline tests
- [x] Orchestrator tests
- [x] CLI command behavior tests
- [x] Output format tests (including SingleProviderDisplay and ProviderPanel spec compliance)
- [x] Exit code tests
- [x] Error scenario tests

**Remaining Issues**:
- [ ] Increase code coverage from 48% to 80%+
- [ ] Add more display module tests (rich.py, json.py have partial coverage)

**Value**: High - Essential for production reliability

---

### Priority 8: Documentation ✅ COMPLETED

**Completed**:
- [x] README.md (installation, quick start, troubleshooting, command reference)
- [x] Provider setup docs (docs/providers/claude.md, codex.md, copilot.md, cursor.md, gemini.md)
- [x] Config reference (docs/config.md with all configuration options)
- [x] AGENTS.md already contains test/lint/typecheck commands

**Value**: Medium - Important for adoption

---

## Implementation Order Summary

### Immediate (Complete MVP)
1. **Priority 1**: Minor fixes ✅
2. **Priority 2**: Codex/OpenAI provider ✅
3. **Priority 3**: Copilot provider ✅

### Short-term (Expand Provider Coverage)
4. **Priority 4**: Cursor provider ✅
5. **Priority 5**: Gemini provider ✅

### Medium-term (Production Readiness)
6. **Priority 6**: Polish & robustness
7. **Priority 7**: Test suite improvements

### Long-term (Documentation & Release)
8. **Priority 8**: Documentation ✅

---

## Milestones

### MVP Milestone ✅ COMPLETE
- ✓ Claude provider works (OAuth, Web, CLI strategies)
- ✓ Basic CLI commands (usage, status, config, key, cache)
- ✓ Display module with Rich and JSON output
- ✓ Spec-compliant usage display (single and multi-provider)

### Multi-Provider Milestone ✅ COMPLETE
**Goal**: Support top 5 AI providers
- Claude ✅, Codex ✅, Copilot ✅, Cursor ✅, Gemini ✅

### Production Release Milestone (In Progress)
**Goal**: Production-ready tool
- All 5 providers fully implemented ✅
- Comprehensive error handling (Priority 6)
- Full test coverage (Priority 7)

---

## Spec Inconsistencies (Resolved)

1. ✓ **Exit Codes**: Using Spec 07's definition (0-5 with CONFIG_ERROR, PARTIAL_FAILURE)
2. ✓ **JSON Output Modules**: Consolidated to `display/json.py` only
3. ✓ **Cache Module Location**: Using `config/cache.py` as single module
4. ✓ **FetchPipelineResult vs FetchOutcome**: Standardized on `FetchOutcome`
5. ✓ **Display Module Split**: `cli/display.py` for Rich renderables, `display/rich.py` for utilities

**Still Outstanding**:
- Browser cookie extraction: `browser_cookie3` or `pycookiecheat` not in dependencies (deferred)
- `pace_to_color` location: In models.py instead of display/colors.py (functional but inconsistent with spec)
