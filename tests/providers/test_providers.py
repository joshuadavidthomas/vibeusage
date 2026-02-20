"""Tests for provider registry and Claude provider."""

from __future__ import annotations

from datetime import UTC
from datetime import datetime
from unittest.mock import MagicMock
from unittest.mock import patch

import pytest

from vibeusage.models import StatusLevel
from vibeusage.providers import ClaudeProvider
from vibeusage.providers import create_provider
from vibeusage.providers import get_all_providers
from vibeusage.providers import get_provider
from vibeusage.providers import list_provider_ids
from vibeusage.providers import register_provider
from vibeusage.providers.base import Provider
from vibeusage.providers.base import ProviderMetadata


class TestProviderMetadata:
    """Tests for ProviderMetadata."""

    def test_create_metadata(self):
        """Can create ProviderMetadata."""
        metadata = ProviderMetadata(
            id="test",
            name="Test Provider",
            description="A test provider",
            homepage="https://example.com",
            status_url="https://status.example.com",
            dashboard_url="https://dashboard.example.com",
        )

        assert metadata.id == "test"
        assert metadata.name == "Test Provider"
        assert metadata.description == "A test provider"
        assert metadata.homepage == "https://example.com"
        assert metadata.status_url == "https://status.example.com"
        assert metadata.dashboard_url == "https://dashboard.example.com"

    def test_metadata_without_optional_fields(self):
        """Can create metadata without optional fields."""
        metadata = ProviderMetadata(
            id="test",
            name="Test",
            description="Test",
            homepage="https://example.com",
        )

        assert metadata.status_url is None
        assert metadata.dashboard_url is None

    def test_metadata_immutability(self):
        """ProviderMetadata is immutable."""
        metadata = ProviderMetadata(
            id="test", name="Test", description="Test", homepage="https://example.com"
        )

        with pytest.raises(AttributeError):
            metadata.name = "Changed"


class TestProviderBase:
    """Tests for Provider base class."""

    def test_provider_has_id_property(self):
        """Provider has id property from metadata."""
        metadata = ProviderMetadata(
            id="test_provider",
            name="Test",
            description="Test",
            homepage="https://example.com",
        )

        class TestProvider(Provider):
            def fetch_strategies(self):
                return []

            async def fetch_status(self):
                from vibeusage.models import ProviderStatus

                return ProviderStatus.unknown()

        TestProvider.metadata = metadata
        provider = TestProvider()
        assert provider.id == "test_provider"

    def test_provider_has_name_property(self):
        """Provider has name property from metadata."""
        metadata = ProviderMetadata(
            id="test",
            name="Test Provider",
            description="Test",
            homepage="https://example.com",
        )

        class TestProvider(Provider):
            def fetch_strategies(self):
                return []

        TestProvider.metadata = metadata
        provider = TestProvider()
        assert provider.name == "Test Provider"

    def test_provider_requires_metadata(self):
        """Provider subclass must define metadata."""
        with pytest.raises(TypeError):
            # Can't instantiate without metadata
            class IncompleteProvider(Provider):
                pass

            IncompleteProvider()

    def test_provider_is_abstract(self):
        """Can't instantiate Provider directly."""
        with pytest.raises(TypeError):
            Provider()


class TestClaudeProvider:
    """Tests for ClaudeProvider."""

    def test_metadata(self):
        """ClaudeProvider has correct metadata."""
        assert ClaudeProvider.metadata.id == "claude"
        assert ClaudeProvider.metadata.name == "Claude"
        assert "Anthropic" in ClaudeProvider.metadata.description
        assert ClaudeProvider.metadata.homepage == "https://claude.ai"
        assert ClaudeProvider.metadata.status_url == "https://status.anthropic.com"
        assert (
            ClaudeProvider.metadata.dashboard_url == "https://claude.ai/settings/usage"
        )

    def test_id_property(self):
        """id property returns correct value."""
        provider = ClaudeProvider()
        assert provider.id == "claude"

    def test_name_property(self):
        """name property returns correct value."""
        provider = ClaudeProvider()
        assert provider.name == "Claude"

    def test_fetch_strategies_returns_list(self):
        """fetch_strategies returns list of strategies."""
        provider = ClaudeProvider()
        strategies = provider.fetch_strategies()

        assert isinstance(strategies, list)
        assert len(strategies) == 4

    def test_fetch_strategy_order(self):
        """Strategies are in correct priority order."""
        provider = ClaudeProvider()
        strategies = provider.fetch_strategies()

        # Should be OAuth, Web, Browser, CLI in that order
        assert "OAuth" in str(type(strategies[0]))
        assert "Web" in str(type(strategies[1]))
        assert "BrowserCookie" in str(type(strategies[2]))
        assert "CLI" in str(type(strategies[3]))

    def test_fetch_status(self):
        """fetch_status returns async operation."""
        provider = ClaudeProvider()

        # Should be async
        import inspect

        assert inspect.iscoroutinefunction(provider.fetch_status)

    @pytest.mark.asyncio
    async def test_fetch_status_returns_provider_status(self):
        """fetch_status returns ProviderStatus."""
        with patch(
            "vibeusage.providers.claude.status.fetch_statuspage_status"
        ) as mock_fetch:
            from vibeusage.models import ProviderStatus

            mock_fetch.return_value = ProviderStatus(level=StatusLevel.OPERATIONAL)

            provider = ClaudeProvider()
            result = await provider.fetch_status()

            assert isinstance(result, ProviderStatus)
            mock_fetch.assert_called_once()


