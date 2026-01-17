"""Error handling for vibeusage."""

from vibeusage.errors.classify import classify_exception
from vibeusage.errors.http import (
    extract_error_message,
    get_retry_after_delay,
    handle_http_request,
)
from vibeusage.errors.messages import (
    AUTH_ERROR_TEMPLATES,
    get_auth_error_message,
    get_provider_remediation,
)
from vibeusage.errors.network import (
    classify_http_status_error,
    classify_network_error,
    is_network_error,
    is_retryable_error,
)
from vibeusage.errors.types import (
    ErrorCategory,
    ErrorSeverity,
    VibeusageError,
    HTTPErrorMapping,
    HTTP_ERROR_MAPPINGS,
    classify_http_error,
)

__all__ = [
    # Core types
    "ErrorCategory",
    "ErrorSeverity",
    "VibeusageError",
    "HTTPErrorMapping",
    "HTTP_ERROR_MAPPINGS",
    # Classification functions
    "classify_http_error",
    "classify_exception",
    "classify_network_error",
    "classify_http_status_error",
    # HTTP utilities
    "handle_http_request",
    "extract_error_message",
    "get_retry_after_delay",
    # Network utilities
    "is_network_error",
    "is_retryable_error",
    # Message templates
    "AUTH_ERROR_TEMPLATES",
    "get_auth_error_message",
    "get_provider_remediation",
]
