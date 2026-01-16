# Spec 03: Authentication Strategies

**Status**: Draft
**Dependencies**: 02-data-models (for ProviderIdentity)
**Dependents**: 01-architecture, 04-providers

## Overview

This specification defines the authentication mechanisms for connecting to LLM provider APIs. Each provider may support multiple authentication strategies with automatic fallback chains.

## Design Goals

1. **Strategy Abstraction**: Common interface for all auth mechanisms
2. **Fallback Chains**: Try OAuth first, fall back to web cookies, then CLI
3. **Secure Storage**: Credentials stored with appropriate file permissions
4. **Token Refresh**: Automatic refresh before expiration
5. **Minimal User Friction**: Reuse existing credentials from provider CLIs where possible

## Authentication Strategy Interface

```python
from abc import ABC, abstractmethod
from datetime import datetime
from typing import Protocol

import msgspec


class AuthCredentials(Protocol):
    """Protocol for authentication credentials."""

    def is_expired(self) -> bool:
        """Return True if credentials need refresh."""
        ...

    def to_headers(self) -> dict[str, str]:
        """Return HTTP headers for authenticated requests."""
        ...


class AuthResult(msgspec.Struct, frozen=True):
    """Result of an authentication attempt."""

    success: bool
    credentials: AuthCredentials | None = None
    error: str | None = None
    source: str | None = None  # Which strategy succeeded

    @classmethod
    def ok(cls, credentials: AuthCredentials, source: str) -> "AuthResult":
        return cls(success=True, credentials=credentials, source=source)

    @classmethod
    def fail(cls, error: str) -> "AuthResult":
        return cls(success=False, error=error)


class AuthStrategy(ABC):
    """Base class for authentication strategies."""

    @property
    @abstractmethod
    def name(self) -> str:
        """Strategy identifier (e.g., 'oauth', 'web', 'cli')."""
        ...

    @abstractmethod
    async def is_available(self) -> bool:
        """Check if this strategy can be attempted (credentials exist)."""
        ...

    @abstractmethod
    async def authenticate(self) -> AuthResult:
        """Attempt authentication, returning credentials or error."""
        ...

    async def refresh(self, credentials: AuthCredentials) -> AuthResult:
        """
        Refresh expired credentials.

        Default implementation re-authenticates from scratch.
        Override for strategies that support token refresh.
        """
        return await self.authenticate()
```

## Credential Types

### OAuth2Credentials

For providers using OAuth 2.0 with refresh tokens.

```python
from datetime import datetime, timedelta

# Refresh buffer - refresh when this close to expiry
REFRESH_BUFFER: timedelta = timedelta(minutes=5)


class OAuth2Credentials(msgspec.Struct, frozen=True):
    """OAuth 2.0 credentials with refresh capability."""

    access_token: str
    refresh_token: str | None = None
    expires_at: datetime | None = None
    token_type: str = "Bearer"
    scope: str | None = None

    def is_expired(self) -> bool:
        """Check if access token needs refresh."""
        if self.expires_at is None:
            return False
        return datetime.now(self.expires_at.tzinfo) >= (
            self.expires_at - REFRESH_BUFFER
        )

    def to_headers(self) -> dict[str, str]:
        """Return Authorization header."""
        return {"Authorization": f"{self.token_type} {self.access_token}"}

    def can_refresh(self) -> bool:
        """Check if refresh is possible."""
        return self.refresh_token is not None
```

### SessionCredentials

For browser session cookie authentication.

```python
class SessionCredentials(msgspec.Struct, frozen=True):
    """Browser session cookie credentials."""

    session_key: str
    cookie_name: str = "sessionKey"
    expires_at: datetime | None = None

    def is_expired(self) -> bool:
        """Session cookies don't auto-refresh; check expiry if known."""
        if self.expires_at is None:
            return False
        return datetime.now(self.expires_at.tzinfo) >= self.expires_at

    def to_headers(self) -> dict[str, str]:
        """Return Cookie header."""
        return {"Cookie": f"{self.cookie_name}={self.session_key}"}
```

