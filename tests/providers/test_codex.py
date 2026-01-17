"""Tests for Codex (OpenAI) provider."""

from datetime import datetime, timezone, timedelta
from unittest.mock import AsyncMock, MagicMock, patch
from pathlib import Path

import pytest

from vibeusage.models import StatusLevel, PeriodType, UsagePeriod, UsageSnapshot
from vibeusage.providers.codex import CodexProvider
from vibeusage.providers.codex.oauth import CodexOAuthStrategy


class TestCodexProvider:
    """Tests for CodexProvider."""

    def test_metadata(self):
        """CodexProvider has correct metadata."""
        assert CodexProvider.metadata.id == "codex"
        assert CodexProvider.metadata.name == "Codex"
        assert "OpenAI" in CodexProvider.metadata.description
        assert CodexProvider.metadata.homepage == "https://chatgpt.com"
        assert CodexProvider.metadata.status_url == "https://status.openai.com"
        assert CodexProvider.metadata.dashboard_url == "https://chatgpt.com/codex/settings/usage"

    def test_id_property(self):
        """id property returns correct value."""
        provider = CodexProvider()
        assert provider.id == "codex"

    def test_name_property(self):
        """name property returns correct value."""
        provider = CodexProvider()
        assert provider.name == "Codex"

    def test_fetch_strategies_returns_list(self):
        """fetch_strategies returns list of strategies."""
        provider = CodexProvider()
        strategies = provider.fetch_strategies()

        assert isinstance(strategies, list)
        assert len(strategies) >= 1

    def test_fetch_strategy_has_oauth(self):
        """Strategies include OAuth."""
        provider = CodexProvider()
        strategies = provider.fetch_strategies()

        assert any(isinstance(s, CodexOAuthStrategy) for s in strategies)

    def test_fetch_status_is_async(self):
        """fetch_status returns async operation."""
        provider = CodexProvider()

        import inspect
        assert inspect.iscoroutinefunction(provider.fetch_status)


