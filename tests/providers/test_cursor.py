"""Tests for Cursor provider."""

import json
from datetime import datetime, timezone
from decimal import Decimal
from unittest.mock import AsyncMock, Mock, patch

import pytest

from vibeusage.models import OverageUsage, PeriodType, UsagePeriod, UsageSnapshot
from vibeusage.providers import CursorProvider, create_provider, get_provider, list_provider_ids
from vibeusage.providers.cursor import CursorWebStrategy


class TestCursorProvider:
    """Tests for CursorProvider."""

    def test_metadata(self):
        """CursorProvider has correct metadata."""
        assert CursorProvider.metadata.id == "cursor"
        assert CursorProvider.metadata.name == "Cursor"
        assert "AI" in CursorProvider.metadata.description or "code editor" in CursorProvider.metadata.description
        assert "cursor.com" in CursorProvider.metadata.homepage
        assert CursorProvider.metadata.status_url == "https://status.cursor.com"
        assert CursorProvider.metadata.dashboard_url == "https://cursor.com/settings/usage"

    def test_id_property(self):
        """id property returns correct value."""
        provider = CursorProvider()
        assert provider.id == "cursor"

    def test_name_property(self):
        """name property returns correct value."""
        provider = CursorProvider()
        assert provider.name == "Cursor"

    def test_fetch_strategies_returns_list(self):
        """fetch_strategies returns list of strategies."""
        provider = CursorProvider()
        strategies = provider.fetch_strategies()

        assert isinstance(strategies, list)
        assert len(strategies) >= 1

    def test_fetch_strategy_has_web(self):
        """Strategies include web strategy."""
        provider = CursorProvider()
        strategies = provider.fetch_strategies()

        assert any(isinstance(s, CursorWebStrategy) for s in strategies)

    def test_fetch_status_is_async(self):
        """fetch_status returns async operation."""
        provider = CursorProvider()

        import inspect
        assert inspect.iscoroutinefunction(provider.fetch_status)