### APIKeyCredentials

For simple API key authentication.

```python
class APIKeyCredentials(msgspec.Struct, frozen=True):
    """API key/token credentials."""

    api_key: str
    header_name: str = "Authorization"
    prefix: str = "Bearer"

    def is_expired(self) -> bool:
        """API keys don't expire automatically."""
        return False

    def to_headers(self) -> dict[str, str]:
        """Return auth header."""
        if self.prefix:
            return {self.header_name: f"{self.prefix} {self.api_key}"}
        return {self.header_name: self.api_key}
```

## Authentication Strategies

### 1. OAuth 2.0 Strategy

Used by: Claude, Codex, Gemini, VertexAI

```python
from pathlib import Path
import httpx
import json

class OAuth2Config(msgspec.Struct):
    """Configuration for OAuth 2.0 authentication."""

    token_endpoint: str
    client_id: str
    client_secret: str | None = None
    scope: str | None = None
    credentials_file: Path | None = None
    # Days before considering refresh even if not technically expired
    refresh_threshold_days: int = 7


class OAuth2Strategy(AuthStrategy):
    """OAuth 2.0 with token refresh."""

    def __init__(self, config: OAuth2Config):
        self.config = config
        self._client = httpx.AsyncClient()

    @property
    def name(self) -> str:
        return "oauth"

    async def is_available(self) -> bool:
        """Check if credentials file exists."""
        if self.config.credentials_file is None:
            return False
        return self.config.credentials_file.exists()

    async def authenticate(self) -> AuthResult:
        """Load credentials from file."""
        if not await self.is_available():
            return AuthResult.fail("No credentials file found")

        try:
            data = json.loads(self.config.credentials_file.read_text())
            creds = self._parse_credentials(data)

            if creds.is_expired() and creds.can_refresh():
                return await self.refresh(creds)

            return AuthResult.ok(creds, self.name)
        except Exception as e:
            return AuthResult.fail(str(e))

    async def refresh(self, credentials: OAuth2Credentials) -> AuthResult:
        """Refresh access token using refresh token."""
        if not credentials.can_refresh():
            return AuthResult.fail("No refresh token available")

        try:
            response = await self._client.post(
                self.config.token_endpoint,
                data={
                    "grant_type": "refresh_token",
                    "refresh_token": credentials.refresh_token,
                    "client_id": self.config.client_id,
                    **({"client_secret": self.config.client_secret}
                       if self.config.client_secret else {}),
                },
            )
            response.raise_for_status()
            data = response.json()

            new_creds = self._parse_credentials(data)
            await self._save_credentials(data)

            return AuthResult.ok(new_creds, self.name)
        except httpx.HTTPStatusError as e:
            return AuthResult.fail(f"Token refresh failed: {e.response.status_code}")
        except Exception as e:
            return AuthResult.fail(f"Token refresh failed: {e}")

    def _parse_credentials(self, data: dict) -> OAuth2Credentials:
        """Parse credentials from OAuth response or stored file."""
        expires_at = None
        if "expires_at" in data:
            expires_at = datetime.fromisoformat(data["expires_at"])
        elif "expires_in" in data:
            expires_at = datetime.now().astimezone() + timedelta(seconds=data["expires_in"])

        return OAuth2Credentials(
            access_token=data["access_token"],
            refresh_token=data.get("refresh_token"),
            expires_at=expires_at,
            token_type=data.get("token_type", "Bearer"),
            scope=data.get("scope"),
        )

    async def _save_credentials(self, data: dict) -> None:
        """Save refreshed credentials back to file."""
        if self.config.credentials_file:
            self.config.credentials_file.write_text(json.dumps(data, indent=2))
            self.config.credentials_file.chmod(0o600)
```

### 2. Browser Cookie Strategy

Used by: Cursor, Augment, Factory, MiniMax

