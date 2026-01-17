"""Error handling for vibeusage."""
from __future__ import annotations

from vibeusage.errors.classify import classify_exception
from vibeusage.errors.http import extract_error_message
from vibeusage.errors.http import get_retry_after_delay
from vibeusage.errors.http import handle_http_request
from vibeusage.errors.messages import AUTH_ERROR_TEMPLATES
from vibeusage.errors.messages import get_auth_error_message
from vibeusage.errors.messages import get_provider_remediation
from vibeusage.errors.network import classify_http_status_error
from vibeusage.errors.network import classify_network_error
from vibeusage.errors.network import is_network_error
from vibeusage.errors.network import is_retryable_error
from vibeusage.errors.types import HTTP_ERROR_MAPPINGS
from vibeusage.errors.types import ErrorCategory
from vibeusage.errors.types import ErrorSeverity
from vibeusage.errors.types import HTTPErrorMapping
from vibeusage.errors.types import VibeusageError
from vibeusage.errors.types import classify_http_error

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
