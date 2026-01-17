"""Tests for auth/base.py authentication base classes and protocols."""

from __future__ import annotations

from datetime import UTC
from datetime import datetime
from datetime import timedelta
from pathlib import Path
from unittest.mock import AsyncMock
from unittest.mock import MagicMock

import pytest

from vibeusage.auth.base import (
    APIKeyCredentials,
    AuthCredentials,
    AuthResult,
    AuthStrategy,
    CLICredentials,
    CLIConfig,
    CookieConfig,
    DeviceFlowConfig,
    LocalProcessConfig,
    LocalProcessCredentials,
    OAuth2Config,
    OAuth2Credentials,
    ProviderAuthConfig,
    REFRESH_BUFFER,
    SessionCredentials,
)

# Test REFRESH_BUFFER constant


def test_refresh_buffer_constant() -> None:
    """REFRESH_BUFFER should be 5 minutes."""
    assert REFRESH_BUFFER == timedelta(minutes=5)


class TestOAuth2Credentials:
    """Tests for OAuth2Credentials class."""

    def test_oauth2_credentials_init_with_all_fields(self, utc_now: datetime) -> None:
        """OAuth2Credentials initializes with all optional fields."""
        expires = utc_now + timedelta(hours=1)
        creds = OAuth2Credentials(
            access_token="test_access_token",
            refresh_token="test_refresh_token",
            expires_at=expires,
            token_type="Bearer",
            scope="usage.read",
        )
        assert creds.access_token == "test_access_token"
        assert creds.refresh_token == "test_refresh_token"
        assert creds.expires_at == expires
        assert creds.token_type == "Bearer"
        assert creds.scope == "usage.read"

    def test_oauth2_credentials_init_with_required_only(self) -> None:
        """OAuth2Credentials initializes with only required field."""
        creds = OAuth2Credentials(access_token="test_token")
        assert creds.access_token == "test_token"
        assert creds.refresh_token is None
        assert creds.expires_at is None
        assert creds.token_type == "Bearer"
        assert creds.scope is None

    def test_is_expired_with_no_expiry(self) -> None:
        """OAuth2Credentials with no expiry is not expired."""
        creds = OAuth2Credentials(access_token="test_token")
        assert creds.is_expired() is False

    def test_is_expired_with_future_expiry(self) -> None:
        """OAuth2Credentials with future expiry is not expired."""
        # Use actual future time since is_expired() uses datetime.now()
        expires = datetime.now(UTC) + timedelta(hours=1)
        creds = OAuth2Credentials(access_token="test_token", expires_at=expires)
        assert creds.is_expired() is False

    def test_is_expired_with_past_expiry(self, utc_now: datetime) -> None:
        """OAuth2Credentials with past expiry is expired."""
        expires = utc_now - timedelta(minutes=1)
        creds = OAuth2Credentials(access_token="test_token", expires_at=expires)
        assert creds.is_expired() is True

    def test_is_expired_within_refresh_buffer(self, utc_now: datetime) -> None:
        """OAuth2Credentials within refresh buffer is considered expired."""
        # 4 minutes in the future is within 5 minute buffer
        expires = utc_now + timedelta(minutes=4)
        creds = OAuth2Credentials(access_token="test_token", expires_at=expires)
        assert creds.is_expired() is True

    def test_is_expired_exactly_at_buffer_boundary(self) -> None:
        """OAuth2Credentials exactly at buffer boundary is considered expired."""
        # Just outside 5 minute buffer
        expires = datetime.now(UTC) + timedelta(minutes=5, seconds=1)
        creds = OAuth2Credentials(access_token="test_token", expires_at=expires)
        assert creds.is_expired() is False

    def test_to_headers_with_default_token_type(self) -> None:
        """to_headers returns Bearer token header by default."""
        creds = OAuth2Credentials(access_token="test_token")
        assert creds.to_headers() == {"Bearer": "test_token"}

    def test_to_headers_with_custom_token_type(self) -> None:
        """to_headers returns custom token type header."""
        creds = OAuth2Credentials(
            access_token="test_token", token_type="Token"
        )
        assert creds.to_headers() == {"Token": "test_token"}

    def test_can_refresh_with_refresh_token(self) -> None:
        """can_refresh returns True when refresh token exists."""
        creds = OAuth2Credentials(
            access_token="test_token", refresh_token="refresh_token"
        )
        assert creds.can_refresh() is True

    def test_can_refresh_without_refresh_token(self) -> None:
        """can_refresh returns False when no refresh token."""
        creds = OAuth2Credentials(access_token="test_token")
        assert creds.can_refresh() is False

    def test_is_expired_with_timezone_aware_datetime(self) -> None:
        """is_expired handles timezone-aware datetimes correctly."""
        now_utc = datetime.now(UTC)
        expires = now_utc + timedelta(minutes=3)
        creds = OAuth2Credentials(access_token="test_token", expires_at=expires)
        assert creds.is_expired() is True  # Within 5-minute buffer


