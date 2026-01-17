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
- ✓ Test suite (1055 passing tests, 82% coverage - **exceeds 80% target**)
- ✓ Provider command aliases (claude, codex, copilot, cursor, gemini as top-level commands)
- ✓ SingleProviderDisplay with title+separator format per spec 05
- ✓ ProviderPanel with compact view (filters model-specific periods) per spec 05

---

## Recent Fixes

### 2026-01-17: Incorrect Key Command Syntax in Auth Instructions ✅ RESOLVED
- **Issue**: Auth instructions suggested `vibeusage key copilot set --type oauth` but the correct syntax is `vibeusage key set copilot --type oauth`
- **Root Cause**: The `key` command has `set` as a subcommand that takes `provider` as an argument, not as a subcommand
- **Resolution**: Fixed auth command instructions in `src/vibeusage/cli/commands/auth.py` and updated documentation in `README.md` and `docs/providers/cursor.md`
- **Verification**: Auth instructions now show correct syntax, all 1055 tests pass

### 2026-01-17: ProviderStatus Type Hint Fix ✅ RESOLVED
- **Issue**: `ProviderStatus.operational()` and `ProviderStatus.unknown()` factory methods had incorrect return type hints
- **Root Cause**: Type hints declared as `type[ProviderStatus]` instead of `ProviderStatus`
- **Resolution**: Changed return type annotations from `type[ProviderStatus]` to `ProviderStatus`
- **Verification**: All 1055 tests pass, type checking passes with `ty check`

### 2026-01-17: Missing CLI Subcommands Issue ✅ RESOLVED
- **Issue**: Core CLI subcommands (auth, init, status, usage, key, cache, config) were missing from --help output
- **Root Cause**: Stale cached build - the virtual environment had an outdated binary
- **Resolution**: Ran `uv sync --reinstall` to rebuild the package
- **Verification**: All 12 commands now showing correctly, all 1055 tests pass (82% coverage)

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
- All 1055 tests pass (82% coverage)

---

## Implementation History (Summary)

**January 2025**: Implemented all 5 priority providers (Claude, Codex, Copilot, Cursor, Gemini), CLI framework with ATyper, display modules (Rich/JSON), error classification system, and configuration management. Fixed OAuth credential loading and API response parsing for all providers.

**Early 2026**: Added comprehensive polish features including progress indicators, stale data warnings, first-run wizard, standardized JSON error responses, verbose/quiet modes. Implemented full test suite reaching 82% coverage (1055 passing tests). Added complete documentation (README, provider guides, config reference). All CLI commands now functional with proper error handling and spec-compliant output.

---

## Remaining Work (Prioritized by Dependencies & Value)

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
**Status**: ALL TARGETS MET (82% coverage, 1055 passing tests, 0 failures)

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
- [x] **All 1055 tests passing** (0 failures)

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
