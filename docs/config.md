# Configuration Reference

Vibeusage behavior can be customized through configuration files, environment variables, and command-line options.

## Config File Location

The main configuration file is stored at:

| Platform | Path |
|----------|------|
| Linux | `~/.config/vibeusage/config.toml` |
| macOS | `~/Library/Application Support/vibeusage/config.toml` |
| Windows | `%APPDATA%\vibeusage\config.toml` |

You can view the current configuration with:

```bash
vibeusage config show
```

And find config paths with:

```bash
vibeusage config path
```

## Configuration Options

### Display Settings

```toml
[display]
show_remaining = true     # Show remaining % (true) or used % (false)
pace_colors = true        # Use pace-based coloring (true) or fixed thresholds (false)
reset_format = "countdown" # Time format: "countdown" or "absolute"
```

#### `show_remaining`

- **Type**: Boolean
- **Default**: `true`
- **Description**: Show the remaining percentage instead of used percentage in progress bars

```bash
# show_remaining = true
Session (5h)  ████████░░░░  42% remaining

# show_remaining = false
Session (5h)  ████████░░░░  58% used
```

#### `pace_colors`

- **Type**: Boolean
- **Default**: `true`
- **Description**: Use pace-based coloring instead of fixed utilization thresholds

When `true`:
- Green: Usage pace ≤ 1.15x (on track)
- Yellow: Usage pace 1.15x - 1.30x (slightly over pace)
- Red: Usage pace > 1.30x (significantly over pace)

When `false`:
- Green: Usage < 50%
- Yellow: Usage 50-80%
- Red: Usage > 80%

#### `reset_format`

- **Type**: String
- **Default**: `"countdown"`
- **Values**: `"countdown"` or `"absolute"`
- **Description**: How to display reset times

```bash
# reset_format = "countdown"
resets in 2h 15m

# reset_format = "absolute"
resets at Jan 18, 2025, 3:45 PM
```

### Fetch Settings

```toml
[fetch]
timeout = 30              # HTTP timeout in seconds
max_concurrent = 5        # Maximum concurrent provider fetches
stale_threshold = 60      # Stale data warning threshold in minutes
```

#### `timeout`

- **Type**: Integer
- **Default**: `30`
- **Description**: HTTP request timeout in seconds

Increase if you have a slow network:

```toml
[fetch]
timeout = 60
```

#### `max_concurrent`

- **Type**: Integer
- **Default**: `5`
- **Description**: Maximum number of providers to fetch concurrently

Decrease if you're hitting rate limits:

```toml
[fetch]
max_concurrent = 2
```

#### `stale_threshold`

- **Type**: Integer
- **Default**: `60`
- **Description**: Minutes before cached data is considered stale

Set to `0` to always show stale warnings, or a high value to disable:

```toml
[fetch]
stale_threshold = 0   # Always warn if using cache
```

### Credential Settings

```toml
[credentials]
use_keyring = false                    # Use system keyring for storage
reuse_provider_credentials = true      # Auto-detect CLI credentials
```

#### `use_keyring`

- **Type**: Boolean
- **Default**: `false`
- **Description**: Store sensitive credentials in system keyring instead of files

When enabled, requires the `keyring` package:

```bash
pip install keyring
```

#### `reuse_provider_credentials`

- **Type**: Boolean
- **Default**: `true`
- **Description**: Automatically detect and reuse credentials from provider CLIs

Auto-detected credential sources:
- Claude: `~/.claude/.credentials.json`
- Codex: `~/.codex/auth.json`
- Gemini: `~/.gemini/oauth_creds.json`
- VertexAI: `~/.config/gcloud/application_default_credentials.json`

### Provider-Specific Settings

Override authentication method per provider:

```toml
[providers.claude]
enabled = true              # Enable/disable provider
auth_source = "auto"        # "auto", "oauth", "web", "cli"

[providers.codex]
enabled = true
auth_source = "auto"

[providers.copilot]
enabled = true
auth_source = "auto"

[providers.cursor]
enabled = true
auth_source = "auto"
preferred_browser = "chrome"  # For cookie extraction

[providers.gemini]
enabled = true
auth_source = "auto"
```

#### `enabled`

- **Type**: Boolean
- **Default**: `true`
- **Description**: Enable or disable a specific provider

#### `auth_source`

- **Type**: String
- **Values**: `"auto"`, `"oauth"`, `"web"`, `"cli"`
- **Description**: Force specific authentication method

#### `preferred_browser` (Cursor only)

- **Type**: String
- **Values**: `"chrome"`, `"firefox"`, `"safari"`, `"brave"`, `"edge"`, `"arc"`
- **Description**: Browser to prioritize for cookie extraction

## Environment Variables

Environment variables override configuration file settings.

| Variable | Description |
|----------|-------------|
| `VIBEUSAGE_CONFIG_DIR` | Override config directory path |
| `VIBEUSAGE_CACHE_DIR` | Override cache directory path |
| `VIBEUSAGE_ENABLED_PROVIDERS` | Comma-separated list of enabled providers |
| `VIBEUSAGE_NO_COLOR` | Disable colored output (set to `1`) |
| `ANTHROPIC_API_KEY` | Claude API key |
| `OPENAI_API_KEY` | OpenAI/Codex API key |
| `GEMINI_API_KEY` | Gemini API key |
| `GITHUB_TOKEN` | GitHub token for Copilot |
| `CURSOR_API_KEY` | Cursor API key |

### Examples

```bash
# Enable specific providers only
export VIBEUSAGE_ENABLED_PROVIDERS="claude,codex"

# Disable colored output
export VIBEUSAGE_NO_COLOR=1

# Use custom config directory
export VIBEUSAGE_CONFIG_DIR="/custom/path/to/config"
```

## Directory Structure

```
~/.config/vibeusage/
├── config.toml              # Main configuration file
└── credentials/
    ├── claude/
    │   ├── session.json     # Session key credentials
    │   └── oauth.json       # OAuth tokens
    ├── codex/
    │   └── oauth.json
    ├── copilot/
    │   └── oauth.json
    ├── cursor/
    │   └── session.json
    └── gemini/
        ├── oauth.json
        └── api_key.json

~/.cache/vibeusage/
├── snapshots/               # Cached usage data
│   ├── claude.json
│   ├── codex.json
│   └── ...
└── org-ids/                 # Cached organization IDs
    ├── claude
    └── codex
```

## Reset Configuration

To reset to default configuration:

```bash
vibeusage config reset
```

This removes `config.toml` and restores default values.

## Edit Configuration Directly

```bash
# Open config in default editor
vibeusage config edit

# Or edit manually
nano ~/.config/vibeusage/config.toml
# or
vim ~/.config/vibeusage/config.toml
```

## Example Configurations

### Minimal Configuration

```toml
# Use all defaults
# config.toml can be empty or missing
```

### Development Configuration

```toml
[display]
show_remaining = false
pace_colors = true
reset_format = "absolute"

[fetch]
timeout = 10
stale_threshold = 5
```

### CI/CD Configuration

```toml
[credentials]
reuse_provider_credentials = false

[fetch]
timeout = 15
```

Then use environment variables for credentials:

```bash
export ANTHROPIC_API_KEY="sk-..."
export OPENAI_API_KEY="sk-..."
vibeusage --json
```
