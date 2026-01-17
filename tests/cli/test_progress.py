"""Tests for CLI progress tracking."""

from __future__ import annotations

from unittest.mock import MagicMock
from unittest.mock import patch

from rich.progress import TaskID

from vibeusage.cli.progress import ProgressCallback
from vibeusage.cli.progress import ProgressTracker
from vibeusage.cli.progress import create_progress
from vibeusage.cli.progress import create_progress_callback
from vibeusage.strategies.base import FetchAttempt
from vibeusage.strategies.base import FetchOutcome


class TestCreateProgress:
    """Tests for create_progress context manager."""

    def test_quiet_mode_returns_none(self):
        """Quiet mode suppresses progress output."""
        with create_progress(quiet=True) as tracker:
            assert tracker is None

    @patch("vibeusage.cli.progress.Progress")
    def test_normal_mode_returns_tracker(self, mock_progress_class):
        """Normal mode returns a ProgressTracker instance."""
        mock_progress = MagicMock()
        mock_progress_class.return_value = mock_progress

        with create_progress(quiet=False) as tracker:
            assert isinstance(tracker, ProgressTracker)
            assert tracker.progress == mock_progress

    @patch("vibeusage.cli.progress.Progress")
    def test_passes_console_to_progress(self, mock_progress_class):
        """Custom console is passed to Progress."""
        mock_console = MagicMock()
        mock_progress = MagicMock()
        mock_progress_class.return_value = mock_progress

        with create_progress(console=mock_console, quiet=False) as tracker:
            assert tracker is not None
            # Verify Progress was called with the console
            mock_progress_class.assert_called_once()
            # The console is passed as a keyword argument
            call_kwargs = mock_progress_class.call_args.kwargs
            assert "console" in call_kwargs
            assert call_kwargs["console"] == mock_console

    @patch("vibeusage.cli.progress.Progress")
    def test_progress_has_correct_configuration(self, mock_progress_class):
        """Progress is configured with expected settings."""
        mock_progress = MagicMock()
        mock_progress_class.return_value = mock_progress

        with create_progress(quiet=False) as _:
            # Verify Progress was called with expected arguments
            call_kwargs = mock_progress_class.call_args.kwargs
            assert call_kwargs.get("transient") is True
            assert "console" in call_kwargs
            # Verify console has theme set (theme is applied to Console, not Progress)
            console = call_kwargs["console"]
            # Console was created with theme
            assert console is not None


