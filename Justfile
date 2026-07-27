set dotenv-load := true
set unstable := true

# List all available commands
[private]
default:
    @just --list --list-submodules

# Build the binary
build *ARGS:
    go build {{ ARGS }} -o vibeusage ./cmd/vibeusage

# Run tests with coverage
coverage *ARGS:
    go test ./... -race -cover {{ ARGS }}

# Format code
fmt *ARGS='.':
    gofmt -w {{ ARGS }}

# Run all pre-commit hooks
lint:
    pre-commit run --all-files

# CLI
run *ARGS:
    go run ./cmd/vibeusage {{ ARGS }}

# Regenerate README screenshots (requires freeze)
screenshots:
    bash scripts/screenshots.sh

# Run tests
test *ARGS:
    go test ./... -race {{ ARGS }}

# Run static analysis
vet:
    go vet ./...

# Run all checks
check: test lint vet tidy

# Tidy go.mod/go.sum
tidy:
    go mod tidy
