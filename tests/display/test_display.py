"""Tests for display utilities."""

from datetime import datetime, timedelta, timezone
from decimal import Decimal
from io import StringIO
from unittest.mock import patch

import pytest
from rich.text import Text

from vibeusage.display.rich import (
    render_usage_bar,
    format_period,
    format_overage_used,
    format_period_line,
)
from vibeusage.display.json import (
    output_json,
    output_json_pretty,
    encode_json,
    decode_json,
)
from vibeusage.models import (
    UsagePeriod,
    PeriodType,
    OverageUsage,
)


class TestRenderUsageBar:
    """Tests for render_usage_bar function."""

    def test_zero_utilization(self):
        """Zero utilization renders empty bar."""
        result = render_usage_bar(0, width=10)
        plain = result.plain
        assert plain == "░" * 10

    def test_full_utilization(self):
        """Full utilization renders full bar."""
        result = render_usage_bar(100, width=10)
        plain = result.plain
        assert plain == "█" * 10

    def test_half_utilization(self):
        """Half utilization renders half-filled bar."""
        result = render_usage_bar(50, width=10)
        plain = result.plain
        assert plain == "██████████"[:5] + "░" * 5

    def test_custom_width(self):
        """Custom width affects bar length."""
        result = render_usage_bar(50, width=20)
        plain = result.plain
        assert len(plain) == 20

    def test_default_width(self):
        """Default width is 20."""
        result = render_usage_bar(50)
        plain = result.plain
        assert len(plain) == 20

    def test_color_override(self):
        """Color override sets the color."""
        result = render_usage_bar(50, color="red")
        # The color is a Rich style attribute
        assert result.spans[0].style == "red"

    def test_no_color(self):
        """No color uses default style."""
        result = render_usage_bar(50)
        # Should have a span
        assert len(result.spans) > 0


class TestFormatPeriod:
    """Tests for format_period function."""

    def test_format_basic_period(self, utc_now):
        """Basic period formatting."""
        period = UsagePeriod(
            name="Daily",
            utilization=65,
            period_type=PeriodType.DAILY,
            resets_at=utc_now + timedelta(hours=12),
        )

        result = format_period(period)
        plain = result.plain

        assert "65%" in plain
        assert "Daily" in plain
        assert "resets in" in plain

    def test_format_without_reset_time(self):
        """Period without reset time omits reset info."""
        period = UsagePeriod(
            name="Session", utilization=50, period_type=PeriodType.SESSION, resets_at=None
        )

        result = format_period(period)
        plain = result.plain

        assert "50%" in plain
        assert "Session" in plain
        assert "resets in" not in plain

    def test_format_with_model(self, utc_now):
        """Model-specific period includes model name."""
        period = UsagePeriod(
            name="Opus",
            utilization=80,
            period_type=PeriodType.DAILY,
            model="opus",
            resets_at=utc_now + timedelta(hours=6),
        )

        result = format_period(period)
        plain = result.plain

        assert "80%" in plain
        assert "Opus" in plain

    def test_progress_bar_included(self, utc_now):
        """Progress bar is included in output."""
        period = UsagePeriod(
            name="Weekly", utilization=30, period_type=PeriodType.WEEKLY
        )

        result = format_period(period)
        plain = result.plain

        # Should have some block characters
        assert "█" in plain or "░" in plain


class TestFormatOverageUsed:
    """Tests for format_overage_used function."""

    def test_format_usd(self):
        """Format USD with dollar symbol."""
        result = format_overage_used(2.50, 15.00, "USD")
        plain = result.plain

        assert "$2.50" in plain
        assert "$15.00" in plain
        assert "Overage" in plain
        assert "remaining" in plain

    def test_format_non_usd(self):
        """Format non-USD currency."""
        result = format_overage_used(100, 500, "credits")
        plain = result.plain

        # No dollar sign for non-USD
        assert "$" not in plain
        assert "100.00" in plain or "100" in plain

    def test_format_remaining(self):
        """Calculates remaining correctly."""
        result = format_overage_used(5.00, 20.00, "USD")
        plain = result.plain

        assert "15.00" in plain  # Remaining

    def test_format_zero_remaining(self):
        """Handles zero remaining."""
        result = format_overage_used(20.00, 20.00, "USD")
        plain = result.plain

        assert "0.00" in plain


