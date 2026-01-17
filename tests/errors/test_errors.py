"""Tests for error classification and handling."""

import asyncio
import json
from datetime import datetime, timezone
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
import httpx

from vibeusage.errors.classify import classify_exception, classify_http_status_error
from vibeusage.errors.http import (
    extract_error_message,
    get_retry_after_delay,
    handle_http_request,
)
from vibeusage.errors.network import (
    classify_http_status_error as classify_http_status_error_network,
    classify_network_error,
    is_network_error,
    is_retryable_error,
)
from vibeusage.errors.types import (
    ErrorCategory,
    ErrorSeverity,
    HTTPErrorMapping,
    HTTP_ERROR_MAPPINGS,
    VibeusageError,
    classify_http_error,
)
from vibeusage.models import StatusLevel


class TestVibeusageError:
    """Tests for VibeusageError."""

    def test_create_error(self):
        """Can create a VibeusageError."""
        error = VibeusageError(
            message="Something went wrong",
            category=ErrorCategory.UNKNOWN,
            severity=ErrorSeverity.RECOVERABLE,
        )

        assert error.message == "Something went wrong"
        assert error.category == ErrorCategory.UNKNOWN
        assert error.severity == ErrorSeverity.RECOVERABLE
        assert error.provider is None
        assert error.remediation is None
        assert error.details is None

    def test_create_full_error(self):
        """Can create error with all fields."""
        now = datetime.now(timezone.utc)
        error = VibeusageError(
            message="Auth failed",
            category=ErrorCategory.AUTHENTICATION,
            severity=ErrorSeverity.FATAL,
            provider="claude",
            remediation="Re-authenticate",
            details={"status_code": 401},
            timestamp=now,
        )

        assert error.message == "Auth failed"
        assert error.category == ErrorCategory.AUTHENTICATION
        assert error.severity == ErrorSeverity.FATAL
        assert error.provider == "claude"
        assert error.remediation == "Re-authenticate"
        assert error.details == {"status_code": 401}
        assert error.timestamp == now

    def test_default_timestamp(self):
        """Default timestamp is current time."""
        before = datetime.now(timezone.utc)
        error = VibeusageError(
            message="Test",
            category=ErrorCategory.UNKNOWN,
            severity=ErrorSeverity.WARNING,
        )
        after = datetime.now(timezone.utc)

        assert before <= error.timestamp <= after

    def test_immutability(self):
        """VibeusageError is immutable."""
        error = VibeusageError(
            message="Test",
            category=ErrorCategory.UNKNOWN,
            severity=ErrorSeverity.WARNING,
        )
        with pytest.raises(AttributeError):
            error.message = "Changed"


class TestErrorCategory:
    """Tests for ErrorCategory enum."""

    def test_values(self):
        """ErrorCategory has correct values."""
        assert ErrorCategory.AUTHENTICATION == "authentication"
        assert ErrorCategory.AUTHORIZATION == "authorization"
        assert ErrorCategory.RATE_LIMITED == "rate_limited"
        assert ErrorCategory.NETWORK == "network"
        assert ErrorCategory.PROVIDER == "provider"
        assert ErrorCategory.PARSE == "parse"
        assert ErrorCategory.CONFIGURATION == "configuration"
        assert ErrorCategory.NOT_FOUND == "not_found"
        assert ErrorCategory.UNKNOWN == "unknown"


class TestErrorSeverity:
    """Tests for ErrorSeverity enum."""

    def test_values(self):
        """ErrorSeverity has correct values."""
        assert ErrorSeverity.FATAL == "fatal"
        assert ErrorSeverity.RECOVERABLE == "recoverable"
        assert ErrorSeverity.TRANSIENT == "transient"
        assert ErrorSeverity.WARNING == "warning"


class TestHTTPErrorMapping:
    """Tests for HTTPErrorMapping."""

    def test_create_mapping(self):
        """Can create HTTPErrorMapping."""
        mapping = HTTPErrorMapping(
            category=ErrorCategory.AUTHENTICATION,
            severity=ErrorSeverity.RECOVERABLE,
            should_retry=False,
            should_fallback=True,
        )

        assert mapping.category == ErrorCategory.AUTHENTICATION
        assert mapping.severity == ErrorSeverity.RECOVERABLE
        assert mapping.should_retry is False
        assert mapping.should_fallback is True
        assert mapping.retry_after_header is False

    def test_mapping_with_retry_after(self):
        """Can create mapping with retry_after_header."""
        mapping = HTTPErrorMapping(
            category=ErrorCategory.RATE_LIMITED,
            severity=ErrorSeverity.TRANSIENT,
            should_retry=True,
            should_fallback=False,
            retry_after_header=True,
        )

        assert mapping.retry_after_header is True


