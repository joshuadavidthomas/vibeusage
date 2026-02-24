# vibeusage

Track usage across agentic LLM providers from your terminal.

A unified CLI tool that aggregates usage statistics from Claude, OpenAI Codex, GitHub Copilot, Cursor, Gemini, Antigravity, Kimi Code, Kimi K2, OpenRouter, Warp, Amp, Z.ai, and Minimax with consistent formatting, progress indicators, and offline support.

## Features

- **Unified Interface**: Single command to check usage across all configured providers
- **Multiple Providers**: Claude, Codex, Copilot, Cursor, Gemini, Antigravity, Kimi Code, Kimi K2, OpenRouter, Warp, Amp, Z.ai, Minimax
- **Concurrent Fetching**: Check all providers in parallel
- **Offline Support**: Displays cached data when network is unavailable
- **JSON Output**: Scriptable with `--json` flag
- **Progress Indicators**: Real-time feedback during fetch operations
- **Pace-Based Coloring**: Visual indicators show if you're on track to stay within quota
- **Stale Data Warnings**: Know when your usage data is outdated

## Installation

### Prerequisites

- Go 1.25.6 or later

### Install with go install

```bash
go install github.com/joshuadavidthomas/vibeusage@latest
```

### Install from source

```bash
git clone https://github.com/joshuadavidthomas/vibeusage.git
cd vibeusage
go build -o vibeusage .
```

## Quick Start

### First Run

The first time you run vibeusage, you'll be guided through setup:

```bash
vibeusage
```

Follow the interactive wizard to configure your providers.

### Check Usage for All Providers

```bash
vibeusage
```

### Check a Specific Provider

```bash
vibeusage claude
vibeusage codex
vibeusage copilot
vibeusage cursor
vibeusage gemini
vibeusage antigravity
vibeusage kimicode
vibeusage kimik2
vibeusage openrouter
vibeusage warp
vibeusage amp
vibeusage zai
vibeusage minimax
```

### Authenticate with a Provider

```bash
vibeusage auth claude
```

## Provider Setup

Each provider requires specific authentication. Follow the guides below:

### Claude

**Required**: Session key from Claude.ai

```bash
vibeusage auth claude
```

To get your session key:
1. Open https://claude.ai in your browser
2. Open DevTools (F12 or Cmd+Option+I)
3. Go to Application → Cookies → https://claude.ai
4. Find the `sessionKey` cookie
5. Copy its value (starts with `sk-ant-sid01-`)
6. Paste it when prompted

**Alternative**: If you have the Claude CLI installed, vibeusage will automatically use its credentials from `~/.claude/.credentials.json`.

### Codex (OpenAI)

**Required**: OAuth tokens from Codex CLI

```bash
# First, authenticate with the Codex CLI
codex auth login

# Then vibeusage will automatically detect your credentials
vibeusage codex
```

**Alternative**: Set the `OPENAI_API_KEY` environment variable.

### GitHub Copilot

**Required**: GitHub OAuth token via device flow

```bash
vibeusage auth copilot
```

You'll be prompted to:
1. Visit a verification URL in your browser
2. Enter a device code
3. Authorize the vibeusage application

**Alternative**: If you have the GitHub CLI installed, vibeusage can use your existing `gh auth login` credentials.

### Cursor

**Required**: Session token from browser cookies

```bash
vibeusage auth cursor
```

The tool will attempt to extract your session token from your browser automatically. If that fails:

1. Open https://cursor.com in your browser
2. Extract your session cookie
3. Run `vibeusage key set cursor` and paste the token

### Gemini

**Required**: API key or OAuth tokens

```bash
# Option 1: Use API key
export GEMINI_API_KEY=your_api_key_here
vibeusage gemini

# Option 2: Use OAuth (recommended for full features)
vibeusage auth gemini
```

### OpenRouter

**Required**: OpenRouter API key

```bash
vibeusage auth openrouter
```

Set `OPENROUTER_API_KEY` as an alternative.

### Warp

**Required**: Warp API token (`wk-...`)

```bash
vibeusage auth warp
```

Set `WARP_API_KEY` or `WARP_TOKEN` as alternatives.

### Kimi K2

**Required**: Kimi K2 API key

```bash
vibeusage auth kimik2
```

Set `KIMI_K2_API_KEY` (or `KIMI_API_KEY` / `KIMI_KEY`) as alternatives.

### Amp

**Required**: Amp API key or local Amp secrets

```bash
vibeusage auth amp
```

If Amp CLI is installed, vibeusage auto-detects `~/.local/share/amp/secrets.json`.

### Antigravity (Google)

**Required**: Antigravity IDE installed with active Google login

Antigravity credentials are automatically detected from the IDE's state database. No manual setup is needed — just sign into the Antigravity IDE.

```bash
vibeusage antigravity
```

### Kimi Code (Moonshot AI)

**Required**: OAuth token via device flow or API key

```bash
# Option 1: Device flow OAuth (recommended)
vibeusage auth kimicode

# Option 2: Use API key
export KIMI_CODE_API_KEY=your_api_key_here
vibeusage kimicode
```

For device flow, you'll be prompted to authorize in your browser. If you have the [kimi-cli](https://github.com/MoonshotAI/kimi-cli) installed, vibeusage will automatically use its credentials from `~/.kimi/credentials/kimi-code.json`.

### Z.ai (Zhipu AI)

**Required**: API key

```bash
vibeusage auth zai
```

To get your API key:
1. Open https://z.ai/manage-apikey/apikey-list
2. Create a new API key (or copy an existing one)
3. Paste it when prompted

**Alternative**: Set the `ZAI_API_KEY` environment variable.

### Minimax

**Required**: Coding Plan API key

