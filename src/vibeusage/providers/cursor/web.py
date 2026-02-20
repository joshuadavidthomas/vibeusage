"""Web (session token) strategy for Cursor provider."""

from __future__ import annotations

import json
from datetime import UTC
from datetime import datetime
from decimal import Decimal

from vibeusage.config.credentials import credential_path
from vibeusage.config.credentials import read_credential
from vibeusage.core.http import get_http_client
from vibeusage.models import OverageUsage
from vibeusage.models import PeriodType
from vibeusage.models import ProviderIdentity
from vibeusage.models import UsagePeriod
from vibeusage.models import UsageSnapshot
from vibeusage.strategies.base import FetchResult
from vibeusage.strategies.base import FetchStrategy


class CursorWebStrategy(FetchStrategy):
    """Fetch Cursor usage using web session token."""

    name = "web"

    # API endpoints
    USAGE_URL = "https://www.cursor.com/api/usage-summary"
    USER_URL = "https://www.cursor.com/api/auth/me"

    # Cookie names to try (in priority order)
    COOKIE_NAMES = [
        "WorkosCursorSessionToken",
        "__Secure-next-auth.session-token",
        "next-auth.session-token",
    ]

    @property
    def SESSION_PATH(self):
        """Get session token file path."""
        return credential_path("cursor", "session")

    def is_available(self) -> bool:
        """Check if session token is available."""
        return self.SESSION_PATH.exists()

    async def fetch(self) -> FetchResult:
        """Fetch usage using session token."""
        session_token = self._load_session_token()
        if not session_token:
            return FetchResult.fail("No session token found")

        # Fetch both usage and user data
        async with get_http_client() as client:
            # Fetch usage data
            usage_response = await client.post(
                self.USAGE_URL,
                cookies={"__Secure-next-auth.session-token": session_token},
                headers={
                    "Content-Type": "application/json",
                    "User-Agent": "Mozilla/5.0",
                },
            )

            if usage_response.status_code == 401:
                return FetchResult.fatal("Session token expired or invalid")
            if usage_response.status_code == 404:
                return FetchResult.fail("User not found or no active subscription")
            if usage_response.status_code != 200:
                return FetchResult.fail(
                    f"Usage request failed: {usage_response.status_code}"
                )

            try:
                usage_data = usage_response.json()
            except json.JSONDecodeError:
                return FetchResult.fail("Invalid usage response")

            # Fetch user data for identity
            user_response = await client.get(
                self.USER_URL,
                cookies={"__Secure-next-auth.session-token": session_token},
                headers={"User-Agent": "Mozilla/5.0"},
            )

            user_data = None
            if user_response.status_code == 200:
                try:
                    user_data = user_response.json()
                except json.JSONDecodeError:
                    pass

        snapshot = self._parse_response(usage_data, user_data)
        if snapshot is None:
            return FetchResult.fail("Failed to parse usage response")

        return FetchResult.ok(snapshot)

    def _load_session_token(self) -> str | None:
        """Load session token from file."""
        content = read_credential(self.SESSION_PATH)
        if content:
            try:
                data = json.loads(content)
                # Try various keys that might store the token
                for key in ["session_token", "token", "session_key", "session"]:
                    if token := data.get(key):
                        return token
                # If no standard key found, return the raw string value
                return content.decode().strip()
            except json.JSONDecodeError:
                # Try as raw string
                return content.decode().strip()
        return None

    def _parse_response(
        self,
        usage_data: dict,
        user_data: dict | None,
    ) -> UsageSnapshot | None:
        """Parse Cursor usage API response.

        Args:
            usage_data: Response from /api/usage-summary
            user_data: Response from /api/auth/me (optional)

        Returns:
            UsageSnapshot or None if parsing fails
        """
        if not usage_data:
            return None

        periods = []
        overage = None
        identity = None

        # Parse premium requests (primary usage period)
        premium_requests = usage_data.get("premium_requests", {})
        if premium_requests:
            used = premium_requests.get("used", 0)
            available = premium_requests.get("available", 0)
            total = used + available

            if total > 0:
                utilization = int((used / total) * 100)
            else:
                utilization = 0

            # Get billing cycle end for reset time
            resets_at = None
            billing_cycle = usage_data.get("billing_cycle", {})
            if isinstance(billing_cycle, dict):
                end_date = billing_cycle.get("end")
                if end_date:
                    try:
                        # Try ISO format first
                        if isinstance(end_date, str):
                            resets_at = datetime.fromisoformat(
                                end_date.replace("Z", "+00:00")
                            )
                        elif isinstance(end_date, int):
                            # Unix timestamp in milliseconds
                            resets_at = datetime.fromtimestamp(end_date / 1000, tz=UTC)
                    except ValueError, TypeError:
                        pass

            periods.append(
                UsagePeriod(
                    name="Premium Requests",
                    utilization=utilization,
                    period_type=PeriodType.MONTHLY,
                    resets_at=resets_at,
                )
            )

        # Parse on-demand spend (overage)
        on_demand = usage_data.get("on_demand_spend", {})
        if isinstance(on_demand, dict):
            limit_cents = on_demand.get("limit_cents", 0)
            if limit_cents > 0:
                try:
                    overage = OverageUsage(
                        used=Decimal(on_demand.get("used_cents", 0)) / Decimal("100"),
                        limit=Decimal(limit_cents) / Decimal("100"),
                        currency="USD",
                        is_enabled=True,
                    )
                except ValueError, TypeError:
                    pass

        # Parse user identity
        if user_data and isinstance(user_data, dict):
            identity = ProviderIdentity(
                email=user_data.get("email"),
                organization=None,
                plan=user_data.get("membership_type"),
            )

        # If no periods were parsed, return None
        if not periods:
            return None

        return UsageSnapshot(
            provider="cursor",
            fetched_at=datetime.now(UTC),
            periods=tuple(periods),
            overage=overage,
            identity=identity,
            status=None,
            source="web",
        )


class CursorBrowserCookieStrategy(FetchStrategy):
    """Extract Cursor usage from browser cookies.

    This strategy attempts to extract the session token from browser cookies.
    """

    name = "browser"

    # Cookie domains and names to try
    COOKIE_DOMAINS = [".cursor.com", "cursor.com", ".cursor.sh", "cursor.sh"]
    COOKIE_NAMES = [
        "WorkosCursorSessionToken",
        "__Secure-next-auth.session-token",
        "next-auth.session-token",
    ]

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

        session_token = None

        # Try each browser
        for browser_name in ["safari", "chrome", "firefox", "brave", "edge", "arc"]:
            try:
                browser = getattr(browser_cookie3, browser_name, None)
                if browser is None:
                    continue

                # Try each domain
                for domain in self.COOKIE_DOMAINS:
                    try:
                        cookies = browser(domain_name=domain)
                        for cookie in cookies:
                            if cookie.name in self.COOKIE_NAMES:
                                session_token = cookie.value
                                break
                    except Exception:
                        continue

                    if session_token:
                        break

            except Exception:
                continue

            if session_token:
                break

        if not session_token:
            return FetchResult.fail("Could not extract session token from browser")

        # Save session token for future use
        self._save_session_token(session_token)

        # Now use the web strategy to fetch
        web_strategy = CursorWebStrategy()
        return await web_strategy.fetch()

    def _save_session_token(self, session_token: str) -> None:
        """Save session token to credential storage."""
        from vibeusage.config.credentials import write_credential

        path = credential_path("cursor", "session")
        data = json.dumps({"session_token": session_token}).encode()
        write_credential(path, data)