class TestCodexOAuthStrategy:
    """Tests for CodexOAuthStrategy."""

    def test_name_property(self):
        """Strategy has correct name."""
        strategy = CodexOAuthStrategy()
        assert strategy.name == "oauth"

    def test_credential_paths(self):
        """Credential paths are defined correctly."""
        strategy = CodexOAuthStrategy()
        assert len(strategy.CREDENTIAL_PATHS) >= 2
        assert "codex" in str(strategy.CREDENTIAL_PATHS[0]).lower()

    def test_is_available_returns_false_when_no_credentials(self):
        """is_available returns False when no credentials exist."""
        strategy = CodexOAuthStrategy()

        with patch.object(strategy, "CREDENTIAL_PATHS", [Path("/nonexistent/path.json")]):
            result = strategy.is_available()
            assert result is False

    def test_is_available_returns_true_when_credentials_exist(self):
        """is_available returns True when credentials exist."""
        strategy = CodexOAuthStrategy()

        with patch("vibeusage.providers.codex.oauth.Path.exists") as mock_exists:
            mock_exists.return_value = True
            result = strategy.is_available()
            assert result is True

    @pytest.mark.asyncio
    async def test_fetch_fails_with_no_credentials(self):
        """fetch fails when no credentials found."""
        strategy = CodexOAuthStrategy()

        with patch.object(strategy, "_load_credentials", return_value=None):
            result = await strategy.fetch()
            assert result.success is False
            assert "No OAuth credentials found" in result.error

    @pytest.mark.asyncio
    async def test_fetch_fails_with_empty_credentials(self):
        """fetch fails when credentials is empty dict."""
        strategy = CodexOAuthStrategy()

        with patch.object(strategy, "_load_credentials", return_value={}):
            result = await strategy.fetch()
            assert result.success is False
            assert "No OAuth credentials found" in result.error

    @pytest.mark.asyncio
    async def test_fetch_success(self):
        """fetch succeeds with valid credentials and API response."""
        strategy = CodexOAuthStrategy()
        credentials = {
            "access_token": "test_token",
            "refresh_token": "test_refresh",
            "expires_at": (datetime.now(timezone.utc) + timedelta(days=30)).isoformat(),
        }

        api_response = {
            "rate_limits": {
                "primary": {
                    "used_percent": 58,
                    "reset_timestamp": 1737000000,
                },
                "secondary": {
                    "used_percent": 23,
                    "reset_timestamp": 1737000000,
                },
            },
            "credits": {
                "has_credits": True,
                "balance": 10.50,
            },
            "plan_type": "plus",
        }

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch("vibeusage.providers.codex.oauth.get_http_client") as mock_http:
                from unittest.mock import Mock

                # Mock the HTTP response (httpx.Response.json() is synchronous)
                mock_response = Mock()
                mock_response.status_code = 200
                mock_response.json = Mock(return_value=api_response)

                # Mock the HTTP client
                mock_client = AsyncMock()
                mock_client.get = AsyncMock(return_value=mock_response)

                # Mock the async context manager
                mock_cm = AsyncMock()
                mock_cm.__aenter__.return_value = mock_client
                mock_cm.__aexit__.return_value = None

                mock_http.return_value = mock_cm

                result = await strategy.fetch()

        assert result.success is True
        assert result.snapshot is not None
        assert result.snapshot.provider == "codex"
        assert result.snapshot.source == "oauth"

    @pytest.mark.asyncio
    async def test_fetch_handles_401_error(self):
        """fetch handles 401 error correctly."""
        strategy = CodexOAuthStrategy()
        credentials = {
            "access_token": "expired_token",
            "refresh_token": "test_refresh",
            "expires_at": (datetime.now(timezone.utc) + timedelta(days=30)).isoformat(),
        }

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch("vibeusage.providers.codex.oauth.get_http_client") as mock_http:
                # Mock the HTTP response
                mock_response = AsyncMock()
                mock_response.status_code = 401

                # Mock the HTTP client
                mock_client = AsyncMock()
                mock_client.get = AsyncMock(return_value=mock_response)

                # Mock the async context manager
                mock_cm = AsyncMock()
                mock_cm.__aenter__.return_value = mock_client
                mock_cm.__aexit__.return_value = None

                mock_http.return_value = mock_cm

                result = await strategy.fetch()

        assert result.success is False
        assert "expired or invalid" in result.error
        assert result.should_fallback is False  # 401 is fatal

    @pytest.mark.asyncio
    async def test_fetch_handles_403_error(self):
        """fetch handles 403 error correctly."""
        strategy = CodexOAuthStrategy()
        credentials = {
            "access_token": "unauthorized_token",
            "refresh_token": "test_refresh",
            "expires_at": (datetime.now(timezone.utc) + timedelta(days=30)).isoformat(),
        }

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch("vibeusage.providers.codex.oauth.get_http_client") as mock_http:
                # Mock the HTTP response
                mock_response = AsyncMock()
                mock_response.status_code = 403

                # Mock the HTTP client
                mock_client = AsyncMock()
                mock_client.get = AsyncMock(return_value=mock_response)

                # Mock the async context manager
                mock_cm = AsyncMock()
                mock_cm.__aenter__.return_value = mock_client
                mock_cm.__aexit__.return_value = None

                mock_http.return_value = mock_cm

                result = await strategy.fetch()

        assert result.success is False
        assert "Not authorized" in result.error

    def test_needs_refresh_true_when_expiring_soon(self):
        """_needs_refresh returns True when token expires within threshold."""
        strategy = CodexOAuthStrategy()
        credentials = {
            "expires_at": (datetime.now(timezone.utc) + timedelta(days=5)).isoformat(),
        }

        result = strategy._needs_refresh(credentials)
        assert result is True  # 5 days < 8 day threshold

    def test_needs_refresh_false_when_expires_later(self):
        """_needs_refresh returns False when token expires well after threshold."""
        strategy = CodexOAuthStrategy()
        credentials = {
            "expires_at": (datetime.now(timezone.utc) + timedelta(days=15)).isoformat(),
        }

        result = strategy._needs_refresh(credentials)
        assert result is False  # 15 days > 8 day threshold

    def test_needs_refresh_false_when_no_expiry(self):
        """_needs_refresh returns False when no expiry in credentials."""
        strategy = CodexOAuthStrategy()
        credentials = {}

        result = strategy._needs_refresh(credentials)
        assert result is False

    def test_parse_usage_response(self):
        """_parse_usage_response correctly parses API response."""
        strategy = CodexOAuthStrategy()

        data = {
            "rate_limits": {
                "primary": {
                    "used_percent": 58,
                    "reset_timestamp": 1737000000,
                },
                "secondary": {
                    "used_percent": 23,
                    "reset_timestamp": 1737086400,
                },
            },
            "credits": {
                "has_credits": True,
                "balance": 10.50,
            },
            "plan_type": "plus",
        }

        snapshot = strategy._parse_usage_response(data)

        assert snapshot is not None
        assert snapshot.provider == "codex"
        assert len(snapshot.periods) == 2

        # Check primary (session) period
        primary = snapshot.periods[0]
        assert primary.name == "Session"
        assert primary.utilization == 58
        assert primary.period_type == PeriodType.SESSION

        # Check secondary (weekly) period
        secondary = snapshot.periods[1]
        assert secondary.name == "Weekly"
        assert secondary.utilization == 23
        assert secondary.period_type == PeriodType.WEEKLY

        # Check overage
        assert snapshot.overage is not None
        assert snapshot.overage.currency == "credits"
        assert snapshot.overage.limit == 10.50

        # Check identity
        assert snapshot.identity is not None
        assert snapshot.identity.plan == "plus"

    def test_parse_usage_response_minimal(self):
        """_parse_usage_response handles minimal response."""
        strategy = CodexOAuthStrategy()

        data = {
            "rate_limits": {
                "primary": {
                    "used_percent": 75,
                },
            },
        }

        snapshot = strategy._parse_usage_response(data)

        assert snapshot is not None
        assert len(snapshot.periods) == 1
        assert snapshot.periods[0].utilization == 75
        assert snapshot.overage is None

    def test_parse_usage_response_returns_none_without_limits(self):
        """_parse_usage_response returns None when no rate_limits."""
        strategy = CodexOAuthStrategy()

        data = {"credits": {"has_credits": True}}

        snapshot = strategy._parse_usage_response(data)
        assert snapshot is None