```python
class CookieConfig(msgspec.Struct):
    """Configuration for browser cookie authentication."""

    cookie_names: list[str]  # Cookie names to look for, in priority order
    domains: list[str]       # Domains to search
    stored_session_file: Path | None = None  # Fallback stored session


class BrowserCookieStrategy(AuthStrategy):
    """Import session cookies from browsers."""

    # Supported browsers in priority order
    BROWSERS = ["safari", "chrome", "firefox", "arc", "brave", "edge"]

    def __init__(self, config: CookieConfig):
        self.config = config

    @property
    def name(self) -> str:
        return "web"

    async def is_available(self) -> bool:
        """
        Check if any browser cookies might be available.

        Note: Actual availability requires reading browser DBs,
        which is expensive. We optimistically return True.
        """
        return True

    async def authenticate(self) -> AuthResult:
        """Try to import cookies from browsers."""
        # Try stored session first (fastest)
        if self.config.stored_session_file and self.config.stored_session_file.exists():
            try:
                data = json.loads(self.config.stored_session_file.read_text())
                return AuthResult.ok(
                    SessionCredentials(
                        session_key=data["session_key"],
                        cookie_name=data.get("cookie_name", self.config.cookie_names[0]),
                    ),
                    "stored"
                )
            except Exception:
                pass  # Fall through to browser import

        # Try each browser
        for browser in self.BROWSERS:
            result = await self._try_browser(browser)
            if result.success:
                # Save for future use
                await self._save_session(result.credentials)
                return result

        return AuthResult.fail(
            f"No session cookies found. Log into the provider in your browser, "
            f"or manually provide a session key."
        )

    async def _try_browser(self, browser: str) -> AuthResult:
        """
        Try to extract cookies from a specific browser.

        Note: Browser cookie extraction requires reading SQLite databases
        and potentially decrypting cookies (Chrome/Chromium on macOS uses
        Keychain, on Linux uses secret service).

        This is a simplified interface - actual implementation will use
        a browser cookie library.
        """
        # Implementation depends on browser and platform
        # See: browser_cookie3, pycookiecheat, or custom implementation
        try:
            cookie = await self._extract_cookie(browser)
            if cookie:
                return AuthResult.ok(
                    SessionCredentials(session_key=cookie),
                    f"web:{browser}"
                )
        except Exception:
            pass
        return AuthResult.fail(f"No cookies in {browser}")

    async def _extract_cookie(self, browser: str) -> str | None:
        """
        Extract cookie from browser.

        Placeholder - actual implementation needs platform-specific code.
        """
        # TODO: Implement browser cookie extraction
        # Options:
        # 1. Use browser_cookie3 library
        # 2. Use pycookiecheat for Chrome
        # 3. Custom SQLite + decryption
        return None

    async def _save_session(self, credentials: SessionCredentials) -> None:
        """Save successful session for future use."""
        if self.config.stored_session_file:
            self.config.stored_session_file.parent.mkdir(parents=True, exist_ok=True)
            self.config.stored_session_file.write_text(json.dumps({
                "session_key": credentials.session_key,
                "cookie_name": credentials.cookie_name,
            }))
            self.config.stored_session_file.chmod(0o600)
```

### 3. Manual Session Key Strategy

Used by: Claude (web fallback), all providers as manual override

```python
class ManualSessionStrategy(AuthStrategy):
    """Use manually provided session key."""

    def __init__(self, session_file: Path, cookie_name: str = "sessionKey"):
        self.session_file = session_file
        self.cookie_name = cookie_name

    @property
    def name(self) -> str:
        return "manual"

    async def is_available(self) -> bool:
        return self.session_file.exists()

    async def authenticate(self) -> AuthResult:
        if not await self.is_available():
            return AuthResult.fail("No session key configured")

        try:
            session_key = self.session_file.read_text().strip()
            return AuthResult.ok(
                SessionCredentials(session_key=session_key, cookie_name=self.cookie_name),
                self.name
            )
        except Exception as e:
            return AuthResult.fail(str(e))
```

### 4. API Key Strategy

Used by: Zai, Copilot (after OAuth)