```bash
vibeusage auth minimax
```

To get your Coding Plan API key:
1. Open https://platform.minimax.io/user-center/payment/coding-plan
2. Copy your Coding Plan API key (starts with `sk-cp-`)

**Note**: Standard API keys (`sk-api-`) won't work — you need a Coding Plan key.

**Alternative**: Set the `MINIMAX_API_KEY` environment variable.

## Commands

### Global Options

| Option | Short | Description |
|--------|-------|-------------|
| `--json` | `-j` | Output as JSON for scripting |
| `--no-color` | | Disable colored output |
| `--verbose` | `-v` | Show detailed output (fetch timing, account info) |
| `--quiet` | `-q` | Minimal output (errors only) |
| `--refresh` | | Force refresh, ignore cache |

### Usage Commands

```bash
# Show usage for all configured providers
vibeusage

# Show usage for a specific provider
vibeusage claude
vibeusage codex
vibeusage usage claude    # Same as above

# Force refresh (ignore cache)
vibeusage --refresh

# JSON output for scripting
vibeusage --json
vibeusage claude --json
```

### Authentication Commands

```bash
# Authenticate with a provider
vibeusage auth claude
vibeusage auth codex
vibeusage auth copilot
vibeusage auth cursor
vibeusage auth gemini
vibeusage auth openrouter
vibeusage auth warp
vibeusage auth kimik2
vibeusage auth amp
vibeusage auth kimicode
vibeusage auth minimax
vibeusage auth zai

# Show authentication status for all providers
vibeusage auth --status
```

### Configuration Commands

```bash
# Show current configuration
vibeusage config show

# Show config/cache/credentials directory paths
vibeusage config path

# Reset to default configuration
vibeusage config reset
```

### Key Management Commands

```bash
# Show credential status for all providers
vibeusage key

# Set a credential manually
vibeusage key set claude
vibeusage key set codex

# Delete credentials
vibeusage key claude delete
```

### Cache Commands

```bash
# Show cache status
vibeusage cache show

# Clear all cached data
vibeusage cache clear

# Clear cache for specific provider
vibeusage cache clear claude
```

### Status Commands

```bash
# Show provider health status
vibeusage status

# JSON output
vibeusage status --json
```

## Configuration

### Config File Location

Configuration is stored in:
- **Linux**: `~/.config/vibeusage/config.toml`
- **macOS**: `~/Library/Application Support/vibeusage/config.toml`
- **Windows**: `%APPDATA%\vibeusage\config.toml`

### Default Configuration

```toml
[display]
show_remaining = true     # Show remaining % instead of used %
pace_colors = true        # Use pace-based coloring
reset_format = "countdown" # "countdown" or "absolute"

[fetch]
timeout = 30              # Fetch timeout in seconds
max_concurrent = 5        # Max concurrent provider fetches
stale_threshold = 60      # Stale data threshold in minutes

[credentials]
use_keyring = false                    # Use system keyring
reuse_provider_credentials = true      # Auto-detect CLI credentials
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `VIBEUSAGE_CONFIG_DIR` | Override config directory |
| `VIBEUSAGE_CACHE_DIR` | Override cache directory |
| `VIBEUSAGE_ENABLED_PROVIDERS` | Comma-separated provider list |
| `VIBEUSAGE_NO_COLOR` | Disable colored output |
| `ANTHROPIC_API_KEY` | Claude API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `GEMINI_API_KEY` | Gemini API key |
| `GITHUB_TOKEN` | GitHub token for Copilot |
| `OPENROUTER_API_KEY` | OpenRouter API key |
| `WARP_API_KEY` | Warp API key |
| `WARP_TOKEN` | Warp token fallback |
| `KIMI_K2_API_KEY` | Kimi K2 API key |
| `KIMI_API_KEY` | Kimi/Kimi K2 API key fallback |
| `KIMI_KEY` | Kimi K2 API key fallback |
| `AMP_API_KEY` | Amp API key |
| `KIMI_CODE_API_KEY` | Kimi API key |
| `ZAI_API_KEY` | Z.ai API key |
| `MINIMAX_API_KEY` | Minimax Coding Plan API key |

## Output Format

### Default Display

```
Claude
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Session (5h)  ████████████░░░░░░░░ 58%    resets in 2h 15m

Weekly
  All Models  ████░░░░░░░░░░░░░░░░ 23%    resets in 4d 12h
  Opus        ██░░░░░░░░░░░░░░░░░░ 12%    resets in 4d 12h
  Sonnet      ██████░░░░░░░░░░░░░░ 31%    resets in 4d 12h

╭─ Overage ──────────────────────────────────────────────╮
│ Extra Usage: $5.50 / $100.00 USD                       │
╰────────────────────────────────────────────────────────╯
```

### Progress Bar Colors

- **Green** (≤ 1.15x pace): On track or under pace
- **Yellow** (1.15-1.30x pace): Slightly over pace
- **Red** (> 1.30x pace): Significantly over pace

### JSON Output

```bash
vibeusage --json
```

Returns structured data for scripting:

```json
{
  "providers": {
    "claude": {
      "provider": "claude",
      "fetched_at": "2025-01-16T12:34:56Z",
      "periods": [...],
      "identity": {...},
      "status": {...}
    }
  },
  "errors": {}
}
```

## Development

### Setup

```bash
git clone https://github.com/joshuadavidthomas/vibeusage.git
cd vibeusage
go mod download
```

### Run Tests

```bash
go test ./...
go test ./... -race -v
go test ./... -cover
```

### Build

```bash
go build -o vibeusage .
```

### Lint

```bash
golangci-lint run
```

## License

MIT

## Author

Josh Thomas - [@joshuadavidthomas](https://github.com/joshuadavidthomas)