class TestSessionCredentials:
    """Tests for SessionCredentials class."""

    def test_session_credentials_init_with_all_fields(self, utc_now: datetime) -> None:
        """SessionCredentials initializes with all fields."""
        expires = utc_now + timedelta(hours=1)
        creds = SessionCredentials(
            session_key="test_session_key",
            cookie_name="customCookie",
            expires_at=expires,
        )
        assert creds.session_key == "test_session_key"
        assert creds.cookie_name == "customCookie"
        assert creds.expires_at == expires

    def test_session_credentials_init_with_required_only(self) -> None:
        """SessionCredentials initializes with only required field."""
        creds = SessionCredentials(session_key="test_key")
        assert creds.session_key == "test_key"
        assert creds.cookie_name == "sessionKey"
        assert creds.expires_at is None

    def test_is_expired_with_no_expiry(self) -> None:
        """SessionCredentials with no expiry is not expired."""
        creds = SessionCredentials(session_key="test_key")
        assert creds.is_expired() is False

    def test_is_expired_with_future_expiry(self) -> None:
        """SessionCredentials with future expiry is not expired."""
        # Use actual future time since is_expired() uses datetime.now()
        expires = datetime.now(UTC) + timedelta(hours=1)
        creds = SessionCredentials(session_key="test_key", expires_at=expires)
        assert creds.is_expired() is False

    def test_is_expired_with_past_expiry(self, utc_now: datetime) -> None:
        """SessionCredentials with past expiry is expired."""
        expires = utc_now - timedelta(minutes=1)
        creds = SessionCredentials(session_key="test_key", expires_at=expires)
        assert creds.is_expired() is True

    def test_is_expired_without_buffer(self) -> None:
        """SessionCredentials does not use refresh buffer (no grace period)."""
        # Unlike OAuth, session cookies don't have a buffer
        expires = datetime.now(UTC) + timedelta(seconds=30)
        creds = SessionCredentials(session_key="test_key", expires_at=expires)
        assert creds.is_expired() is False

    def test_to_headers_with_default_cookie_name(self) -> None:
        """to_headers returns default sessionKey cookie."""
        creds = SessionCredentials(session_key="test_key")
        assert creds.to_headers() == {"sessionKey": "test_key"}

    def test_to_headers_with_custom_cookie_name(self) -> None:
        """to_headers returns custom cookie name."""
        creds = SessionCredentials(
            session_key="test_key", cookie_name="WorkosSessionToken"
        )
        assert creds.to_headers() == {"WorkosSessionToken": "test_key"}