```python
import os

class APIKeyStrategy(AuthStrategy):
    """Use API key from file or environment."""

    def __init__(
        self,
        key_file: Path | None = None,
        env_var: str | None = None,
        header_name: str = "Authorization",
        prefix: str = "Bearer",
    ):
        self.key_file = key_file
        self.env_var = env_var
        self.header_name = header_name
        self.prefix = prefix

    @property
    def name(self) -> str:
        return "api_key"

    async def is_available(self) -> bool:
        if self.env_var and os.environ.get(self.env_var):
            return True
        if self.key_file and self.key_file.exists():
            return True
        return False

    async def authenticate(self) -> AuthResult:
        # Try environment variable first
        if self.env_var:
            api_key = os.environ.get(self.env_var)
            if api_key:
                return AuthResult.ok(
                    APIKeyCredentials(api_key, self.header_name, self.prefix),
                    f"env:{self.env_var}"
                )

        # Try file
        if self.key_file and self.key_file.exists():
            api_key = self.key_file.read_text().strip()
            return AuthResult.ok(
                APIKeyCredentials(api_key, self.header_name, self.prefix),
                "file"
            )

        return AuthResult.fail("No API key found")
```

### 5. CLI Session Strategy

Used by: Claude, Kiro (shell out to provider CLIs)

```python
import asyncio
import shutil

class CLIConfig(msgspec.Struct):
    """Configuration for CLI-based authentication."""

    command: str           # CLI binary name (e.g., "claude", "kiro-cli")
    usage_args: list[str]  # Arguments to get usage (e.g., ["/usage"])
    version_args: list[str] | None = None  # Arguments to check version


class CLISessionStrategy(AuthStrategy):
    """
    Delegate to provider CLI for authentication.

    The CLI manages its own OAuth/session internally.
    We shell out to it and parse the output.
    """

    def __init__(self, config: CLIConfig):
        self.config = config

    @property
    def name(self) -> str:
        return "cli"

    async def is_available(self) -> bool:
        """Check if CLI is installed and accessible."""
        return shutil.which(self.config.command) is not None

    async def authenticate(self) -> AuthResult:
        """
        CLI strategy doesn't return traditional credentials.

        Instead, it indicates that the CLI should be used for data fetching.
        The provider's fetch implementation will call the CLI directly.
        """
        if not await self.is_available():
            return AuthResult.fail(f"CLI '{self.config.command}' not found in PATH")

        # Verify CLI works by running version check
        try:
            version_args = self.config.version_args or ["--version"]
            proc = await asyncio.create_subprocess_exec(
                self.config.command,
                *version_args,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            await asyncio.wait_for(proc.communicate(), timeout=10.0)

            if proc.returncode == 0:
                # Return a marker credential indicating CLI should be used
                return AuthResult.ok(CLICredentials(self.config.command), self.name)
            else:
                return AuthResult.fail(f"CLI returned exit code {proc.returncode}")
        except asyncio.TimeoutError:
            return AuthResult.fail("CLI timed out")
        except Exception as e:
            return AuthResult.fail(str(e))


class CLICredentials(msgspec.Struct, frozen=True):
    """Marker credentials indicating CLI should be used for fetching."""

    command: str

    def is_expired(self) -> bool:
        return False  # CLI manages its own session

    def to_headers(self) -> dict[str, str]:
        return {}  # Not used for HTTP requests
```

### 6. GitHub Device Flow Strategy

Used by: Copilot