class TestProviderRegistry:
    """Tests for provider registry functions."""

    def setup_method(self):
        """Clear registry before each test."""
        import vibeusage.providers as providers_module

        providers_module._PROVIDERS.clear()

    def teardown_method(self):
        """Restore registry after each test."""
        import vibeusage.providers as providers_module
        from vibeusage.providers.claude import ClaudeProvider
        from vibeusage.providers.codex import CodexProvider
        from vibeusage.providers.copilot import CopilotProvider
        from vibeusage.providers.cursor import CursorProvider
        from vibeusage.providers.gemini import GeminiProvider

        # Re-register the default providers
        providers_module._PROVIDERS["claude"] = ClaudeProvider
        providers_module._PROVIDERS["codex"] = CodexProvider
        providers_module._PROVIDERS["copilot"] = CopilotProvider
        providers_module._PROVIDERS["cursor"] = CursorProvider
        providers_module._PROVIDERS["gemini"] = GeminiProvider

    def test_register_provider_decorator(self):
        """register_provider decorator registers provider."""
        import vibeusage.providers as providers_module

        @register_provider
        class TestProvider(Provider):
            metadata = ProviderMetadata(
                id="test",
                name="Test",
                description="Test",
                homepage="https://example.com",
            )

            def fetch_strategies(self):
                return []

        assert "test" in providers_module._PROVIDERS

    def test_register_provider_requires_metadata(self):
        """register_provider raises ValueError if no metadata."""

        with pytest.raises(ValueError, match="must define metadata"):

            @register_provider
            class BadProvider:
                pass

    def test_get_provider_exists(self):
        """get_provider returns registered provider."""
        import vibeusage.providers as providers_module

        providers_module._PROVIDERS["test"] = Provider
        result = get_provider("test")
        assert result == Provider

    def test_get_provider_not_found(self):
        """get_provider returns None for unknown provider."""
        result = get_provider("unknown")
        assert result is None

    def test_get_all_providers(self):
        """get_all_providers returns all registered."""
        import vibeusage.providers as providers_module

        providers_module._PROVIDERS.clear()
        providers_module._PROVIDERS["claude"] = ClaudeProvider

        result = get_all_providers()
        assert "claude" in result
        assert isinstance(result, dict)

    def test_list_provider_ids(self):
        """list_provider_ids returns list of IDs."""
        import vibeusage.providers as providers_module

        providers_module._PROVIDERS.clear()
        providers_module._PROVIDERS["claude"] = ClaudeProvider
        providers_module._PROVIDERS["codex"] = Provider

        result = list_provider_ids()
        assert isinstance(result, list)
        assert "claude" in result
        assert "codex" in result

    def test_create_provider_exists(self):
        """create_provider creates instance."""
        import vibeusage.providers as providers_module

        providers_module._PROVIDERS["claude"] = ClaudeProvider
        instance = create_provider("claude")

        assert isinstance(instance, ClaudeProvider)
        assert instance.id == "claude"

    def test_create_provider_not_found(self):
        """create_provider raises ValueError for unknown provider."""
        with pytest.raises(ValueError, match="Unknown provider"):
            create_provider("nonexistent")