class TestHTTPErrorMappings:
    """Tests for HTTP_ERROR_MAPPINGS."""

    def test_401_mapping(self):
        """401 is mapped to authentication error."""
        mapping = HTTP_ERROR_MAPPINGS[401]
        assert mapping.category == ErrorCategory.AUTHENTICATION
        assert mapping.severity == ErrorSeverity.RECOVERABLE
        assert mapping.should_retry is False
        assert mapping.should_fallback is True

    def test_403_mapping(self):
        """403 is mapped to authorization error."""
        mapping = HTTP_ERROR_MAPPINGS[403]
        assert mapping.category == ErrorCategory.AUTHORIZATION
        assert mapping.severity == ErrorSeverity.RECOVERABLE
        assert mapping.should_fallback is True

    def test_404_mapping(self):
        """404 is mapped to not_found."""
        mapping = HTTP_ERROR_MAPPINGS[404]
        assert mapping.category == ErrorCategory.NOT_FOUND

    def test_429_mapping(self):
        """429 is mapped to rate_limited with retry."""
        mapping = HTTP_ERROR_MAPPINGS[429]
        assert mapping.category == ErrorCategory.RATE_LIMITED
        assert mapping.severity == ErrorSeverity.TRANSIENT
        assert mapping.should_retry is True
        assert mapping.should_fallback is False
        assert mapping.retry_after_header is True

    def test_500_mapping(self):
        """500 is mapped to provider error with retry."""
        mapping = HTTP_ERROR_MAPPINGS[500]
        assert mapping.category == ErrorCategory.PROVIDER
        assert mapping.severity == ErrorSeverity.TRANSIENT
        assert mapping.should_retry is True
        assert mapping.should_fallback is True

    def test_502_mapping(self):
        """502 is mapped to provider error."""
        mapping = HTTP_ERROR_MAPPINGS[502]
        assert mapping.category == ErrorCategory.PROVIDER
        assert mapping.should_retry is True

    def test_503_mapping(self):
        """503 is mapped to provider error."""
        mapping = HTTP_ERROR_MAPPINGS[503]
        assert mapping.category == ErrorCategory.PROVIDER
        assert mapping.should_retry is True

    def test_504_mapping(self):
        """504 is mapped to provider error."""
        mapping = HTTP_ERROR_MAPPINGS[504]
        assert mapping.category == ErrorCategory.PROVIDER
        assert mapping.should_retry is True


class TestClassifyHttpError:
    """Tests for classify_http_error function."""

    def test_known_status_401(self):
        """Known status code returns correct mapping."""
        mapping = classify_http_error(401)
        assert mapping.category == ErrorCategory.AUTHENTICATION

    def test_known_status_429(self):
        """429 mapping has special properties."""
        mapping = classify_http_error(429)
        assert mapping.category == ErrorCategory.RATE_LIMITED
        assert mapping.should_retry is True
        assert mapping.retry_after_header is True

    def test_unknown_4xx_status(self):
        """Unknown 4xx returns default client error."""
        mapping = classify_http_error(418)
        assert mapping.category == ErrorCategory.UNKNOWN
        assert mapping.severity == ErrorSeverity.RECOVERABLE
        assert mapping.should_fallback is True

    def test_unknown_5xx_status(self):
        """Unknown 5xx returns default server error."""
        mapping = classify_http_error(599)
        assert mapping.category == ErrorCategory.PROVIDER
        assert mapping.severity == ErrorSeverity.TRANSIENT
        assert mapping.should_retry is True

    def test_unknown_other_status(self):
        """Unknown other status returns default."""
        mapping = classify_http_error(600)
        assert mapping.category == ErrorCategory.UNKNOWN