```python
import webbrowser

class DeviceFlowConfig(msgspec.Struct):
    """Configuration for GitHub device flow OAuth."""

    client_id: str
    device_code_url: str = "https://github.com/login/device/code"
    token_url: str = "https://github.com/login/oauth/access_token"
    scope: str = "read:user"
    credentials_file: Path | None = None


class GitHubDeviceFlowStrategy(AuthStrategy):
    """
    GitHub device flow OAuth.

    User visits a URL and enters a code to authorize.
    Used by Copilot.
    """

    def __init__(self, config: DeviceFlowConfig):
        self.config = config
        self._client = httpx.AsyncClient(
            headers={"Accept": "application/json"}
        )

    @property
    def name(self) -> str:
        return "device_flow"

    async def is_available(self) -> bool:
        """Check if stored credentials exist."""
        if self.config.credentials_file and self.config.credentials_file.exists():
            return True
        return True  # Can always attempt interactive flow

    async def authenticate(self) -> AuthResult:
        """Load existing credentials or initiate device flow."""
        # Try stored credentials first
        if self.config.credentials_file and self.config.credentials_file.exists():
            try:
                data = json.loads(self.config.credentials_file.read_text())
                return AuthResult.ok(
                    APIKeyCredentials(
                        api_key=data["access_token"],
                        header_name="Authorization",
                        prefix="token",
                    ),
                    self.name
                )
            except Exception:
                pass

        # Initiate device flow (interactive)
        return await self._device_flow()

    async def _device_flow(self) -> AuthResult:
        """Run interactive device flow."""
        try:
            # Request device code
            response = await self._client.post(
                self.config.device_code_url,
                data={
                    "client_id": self.config.client_id,
                    "scope": self.config.scope,
                },
            )
            response.raise_for_status()
            data = response.json()

            device_code = data["device_code"]
            user_code = data["user_code"]
            verification_uri = data["verification_uri"]
            interval = data.get("interval", 5)

            # Prompt user
            print(f"\nTo authenticate with GitHub:")
            print(f"1. Open: {verification_uri}")
            print(f"2. Enter code: {user_code}\n")

            # Try to open browser
            try:
                webbrowser.open(verification_uri)
            except Exception:
                pass

            # Poll for token
            return await self._poll_for_token(device_code, interval)

        except Exception as e:
            return AuthResult.fail(str(e))

    async def _poll_for_token(self, device_code: str, interval: int) -> AuthResult:
        """Poll GitHub for access token."""
        while True:
            await asyncio.sleep(interval)

            response = await self._client.post(
                self.config.token_url,
                data={
                    "client_id": self.config.client_id,
                    "device_code": device_code,
                    "grant_type": "urn:ietf:params:oauth:grant-type:device_code",
                },
            )
            data = response.json()

            if "access_token" in data:
                # Save credentials
                if self.config.credentials_file:
                    self.config.credentials_file.parent.mkdir(parents=True, exist_ok=True)
                    self.config.credentials_file.write_text(json.dumps(data))
                    self.config.credentials_file.chmod(0o600)

                return AuthResult.ok(
                    APIKeyCredentials(
                        api_key=data["access_token"],
                        header_name="Authorization",
                        prefix="token",
                    ),
                    self.name
                )

            error = data.get("error")
            if error == "authorization_pending":
                continue  # Keep polling
            elif error == "slow_down":
                interval += 5  # Back off
                continue
            elif error == "expired_token":
                return AuthResult.fail("Device code expired. Please try again.")
            elif error == "access_denied":
                return AuthResult.fail("Authorization denied by user.")
            else:
                return AuthResult.fail(f"Device flow error: {error}")
```

### 7. Local Process Strategy

Used by: Antigravity (extracts CSRF from running IDE process)

