"""Exception classification for structured error handling."""

from __future__ import annotations

import asyncio
import json
from pathlib import Path

import httpx

from vibeusage.errors.types import (
    ErrorCategory,
    ErrorSeverity,
    VibeusageError,
    classify_http_error,
)


def classify_http_status_error(error: httpx.HTTPStatusError) -> VibeusageError:
    """Classify HTTP status errors into structured errors."""

    status = error.response.status_code
    mapping = classify_http_error(status)

    # Try to extract error message from response
    try:
        body = error.response.json()
        detail = body.get("error", body.get("message", str(status)))
    except Exception:
        detail = error.response.text[:200] if error.response.text else str(status)

    message = f"HTTP {status}: {detail}"

    return VibeusageError(
        message=message,
        category=mapping.category,
        severity=mapping.severity,
        details={"status_code": status, "response": detail},
    )


def classify_exception(
    e: Exception,
    provider_id: str | None = None,
) -> VibeusageError:
    """Classify any exception into a structured error."""

    # Network errors - httpx specific
    if isinstance(e, httpx.TimeoutException):
        return VibeusageError(
            message="Request timed out",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            provider=provider_id,
            remediation="Check your network connection and try again.",
        )

    if isinstance(e, httpx.ConnectError):
        return VibeusageError(
            message="Failed to connect to server",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            provider=provider_id,
            remediation="Check your internet connection. The provider may be down.",
        )

    if isinstance(e, httpx.HTTPStatusError):
        error = classify_http_status_error(e)
        if provider_id is not None:
            return VibeusageError(
                message=error.message,
                category=error.category,
                severity=error.severity,
                provider=provider_id,
                remediation=error.remediation,
                details=error.details,
            )
        return error

    # Parse errors
    if isinstance(e, json.JSONDecodeError):
        return VibeusageError(
            message="Failed to parse response",
            category=ErrorCategory.PARSE,
            severity=ErrorSeverity.RECOVERABLE,
            provider=provider_id,
            details={"error": str(e)},
        )

    if isinstance(e, (KeyError, ValueError, TypeError)):
        return VibeusageError(
            message=f"Invalid response format: {e}",
            category=ErrorCategory.PARSE,
            severity=ErrorSeverity.RECOVERABLE,
            provider=provider_id,
        )

    # Async errors
    if isinstance(e, asyncio.TimeoutError):
        return VibeusageError(
            message="Operation timed out",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            provider=provider_id,
            remediation="Try again. If the issue persists, check provider status.",
        )

    if isinstance(e, asyncio.CancelledError):
        return VibeusageError(
            message="Operation cancelled",
            category=ErrorCategory.UNKNOWN,
            severity=ErrorSeverity.RECOVERABLE,
            provider=provider_id,
        )

    # File errors
    if isinstance(e, FileNotFoundError):
        filename = getattr(e, "filename", None)
        return VibeusageError(
            message=f"File not found: {filename}" if filename else "File not found",
            category=ErrorCategory.CONFIGURATION,
            severity=ErrorSeverity.RECOVERABLE,
            provider=provider_id,
        )

    if isinstance(e, PermissionError):
        filename = getattr(e, "filename", None)
        return VibeusageError(
            message=f"Permission denied: {filename}"
            if filename
            else "Permission denied",
            category=ErrorCategory.CONFIGURATION,
            severity=ErrorSeverity.FATAL,
            provider=provider_id,
            remediation="Check file permissions for vibeusage config directory.",
        )

    # Unknown
    return VibeusageError(
        message=str(e),
        category=ErrorCategory.UNKNOWN,
        severity=ErrorSeverity.RECOVERABLE,
        provider=provider_id,
        details={"type": type(e).__name__},
    )
