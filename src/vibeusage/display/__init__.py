"""Display utilities for vibeusage.

This module provides rendering and output utilities for both terminal
(Rich-based) and JSON output modes.
"""
from __future__ import annotations

from vibeusage.display.json import decode_json
from vibeusage.display.json import encode_json
from vibeusage.display.json import output_json
from vibeusage.display.json import output_json_pretty
from vibeusage.display.rich import format_overage_used
from vibeusage.display.rich import format_period
from vibeusage.display.rich import format_period_line
from vibeusage.display.rich import render_usage_bar

__all__ = [
    # Rich rendering
    "render_usage_bar",
    "format_period",
    "format_period_line",
    "format_overage_used",
    # JSON output
    "output_json",
    "output_json_pretty",
    "encode_json",
    "decode_json",
]