class TestClassifyException:
    """Tests for classify_exception function."""

    def test_classify_timeout_exception(self):
        """TimeoutException is classified as transient network error."""
        error = httpx.TimeoutException("Request timed out")
        result = classify_exception(error)

        assert result.category == ErrorCategory.NETWORK
        assert result.severity == ErrorSeverity.TRANSIENT
        assert result.remediation is not None

    def test_classify_connect_error(self):
        """ConnectError is classified as network error."""
        error = httpx.ConnectError("Failed to connect")
        result = classify_exception(error)

        assert result.category == ErrorCategory.NETWORK
        assert result.severity == ErrorSeverity.TRANSIENT

    def test_classify_http_status_error_401(self, mock_response):
        """HTTPStatusError 401 is classified."""
        mock_response.status_code = 401
        mock_response.text = "Unauthorized"
        error = httpx.HTTPStatusError(
            "Unauthorized", request=MagicMock(), response=mock_response
        )
        result = classify_exception(error)

        assert result.category == ErrorCategory.AUTHENTICATION
        assert "401" in result.message

    def test_classify_http_status_error_with_provider(self, mock_response):
        """HTTPStatusError includes provider when provided."""
        mock_response.status_code = 403
        mock_response.text = "Forbidden"
        error = httpx.HTTPStatusError(
            "Forbidden", request=MagicMock(), response=mock_response
        )
        result = classify_exception(error, provider_id="claude")

        assert result.provider == "claude"

    def test_classify_json_decode_error(self):
        """JSONDecodeError is classified as parse error."""
        error = json.JSONDecodeError("Expecting value", doc="{}", pos=0)
        result = classify_exception(error)

        assert result.category == ErrorCategory.PARSE
        assert result.severity == ErrorSeverity.RECOVERABLE

    def test_classify_key_error(self):
        """KeyError is classified as parse error."""
        error = KeyError("missing_key")
        result = classify_exception(error)

        assert result.category == ErrorCategory.PARSE
        assert "missing_key" in result.message

    def test_classify_value_error(self):
        """ValueError is classified as parse error."""
        error = ValueError("Invalid value")
        result = classify_exception(error)

        assert result.category == ErrorCategory.PARSE
        assert "Invalid value" in result.message

    def test_classify_type_error(self):
        """TypeError is classified as parse error."""
        error = TypeError("Expected int, got str")
        result = classify_exception(error)

        assert result.category == ErrorCategory.PARSE

    def test_classify_asyncio_timeout_error(self):
        """asyncio.TimeoutError is classified as transient network error."""
        error = asyncio.TimeoutError()
        result = classify_exception(error)

        assert result.category == ErrorCategory.NETWORK
        assert result.severity == ErrorSeverity.TRANSIENT

    def test_classify_asyncio_cancelled_error(self):
        """asyncio.CancelledError is classified as unknown."""
        error = asyncio.CancelledError()
        result = classify_exception(error)

        assert result.category == ErrorCategory.UNKNOWN
        assert "cancelled" in result.message

    def test_classify_file_not_found_error(self):
        """FileNotFoundError is classified as configuration error."""
        error = FileNotFoundError("/path/to/file")
        result = classify_exception(error)

        assert result.category == ErrorCategory.CONFIGURATION
        # Message is generic, doesn't include the path
        assert "not found" in result.message.lower()

    def test_classify_permission_error(self):
        """PermissionError is classified as fatal configuration error."""
        error = PermissionError("/path/to/file")
        result = classify_exception(error)

        assert result.category == ErrorCategory.CONFIGURATION
        assert result.severity == ErrorSeverity.FATAL
        assert result.remediation is not None

    def test_classify_unknown_exception(self):
        """Unknown exception is classified as unknown."""
        error = RuntimeError("Unexpected error")
        result = classify_exception(error)

        assert result.category == ErrorCategory.UNKNOWN
        assert result.severity == ErrorSeverity.RECOVERABLE
        assert result.details == {"type": "RuntimeError"}


class TestClassifyHttpStatusError:
    """Tests for classify_http_status_error function."""

    def test_classify_401_with_json_body(self, mock_response):
        """401 with JSON response extracts error message."""
        mock_response.status_code = 401
        mock_response.json = MagicMock(return_value={"error": "Invalid token"})
        mock_response.text = ""

        request = MagicMock()
        error = httpx.HTTPStatusError(
            "Unauthorized", request=request, response=mock_response
        )
        result = classify_http_status_error(error)

        assert result.category == ErrorCategory.AUTHENTICATION
        assert "Invalid token" in result.message

    def test_classify_401_with_text_body(self, mock_response):
        """401 with text response uses text."""
        mock_response.status_code = 401
        mock_response.json = MagicMock(side_effect=ValueError)
        mock_response.text = "Unauthorized access"

        request = MagicMock()
        error = httpx.HTTPStatusError(
            "Unauthorized", request=request, response=mock_response
        )
        result = classify_http_status_error(error)

        assert result.category == ErrorCategory.AUTHENTICATION
        assert "Unauthorized access" in result.message

    def test_classify_500(self, mock_response):
        """500 is classified as provider error."""
        mock_response.status_code = 500
        mock_response.json = MagicMock(return_value={})
        mock_response.text = ""

        request = MagicMock()
        error = httpx.HTTPStatusError(
            "Server error", request=request, response=mock_response
        )
        result = classify_http_status_error(error)

        assert result.category == ErrorCategory.PROVIDER
        assert result.severity == ErrorSeverity.TRANSIENT


