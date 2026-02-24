# vibeusage

Track usage across agentic LLM providers from your terminal.

`vibeusage` gives you one place to see account usage, pace, and remaining headroom across your configured providers.

## Why vibeusage

As an OSS contributor, I’ve had free GitHub Copilot Pro access for a while, but I kept forgetting to use it and leaving free usage on the table. `vibeusage` keeps that visible across providers.

Includes:

- One command for all connected providers
- Pace-aware output (not just percent used)
- Smart model routing with `vibeusage route`
- Works with existing local credentials where possible
- JSON output for scripts and automation

## Installation

### Quick install (recommended)

macOS/Linux/Windows Subsystem for Linux (WSL):

```bash
curl -fsSL https://raw.githubusercontent.com/joshuadavidthomas/vibeusage/main/install.sh | sh
```

Windows (PowerShell):

```powershell
iwr https://raw.githubusercontent.com/joshuadavidthomas/vibeusage/main/install.ps1 -useb | iex
```

Install scripts place the binary in `~/.local/bin` by default (override with `VIBEUSAGE_INSTALL_DIR`). Ensure that directory is on your `PATH`.

### Go install

If you have [Go](https://go.dev/) available, you can also use:

```bash
go install github.com/joshuadavidthomas/vibeusage@latest
```

Or install from source:

```bash
git clone https://github.com/joshuadavidthomas/vibeusage.git
cd vibeusage
go build -o vibeusage ./cmd/vibeusage
```

## Updating

```bash
# Check for updates
vibeusage update --check

# Update to latest release (interactive)
vibeusage update

# Update to latest release (non-interactive)
vibeusage update --yes
```

You can also re-run the [install scripts](#quick-install-recommended) to upgrade in place.

## Quick Start

After installing, run the setup wizard once:

```bash
vibeusage init

```

Then check usage across your configured providers:

```bash
$ vibeusage
╭─Claude───────────────────────────────────────────────────────────────╮
│ Session (5h)           ███████░░░░░░░░░░░░░ 38%    resets in 1h 40m  │
│ Weekly                 ██████████████████░░ 98%    resets in 16h 40m │
╰──────────────────────────────────────────────────────────────────────╯
╭─Codex────────────────────────────────────────────────────────────────╮
│ Session                ███░░░░░░░░░░░░░░░░░ 15%    resets in 15m     │
│ Weekly                 ██░░░░░░░░░░░░░░░░░░ 12%    resets in 6d 19h  │
╰──────────────────────────────────────────────────────────────────────╯
╭─Copilot──────────────────────────────────────────────────────────────╮
│ Monthly (Premium)      ██░░░░░░░░░░░░░░░░░░ 11%    resets in 4d 1h   │
│ Monthly (Chat)         ░░░░░░░░░░░░░░░░░░░░  0%    resets in 4d 1h   │
│ Monthly (Completions)  ░░░░░░░░░░░░░░░░░░░░  0%    resets in 4d 1h   │
╰──────────────────────────────────────────────────────────────────────╯
```

Bars are pace-colored: green (on track), yellow (slightly over pace), red (well over pace).

## Smart routing

Inspired by OpenRouter-style routing, `vibeusage route` picks the best provider for a model based on real usage headroom from your own connected accounts.

You can route a model to the provider with the best current headroom:

```bash
$ vibeusage route claude-opus-4-6
Route: Claude Opus 4.6

╭─────────────┬─────────────────────┬──────────┬──────┬─────────┬───────────┬─────────────╮
│ Provider    │ Usage               │ Headroom │ Cost │ Period  │ Resets In │ Plan        │
├─────────────┼─────────────────────┼──────────┼──────┼─────────┼───────────┼─────────────┤
│ Antigravity │ ░░░░░░░░░░░░░░░ 0%  │ 100%     │ —    │ weekly  │ 1h 19m    │ Antigravity │
│ Copilot     │ █░░░░░░░░░░░░░░ 11% │ 29%      │ 3x   │ monthly │ 4d 1h     │ individual  │
│ Claude      │ ██████████████░ 99% │ 1%       │ —    │ weekly  │ 16h 24m   │ Pro         │
╰─────────────┴─────────────────────┴──────────┴──────┴─────────┴───────────┴─────────────╯
```

Or route via your own role/model group:

```bash
$ vibeusage route --role coding
Route: coding (role)

╭──────────────────┬─────────────┬─────────────────────┬──────────┬──────┬─────────┬───────────┬─────────────╮
│ Model            │ Provider    │ Usage               │ Headroom │ Cost │ Period  │ Resets In │ Plan        │
├──────────────────┼─────────────┼─────────────────────┼──────────┼──────┼─────────┼───────────┼─────────────┤
│ claude-opus-4-6  │ Antigravity │ ░░░░░░░░░░░░░░░ 0%  │ 100%     │ —    │ weekly  │ 2h 24m    │ Antigravity │
│ gpt-5.3-codex    │ Codex       │ █░░░░░░░░░░░░░░ 12% │ 88%      │ —    │ weekly  │ 6d 18h    │ plus        │
│ claude-opus-4-6  │ Copilot     │ █░░░░░░░░░░░░░░ 11% │ 29%      │ 3x   │ monthly │ 4d 1h     │ individual  │
│ claude-opus-4-6  │ Claude      │ ██████████████░ 99% │ 1%       │ —    │ weekly  │ 16h 24m   │ Pro         │
╰──────────────────┴─────────────┴─────────────────────┴──────────┴──────┴─────────┴───────────┴─────────────╯
```

Not sure which model ID to use?

```bash
vibeusage route --list
vibeusage route --list-roles
```

If a model name is close but not exact, `vibeusage` suggests likely matches.

Role-based model groups are configured in `config.toml` under `[roles.<name>]` (see [Routing Roles](#routing-roles)).

## Core Commands

```bash
vibeusage                 # Usage for all configured providers
vibeusage <provider>      # Usage for one provider
vibeusage --json          # JSON output
vibeusage --refresh       # Bypass cache fallback
vibeusage auth --status   # Credential/auth status
vibeusage key             # Credential sources and state
vibeusage route <model>   # Best provider for a model
```

## Providers

Each provider requires specific authentication. Follow the guides below:

### Amp

**Required**: Amp API key or local Amp secrets

```bash
vibeusage auth amp
```

If Amp CLI is installed, vibeusage auto-detects `~/.local/share/amp/secrets.json`.

### Claude Pro/Max

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

### Cursor

**Required**: Session token from browser cookies

```bash
vibeusage auth cursor
```

The tool will attempt to extract your session token from your browser automatically. If that fails:

1. Open https://cursor.com in your browser
2. Extract your session cookie
3. Run `vibeusage key cursor set` and paste the token

### Google Antigravity

**Required**: Antigravity IDE installed with active Google login

Antigravity credentials are automatically detected from the IDE's state database. No manual setup is needed — just sign into the Antigravity IDE.

```bash
vibeusage antigravity
```

### Google Gemini

**Required**: API key or OAuth tokens

```bash
# Option 1: Use API key
export GEMINI_API_KEY=your_api_key_here
vibeusage gemini

# Option 2: Use OAuth (recommended for full features)
vibeusage auth gemini
```

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

### Kimi Code

**Required**: OAuth token via device flow or API key

```bash
# Option 1: Device flow OAuth (recommended)
vibeusage auth kimicode

# Option 2: Use API key
export KIMI_CODE_API_KEY=your_api_key_here
vibeusage kimicode
```

For device flow, you'll be prompted to authorize in your browser. If you have the [kimi-cli](https://github.com/MoonshotAI/kimi-cli) installed, vibeusage will automatically use its credentials from `~/.kimi/credentials/kimi-code.json`.

### Kimi K2

**Required**: Kimi K2 API key

```bash
vibeusage auth kimik2
```

Set `KIMI_K2_API_KEY` (or `KIMI_API_KEY` / `KIMI_KEY`) as alternatives.

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

### OpenAI Codex

**Required**: OAuth tokens from Codex CLI

```bash
# First, authenticate with the Codex CLI
codex auth login

# Then vibeusage will automatically detect your credentials
vibeusage codex
```

**Alternative**: Set the `OPENAI_API_KEY` environment variable.

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

### Z.ai

**Required**: API key

```bash
vibeusage auth zai
```

To get your API key:
1. Open https://z.ai/manage-apikey/apikey-list
2. Create a new API key (or copy an existing one)
3. Paste it when prompted

**Alternative**: Set the `ZAI_API_KEY` environment variable.

## Additional Commands

```bash
# Inspect provider/credential state
vibeusage status
vibeusage auth --status
vibeusage key

# Config and cache
vibeusage config show
vibeusage config path
vibeusage cache show
vibeusage cache clear

# Route discovery
vibeusage route --list
vibeusage route --list-roles
```

Global options:

| Option | Short | Description |
|--------|-------|-------------|
| `--json` | `-j` | Output as JSON for scripting |
| `--no-color` | | Disable colored output |
| `--verbose` | `-v` | Show detailed output |
| `--quiet` | `-q` | Minimal output |
| `--refresh` | `-r` | Disable cache fallback (fresh data or error) |

## Configuration

### Config File Location

Configuration is stored in:
- **Linux**: `~/.config/vibeusage/config.toml`
- **macOS**: `~/Library/Application Support/vibeusage/config.toml`
- **Windows**: `%APPDATA%\vibeusage\config.toml`

### Default Configuration

```toml
[credentials]
reuse_provider_credentials = true  # Auto-detect CLI credentials
use_keyring = false                # Use system keyring

[display]
pace_colors = true                 # Use pace-based coloring
reset_format = "countdown"         # "countdown" or "absolute"
show_remaining = true              # Show remaining % instead of used %

[fetch]
max_concurrent = 5                 # Max concurrent provider fetches
stale_threshold_minutes = 60       # Stale data threshold in minutes
timeout = 30                       # Fetch timeout in seconds
```

### Routing Roles

Define model groups for `vibeusage route --role <name>`:

```toml
[roles.coding]
models = ["claude-opus-4-6", "gpt-5.3-codex", "gemini-3.1-pro-preview"]

[roles.fast]
models = ["claude-haiku-4-5", "gemini-3-flash-preview"]
```

Then route by role:

```bash
vibeusage route --role coding
```

### Environment Variables

| Variable | Description |
|----------|-------------|
| `AMP_API_KEY` | Amp API key |
| `ANTHROPIC_API_KEY` | Claude API key |
| `GEMINI_API_KEY` | Gemini API key |
| `GITHUB_TOKEN` | GitHub token for Copilot |
| `KIMI_API_KEY` | Kimi/Kimi K2 API key fallback |
| `KIMI_CODE_API_KEY` | Kimi API key |
| `KIMI_K2_API_KEY` | Kimi K2 API key |
| `KIMI_KEY` | Kimi K2 API key fallback |
| `MINIMAX_API_KEY` | Minimax Coding Plan API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `OPENROUTER_API_KEY` | OpenRouter API key |
| `VIBEUSAGE_CACHE_DIR` | Override cache directory |
| `VIBEUSAGE_CONFIG_DIR` | Override config directory |
| `VIBEUSAGE_ENABLED_PROVIDERS` | Comma-separated provider list |
| `VIBEUSAGE_NO_COLOR` | Disable colored output |
| `VIBEUSAGE_UPDATE_GITHUB_TOKEN` | Optional GitHub token used by `vibeusage update` (helps avoid rate limits) |
| `WARP_API_KEY` | Warp API key |
| `WARP_TOKEN` | Warp token fallback |
| `ZAI_API_KEY` | Z.ai API key |

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
go build -o vibeusage ./cmd/vibeusage
```

### Lint

```bash
golangci-lint run
```

### Release

See `docs/releasing.md` for the tag-and-publish workflow.

## License

MIT

## Author

Josh Thomas - [@joshuadavidthomas](https://github.com/joshuadavidthomas)