class TestCursorWebStrategy:
    """Tests for CursorWebStrategy."""

    def test_name_property(self):
        """Strategy has correct name."""
        strategy = CursorWebStrategy()
        assert strategy.name == "web"

    def test_usage_url(self):
        """USAGE_URL is defined correctly."""
        strategy = CursorWebStrategy()
        assert "cursor.com" in strategy.USAGE_URL
        assert "/api/" in strategy.USAGE_URL

    def test_user_url(self):
        """USER_URL is defined correctly."""
        strategy = CursorWebStrategy()
        assert "cursor.com" in strategy.USER_URL
        assert "/api/" in strategy.USER_URL

    def test_session_path(self):
        """SESSION_PATH includes cursor."""
        strategy = CursorWebStrategy()
        assert "cursor" in str(strategy.SESSION_PATH).lower()

    def test_cookie_names(self):
        """COOKIE_NAMES includes expected values."""
        strategy = CursorWebStrategy()
        assert "WorkosCursorSessionToken" in strategy.COOKIE_NAMES
        assert "session-token" in str(strategy.COOKIE_NAMES)

    def test_is_available_returns_false_when_no_session(self):
        """is_available returns False when no session file exists."""
        strategy = CursorWebStrategy()

        mock_path = Mock()
        mock_path.exists.return_value = False

        with patch("vibeusage.providers.cursor.web.credential_path", return_value=mock_path):
            result = strategy.is_available()
            assert result is False

    def test_is_available_returns_true_when_session_exists(self):
        """is_available returns True when session file exists."""
        strategy = CursorWebStrategy()

        mock_path = Mock()
        mock_path.exists.return_value = True

        with patch("vibeusage.providers.cursor.web.credential_path", return_value=mock_path):
            result = strategy.is_available()
            assert result is True

    def test_load_session_token_from_json(self):
        """_load_session_token extracts token from JSON."""
        strategy = CursorWebStrategy()
        mock_content = b'{"session_token": "test_token_123"}'

        with patch("vibeusage.providers.cursor.web.read_credential", return_value=mock_content):
            token = strategy._load_session_token()
            assert token == "test_token_123"

    def test_load_session_token_alternative_keys(self):
        """_load_session_token tries various token keys."""
        strategy = CursorWebStrategy()

        # Try 'token' key
        mock_content = b'{"token": "alt_token_456"}'
        with patch("vibeusage.providers.cursor.web.read_credential", return_value=mock_content):
            token = strategy._load_session_token()
            assert token == "alt_token_456"

    def test_load_session_token_raw_string(self):
        """_load_session_token handles raw string content."""
        strategy = CursorWebStrategy()
        mock_content = b'raw_session_token_789'

        with patch("vibeusage.providers.cursor.web.read_credential", return_value=mock_content):
            token = strategy._load_session_token()
            assert token == "raw_session_token_789"

    def test_load_session_token_returns_none_when_no_content(self):
        """_load_session_token returns None when no content."""
        strategy = CursorWebStrategy()

        with patch("vibeusage.providers.cursor.web.read_credential", return_value=None):
            token = strategy._load_session_token()
            assert token is None

    @pytest.mark.asyncio
    async def test_fetch_fails_with_no_session_token(self):
        """fetch fails when no session token found."""
        strategy = CursorWebStrategy()

        with patch.object(strategy, "_load_session_token", return_value=None):
            result = await strategy.fetch()
            assert result.success is False
            assert "session token" in result.error.lower()

    @pytest.mark.asyncio
    async def test_fetch_handles_401_error(self):
        """fetch handles 401 error (expired session)."""
        strategy = CursorWebStrategy()

        with patch.object(strategy, "_load_session_token", return_value="expired_token"):
            with patch("vibeusage.providers.cursor.web.get_http_client") as mock_http:
                mock_response = AsyncMock()
                mock_response.status_code = 401

                mock_client = AsyncMock()
                mock_client.post = AsyncMock(return_value=mock_response)

                mock_cm = AsyncMock()
                mock_cm.__aenter__.return_value = mock_client
                mock_cm.__aexit__.return_value = None

                mock_http.return_value = mock_cm

                result = await strategy.fetch()

        assert result.success is False
        assert "expired" in result.error.lower() or "invalid" in result.error.lower()
        assert result.should_fallback is False  # Fatal error

    @pytest.mark.asyncio
    async def test_fetch_handles_404_error(self):
        """fetch handles 404 error (user not found)."""
        strategy = CursorWebStrategy()

        with patch.object(strategy, "_load_session_token", return_value="test_token"):
            with patch("vibeusage.providers.cursor.web.get_http_client") as mock_http:
                mock_response = AsyncMock()
                mock_response.status_code = 404

                mock_client = AsyncMock()
                mock_client.post = AsyncMock(return_value=mock_response)

                mock_cm = AsyncMock()
                mock_cm.__aenter__.return_value = mock_client
                mock_cm.__aexit__.return_value = None

                mock_http.return_value = mock_cm

                result = await strategy.fetch()

        assert result.success is False
        assert "not found" in result.error.lower()

    @pytest.mark.asyncio
    async def test_fetch_handles_other_http_errors(self):
        """fetch handles generic HTTP errors."""
        strategy = CursorWebStrategy()

        with patch.object(strategy, "_load_session_token", return_value="test_token"):
            with patch("vibeusage.providers.cursor.web.get_http_client") as mock_http:
                mock_response = AsyncMock()
                mock_response.status_code = 500

                mock_client = AsyncMock()
                mock_client.post = AsyncMock(return_value=mock_response)

                mock_cm = AsyncMock()
                mock_cm.__aenter__.return_value = mock_client
                mock_cm.__aexit__.return_value = None

                mock_http.return_value = mock_cm

                result = await strategy.fetch()

        assert result.success is False
        assert "failed" in result.error.lower()

    @pytest.mark.asyncio
    async def test_fetch_handles_invalid_json(self):
        """fetch handles invalid JSON response."""
        strategy = CursorWebStrategy()

        with patch.object(strategy, "_load_session_token", return_value="test_token"):
            with patch("vibeusage.providers.cursor.web.get_http_client") as mock_http:
                mock_response = Mock()
                mock_response.status_code = 200
                mock_response.json = Mock(side_effect=json.JSONDecodeError("Invalid JSON", "", 0))

                mock_client = AsyncMock()
                mock_client.post = AsyncMock(return_value=mock_response)

                mock_cm = AsyncMock()
                mock_cm.__aenter__.return_value = mock_client
                mock_cm.__aexit__.return_value = None

                mock_http.return_value = mock_cm

                result = await strategy.fetch()

        assert result.success is False
        assert "Invalid" in result.error or "parse" in result.error.lower()

    @pytest.mark.asyncio
    async def test_fetch_success_with_full_response(self):
        """fetch succeeds with complete API response."""
        strategy = CursorWebStrategy()

        usage_response_data = {
            "premium_requests": {
                "used": 450,
                "available": 550,
            },
            "billing_cycle": {
                "end": "2026-02-15T23:59:59Z",
            },
            "on_demand_spend": {
                "used_cents": 250,
                "limit_cents": 1000,
            },
        }

        user_response_data = {
            "email": "user@example.com",
            "membership_type": "pro",
        }

        with patch.object(strategy, "_load_session_token", return_value="test_token"):
            with patch("vibeusage.providers.cursor.web.get_http_client") as mock_http:
                # Mock usage response
                mock_usage_response = Mock()
                mock_usage_response.status_code = 200
                mock_usage_response.json = Mock(return_value=usage_response_data)

                # Mock user response
                mock_user_response = Mock()
                mock_user_response.status_code = 200
                mock_user_response.json = Mock(return_value=user_response_data)

                mock_client = AsyncMock()
                mock_client.post = AsyncMock(return_value=mock_usage_response)
                mock_client.get = AsyncMock(return_value=mock_user_response)

                mock_cm = AsyncMock()
                mock_cm.__aenter__.return_value = mock_client
                mock_cm.__aexit__.return_value = None

                mock_http.return_value = mock_cm

                result = await strategy.fetch()

        assert result.success is True
        assert result.snapshot is not None
        assert result.snapshot.provider == "cursor"
        assert result.snapshot.source == "web"

    @pytest.mark.asyncio
    async def test_fetch_success_without_user_data(self):
        """fetch succeeds even if user data fetch fails."""
        strategy = CursorWebStrategy()

        usage_response_data = {
            "premium_requests": {
                "used": 300,
                "available": 700,
            },
        }

        with patch.object(strategy, "_load_session_token", return_value="test_token"):
            with patch("vibeusage.providers.cursor.web.get_http_client") as mock_http:
                # Mock usage response
                mock_usage_response = Mock()
                mock_usage_response.status_code = 200
                mock_usage_response.json = Mock(return_value=usage_response_data)

                # Mock user response failure
                mock_user_response = AsyncMock()
                mock_user_response.status_code = 401

                mock_client = AsyncMock()
                mock_client.post = AsyncMock(return_value=mock_usage_response)
                mock_client.get = AsyncMock(return_value=mock_user_response)

                mock_cm = AsyncMock()
                mock_cm.__aenter__.return_value = mock_client
                mock_cm.__aexit__.return_value = None

                mock_http.return_value = mock_cm

                result = await strategy.fetch()

        assert result.success is True
        assert result.snapshot is not None
        assert result.snapshot.identity is None  # User data failed

    @pytest.mark.asyncio
    async def test_fetch_succeeds_without_overage(self):
        """fetch succeeds when no overage data present."""
        strategy = CursorWebStrategy()

        usage_response_data = {
            "premium_requests": {
                "used": 100,
                "available": 900,
            },
        }

        with patch.object(strategy, "_load_session_token", return_value="test_token"):
            with patch("vibeusage.providers.cursor.web.get_http_client") as mock_http:
                mock_usage_response = Mock()
                mock_usage_response.status_code = 200
                mock_usage_response.json = Mock(return_value=usage_response_data)

                mock_client = AsyncMock()
                mock_client.post = AsyncMock(return_value=mock_usage_response)
                mock_client.get = AsyncMock(return_value=Mock(status_code=404))

                mock_cm = AsyncMock()
                mock_cm.__aenter__.return_value = mock_client
                mock_cm.__aexit__.return_value = None

                mock_http.return_value = mock_cm

                result = await strategy.fetch()

        assert result.success is True
        assert result.snapshot.overage is None

    def test_parse_response_full(self):
        """_parse_response correctly parses full API response."""
        strategy = CursorWebStrategy()

        usage_data = {
            "premium_requests": {
                "used": 450,
                "available": 550,
            },
            "billing_cycle": {
                "end": "2026-02-15T23:59:59Z",
            },
            "on_demand_spend": {
                "used_cents": 250,
                "limit_cents": 1000,
            },
        }

        user_data = {
            "email": "user@example.com",
            "membership_type": "pro",
        }

        snapshot = strategy._parse_response(usage_data, user_data)

        assert snapshot is not None
        assert snapshot.provider == "cursor"
        assert len(snapshot.periods) == 1

        period = snapshot.periods[0]
        assert period.name == "Premium Requests"
        assert period.utilization == 45  # 450/1000
        assert period.period_type == PeriodType.MONTHLY
        assert period.resets_at is not None

        # Check overage
        assert snapshot.overage is not None
        assert snapshot.overage.used == Decimal("2.50")  # 250 cents
        assert snapshot.overage.limit == Decimal("10.00")  # 1000 cents
        assert snapshot.overage.currency == "USD"

        # Check identity
        assert snapshot.identity is not None
        assert snapshot.identity.email == "user@example.com"
        assert snapshot.identity.plan == "pro"

    def test_parse_response_with_unix_timestamp(self):
        """_parse_response handles Unix timestamp in milliseconds."""
        strategy = CursorWebStrategy()

        # Feb 15, 2026 23:59:59 UTC in milliseconds
        timestamp = 1770086399000

        usage_data = {
            "premium_requests": {
                "used": 200,
                "available": 800,
            },
            "billing_cycle": {
                "end": timestamp,
            },
        }

        snapshot = strategy._parse_response(usage_data, None)

        assert snapshot is not None
        assert snapshot.periods[0].resets_at is not None
        assert snapshot.periods[0].resets_at.year == 2026
        assert snapshot.periods[0].resets_at.month == 2

    def test_parse_response_minimal(self):
        """_parse_response handles minimal response."""
        strategy = CursorWebStrategy()

        usage_data = {
            "premium_requests": {
                "used": 250,
                "available": 750,
            },
        }

        snapshot = strategy._parse_response(usage_data, None)

        assert snapshot is not None
        assert len(snapshot.periods) == 1
        assert snapshot.periods[0].utilization == 25  # 250/1000
        assert snapshot.periods[0].resets_at is None
        assert snapshot.overage is None
        assert snapshot.identity is None

    def test_parse_response_zero_total(self):
        """_parse_response handles zero total requests."""
        strategy = CursorWebStrategy()

        usage_data = {
            "premium_requests": {
                "used": 0,
                "available": 0,
            },
        }

        snapshot = strategy._parse_response(usage_data, None)

        assert snapshot is not None
        assert snapshot.periods[0].utilization == 0

    def test_parse_response_no_premium_requests(self):
        """_parse_response returns None when no premium_requests."""
        strategy = CursorWebStrategy()

        usage_data = {
            "billing_cycle": {"end": "2026-02-15T23:59:59Z"},
        }

        snapshot = strategy._parse_response(usage_data, None)

        assert snapshot is None

    def test_parse_response_with_zero_overage_limit(self):
        """_parse_response handles zero overage limit (disabled overage)."""
        strategy = CursorWebStrategy()

        usage_data = {
            "premium_requests": {
                "used": 500,
                "available": 500,
            },
            "on_demand_spend": {
                "used_cents": 100,
                "limit_cents": 0,  # Disabled
            },
        }

        snapshot = strategy._parse_response(usage_data, None)

        assert snapshot is not None
        assert snapshot.overage is None  # Zero limit means no overage

    def test_parse_response_invalid_date(self):
        """_parse_response handles invalid date gracefully."""
        strategy = CursorWebStrategy()

        usage_data = {
            "premium_requests": {
                "used": 300,
                "available": 700,
            },
            "billing_cycle": {
                "end": "invalid-date",
            },
        }

        snapshot = strategy._parse_response(usage_data, None)

        assert snapshot is not None
        assert snapshot.periods[0].resets_at is None


