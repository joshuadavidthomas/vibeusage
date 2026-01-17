"""Web (session key) strategy for Claude provider."""

from __future__ import annotations

import json
from datetime import UTC
from datetime import datetime

from vibeusage.config.cache import cache_org_id
from vibeusage.config.cache import load_cached_org_id
from vibeusage.config.credentials import credential_path
from vibeusage.config.credentials import read_credential
from vibeusage.config.credentials import write_credential
from vibeusage.core.http import get_http_client
from vibeusage.models import OverageUsage
from vibeusage.models import PeriodType
from vibeusage.models import UsagePeriod
from vibeusage.models import UsageSnapshot
from vibeusage.strategies.base import FetchResult
from vibeusage.strategies.base import FetchStrategy


class ClaudeWebStrategy(FetchStrategy):
    """Fetch Claude usage using web session key."""

    name = "web"

    # API endpoints
    ORG_URL = "https://claude.ai/api/organizations"
    USAGE_URL_TEMPLATE = "https://claude.ai/api/organizations/{org_id}/usage"
    OVERAGE_URL_TEMPLATE = (
        "https://claude.ai/api/organizations/{org_id}/overage_spend_limit"
    )

    # Session key location
    SESSION_PATH = credential_path("claude", "session")

    def is_available(self) -> bool:
        """Check if session key is available."""
        return self.SESSION_PATH.exists()

    async def fetch(self) -> FetchResult:
        """Fetch usage using session key."""
        session_key = self._load_session_key()
        if not session_key:
            return FetchResult.fail("No session key found")

        # Get organization ID
        org_id = await self._get_org_id(session_key)
        if not org_id:
            return FetchResult.fail("Failed to get organization ID")

        # Fetch usage
        async with get_http_client() as client:
            # Fetch usage
            usage_url = self.USAGE_URL_TEMPLATE.format(org_id=org_id)
            usage_response = await client.get(
                usage_url,
                cookies={"sessionKey": session_key},
            )

            if usage_response.status_code == 401:
                return FetchResult.fail("Session key expired or invalid", fatal=True)
            if usage_response.status_code != 200:
                return FetchResult.fail(
                    f"Usage request failed: {usage_response.status_code}"
                )

            try:
                usage_data = usage_response.json()
            except json.JSONDecodeError:
                return FetchResult.fail("Invalid usage response")

            # Fetch overage
            overage = None
            overage_url = self.OVERAGE_URL_TEMPLATE.format(org_id=org_id)
            overage_response = await client.get(
                overage_url,
                cookies={"sessionKey": session_key},
            )

            if overage_response.status_code == 200:
                try:
                    overage = self._parse_overage(overage_response.json())
                except (json.JSONDecodeError, ValueError):
                    pass

        snapshot = self._parse_usage_response(usage_data, overage)
        if snapshot is None:
            return FetchResult.fail("Failed to parse usage response")

        return FetchResult.ok(snapshot)

    def _load_session_key(self) -> str | None:
        """Load session key from file."""
        content = read_credential(self.SESSION_PATH)
        if content:
            try:
                data = json.loads(content)
                return data.get("session_key")
            except json.JSONDecodeError:
                # Try as raw string
                return content.decode().strip()
        return None

    async def _get_org_id(self, session_key: str) -> str | None:
        """Get organization ID, using cache if available."""
        # Check cache first
        cached = load_cached_org_id("claude")
        if cached:
            return cached

        # Fetch from API
        async with get_http_client() as client:
            response = await client.get(
                self.ORG_URL,
                cookies={"sessionKey": session_key},
            )

            if response.status_code != 200:
                return None

            try:
                data = response.json()
            except json.JSONDecodeError:
                return None

        # Find organization with "chat" capability (Claude Max)
        orgs = data if isinstance(data, list) else []
        for org in orgs:
            capabilities = org.get("capabilities", [])
            if "chat" in capabilities:
                org_id = org.get("uuid") or org.get("id")
                if org_id:
                    # Cache the org ID
                    cache_org_id("claude", org_id)
                    return org_id

        # Fallback to first org
        if orgs:
            org_id = orgs[0].get("uuid") or orgs[0].get("id")
            if org_id:
                cache_org_id("claude", org_id)
                return org_id

        return None

    def _parse_overage(self, data: dict) -> OverageUsage | None:
        """Parse overage data from response."""
        try:
            return OverageUsage(
                used=float(data.get("current_spend", 0)),
                limit=float(data.get("hard_limit", 0)),
                currency="USD",
                is_enabled=data.get("has_hard_limit", False),
            )
        except (ValueError, TypeError):
            return None

    def _parse_usage_response(
        self,
        data: dict,
        overage: OverageUsage | None,
    ) -> UsageSnapshot | None:
        """Parse usage response from web API.

        Expected format:
        {
            "usage": {
                "usage_amount": 45.2,
                "usage_limit": 100.0
            },
            "period_start": "2024-01-01T00:00:00Z",
            "period_end": "2024-01-08T00:00:00Z"
        }
        """
        if not data:
            return None

        periods = []

        # Primary usage period
        usage_amount = data.get("usage_amount")
        usage_limit = data.get("usage_limit")

        if usage_amount is not None and usage_limit is not None:
            try:
                utilization = (
                    int((usage_amount / usage_limit) * 100) if usage_limit > 0 else 0
                )
            except (ValueError, TypeError):
                utilization = 0

            # Parse period end for reset time
            resets_at = None
            period_end = data.get("period_end") or data.get("reset_at")
            if period_end:
                try:
                    resets_at = datetime.fromisoformat(
                        period_end.replace("Z", "+00:00")
                    )
                except (ValueError, AttributeError):
                    pass

            periods.append(
                UsagePeriod(
                    name="Usage",
                    utilization=utilization,
                    period_type=PeriodType.DAILY,
                    resets_at=resets_at,
                )
            )

        # Build identity if available
        identity = None
        if "organization" in data or "email" in data:
            from vibeusage.models import ProviderIdentity

            identity = ProviderIdentity(
                email=data.get("email"),
                organization=data.get("organization"),
                plan=data.get("plan"),
            )

        return UsageSnapshot(
            provider="claude",
            fetched_at=datetime.now(UTC),
            periods=periods,
            overage=overage,
            identity=identity,
            status=None,
            source="web",
        )


