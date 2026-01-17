"""API key strategy for Gemini (Google AI) provider."""
from __future__ import annotations

import json
import os
from datetime import datetime
from datetime import timezone

import httpx

from vibeusage.config.credentials import read_credential
from vibeusage.config.paths import config_dir
from vibeusage.core.http import get_http_client
from vibeusage.models import PeriodType
from vibeusage.models import UsagePeriod
from vibeusage.models import UsageSnapshot
from vibeusage.strategies.base import FetchResult
from vibeusage.strategies.base import FetchStrategy


class GeminiApiKeyStrategy(FetchStrategy):
    """Fetch Gemini usage using an API key.

    Google AI API uses API keys for authentication. Usage is tracked through
    Google Cloud Platform quotas rather than a simple usage endpoint.

    This strategy attempts to:
    1. Load API key from credentials or environment variable
    2. Make a test request to validate the key
    3. Return basic quota information if available

    Note: Google doesn't provide a simple usage API like other providers.
    Full usage tracking requires Google Cloud Platform monitoring APIs which
    need OAuth authentication and project-specific setup.
    """

    name = "api_key"

    # API endpoints
    API_BASE = "https://generativelanguage.googleapis.com/v1beta"
    MODELS_ENDPOINT = "models"
    COUNT_TOKENS_ENDPOINT = "models/{model}:countTokens"

    # Credential locations (check vibeusage first, then environment)
    CREDENTIAL_PATHS = [
        config_dir() / "credentials" / "gemini" / "api_key.txt",
        config_dir() / "credentials" / "gemini" / "api_key.json",
    ]

    # Environment variable name
    ENV_VAR = "GEMINI_API_KEY"

    # Default model for testing
    DEFAULT_MODEL = "gemini-1.5-flash"

    # Rate limit defaults (based on Google AI free tier)
    # These are conservative defaults; actual limits vary by model and tier
    DEFAULT_DAILY_LIMIT = 1500  # requests per day for free tier
    DEFAULT_RPM_LIMIT = 15  # requests per minute for free tier

    def is_available(self) -> bool:
        """Check if API key credentials are available."""
        # Check credential files
        for path in self.CREDENTIAL_PATHS:
            if path.exists():
                return True

        # Check environment variable
        return os.environ.get(self.ENV_VAR) is not None

    async def fetch(self) -> FetchResult:
        """Fetch usage using API key."""
        api_key = self._load_api_key()
        if not api_key:
            return FetchResult.fail(
                "No API key found. Set GEMINI_API_KEY or use 'vibeusage key set gemini'"
            )

        # Validate the API key by fetching models
        async with get_http_client() as client:
            # Try to list models to validate the key
            response = await self._fetch_models(client, api_key)

            if response.status_code == 401:
                return FetchResult.fail(
                    "API key is invalid or expired", should_fallback=False
                )
            if response.status_code == 403:
                return FetchResult.fail(
                    "API key does not have access to Generative Language API",
                    should_fallback=False,
                )
            if response.status_code == 429:
                return FetchResult.fail(
                    "Rate limit exceeded. Please try again later.",
                    should_fallback=False,
                )
            if response.status_code != 200:
                return FetchResult.fail(
                    f"Failed to validate API key: {response.status_code}",
                    should_fallback=False,
                )

        # Parse response to get model info
        snapshot = self._parse_models_response(
            response.json() if response.content else {}, api_key
        )
        if snapshot is None:
            return FetchResult.fail("Failed to parse API response")

        return FetchResult.ok(snapshot)

    def _load_api_key(self) -> str | None:
        """Load API key from credential files or environment."""
        # Check environment variable first
        if api_key := os.environ.get(self.ENV_VAR):
            return api_key

        # Check credential files
        for path in self.CREDENTIAL_PATHS:
            content = read_credential(path)
            if content:
                # Try JSON format first
                if path.suffix == ".json":
                    try:
                        data = json.loads(content)
                        if isinstance(data, dict):
                            return data.get("api_key") or data.get("key")
                    except json.JSONDecodeError:
                        pass
                # Treat as plain text API key
                return content.strip()

        return None

    async def _fetch_models(
        self, client: httpx.AsyncClient, api_key: str
    ) -> httpx.Response:
        """Fetch available models to validate API key."""
        url = f"{self.API_BASE}/{self.MODELS_ENDPOINT}"
        params = {"key": api_key}
        return await client.get(url, params=params)

    def _parse_models_response(self, data: dict, api_key: str) -> UsageSnapshot | None:
        """Parse models response to extract usage information.

        The Google AI API doesn't provide usage/quota info in the models response.
        We return a snapshot with default limits and note that detailed tracking
        requires Google Cloud Platform monitoring.

        Models response format:
        {
            "models": [
                {
                    "name": "models/gemini-1.5-flash",
                    "version": "001",
                    "displayName": "Gemini 1.5 Flash",
                    "description": "...",
                    "inputTokenLimit": 1000000,
                    "outputTokenLimit": 8000,
                    ...
                },
                ...
            ]
        }
        """
        periods = []

        # Add a daily period with default limit
        # Note: This is a placeholder - actual usage requires GCP monitoring
        periods.append(
            UsagePeriod(
                name="Daily",
                utilization=0,  # Unknown - would require GCP monitoring
                period_type=PeriodType.DAILY,
                resets_at=self._next_midnight_utc(),
            )
        )

        # Parse quota info from model limits if available
        model_info = self._extract_model_info(data)

        # Identity with model info
        from vibeusage.models import ProviderIdentity

        identity = ProviderIdentity(
            plan="API Key",
            organization=f"Available models: {model_info['count']}",
        )

        return UsageSnapshot(
            provider="gemini",
            fetched_at=datetime.now(timezone.utc),
            periods=tuple(periods),
            overage=None,  # No overage info from simple API key
            identity=identity,
            status=None,
            source="api_key",
        )

    def _extract_model_info(self, data: dict) -> dict:
        """Extract information about available models."""
        models = data.get("models", [])
        model_names = []

        for model in models:
            name = model.get("name", "")
            if name.startswith("models/"):
                name = name[7:]  # Remove "models/" prefix
            model_names.append(name)

        return {
            "count": len(models),
            "models": sorted(model_names),
        }

    def _next_midnight_utc(self) -> datetime:
        """Calculate next midnight UTC for daily reset."""
        from datetime import timedelta

        now = datetime.now(timezone.utc)
        tomorrow = now.replace(hour=0, minute=0, second=0, microsecond=0) + timedelta(
            days=1
        )
        return tomorrow
