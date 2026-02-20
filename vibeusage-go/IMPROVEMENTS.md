# vibeusage-go Improvement Plan

Go-idiomatic improvements to move beyond the 1-to-1 Python port.

## Status Key

- [ ] Not started
- [~] In progress
- [x] Complete

## 1. `huh` Forms for Interactive Flows

**Priority**: Highest — biggest UX upgrade
**Packages**: `github.com/charmbracelet/huh`

Replace all raw `fmt.Scanln` calls with styled, validated Charm forms.

### Where

| File | Current Code | Replacement |
|------|-------------|-------------|
| `cmd/init.go` `interactiveWizard()` | `fmt.Scanln(&input)` for provider selection | `huh.NewMultiSelect` with provider options |
| `cmd/auth.go` `authClaude()` | `fmt.Scanln(&sessionKey)` | `huh.NewInput` with `sk-ant-sid01-` prefix validation |
| `cmd/auth.go` `authCursor()` | `fmt.Scanln(&sessionToken)` | `huh.NewInput` with non-empty validation |
| `cmd/auth.go` `authCopilot()` | `fmt.Scanln(&confirm)` for re-auth | `huh.NewConfirm` |
| `cmd/key.go` `makeKeyProviderCmd()` set | `fmt.Scanln(&value)` | `huh.NewInput` with non-empty validation |
| `cmd/key.go` `makeKeyProviderCmd()` delete | `fmt.Scanln(&confirm)` | `huh.NewConfirm` |
| `cmd/config.go` `configResetCmd` | `fmt.Scanln(&response)` | `huh.NewConfirm` |

### Tasks

- [x] Add `github.com/charmbracelet/huh` dependency
- [x] Replace init wizard with `huh.NewMultiSelect` for provider picking
- [x] Replace auth inputs with `huh.NewInput` (with validation funcs)
- [x] Replace all y/N confirmations with `huh.NewConfirm`
- [x] Replace key set input with `huh.NewInput`
- [x] Ensure all forms degrade to non-interactive when piped (huh handles this)
- [x] Skip forms entirely when `--quiet` or `--json` flags are set

## 2. Bubbletea Spinner for Fetch Progress

**Priority**: High — users currently stare at a blank screen during fetches
**Packages**: `github.com/charmbracelet/huh/spinner` (simple) or `github.com/charmbracelet/bubbletea` (full)

### Current Behavior

`fetchAndDisplayAll` and `fetchAndDisplayProvider` block silently while hitting providers over the network. The Python version showed Rich spinners.

### Target Behavior

```
⠋ Fetching claude, copilot, gemini...
✓ copilot (device_flow, 189ms)
✓ claude (oauth, 342ms)
⠋ Fetching gemini...
✓ gemini (oauth, 1.2s)
```

### Hook Point

The orchestrator already has an `onComplete func(FetchOutcome)` callback — perfect for updating a spinner/progress model.

### Tasks

- [x] Add spinner dependency (`huh/spinner` for simple, or bubbletea for full control)
- [x] Create spinner wrapper that tracks in-flight providers
- [x] Wire `onComplete` callback to update spinner state
- [x] Show per-provider completion with timing
- [x] Skip spinner when `--quiet` or `--json` flags are set
- [x] Ensure spinner doesn't interfere with non-TTY output (piped)

## 3. `lipgloss/table` for Tabular Displays

**Priority**: High — replaces all hand-rolled `fmt.Printf("%-12s %-8s ...")` tables
**Packages**: `github.com/charmbracelet/lipgloss/table` (already have lipgloss)

### Where

| File | Function | Description |
|------|----------|-------------|
| `cmd/status.go` | `displayStatusTable()` | Provider health status table |
| `cmd/cache.go` | `cacheShowCmd` | Cache status per provider |
| `cmd/key.go` | `displayAllCredentialStatus()` | Credential status table |
| `cmd/auth.go` | `authStatusCommand()` | Auth status table |

All four do the same pattern:
```go
fmt.Println(strings.Repeat("─", N))
fmt.Printf("%-Ns %-Ns ...\n", headers...)
fmt.Println(strings.Repeat("─", N))
for _, row := range data {
    fmt.Printf("%-Ns %-Ns ...\n", row...)
}
```

### Tasks

- [ ] Create shared table helper in `internal/display/` that wraps lipgloss/table
- [ ] Migrate `displayStatusTable` to use lipgloss table
- [ ] Migrate `cacheShowCmd` display to use lipgloss table
- [ ] Migrate `displayAllCredentialStatus` to use lipgloss table
- [ ] Migrate `authStatusCommand` display to use lipgloss table
- [ ] Respect `--no-color` flag in table styling