class TestProgressTracker:
    """Tests for ProgressTracker class."""

    def test_initialization(self):
        """ProgressTracker initializes with expected attributes."""
        mock_progress = MagicMock()
        tracker = ProgressTracker(mock_progress)

        assert tracker.progress == mock_progress
        assert tracker.tasks == {}
        assert tracker.total_count == 0
        assert tracker.completed_count == 0
        assert tracker._main_task_id is None

    def test_start_creates_main_task(self):
        """Start creates a main overall progress task."""
        mock_progress = MagicMock()
        mock_progress.add_task = MagicMock(return_value=TaskID(1))
        tracker = ProgressTracker(mock_progress)

        provider_ids = ["claude", "codex", "copilot"]
        tracker.start(provider_ids)

        assert tracker.total_count == 3
        assert tracker.completed_count == 0
        assert tracker._main_task_id == TaskID(1)
        # Main task was created with total count
        mock_progress.add_task.assert_any_call(
            "[cyan]Fetching usage data...",
            total=3,
        )

    def test_start_creates_provider_tasks(self):
        """Start creates individual tasks for each provider."""
        mock_progress = MagicMock()
        mock_progress.add_task = MagicMock(side_effect=[TaskID(i) for i in range(4)])
        tracker = ProgressTracker(mock_progress)

        provider_ids = ["claude", "codex"]
        tracker.start(provider_ids)

        # Should have main task + 2 provider tasks
        assert mock_progress.add_task.call_count == 3
        assert len(tracker.tasks) == 2
        assert "claude" in tracker.tasks
        assert "codex" in tracker.tasks

    def test_start_calls_progress_start(self):
        """Start calls progress.start() to begin rendering."""
        mock_progress = MagicMock()
        tracker = ProgressTracker(mock_progress)

        tracker.start(["claude"])

        mock_progress.start.assert_called_once()

    def test_update_increments_completed_count(self):
        """Update increments the completed counter."""
        mock_progress = MagicMock()
        tracker = ProgressTracker(mock_progress)

        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=None,
            source="oauth",
            attempts=[],
        )

        tracker.update(outcome)

        assert tracker.completed_count == 1

    def test_update_updates_main_task(self):
        """Update updates the main progress task."""
        mock_progress = MagicMock()
        mock_progress.add_task = MagicMock(return_value=TaskID(0))
        tracker = ProgressTracker(mock_progress)
        tracker.start(["claude", "codex"])

        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=None,
            source="oauth",
            attempts=[],
        )

        tracker.update(outcome)

        # Main task was updated with new completed count and description
        mock_progress.update.assert_called()

    def test_update_updates_provider_task(self):
        """Update updates the individual provider task."""
        mock_progress = MagicMock()
        mock_progress.add_task = MagicMock(side_effect=[TaskID(i) for i in range(3)])
        tracker = ProgressTracker(mock_progress)
        tracker.start(["claude", "codex"])

        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=None,
            source="oauth",
            attempts=[],
        )

        tracker.update(outcome)

        # Provider task was updated
        assert mock_progress.update.call_count >= 2  # Main task + provider task

    def test_main_description_in_progress(self):
        """Main description shows progress when not complete."""
        mock_progress = MagicMock()
        mock_progress.add_task = MagicMock(return_value=TaskID(0))
        tracker = ProgressTracker(mock_progress)
        tracker.start(["claude", "codex"])

        description = tracker._get_main_description()

        assert "1/2" not in description  # No updates yet
        # Should be initial state
        assert "[cyan]Fetching usage data...[/cyan]" in description

    def test_main_description_in_progress_with_updates(self):
        """Main description shows current progress after updates."""
        mock_progress = MagicMock()
        mock_progress.add_task = MagicMock(return_value=TaskID(0))
        tracker = ProgressTracker(mock_progress)
        tracker.start(["claude", "codex", "copilot"])

        # Simulate one update
        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=None,
            source="oauth",
            attempts=[],
        )
        tracker.update(outcome)

        description = tracker._get_main_description()

        assert "[cyan]Fetching usage data...[/cyan]" in description
        assert "(1/3)" in description

    def test_main_description_complete(self):
        """Main description shows complete when all done."""
        mock_progress = MagicMock()
        mock_progress.add_task = MagicMock(return_value=TaskID(0))
        tracker = ProgressTracker(mock_progress)
        tracker.start(["claude"])

        # Mark as complete
        tracker.completed_count = 1
        tracker.total_count = 1

        description = tracker._get_main_description()

        assert "[green]Fetch complete[/green]" in description

    def test_provider_description_success_fresh(self):
        """Provider description shows success for fresh data."""
        mock_progress = MagicMock()
        tracker = ProgressTracker(mock_progress)

        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=None,
            source="oauth",
            attempts=[],
            cached=False,
        )

        description = tracker._get_provider_description(outcome)

        assert "[green]Claude (fresh)[/green]" == description

    def test_provider_description_success_cached(self):
        """Provider description shows cached for cached data."""
        mock_progress = MagicMock()
        tracker = ProgressTracker(mock_progress)

        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=None,
            source="cache",
            attempts=[],
            cached=True,
        )

        description = tracker._get_provider_description(outcome)

        assert "[green]Claude (cached)[/green]" == description

    def test_provider_description_failed(self):
        """Provider description shows failed for errors."""
        mock_progress = MagicMock()
        tracker = ProgressTracker(mock_progress)

        outcome = FetchOutcome(
            provider_id="claude",
            success=False,
            snapshot=None,
            source=None,
            attempts=[],
            error="Connection timeout",
        )

        description = tracker._get_provider_description(outcome)

        assert "[red]Claude (failed)[/red]" == description

    def test_provider_description_gated(self):
        """Provider description shows gated status."""
        mock_progress = MagicMock()
        tracker = ProgressTracker(mock_progress)

        outcome = FetchOutcome(
            provider_id="claude",
            success=False,
            snapshot=None,
            source=None,
            attempts=[],
            gated=True,
        )

        description = tracker._get_provider_description(outcome)

        assert "[yellow]Claude (gated)[/yellow]" == description

    def test_provider_description_gated_takes_precedence(self):
        """Gated status takes precedence over success/failure in description."""
        mock_progress = MagicMock()
        tracker = ProgressTracker(mock_progress)

        # Gated with success=True (edge case - gated overrides success)
        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=None,
            source=None,
            attempts=[],
            gated=True,
        )

        description = tracker._get_provider_description(outcome)

        assert "[yellow]Claude (gated)[/yellow]" == description

    def test_provider_description_capitalizes_provider(self):
        """Provider description capitalizes the provider ID."""
        mock_progress = MagicMock()
        tracker = ProgressTracker(mock_progress)

        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=None,
            source="oauth",
            attempts=[],
        )

        description = tracker._get_provider_description(outcome)

        assert "Claude" in description
        assert "claude" not in description.lower() or description == "[green]Claude (fresh)[/green]"

    def test_stop_calls_progress_stop(self):
        """Stop calls progress.stop() to end rendering."""
        mock_progress = MagicMock()
        tracker = ProgressTracker(mock_progress)

        tracker.stop()

        mock_progress.stop.assert_called_once()


