"""Tests for Copilot (GitHub) provider."""

from __future__ import annotations

from datetime import UTC
from datetime import datetime
from datetime import timedelta
from pathlib import Path
from unittest.mock import AsyncMock
from unittest.mock import Mock
from unittest.mock import patch

import pytest

from vibeusage.models import PeriodType
from vibeusage.models import StatusLevel
from vibeusage.providers import CopilotProvider
from vibeusage.providers.copilot import CopilotDeviceFlowStrategy


class TestCopilotProvider:
    """Tests for CopilotProvider."""

    def test_metadata(self):
        """CopilotProvider has correct metadata."""
        assert CopilotProvider.metadata.id == "copilot"
        assert CopilotProvider.metadata.name == "Copilot"
        assert "GitHub" in CopilotProvider.metadata.description
        assert (
            CopilotProvider.metadata.homepage == "https://github.com/features/copilot"
        )
        assert CopilotProvider.metadata.status_url == "https://www.githubstatus.com"
        assert (
            CopilotProvider.metadata.dashboard_url
            == "https://github.com/settings/copilot"
        )

    def test_id_property(self):
        """id property returns correct value."""
        provider = CopilotProvider()
        assert provider.id == "copilot"

    def test_name_property(self):
        """name property returns correct value."""
        provider = CopilotProvider()
        assert provider.name == "Copilot"

    def test_fetch_strategies_returns_list(self):
        """fetch_strategies returns list of strategies."""
        provider = CopilotProvider()
        strategies = provider.fetch_strategies()

        assert isinstance(strategies, list)
        assert len(strategies) >= 1

    def test_fetch_strategy_has_device_flow(self):
        """Strategies include device flow."""
        provider = CopilotProvider()
        strategies = provider.fetch_strategies()

        assert any(isinstance(s, CopilotDeviceFlowStrategy) for s in strategies)

    def test_fetch_status_is_async(self):
        """fetch_status returns async operation."""
        provider = CopilotProvider()

        import inspect

        assert inspect.iscoroutinefunction(provider.fetch_status)


