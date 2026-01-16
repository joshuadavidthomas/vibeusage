"""Error types and classifications."""

from __future__ import annotations

from datetime import datetime
from enum import StrEnum

import msgspec


class ErrorCategory(StrEnum):
    """Error categories for handling decisions."""

    AUTHENTICATION = "authentication"
    AUTHORIZATION = "authorization"
    RATE_LIMITED = "rate_limited"
    NETWORK = "network"
    PROVIDER = "provider"
    PARSE = "parse"
    CONFIGURATION = "configuration"
    NOT_FOUND = "not_found"
    UNKNOWN = "unknown"


class ErrorSeverity(StrEnum):
    """Error severity levels."""

    FATAL = "fatal"
    RECOVERABLE = "recoverable"
    TRANSIENT = "transient"
    WARNING = "warning"


class VibeusageError(msgspec.Struct, frozen=True):
    """Structured error with category and remediation."""

    message: str
    category: ErrorCategory
    severity: ErrorSeverity
    provider: str | None = None
    remediation: str | None = None
    details: dict | None = None
    timestamp: datetime = msgspec.field(
        default_factory=lambda: datetime.now().astimezone()
    )


class HTTPErrorMapping(msgspec.Struct, frozen=True):
    """How to handle an HTTP status code."""

    category: ErrorCategory
    severity: ErrorSeverity
    should_retry: bool = False
    should_fallback: bool = True
    retry_after_header: bool = False


HTTP_ERROR_MAPPINGS: dict[int, HTTPErrorMapping] = {
    # Authentication errors
    401: HTTPErrorMapping(
        category=ErrorCategory.AUTHENTICATION,
        severity=ErrorSeverity.RECOVERABLE,
        should_fallback=True,
    ),
    # Authorization errors
    403: HTTPErrorMapping(
        category=ErrorCategory.AUTHORIZATION,
        severity=ErrorSeverity.RECOVERABLE,
        should_fallback=True,
    ),
    # Not found
    404: HTTPErrorMapping(
        category=ErrorCategory.NOT_FOUND,
        severity=ErrorSeverity.RECOVERABLE,
        should_fallback=True,
    ),
    # Rate limiting
    429: HTTPErrorMapping(
        category=ErrorCategory.RATE_LIMITED,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=False,
        retry_after_header=True,
    ),
    # Server errors
    500: HTTPErrorMapping(
        category=ErrorCategory.PROVIDER,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=True,
    ),
    502: HTTPErrorMapping(
        category=ErrorCategory.PROVIDER,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=True,
    ),
    503: HTTPErrorMapping(
        category=ErrorCategory.PROVIDER,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=True,
    ),
    504: HTTPErrorMapping(
        category=ErrorCategory.PROVIDER,
        severity=ErrorSeverity.TRANSIENT,
        should_retry=True,
        should_fallback=True,
    ),
}


def classify_http_error(
    status_code: int,
    response_body: str | None = None,
) -> HTTPErrorMapping:
    """Classify an HTTP error by status code."""
    if status_code in HTTP_ERROR_MAPPINGS:
        return HTTP_ERROR_MAPPINGS[status_code]

    # Default mapping for unrecognized codes
    if 400 <= status_code < 500:
        return HTTPErrorMapping(
            category=ErrorCategory.UNKNOWN,
            severity=ErrorSeverity.RECOVERABLE,
            should_fallback=True,
        )
    elif 500 <= status_code < 600:
        return HTTPErrorMapping(
            category=ErrorCategory.PROVIDER,
            severity=ErrorSeverity.TRANSIENT,
            should_retry=True,
            should_fallback=True,
        )
    else:
        return HTTPErrorMapping(
            category=ErrorCategory.UNKNOWN,
            severity=ErrorSeverity.RECOVERABLE,
        )