## 4. Typed JSON Response Structs

**Priority**: High — biggest code quality upgrade, eliminates runtime panic risk
**Packages**: None (stdlib `encoding/json`)

### Problem

Every provider does:
```go
var data map[string]any
json.Unmarshal(body, &data)
usageAmount, _ := data["usage_amount"].(float64)  // silent zero on wrong type
```

This is fragile — type assertions silently return zero values, nested maps require chains of assertions, and there's no compile-time feedback.

### Target

Define response structs per provider API endpoint:

```go
type claudeUsageResponse struct {
    UsageAmount float64 `json:"usage_amount"`
    UsageLimit  float64 `json:"usage_limit"`
    PeriodEnd   string  `json:"period_end,omitempty"`
}
```

### Files to Migrate

| File | Structs Needed |
|------|---------------|
| `provider/claude/oauth.go` | OAuth usage response, extra usage response |
| `provider/claude/web.go` | Web usage response, org list response, overage response |
| `provider/codex/codex.go` | Usage response (rate_limit, credits, plan_type) |
| `provider/copilot/copilot.go` | User response (quota_snapshots), device flow responses |
| `provider/cursor/cursor.go` | Usage summary response, user/me response |
| `provider/gemini/gemini.go` | Quota response (quota_buckets), code assist response, token refresh response |
| `internal/display/json.go` | `SnapshotToJSON` — already mostly fine, but `OutputErrorJSON` uses `map[string]any` |

### Tasks

- [ ] Define response structs for Claude OAuth API
- [ ] Define response structs for Claude Web API
- [ ] Define response structs for Codex API
- [ ] Define response structs for Copilot API (usage + device flow)
- [ ] Define response structs for Cursor API
- [ ] Define response structs for Gemini API (quota + token refresh)
- [ ] Migrate each provider's `Fetch()` and parse methods to use typed structs
- [ ] Replace `map[string]any` in credential loading where possible

## 5. Shared HTTP Client

**Priority**: Medium — deduplicates ~15 instances of the same boilerplate
**Packages**: None (stdlib `net/http`)

### Problem

Every strategy creates its own client and repeats the same pattern:
```go
client := &http.Client{Timeout: 30 * time.Second}
req, _ := http.NewRequest("GET", url, nil)
req.Header.Set(...)
resp, err := client.Do(req)
defer resp.Body.Close()
body, _ := io.ReadAll(resp.Body)
var data SomeType
json.Unmarshal(body, &data)
```

