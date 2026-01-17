# Gemini Provider Setup

The Gemini provider tracks usage statistics from Google Gemini AI.

## Authentication Methods

### Method 1: OAuth (Recommended)

**Pros**: Full feature access, per-model quota tracking
**Cons**: More complex setup

#### Via Gemini CLI

If you have the Gemini CLI installed:

```bash
gemini auth login
```

Vibeusage will automatically detect your credentials from:
- `~/.gemini/oauth_creds.json`

#### Manual OAuth Setup

Run:

```bash
vibeusage auth gemini
```

Follow the OAuth flow in your browser.

### Method 2: API Key

**Pros**: Simple setup
**Cons**: Limited usage data (may only show placeholder usage)

#### Steps

1. Get your API key from [Google AI Studio](https://makersuite.google.com/app/apikey)
2. Set the environment variable:

```bash
export GEMINI_API_KEY=your_api_key_here
vibeusage gemini
```

3. Add to `~/.bashrc` or `~/.zshrc` for persistence:

```bash
echo 'export GEMINI_API_KEY=your_api_key_here' >> ~/.bashrc
source ~/.bashrc
```

### Method 3: Service Account (For enterprise)

**Pros**: Works with Google Cloud projects
**Cons**: More complex setup

```bash
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
vibeusage gemini
```

## Credential Storage

Credentials are detected from:
- `~/.gemini/oauth_creds.json` (Gemini CLI, auto-detected)
- `~/.config/vibeusage/credentials/gemini/oauth.json` (manual OAuth)
- `~/.config/vibeusage/credentials/gemini/api_key.json` (API key)
- `GEMINI_API_KEY` environment variable

## Usage Data Display

The Gemini provider shows:

- **Daily Quota**: Per-model daily usage limits
- **Model-Specific Buckets**: Separate tracking for each Gemini model
- **User Tier**: Free, Pro, or Enterprise tier

## Data Limitations

### API Key Method

When using an API key, vibeusage may show placeholder usage data. For full quota tracking:
1. Use OAuth authentication (Method 1)
2. Or check Google Cloud Console for detailed usage metrics

### OAuth Method

OAuth provides the most accurate usage data including:
- Per-model quota buckets
- Daily reset information
- User tier information

## Requirements

- Google account with Gemini access
- For OAuth: Google Cloud project (optional for basic usage)
- For API key: Valid API key from Google AI Studio

## Troubleshooting

### "Strategy not available"

No credentials detected. Either:

1. Set up OAuth:
   ```bash
   vibeusage auth gemini
   ```

2. Or set an API key:
   ```bash
   export GEMINI_API_KEY=your_api_key_here
   ```

### "No quota data available" (API key)

API keys have limited usage tracking. Switch to OAuth for full features.

### OAuth token expired

OAuth tokens have limited lifetime. Re-authenticate:

```bash
vibeusage auth gemini
```

### Credentials from CLI not detected

Verify the credential file exists:

```bash
ls -la ~/.gemini/oauth_creds.json
cat ~/.gemini/oauth_creds.json
```

### Google Cloud Enterprise

For enterprise users with custom quotas, make sure:
- Your Google Cloud project has Gemini API enabled
- Your service account has usage monitoring permissions
- You're using the correct OAuth scope

## Google Workspace Status

The Gemini provider also checks Google Workspace status for incidents affecting Gemini. Status is fetched from Google's public incidents feed.

## Related Links

- [Google AI Studio](https://makersuite.google.com/)
- [Gemini API Documentation](https://ai.google.dev/docs)
- [Google Cloud Status Dashboard](https://status.cloud.google.com/)