class TestCopilotDeviceFlowStrategy:
    """Tests for CopilotDeviceFlowStrategy."""

    def test_name_property(self):
        """Strategy has correct name."""
        strategy = CopilotDeviceFlowStrategy()
        assert strategy.name == "device_flow"

    def test_oauth_constants(self):
        """OAuth constants are defined correctly."""
        strategy = CopilotDeviceFlowStrategy()
        assert strategy.CLIENT_ID == "Iv1.b507a08c87ecfe98"
        assert "github.com" in strategy.DEVICE_CODE_URL
        assert "github.com" in strategy.TOKEN_URL
        assert strategy.SCOPE == "read:user"

    def test_credential_file_path(self):
        """Credential file path includes copilot."""
        strategy = CopilotDeviceFlowStrategy()
        assert "copilot" in str(strategy.CREDENTIAL_FILE).lower()

    def test_is_available_returns_false_when_no_credentials(self):
        """is_available returns False when no credentials exist."""
        strategy = CopilotDeviceFlowStrategy()

        with patch.object(strategy, "CREDENTIAL_FILE", Path("/nonexistent/path.json")):
            result = strategy.is_available()
            assert result is False

    def test_is_available_returns_true_when_credentials_exist(self):
        """is_available returns True when credentials exist."""
        import tempfile

        strategy = CopilotDeviceFlowStrategy()

        # Create a temporary file that exists
        with tempfile.NamedTemporaryFile(mode="w", delete=False) as f:
            temp_path = Path(f.name)

        try:
            with patch.object(strategy, "CREDENTIAL_FILE", temp_path):
                result = strategy.is_available()
                assert result is True
        finally:
            temp_path.unlink(missing_ok=True)

    @pytest.mark.asyncio
    async def test_fetch_fails_with_no_credentials(self):
        """fetch fails when no credentials found."""
        strategy = CopilotDeviceFlowStrategy()

        with patch.object(strategy, "_load_credentials", return_value=None):
            result = await strategy.fetch()
            assert result.success is False
            assert "No OAuth credentials found" in result.error
            assert result.should_fallback is False  # Fatal error

    @pytest.mark.asyncio
    async def test_fetch_fails_with_empty_credentials(self):
        """fetch fails when credentials is empty dict."""
        strategy = CopilotDeviceFlowStrategy()

        with patch.object(strategy, "_load_credentials", return_value={}):
            result = await strategy.fetch()
            assert result.success is False
            # Empty dict is truthy but has no access_token
            assert (
                "credentials" in result.error.lower()
                or "access_token" in result.error.lower()
            )

    @pytest.mark.asyncio
    async def test_fetch_success_with_premium_interactions(self):
        """fetch succeeds with valid credentials and premium interactions API response."""
        strategy = CopilotDeviceFlowStrategy()
        credentials = {
            "access_token": "test_token",
        }

        api_response = {
            "premium_interactions": {
                "total": 1000,
                "used": 450,
                "reset_at": "2026-01-23T00:00:00Z",
            },
            "chat_quotas": [
                {
                    "model": "gpt-4",
                    "limit": 30,
                    "used": 15,
                    "reset_at": "2026-01-23T00:00:00Z",
                }
            ],
            "billing_cycle": {
                "start": "2026-01-16T00:00:00Z",
                "end": "2026-02-16T00:00:00Z",
            },
        }

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch(
                "vibeusage.providers.copilot.device_flow.get_http_client"
            ) as mock_http:
                # Mock the HTTP response
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
        assert result.snapshot.provider == "copilot"
        assert result.snapshot.source == "device_flow"

    @pytest.mark.asyncio
    async def test_fetch_success_with_alternative_quota_format(self):
        """fetch succeeds with alternative quota format."""
        strategy = CopilotDeviceFlowStrategy()
        credentials = {
            "access_token": "test_token",
        }

        api_response = {
            "quota": {
                "total": 500,
                "used": 250,
            }
        }

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch(
                "vibeusage.providers.copilot.device_flow.get_http_client"
            ) as mock_http:
                # Mock the HTTP response
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
        assert len(result.snapshot.periods) == 1
        assert result.snapshot.periods[0].utilization == 50  # 250/500

    @pytest.mark.asyncio
    async def test_fetch_handles_401_error(self):
        """fetch handles 401 error correctly."""
        strategy = CopilotDeviceFlowStrategy()
        credentials = {"access_token": "expired_token"}

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch(
                "vibeusage.providers.copilot.device_flow.get_http_client"
            ) as mock_http:
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
        strategy = CopilotDeviceFlowStrategy()
        credentials = {"access_token": "unauthorized_token"}

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch(
                "vibeusage.providers.copilot.device_flow.get_http_client"
            ) as mock_http:
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

    @pytest.mark.asyncio
    async def test_fetch_handles_404_error(self):
        """fetch handles 404 error (no Copilot access)."""
        strategy = CopilotDeviceFlowStrategy()
        credentials = {"access_token": "valid_token"}

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch(
                "vibeusage.providers.copilot.device_flow.get_http_client"
            ) as mock_http:
                # Mock the HTTP response
                mock_response = AsyncMock()
                mock_response.status_code = 404

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
        assert "Copilot access" in result.error

    @pytest.mark.asyncio
    async def test_fetch_handles_invalid_json(self):
        """fetch handles invalid JSON response."""
        strategy = CopilotDeviceFlowStrategy()
        credentials = {"access_token": "test_token"}

        with patch.object(strategy, "_load_credentials", return_value=credentials):
            with patch(
                "vibeusage.providers.copilot.device_flow.get_http_client"
            ) as mock_http:
                # Mock the HTTP response - use Mock (not AsyncMock) for json since it's synchronous
                mock_response = Mock()
                mock_response.status_code = 200
                mock_response.json = Mock(side_effect=ValueError("Invalid JSON"))

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
        assert "Invalid response" in result.error

    def test_needs_refresh_false_when_no_expiry(self):
        """_needs_refresh returns False when no expiry in credentials."""
        strategy = CopilotDeviceFlowStrategy()
        credentials = {}

        result = strategy._needs_refresh(credentials)
        assert result is False

    def test_needs_refresh_false_when_expires_later(self):
        """_needs_refresh returns False when token expires well after threshold."""
        strategy = CopilotDeviceFlowStrategy()
        credentials = {
            "expires_at": (datetime.now(UTC) + timedelta(days=2)).isoformat(),
        }

        result = strategy._needs_refresh(credentials)
        assert (
            result is False
        )  # GitHub tokens don't expire, but we have a 1-day threshold

    def test_needs_refresh_true_when_expiring_soon(self):
        """_needs_refresh returns True when token expires within threshold."""
        strategy = CopilotDeviceFlowStrategy()
        credentials = {
            "expires_at": (datetime.now(UTC) + timedelta(hours=12)).isoformat(),
        }

        result = strategy._needs_refresh(credentials)
        assert result is True  # Within 1 day threshold

    @pytest.mark.asyncio
    async def test_refresh_token_verifies_valid_token(self):
        """_refresh_token verifies token is still valid."""
        strategy = CopilotDeviceFlowStrategy()
        credentials = {"access_token": "valid_token"}

        with patch(
            "vibeusage.providers.copilot.device_flow.get_http_client"
        ) as mock_http:
            # Mock the HTTP response for user verification
            mock_user_response = Mock()
            mock_user_response.status_code = 200

            # Mock the HTTP client
            mock_client = AsyncMock()
            mock_client.get = AsyncMock(return_value=mock_user_response)

            # Mock the async context manager
            mock_cm = AsyncMock()
            mock_cm.__aenter__.return_value = mock_client
            mock_cm.__aexit__.return_value = None

            mock_http.return_value = mock_cm

            result = await strategy._refresh_token(credentials)

        assert result is not None
        assert result["access_token"] == "valid_token"

    @pytest.mark.asyncio
    async def test_refresh_token_returns_none_for_expired_token(self):
        """_refresh_token returns None for expired/invalid token."""
        strategy = CopilotDeviceFlowStrategy()
        credentials = {"access_token": "expired_token"}

        with patch(
            "vibeusage.providers.copilot.device_flow.get_http_client"
        ) as mock_http:
            # Mock the HTTP response for user verification
            mock_user_response = AsyncMock()
            mock_user_response.status_code = 401

            # Mock the HTTP client
            mock_client = AsyncMock()
            mock_client.get = AsyncMock(return_value=mock_user_response)

            # Mock the async context manager
            mock_cm = AsyncMock()
            mock_cm.__aenter__.return_value = mock_client
            mock_cm.__aexit__.return_value = None

            mock_http.return_value = mock_cm

            result = await strategy._refresh_token(credentials)

        assert result is None

    def test_parse_usage_response_full(self):
        """_parse_usage_response correctly parses full API response."""
        strategy = CopilotDeviceFlowStrategy()

        data = {
            "premium_interactions": {
                "total": 1000,
                "used": 450,
                "reset_at": "2026-01-23T00:00:00Z",
            },
            "chat_quotas": [
                {
                    "model": "gpt-4",
                    "limit": 30,
                    "used": 15,
                    "reset_at": "2026-01-23T00:00:00Z",
                },
                {
                    "model": "gpt-3.5-turbo",
                    "limit": 100,
                    "used": 80,
                    "reset_at": "2026-01-23T00:00:00Z",
                },
            ],
            "account": {
                "plan": "pro",
                "organization": "Acme Corp",
                "email": "user@example.com",
            },
        }

        snapshot = strategy._parse_usage_response(data)

        assert snapshot is not None
        assert snapshot.provider == "copilot"
        assert len(snapshot.periods) == 3

        # Check monthly period
        monthly = next(
            (p for p in snapshot.periods if p.period_type == PeriodType.MONTHLY), None
        )
        assert monthly is not None
        assert monthly.name == "Monthly"
        assert monthly.utilization == 45  # 450/1000

        # Check daily chat quotas
        daily_gpt4 = next((p for p in snapshot.periods if p.model == "gpt-4"), None)
        assert daily_gpt4 is not None
        assert daily_gpt4.utilization == 50  # 15/30

        daily_gpt35 = next(
            (p for p in snapshot.periods if p.model == "gpt-3.5-turbo"), None
        )
        assert daily_gpt35 is not None
        assert daily_gpt35.utilization == 80  # 80/100

        # Check identity
        assert snapshot.identity is not None
        assert snapshot.identity.plan == "pro"
        assert snapshot.identity.organization == "Acme Corp"
        assert snapshot.identity.email == "user@example.com"

    def test_parse_usage_response_minimal(self):
        """_parse_usage_response handles minimal response."""
        strategy = CopilotDeviceFlowStrategy()

        data = {
            "premium_interactions": {
                "total": 500,
                "used": 250,
            }
        }

        snapshot = strategy._parse_usage_response(data)

        assert snapshot is not None
        assert len(snapshot.periods) == 1
        assert snapshot.periods[0].utilization == 50  # 250/500
        assert snapshot.periods[0].period_type == PeriodType.MONTHLY
        assert snapshot.identity is None

    def test_parse_usage_response_without_reset_at(self):
        """_parse_usage_response handles missing reset_at."""
        strategy = CopilotDeviceFlowStrategy()

        data = {
            "premium_interactions": {
                "total": 1000,
                "used": 600,
            },
            "chat_quotas": [
                {
                    "model": "gpt-4",
                    "limit": 30,
                    "used": 20,
                }
            ],
        }

        snapshot = strategy._parse_usage_response(data)

        assert snapshot is not None
        assert len(snapshot.periods) == 2
        # All periods should have no reset time
        for period in snapshot.periods:
            assert period.resets_at is None

    def test_parse_usage_response_returns_none_without_quota(self):
        """_parse_usage_response returns None when no quota data."""
        strategy = CopilotDeviceFlowStrategy()

        data = {"account": {"plan": "free"}}

        snapshot = strategy._parse_usage_response(data)
        assert snapshot is None


