## Commands

- `just check`: Run all checks (lint + test)
- `just test`: Run tests
- `just lint`: Run linter
- `just fmt`: Format code
- `just build`: Build the binary
- `just coverage`: Run tests with coverage
- `just tidy`: Tidy go.mod/go.sum

## Validation

Run these after implementing to get immediate feedback:

- All checks: `just check`
- Tests only: `just test`
- Lint only: `just lint`
- Format: `just fmt`

## Operational Notes

- CLI entry point: `go run . [OPTIONS] COMMAND [ARGS]`
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