class ClaudeBrowserCookieStrategy(FetchStrategy):
    """Extract Claude usage from browser cookies.

    This strategy attempts to extract the session key from browser cookies.
    """

    name = "browser"

    # Cookie domains and names to try
    COOKIE_DOMAINS = [".claude.ai", "claude.ai"]
    COOKIE_NAMES = ["sessionKey"]

    def is_available(self) -> bool:
        """Check if browser cookie extraction is available.

        Always returns True - we'll attempt extraction.
        """
        return True

    async def fetch(self) -> FetchResult:
        """Fetch by extracting browser cookies."""
        # Try to import browser cookie library
        try:
            import browser_cookie3
        except ImportError:
            try:
                import pycookiecheat as browser_cookie3
            except ImportError:
                return FetchResult.fail(
                    "Browser cookie extraction requires browser_cookie3 or pycookiecheat"
                )

        session_key = None

        # Try each browser
        for browser_name in ["safari", "chrome", "firefox", "brave", "edge"]:
            try:
                browser = getattr(browser_cookie3, browser_name, None)
                if browser is None:
                    continue

                cookies = browser(domain_name="claude.ai")
                for cookie in cookies:
                    if cookie.name in self.COOKIE_NAMES:
                        session_key = cookie.value
                        break
            except Exception:
                continue

            if session_key:
                break

        if not session_key:
            return FetchResult.fail("Could not extract session key from browser")

        # Save session key for future use
        self._save_session_key(session_key)

        # Now use the web strategy to fetch
        web_strategy = ClaudeWebStrategy()
        return await web_strategy.fetch()

    def _save_session_key(self, session_key: str) -> None:
        """Save session key to credential storage."""
        import json

        data = json.dumps({"session_key": session_key}).encode()
        write_credential(ClaudeWebStrategy.SESSION_PATH, data)