class TestAPIKeyCredentials:
    """Tests for APIKeyCredentials class."""

    def test_api_key_credentials_init_with_defaults(self) -> None:
        """APIKeyCredentials initializes with default values."""
        creds = APIKeyCredentials(api_key="test_api_key")
        assert creds.api_key == "test_api_key"
        assert creds.header_name == "Authorization"
        assert creds.prefix == "Bearer"

    def test_api_key_credentials_init_with_custom_values(self) -> None:
        """APIKeyCredentials initializes with custom values."""
        creds = APIKeyCredentials(
            api_key="test_key",
            header_name="X-API-Key",
            prefix="",
        )
        assert creds.api_key == "test_key"
        assert creds.header_name == "X-API-Key"
        assert creds.prefix == ""

    def test_is_expired_always_false(self) -> None:
        """APIKeyCredentials never expire."""
        creds = APIKeyCredentials(api_key="test_key")
        assert creds.is_expired() is False

    def test_to_headers_with_prefix(self) -> None:
        """to_headers returns Authorization header with prefix."""
        creds = APIKeyCredentials(api_key="test_key", prefix="Bearer")
        assert creds.to_headers() == {"Authorization": "Bearer test_key"}

    def test_to_headers_without_prefix(self) -> None:
        """to_headers returns header without prefix."""
        creds = APIKeyCredentials(
            api_key="test_key", header_name="X-API-Key", prefix=""
        )
        assert creds.to_headers() == {"X-API-Key": "test_key"}

    def test_to_headers_with_custom_prefix(self) -> None:
        """to_headers returns header with custom prefix."""
        creds = APIKeyCredentials(
            api_key="test_key", prefix="Token"
        )
        assert creds.to_headers() == {"Authorization": "Token test_key"}

    def test_to_headers_with_api_key_prefix(self) -> None:
        """to_headers works for common API key patterns."""
        creds = APIKeyCredentials(
            api_key="sk-12345", header_name="X-API-Key", prefix=""
        )
        assert creds.to_headers() == {"X-API-Key": "sk-12345"}


class TestCLICredentials:
    """Tests for CLICredentials class."""

    def test_cli_credentials_init(self) -> None:
        """CLICredentials initializes with command."""
        creds = CLICredentials(command="claude")
        assert creds.command == "claude"

    def test_is_expired_always_false(self) -> None:
        """CLICredentials never expire (CLI manages its own session)."""
        creds = CLICredentials(command="claude")
        assert creds.is_expired() is False

    def test_to_headers_returns_empty_dict(self) -> None:
        """CLICredentials to_headers returns empty dict (not used for HTTP)."""
        creds = CLICredentials(command="claude")
        assert creds.to_headers() == {}


class TestLocalProcessCredentials:
    """Tests for LocalProcessCredentials class."""

    def test_local_process_credentials_init_defaults(self) -> None:
        """LocalProcessCredentials initializes with default host."""
        creds = LocalProcessCredentials(csrf_token="test_csrf", port=8080)
        assert creds.csrf_token == "test_csrf"
        assert creds.port == 8080
        assert creds.host == "127.0.0.1"

    def test_local_process_credentials_init_custom_host(self) -> None:
        """LocalProcessCredentials initializes with custom host."""
        creds = LocalProcessCredentials(
            csrf_token="test_csrf", port=8080, host="localhost"
        )
        assert creds.host == "localhost"

    def test_is_expired_always_false(self) -> None:
        """LocalProcessCredentials never expire (valid while process running)."""
        creds = LocalProcessCredentials(csrf_token="test_csrf", port=8080)
        assert creds.is_expired() is False

    def test_to_headers(self) -> None:
        """to_headers returns CSRF token header."""
        creds = LocalProcessCredentials(csrf_token="test_csrf", port=8080)
        assert creds.to_headers() == {"X-CSRF-Token": "test_csrf"}

    def test_base_url_property_default_host(self) -> None:
        """base_url returns correct URL with default host."""
        creds = LocalProcessCredentials(csrf_token="test_csrf", port=8080)
        assert creds.base_url == "http://127.0.0.1:8080"

    def test_base_url_property_custom_host(self) -> None:
        """base_url returns correct URL with custom host."""
        creds = LocalProcessCredentials(
            csrf_token="test_csrf", port=9000, host="localhost"
        )
        assert creds.base_url == "http://localhost:9000"


