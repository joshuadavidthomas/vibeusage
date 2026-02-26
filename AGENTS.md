## Commands

- `just test`: Run tests
- `just lint`: Run linter
- `just fmt`: Format code
- `just build`: Build the binary
- `just coverage`: Run tests with coverage
- `just tidy`: Tidy go.mod/go.sum

## Validation

Run these after implementing to get immediate feedback:

- Tests: `just test`
- Lint: `just lint`
- Format: `just fmt`

## Sandbox

Development uses a local sandbox via `.env` / `.envrc` (loaded by direnv):

- `VIBEUSAGE_CONFIG_DIR=.sandbox/vibeusage/config`
- `VIBEUSAGE_DATA_DIR=.sandbox/vibeusage/data`
- `VIBEUSAGE_CACHE_DIR=.sandbox/vibeusage/cache`

The `vibeusage` shell alias runs `go run ./cmd/vibeusage` with these env vars. When testing CLI behavior manually, use the alias or set these env vars. Check `.sandbox/vibeusage/config/config.toml` for the active sandbox config — it may differ from defaults.

## Operational Notes

- CLI entry point: `go run ./cmd/vibeusage [OPTIONS] COMMAND [ARGS]`
- Module: `github.com/joshuadavidthomas/vibeusage`
- Uses cobra for CLI, charmbracelet libs for TUI (huh, lipgloss, bubbletea, log)

### Codebase Patterns

- Typed JSON response structs per provider (no `map[string]any`); use `json.RawMessage` for fields that vary between string/number
- Shared HTTP client in `internal/httpclient/` with `RequestOption` pattern
- `context.Context` threaded through all strategy `Fetch` calls
- `internal/prompt/` wraps `charmbracelet/huh` forms with testable mock interface
- `internal/spinner/` wraps bubbletea for transient fetch progress (clears on completion)
- `internal/display/table.go` wraps `lipgloss/table` for all tabular output
- `internal/strutil/title.go` replaces deprecated `strings.Title`
- `charmbracelet/log` for structured verbose/debug output
- ANSI-aware padding: don't use `%-*s` with styled text; pad manually after styling
- Cache fallback: always serves stale cache when credentials exist (API down); rejects stale cache when unconfigured
- `--refresh` global flag disables cache fallback entirely
- Error messages: include underlying errors (e.g. JSON parse errors) — don't swallow them
- Error hints: only suggest `vibeusage auth` when the error is actually about credentials