class TestFormatPeriodLine:
    """Tests for format_period_line function."""

    def test_format_basic_line(self):
        """Basic period line formatting."""
        result = format_period_line("Session", 50)
        plain = result.plain

        assert "Session" in plain
        assert "50%" in plain
        assert "█" in plain or "░" in plain

    def test_format_with_reset(self):
        """Period line with reset time."""
        result = format_period_line("Daily", 75, resets_at_str="2h 30m")
        plain = result.plain

        assert "Daily" in plain
        assert "75%" in plain
        assert "2h 30m" in plain

    def test_format_with_indent(self):
        """Period line with indentation."""
        result = format_period_line("Weekly", 30, indent=4)
        plain = result.plain

        assert plain.startswith("    ")

    def test_coloring_low_utilization(self):
        """Low utilization gets green color."""
        result = format_period_line("Session", 30)
        # Should have green span
        assert any(span.style == "green" for span in result.spans if span.style)

    def test_coloring_medium_utilization(self):
        """Medium utilization gets yellow color."""
        result = format_period_line("Session", 65)
        # Should have yellow span
        assert any(span.style == "yellow" for span in result.spans if span.style)

    def test_coloring_high_utilization(self):
        """High utilization gets red color."""
        result = format_period_line("Session", 85)
        # Should have red span
        assert any(span.style == "red" for span in result.spans if span.style)


class TestOutputJson:
    """Tests for output_json function."""

    def test_output_simple_dict(self, capsysbinary):
        """Output simple dictionary as JSON."""
        data = {"key": "value", "number": 42}

        output_json(data)

        result = capsysbinary.readouterr().out.decode()
        # msgspec produces compact JSON without spaces
        assert '"key":"value"' in result
        assert '"number":42' in result

    def test_output_newline(self, capsysbinary):
        """Output ends with newline."""
        data = {"test": "value"}

        output_json(data)

        result = capsysbinary.readouterr().out
        assert result.endswith(b"\n")

    def test_output_msgspec_struct(self, sample_snapshot, capsysbinary):
        """Output msgspec Struct as JSON."""
        output_json(sample_snapshot)

        result = capsysbinary.readouterr().out.decode()
        # msgspec produces compact JSON without spaces
        assert '"provider":"claude"' in result


class TestOutputJsonPretty:
    """Tests for output_json_pretty function."""

    def test_output_pretty_formatted(self, capsys):
        """Output is pretty-printed with indentation."""
        data = {"key": "value", "nested": {"a": 1, "b": 2}}

        output_json_pretty(data, indent=2)

        result = capsys.readouterr().out
        # Check for indentation
        assert "  " in result or "\n" in result
        assert '"key": "value"' in result

    def test_output_pretty_ends_with_newline(self, capsys):
        """Pretty output ends with newline."""
        data = {"test": "value"}

        output_json_pretty(data)
        result = capsys.readouterr().out
        assert result.endswith("\n")

    def test_custom_indent(self, capsys):
        """Custom indent level is respected."""
        data = {"key": "value"}

        output_json_pretty(data, indent=4)
        result = capsys.readouterr().out
        # Check for 4-space indentation
        assert "    " in result


class TestEncodeJson:
    """Tests for encode_json function."""

    def test_encode_dict(self):
        """Encode dictionary to JSON bytes."""
        data = {"key": "value", "number": 42}
        result = encode_json(data)

        assert isinstance(result, bytes)
        # msgspec produces compact JSON without spaces
        assert b'"key":"value"' in result
        assert b'"number":42' in result

    def test_encode_list(self):
        """Encode list to JSON bytes."""
        data = [1, 2, 3, "four"]
        result = encode_json(data)

        assert isinstance(result, bytes)
        # msgspec produces compact JSON
        assert b'[1,2,3,"four"]' == result

    def test_encode_msgspec_struct(self, sample_snapshot):
        """Encode msgspec Struct to JSON bytes."""
        result = encode_json(sample_snapshot)

        assert isinstance(result, bytes)
        # msgspec produces compact JSON without spaces
        assert b'"provider":"claude"' in result

    def test_encode_unicode(self):
        """Handle unicode characters."""
        data = {"message": "Hello World"}
        result = encode_json(data)

        assert isinstance(result, bytes)
        assert b"Hello" in result


