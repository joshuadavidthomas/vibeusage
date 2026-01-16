"""OAuth strategy for Claude provider."""

import json
from datetime import datetime, timezone
from pathlib import Path

import httpx

from vibeusage.config.credentials import read_credential, write_credential
from vibeusage.config.paths import config_dir
from vibeusage.core.http import get_http_client
from vibeusage.models import OverageUsage, PeriodType, UsagePeriod, UsageSnapshot
from vibeusage.strategies.base import FetchResult, FetchStrategy


class ClaudeOAuthStrategy(FetchStrategy):
    """Fetch Claude usage using OAuth tokens."""

    name = "oauth"

    # OAuth endpoints
    TOKEN_URL = "https://api.anthropic.com/oauth/token"
    USAGE_URL = "https://api.anthropic.com/api/oauth/usage"

    # Credential locations
    CREDENTIAL_PATHS = [
        config_dir() / "credentials" / "claude" / "oauth.json",
        Path.home() / ".claude" / ".credentials.json",
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

        # Fetch usage
        async with get_http_client() as client:
            response = await client.get(
                self.USAGE_URL,
                headers={
                    "Authorization": f"Bearer {access_token}",
                    "anthropic-beta": "oauth-2025-04-20",
                },
            )

            if response.status_code == 401:
                return FetchResult.fail("OAuth token expired or invalid")
            if response.status_code == 403:
                return FetchResult.fail("Not authorized to access usage")
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
        """Check if token needs refresh."""
        expires_at = credentials.get("expires_at")
        if not expires_at:
            return False

        try:
            expiry = datetime.fromisoformat(expires_at)
            # Refresh if expires within 5 minutes
            return datetime.now(timezone.utc) >= expiry
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
                },
                headers={
                    "anthropic-beta": "oauth-2025-04-20",
                },
            )

            if response.status_code != 200:
                return None

            try:
                data = response.json()
            except json.JSONDecodeError:
                return None

        # Update expires_at
        if "expires_in" in data:
            expires_at = datetime.now(timezone.utc) + data["expires_in"]
            data["expires_at"] = expires_at.isoformat()

        # Save updated credentials
        self._save_credentials(data)

        return data

    def _save_credentials(self, credentials: dict) -> None:
        """Save credentials to vibeusage storage."""
        path = self.CREDENTIAL_PATHS[0]
        content = json.dumps(credentials).encode()
        write_credential(path, content)

    def _parse_usage_response(self, data: dict) -> UsageSnapshot | None:
        """Parse usage response from OAuth endpoint.

        Expected format:
        {
            "usage": {
                "five_hour": { "usage": 45, "limit": 100 },
                "seven_day": { "usage": 320, "limit": 1000 },
                ...
            }
        }
        """
        usage_data = data.get("usage", {})
        if not usage_data:
            return None

        periods = []

        # Parse standard periods
        period_mapping = {
            "five_hour": ("5-hour session", PeriodType.SESSION, 5 * 3600),
            "seven_day": ("7-day period", PeriodType.WEEKLY, 7 * 86400),
            "monthly": ("Monthly", PeriodType.MONTHLY, 30 * 86400),
        }

        for key, (name, period_type, seconds) in period_mapping.items():
            if key in usage_data:
                period_data = usage_data[key]
                usage = period_data.get("usage")
                limit = period_data.get("limit")

                if usage is not None and limit is not None:
                    utilization = int((usage / limit) * 100) if limit > 0 else 0

                    # Calculate reset time (approximate)
                    resets_at = None  # OAuth doesn't provide exact reset time

                    periods.append(
                        UsagePeriod(
                            name=name,
                            utilization=utilization,
                            period_type=period_type,
                            resets_at=resets_at,
                        )
                    )

        # Parse model-specific usage if available
        for key in usage_data:
            if key.startswith("model:"):
                model_name = key.split(":", 1)[1]
                period_data = usage_data[key]
                usage = period_data.get("usage")
                limit = period_data.get("limit")

                if usage is not None and limit is not None:
                    utilization = int((usage / limit) * 100) if limit > 0 else 0

                    periods.append(
                        UsagePeriod(
                            name=f"Model: {model_name}",
                            utilization=utilization,
                            period_type=PeriodType.SESSION,
                            resets_at=None,
                            model=model_name,
                        )
                    )

        # Parse overage if available
        overage = None
        if "overage" in usage_data:
            overage_data = usage_data["overage"]
            overage = OverageUsage(
                used=float(overage_data.get("used", 0)),
                limit=float(overage_data.get("limit", 0)),
                currency="USD",
                is_enabled=overage_data.get("enabled", False),
            )

        return UsageSnapshot(
            provider="claude",
            fetched_at=datetime.now(timezone.utc),
            periods=periods,
            overage=overage,
            identity=None,  # OAuth doesn't provide identity
            status=None,
            source="oauth",
        )
