## Commands
- `go build`: Build the binary
- `go run .`: Run the CLI
- `go test ./...`: Run all tests
- `go test ./... -race -v`: Run tests with race detector and verbose output
- `go test ./... -cover`: Run tests with coverage

## Validation

Run these after implementing to get immediate feedback:

- Tests: `go test ./... -v`
- Race detector: `go test ./... -race`
- Coverage: `go test ./... -cover`
- Lint: `golangci-lint run`
- Format check: `gofmt -l .`
- Tidy: `go mod tidy`

## Operational Notes

- CLI entry point: `go run . [OPTIONS] COMMAND [ARGS]`
- Module: `github.com/joshuadavidthomas/vibeusage`
- Uses cobra for CLI, charmbracelet libs for TUI (huh, lipgloss, bubbletea, log)

### Codebase Patterns

- Typed JSON response structs per provider (no `map[string]any`)
- Shared HTTP client in `internal/httpclient/` with `RequestOption` pattern
- `context.Context` threaded through all strategy `Fetch` calls
- `internal/prompt/` wraps `charmbracelet/huh` forms with testable mock interface
- `internal/spinner/` wraps bubbletea for fetch progress display
- `internal/display/table.go` wraps `lipgloss/table` for all tabular output
- `internal/strutil/title.go` replaces deprecated `strings.Title`
- `charmbracelet/log` for structured verbose/debug output
