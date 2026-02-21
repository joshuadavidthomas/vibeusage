set dotenv-load := true
set unstable := true

# List all available commands
[private]
default:
    @just --list --list-submodules

# Build the binary
build *ARGS:
    go build {{ ARGS }} -o vibeusage .

# Run tests with coverage
coverage *ARGS:
    go test ./... -race -cover {{ ARGS }}

# Format code
fmt *ARGS='.':
    gofmt -w {{ ARGS }}

# Run linter
lint *ARGS:
    golangci-lint run {{ ARGS }}

# Run tests
test *ARGS:
    go test ./... -race {{ ARGS }}

# Tidy go.mod/go.sum
tidy:
    go mod tidy
