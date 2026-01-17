# vibeusage Implementation Plan

A CLI application to track usage stats from all LLM providers to understand session usage windows and costs.

**Target**: Python 3.14+
**Core Dependencies**: httpx, typer, rich, msgspec, platformdirs, tomli-w

---

## Current Status

**Implementation State**: Phase 0-5 complete (All 5 priority providers implemented: Claude, Codex, Copilot, Cursor, Gemini)

**Known Gaps** (discovered 2026-01-17) - **ALL ARE NEXT TASKS**:
- 🚨 **Copilot device flow**: Fully specified in spec 03 but never implemented - users cannot authenticate
- 🚨 **Claude browser cookies**: Strategy implemented but not registered - manual paste only
- 🚨 **Cursor browser cookies**: Strategy implemented but not registered - manual paste only

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
- ✓ Test suite (1062 passing tests, 81% coverage - **exceeds 80% target**)
- ✓ Provider command aliases (claude, codex, copilot, cursor, gemini as top-level commands)
- ✓ SingleProviderDisplay with title+separator format per spec 05
- ✓ ProviderPanel with compact view (filters model-specific periods) per spec 05

---

## Recent Fixes

### 2026-01-17: Browser Cookie Dependency Added ✅ RESOLVED
- **Issue**: Browser cookie extraction strategies (`ClaudeBrowserCookieStrategy`, `CursorBrowserCookieStrategy`) were implemented but couldn't function because `browser_cookie3` was not in dependencies
- **Root Cause**: The code attempted to import `browser_cookie3` or `pycookiecheat`, but neither was listed in pyproject.toml
- **Resolution**:
  - Added `browser-cookie3>=0.19.0` to dependencies in pyproject.toml
  - Added 5 tests for `ClaudeBrowserCookieStrategy` in test_providers.py
  - Updated tests for both strategies to verify dependency availability
- **Verification**: All 1062 tests pass (up from 1060), browser cookie strategies can now import required library
- **Note**: Browser strategies are still not registered in `fetch_strategies()` - see Phase 9.2 for remaining activation work

### 2026-01-17: Key Command Syntax - Provider First, Then Action ✅ RESOLVED
- **Issue**: `vibeusage key copilot set` command failed with "No such command 'copilot'"
- **Root Cause**: The `key` command was implemented as `key set <provider>` but spec 06 defines `key <provider> set` (factory pattern)
- **Resolution**:
  - Refactored `key.py` to use factory pattern with provider-specific subcommands
  - Added `--json` and `--quiet` options to provider callbacks for proper context propagation
  - Fixed auth command instructions for all providers (codex, copilot, cursor, gemini)
  - Updated Copilot auth to clarify that `gh auth login` is for GitHub CLI, not Copilot
  - Updated tests to use CLI runner testing instead of direct function calls
- **Verification**: `vibeusage key copilot set` now works correctly, all 1057 tests pass (81% coverage)

### 2026-01-17: ProviderStatus Type Hint Fix ✅ RESOLVED
- **Issue**: `ProviderStatus.operational()` and `ProviderStatus.unknown()` factory methods had incorrect return type hints
- **Root Cause**: Type hints declared as `type[ProviderStatus]` instead of `ProviderStatus`
- **Resolution**: Changed return type annotations from `type[ProviderStatus]` to `ProviderStatus`
- **Verification**: All 1057 tests pass, type checking passes with `ty check`

### 2026-01-17: Missing CLI Subcommands Issue ✅ RESOLVED
- **Issue**: Core CLI subcommands (auth, init, status, usage, key, cache, config) were missing from --help output
- **Root Cause**: Stale cached build - the virtual environment had an outdated binary
- **Resolution**: Ran `uv sync --reinstall` to rebuild the package
- **Verification**: All 12 commands now showing correctly, all 1057 tests pass (81% coverage)

### 2026-01-17: CLI Subcommands Investigation

**Issue Reported**: CLI subcommands (auth, init, status, usage, cache, config, key) were missing from `vibeusage --help`. Only provider commands (claude, codex, copilot, cursor, gemini) were showing.

**Investigation**: When investigated, all 12 commands were actually present and registered correctly. The issue was likely caused by:
- Stale shell command hash cache (fix: `hash -r` in bash/zsh)
- Not running `uv sync` after code changes
- Using an old installed version instead of the development environment

**Fix Applied**:
- Removed duplicate `app.add_typer()` calls from key.py, config.py, cache.py (now registered only once in cli/app.py)

**Verification**:
- All commands appear in `vibeusage --help`: auth, init, status, usage, claude, codex, copilot, cursor, gemini, key, cache, config
- All subcommands work correctly (key set/delete, cache show/clear, config show/path/reset/edit)
- All 1057 tests pass (81% coverage)

