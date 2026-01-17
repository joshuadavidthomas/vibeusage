"""HTTP error classification and handling utilities.

This module provides HTTP-specific error handling on top of the base
classifications in errors/types.py.
"""

from __future__ import annotations

import asyncio
from typing import Callable

import httpx

from vibeusage.errors.types import HTTPErrorMapping, classify_http_error


async def handle_http_request(
    client: httpx.AsyncClient,
    method: str,
    url: str,
    *,
    max_retries: int = 3,
    base_delay: float = 1.0,
    on_retry: Callable[[int, float], None] | None = None,
    **kwargs,
) -> httpx.Response:
    """Make an HTTP request with automatic retry for transient errors.

    Args:
        client: HTTP client
        method: HTTP method (GET, POST, etc.)
        url: Request URL
        max_retries: Maximum retry attempts
        base_delay: Base delay for exponential backoff (seconds)
        on_retry: Callback for retry notification (attempt, delay)
        **kwargs: Additional arguments passed to client.request

    Returns:
        HTTP response

    Raises:
        httpx.HTTPStatusError: If all retries exhausted or non-retryable error
        httpx.NetworkError: If connection fails after all retries
    """
    import random

    last_error: httpx.HTTPStatusError | httpx.NetworkError | None = None

    for attempt in range(max_retries + 1):
        try:
            response = await client.request(method, url, **kwargs)
            response.raise_for_status()
            return response

        except httpx.HTTPStatusError as e:
            last_error = e
            mapping = classify_http_error(e.response.status_code)

            if not mapping.should_retry or attempt >= max_retries:
                raise

            # Calculate delay with exponential backoff
            delay = base_delay * (2 ** attempt)

            # Add jitter to prevent thundering herd
            delay = delay * (1.0 + random.random() * 0.25)

            # Check Retry-After header
            if mapping.retry_after_header:
                retry_after = e.response.headers.get("Retry-After")
                if retry_after:
                    try:
                        delay = max(delay, float(retry_after))
                    except ValueError:
                        pass  # Ignore invalid header

            if on_retry:
                on_retry(attempt + 1, delay)

            await asyncio.sleep(delay)

        except (httpx.ConnectError, httpx.TimeoutException, httpx.NetworkError) as e:
            last_error = e

            if attempt >= max_retries:
                raise

            # Calculate delay with exponential backoff
            delay = base_delay * (2 ** attempt)
            delay = delay * (1.0 + random.random() * 0.25)

            if on_retry:
                on_retry(attempt + 1, delay)

            await asyncio.sleep(delay)

    # Should not reach here, but just in case
    if last_error:
        raise last_error
    raise RuntimeError("Unexpected state in retry loop")


def extract_error_message(response: httpx.Response) -> str:
    """Extract a meaningful error message from an HTTP response.

    Args:
        response: HTTP response with error status

    Returns:
        Extracted error message
    """
    status = response.status_code

    # Try JSON response first
    try:
        body = response.json()
        if isinstance(body, dict):
            # Common error field names
            for key in ("error", "message", "detail", "error_description"):
                if key in body:
                    value = body[key]
                    if isinstance(value, str):
                        return value
                    elif isinstance(value, dict):
                        # Some APIs nest the message
                        for nested_key in ("message", "description"):
                            if nested_key in value:
                                return str(value[nested_key])
    except Exception:
        pass

    # Fall back to text content
    text = response.text.strip()
    if text and len(text) < 200:
        return text

    # Default to status code
    return f"HTTP {status}"


def get_retry_after_delay(response: httpx.Response, default_delay: float = 1.0) -> float:
    """Get the delay from a Retry-After header.

    Args:
        response: HTTP response
        default_delay: Default delay if header is missing/invalid

    Returns:
        Delay in seconds
    """
    retry_after = response.headers.get("Retry-After")
    if not retry_after:
        return default_delay

    try:
        # Try as integer (seconds)
        return float(retry_after)
    except ValueError:
        # Could be a date - for now just use default
        # A full implementation would parse HTTP-date format
        return default_delay
