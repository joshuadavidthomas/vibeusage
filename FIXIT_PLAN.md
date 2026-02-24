# Fix It Plan

Structural inconsistencies across the codebase — things that work but don't cohere.

## 1. Wrong project layout

`main.go` is in the root. `cmd/` is a Go package (`package cmd`) containing all cobra command definitions, output helpers, logger config, and CLI wiring — hundreds of lines of application logic. This is backwards from the standard Go project layout:

- `cmd/<appname>/` should contain the `main` package — a small `main.go` that wires things together
- What's currently in `cmd/` is internal application code that happens to set up CLI commands

Currently:
```
main.go              ← package main, calls cmd.ExecuteContext
cmd/
  root.go            ← package cmd, 362 lines of cobra setup + fetch orchestration
  auth.go            ← package cmd, 224 lines
  route.go           ← package cmd, 421 lines
  ...12 more files
internal/
  ...
```

### Fix

- [x] Move `main.go` → `cmd/vibeusage/main.go`
- [x] Move current `cmd/*.go` → `internal/cli/` (package `cli`)
- [x] Update `cmd/vibeusage/main.go` to import `internal/cli` instead of `cmd`
- [x] Update all internal test imports
- [x] The blank provider imports (`_ "...provider/claude"` etc.) stay in `internal/cli/root.go` — that's where the app is assembled

Result:
```
cmd/
  vibeusage/
    main.go          ← package main, tiny, calls cli.ExecuteContext
internal/
  cli/               ← what was cmd/, all the cobra wiring
    root.go
    auth.go
    route.go
    ...
  config/
  display/
  ...
```

## 2. Credential system split from provider system

`config/credentials.go` has hardcoded `ProviderCLIPaths` and `ProviderEnvVars` maps that duplicate knowledge each provider already has. `IsFirstRun()`, `CountConfiguredProviders()`, and `GetAllCredentialStatus()` iterate these maps as the source of truth for "what providers exist" — separate from the actual provider registry.

Adding a provider means updating both the provider package AND the credential maps. These can drift.

### Fix

Move credential knowledge into the provider interface. Each provider already knows its credential paths (strategy `IsAvailable()` checks them) and env vars.

- [x] Add `CredentialSources() CredentialInfo` to the `Provider` interface (or a new optional interface)
  ```go
  type CredentialInfo struct {
      CLIPaths []string // e.g. {"~/.claude/.credentials.json"}
      EnvVar   string   // e.g. "ANTHROPIC_API_KEY"
  }
  ```
- [x] Implement on each provider
- [x] Replace `ProviderCLIPaths` / `ProviderEnvVars` maps with calls to the registry
- [x] `IsFirstRun()`, `CountConfiguredProviders()`, `GetAllCredentialStatus()` iterate `provider.All()` instead of a hardcoded map
- [x] Delete the maps from `config/credentials.go`

## 3. Test file organization in `internal/cli/`

Six test files have no corresponding source file:

| Test file | Tests code in |
|---|---|
| `context_test.go` | `root.go` |
| `flags_test.go` | `init.go` |
| `helpers_test.go` | itself (test-only helper) |
| `json_types_test.go` | auth, config, key, init, cache |
| `logging_test.go` | `root.go` |
| `spinner_test.go` | `root.go` |

Meanwhile `logger_test.go` tests `logger.go`, and `logging_test.go` tests logging behavior — two files for overlapping concepts.

### Fix

- [x] Move `context_test.go` tests into `root_test.go`
- [x] Move `flags_test.go` tests into `init_test.go`
- [x] Move `logging_test.go` tests into `root_test.go`
- [x] Move `spinner_test.go` tests into `root_test.go`
- [x] Merge `logger_test.go` into `root_test.go` (or keep in `logger_test.go` since `logger.go` exists — but then `logging_test.go` should not also exist)
- [x] Move `json_types_test.go` tests into their respective command test files (`auth_test.go`, `config_test.go`, etc.)
- [x] Move `reloadConfig()` from `helpers_test.go` into a `testutil_test.go` or inline where used
- [x] Delete empty/orphaned test files

## 4. Inconsistent provider structure

Every provider is organized differently for no apparent reason:

**File layout:**
- antigravity: `antigravity.go`, `auth.go`, `proto.go`, `response.go` (4 files)
- claude: `claude.go`, `oauth.go`, `web.go`, `response.go` (4 files, different split)
- codex through zai: `<name>.go`, `response.go` (2 files each)