class TestAuthResult:
    """Tests for AuthResult class."""

    def test_auth_result_ok_factory(self) -> None:
        """AuthResult.ok creates successful result."""
        creds = OAuth2Credentials(access_token="test_token")
        result = AuthResult.ok(creds, "oauth")
        assert result.success is True
        assert result.credentials is creds
        assert result.source == "oauth"
        assert result.error is None

    def test_auth_result_fail_factory(self) -> None:
        """AuthResult.fail creates failed result."""
        result = AuthResult.fail("Invalid credentials")
        assert result.success is False
        assert result.credentials is None
        assert result.error == "Invalid credentials"
        assert result.source is None

    def test_auth_result_ok_with_session_credentials(self) -> None:
        """AuthResult.ok works with different credential types."""
        creds = SessionCredentials(session_key="test_key")
        result = AuthResult.ok(creds, "web")
        assert result.success is True
        assert isinstance(result.credentials, SessionCredentials)

    def test_auth_result_fail_with_empty_error(self) -> None:
        """AuthResult.fail works with empty error message."""
        result = AuthResult.fail("")
        assert result.success is False
        assert result.error == ""

    def test_auth_result_fail_with_detailed_error(self) -> None:
        """AuthResult.fail works with detailed error messages."""
        error = "401 Unauthorized: Invalid token"
        result = AuthResult.fail(error)
        assert result.success is False
        assert result.error == error


class TestAuthStrategy:
    """Tests for AuthStrategy abstract base class."""

    def test_auth_strategy_is_abstract(self) -> None:
        """AuthStrategy cannot be instantiated directly."""
        with pytest.raises(TypeError):
            AuthStrategy()  # type: ignore

    def test_auth_strategy_requires_name_property(self) -> None:
        """Subclass must implement name property."""
        with pytest.raises(TypeError):

            class IncompleteStrategy(AuthStrategy):
                async def is_available(self) -> bool:
                    return True

                async def authenticate(self) -> AuthResult:
                    return AuthResult.fail("Not implemented")

            IncompleteStrategy()

    def test_auth_strategy_requires_is_available_method(self) -> None:
        """Subclass must implement is_available method."""
        with pytest.raises(TypeError):

            class IncompleteStrategy(AuthStrategy):
                @property
                def name(self) -> str:
                    return "test"

                async def authenticate(self) -> AuthResult:
                    return AuthResult.fail("Not implemented")

            IncompleteStrategy()

    def test_auth_strategy_requires_authenticate_method(self) -> None:
        """Subclass must implement authenticate method."""
        with pytest.raises(TypeError):

            class IncompleteStrategy(AuthStrategy):
                @property
                def name(self) -> str:
                    return "test"

                async def is_available(self) -> bool:
                    return True

            IncompleteStrategy()

    def test_auth_strategy_concrete_implementation(self) -> None:
        """Concrete AuthStrategy subclass can be instantiated."""
        class ConcreteStrategy(AuthStrategy):
            @property
            def name(self) -> str:
                return "concrete"

            async def is_available(self) -> bool:
                return True

            async def authenticate(self) -> AuthResult:
                return AuthResult.ok(
                    OAuth2Credentials(access_token="test"), "concrete"
                )

        strategy = ConcreteStrategy()
        assert strategy.name == "concrete"

    async def test_auth_strategy_refresh_default_reauth(self) -> None:
        """Default refresh implementation calls authenticate."""
        class ConcreteStrategy(AuthStrategy):
            @property
            def name(self) -> str:
                return "test"

            async def is_available(self) -> bool:
                return True

            async def authenticate(self) -> AuthResult:
                return AuthResult.ok(
                    OAuth2Credentials(access_token="new_token"), "test"
                )

        strategy = ConcreteStrategy()
        result = await strategy.refresh(OAuth2Credentials(access_token="old"))
        assert result.success is True
        assert result.credentials.access_token == "new_token"

    async def test_auth_strategy_custom_refresh(self) -> None:
        """Subclass can override refresh for token refresh."""
        refresh_called = []

        class ConcreteStrategy(AuthStrategy):
            @property
            def name(self) -> str:
                return "test"

            async def is_available(self) -> bool:
                return True

            async def authenticate(self) -> AuthResult:
                return AuthResult.fail("Use refresh instead")

            async def refresh(self, credentials: AuthCredentials) -> AuthResult:
                refresh_called.append(credentials)
                return AuthResult.ok(
                    OAuth2Credentials(access_token="refreshed_token"), "test"
                )

        strategy = ConcreteStrategy()
        old_creds = OAuth2Credentials(access_token="old")
        result = await strategy.refresh(old_creds)

        assert len(refresh_called) == 1
        assert refresh_called[0] is old_creds
        assert result.success is True
        assert result.credentials.access_token == "refreshed_token"