---

## Implementation History (Summary)

**January 2025**: Implemented all 5 priority providers (Claude, Codex, Copilot, Cursor, Gemini), CLI framework with ATyper, display modules (Rich/JSON), error classification system, and configuration management. Fixed OAuth credential loading and API response parsing for all providers.

**Early 2026**: Added comprehensive polish features including progress indicators, stale data warnings, first-run wizard, standardized JSON error responses, verbose/quiet modes. Implemented full test suite reaching 81% coverage (1055 passing tests). Added complete documentation (README, provider guides, config reference). All CLI commands now functional with proper error handling and spec-compliant output.

---

## Remaining Work (Prioritized by Dependencies & Value)

### 🚨 NEXT TASKS: Authentication Flows (Phase 9.1 + 9.2)
**Status**: HIGHEST PRIORITY - Multiple providers have broken or degraded authentication

#### Task A: Copilot Device Flow (Critical)
**Impact**: New Copilot users cannot authenticate at all

The device flow is fully specified in `specs/03-authentication.md` lines 557-697 but was never implemented. The `auth copilot` command only shows instructions instead of running the interactive OAuth device flow.

**Actions**:
1. Implement `_device_flow()` in `providers/copilot/device_flow.py`
2. Implement `_poll_for_token()` with proper error handling
3. Update `auth copilot` command to invoke the device flow
4. Add comprehensive tests

**See**: Priority 9.1 below for full task breakdown

#### Task B: Claude Browser Cookie Activation (Medium)
**Impact**: Users must manually extract cookies from DevTools instead of automatic extraction

`ClaudeBrowserCookieStrategy` is fully implemented at `providers/claude/web.py:236-304` but NOT registered in `fetch_strategies()`. The `browser_cookie3` dependency is now installed.

**Actions**:
1. Register `ClaudeBrowserCookieStrategy` in `ClaudeProvider.fetch_strategies()`
2. Update `auth claude` to try automatic extraction before manual paste
3. Add tests for the integrated flow

#### Task C: Cursor Browser Cookie Activation (Medium)
**Impact**: Users must manually extract cookies from DevTools instead of automatic extraction

`CursorBrowserCookieStrategy` is fully implemented at `providers/cursor/web.py:214-295` but NOT registered in `fetch_strategies()`. The `browser_cookie3` dependency is now installed.

**Actions**:
1. Register `CursorBrowserCookieStrategy` in `CursorProvider.fetch_strategies()`
2. Update `auth cursor` to try automatic extraction before manual paste
3. Add tests for the integrated flow

**See**: Priority 9.2 below for full task breakdown

---

### Priority 1: Minor Fixes & UX Improvements ✅ COMPLETE
**Goal**: Fix remaining interface issues and polish UX

- [x] Fix `key set` command interface
- [x] Implement `--json` flag for all commands
- [x] Implement `--verbose` and `--quiet` flags

**Remaining (Optional):**
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

### Priority 7: Test Suite ✅ COMPLETED
**Status**: ALL TARGETS MET (81% coverage, 1055 passing tests, 0 failures)

**Completed**:
- [x] Test infrastructure (pytest, pytest-asyncio, pytest-cov, pytest-mock)
- [x] Fixtures (conftest.py with comprehensive test fixtures)
- [x] Model validation tests
- [x] Error classification tests
- [x] Config system tests
- [x] Cache module tests (42 tests, 97% coverage)
- [x] Gate module tests (27 tests, 95% coverage)
- [x] HTTP client tests (19 tests, 94% coverage)
- [x] Error messages tests (44 tests, 100% coverage)
- [x] CLI app tests (22 tests, 80% coverage)
- [x] Claude OAuth strategy tests (35 tests, 91% coverage)
- [x] Provider fetch tests (Claude, Codex, Copilot, Cursor, Gemini)
- [x] Fetch pipeline tests
- [x] Orchestrator tests
- [x] CLI command behavior tests (usage, auth, status, config, key, cache, init)
- [x] Key command tests (19 tests, 98% coverage)
- [x] Status command tests (39 tests, 86% coverage)
- [x] Output format tests (including SingleProviderDisplay and ProviderPanel spec compliance)
- [x] Exit code tests
- [x] Error scenario tests
- [x] JSON output tests (using capsys for proper stdout capture)
- [x] format_status_updated tests (simplified to pattern matching)
- [x] Status module tests (copilot/status.py, gemini/status.py at 100% coverage)
- [x] **80% coverage target exceeded** (currently at 82%)
- [x] **All 1057 tests passing** (0 failures)