**Auth:**
- 7/9 providers implement `Authenticator` — codex and gemini don't, silently falling through to the generic handler
- copilot and kimi use `DeviceAuthFlow`; claude, cursor, zai, minimax use `ManualKeyAuthFlow`; antigravity uses `DeviceAuthFlow`
- codex and gemini have full OAuth built into their strategies but no way to trigger it interactively

**Test coverage:**
- claude: 50 tests (strategies + responses)
- antigravity: 23 tests (strategies + responses)
- codex/copilot/cursor/gemini: 17-20 tests (strategies + responses)
- kimi/zai/minimax: 12 tests each (response parsing only, no strategy tests, no `<name>_test.go`)

**Validation:**
- claude: `prompt.ValidateClaudeSessionKey` (lives in `prompt` package)
- cursor, zai: `prompt.ValidateNotEmpty` (lives in `prompt` package)
- minimax: `ValidateCodingPlanKey` (lives locally in `minimax` package)
- copilot, kimi, antigravity, codex, gemini: no validation

### Fix

Pick one pattern and apply it everywhere:

- [ ] Every provider gets the same file layout: `<name>.go` (provider struct + strategies), `response.go` (response types), optionally `auth.go` if the auth flow is complex enough to warrant it (e.g. device flows with polling)
- [ ] Every provider implements `Authenticator` — codex and gemini should get proper auth flows or at least explicit `ManualKeyAuthFlow` declarations instead of silent fallthrough
- [ ] Every provider with a `ManualKeyAuthFlow` has validation — move `ValidateClaudeSessionKey` from `prompt` to `claude`, move `ValidateNotEmpty` usages to use a shared helper or define locally, move `ValidateCodingPlanKey` pattern to match wherever validation lands
- [ ] Every provider has a `<name>_test.go` with at minimum strategy availability + basic fetch path tests, matching the pattern claude/codex/copilot already use
- [ ] Strategy type names follow a consistent convention (propose: `OAuthStrategy`, `WebStrategy`, `APIKeyStrategy`, `DeviceFlowStrategy` — drop `BearerTokenStrategy` alias)

## 5. Delete `internal/strutil/`

`strutil.TitleCase` is used ~20 times to convert provider IDs to display names. But every provider already declares its display name in `Meta().Name`. And `TitleCase("zai")` returns `"Zai"` while the actual name is `"Z.ai"`.

### Fix

- [ ] Where code has a provider ID and needs a display name, look up `Meta().Name` from the registry instead of calling `strutil.TitleCase`
- [ ] For non-provider uses (model names in gemini/antigravity), inline the title-casing or use `cases.Title` from `golang.org/x/text`
- [ ] Delete `internal/strutil/` entirely

## 6. Fold `internal/spinner/` into `internal/display/`

The spinner package is 3 files (~130 lines of real code). `format.go` is a type definition and a function that returns a struct field. It's only imported by `cmd/root.go` and `cmd/route.go`.

### Fix

- [ ] Move `spinner.go` and `model.go` into `internal/display/spinner.go` (or `internal/display/progress.go`)
- [ ] Inline `CompletionInfo` and `FormatCompletionText` — the "format" file adds nothing
- [ ] Delete `internal/spinner/`
- [ ] Update imports in `cmd/root.go` and `cmd/route.go`

## 7. Move validation to providers

`internal/prompt/validation.go` has `ValidateClaudeSessionKey` and `ValidateNotEmpty`. The Claude-specific validator has no business in a generic prompt package. `ValidateNotEmpty` is generic enough to live anywhere.

### Fix

- [ ] Move `ValidateClaudeSessionKey` to `internal/provider/claude/`
- [ ] Move `ValidateNotEmpty` to `internal/provider/` (shared by any `ManualKeyAuthFlow` that needs it) or keep in `prompt` since it's genuinely generic
- [ ] Move `ValidateCodingPlanKey` to stay in minimax (already correct) — but the pattern should match the others
- [ ] Delete `internal/prompt/validation.go` if empty after moves
- [ ] `prompt` package should only contain prompt UI concerns (`Prompter` interface, `Huh` impl, `Mock`)

## Order of operations

1. **#1 (project layout)** — do first, every subsequent change uses the new paths
2. **#2 (credentials)** — biggest structural fix, touches the most files
3. **#4 (provider consistency)** — do alongside #2 since both touch every provider
4. **#7 (validation)** — small, falls out of #4 naturally
5. **#5 (strutil)** — easy deletion once providers expose display names properly
6. **#6 (spinner)** — mechanical move
7. **#3 (test files)** — do last, mechanical reorganization
