"""Authentication base classes and protocols."""

from __future__ import annotations

from abc import ABC, abstractmethod
from datetime import datetime, timedelta
from pathlib import Path
from typing import Protocol

import msgspec


# Refresh buffer - refresh when this close to expiry
REFRESH_BUFFER: timedelta = timedelta(minutes=5)


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
    def ok(cls, credentials: AuthCredentials, source: str) -> type[AuthResult]:
        return cls(success=True, credentials=credentials, source=source)

    @classmethod
    def fail(cls, error: str) -> type[AuthResult]:
        return cls(success=False, error=error)


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
        return {f"{self.token_type}": self.access_token}

    def can_refresh(self) -> bool:
        """Check if refresh is possible."""
        return self.refresh_token is not None


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
        return {self.cookie_name: self.session_key}


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


class CLICredentials(msgspec.Struct, frozen=True):
    """Marker credentials indicating CLI should be used for fetching."""

    command: str

    def is_expired(self) -> bool:
        return False  # CLI manages its own session

    def to_headers(self) -> dict[str, str]:
        return {}  # Not used for HTTP requests


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
        return f"http://{self.host}:{self.port}"


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


class OAuth2Config(msgspec.Struct):
    """Configuration for OAuth 2.0 authentication."""

    token_endpoint: str
    client_id: str
    client_secret: str | None = None
    scope: str | None = None
    credentials_file: Path | None = None
    refresh_threshold_days: int = 7


class CookieConfig(msgspec.Struct):
    """Configuration for browser cookie authentication."""

    cookie_names: list[str]  # Cookie names to look for, in priority order
    domains: list[str]  # Domains to search
    stored_session_file: Path | None = None  # Fallback stored session


class CLIConfig(msgspec.Struct):
    """Configuration for CLI-based authentication."""

    command: str  # CLI binary name (e.g., "claude", "kiro-cli")
    usage_args: list[str]  # Arguments to get usage (e.g., ["/usage"])
    version_args: list[str] | None = None  # Arguments to check version


class DeviceFlowConfig(msgspec.Struct):
    """Configuration for GitHub device flow OAuth."""

    client_id: str
    device_code_url: str = "https://github.com/login/device/code"
    token_url: str = "https://github.com/login/oauth/access_token"
    scope: str = "read:user"
    credentials_file: Path | None = None


class LocalProcessConfig(msgspec.Struct):
    """Configuration for local process authentication."""

    process_name: str  # Process to look for
    csrf_pattern: str  # Regex to extract CSRF from command line
    port_range: tuple[int, int] = (8000, 9000)  # Port range to scan


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
            "All authentication methods failed:\n"
            + "\n".join(f"  - {e}" for e in errors)
        )
