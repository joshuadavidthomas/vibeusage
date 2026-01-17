# Codex (OpenAI) Provider Setup

The Codex provider tracks usage statistics from OpenAI's ChatGPT/Codex service.

## Authentication Methods

### Method 1: Codex CLI (Recommended)

**Pros**: OAuth with automatic token refresh
**Cons**: Requires Codex CLI installation

#### Steps

1. Install the OpenAI Codex CLI if you haven't already:

```bash
npm install -g @openai/codex
# or
pip install openai-codex
```

2. Authenticate with the Codex CLI:

```bash
codex auth login
```

3. Vibeusage will automatically detect your credentials from:
   - `~/.codex/auth.json`

4. Run:

```bash
vibeusage codex
```

### Method 2: Environment Variable

**Pros**: Simple for CI/CD or scripts
**Cons**: Less secure (key in environment), no automatic refresh

```bash
export OPENAI_API_KEY=your_api_key_here
vibeusage codex
```

Add to your `~/.bashrc` or `~/.zshrc` for persistence:

```bash
echo 'export OPENAI_API_KEY=your_api_key_here' >> ~/.bashrc
source ~/.bashrc
```

### Method 3: Manual Credential File

Create a manual OAuth credential file at:
`~/.config/vibeusage/credentials/codex/oauth.json`

```json
{
  "access_token": "your_access_token",
  "refresh_token": "your_refresh_token",
  "expires_at": "2025-01-16T12:00:00Z"
}
```

## Credential Storage

Credentials are detected from:
- `~/.codex/auth.json` (Codex CLI, auto-detected)
- `~/.config/vibeusage/credentials/codex/oauth.json` (manual)
- `OPENAI_API_KEY` environment variable

## Custom Usage URL

If you're using a custom OpenAI endpoint (e.g., enterprise), you can configure it in the Codex CLI config:

Create or edit `~/.codex/config.toml`:

```toml
[api]
usage_url = "https://your-custom-endpoint.com/api/usage"
```

## Usage Data Display

The Codex provider shows:

- **Session**: Primary rate limit window (typically 5 hours)
- **Weekly**: Secondary rate limit window (7 days)
- **Credits**: Available credit balance (if applicable)

## Requirements

- ChatGPT Plus, ChatGPT Team, or ChatGPT Enterprise subscription
- Account with usage API access enabled

## Troubleshooting

### "Strategy not available"

No credentials detected. Make sure you've run:

```bash
codex auth login
```

Or set the `OPENAI_API_KEY` environment variable.

### "No usage data available"

Free tier accounts may not have access to usage statistics. Upgrade to ChatGPT Plus or Team.

### Credentials from CLI not detected

Verify the credential file exists and is readable:

```bash
ls -la ~/.codex/auth.json
cat ~/.codex/auth.json
```

### Enterprise/Custom Endpoint

If using an enterprise endpoint, configure the custom URL in `~/.codex/config.toml` as shown above.

## Related Links

- [OpenAI Platform](https://platform.openai.com/)
- [ChatGPT](https://chat.openai.com/)
- [OpenAI Status Page](https://status.openai.com/)
- [Codex CLI Documentation](https://platform.openai.com/docs/cli)
