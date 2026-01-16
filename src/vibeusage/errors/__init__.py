"""Error handling for vibeusage."""

from vibeusage.errors.types import (
    ErrorCategory,
    ErrorSeverity,
    VibeusageError,
    HTTPErrorMapping,
    HTTP_ERROR_MAPPINGS,
    classify_http_error,
)

__all__ = [
    "ErrorCategory",
    "ErrorSeverity",
    "VibeusageError",
    "HTTPErrorMapping",
    "HTTP_ERROR_MAPPINGS",
    "classify_http_error",
]