class TestCopilotProviderIntegration:
    """Integration tests for Copilot provider with registry."""

    def test_copilot_registered(self):
        """CopilotProvider is registered in the registry."""
        from vibeusage.providers import get_provider

        provider_cls = get_provider("copilot")
        assert provider_cls is not None
        assert provider_cls == CopilotProvider

    def test_create_copilot_provider(self):
        """Can create CopilotProvider instance via registry."""
        from vibeusage.providers import create_provider

        provider = create_provider("copilot")
        assert isinstance(provider, CopilotProvider)

    def test_list_includes_copilot(self):
        """list_provider_ids includes copilot."""
        from vibeusage.providers import list_provider_ids

        ids = list_provider_ids()
        assert "copilot" in ids


class TestCopilotStatus:
    """Tests for Copilot status fetching."""

    @pytest.mark.asyncio
    async def test_fetch_copilot_status_calls_statuspage(self):
        """fetch_copilot_status calls fetch_statuspage_status with correct URL."""
        from vibeusage.providers.copilot.status import fetch_copilot_status
        from vibeusage.models import ProviderStatus

        with patch(
            "vibeusage.providers.copilot.status.fetch_statuspage_status"
        ) as mock_fetch:
            mock_fetch.return_value = ProviderStatus(
                level=StatusLevel.OPERATIONAL,
                description="All systems operational",
                updated_at=datetime.now(UTC),
            )

            result = await fetch_copilot_status()

            assert result.level == StatusLevel.OPERATIONAL
            mock_fetch.assert_called_once_with(
                "https://www.githubstatus.com/api/v2/status.json"
            )

    @pytest.mark.asyncio
    async def test_fetch_copilot_status_propagates_degraded(self):
        """fetch_copilot_status propagates degraded status."""
        from vibeusage.providers.copilot.status import fetch_copilot_status
        from vibeusage.models import ProviderStatus

        with patch(
            "vibeusage.providers.copilot.status.fetch_statuspage_status"
        ) as mock_fetch:
            mock_fetch.return_value = ProviderStatus(
                level=StatusLevel.DEGRADED,
                description="Some systems degraded",
                updated_at=datetime.now(UTC),
            )

            result = await fetch_copilot_status()

            assert result.level == StatusLevel.DEGRADED
            assert result.description == "Some systems degraded"

    @pytest.mark.asyncio
    async def test_copilot_provider_fetch_status(self):
        """CopilotProvider.fetch_status calls fetch_copilot_status."""
        from vibeusage.models import ProviderStatus

        with patch(
            "vibeusage.providers.copilot.status.fetch_copilot_status"
        ) as mock_fetch:
            mock_fetch.return_value = ProviderStatus(
                level=StatusLevel.OPERATIONAL,
                description="All systems operational",
                updated_at=datetime.now(UTC),
            )

            provider = CopilotProvider()
            status = await provider.fetch_status()

            assert status.level == StatusLevel.OPERATIONAL
            mock_fetch.assert_called_once()
