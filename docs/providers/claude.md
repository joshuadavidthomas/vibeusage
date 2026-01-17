# Claude Provider Setup

The Claude provider tracks usage statistics from Anthropic's Claude AI service.

## Authentication Methods

### Method 1: Session Key (Recommended for most users)

**Pros**: Simple, works with Claude.ai web account
**Cons**: Session keys expire periodically (needs re-authentication)

#### Steps

1. Open https://claude.ai in your browser
2. Open Developer Tools:
   - **Chrome/Edge**: F12 or Ctrl+Shift+I (Windows/Linux), Cmd+Option+I (macOS)
   - **Firefox**: F12 or Ctrl+Shift+I (Windows/Linux), Cmd+Option+I (macOS)
   - **Safari**: Cmd+Option+I (must enable in Preferences > Advanced > Show Develop menu)
3. Go to the **Application** tab (Chrome/Edge) or **Storage** tab (Firefox)
4. Expand **Cookies** and select https://claude.ai
5. Find the cookie named `sessionKey`
6. Copy its value (starts with `sk-ant-sid01-`)
7. Run the authentication command:

```bash
vibeusage auth claude
```

8. Paste your session key when prompted

### Method 2: Claude CLI (Automatic)

**Pros**: Automatic credential detection, OAuth with refresh tokens
**Cons**: Requires Claude CLI installation

If you have the Claude CLI installed, vibeusage will automatically detect and use your existing credentials from:
- `~/.claude/.credentials.json`

No additional setup required!

## Credential Storage

Credentials are stored in:
- `~/.config/vibeusage/credentials/claude/session.json` (session key method)

## Usage Data Display

The Claude provider shows:

- **Session (5h)**: 5-hour rolling window usage
- **Weekly**: 7-day usage across all models
- **Opus**: Claude Opus model-specific usage
- **Sonnet**: Claude Sonnet model-specific usage
- **Overage**: Extra spend beyond your monthly limit (if applicable)

## Troubleshooting

### "Invalid session key format"

Session keys must start with `sk-ant-sid01-`. Make sure you copied the entire cookie value.

### Session expired

Claude session keys expire periodically. Re-authenticate:

```bash
vibeusage auth claude
```

### "No organizations found"

This error occurs when your session key is valid but your account doesn't have access to any organizations with chat capabilities. Make sure:
- Your Claude account is active
- You're using the correct account (check browser for which account is logged in)

### OAuth not refreshing

If using the Claude CLI credentials and token refresh fails, try:

```bash
claude auth login
```

Then re-run vibeusage.

## Related Links

- [Claude.ai](https://claude.ai)
- [Anthropic API Documentation](https://docs.anthropic.com/)
- [Claude Status Page](https://status.anthropic.com)
