# Cursor Provider Setup

The Cursor provider tracks usage statistics from Cursor, the AI-powered code editor.

## Authentication Methods

### Method 1: Browser Cookie Extraction (Automatic)

**Pros**: Automatic extraction from browser
**Cons**: Requires additional libraries for some browsers

#### Steps

1. Make sure you're logged into https://cursor.com in your browser
2. Run the authentication command:

```bash
vibeusage auth cursor
```

3. Vibeusage will attempt to extract your session token automatically

#### Supported Browsers

- Chrome/Chromium
- Firefox
- Safari
- Brave
- Edge
- Arc

#### Browser Cookie Libraries

For automatic extraction, install one of:

```bash
pip install browser-cookie3
# or
pip install pycookiecheat
```

### Method 2: Manual Session Token

**Pros**: Works without additional libraries
**Cons**: Manual process, must be repeated when token expires

#### Steps

1. Open https://cursor.com in your browser
2. Open Developer Tools (F12)
3. Go to the **Application** tab (Chrome/Edge) or **Storage** tab (Firefox)
4. Expand **Cookies** and select https://cursor.com or https://cursor.sh
5. Look for one of these cookies:
   - `WorkosCursorSessionToken`
   - `__Secure-next-auth.session-token`
   - `next-auth.session-token`
6. Copy the cookie value
7. Set the credential:

```bash
vibeusage key set cursor
```

8. Paste the session token when prompted

## Credential Storage

Credentials are stored in:
- `~/.config/vibeusage/credentials/cursor/session.json`

## Session Token Names

Vibeusage tries multiple cookie names in order:
1. `WorkosCursorSessionToken` (primary)
2. `__Secure-next-auth.session-token`
3. `next-auth.session-token`

## Usage Data Display

The Cursor provider shows:

- **Premium Requests**: Monthly quota for AI code completions
- **On-Demand Spend**: Extra usage beyond your monthly quota (USD)

## Requirements

- Cursor subscription (Free or Pro)
- Active Cursor account

## Troubleshooting

### "Browser cookie extraction failed"

Install the browser cookie extraction library:

```bash
pip install browser-cookie3
```

Then retry authentication.

### "No session token found"

Make sure:
- You're logged into https://cursor.com in your browser
- You've used Cursor recently (session may have expired)
- You're checking the correct domain (cursor.com or cursor.sh)

Try manual extraction as described in Method 2 above.

### Session expired

Cursor session tokens expire periodically. Re-authenticate:

```bash
vibeusage auth cursor
```

### Multiple browsers detected

Vibeusage will try all supported browsers. If you want to use a specific browser's session, use manual extraction.

## Linux Users

On Linux, browser cookie extraction may require additional system dependencies:

```bash
# Debian/Ubuntu
sudo apt-get install libsecret-1-dev

# Fedora
sudo dnf install libsecret-devel

# Arch Linux
sudo pacman -S libsecret
```

## Related Links

- [Cursor Website](https://cursor.com)
- [Cursor Status Page](https://status.cursor.com)
- [Cursor Documentation](https://cursor.sh/docs)
