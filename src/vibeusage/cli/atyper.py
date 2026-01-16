"""Async wrapper for Typer to support async commands."""

import asyncio
import inspect
from typing import Any

import typer
from typer.core import TyperCommand, TyperGroup


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
