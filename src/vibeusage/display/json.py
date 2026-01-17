"""JSON output utilities for vibeusage."""

from __future__ import annotations

import sys
from datetime import datetime

import msgspec

__all__ = [
    "ErrorResponse",
    "ErrorData",
    "create_error_response",
    "output_json",
    "output_json_pretty",
    "output_json_error",
    "from_vibeusage_error",
    "encode_json",
    "decode_json",
]


class ErrorResponse(msgspec.Struct, frozen=True):
    """Structured error response for JSON output per spec 07."""

    error: ErrorData

    def to_dict(self) -> dict:
        """Convert to dict for JSON serialization."""
        return {"error": self.error.to_dict()}


class ErrorData(msgspec.Struct, frozen=True):
    """Error data with category, severity, and remediation."""

    message: str
    category: str
    severity: str
    provider: str | None = None
    remediation: str | None = None
    details: dict | None = None
    timestamp: str = msgspec.field(
        default_factory=lambda: datetime.now().astimezone().isoformat()
    )

    def to_dict(self) -> dict:
        """Convert to dict for JSON serialization."""
        data = {
            "message": self.message,
            "category": self.category,
            "severity": self.severity,
            "timestamp": self.timestamp,
        }
        if self.provider:
            data["provider"] = self.provider
        if self.remediation:
            data["remediation"] = self.remediation
        if self.details:
            data["details"] = self.details
        return data


def create_error_response(
    message: str,
    category: str,
    severity: str,
    provider: str | None = None,
    remediation: str | None = None,
    details: dict | None = None,
) -> ErrorResponse:
    """Create an ErrorResponse from individual fields.

    Args:
        message: Human-readable error message
        category: Error category (e.g., "authentication", "network")
        severity: Error severity (e.g., "fatal", "recoverable", "transient")
        provider: Provider that caused the error
        remediation: How to fix the error
        details: Additional technical details

    Returns:
        ErrorResponse struct
    """
    error_data = ErrorData(
        message=message,
        category=category,
        severity=severity,
        provider=provider,
        remediation=remediation,
        details=details,
    )
    return ErrorResponse(error=error_data)


def output_json(data: object) -> None:
    """Output data as JSON to stdout.

    Args:
        data: Any msgspec-serializable object (Struct, dict, list, etc.)
    """
    json_bytes = msgspec.json.encode(data)
    sys.stdout.buffer.write(json_bytes)
    sys.stdout.buffer.write(b"\n")


def output_json_pretty(data: object, indent: int = 2) -> None:
    """Output data as pretty-printed JSON to stdout.

    Args:
        data: Any msgspec-serializable object
        indent: Number of spaces for indentation
    """
    import json

    # Convert msgspec data to dict/list for json module
    # Use msgspec to decode, then json module for pretty printing
    json_bytes = msgspec.json.encode(data)
    python_obj = msgspec.json.decode(json_bytes)
    json_str = json.dumps(python_obj, indent=indent)
    sys.stdout.write(json_str)
    sys.stdout.write("\n")


def output_json_error(
    message: str,
    category: str = "unknown",
    severity: str = "recoverable",
    provider: str | None = None,
    remediation: str | None = None,
    details: dict | None = None,
    indent: int = 2,
) -> None:
    """Output an error in standardized JSON format.

    Args:
        message: Human-readable error message
        category: Error category (e.g., "authentication", "network")
        severity: Error severity (e.g., "fatal", "recoverable", "transient")
        provider: Provider that caused the error
        remediation: How to fix the error
        details: Additional technical details
        indent: Number of spaces for indentation
    """
    error_response = create_error_response(
        message=message,
        category=category,
        severity=severity,
        provider=provider,
        remediation=remediation,
        details=details,
    )
    output_json_pretty(error_response.to_dict(), indent=indent)


def from_vibeusage_error(vibeerror) -> ErrorResponse:
    """Create an ErrorResponse from a VibeusageError instance.

    Args:
        vibeerror: A VibeusageError instance from errors/types.py

    Returns:
        ErrorResponse struct for JSON output
    """
    return ErrorResponse(
        error=ErrorData(
            message=vibeerror.message,
            category=vibeerror.category.value,
            severity=vibeerror.severity.value,
            provider=vibeerror.provider,
            remediation=vibeerror.remediation,
            details=vibeerror.details,
            timestamp=vibeerror.timestamp.isoformat(),
        )
    )


def encode_json(data: object) -> bytes:
    """Encode data as JSON bytes.

    Args:
        data: Any msgspec-serializable object

    Returns:
        JSON-encoded bytes
    """
    return msgspec.json.encode(data)


def decode_json(json_bytes: bytes, type_hint: type | None = None) -> object:
    """Decode JSON bytes to Python object.

    Args:
        json_bytes: JSON-encoded bytes
        type_hint: Optional msgspec type for validation

    Returns:
        Decoded Python object
    """
    if type_hint:
        return msgspec.json.decode(json_bytes, type=type_hint)
    return msgspec.json.decode(json_bytes)
