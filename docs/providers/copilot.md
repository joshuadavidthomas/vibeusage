# GitHub Copilot Provider Setup

The Copilot provider tracks usage statistics from GitHub Copilot.

## Authentication Methods

### Method 1: GitHub Device Flow (Recommended)

**Pros**: Official OAuth flow, works without GitHub CLI
**Cons**: Requires browser authorization

#### Steps

1. Run the authentication command:

```bash
vibeusage auth copilot
```

2. You'll see output like:

```
Please visit:
  https://github.com/login/device

And enter code: XXXX-XXXXX
```

3. Open the URL in your browser
4. Enter the device code
5. Authorize the vibeusage application
6. Vibeusage will automatically complete the authentication

### Method 2: GitHub CLI (Automatic)

**Pros**: Uses existing GitHub credentials
**Cons**: Requires GitHub CLI installation

If you have the GitHub CLI (`gh`) installed and authenticated:

```bash
gh auth login
```

Vibeusage will automatically detect your credentials from:
- `~/.config/gh/hosts.yml`

## Credential Storage

Credentials are stored in:
- `~/.config/vibeusage/credentials/copilot/oauth.json`

## Usage Data Display

The Copilot provider shows:

- **Premium Interactions**: Monthly quota for code completion suggestions
- **Chat**: Monthly quota for Copilot Chat (if enabled)
- **Plan Type**: Free, Pro, or Enterprise

## Requirements

- GitHub Copilot subscription (Free, Pro, or Enterprise)
- For Copilot Chat: Copilot Chat add-on or Enterprise plan

## Troubleshooting

### "Authorization pending" during device flow

This is normal while waiting for you to complete authorization in the browser. The authorization will timeout after 90 seconds if not completed.

### "Slow down" message

Wait a few seconds before retrying. The GitHub API is rate-limiting the device flow polling.

### "Access denied"

Make sure:
- You have an active GitHub Copilot subscription
- Your GitHub account has permission to use Copilot
- You're not using a personal access token (use the device flow instead)

### "No quota data available"

Free tier accounts may not have detailed quota information. Upgrade to Copilot Pro for detailed usage tracking.

### Credentials expired

GitHub Copilot OAuth tokens are long-lived but may expire. Re-authenticate:

```bash
vibeusage auth copilot
```

## Enterprise Accounts

For GitHub Enterprise Server deployments, you may need to configure a custom GitHub hostname. Set the environment variable:

```bash
export GITHUB_HOST=github.your-company.com
vibeusage auth copilot
```

## Related Links

- [GitHub Copilot](https://github.com/features/copilot)
- [GitHub Status Page](https://www.githubstatus.com/)
- [GitHub CLI](https://cli.github.com/)
