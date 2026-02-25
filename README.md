# vibeusage

Track usage across agentic LLM providers from your terminal.

As an OSS contributor, I’ve had free GitHub Copilot Pro access for a while, but I kept forgetting to use it and leaving free usage on the table. vibeusage keeps that visible across providers and gives you one place to see account usage, pace, and remaining headroom across your configured providers.

## Installation

### Quick install

macOS/Linux/Windows Subsystem for Linux (WSL):

```bash
curl -fsSL https://raw.githubusercontent.com/joshuadavidthomas/vibeusage/main/install.sh | sh
```

Windows (PowerShell):

```powershell
iwr https://raw.githubusercontent.com/joshuadavidthomas/vibeusage/main/install.ps1 -useb | iex
```

Install scripts place the binary in `~/.local/bin` by default (override with `VIBEUSAGE_INSTALL_DIR`). Ensure that directory is on your `PATH`.

### Homebrew

```bash
brew tap joshuadavidthomas/homebrew
brew install vibeusage
```

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

## Quick Start

If you already use AI coding tools (Claude Code, Codex CLI, Gemini CLI, Copilot, etc.), vibeusage auto-detects their credentials. Just run:

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

For providers that need manual setup (API keys, browser tokens), see [Providers](#providers) below. You can also run `vibeusage init` for a guided setup wizard.

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

If a model name is close but not exact, vibeusage suggests likely matches.

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

vibeusage checks for existing credentials first — CLI/app state files, then environment variables — so most providers work automatically if you already use their tools. Manual auth is there as a fallback.

### Amp

[ampcode.com](https://ampcode.com) — Amp coding assistant. Reports Amp Free daily quota usage and credit balance.

If you have the Amp CLI installed, vibeusage reads credentials from `~/.local/share/amp/secrets.json` automatically. It also picks up `AMP_API_KEY` if you already have it set. Otherwise:

```bash
vibeusage auth amp
```

### Claude Code Pro/Max

[claude.ai](https://claude.ai) — Anthropic's Claude AI assistant. Reports session (5-hour) and weekly usage periods, plus overage spend if enabled. Shows your plan tier (Pro, Max, etc.).

If you have Claude Code installed, vibeusage reads its OAuth credentials automatically — from `~/.claude/.credentials.json` on Linux/Windows, or from macOS Keychain on macOS — including token refresh. This is the recommended path.

As a fallback, you can authenticate with a browser session key:

```bash
vibeusage auth claude
```

This prompts you to copy the `sessionKey` cookie from https://claude.ai (DevTools → Application → Cookies). Session keys don't auto-refresh, so you'll need to re-auth when they expire.

### Cursor

[cursor.com](https://cursor.com) — AI-powered code editor. Reports monthly premium request usage and on-demand spend. Shows your membership type.

Cursor requires a browser session token:

```bash
vibeusage auth cursor
```

The prompt walks you through extracting your session cookie from https://cursor.com (DevTools → Application → Cookies). You can also set it directly with `vibeusage key cursor set`.

### Google Antigravity

[antigravity.google](https://antigravity.google) — Google's AI IDE. Reports per-model usage quotas. Shows your subscription tier.

vibeusage reads credentials from the local Antigravity IDE state automatically. Just sign into Antigravity and it should work — no manual setup needed.

### Google Gemini CLI

[gemini.google.com](https://gemini.google.com) — Google Gemini AI. Reports daily per-model request quotas. Shows your user tier.

If you have the Gemini CLI installed, vibeusage reads its OAuth credentials from `~/.gemini/oauth_creds.json` automatically — including token refresh. This gives you the full quota view from the Cloud Code API.

You can also use an AI Studio API key:

```bash
# If you already have GEMINI_API_KEY set, it just works
vibeusage gemini

# Otherwise, set one up:
vibeusage auth gemini
```

The API key path reports rate-limit-based usage (requests per minute/day) rather than quota percentages.

### GitHub Copilot

[github.com/features/copilot](https://github.com/features/copilot) — GitHub's AI pair programmer. Reports monthly usage across premium interactions, chat, and completions quotas. [Smart routing](#smart-routing) takes into account the multiplier Copilot applies to model requests when considering the Copilot provider.

vibeusage reuses existing Copilot credentials from `~/.config/github-copilot/hosts.json` when available. If you don't have those, authenticate via device flow:

```bash
vibeusage auth copilot
```

This opens a browser-based GitHub authorization flow — you'll get a device code to enter at github.com/login/device.

### Kimi Code

[kimi.com](https://www.kimi.com) — Moonshot AI coding assistant. Reports weekly usage and per-window quotas.

If you have the [kimi-cli](https://github.com/MoonshotAI/kimi-cli) installed, vibeusage reads its credentials from `~/.kimi/credentials/kimi-code.json` automatically — including token refresh. It also picks up `KIMI_CODE_API_KEY` if set.

Otherwise, authenticate via device flow:

```bash
vibeusage auth kimicode
```

### Minimax

[minimax.io](https://www.minimax.io) — Minimax AI coding assistant. Reports per-model usage against your coding plan limits.

Requires a **Coding Plan** API key (starts with `sk-cp-`). Standard API keys (`sk-api-`) won't work. Get yours from https://platform.minimax.io/user-center/payment/coding-plan.

Set `MINIMAX_API_KEY` in your environment, or store one with:

```bash
vibeusage auth minimax
```

### OpenAI Codex

[chatgpt.com](https://chatgpt.com) — OpenAI's ChatGPT and Codex. Reports session and weekly usage periods. Shows your subscription tier (Plus, Pro, etc.).

If you have the Codex CLI installed, vibeusage reads its OAuth credentials automatically — from `~/.codex/auth.json` or macOS Keychain (when configured) — including token refresh. This is the recommended path:

```bash
# Authenticate with the Codex CLI first
codex auth login

# Then vibeusage picks it up automatically
vibeusage codex
```

As a fallback, `vibeusage auth codex` lets you paste a bearer token manually, though those don't auto-refresh.

### macOS Keychain troubleshooting (Claude/Codex)

If `vibeusage` says Claude or Codex is not configured on macOS, but their CLIs are logged in:

```bash
claude auth status --json
codex login status
```

If those succeed, macOS may be blocking keychain access for your terminal process. Open **Keychain Access**, find the relevant entries (`Claude Code-credentials` and/or `Codex Auth`), and allow your terminal app access when prompted.

If your keychain is locked (common over SSH/headless sessions), unlock it first:

```bash
security unlock-keychain
```

Then run `vibeusage auth --status` again.

### OpenRouter

[openrouter.ai](https://openrouter.ai) — Unified model gateway. Reports credit usage (dollars spent vs. total credits).

Requires an API key. Set `OPENROUTER_API_KEY` in your environment, or store one with:

```bash
vibeusage auth openrouter
```

### Warp

[warp.dev](https://warp.dev) — Warp terminal AI. Reports monthly credit usage and bonus credits.

Requires an API key. Set `WARP_API_KEY` in your environment (also accepts `WARP_TOKEN`), or store one with:

```bash
vibeusage auth warp
```

### Z.ai

[z.ai](https://z.ai) — Zhipu AI coding assistant. Reports token quotas and tool usage across session, daily, and monthly windows. Shows your plan tier (Lite, Pro, Max).

Requires an API key. Get one from https://z.ai/manage-apikey/apikey-list. Set `ZAI_API_KEY` in your environment, or store one with:

```bash
vibeusage auth zai
```

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

## Updating

```bash
# Check for updates
vibeusage update --check

# Update to latest release (interactive)
vibeusage update

# Update to latest release (non-interactive)
vibeusage update --yes
```

`vibeusage update` only applies updates for installs managed by the official install scripts.

If you installed with Homebrew, upgrade with:

```bash
brew upgrade vibeusage
```

If you installed with `go install`, rerun:

```bash
go install github.com/joshuadavidthomas/vibeusage@latest
```

## Configuration

### Config File Location

Configuration is stored in:
- **Linux**: `~/.config/vibeusage/config.toml`
- **macOS**: `~/.config/vibeusage/config.toml` (preferred), `~/Library/Application Support/vibeusage/config.toml` (fallback)
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
| `KIMI_API_KEY` | Kimi API key fallback |
| `KIMI_CODE_API_KEY` | Kimi API key |
| `MINIMAX_API_KEY` | Minimax Coding Plan API key |
| `OPENAI_API_KEY` | OpenAI API key |
| `OPENROUTER_API_KEY` | OpenRouter API key |
| `VIBEUSAGE_CACHE_DIR` | Override cache directory |
| `VIBEUSAGE_CONFIG_DIR` | Override config directory |
| `VIBEUSAGE_DATA_DIR` | Override data directory (credentials storage) |
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

vibeusage is licensed under the MIT license. See the [`LICENSE`](LICENSE) file for more information.