```python
import re

class LocalProcessConfig(msgspec.Struct):
    """Configuration for local process authentication."""

    process_name: str        # Process to look for
    csrf_pattern: str        # Regex to extract CSRF from command line
    port_range: tuple[int, int] = (8000, 9000)  # Port range to scan


class LocalProcessStrategy(AuthStrategy):
    """
    Authenticate via locally running process.

    Extracts CSRF tokens from running IDE/language server processes.
    Used by Antigravity.
    """

    def __init__(self, config: LocalProcessConfig):
        self.config = config

    @property
    def name(self) -> str:
        return "local_process"

    async def is_available(self) -> bool:
        """Check if target process is running."""
        return await self._find_process() is not None

    async def authenticate(self) -> AuthResult:
        """Extract CSRF token from running process."""
        process_info = await self._find_process()
        if not process_info:
            return AuthResult.fail(
                f"Process '{self.config.process_name}' not running"
            )

        csrf_token, port = process_info
        if not csrf_token:
            return AuthResult.fail("Could not extract CSRF token from process")

        return AuthResult.ok(
            LocalProcessCredentials(csrf_token=csrf_token, port=port),
            self.name
        )

    async def _find_process(self) -> tuple[str | None, int | None] | None:
        """
        Find target process and extract CSRF token.

        Returns (csrf_token, port) or None if not found.
        """
        try:
            # Get process list
            proc = await asyncio.create_subprocess_exec(
                "ps", "-ax", "-o", "pid,command",
                stdout=asyncio.subprocess.PIPE,
            )
            stdout, _ = await proc.communicate()

            for line in stdout.decode().split("\n"):
                if self.config.process_name in line:
                    # Extract CSRF token
                    match = re.search(self.config.csrf_pattern, line)
                    if match:
                        csrf_token = match.group(1)

                        # Find port (separate call to lsof)
                        port = await self._find_port()
                        return (csrf_token, port)

            return None
        except Exception:
            return None

    async def _find_port(self) -> int | None:
        """Find listening port for the process."""
        try:
            proc = await asyncio.create_subprocess_exec(
                "lsof", "-nP", "-iTCP", "-sTCP:LISTEN",
                stdout=asyncio.subprocess.PIPE,
            )
            stdout, _ = await proc.communicate()

            for line in stdout.decode().split("\n"):
                if self.config.process_name in line:
                    # Extract port from lsof output
                    match = re.search(r":(\d+)\s", line)
                    if match:
                        return int(match.group(1))
            return None
        except Exception:
            return None


class LocalProcessCredentials(msgspec.Struct, frozen=True):
    """Credentials for local process communication."""

    csrf_token: str
    port: int
    host: str = "127.0.0.1"

    def is_expired(self) -> bool:
        return False  # Valid as long as process is running

    def to_headers(self) -> dict[str, str]:
        return {"X-CSRF-Token": self.csrf_token}

    @property
    def base_url(self) -> str:
        return f"https://{self.host}:{self.port}"
```

## Credential Storage

### Storage Locations

All credentials use `platformdirs` for cross-platform paths:

```python
from pathlib import Path
import platformdirs

APP_NAME = "vibeusage"

def config_dir() -> Path:
    """User config directory for credentials."""
    return Path(platformdirs.user_config_dir(APP_NAME))

def cache_dir() -> Path:
    """Cache directory for temporary data."""
    return Path(platformdirs.user_cache_dir(APP_NAME))
```

**Layout**:

```
~/.config/vibeusage/           # Linux
~/Library/Application Support/vibeusage/  # macOS
%APPDATA%/vibeusage/           # Windows
├── credentials/
│   ├── claude/
│   │   ├── oauth.json         # OAuth credentials
│   │   └── session-key        # Manual session key
│   ├── codex/
│   │   └── oauth.json
│   ├── copilot/
│   │   └── oauth.json
│   ├── cursor/
│   │   └── session.json       # Stored browser session
│   └── ...
└── config.toml                # User preferences

~/.cache/vibeusage/
├── org-ids/
│   ├── claude
│   └── codex
└── snapshots/                 # Cached usage data
    └── ...
```

### Reusing Existing Credentials

Where possible, vibeusage reuses credentials from provider CLIs:

| Provider | Existing Credential Location |
|----------|------------------------------|
| Claude | `~/.claude/.credentials.json` |
| Codex | `~/.codex/auth.json` |
| VertexAI | `~/.config/gcloud/application_default_credentials.json` |
| Gemini | `~/.gemini/oauth_creds.json` |

The fallback chain tries vibeusage's own credential storage if provider credentials aren't found.

### File Permissions

All credential files are created with mode `0o600` (owner read/write only):

```python
def write_credential_file(path: Path, content: str) -> None:
    """Write credential file with secure permissions."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content)
    path.chmod(0o600)
```

### Keyring Integration (Optional)

For additional security, sensitive credentials can be stored in the system keyring:

