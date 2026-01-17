"""CLI commands for vibeusage."""

# Top-level commands
from vibeusage.cli.commands.auth import auth_command
from vibeusage.cli.commands.status import status_command
from vibeusage.cli.commands.usage import usage_command

# Command groups (these register themselves with the main app)
from vibeusage.cli.commands import (
    cache,
    config,
    key,
)

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
