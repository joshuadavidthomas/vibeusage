"""HTTP client with connection pooling for vibeusage."""

import asyncio
from contextlib import asynccontextmanager

import httpx

from vibeusage.config.settings import get_config

# Global HTTP client
_client: httpx.AsyncClient | None = None


def get_timeout_config() -> httpx.Timeout:
    """Get timeout configuration from settings."""
    config = get_config()
    timeout = config.fetch.timeout
    return httpx.Timeout(timeout, connect=10.0)


@asynccontextmanager
async def get_http_client():
    """Get or create the shared HTTP client.

    Usage:
        async with get_http_client() as client:
            response = await client.get(...)
    """
    global _client

    if _client is None:
        limits = httpx.Limits(
            max_connections=20,
            max_keepalive_connections=5,
        )
        _client = httpx.AsyncClient(
            timeout=get_timeout_config(),
            limits=limits,
            follow_redirects=True,
        )

    try:
        yield _client
    finally:
        # Don't close - keep for reuse
        pass


async def cleanup() -> None:
    """Close the HTTP client.

    Should be called on application shutdown.
    """
    global _client
    if _client is not None:
        await _client.aclose()
        _client = None


async def fetch_url(url: str, headers: dict | None = None) -> bytes | None:
    """Simple fetch function for getting URL content.

    Returns:
        Response body as bytes, or None on error
    """
    try:
        async with get_http_client() as client:
            response = await client.get(url, headers=headers)
            response.raise_for_status()
            return response.content
    except httpx.HTTPError:
        return None