class TestClaudeProviderIntegration:
    """Integration tests for Claude provider with registry."""

    def test_claude_registered(self):
        """ClaudeProvider is registered in the registry."""
        from vibeusage.providers import get_provider

        provider_cls = get_provider("claude")
        assert provider_cls is not None
        assert provider_cls == ClaudeProvider

    def test_create_claude_provider(self):
        """Can create ClaudeProvider instance via registry."""
        from vibeusage.providers import create_provider

        provider = create_provider("claude")
        assert isinstance(provider, ClaudeProvider)

    def test_list_includes_claude(self):
        """list_provider_ids includes claude."""
        from vibeusage.providers import list_provider_ids

        ids = list_provider_ids()
        assert "claude" in ids


class TestProviderStatus:
    """Tests for provider status fetching."""

    @pytest.mark.asyncio
    async def test_default_status_unknown(self):
        """Provider base class returns unknown status by default."""
        from vibeusage.providers.base import Provider

        class TestProvider(Provider):
            metadata = ProviderMetadata(
                id="test",
                name="Test",
                description="Test",
                homepage="https://example.com",
            )

            def fetch_strategies(self):
                return []

        provider = TestProvider()
        status = await provider.fetch_status()

        assert status.level == StatusLevel.UNKNOWN

    @pytest.mark.asyncio
    async def test_claude_status_fetch(self):
        """Claude provider fetches status from statuspage."""
        with patch(
            "vibeusage.providers.claude.status.fetch_statuspage_status"
        ) as mock_fetch:
            from vibeusage.models import ProviderStatus

            mock_fetch.return_value = ProviderStatus(
                level=StatusLevel.OPERATIONAL,
                description="All systems operational",
                updated_at=datetime.now(UTC),
            )

            provider = ClaudeProvider()
            status = await provider.fetch_status()

            assert status.level == StatusLevel.OPERATIONAL
            mock_fetch.assert_called_once()


class TestProviderIsEnabled:
    """Tests for is_enabled method."""

    def test_is_enabled_uses_config(self):
        """is_enabled checks configuration."""
        with patch("vibeusage.config.settings.get_config") as mock_config:
            config = MagicMock()
            config.is_provider_enabled.return_value = True
            mock_config.return_value = config

            provider = ClaudeProvider()
            result = provider.is_enabled()

            assert result is True
            config.is_provider_enabled.assert_called_once_with("claude")

    def test_is_disabled_when_config_says_no(self):
        """is_enabled returns False when config disables."""
        with patch("vibeusage.config.settings.get_config") as mock_config:
            config = MagicMock()
            config.is_provider_enabled.return_value = False
            mock_config.return_value = config

            provider = ClaudeProvider()
            result = provider.is_enabled()

            assert result is False