class TestExtractErrorMessage:
    """Tests for extract_error_message function."""

    def test_extract_from_json_error_field(self, mock_response):
        """Extracts error from 'error' field."""
        mock_response.status_code = 400
        mock_response.json = MagicMock(return_value={"error": "Invalid request"})
        mock_response.text = ""

        result = extract_error_message(mock_response)
        assert result == "Invalid request"

    def test_extract_from_json_message_field(self, mock_response):
        """Extracts error from 'message' field."""
        mock_response.status_code = 400
        mock_response.json = MagicMock(return_value={"message": "Bad input"})
        mock_response.text = ""

        result = extract_error_message(mock_response)
        assert result == "Bad input"

    def test_extract_from_json_detail_field(self, mock_response):
        """Extracts error from 'detail' field."""
        mock_response.status_code = 400
        mock_response.json = MagicMock(return_value={"detail": "Validation failed"})
        mock_response.text = ""

        result = extract_error_message(mock_response)
        assert result == "Validation failed"

    def test_extract_from_nested_json(self, mock_response):
        """Extracts error from nested structure."""
        mock_response.status_code = 400
        mock_response.json = MagicMock(
            return_value={"error": {"message": "Nested error message"}}
        )
        mock_response.text = ""

        result = extract_error_message(mock_response)
        assert result == "Nested error message"

    def test_extract_from_text(self, mock_response):
        """Falls back to text content."""
        mock_response.status_code = 400
        mock_response.json = MagicMock(side_effect=ValueError)
        mock_response.text = "Plain text error"

        result = extract_error_message(mock_response)
        assert result == "Plain text error"

    def test_extract_from_long_text(self, mock_response):
        """Long text is ignored."""
        mock_response.status_code = 400
        mock_response.json = MagicMock(side_effect=ValueError)
        mock_response.text = "x" * 300

        result = extract_error_message(mock_response)
        assert "HTTP 400" in result

    def test_extract_default(self, mock_response):
        """Default to status code."""
        mock_response.status_code = 404
        mock_response.json = MagicMock(side_effect=ValueError)
        mock_response.text = ""

        result = extract_error_message(mock_response)
        assert result == "HTTP 404"


class TestGetRetryAfterDelay:
    """Tests for get_retry_after_delay function."""

    def test_no_header_returns_default(self, mock_response):
        """No Retry-After header returns default."""
        mock_response.headers = httpx.Headers({})

        result = get_retry_after_delay(mock_response, default_delay=5.0)
        assert result == 5.0

    def test_numeric_header(self, mock_response):
        """Numeric Retry-After header is parsed."""
        mock_response.headers = httpx.Headers({"Retry-After": "120"})

        result = get_retry_after_delay(mock_response)
        assert result == 120.0

    def test_invalid_header_returns_default(self, mock_response):
        """Invalid Retry-After header returns default."""
        mock_response.headers = httpx.Headers({"Retry-After": "invalid"})

        result = get_retry_after_delay(mock_response, default_delay=10.0)
        assert result == 10.0


