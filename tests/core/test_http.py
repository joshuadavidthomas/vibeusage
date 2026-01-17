"""Tests for core/http.py (HTTP client with connection pooling)."""

from __future__ import annotations

from unittest.mock import AsyncMock
from unittest.mock import Mock
from unittest.mock import patch

import httpx
import pytest

from vibeusage.config.settings import Config
from vibeusage.config.settings import FetchConfig
from vibeusage.core.http import cleanup
from vibeusage.core.http import fetch_url
from vibeusage.core.http import get_http_client
from vibeusage.core.http import get_timeout_config


class TestGetTimeoutConfig:
    """Tests for get_timeout_config function."""

    def test_returns_timeout_spec_with_defaults(self):
        """Returns timeout spec with default values from config."""
        with patch("vibeusage.core.http.get_config") as mock_get_config:
            mock_get_config.return_value = Config(
                fetch=FetchConfig(timeout=30.0)
            )

            timeout = get_timeout_config()

            assert isinstance(timeout, httpx.Timeout)
            assert timeout.connect == 10.0

    def test_uses_config_timeout_value(self):
        """Uses the timeout value from configuration."""
        with patch("vibeusage.core.http.get_config") as mock_get_config:
            mock_get_config.return_value = Config(
                fetch=FetchConfig(timeout=45.0)
            )

            timeout = get_timeout_config()

            # The timeout from config is used as the read timeout
            assert timeout.read == 45.0
            assert timeout.connect == 10.0

    def test_uses_config_timeout_for_write_and_connect(self):
        """Timeout spec uses config timeout for multiple operations."""
        with patch("vibeusage.core.http.get_config") as mock_get_config:
            mock_get_config.return_value = Config(
                fetch=FetchConfig(timeout=60.0)
            )

            timeout = get_timeout_config()

            # httpx.Timeout(timeout=..., connect=10) sets read/write/pool to timeout
            assert timeout.read == 60.0
            assert timeout.write == 60.0
            assert timeout.pool == 60.0
            assert timeout.connect == 10.0


class TestGetHttpClient:
    """Tests for get_http_client context manager."""

    @pytest.mark.asyncio
    async def test_creates_new_client_on_first_call(self):
        """Creates a new httpx.AsyncClient on first call."""
        # Reset global client
        import vibeusage.core.http
        vibeusage.core.http._client = None

        async with get_http_client() as client:
            assert client is not None
            assert isinstance(client, httpx.AsyncClient)
            # Client should have been created
            assert vibeusage.core.http._client is not None

    @pytest.mark.asyncio
    async def test_reuses_existing_client(self):
        """Reuses existing client instead of creating new one."""
        # Reset global client
        import vibeusage.core.http
        vibeusage.core.http._client = None

        async with get_http_client() as client1:
            async with get_http_client() as client2:
                # Should be the same client instance
                assert client1 is client2

    @pytest.mark.asyncio
    async def test_client_has_follow_redirects_enabled(self):
        """Client is configured with follow_redirects=True."""
        # Reset global client
        import vibeusage.core.http
        vibeusage.core.http._client = None

        async with get_http_client() as client:
            assert client.follow_redirects is True

    @pytest.mark.asyncio
    async def test_client_has_timeout_configured(self):
        """Client is configured with timeout from settings."""
        # Reset global client
        import vibeusage.core.http
        vibeusage.core.http._client = None

        with patch("vibeusage.core.http.get_timeout_config") as mock_timeout:
            mock_timeout.return_value = httpx.Timeout(30.0, connect=10.0)

            async with get_http_client() as client:
                assert client.timeout is not None
                mock_timeout.assert_called_once()

    @pytest.mark.asyncio
    async def test_does_not_close_client_on_exit(self):
        """Does not close the client on context exit (for reuse)."""
        # Reset global client
        import vibeusage.core.http
        vibeusage.core.http._client = None

        async with get_http_client() as client1:
            # Exit context
            pass

        # Client should still be usable
        async with get_http_client() as client2:
            assert client2 is not None
            assert client2 is client1