class TestClaudeBrowserCookieStrategy:
    """Tests for ClaudeBrowserCookieStrategy."""

    def setup_method(self):
        """Import strategy class for testing."""
        from vibeusage.providers.claude.web import ClaudeBrowserCookieStrategy

        self.strategy_cls = ClaudeBrowserCookieStrategy

    def test_name_property(self):
        """Strategy has correct name."""
        strategy = self.strategy_cls()
        assert strategy.name == "browser"

    def test_cookie_domains(self):
        """COOKIE_DOMAINS includes expected values."""
        strategy = self.strategy_cls()
        assert "claude.ai" in str(strategy.COOKIE_DOMAINS)
        assert ".claude.ai" in str(strategy.COOKIE_DOMAINS)

    def test_cookie_names(self):
        """COOKIE_NAMES includes expected values."""
        strategy = self.strategy_cls()
        assert "sessionKey" in strategy.COOKIE_NAMES

    def test_is_available_always_returns_true(self):
        """is_available always returns True."""
        strategy = self.strategy_cls()
        assert strategy.is_available() is True

    def test_strategy_imports_browser_cookie3(self):
        """Strategy can import browser_cookie3 module."""
        # Verify browser_cookie3 is available as a dependency
        import browser_cookie3

        # Module should have browser attribute methods
        assert (
            hasattr(browser_cookie3, "chrome")
            or hasattr(browser_cookie3, "safari")
            or hasattr(browser_cookie3, "firefox")
        )

    @pytest.mark.asyncio
    async def test_fetch_fails_when_no_cookie_library(self):
        """fetch returns failure when browser_cookie3 not importable."""
        import builtins

        original_import = builtins.__import__

        def mock_import(name, *args, **kwargs):
            if name in ("browser_cookie3", "pycookiecheat"):
                raise ImportError(f"No module named '{name}'")
            return original_import(name, *args, **kwargs)

        strategy = self.strategy_cls()
        with patch("builtins.__import__", side_effect=mock_import):
            result = await strategy.fetch()

        assert result.success is False
        assert "browser_cookie3" in result.error or "pycookiecheat" in result.error

    @pytest.mark.asyncio
    async def test_fetch_extracts_cookie_and_delegates(self):
        """fetch extracts cookie from browser and delegates to WebStrategy."""
        strategy = self.strategy_cls()

        mock_cookie = MagicMock()
        mock_cookie.name = "sessionKey"
        mock_cookie.value = "sk-ant-sid01-extracted-key"

        mock_browser_cookie3 = MagicMock()
        mock_browser_cookie3.chrome.return_value = [mock_cookie]

        with patch.dict("sys.modules", {"browser_cookie3": mock_browser_cookie3}):
            with patch.object(strategy, "_save_session_key") as mock_save:
                with patch(
                    "vibeusage.providers.claude.web.ClaudeWebStrategy.fetch"
                ) as mock_web_fetch:
                    from vibeusage.strategies.base import FetchResult

                    mock_web_fetch.return_value = FetchResult(
                        success=True, snapshot=MagicMock()
                    )
                    result = await strategy.fetch()

        assert result.success is True
        mock_save.assert_called_once_with("sk-ant-sid01-extracted-key")

    @pytest.mark.asyncio
    async def test_fetch_tries_multiple_browsers(self):
        """fetch tries next browser when first fails."""
        strategy = self.strategy_cls()

        mock_cookie = MagicMock()
        mock_cookie.name = "sessionKey"
        mock_cookie.value = "sk-ant-sid01-from-firefox"

        mock_browser_cookie3 = MagicMock()
        mock_browser_cookie3.safari = None  # Safari not available
        mock_browser_cookie3.chrome.side_effect = Exception("Chrome locked")
        mock_browser_cookie3.firefox.return_value = [mock_cookie]

        with patch.dict("sys.modules", {"browser_cookie3": mock_browser_cookie3}):
            with patch.object(strategy, "_save_session_key"):
                with patch(
                    "vibeusage.providers.claude.web.ClaudeWebStrategy.fetch"
                ) as mock_web_fetch:
                    from vibeusage.strategies.base import FetchResult

                    mock_web_fetch.return_value = FetchResult(
                        success=True, snapshot=MagicMock()
                    )
                    result = await strategy.fetch()

        assert result.success is True

    @pytest.mark.asyncio
    async def test_fetch_fails_when_no_cookie_found(self):
        """fetch returns failure when no matching cookie found."""
        strategy = self.strategy_cls()

        mock_browser_cookie3 = MagicMock()
        mock_browser_cookie3.safari = None
        mock_browser_cookie3.chrome.return_value = []  # No cookies
        mock_browser_cookie3.firefox.return_value = []
        mock_browser_cookie3.brave = None
        mock_browser_cookie3.edge = None

        with patch.dict("sys.modules", {"browser_cookie3": mock_browser_cookie3}):
            result = await strategy.fetch()

        assert result.success is False
        assert "Could not extract" in result.error

    @pytest.mark.asyncio
    async def test_fetch_ignores_wrong_cookie_names(self):
        """fetch ignores cookies that don't match expected names."""
        strategy = self.strategy_cls()

        wrong_cookie = MagicMock()
        wrong_cookie.name = "otherCookie"
        wrong_cookie.value = "not-what-we-want"

        mock_browser_cookie3 = MagicMock()
        mock_browser_cookie3.safari = None
        mock_browser_cookie3.chrome.return_value = [wrong_cookie]
        mock_browser_cookie3.firefox.return_value = []
        mock_browser_cookie3.brave = None
        mock_browser_cookie3.edge = None

        with patch.dict("sys.modules", {"browser_cookie3": mock_browser_cookie3}):
            result = await strategy.fetch()

        assert result.success is False

    def test_save_session_key_writes_json(self):
        """_save_session_key writes JSON to correct path."""
        strategy = self.strategy_cls()

        with patch("vibeusage.providers.claude.web.write_credential") as mock_write:
            strategy._save_session_key("sk-ant-sid01-test-key")

            mock_write.assert_called_once()
            args = mock_write.call_args
            written_data = args[0][1]
            import json

            parsed = json.loads(written_data)
            assert parsed["session_key"] == "sk-ant-sid01-test-key"