class TestCursorBrowserCookieStrategy:
    """Tests for CursorBrowserCookieStrategy."""

    def setup_method(self):
        """Import strategy class for testing."""
        from vibeusage.providers.cursor.web import CursorBrowserCookieStrategy

        self.strategy_cls = CursorBrowserCookieStrategy

    def test_name_property(self):
        """Strategy has correct name."""
        strategy = self.strategy_cls()
        assert strategy.name == "browser"

    def test_cookie_domains(self):
        """COOKIE_DOMAINS includes expected values."""
        strategy = self.strategy_cls()
        assert "cursor.com" in str(strategy.COOKIE_DOMAINS)
        assert "cursor.sh" in str(strategy.COOKIE_DOMAINS)

    def test_cookie_names(self):
        """COOKIE_NAMES includes expected values."""
        strategy = self.strategy_cls()
        assert "WorkosCursorSessionToken" in strategy.COOKIE_NAMES

    def test_is_available_always_returns_true(self):
        """is_available always returns True."""
        strategy = self.strategy_cls()
        assert strategy.is_available() is True

    @pytest.mark.asyncio
    async def test_fetch_fails_without_browser_cookie_lib(self):
        """fetch fails when browser_cookie3 not available."""
        strategy = self.strategy_cls()

        with patch("builtins.__import__", side_effect=ImportError):
            result = await strategy.fetch()
            assert result.success is False
            assert "browser_cookie3" in result.error or "pycookiecheat" in result.error


class TestCursorProviderIntegration:
    """Integration tests for Cursor provider with registry."""

    def test_cursor_registered(self):
        """CursorProvider is registered in the registry."""
        provider_cls = get_provider("cursor")
        assert provider_cls is not None
        assert provider_cls == CursorProvider

    def test_create_cursor_provider(self):
        """Can create CursorProvider instance via registry."""
        provider = create_provider("cursor")
        assert isinstance(provider, CursorProvider)

    def test_list_includes_cursor(self):
        """list_provider_ids includes cursor."""
        ids = list_provider_ids()
        assert "cursor" in ids


class TestCursorStatus:
    """Tests for Cursor status fetching."""

    @pytest.mark.asyncio
    async def test_fetch_cursor_status(self):
        """fetch_cursor_status returns ProviderStatus."""
        from vibeusage.providers.cursor.status import fetch_cursor_status

        with patch("vibeusage.providers.claude.status.fetch_url", return_value=b'{"status": {"indicator": "none"}}'):
            status = await fetch_cursor_status()

            assert status is not None
            assert status.level.value == "operational"
