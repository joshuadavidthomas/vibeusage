"""Network error classification utilities.

This module provides network-specific error handling and classification.
"""

from __future__ import annotations

import httpx

from vibeusage.errors.types import ErrorCategory, ErrorSeverity, VibeusageError
from vibeusage.errors.types import classify_http_error as _classify_http_error


def classify_network_error(error: Exception) -> VibeusageError:
    """Classify network-related errors into structured errors.

    Args:
        error: Network exception to classify

    Returns:
        VibeusageError with appropriate category and remediation
    """
    if isinstance(error, httpx.ConnectTimeout):
        return VibeusageError(
            message="Connection timed out",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            remediation="Check your internet connection and try again.",
        )

    if isinstance(error, httpx.ReadTimeout):
        return VibeusageError(
            message="Request timed out waiting for response",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            remediation="The provider may be slow. Try again or check provider status.",
        )

    if isinstance(error, httpx.WriteTimeout):
        return VibeusageError(
            message="Request timed out while sending data",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            remediation="Check your internet connection and try again.",
        )

    if isinstance(error, httpx.ConnectError):
        # Get more specific error message if available
        message = str(error)
        if "connection refused" in message.lower():
            return VibeusageError(
                message="Connection refused by server",
                category=ErrorCategory.NETWORK,
                severity=ErrorSeverity.TRANSIENT,
                remediation="The provider service may be down. Check their status page.",
            )
        elif "dns" in message.lower() or "hostname" in message.lower():
            return VibeusageError(
                message="Could not resolve server address",
                category=ErrorCategory.NETWORK,
                severity=ErrorSeverity.TRANSIENT,
                remediation="Check your internet connection and DNS settings.",
            )
        else:
            return VibeusageError(
                message="Failed to connect to server",
                category=ErrorCategory.NETWORK,
                severity=ErrorSeverity.TRANSIENT,
                remediation="Check your internet connection. The provider may be down.",
            )

    if isinstance(error, httpx.NetworkError):
        return VibeusageError(
            message=f"Network error: {error}",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            remediation="Check your internet connection and try again.",
        )

    if isinstance(error, httpx.HTTPStatusError):
        return classify_http_status_error(error)

    # Fallback for unexpected error types
    return VibeusageError(
        message=f"Network error: {error}",
        category=ErrorCategory.NETWORK,
        severity=ErrorSeverity.TRANSIENT,
        remediation="Check your internet connection and try again.",
    )


def classify_http_status_error(error: httpx.HTTPStatusError) -> VibeusageError:
    """Classify HTTP status errors into structured errors.

    Args:
        error: HTTPStatusError to classify

    Returns:
        VibeusageError with category, severity, and remediation
    """
    from vibeusage.errors.http import extract_error_message

    status = error.response.status_code
    mapping = _classify_http_error(status)

    # Extract meaningful error message
    detail = extract_error_message(error.response)
    message = f"HTTP {status}: {detail}"

    # Build remediation based on status
    remediation = None
    if status == 401:
        remediation = "Your credentials may have expired. Run 'vibeusage auth <provider>' to re-authenticate."
    elif status == 403:
        remediation = "Your account may not have access to this resource. Check your subscription status."
    elif status == 404:
        remediation = (
            "The requested resource was not found. The provider API may have changed."
        )
    elif status == 429:
        remediation = "Rate limited. Wait a few minutes before trying again."
    elif status >= 500:
        remediation = "The provider service is experiencing issues. Try again later."

    return VibeusageError(
        message=message,
        category=mapping.category,
        severity=mapping.severity,
        details={"status_code": status, "response": detail},
        remediation=remediation,
    )


def is_network_error(error: Exception) -> bool:
    """Check if an exception is a network-related error.

    Args:
        error: Exception to check

    Returns:
        True if this is a network error
    """
    return isinstance(
        error,
        (
            httpx.NetworkError,
            httpx.TimeoutException,
            httpx.ConnectError,
            httpx.ConnectTimeout,
            httpx.ReadTimeout,
            httpx.WriteTimeout,
        ),
    )


def is_retryable_error(error: Exception) -> bool:
    """Check if an error is retryable (transient).

    Args:
        error: Exception to check

    Returns:
        True if this error should trigger a retry
    """
    # Network errors are generally retryable
    if is_network_error(error):
        return True

    # Some HTTP status codes are retryable
    if isinstance(error, httpx.HTTPStatusError):
        status = error.response.status_code
        mapping = _classify_http_error(status)
        return mapping.should_retry

    return False
