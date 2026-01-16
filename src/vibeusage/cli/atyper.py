"""Async wrapper for Typer to support async commands."""

import asyncio
import inspect
from typing import Any, Callable
from functools import wraps

import typer
from typer.core import TyperCommand, TyperGroup
from typer.models import CommandInfo


def _async_command_wrapper(f: Callable) -> Callable:
    """Wrap an async function to run synchronously with asyncio.run."""

    @wraps(f)
    def sync_wrapper(*args: Any, **kwargs: Any) -> Any:
        return asyncio.run(f(*args, **kwargs))

    return sync_wrapper


class AsyncTyperCommand(TyperCommand):
    """Custom command that handles async functions."""

    def invoke(self, ctx: Any) -> Any:
        """Invoke async command with asyncio.run."""
        if inspect.iscoroutinefunction(self.callback):
            return asyncio.run(self.callback(**ctx.params))
        return super().invoke(ctx)


class AsyncTyperGroup(TyperGroup):
    """Custom group that handles async callbacks."""

    def invoke(self, ctx: Any) -> Any:
        """Invoke async callback with asyncio.run."""
        if inspect.iscoroutinefunction(self.callback):
            return asyncio.run(self.callback(**ctx.params))
        return super().invoke(ctx)


class ATyper(typer.Typer):
    """Typer subclass with async command support."""

    def __init__(self, *args: Any, **kwargs: Any) -> None:
        """Initialize ATyper with async group class."""
        kwargs.setdefault("cls", AsyncTyperGroup)
        kwargs.setdefault("no_args_is_help", False)
        super().__init__(*args, **kwargs)

    def command(  # type: ignore
        self,
        name: str | None = None,
        *,
        cls: type[TyperCommand] | None = None,
        **kwargs: Any,
    ) -> Any:
        """Register a command, wrapping async functions for execution."""

        def decorator(f: Callable) -> Callable:
            # Wrap async functions to run synchronously
            if inspect.iscoroutinefunction(f):
                f = _async_command_wrapper(f)
            # Use AsyncTyperCommand class for all commands
            if cls is None:
                kwargs["cls"] = AsyncTyperCommand
            return typer.Typer.command(self, name, **kwargs)(f)

        return decorator