class TestOAuth2Config:
    """Tests for OAuth2Config struct."""

    def test_oauth2_config_init_with_required_fields(self) -> None:
        """OAuth2Config initializes with required fields."""
        config = OAuth2Config(
            token_endpoint="https://example.com/oauth/token",
            client_id="test_client_id",
        )
        assert config.token_endpoint == "https://example.com/oauth/token"
        assert config.client_id == "test_client_id"
        assert config.client_secret is None
        assert config.scope is None
        assert config.credentials_file is None
        assert config.refresh_threshold_days == 7

    def test_oauth2_config_init_with_all_fields(self, tmp_path: Path) -> None:
        """OAuth2Config initializes with all fields."""
        config = OAuth2Config(
            token_endpoint="https://example.com/oauth/token",
            client_id="test_client_id",
            client_secret="test_secret",
            scope="usage.read",
            credentials_file=tmp_path / "creds.json",
            refresh_threshold_days=14,
        )
        assert config.client_secret == "test_secret"
        assert config.scope == "usage.read"
        assert config.credentials_file == tmp_path / "creds.json"
        assert config.refresh_threshold_days == 14


class TestCookieConfig:
    """Tests for CookieConfig struct."""

    def test_cookie_config_init(self) -> None:
        """CookieConfig initializes correctly."""
        config = CookieConfig(
            cookie_names=["sessionKey", "WorkosSessionToken"],
            domains=[".claude.ai", ".anthropic.com"],
        )
        assert config.cookie_names == ["sessionKey", "WorkosSessionToken"]
        assert config.domains == [".claude.ai", ".anthropic.com"]
        assert config.stored_session_file is None

    def test_cookie_config_with_stored_session_file(self, tmp_path: Path) -> None:
        """CookieConfig with stored session file."""
        config = CookieConfig(
            cookie_names=["sessionKey"],
            domains=[".example.com"],
            stored_session_file=tmp_path / "session.json",
        )
        assert config.stored_session_file == tmp_path / "session.json"


class TestCLIConfig:
    """Tests for CLIConfig struct."""

    def test_cli_config_init_with_required_fields(self) -> None:
        """CLIConfig initializes with required fields."""
        config = CLIConfig(
            command="claude",
            usage_args=["/usage"],
        )
        assert config.command == "claude"
        assert config.usage_args == ["/usage"]
        assert config.version_args is None

    def test_cli_config_init_with_all_fields(self) -> None:
        """CLIConfig initializes with all fields."""
        config = CLIConfig(
            command="claude",
            usage_args=["/usage", "--json"],
            version_args=["--version"],
        )
        assert config.usage_args == ["/usage", "--json"]
        assert config.version_args == ["--version"]


class TestDeviceFlowConfig:
    """Tests for DeviceFlowConfig struct."""

    def test_device_flow_config_init_with_required(self) -> None:
        """DeviceFlowConfig initializes with required field."""
        config = DeviceFlowConfig(client_id="test_client_id")
        assert config.client_id == "test_client_id"
        assert config.device_code_url == "https://github.com/login/device/code"
        assert config.token_url == "https://github.com/login/oauth/access_token"
        assert config.scope == "read:user"
        assert config.credentials_file is None

    def test_device_flow_config_init_with_custom_values(
        self, tmp_path: Path
    ) -> None:
        """DeviceFlowConfig with custom values."""
        config = DeviceFlowConfig(
            client_id="custom_client_id",
            device_code_url="https://example.com/device/code",
            token_url="https://example.com/oauth/token",
            scope="read:org",
            credentials_file=tmp_path / "device.json",
        )
        assert config.device_code_url == "https://example.com/device/code"
        assert config.token_url == "https://example.com/oauth/token"
        assert config.scope == "read:org"
        assert config.credentials_file == tmp_path / "device.json"


