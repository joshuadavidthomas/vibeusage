"""OAuth strategy for Codex (OpenAI) provider."""

import json
from datetime import datetime, timezone, timedelta
from pathlib import Path

import httpx

from vibeusage.config.credentials import read_credential, write_credential
from vibeusage.config.paths import config_dir
from vibeusage.config.settings import get_config
from vibeusage.core.http import get_http_client
from vibeusage.models import OverageUsage, PeriodType, UsagePeriod, UsageSnapshot
from vibeusage.strategies.base import FetchResult, FetchStrategy


class CodexOAuthStrategy(FetchStrategy):
    """Fetch Codex usage using OAuth tokens."""

    name = "oauth"

    # OAuth endpoints
    TOKEN_URL = "https://auth.openai.com/oauth/token"
    DEFAULT_USAGE_URL = "https://chatgpt.com/backend-api/wham/usage"

    # Refresh threshold: 8 days before expiry
    REFRESH_THRESHOLD_DAYS = 8

    # Credential locations (check vibeusage first, then Codex CLI)
    CREDENTIAL_PATHS = [
        config_dir() / "credentials" / "codex" / "oauth.json",
        Path.home() / ".codex" / "auth.json",
    ]

    def is_available(self) -> bool:
        """Check if OAuth credentials are available."""
        for path in self.CREDENTIAL_PATHS:
            if path.exists():
                return True
        return False

    async def fetch(self) -> FetchResult:
        """Fetch usage using OAuth credentials."""
        credentials = self._load_credentials()
        if not credentials:
            return FetchResult.fail("No OAuth credentials found")

        access_token = credentials.get("access_token")
        if not access_token:
            return FetchResult.fail("Invalid credentials: missing access_token")

        # Check if token needs refresh
        if self._needs_refresh(credentials):
            credentials = await self._refresh_token(credentials)
            if not credentials:
                return FetchResult.fail("Failed to refresh token")
            access_token = credentials.get("access_token")

        # Get usage URL from config if specified
        usage_url = self._get_usage_url()

        # Fetch usage
        async with get_http_client() as client:
            response = await client.get(
                usage_url,
                headers={
                    "Authorization": f"Bearer {access_token}",
                },
            )

            if response.status_code == 401:
                return FetchResult.fail("OAuth token expired or invalid", should_fallback=False)
            if response.status_code == 403:
                return FetchResult.fail("Not authorized to access usage. Account may not have ChatGPT Plus/Pro subscription.")
            if response.status_code != 200:
                return FetchResult.fail(f"Usage request failed: {response.status_code}")

            try:
                data = response.json()
            except json.JSONDecodeError:
                return FetchResult.fail("Invalid response from usage endpoint")

        snapshot = self._parse_usage_response(data)
        if snapshot is None:
            return FetchResult.fail("Failed to parse usage response")

        return FetchResult.ok(snapshot)

    def _load_credentials(self) -> dict | None:
        """Load OAuth credentials from file."""
        for path in self.CREDENTIAL_PATHS:
            content = read_credential(path)
            if content:
                try:
                    return json.loads(content)
                except json.JSONDecodeError:
                    continue
        return None

    def _needs_refresh(self, credentials: dict) -> bool:
        """Check if token needs refresh (8 days before expiry)."""
        expires_at = credentials.get("expires_at")
        if not expires_at:
            return False

        try:
            expiry = datetime.fromisoformat(expires_at)
            # Refresh if expires within REFRESH_THRESHOLD_DAYS
            threshold = datetime.now(timezone.utc) + timedelta(days=self.REFRESH_THRESHOLD_DAYS)
            return threshold >= expiry
        except (ValueError, TypeError):
            return True

    async def _refresh_token(self, credentials: dict) -> dict | None:
        """Refresh the OAuth access token."""
        refresh_token = credentials.get("refresh_token")
        if not refresh_token:
            return None

        async with get_http_client() as client:
            response = await client.post(
                self.TOKEN_URL,
                data={
                    "grant_type": "refresh_token",
                    "refresh_token": refresh_token,
                    "client_id": "app_EMoamEEZ73f0CkXaXp7hrann",
                },
            )

            if response.status_code != 200:
                return None

            try:
                data = response.json()
            except json.JSONDecodeError:
                return None

        # Update expires_at (OpenAI uses expires_in seconds)
        if "expires_in" in data:
            expires_at = datetime.now(timezone.utc) + timedelta(seconds=data["expires_in"])
            data["expires_at"] = expires_at.isoformat()

        # Preserve refresh_token if not in response
        if "refresh_token" not in data and "refresh_token" in credentials:
            data["refresh_token"] = credentials["refresh_token"]

        # Save updated credentials
        self._save_credentials(data)

        return data

    def _save_credentials(self, credentials: dict) -> None:
        """Save credentials to vibeusage storage."""
        path = self.CREDENTIAL_PATHS[0]
        content = json.dumps(credentials).encode()
        write_credential(path, content)

    def _get_usage_url(self) -> str:
        """Get usage URL from Codex CLI config or default."""
        # Check ~/.codex/config.toml for custom usage_url
        codex_config_path = Path.home() / ".codex" / "config.toml"
        if codex_config_path.exists():
            try:
                import tomli
            except ImportError:
                # tomli not available, use default
                return self.DEFAULT_USAGE_URL

            try:
                with codex_config_path.open("rb") as f:
                    codex_config = tomli.load(f)
                    if usage_url := codex_config.get("usage_url"):
                        return usage_url
            except (tomli.TOMLDecodeError, OSError):
                # Fall back to default if config can't be read
                pass
        return self.DEFAULT_USAGE_URL

    def _parse_usage_response(self, data: dict) -> UsageSnapshot | None:
        """Parse usage response from Codex API.

        Expected format:
        {
            "rate_limits": {
                "primary": { "used_percent": 58, "reset_timestamp": 1234567890 },
                "secondary": { "used_percent": 23, "reset_timestamp": 1234567890 }
            },
            "credits": { "has_credits": true, "balance": 10.50 },
            "plan_type": "plus"
        }
        """
        periods = []

        rate_limits = data.get("rate_limits", {})

        # Parse primary (session) limit
        if primary := rate_limits.get("primary"):
            utilization = int(primary.get("used_percent", 0))
            resets_at = None
            if reset_ts := primary.get("reset_timestamp"):
                try:
                    resets_at = datetime.fromtimestamp(reset_ts, tz=timezone.utc)
                except (ValueError, TypeError):
                    pass

            periods.append(
                UsagePeriod(
                    name="Session",
                    utilization=utilization,
                    period_type=PeriodType.SESSION,
                    resets_at=resets_at,
                )
            )

        # Parse secondary (weekly) limit
        if secondary := rate_limits.get("secondary"):
            utilization = int(secondary.get("used_percent", 0))
            resets_at = None
            if reset_ts := secondary.get("reset_timestamp"):
                try:
                    resets_at = datetime.fromtimestamp(reset_ts, tz=timezone.utc)
                except (ValueError, TypeError):
                    pass

            periods.append(
                UsagePeriod(
                    name="Weekly",
                    utilization=utilization,
                    period_type=PeriodType.WEEKLY,
                    resets_at=resets_at,
                )
            )

        if not periods:
            return None

        # Parse credits (overage)
        overage = None
        credits = data.get("credits", {})
        if credits.get("has_credits"):
            overage = OverageUsage(
                used=0,  # API doesn't expose used amount
                limit=float(credits.get("balance", 0)),
                currency="credits",
                is_enabled=True,
            )

        # Parse plan info for identity
        from vibeusage.models import ProviderIdentity
        identity = None
        if plan_type := data.get("plan_type"):
            identity = ProviderIdentity(plan=plan_type)

        return UsageSnapshot(
            provider="codex",
            fetched_at=datetime.now(timezone.utc),
            periods=tuple(periods),
            overage=overage,
            identity=identity,
            status=None,
            source="oauth",
        )
