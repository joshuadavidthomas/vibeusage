"""JSON output utilities for vibeusage."""

from __future__ import annotations

import sys

import msgspec


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
    encoder = msgspec.json.Encoder(indent=indent)
    json_bytes = encoder.encode(data)
    sys.stdout.buffer.write(json_bytes)
    sys.stdout.buffer.write(b"\n")


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
