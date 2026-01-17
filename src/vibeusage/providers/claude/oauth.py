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
        """Load OAuth credentials from file.

        Handles two formats:
        1. Claude CLI format: {"claudeAiOauth": {"accessToken": "...", ...}}
        2. Vibeusage format: {"access_token": "...", ...}
        """
        for path in self.CREDENTIAL_PATHS:
            content = read_credential(path)
            if content:
                try:
                    data = json.loads(content)
                except json.JSONDecodeError:
                    continue

                # Handle Claude CLI format with nested claudeAiOauth key
                if "claudeAiOauth" in data:
                    data = data["claudeAiOauth"]
                    # Convert camelCase to snake_case
                    data = self._convert_claude_cli_format(data)

                return data
        return None

    def _convert_claude_cli_format(self, data: dict) -> dict:
        """Convert Claude CLI credential format to standard format.

        Claude CLI uses camelCase keys, convert to snake_case:
        - accessToken -> access_token
        - refreshToken -> refresh_token
        - expiresAt -> expires_at
        """
        result = {}
        for key, value in data.items():
            # Convert camelCase to snake_case
            snake_key = "".join(
                "_" + c.lower() if c.isupper() else c
                for c in key
            ).lstrip("_")

            # Handle expiresAt timestamp -> expires_at ISO string
            if snake_key == "expires_at":
                if isinstance(value, (int, float)):
                    # Convert millisecond timestamp to ISO string
                    from datetime import datetime, timezone
                    value = datetime.fromtimestamp(value / 1000, tz=timezone.utc).isoformat()

            result[snake_key] = value

        return result

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

        Actual API format (2025-01):
        {
            "five_hour": { "utilization": 0.0, "resets_at": "2026-01-17T06:59:59.846865+00:00" },
            "seven_day": { "utilization": 27.0, "resets_at": "2026-01-22T18:59:59.846886+00:00" },
            "seven_day_sonnet": { "utilization": 3.0, "resets_at": "..." },
            "extra_usage": { "is_enabled": false, ... }
        }
        """
        periods = []

        # Parse standard periods (top-level keys that aren't extra_usage or null)
        period_mapping = {
            "five_hour": ("Session (5h)", PeriodType.SESSION),
            "seven_day": ("All Models", PeriodType.WEEKLY),
            "monthly": ("Monthly", PeriodType.MONTHLY),
        }

        for key, (name, period_type) in period_mapping.items():
            if key in data:
                period_data = data[key]
                if period_data is None:
                    continue

                utilization = period_data.get("utilization")
                resets_at_str = period_data.get("resets_at")

                if utilization is not None:
                    # Parse reset time
                    resets_at = None
                    if resets_at_str:
                        try:
                            resets_at = datetime.fromisoformat(resets_at_str)
                        except (ValueError, TypeError):
                            pass

                    periods.append(
                        UsagePeriod(
                            name=name,
                            utilization=int(utilization),
                            period_type=period_type,
                            resets_at=resets_at,
                        )
                    )

        # Parse model-specific usage (keys like seven_day_sonnet, seven_day_opus)
        model_prefixes = {
            "seven_day_sonnet": "Sonnet",
            "seven_day_opus": "Opus",
            "seven_day_haiku": "Haiku",
        }

        for key, model_name in model_prefixes.items():
            if key in data:
                period_data = data[key]
                if period_data is None:
                    continue

                utilization = period_data.get("utilization")
                resets_at_str = period_data.get("resets_at")

                if utilization is not None:
                    resets_at = None
                    if resets_at_str:
                        try:
                            resets_at = datetime.fromisoformat(resets_at_str)
                        except (ValueError, TypeError):
                            pass

                    periods.append(
                        UsagePeriod(
                            name=model_name,
                            utilization=int(utilization),
                            period_type=PeriodType.WEEKLY,
                            resets_at=resets_at,
                            model=model_name.lower(),
                        )
                    )

        # Parse extra_usage (overage)
        overage = None
        extra_usage = data.get("extra_usage", {})
        if extra_usage and extra_usage.get("is_enabled"):
            used_credits = extra_usage.get("used_credits")
            monthly_limit = extra_usage.get("monthly_limit")
            if used_credits is not None and monthly_limit is not None:
                overage = OverageUsage(
                    used=float(used_credits),
                    limit=float(monthly_limit),
                    currency="USD",
                    is_enabled=True,
                )

        return UsageSnapshot(
            provider="claude",
            fetched_at=datetime.now(timezone.utc),
            periods=tuple(periods),
            overage=overage,
            identity=None,  # OAuth doesn't provide identity
            status=None,
            source="oauth",
        )