**Remaining (Optional Improvements)**:
- [ ] Add more provider strategy tests for web.py files (currently 17% coverage)
- [ ] Add more CLI command tests for init.py (currently 50% coverage)

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

### Priority 9: Interactive Authentication ⚠ NEW
**Goal**: Implement spec-compliant interactive authentication flows

**Problem Statement** (discovered 2026-01-17):
All `auth <provider>` commands currently only display instructions - they don't actually perform authentication. Users must use external CLIs or manually paste credentials. This violates the specs which define complete interactive flows.

#### Gap Analysis by Provider

| Provider | Spec Says | Implementation Has | Gap | Status |
|----------|-----------|-------------------|-----|--------|
| **Copilot** | Full device flow (spec 03:557-697) | Only credential loading | **Critical** - flow never implemented | 🚨 NEXT |
| **Claude** | Browser cookie extraction | Code exists, not registered | **Major** - dead code | 🚨 NEXT |
| **Cursor** | Browser cookie extraction | Code exists, not registered | **Major** - dead code | 🚨 NEXT |
| **Codex** | Web strategy "(future)" | Not implemented | Intentional | — |
| **Gemini** | OAuth via CLI credentials | Only loads existing creds | Minor | — |

#### Phase 9.1: Copilot Device Flow 🚨 NEXT
**Spec Reference**: `specs/03-authentication.md` lines 557-697
**Status**: NEXT TASK - Critical, users cannot authenticate at all

The spec fully defines `GitHubDeviceFlowStrategy` with:
- Request device code from `POST https://github.com/login/device/code`
- Display user code and verification URL
- Open browser automatically
- Poll for token at `POST https://github.com/login/oauth/access_token`
- Handle authorization_pending, slow_down, expired_token, access_denied
- Save credentials to `~/.config/vibeusage/credentials/copilot/oauth.json`

**Tasks**:
- [ ] Implement `_device_flow()` method in `providers/copilot/device_flow.py`
  - [ ] Request device code with client_id and scope
  - [ ] Display user code with Rich formatting
  - [ ] Attempt to open browser via `webbrowser.open()`
  - [ ] Show fallback URL if browser fails
- [ ] Implement `_poll_for_token()` method
  - [ ] Poll with configurable interval (default 5s)
  - [ ] Handle `authorization_pending` (continue polling)
  - [ ] Handle `slow_down` (increase interval by 5s)
  - [ ] Handle `expired_token` (fail with retry message)
  - [ ] Handle `access_denied` (fail with clear message)
  - [ ] Max 60 attempts (5 minute timeout)
- [ ] Update `auth copilot` command to invoke device flow
  - [ ] Check for existing valid credentials first
  - [ ] Offer to re-authenticate if credentials exist
  - [ ] Run device flow interactively
  - [ ] Show success message with credential location
- [ ] Add tests for device flow
  - [ ] Mock HTTP responses for device code request
  - [ ] Mock polling responses (pending, success, error cases)
  - [ ] Test timeout behavior
  - [ ] Test credential storage

**Value**: High - Copilot is a major provider, currently completely broken for new users

---

#### Phase 9.2: Activate Browser Cookie Extraction 🚨 NEXT
**Spec Reference**: `specs/03-authentication.md` lines 280-389
**Status**: NEXT TASK - Code exists, dependency installed, just needs registration

Browser cookie strategies are fully implemented but not registered in `fetch_strategies()`:
- `ClaudeBrowserCookieStrategy` at `providers/claude/web.py:236-304`
- `CursorBrowserCookieStrategy` at `providers/cursor/web.py:214-295`

**Tasks**:
- [x] Add `browser_cookie3` to project dependencies (pyproject.toml)
  - [x] `browser-cookie3>=0.19.0` added to dependencies
  - [ ] Consider `pycookiecheat` as fallback
  - [ ] Make optional dependency group: `pip install vibeusage[browser]`
- [ ] Register `ClaudeBrowserCookieStrategy` in Claude provider
  - [ ] Add to `fetch_strategies()` before `ClaudeWebStrategy`
  - [ ] Update comment to reflect actual strategy order
- [ ] Register `CursorBrowserCookieStrategy` in Cursor provider
  - [ ] Add to `fetch_strategies()` before `CursorWebStrategy`
  - [ ] Update comment to reflect actual strategy order
- [ ] Update `auth claude` command
  - [ ] Try automatic browser extraction first
  - [ ] Fall back to manual paste if extraction fails
  - [ ] Show which browser cookie was found