class TestHandleHttpRequest:
    """Tests for handle_http_request function."""

    @pytest.mark.asyncio
    async def test_success_on_first_attempt(self, mock_httpx_client, mock_response):
        """Successful request returns immediately."""
        mock_httpx_client.request = AsyncMock(return_value=mock_response)

        result = await handle_http_request(
            mock_httpx_client, "GET", "https://example.com"
        )

        assert result == mock_response
        assert mock_httpx_client.request.call_count == 1

    @pytest.mark.asyncio
    async def test_retry_on_500(self, mock_httpx_client):
        """Retries on 500 server error."""
        fail_response = MagicMock(spec=httpx.Response)
        fail_response.status_code = 500
        fail_response.raise_for_status = MagicMock(
            side_effect=httpx.HTTPStatusError(
                "Server error", request=MagicMock(), response=fail_response
            )
        )
        fail_response.headers = httpx.Headers({})

        success_response = MagicMock(spec=httpx.Response)
        success_response.status_code = 200

        mock_httpx_client.request = AsyncMock(
            side_effect=[fail_response, fail_response, success_response]
        )

        result = await handle_http_request(
            mock_httpx_client,
            "GET",
            "https://example.com",
            max_retries=3,
            base_delay=0.01,
        )

        assert result.status_code == 200
        assert mock_httpx_client.request.call_count == 3

    @pytest.mark.asyncio
    async def test_no_retry_on_401(self, mock_httpx_client):
        """Does not retry on 401 authentication error."""
        fail_response = MagicMock(spec=httpx.Response)
        fail_response.status_code = 401
        fail_response.headers = httpx.Headers({})
        fail_response.raise_for_status = MagicMock(
            side_effect=httpx.HTTPStatusError(
                "Unauthorized", request=MagicMock(), response=fail_response
            )
        )

        mock_httpx_client.request = AsyncMock(return_value=fail_response)

        with pytest.raises(httpx.HTTPStatusError):
            await handle_http_request(
                mock_httpx_client, "GET", "https://example.com", max_retries=3
            )

        assert mock_httpx_client.request.call_count == 1

    @pytest.mark.asyncio
    async def test_retry_on_network_error(self, mock_httpx_client):
        """Retries on network error."""
        success_response = MagicMock(spec=httpx.Response)
        success_response.status_code = 200
        success_response.raise_for_status = MagicMock()

        mock_httpx_client.request = AsyncMock(
            side_effect=[
                httpx.ConnectError("Connection failed"),
                httpx.ConnectError("Connection failed"),
                success_response,
            ]
        )

        result = await handle_http_request(
            mock_httpx_client,
            "GET",
            "https://example.com",
            max_retries=3,
            base_delay=0.01,
        )

        assert result.status_code == 200
        assert mock_httpx_client.request.call_count == 3

    @pytest.mark.asyncio
    async def test_exhausts_retries(self, mock_httpx_client):
        """Raises after exhausting retries."""
        fail_response = MagicMock(spec=httpx.Response)
        fail_response.status_code = 503
        fail_response.headers = httpx.Headers({})
        fail_response.raise_for_status = MagicMock(
            side_effect=httpx.HTTPStatusError(
                "Service unavailable", request=MagicMock(), response=fail_response
            )
        )

        mock_httpx_client.request = AsyncMock(return_value=fail_response)

        with pytest.raises(httpx.HTTPStatusError):
            await handle_http_request(
                mock_httpx_client,
                "GET",
                "https://example.com",
                max_retries=2,
                base_delay=0.01,
            )

        assert mock_httpx_client.request.call_count == 3  # initial + 2 retries

    @pytest.mark.asyncio
    async def test_on_retry_callback(self, mock_httpx_client):
        """Calls on_retry callback."""
        fail_response = MagicMock(spec=httpx.Response)
        fail_response.status_code = 503
        fail_response.headers = httpx.Headers({})
        fail_response.raise_for_status = MagicMock(
            side_effect=httpx.HTTPStatusError(
                "Service unavailable", request=MagicMock(), response=fail_response
            )
        )

        success_response = MagicMock(spec=httpx.Response)
        success_response.status_code = 200

        mock_httpx_client.request = AsyncMock(
            side_effect=[fail_response, success_response]
        )

        callback = MagicMock()
        result = await handle_http_request(
            mock_httpx_client,
            "GET",
            "https://example.com",
            max_retries=3,
            base_delay=0.1,
            on_retry=callback,
        )

        assert result.status_code == 200
        callback.assert_called_once()
        args = callback.call_args[0]
        assert args[0] == 1  # attempt number
        assert args[1] >= 0.1  # delay

    @pytest.mark.asyncio
    async def test_respects_retry_after_header(self, mock_httpx_client):
        """Respects Retry-After header from 429 response."""
        fail_response = MagicMock(spec=httpx.Response)
        fail_response.status_code = 429
        fail_response.headers = httpx.Headers({"Retry-After": "1"})
        fail_response.raise_for_status = MagicMock(
            side_effect=httpx.HTTPStatusError(
                "Rate limited", request=MagicMock(), response=fail_response
            )
        )

        success_response = MagicMock(spec=httpx.Response)
        success_response.status_code = 200

        mock_httpx_client.request = AsyncMock(
            side_effect=[fail_response, success_response]
        )

        import time

        start = time.time()
        result = await handle_http_request(
            mock_httpx_client,
            "GET",
            "https://example.com",
            max_retries=3,
            base_delay=0.01,
        )
        elapsed = time.time() - start

        assert result.status_code == 200
        # Should have waited at least 1 second due to Retry-After
        assert elapsed >= 1.0


