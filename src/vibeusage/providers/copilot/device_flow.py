"""Device flow OAuth strategy for Copilot (GitHub) provider."""

from __future__ import annotations

import json
from datetime import UTC
from datetime import datetime
from datetime import timedelta

from vibeusage.config.credentials import read_credential
from vibeusage.config.credentials import write_credential
from vibeusage.config.paths import config_dir
from vibeusage.core.http import get_http_client
from vibeusage.models import PeriodType
from vibeusage.models import UsagePeriod
from vibeusage.models import UsageSnapshot
from vibeusage.strategies.base import FetchResult
from vibeusage.strategies.base import FetchStrategy


class CopilotDeviceFlowStrategy(FetchStrategy):
    """Fetch Copilot usage using GitHub device flow OAuth.

    Copilot uses GitHub OAuth with the device flow:
    1. Request device code from GitHub
    2. User enters code on GitHub
    3. Poll for token until user completes auth
    4. Use token to fetch usage from Copilot API
    """

    name = "device_flow"

    # GitHub OAuth endpoints
    DEVICE_CODE_URL = "https://github.com/login/device/code"
    TOKEN_URL = "https://github.com/login/oauth/access_token"
    USAGE_URL = "https://api.github.com/copilot_internal/user"

    # OAuth configuration
    CLIENT_ID = "Iv1.b507a08c87ecfe98"  # VS Code Copilot client ID
    SCOPE = "read:user"

    # Credential storage
    CREDENTIAL_FILE = config_dir() / "credentials" / "copilot" / "oauth.json"

    # Polling configuration
    DEFAULT_INTERVAL = 5  # seconds
    MAX_POLL_ATTEMPTS = 60  # 5 minutes max

    def is_available(self) -> bool:
        """Check if OAuth credentials are available."""
        return self.CREDENTIAL_FILE.exists()

    async def fetch(self) -> FetchResult:
        """Fetch usage using device flow OAuth credentials."""
        credentials = self._load_credentials()
        if not credentials:
            return FetchResult.fail(
                "No OAuth credentials found. Run `vibeusage auth copilot` to authenticate.",
                should_fallback=False,
            )

        access_token = credentials.get("access_token")
        if not access_token:
            return FetchResult.fail(
                "Invalid credentials: missing access_token", should_fallback=False
            )

        # Check if token needs refresh
        if self._needs_refresh(credentials):
            credentials = await self._refresh_token(credentials)
            if not credentials:
                return FetchResult.fail(
                    "Failed to refresh token. Run `vibeusage auth copilot` to re-authenticate.",
                    should_fallback=False,
                )
            access_token = credentials.get("access_token")

        # Fetch usage from Copilot API
        async with get_http_client() as client:
            response = await client.get(
                self.USAGE_URL,
                headers={
                    "Authorization": f"Bearer {access_token}",
                    "Accept": "application/json",
                },
            )

            if response.status_code == 401:
                return FetchResult.fail(
                    "OAuth token expired or invalid. Run `vibeusage auth copilot` to re-authenticate.",
                    should_fallback=False,
                )
            if response.status_code == 403:
                return FetchResult.fail(
                    "Not authorized to access Copilot usage. Account may not have Copilot subscription."
                )
            if response.status_code == 404:
                return FetchResult.fail(
                    "Copilot API endpoint not found. Your account may not have Copilot access."
                )
            if response.status_code != 200:
                return FetchResult.fail(f"Usage request failed: {response.status_code}")

            try:
                data = response.json()
            except (json.JSONDecodeError, ValueError):
                return FetchResult.fail("Invalid response from Copilot API")

        snapshot = self._parse_usage_response(data)
        if snapshot is None:
            return FetchResult.fail("Failed to parse Copilot usage response")

        return FetchResult.ok(snapshot)

    def _load_credentials(self) -> dict | None:
        """Load OAuth credentials from file."""
        content = read_credential(self.CREDENTIAL_FILE)
        if content:
            try:
                return json.loads(content)
            except json.JSONDecodeError:
                return None
        return None

    def _needs_refresh(self, credentials: dict) -> bool:
        """Check if token needs refresh (GitHub tokens typically don't expire, but we check anyway)."""
        # GitHub OAuth tokens from device flow don't have a set expiration
        # But we'll check if there's an expires_at field
        expires_at = credentials.get("expires_at")
        if not expires_at:
            return False

        try:
            expiry = datetime.fromisoformat(expires_at)
            # Refresh if expires within 1 day
            threshold = datetime.now(UTC) + timedelta(days=1)
            return threshold >= expiry
        except (ValueError, TypeError):
            return False

    async def _refresh_token(self, credentials: dict) -> dict | None:
        """GitHub OAuth tokens from device flow don't expire, so no refresh needed.

        This is a placeholder for future if GitHub implements refresh tokens.
        """
        # GitHub device flow tokens don't expire, but we check validity
        access_token = credentials.get("access_token")
        if not access_token:
            return None

        # Verify token is still valid
        async with get_http_client() as client:
            response = await client.get(
                "https://api.github.com/user",
                headers={
                    "Authorization": f"Bearer {access_token}",
                    "Accept": "application/json",
                },
            )

            if response.status_code == 200:
                return credentials
            elif response.status_code == 401:
                return None
            else:
                # Assume token is still valid for other status codes
                return credentials

    def _save_credentials(self, credentials: dict) -> None:
        """Save credentials to vibeusage storage."""
        content = json.dumps(credentials).encode()
        write_credential(self.CREDENTIAL_FILE, content)

    def _parse_usage_response(self, data: dict) -> UsageSnapshot | None:
        """Parse usage response from Copilot API.

        Expected format (GitHub Copilot API):
        {
            "premium_interactions": {
                "total": 1000,
                "used": 450,
                "reset_at": "2026-01-23T00:00:00Z"
            },
            "chat_quotas": [
                {
                    "model": "gpt-4",
                    "limit": 30,
                    "used": 15,
                    "reset_at": "2026-01-23T00:00:00Z"
                }
            ],
            "billing_cycle": {
                "start": "2026-01-16T00:00:00Z",
                "end": "2026-02-16T00:00:00Z"
            }
        }
        """
        periods = []

        # Parse premium interactions (monthly quota)
        if premium := data.get("premium_interactions"):
            total = premium.get("total", 0)
            used = premium.get("used", 0)
            if total > 0:
                utilization = int((used / total) * 100)
                resets_at = None
                if reset_str := premium.get("reset_at"):
                    try:
                        resets_at = datetime.fromisoformat(
                            reset_str.replace("Z", "+00:00")
                        )
                    except (ValueError, TypeError):
                        pass

                periods.append(
                    UsagePeriod(
                        name="Monthly",
                        utilization=utilization,
                        period_type=PeriodType.MONTHLY,
                        resets_at=resets_at,
                    )
                )

        # Parse chat quotas (model-specific daily quotas)
        if chat_quotas := data.get("chat_quotas"):
            for quota in chat_quotas:
                model = quota.get("model", "unknown")
                limit = quota.get("limit", 0)
                used = quota.get("used", 0)

                if limit > 0:
                    utilization = int((used / limit) * 100)
                    resets_at = None
                    if reset_str := quota.get("reset_at"):
                        try:
                            resets_at = datetime.fromisoformat(
                                reset_str.replace("Z", "+00:00")
                            )
                        except (ValueError, TypeError):
                            pass

                    periods.append(
                        UsagePeriod(
                            name=f"{model} (Daily)",
                            utilization=utilization,
                            period_type=PeriodType.DAILY,
                            resets_at=resets_at,
                            model=model,
                        )
                    )

        # If no periods found, create a default monthly period
        if not periods:
            # Try to extract from alternative format
            if "quota" in data:
                quota = data["quota"]
                if isinstance(quota, dict):
                    total = quota.get("total", 0)
                    used = quota.get("used", 0)
                    if total > 0:
                        utilization = int((used / total) * 100)
                        periods.append(
                            UsagePeriod(
                                name="Monthly",
                                utilization=utilization,
                                period_type=PeriodType.MONTHLY,
                            )
                        )

        if not periods:
            return None

        # Parse overage (Copilot doesn't have traditional overage, but we check)
        overage = None

        # Parse identity
        from vibeusage.models import ProviderIdentity

        identity = None
        if account := data.get("account"):
            if isinstance(account, dict):
                plan = account.get("plan") or account.get("subscription_tier")
                org = account.get("organization")
                email = account.get("email")
                identity = ProviderIdentity(email=email, organization=org, plan=plan)

        return UsageSnapshot(
            provider="copilot",
            fetched_at=datetime.now(UTC),
            periods=tuple(periods),
            overage=overage,
            identity=identity,
            status=None,
            source="device_flow",
        )