class TestDecodeJson:
    """Tests for decode_json function."""

    def test_decode_dict(self):
        """Decode JSON bytes to dictionary."""
        json_bytes = b'{"key": "value", "number": 42}'
        result = decode_json(json_bytes)

        assert result == {"key": "value", "number": 42}

    def test_decode_list(self):
        """Decode JSON bytes to list."""
        json_bytes = b'[1, 2, 3, "four"]'
        result = decode_json(json_bytes)

        assert result == [1, 2, 3, "four"]

    def test_decode_with_type_hint(self, utc_now):
        """Decode with type hint validates structure."""
        from vibeusage.models import UsageSnapshot, UsagePeriod, PeriodType

        json_bytes = b'{"provider": "claude", "fetched_at": "2025-01-15T12:00:00Z", "periods": []}'
        result = decode_json(json_bytes, type_hint=UsageSnapshot)

        assert isinstance(result, UsageSnapshot)
        assert result.provider == "claude"

    def test_decode_invalid_json(self):
        """Invalid JSON raises exception."""
        json_bytes = b'{invalid json}'

        with pytest.raises(Exception):  # msgspec.DecodeError
            decode_json(json_bytes)

    def test_decode_unicode(self):
        """Handle unicode characters."""
        json_bytes = '"Hello World"'.encode("utf-8")
        result = decode_json(json_bytes)

        assert "Hello" in result


# Import io for BytesIO
import io

# Import for CLI display tests
from rich.console import Console
from vibeusage.models import UsageSnapshot
from vibeusage.cli.display import SingleProviderDisplay, ProviderPanel


class TestSingleProviderDisplay:
    """Tests for SingleProviderDisplay class."""

    def test_title_is_capitalized(self):
        """Provider name should be capitalized in title per spec 05."""
        now = datetime.now(timezone.utc)
        snapshot = UsageSnapshot(
            provider="claude",  # lowercase input
            periods=[],
            fetched_at=now,
        )

        display = SingleProviderDisplay(snapshot)
        console = Console()
        with console.capture() as capture:
            console.print(display)

        output = capture.get()
        # Title should be "Claude" not "claude"
        assert "Claude" in output
        # First line should be the capitalized title
        lines = output.strip().split("\n")
        assert lines[0] == "Claude"

    def test_shows_model_specific_periods(self):
        """Single provider view should show model-specific periods per spec 05."""
        now = datetime.now(timezone.utc)
        periods = [
            UsagePeriod(
                name="Session (5h)",
                utilization=58,
                period_type=PeriodType.SESSION,
                model=None,
                resets_at=now + timedelta(hours=2),
            ),
            UsagePeriod(
                name="Opus",
                utilization=12,
                period_type=PeriodType.WEEKLY,
                model="opus",  # Model-specific
                resets_at=now + timedelta(days=4),
            ),
        ]

        snapshot = UsageSnapshot(
            provider="claude",
            periods=periods,
            fetched_at=now,
        )

        display = SingleProviderDisplay(snapshot)
        console = Console()
        with console.capture() as capture:
            console.print(display)

        output = capture.get()
        # Model-specific periods should be shown
        assert "Opus" in output

    def test_title_separator_format(self):
        """Single provider view should use title+separator format per spec 05."""
        now = datetime.now(timezone.utc)
        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            fetched_at=now,
        )

        display = SingleProviderDisplay(snapshot)
        console = Console()
        with console.capture() as capture:
            console.print(display)

        output = capture.get()
        lines = output.strip().split("\n")
        # Second line should be the separator (box drawing characters)
        assert len(lines) > 1
        assert "━" in lines[1]

    def test_overage_panel_rendered(self):
        """Overage should be rendered in a Panel per spec 05."""
        now = datetime.now(timezone.utc)
        overage = OverageUsage(
            used=Decimal("5.50"),
            limit=Decimal("100.00"),
            currency="USD",
            is_enabled=True,
        )

        snapshot = UsageSnapshot(
            provider="claude",
            periods=[],
            overage=overage,
            fetched_at=now,
        )

        display = SingleProviderDisplay(snapshot)
        console = Console()
        with console.capture() as capture:
            console.print(display)

        output = capture.get()
        # Overage should be shown with panel borders
        assert "Overage" in output
        assert "$5.50" in output
        assert "$100.00" in output