This appears ~15 times across provider files. Error handling is inconsistent (some check `err` from `http.NewRequest`, most don't).

### Target

```go
// internal/httpclient/client.go
type Client struct {
    http *http.Client
}

func New() *Client { ... }

func (c *Client) GetJSON(ctx context.Context, url string, out any, opts ...RequestOption) error { ... }
func (c *Client) PostJSON(ctx context.Context, url string, body any, out any, opts ...RequestOption) error { ... }
func (c *Client) PostForm(ctx context.Context, url string, form url.Values, out any, opts ...RequestOption) error { ... }

// RequestOption for headers, cookies, etc.
type RequestOption func(*http.Request)
func WithHeader(k, v string) RequestOption { ... }
func WithCookie(c *http.Cookie) RequestOption { ... }
func WithBearer(token string) RequestOption { ... }
```

### Tasks

- [ ] Create `internal/httpclient/` package with shared client
- [ ] Define `RequestOption` pattern for headers/cookies/auth
- [ ] Add `GetJSON`, `PostJSON`, `PostForm` convenience methods
- [ ] Migrate Claude strategies to use shared client
- [ ] Migrate Codex strategy to use shared client
- [ ] Migrate Copilot strategy to use shared client
- [ ] Migrate Cursor strategy to use shared client
- [ ] Migrate Gemini strategies to use shared client
- [ ] Migrate status fetchers to use shared client
- [ ] Read timeout from config instead of hardcoding `30 * time.Second`

## 6. Context Threading Through Strategy Interface

**Priority**: Medium — enables proper Ctrl+C cancellation
**Packages**: None (stdlib `context`)

### Problem

The `Strategy` interface defines `Fetch() (FetchResult, error)` with no context. A context is created in `cmd/root.go` and passed to `ExecutePipeline`, but it only controls the outer timeout — individual HTTP requests inside strategies ignore cancellation.

### Target

```go
type Strategy interface {
    Name() string
    IsAvailable() bool
    Fetch(ctx context.Context) (FetchResult, error)
}
```

### Tasks

- [ ] Update `Strategy` interface to accept `context.Context`
- [ ] Update `ExecutePipeline` to pass context to `strategy.Fetch(ctx)`
- [ ] Update all strategy `Fetch` implementations (12 total across 5 providers)
- [ ] Pass context to HTTP requests via `http.NewRequestWithContext`
- [ ] Wire signal handling (SIGINT/SIGTERM) to context cancellation in `main.go`

## 7. Fix Deprecated `strings.Title`

**Priority**: Medium — generates compiler warnings, will break eventually
**Packages**: `golang.org/x/text/cases`, `golang.org/x/text/language` (or a simple helper)

### Where

`strings.Title` is used in:
- `cmd/root.go` — `makeProviderCmd`
- `cmd/init.go` — `interactiveWizard`, `providerDescriptions`
- `cmd/auth.go` — `authGeneric`, `authStatusCommand`
- `cmd/key.go` — `makeKeyProviderCmd`
- `internal/display/display.go` — `RenderSingleProvider`, `RenderProviderPanel`
- `internal/provider/gemini/gemini.go` — model name formatting

### Options

**Option A**: `golang.org/x/text` (heavy dependency for a simple need)
```go
cases.Title(language.English).String(s)
```

**Option B**: Simple helper (all inputs are ASCII provider names)
```go
func titleCase(s string) string {
    if s == "" { return s }
    return strings.ToUpper(s[:1]) + s[1:]
}
```

### Tasks

- [ ] Choose approach (Option B recommended — inputs are all single-word ASCII)
- [ ] Create `internal/strutil/title.go` helper (or inline in display package)
- [ ] Replace all `strings.Title` calls
- [ ] Verify no compiler warnings remain

## 8. Tests

**Priority**: Medium — the Go version currently has zero tests
**Packages**: `testing` (stdlib), optionally `github.com/stretchr/testify`

### High-Value Test Targets (by risk/complexity)

| Package | What to Test | Estimated Tests |
|---------|-------------|-----------------|
| `internal/models` | `PaceRatio`, `ElapsedRatio`, `Remaining`, `FormatResetCountdown`, `PaceToColor`, `PrimaryPeriod` | ~25 |
| `internal/config` | `Load`, `Save`, `DefaultConfig`, `IsProviderEnabled`, env overrides | ~15 |
| `internal/config` | Credential path resolution, `CheckProviderCredentials` | ~10 |
| `internal/config` | Cache snapshot read/write/clear | ~10 |
| `internal/fetch` | `ExecutePipeline` with mock strategies (success, fallback, fatal, timeout, cache) | ~15 |
| `internal/display` | `RenderBar`, `FormatPeriodLine`, `RenderStaleWarning`, `FormatStatusUpdated` | ~15 |
| `internal/display` | `SnapshotToJSON`, `OutputMultiProviderJSON` structure | ~10 |
| `cmd/` | Integration tests with cobra command execution | ~20 |

### Tasks

- [ ] Set up test infrastructure (decide on testify vs stdlib-only)
- [ ] Write `models` package tests (table-driven)
- [ ] Write `config` package tests (with temp dirs)
- [ ] Write `fetch` pipeline tests (with mock strategies)
- [ ] Write `display` package tests
- [ ] Write provider parse tests (with fixture JSON files)
- [ ] Write CLI integration tests

## 9. `charmbracelet/log` for Verbose Output

**Priority**: Low — nice-to-have polish
**Packages**: `github.com/charmbracelet/log`

### Current

Verbose output is raw `fmt.Printf`:
```go
if verbose {
    fmt.Printf("Fetched in %dms\n", durationMs)
    fmt.Printf("Account: %s\n", snap.Identity.Email)
}
```

### Target

Structured, styled logging:
```go
log.Debug("fetch complete", "provider", "claude", "duration", "342ms", "source", "oauth")
log.Info("account", "email", snap.Identity.Email)
```

### Tasks

- [ ] Add `github.com/charmbracelet/log` dependency
- [ ] Set log level based on `--verbose` / `--quiet` flags
- [ ] Replace verbose `fmt.Printf` calls with structured log calls
- [ ] Ensure log output respects `--no-color` and `--json` flags

## Bonus: Other Small Fixes

These can be done opportunistically alongside the items above.

- [ ] Replace hand-rolled `itoa` in `models.go` and `gemini.go` with `strconv.Itoa`
- [ ] Remove duplicate `fileExists` helper (defined in both `config/credentials.go` and `provider/claude/web.go`)
- [ ] Consistent error wrapping with `fmt.Errorf("...: %w", err)` throughout
- [ ] `http.NewRequest` error handling (currently `req, _ := http.NewRequest(...)` ignores errors everywhere)