class TestClassifyNetworkError:
    """Tests for classify_network_error function."""

    def test_classify_connect_timeout(self):
        """ConnectTimeout is classified correctly."""
        error = httpx.ConnectTimeout("Connection timed out")
        result = classify_network_error(error)

        assert result.category == ErrorCategory.NETWORK
        assert result.severity == ErrorSeverity.TRANSIENT
        assert "timed out" in result.message.lower()
        assert result.remediation is not None

    def test_classify_read_timeout(self):
        """ReadTimeout is classified correctly."""
        error = httpx.ReadTimeout("Read timed out")
        result = classify_network_error(error)

        assert result.category == ErrorCategory.NETWORK
        assert result.severity == ErrorSeverity.TRANSIENT
        assert "timed out" in result.message.lower()

    def test_classify_write_timeout(self):
        """WriteTimeout is classified correctly."""
        error = httpx.WriteTimeout("Write timed out")
        result = classify_network_error(error)

        assert result.category == ErrorCategory.NETWORK
        assert "timed out" in result.message.lower()

    def test_classify_connect_error_refused(self):
        """ConnectError with connection refused."""
        error = httpx.ConnectError("Connection refused")
        result = classify_network_error(error)

        assert result.category == ErrorCategory.NETWORK
        assert "refused" in result.message.lower()

    def test_classify_connect_error_dns(self):
        """ConnectError with DNS issue."""
        error = httpx.ConnectError("DNS lookup failed")
        result = classify_network_error(error)

        assert result.category == ErrorCategory.NETWORK
        assert "address" in result.message.lower()

    def test_classify_generic_connect_error(self):
        """Generic ConnectError."""
        error = httpx.ConnectError("Connection failed")
        result = classify_network_error(error)

        assert result.category == ErrorCategory.NETWORK
        assert "connect" in result.message.lower()

    def test_classify_network_error(self):
        """Generic NetworkError."""
        error = httpx.NetworkError("Network unreachable")
        result = classify_network_error(error)

        assert result.category == ErrorCategory.NETWORK

    def test_classify_http_status_error_via_network(self, mock_response):
        """HTTPStatusError via classify_network_error delegates appropriately."""
        mock_response.status_code = 503
        mock_response.json = MagicMock(return_value={})
        mock_response.text = ""

        request = MagicMock()
        error = httpx.HTTPStatusError(
            "Service unavailable", request=request, response=mock_response
        )
        result = classify_network_error(error)

        assert result.category == ErrorCategory.PROVIDER
        assert result.remediation is not None

    def test_classify_unknown_error(self):
        """Unknown error type gets default classification."""
        error = RuntimeError("Something went wrong")
        result = classify_network_error(error)

        assert result.category == ErrorCategory.NETWORK
        assert "something went wrong" in result.message.lower()


class TestIsNetworkError:
    """Tests for is_network_error function."""

    def test_recognizes_network_error(self):
        """Recognizes NetworkError."""
        assert is_network_error(httpx.NetworkError("error"))

    def test_recognizes_timeout_exception(self):
        """Recognizes TimeoutException."""
        assert is_network_error(httpx.TimeoutException("timeout"))

    def test_recognizes_connect_error(self):
        """Recognizes ConnectError."""
        assert is_network_error(httpx.ConnectError("failed"))

    def test_recognizes_connect_timeout(self):
        """Recognizes ConnectTimeout."""
        assert is_network_error(httpx.ConnectTimeout("timeout"))

    def test_recognizes_read_timeout(self):
        """Recognizes ReadTimeout."""
        assert is_network_error(httpx.ReadTimeout("timeout"))

    def test_recognizes_write_timeout(self):
        """Recognizes WriteTimeout."""
        assert is_network_error(httpx.WriteTimeout("timeout"))

    def test_rejects_other_exceptions(self):
        """Rejects non-network exceptions."""
        assert not is_network_error(ValueError("not a network error"))
        assert not is_network_error(KeyError("missing"))


