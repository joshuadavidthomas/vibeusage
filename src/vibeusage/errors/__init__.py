"""Error handling for vibeusage."""

from vibeusage.errors.classify import classify_exception
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
    "classify_exception",
]
