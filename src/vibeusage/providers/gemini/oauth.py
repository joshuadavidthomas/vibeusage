"""OAuth strategy for Gemini (Google AI) provider."""

import json
from datetime import datetime, timezone, timedelta
from pathlib import Path

from vibeusage.config.credentials import read_credential, write_credential
from vibeusage.config.paths import config_dir
from vibeusage.core.http import get_http_client
from vibeusage.models import PeriodType, UsagePeriod, UsageSnapshot
from vibeusage.strategies.base import FetchResult, FetchStrategy


class GeminiOAuthStrategy(FetchStrategy):
    """Fetch Gemini usage using OAuth tokens.

    Uses the Gemini CLI OAuth credentials to access usage data from
    Google Cloud Code API which provides per-model quota information.
    """

    name = "oauth"

    # OAuth endpoints
    TOKEN_URL = "https://oauth2.googleapis.com/token"
    QUOTA_URL = "https://cloudcode-pa.googleapis.com/v1internal:retrieveUserQuota"
    TIER_URL = "https://cloudcode-pa.googleapis.com/v1internal:loadCodeAssist"

    # Refresh threshold: 5 minutes before expiry
    REFRESH_THRESHOLD_MINUTES = 5

    # Credential locations (check vibeusage first, then Gemini CLI)
    CREDENTIAL_PATHS = [
        config_dir() / "credentials" / "gemini" / "oauth.json",
        Path.home() / ".gemini" / "oauth_creds.json",
    ]

    # Google OAuth client credentials for Gemini CLI
    # These are extracted from the Gemini CLI installation
    CLIENT_ID = "77185425430.apps.googleusercontent.com"
    CLIENT_SECRET = "GOCSPX-1mdrl61JR9D-iFHq4QPq2mJGwZv"

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

        # Fetch quota and user data
        quota_data, user_data = await self._fetch_quota_data(access_token)

        if not quota_data:
            return FetchResult.fail("Failed to fetch quota data")

        snapshot = self._parse_usage_response(quota_data, user_data)
        if snapshot is None:
            return FetchResult.fail("Failed to parse usage response")

        return FetchResult.ok(snapshot)

    def _load_credentials(self) -> dict | None:
        """Load OAuth credentials from file.

        Handles multiple formats:
        1. Gemini CLI format: {"installed": {"token": "..."}}
        2. Vibeusage format: {"access_token": "...", ...}
        3. Standard OAuth format: {"token": "...", "refresh_token": "..."}
        """
        for path in self.CREDENTIAL_PATHS:
            content = read_credential(path)
            if content:
                try:
                    data = json.loads(content)
                except json.JSONDecodeError:
                    continue

                # Handle Gemini CLI format with nested "installed" key
                if "installed" in data and isinstance(data["installed"], dict):
                    cli_data = data["installed"]
                    # Extract token and convert to standard format
                    return self._convert_gemini_cli_format(cli_data)

                # Handle nested "token" format (some Gemini CLI versions)
                # Convert "token" key to "access_token" if present
                if "token" in data and "access_token" not in data:
                    return {
                        "access_token": data.get("token"),
                        "refresh_token": data.get("refresh_token"),
                        "expires_at": data.get("expiry_date") or data.get("expires_at"),
                    }

                # Already in standard format
                if "access_token" in data:
                    return data

        return None

    def _convert_gemini_cli_format(self, data: dict) -> dict | None:
        """Convert Gemini CLI credential format to standard OAuth format.

        Gemini CLI uses different key names:
        - token -> access_token
        - refresh_token -> refresh_token (same)
        - expiry_date -> expires_at
        """
        if not data:
            return None

        access_token = data.get("token") or data.get("access_token")
        if not access_token:
            return None

        result = {
            "access_token": access_token,
            "refresh_token": data.get("refresh_token"),
        }

        # Handle expiry date format
        expiry = data.get("expiry_date") or data.get("expires_at")
        if expiry:
            if isinstance(expiry, (int, float)):
                # Convert millisecond timestamp to ISO string
                result["expires_at"] = datetime.fromtimestamp(
                    expiry / 1000, tz=timezone.utc
                ).isoformat()
            elif isinstance(expiry, str):
                # Already a string, might be ISO or other format
                try:
                    # Try parsing as ISO format
                    datetime.fromisoformat(expiry)
                    result["expires_at"] = expiry
                except ValueError:
                    # Try parsing as timestamp number in string
                    try:
                        ts = int(expiry)
                        result["expires_at"] = datetime.fromtimestamp(
                            ts / 1000, tz=timezone.utc
                        ).isoformat()
                    except (ValueError, OSError):
                        pass

        return result

    def _needs_refresh(self, credentials: dict) -> bool:
        """Check if token needs refresh."""
        expires_at = credentials.get("expires_at")
        if not expires_at:
            return False

        try:
            expiry = datetime.fromisoformat(expires_at)
            # Refresh if expires within REFRESH_THRESHOLD_MINUTES
            threshold = datetime.now(timezone.utc) + timedelta(
                minutes=self.REFRESH_THRESHOLD_MINUTES
            )
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
                    "client_id": self.CLIENT_ID,
                    "client_secret": self.CLIENT_SECRET,
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
            expires_at = datetime.now(timezone.utc) + timedelta(
                seconds=data["expires_in"]
            )
            data["expires_at"] = expires_at.isoformat()

        # Preserve refresh_token if not in response
        if "refresh_token" not in data:
            data["refresh_token"] = refresh_token

        # Save updated credentials
        self._save_credentials(data)

        return data

    def _save_credentials(self, credentials: dict) -> None:
        """Save credentials to vibeusage storage."""
        path = self.CREDENTIAL_PATHS[0]
        content = json.dumps(credentials).encode()
        write_credential(path, content)

    async def _fetch_quota_data(
        self, access_token: str
    ) -> tuple[dict | None, dict | None]:
        """Fetch quota and user tier data from Gemini API.

        Returns:
            Tuple of (quota_data, user_data)
        """
        quota_data = None
        user_data = None

        async with get_http_client() as client:
            # Fetch quota data
            try:
                quota_response = await client.post(
                    self.QUOTA_URL,
                    headers={
                        "Authorization": f"Bearer {access_token}",
                    },
                    json={},
                )

                if quota_response.status_code == 200:
                    quota_data = quota_response.json()
            except Exception:
                pass

            # Fetch user tier data (optional)
            try:
                tier_response = await client.post(
                    self.TIER_URL,
                    headers={
                        "Authorization": f"Bearer {access_token}",
                    },
                    json={},
                )

                if tier_response.status_code == 200:
                    user_data = tier_response.json()
            except Exception:
                pass

        return quota_data, user_data

    def _parse_usage_response(
        self, quota_data: dict, user_data: dict | None
    ) -> UsageSnapshot | None:
        """Parse usage response from Gemini API.

        API format:
        {
            "quota_buckets": [
                {
                    "model_id": "models/gemini-1.5-flash",
                    "remaining_fraction": 0.75,
                    "reset_time": "2026-01-17T00:00:00Z"
                },
                ...
            ]
        }

        User tier format:
        {
            "user_tier": "free" | "paid",
            ...
        }
        """
        periods = []

        # Parse quota buckets
        quota_buckets = quota_data.get("quota_buckets", []) if quota_data else []

        for bucket in quota_buckets:
            remaining_fraction = bucket.get("remaining_fraction", 1.0)
            utilization = int((1 - remaining_fraction) * 100)
            model_id = bucket.get("model_id", "unknown")

            # Extract model name from ID (e.g., "models/gemini-1.5-flash" -> "gemini-1.5-flash")
            if "/" in model_id:
                model_name = model_id.split("/")[-1]
            else:
                model_name = model_id

            # Parse reset time
            resets_at = None
            reset_time_str = bucket.get("reset_time")
            if reset_time_str:
                try:
                    # Try ISO format first
                    resets_at = datetime.fromisoformat(reset_time_str)
                    if resets_at.tzinfo is None:
                        resets_at = resets_at.replace(tzinfo=timezone.utc)
                except (ValueError, TypeError):
                    # Try Unix timestamp
                    try:
                        resets_at = datetime.fromtimestamp(
                            float(reset_time_str), tz=timezone.utc
                        )
                    except (ValueError, OSError, TypeError):
                        pass

            # Create readable name (title case)
            display_name = model_name.replace("-", " ").replace("_", " ").title()

            periods.append(
                UsagePeriod(
                    name=display_name,
                    utilization=utilization,
                    period_type=PeriodType.DAILY,  # Gemini uses daily quotas
                    resets_at=resets_at,
                    model=model_name,
                )
            )

        # If no periods found, create a default one
        if not periods:
            periods.append(
                UsagePeriod(
                    name="Daily",
                    utilization=0,
                    period_type=PeriodType.DAILY,
                    resets_at=self._next_midnight_utc(),
                )
            )

        # Parse user tier for identity
        from vibeusage.models import ProviderIdentity

        identity = None
        if user_data:
            tier = user_data.get("user_tier", "unknown")
            identity = ProviderIdentity(plan=tier)

        return UsageSnapshot(
            provider="gemini",
            fetched_at=datetime.now(timezone.utc),
            periods=tuple(periods),
            overage=None,  # No overage info from Gemini API
            identity=identity,
            status=None,
            source="oauth",
        )

    def _next_midnight_utc(self) -> datetime:
        """Calculate next midnight UTC for daily reset."""
        now = datetime.now(timezone.utc)
        tomorrow = now.replace(hour=0, minute=0, second=0, microsecond=0) + timedelta(
            days=1
        )
        return tomorrow