class TestCleanup:
    """Tests for cleanup function."""

    @pytest.mark.asyncio
    async def test_closes_existing_client(self):
        """Closes the HTTP client when it exists."""
        # Create a client first
        import vibeusage.core.http
        vibeusage.core.http._client = None

        async with get_http_client():
            pass

        # Client should exist now
        assert vibeusage.core.http._client is not None

        # Cleanup should close it
        await cleanup()

        assert vibeusage.core.http._client is None

    @pytest.mark.asyncio
    async def test_cleanup_when_no_client_exists(self):
        """Does not error when no client exists."""
        # Reset global client
        import vibeusage.core.http
        vibeusage.core.http._client = None

        # Should not raise
        await cleanup()

        assert vibeusage.core.http._client is None

    @pytest.mark.asyncio
    async def test_can_create_new_client_after_cleanup(self):
        """Can create a new client after cleanup."""
        import vibeusage.core.http
        vibeusage.core.http._client = None

        # Create and cleanup
        async with get_http_client():
            pass
        await cleanup()

        # Should be able to create again
        async with get_http_client() as client:
            assert client is not None


class TestFetchUrl:
    """Tests for fetch_url function."""

    @pytest.mark.asyncio
    async def test_fetches_url_successfully(self):
        """Fetches URL content and returns bytes."""
        mock_response = Mock()
        mock_response.content = b"<html>test content</html>"
        mock_response.raise_for_status = Mock()

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(return_value=mock_response)
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock()

        with patch("vibeusage.core.http.get_http_client") as mock_get_client:
            mock_get_client.return_value = mock_client

            result = await fetch_url("https://example.com")

            assert result == b"<html>test content</html>"
            mock_client.get.assert_called_once_with(
                "https://example.com", headers=None
            )

    @pytest.mark.asyncio
    async def test_fetches_url_with_headers(self):
        """Fetches URL with custom headers."""
        mock_response = Mock()
        mock_response.content = b"response"
        mock_response.raise_for_status = Mock()

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(return_value=mock_response)
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock()

        with patch("vibeusage.core.http.get_http_client") as mock_get_client:
            mock_get_client.return_value = mock_client

            headers = {"Authorization": "Bearer token"}
            result = await fetch_url("https://example.com", headers=headers)

            assert result == b"response"
            mock_client.get.assert_called_once_with(
                "https://example.com", headers=headers
            )

    @pytest.mark.asyncio
    async def test_returns_none_on_http_error(self):
        """Returns None when HTTP request fails."""
        mock_client = AsyncMock()
        mock_client.get = AsyncMock(side_effect=httpx.HTTPError("Network error"))
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock()

        with patch("vibeusage.core.http.get_http_client") as mock_get_client:
            mock_get_client.return_value = mock_client

            result = await fetch_url("https://example.com")

            assert result is None

    @pytest.mark.asyncio
    async def test_returns_none_on_status_error(self):
        """Returns None when response status indicates error."""
        mock_response = Mock()
        mock_response.content = None
        mock_response.raise_for_status = Mock(
            side_effect=httpx.HTTPStatusError(
                "404 Not Found", request=None, response=None
            )
        )

        mock_client = AsyncMock()
        mock_client.get = AsyncMock(return_value=mock_response)
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock()

        with patch("vibeusage.core.http.get_http_client") as mock_get_client:
            mock_get_client.return_value = mock_client

            result = await fetch_url("https://example.com")

            assert result is None

    @pytest.mark.asyncio
    async def test_returns_none_on_timeout(self):
        """Returns None when request times out."""
        mock_client = AsyncMock()
        mock_client.get = AsyncMock(side_effect=httpx.TimeoutException("Timeout"))
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock()

        with patch("vibeusage.core.http.get_http_client") as mock_get_client:
            mock_get_client.return_value = mock_client

            result = await fetch_url("https://example.com")

            assert result is None

    @pytest.mark.asyncio
    async def test_returns_none_on_connection_error(self):
        """Returns None on connection errors."""
        mock_client = AsyncMock()
        mock_client.get = AsyncMock(side_effect=httpx.ConnectError("Connection failed"))
        mock_client.__aenter__ = AsyncMock(return_value=mock_client)
        mock_client.__aexit__ = AsyncMock()

        with patch("vibeusage.core.http.get_http_client") as mock_get_client:
            mock_get_client.return_value = mock_client

            result = await fetch_url("https://example.com")

            assert result is None