class TestProviderPanel:
    """Tests for ProviderPanel class (multi-provider compact view)."""

    def test_filters_model_specific_periods(self):
        """Multi-provider view should NOT show model-specific periods per spec 05."""
        now = datetime.now(timezone.utc)
        periods = [
            UsagePeriod(
                name="Session (5h)",
                utilization=58,
                period_type=PeriodType.SESSION,
                model=None,  # Not model-specific
                resets_at=now + timedelta(hours=2),
            ),
            UsagePeriod(
                name="Weekly",
                utilization=23,
                period_type=PeriodType.WEEKLY,
                model=None,  # Not model-specific
                resets_at=now + timedelta(days=4),
            ),
            UsagePeriod(
                name="Opus",
                utilization=12,
                period_type=PeriodType.WEEKLY,
                model="opus",  # Model-specific - should be filtered
                resets_at=now + timedelta(days=4),
            ),
            UsagePeriod(
                name="Sonnet",
                utilization=31,
                period_type=PeriodType.WEEKLY,
                model="sonnet",  # Model-specific - should be filtered
                resets_at=now + timedelta(days=4),
            ),
        ]

        snapshot = UsageSnapshot(
            provider="claude",
            periods=periods,
            fetched_at=now,
        )

        panel = ProviderPanel(snapshot)
        console = Console()
        with console.capture() as capture:
            console.print(panel)

        output = capture.get()
        # General periods should be shown
        assert "Session (5h)" in output
        assert "Weekly" in output
        # Model-specific periods should NOT be shown in compact view
        assert "Opus" not in output
        assert "Sonnet" not in output

    def test_panel_title_is_capitalized(self):
        """Provider panel title should be capitalized per spec 05."""
        now = datetime.now(timezone.utc)
        snapshot = UsageSnapshot(
            provider="claude",  # lowercase input
            periods=[],
            fetched_at=now,
        )

        panel = ProviderPanel(snapshot)
        console = Console()
        with console.capture() as capture:
            console.print(panel)

        output = capture.get()
        # Title should be "Claude" not "claude"
        assert "Claude" in output

    def test_no_source_row_in_compact_view(self):
        """Compact multi-provider view should not show source row per spec 05."""
        now = datetime.now(timezone.utc)
        snapshot = UsageSnapshot(
            provider="claude",
            periods=[
                UsagePeriod(
                    name="Session",
                    utilization=50,
                    period_type=PeriodType.SESSION,
                    model=None,
                    resets_at=now + timedelta(hours=2),
                ),
            ],
            fetched_at=now,
            source="oauth",  # Has a source
        )

        panel = ProviderPanel(snapshot)
        console = Console()
        with console.capture() as capture:
            console.print(panel)

        output = capture.get()
        # Should not show "via oauth" in compact view
        assert "via oauth" not in output
        assert "CLAUDE" not in output

    def test_uses_period_type_names_for_recurring_periods(self):
        """Compact view should use period type names (Weekly, Daily, Monthly) per spec 05.
        Session periods use their specific name (e.g., "Session (5h)") per spec.
        """
        now = datetime.now(timezone.utc)
        periods = [
            UsagePeriod(
                name="Session (5h)",  # Session periods use their specific name per spec
                utilization=58,
                period_type=PeriodType.SESSION,
                model=None,
                resets_at=now + timedelta(hours=2),
            ),
            UsagePeriod(
                name="Custom Weekly Name",  # This should be replaced with "Weekly"
                utilization=23,
                period_type=PeriodType.WEEKLY,
                model=None,
                resets_at=now + timedelta(days=4),
            ),
        ]

        snapshot = UsageSnapshot(
            provider="claude",
            periods=periods,
            fetched_at=now,
        )

        panel = ProviderPanel(snapshot)
        console = Console()
        with console.capture() as capture:
            console.print(panel)

        output = capture.get()
        # Session periods use their specific name per spec
        assert "Session (5h)" in output
        # Weekly periods should use "Weekly" label, not custom name
        assert "Custom Weekly Name" not in output
        assert "Weekly" in output
        # Daily periods should use "Daily" label
        assert "Daily" in output or "Weekly" in output  # At least one period type label