class TestLocalProcessConfig:
    """Tests for LocalProcessConfig struct."""

    def test_local_process_config_init_with_required(self) -> None:
        """LocalProcessConfig with required fields."""
        config = LocalProcessConfig(
            process_name="Cursor",
            csrf_pattern=r"--csrf=([a-f0-9-]+)",
        )
        assert config.process_name == "Cursor"
        assert config.csrf_pattern == r"--csrf=([a-f0-9-]+)"
        assert config.port_range == (8000, 9000)

    def test_local_process_config_with_custom_port_range(self) -> None:
        """LocalProcessConfig with custom port range."""
        config = LocalProcessConfig(
            process_name="Cursor",
            csrf_pattern=r"--csrf=([a-f0-9-]+)",
            port_range=(9000, 10000),
        )
        assert config.port_range == (9000, 10000)


class TestProviderAuthConfig:
    """Tests for ProviderAuthConfig class."""

    async def test_provider_auth_config_authenticate_first_success(self) -> None:
        """ProviderAuthConfig succeeds with first available strategy."""
        strategy1 = MagicMock(spec=AuthStrategy)
        strategy1.name = "strategy1"
        strategy1.is_available = AsyncMock(return_value=True)
        strategy1.authenticate = AsyncMock(
            return_value=AuthResult.ok(
                OAuth2Credentials(access_token="token1"), "strategy1"
            )
        )

        strategy2 = MagicMock(spec=AuthStrategy)
        strategy2.name = "strategy2"
        strategy2.is_available = AsyncMock(return_value=True)
        strategy2.authenticate = AsyncMock(
            return_value=AuthResult.ok(
                OAuth2Credentials(access_token="token2"), "strategy2"
            )
        )

        config = ProviderAuthConfig(strategies=[strategy1, strategy2])
        result = await config.authenticate()

        assert result.success is True
        assert result.source == "strategy1"
        assert result.credentials.access_token == "token1"
        strategy1.is_available.assert_awaited_once()
        strategy1.authenticate.assert_awaited_once()
        # Second strategy should not be tried
        strategy2.is_available.assert_not_awaited()

    async def test_provider_auth_config_fallback_to_second_strategy(self) -> None:
        """ProviderAuthConfig falls back to second strategy when first fails."""
        strategy1 = MagicMock(spec=AuthStrategy)
        strategy1.name = "strategy1"
        strategy1.is_available = AsyncMock(return_value=True)
        strategy1.authenticate = AsyncMock(
            return_value=AuthResult.fail("No credentials")
        )

        strategy2 = MagicMock(spec=AuthStrategy)
        strategy2.name = "strategy2"
        strategy2.is_available = AsyncMock(return_value=True)
        strategy2.authenticate = AsyncMock(
            return_value=AuthResult.ok(
                OAuth2Credentials(access_token="token2"), "strategy2"
            )
        )

        config = ProviderAuthConfig(strategies=[strategy1, strategy2])
        result = await config.authenticate()

        assert result.success is True
        assert result.source == "strategy2"
        assert result.credentials.access_token == "token2"
        strategy1.is_available.assert_awaited_once()
        strategy1.authenticate.assert_awaited_once()
        strategy2.is_available.assert_awaited_once()
        strategy2.authenticate.assert_awaited_once()

    async def test_provider_auth_config_skips_unavailable_strategies(self) -> None:
        """ProviderAuthConfig skips strategies that are not available."""
        strategy1 = MagicMock(spec=AuthStrategy)
        strategy1.name = "unavailable"
        strategy1.is_available = AsyncMock(return_value=False)

        strategy2 = MagicMock(spec=AuthStrategy)
        strategy2.name = "available"
        strategy2.is_available = AsyncMock(return_value=True)
        strategy2.authenticate = AsyncMock(
            return_value=AuthResult.ok(
                OAuth2Credentials(access_token="token2"), "available"
            )
        )

        config = ProviderAuthConfig(strategies=[strategy1, strategy2])
        result = await config.authenticate()

        assert result.success is True
        assert result.source == "available"
        strategy1.is_available.assert_awaited_once()
        strategy1.authenticate.assert_not_awaited()
        strategy2.is_available.assert_awaited_once()
        strategy2.authenticate.assert_awaited_once()

    async def test_provider_auth_config_all_strategies_fail(self) -> None:
        """ProviderAuthConfig fails when all strategies fail."""
        strategy1 = MagicMock(spec=AuthStrategy)
        strategy1.name = "strategy1"
        strategy1.is_available = AsyncMock(return_value=True)
        strategy1.authenticate = AsyncMock(
            return_value=AuthResult.fail("Error 1")
        )

        strategy2 = MagicMock(spec=AuthStrategy)
        strategy2.name = "strategy2"
        strategy2.is_available = AsyncMock(return_value=True)
        strategy2.authenticate = AsyncMock(
            return_value=AuthResult.fail("Error 2")
        )

        config = ProviderAuthConfig(strategies=[strategy1, strategy2])
        result = await config.authenticate()

        assert result.success is False
        assert "All authentication methods failed" in result.error
        assert "strategy1: Error 1" in result.error
        assert "strategy2: Error 2" in result.error

    async def test_provider_auth_config_no_available_strategies(self) -> None:
        """ProviderAuthConfig fails when no strategies are available."""
        strategy1 = MagicMock(spec=AuthStrategy)
        strategy1.name = "strategy1"
        strategy1.is_available = AsyncMock(return_value=False)

        strategy2 = MagicMock(spec=AuthStrategy)
        strategy2.name = "strategy2"
        strategy2.is_available = AsyncMock(return_value=False)

        config = ProviderAuthConfig(strategies=[strategy1, strategy2])
        result = await config.authenticate()

        assert result.success is False
        assert "All authentication methods failed" in result.error

    async def test_provider_auth_config_empty_strategy_list(self) -> None:
        """ProviderAuthConfig fails with empty strategy list."""
        config = ProviderAuthConfig(strategies=[])
        result = await config.authenticate()

        assert result.success is False
        assert result.error == "All authentication methods failed:\n"

    async def test_provider_auth_config_partial_availability_then_failure(
        self,
    ) -> None:
        """ProviderAuthConfig handles mixed availability and failures."""
        strategy1 = MagicMock(spec=AuthStrategy)
        strategy1.name = "unavailable"
        strategy1.is_available = AsyncMock(return_value=False)

        strategy2 = MagicMock(spec=AuthStrategy)
        strategy2.name = "available_but_fails"
        strategy2.is_available = AsyncMock(return_value=True)
        strategy2.authenticate = AsyncMock(
            return_value=AuthResult.fail("Invalid token")
        )

        strategy3 = MagicMock(spec=AuthStrategy)
        strategy3.name = "also_unavailable"
        strategy3.is_available = AsyncMock(return_value=False)

        config = ProviderAuthConfig(strategies=[strategy1, strategy2, strategy3])
        result = await config.authenticate()

        assert result.success is False
        assert "available_but_fails: Invalid token" in result.error
        # Unavailable strategies should not have error messages
        assert "unavailable:" not in result.error
        assert "also_unavailable:" not in result.error