class TestCodexProviderIntegration:
    """Integration tests for Codex provider with registry."""

    def test_codex_registered(self):
        """CodexProvider is registered in the registry."""
        from vibeusage.providers import get_provider

        provider_cls = get_provider("codex")
        assert provider_cls is not None
        assert provider_cls == CodexProvider

    def test_create_codex_provider(self):
        """Can create CodexProvider instance via registry."""
        from vibeusage.providers import create_provider

        provider = create_provider("codex")
        assert isinstance(provider, CodexProvider)

    def test_list_includes_codex(self):
        """list_provider_ids includes codex."""
        from vibeusage.providers import list_provider_ids

        ids = list_provider_ids()
        assert "codex" in ids

    @pytest.mark.asyncio
    async def test_codex_status_fetch(self):
        """Codex provider fetches status from OpenAI status page."""
        with patch("vibeusage.providers.claude.status.fetch_statuspage_status") as mock_fetch:
            from vibeusage.models import ProviderStatus

            mock_fetch.return_value = ProviderStatus(
                level=StatusLevel.OPERATIONAL,
                description="All systems operational",
                updated_at=datetime.now(timezone.utc),
            )

            provider = CodexProvider()
            status = await provider.fetch_status()

            assert status.level == StatusLevel.OPERATIONAL
            mock_fetch.assert_called_once_with("https://status.openai.com/api/v2/status.json")
