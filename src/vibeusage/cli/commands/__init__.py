"""CLI commands for vibeusage."""

# Top-level commands
# Command groups (these register themselves with the main app)
from __future__ import annotations

from vibeusage.cli.commands import cache
from vibeusage.cli.commands import config
from vibeusage.cli.commands import key
from vibeusage.cli.commands.auth import auth_command
from vibeusage.cli.commands.status import status_command
from vibeusage.cli.commands.usage import usage_command

__all__ = [
    # Top-level commands
    "usage_command",
    "status_command",
    "auth_command",
    # Command groups
    "cache",
    "config",
    "key",
]