# Test Protocol compatibility


def test_oauth2_credentials_implements_protocol() -> None:
    """OAuth2Credentials implements AuthCredentials protocol."""
    creds = OAuth2Credentials(access_token="test_token")
    # Should not raise - implements protocol
    assert isinstance(creds.is_expired(), bool)
    assert isinstance(creds.to_headers(), dict)


def test_session_credentials_implements_protocol() -> None:
    """SessionCredentials implements AuthCredentials protocol."""
    creds = SessionCredentials(session_key="test_key")
    assert isinstance(creds.is_expired(), bool)
    assert isinstance(creds.to_headers(), dict)


def test_api_key_credentials_implements_protocol() -> None:
    """APIKeyCredentials implements AuthCredentials protocol."""
    creds = APIKeyCredentials(api_key="test_key")
    assert isinstance(creds.is_expired(), bool)
    assert isinstance(creds.to_headers(), dict)


def test_cli_credentials_implements_protocol() -> None:
    """CLICredentials implements AuthCredentials protocol."""
    creds = CLICredentials(command="test")
    assert isinstance(creds.is_expired(), bool)
    assert isinstance(creds.to_headers(), dict)


def test_local_process_credentials_implements_protocol() -> None:
    """LocalProcessCredentials implements AuthCredentials protocol."""
    creds = LocalProcessCredentials(csrf_token="test", port=8080)
    assert isinstance(creds.is_expired(), bool)
    assert isinstance(creds.to_headers(), dict)