class TestIsRetryableError:
    """Tests for is_retryable_error function."""

    def test_network_errors_are_retryable(self):
        """Network errors are retryable."""
        assert is_retryable_error(httpx.NetworkError("error"))
        assert is_retryable_error(httpx.TimeoutException("timeout"))
        assert is_retryable_error(httpx.ConnectError("failed"))

    def test_429_is_retryable(self, mock_response):
        """429 rate limit is retryable."""
        mock_response.status_code = 429
        error = httpx.HTTPStatusError(
            "Rate limited", request=MagicMock(), response=mock_response
        )
        assert is_retryable_error(error)

    def test_500_is_retryable(self, mock_response):
        """500 server error is retryable."""
        mock_response.status_code = 500
        error = httpx.HTTPStatusError(
            "Server error", request=MagicMock(), response=mock_response
        )
        assert is_retryable_error(error)

    def test_401_is_not_retryable(self, mock_response):
        """401 auth error is not retryable."""
        mock_response.status_code = 401
        error = httpx.HTTPStatusError(
            "Unauthorized", request=MagicMock(), response=mock_response
        )
        assert not is_retryable_error(error)

    def test_403_is_not_retryable(self, mock_response):
        """403 forbidden is not retryable."""
        mock_response.status_code = 403
        error = httpx.HTTPStatusError(
            "Forbidden", request=MagicMock(), response=mock_response
        )
        assert not is_retryable_error(error)

    def test_other_exceptions_not_retryable(self):
        """Other exceptions are not retryable."""
        assert not is_retryable_error(ValueError("error"))
        assert not is_retryable_error(KeyError("missing"))


class TestClassifyHttpStatusErrorNetwork:
    """Tests for classify_http_status_error in network module."""

    def test_401_has_remediation(self, mock_response):
        """401 includes remediation."""
        mock_response.status_code = 401
        mock_response.json = MagicMock(return_value={"error": "Unauthorized"})
        mock_response.text = ""

        request = MagicMock()
        error = httpx.HTTPStatusError(
            "Unauthorized", request=request, response=mock_response
        )
        result = classify_http_status_error_network(error)

        assert result.category == ErrorCategory.AUTHENTICATION
        assert result.remediation is not None
        assert "vibeusage auth" in result.remediation.lower()

    def test_403_has_remediation(self, mock_response):
        """403 includes remediation."""
        mock_response.status_code = 403
        mock_response.json = MagicMock(return_value={"error": "Forbidden"})
        mock_response.text = ""

        request = MagicMock()
        error = httpx.HTTPStatusError(
            "Forbidden", request=request, response=mock_response
        )
        result = classify_http_status_error_network(error)

        assert result.category == ErrorCategory.AUTHORIZATION
        assert result.remediation is not None
        assert "subscription" in result.remediation.lower()

    def test_404_has_remediation(self, mock_response):
        """404 includes remediation."""
        mock_response.status_code = 404
        mock_response.json = MagicMock(return_value={"error": "Not found"})
        mock_response.text = ""

        request = MagicMock()
        error = httpx.HTTPStatusError(
            "Not found", request=request, response=mock_response
        )
        result = classify_http_status_error_network(error)

        assert result.category == ErrorCategory.NOT_FOUND
        assert result.remediation is not None

    def test_429_has_remediation(self, mock_response):
        """429 includes remediation."""
        mock_response.status_code = 429
        mock_response.json = MagicMock(return_value={"error": "Rate limited"})
        mock_response.text = ""

        request = MagicMock()
        error = httpx.HTTPStatusError(
            "Rate limited", request=request, response=mock_response
        )
        result = classify_http_status_error_network(error)

        assert result.category == ErrorCategory.RATE_LIMITED
        assert result.remediation is not None
        assert "wait" in result.remediation.lower()

    def test_500_has_remediation(self, mock_response):
        """500 includes remediation."""
        mock_response.status_code = 500
        mock_response.json = MagicMock(return_value={"error": "Internal error"})
        mock_response.text = ""

        request = MagicMock()
        error = httpx.HTTPStatusError(
            "Internal error", request=request, response=mock_response
        )
        result = classify_http_status_error_network(error)

        assert result.category == ErrorCategory.PROVIDER
        assert result.remediation is not None
        assert "later" in result.remediation.lower()