```python
import keyring

SERVICE_NAME = "vibeusage"

def store_in_keyring(provider: str, credential_type: str, value: str) -> None:
    """Store credential in system keyring."""
    key = f"{provider}:{credential_type}"
    keyring.set_password(SERVICE_NAME, key, value)

def get_from_keyring(provider: str, credential_type: str) -> str | None:
    """Retrieve credential from system keyring."""
    key = f"{provider}:{credential_type}"
    return keyring.get_password(SERVICE_NAME, key)
```

**Note**: Keyring support is optional and disabled by default. Enable via config if security requirements warrant it.

## Authentication Chain

Providers define their authentication chain in priority order:

```python
class ProviderAuthConfig(msgspec.Struct):
    """Authentication configuration for a provider."""

    strategies: list[AuthStrategy]  # In priority order

    async def authenticate(self) -> AuthResult:
        """Try each strategy until one succeeds."""
        errors = []

        for strategy in self.strategies:
            if not await strategy.is_available():
                continue

            result = await strategy.authenticate()
            if result.success:
                return result

            errors.append(f"{strategy.name}: {result.error}")

        return AuthResult.fail(
            "All authentication methods failed:\n" +
            "\n".join(f"  - {e}" for e in errors)
        )
```

### Provider-Specific Chains

| Provider | Strategy Order |
|----------|---------------|
| Claude | OAuth → Web (manual session) → CLI |
| Codex | OAuth → Web (manual session) |
| Copilot | Device Flow OAuth |
| Cursor | Browser Cookies → Manual Session |
| Gemini | OAuth (via CLI credentials) |
| VertexAI | OAuth (gcloud ADC) |
| Augment | Browser Cookies → Manual Session |
| Factory | Browser Cookies → Manual Session |
| MiniMax | Browser Cookies + Bearer Token |
| Antigravity | Local Process |
| Kiro | CLI |
| Zai | API Key (env var → file) |

## Token Refresh Flow

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Load       │     │  Check       │     │  Use        │
│  Credentials├────►│  Expiration  ├────►│  Credentials│
└─────────────┘     └──────┬───────┘     └─────────────┘
                           │
                    expired?│
                           ▼
                    ┌──────────────┐
                    │  Refresh     │
                    │  Token       │
                    └──────┬───────┘
                           │
                  success? │
              ┌────────────┼────────────┐
              ▼            ▼            ▼
        ┌─────────┐  ┌─────────┐  ┌─────────┐
        │  Save   │  │  Fall   │  │  Report │
        │  New    │  │  Back   │  │  Error  │
        │  Creds  │  │  Chain  │  │         │
        └─────────┘  └─────────┘  └─────────┘
```

## Error Messages

User-friendly error messages with remediation steps:

```python
AUTH_ERROR_MESSAGES = {
    "oauth_expired": (
        "OAuth token expired and refresh failed.\n"
        "Re-authenticate with: vibeusage auth {provider}"
    ),
    "session_invalid": (
        "Session key is invalid or expired.\n"
        "Get a new session key from your browser cookies and run:\n"
        "  vibeusage key {provider} set"
    ),
    "no_credentials": (
        "No credentials found for {provider}.\n"
        "Set up authentication with: vibeusage auth {provider}"
    ),
    "cli_not_found": (
        "{provider} CLI not found.\n"
        "Install it or use a different authentication method."
    ),
    "process_not_running": (
        "{provider} is not running.\n"
        "Start the application and try again."
    ),
}
```

## Open Questions

1. **Browser cookie extraction**: Should we bundle a browser cookie library, or require manual session key entry? Cookie extraction is complex (encryption, platform differences) and may require elevated permissions.

2. **Keyring priority**: Should keyring be checked before files, or only used when explicitly enabled?

3. **Interactive flows**: How should device flow OAuth work in non-interactive environments (CI/CD, cron)?

4. **Credential migration**: Should we automatically migrate credentials from provider CLIs to vibeusage's storage?

## Implementation Notes

- All auth code in `vibeusage/auth/` module
- Use `httpx.AsyncClient` for OAuth token requests
- Consider `keyring` library for optional secure storage
- Browser cookie extraction may need `browser_cookie3` or platform-specific code
- CLI shelling should use `asyncio.create_subprocess_exec` for non-blocking execution