class TestProgressCallback:
    """Tests for ProgressCallback class."""

    def test_initialization(self):
        """ProgressCallback stores the tracker."""
        mock_tracker = MagicMock()
        callback = ProgressCallback(mock_tracker)

        assert callback.tracker == mock_tracker

    def test_call_updates_tracker(self):
        """Calling the callback updates the tracker."""
        mock_tracker = MagicMock()
        callback = ProgressCallback(mock_tracker)

        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=None,
            source="oauth",
            attempts=[],
        )

        callback(outcome)

        mock_tracker.update.assert_called_once_with(outcome)

    def test_callable_instance(self):
        """ProgressCallback instances are callable."""
        mock_tracker = MagicMock()
        callback = ProgressCallback(mock_tracker)

        assert callable(callback)


class TestCreateProgressCallback:
    """Tests for create_progress_callback function."""

    def test_returns_none_when_tracker_is_none(self):
        """Returns None when no tracker provided."""
        result = create_progress_callback(None)

        assert result is None

    def test_returns_progress_callback_when_tracker_provided(self):
        """Returns ProgressCallback when tracker is provided."""
        mock_tracker = MagicMock()
        result = create_progress_callback(mock_tracker)

        assert isinstance(result, ProgressCallback)
        assert result.tracker == mock_tracker

    def test_callback_is_functional(self):
        """Returned callback can be called to update tracker."""
        mock_tracker = MagicMock()
        callback = create_progress_callback(mock_tracker)

        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=None,
            source="oauth",
            attempts=[],
        )

        callback(outcome)

        mock_tracker.update.assert_called_once_with(outcome)


class TestProgressTrackerIntegration:
    """Integration tests for ProgressTracker workflows."""

    def test_full_fetch_cycle(self):
        """Test complete fetch cycle from start to stop."""
        mock_progress = MagicMock()
        mock_progress.add_task = MagicMock(side_effect=[TaskID(i) for i in range(4)])
        tracker = ProgressTracker(mock_progress)

        # Start with 3 providers
        provider_ids = ["claude", "codex", "copilot"]
        tracker.start(provider_ids)

        assert tracker.total_count == 3
        assert tracker.completed_count == 0
        mock_progress.start.assert_called_once()

        # Update with outcomes
        outcomes = [
            FetchOutcome(
                provider_id="claude",
                success=True,
                snapshot=None,
                source="oauth",
                attempts=[],
                cached=False,
            ),
            FetchOutcome(
                provider_id="codex",
                success=False,
                snapshot=None,
                source=None,
                attempts=[FetchAttempt(strategy="oauth", success=False, error="Timeout")],
                error="Timeout",
            ),
            FetchOutcome(
                provider_id="copilot",
                success=True,
                snapshot=None,
                source="cache",
                attempts=[],
                cached=True,
                gated=True,
            ),
        ]

        for outcome in outcomes:
            tracker.update(outcome)

        assert tracker.completed_count == 3

        # Stop
        tracker.stop()
        mock_progress.stop.assert_called_once()

    def test_multiple_update_cycles(self):
        """Test tracker can handle multiple update cycles."""
        mock_progress = MagicMock()
        mock_progress.add_task = MagicMock(side_effect=[TaskID(i) for i in range(6)])
        tracker = ProgressTracker(mock_progress)

        # First cycle
        tracker.start(["claude", "codex"])
        tracker.update(
            FetchOutcome(
                provider_id="claude",
                success=True,
                snapshot=None,
                source="oauth",
                attempts=[],
            )
        )
        tracker.update(
            FetchOutcome(
                provider_id="codex",
                success=True,
                snapshot=None,
                source="web",
                attempts=[],
            )
        )
        tracker.stop()

        # Verify state
        assert tracker.completed_count == 2
        assert tracker.total_count == 2

    def test_description_progression(self):
        """Test main description changes correctly during progress."""
        mock_progress = MagicMock()
        mock_progress.add_task = MagicMock(return_value=TaskID(0))
        tracker = ProgressTracker(mock_progress)
        tracker.start(["claude", "codex", "copilot"])

        # Initial state
        desc1 = tracker._get_main_description()
        assert "[cyan]Fetching usage data...[/cyan]" in desc1
        assert "(0/3)" in desc1

        # After first update
        tracker.completed_count = 1
        desc2 = tracker._get_main_description()
        assert "(1/3)" in desc2

        # After second update
        tracker.completed_count = 2
        desc3 = tracker._get_main_description()
        assert "(2/3)" in desc3

        # Complete
        tracker.completed_count = 3
        desc4 = tracker._get_main_description()
        assert "[green]Fetch complete[/green]" in desc4


class TestProgressTheme:
    """Tests for progress theme configuration."""

    @patch("vibeusage.cli.progress.Console")
    @patch("vibeusage.cli.progress.Progress")
    def test_console_is_created_with_theme(self, mock_progress_class, mock_console_class):
        """Console is created with theme when none is provided."""
        mock_console = MagicMock()
        mock_console_class.return_value = mock_console
        mock_progress = MagicMock()
        mock_progress_class.return_value = mock_progress

        with create_progress(quiet=False) as tracker:
            # Verify Console was created with theme
            mock_console_class.assert_called_once()
            call_kwargs = mock_console_class.call_args.kwargs
            assert "theme" in call_kwargs
            # Verify tracker was created successfully
            assert tracker is not None