- [ ] Update `auth cursor` command
  - [ ] Try automatic browser extraction first
  - [ ] Improve instructions with specific cookie names
  - [ ] Show step-by-step DevTools navigation
- [ ] Add tests for browser cookie strategies
  - [ ] Mock browser_cookie3 responses
  - [ ] Test fallback to manual entry
  - [ ] Test cookie storage after extraction

**Value**: High - Quick win, code already exists, just needs activation

---

#### Phase 9.3: Improve Auth Command Instructions
**Goal**: Make instruction-only auth commands more helpful

Even without interactive flows, the current instructions are sparse.

**Tasks**:
- [ ] Claude auth instructions
  - [x] Already has detailed DevTools navigation
  - [ ] Add expected cookie format/prefix validation
- [ ] Codex auth instructions
  - [ ] Add `~/.codex/auth.json` file format example
  - [ ] Explain what the OAuth token looks like
- [ ] Cursor auth instructions
  - [ ] Add which cookie names to look for
  - [ ] Add step-by-step DevTools navigation (like Claude)
  - [ ] Show expected cookie format
- [ ] Gemini auth instructions
  - [ ] Add `~/.gemini/oauth_creds.json` file format example
  - [ ] Explain credential structure
- [ ] Copilot auth instructions (after device flow)
  - [ ] Show VS Code hosts.json location as alternative
  - [ ] Explain token format

**Value**: Low - Nice to have, doesn't add functionality

---

#### Phase 9.4: Optional Keyring Integration
**Spec Reference**: `specs/03-authentication.md` lines 890-910

**Tasks**:
- [ ] Add `keyring` to optional dependencies
- [ ] Implement `store_in_keyring()` and `get_from_keyring()` helpers
- [ ] Add config option to enable keyring storage
- [ ] Update credential loading to check keyring first when enabled
- [ ] Add macOS Keychain support for Claude (`"Claude Code-credentials"` service)

**Value**: Low - Security enhancement, optional

---

#### Implementation Order

**🚨 IMMEDIATE (All Next Tasks)**:
1. **Phase 9.1** (Copilot Device Flow) - Critical, users cannot authenticate
2. **Phase 9.2** (Browser Cookies) - Medium, activate existing code for Claude + Cursor

**Later**:
3. **Phase 9.3** (Better Instructions) - Low priority, polish
4. **Phase 9.4** (Keyring) - Low priority, optional security

**Dependencies**:
- Phase 9.1 requires no external dependencies
- Phase 9.2: `browser_cookie3` already added ✅
- Phase 9.4 requires `keyring` package

---

## Implementation Order Summary

### 🚨 IMMEDIATE (Next Tasks)
- **Phase 9.1**: Copilot Device Flow - Critical, users cannot authenticate
- **Phase 9.2**: Browser Cookie Activation - Claude + Cursor, code exists, just needs registration

### Completed (MVP)
1. **Priority 1**: Minor fixes ✅
2. **Priority 2**: Codex/OpenAI provider ✅
3. **Priority 3**: Copilot provider ✅
4. **Priority 4**: Cursor provider ✅
5. **Priority 5**: Gemini provider ✅
6. **Priority 6**: Polish & robustness ✅
7. **Priority 7**: Test suite ✅
8. **Priority 8**: Documentation ✅

### Upcoming (Authentication Gaps)
9. **Priority 9**: Interactive Authentication
   - Phase 9.1: Copilot device flow 🚨 NEXT
   - Phase 9.2: Claude + Cursor browser cookies 🚨 NEXT
   - Phase 9.3: Better auth instructions (Later)
   - Phase 9.4: Keyring integration (Later)

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
- Interactive authentication flows (Priority 9) ⚠ BLOCKED

---

## Spec Inconsistencies (Resolved)

1. ✓ **Exit Codes**: Using Spec 07's definition (0-5 with CONFIG_ERROR, PARTIAL_FAILURE)
2. ✓ **JSON Output Modules**: Consolidated to `display/json.py` only
3. ✓ **Cache Module Location**: Using `config/cache.py` as single module
4. ✓ **FetchPipelineResult vs FetchOutcome**: Standardized on `FetchOutcome`
5. ✓ **Display Module Split**: `cli/display.py` for Rich renderables, `display/rich.py` for utilities

**Still Outstanding**:
- Browser cookie extraction: Dependency added (`browser_cookie3>=0.19.0`) but strategies not yet registered in `fetch_strategies()` → See Priority 9.2
- `pace_to_color` location: In models.py instead of display/colors.py (functional but inconsistent with spec)
- Interactive authentication: All auth commands only show instructions → See Priority 9
- Copilot device flow: Never implemented despite being fully specified → See Priority 9.1
