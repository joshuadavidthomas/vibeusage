"""Progress tracking for concurrent fetch operations."""

from __future__ import annotations

from contextlib import contextmanager
from typing import TYPE_CHECKING

from rich.console import Console
from rich.progress import BarColumn
from rich.progress import Progress
from rich.progress import SpinnerColumn
from rich.progress import TaskID
from rich.progress import TextColumn
from rich.progress import TimeRemainingColumn
from rich.theme import Theme

if TYPE_CHECKING:
    from vibeusage.strategies.base import FetchOutcome

# Custom theme for progress indicators
PROGRESS_THEME = Theme(
    {
        "bar.back": "black",
        "bar.complete": "rgb(67, 142, 247)",  # Blue
        "bar.finished": "rgb(98, 189, 119)",  # Green
        "progress.description": "white",
        "progress.download": "cyan",
        "progress.data.speed": "cyan",
        "progress.data.filesize": "green",
        "progress.remaining": "yellow",
        "progress.percentage": "cyan",
    }
)


@contextmanager
def create_progress(console: Console | None = None, quiet: bool = False):
    """Create a Rich progress context for tracking fetch operations.

    Args:
        console: Rich console (uses default if None)
        quiet: If True, suppresses progress output

    Yields:
        ProgressTracker instance or None if quiet
    """
    if quiet:
        yield None
        return

    progress = Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TextColumn("[progress.percentage]{task.percentage:>3.0f}%"),
        TimeRemainingColumn(),
        console=console,
        theme=PROGRESS_THEME,
        transient=True,  # Auto-remove progress when complete
    )

    yield ProgressTracker(progress)


class ProgressTracker:
    """Tracks progress for concurrent provider fetch operations."""

    def __init__(self, progress: Progress) -> None:
        self.progress = progress
        self.tasks: dict[str, TaskID] = {}
        self.total_count = 0
        self.completed_count = 0
        self._main_task_id: TaskID | None = None

    def start(self, provider_ids: list[str]) -> None:
        """Initialize progress tracking for a list of providers.

        Args:
            provider_ids: List of provider identifiers being fetched
        """
        self.total_count = len(provider_ids)
        self.completed_count = 0

        # Create main overall task
        self._main_task_id = self.progress.add_task(
            "[cyan]Fetching usage data...",
            total=self.total_count,
        )

        # Create individual tasks for each provider
        for provider_id in provider_ids:
            task_id = self.progress.add_task(
                f"[dim]{provider_id.capitalize()}[/dim]",
                total=1,
                visible=False,  # Hidden by default
            )
            self.tasks[provider_id] = task_id

        self.progress.start()

    def update(self, outcome: FetchOutcome) -> None:
        """Update progress for a completed provider fetch.

        Args:
            outcome: FetchOutcome with provider status
        """
        self.completed_count += 1

        # Update main progress
        if self._main_task_id is not None:
            self.progress.update(
                self._main_task_id,
                completed=self.completed_count,
                description=self._get_main_description(),
            )

        # Update individual provider task
        if outcome.provider_id in self.tasks:
            task_id = self.tasks[outcome.provider_id]
            description = self._get_provider_description(outcome)
            self.progress.update(
                task_id,
                completed=1,
                description=description,
                visible=True,
            )

    def _get_main_description(self) -> str:
        """Get the main progress description."""
        if self.completed_count < self.total_count:
            return f"[cyan]Fetching usage data...[/cyan] ({self.completed_count}/{self.total_count})"
        return "[green]Fetch complete[/green]"

    def _get_provider_description(self, outcome: FetchOutcome) -> str:
        """Get description for a provider based on outcome."""
        provider = outcome.provider_id.capitalize()

        if outcome.gated:
            return f"[yellow]{provider} (gated)[/yellow]"
        if outcome.success:
            status = "cached" if outcome.cached else "fresh"
            return f"[green]{provider} ({status})[/green]"
        return f"[red]{provider} (failed)[/red]"

    def stop(self) -> None:
        """Stop progress tracking."""
        self.progress.stop()


class ProgressCallback:
    """Callback adapter for use with orchestrator fetch functions."""

    def __init__(self, tracker: ProgressTracker) -> None:
        self.tracker = tracker

    def __call__(self, outcome: FetchOutcome) -> None:
        """Update progress tracker with fetch outcome."""
        self.tracker.update(outcome)


def create_progress_callback(tracker: ProgressTracker | None) -> callable | None:
    """Create a progress callback for use with orchestrator.

    Args:
        tracker: ProgressTracker instance (returns None if tracker is None)

    Returns:
        Callback function or None
    """
    if tracker is None:
        return None
    return ProgressCallback(tracker)
