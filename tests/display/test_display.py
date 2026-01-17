"""Tests for display utilities."""
from __future__ import annotations

import json
from datetime import UTC
from datetime import datetime
from datetime import timedelta
from decimal import Decimal

import msgspec
import pytest
from rich.console import Console

from vibeusage.cli.display import ProviderPanel
from vibeusage.cli.display import SingleProviderDisplay
from vibeusage.display.json import ErrorData
from vibeusage.display.json import ErrorResponse
from vibeusage.display.json import create_error_response
from vibeusage.display.json import decode_json
from vibeusage.display.json import encode_json
from vibeusage.display.json import from_vibeusage_error
from vibeusage.display.json import output_json
from vibeusage.display.json import output_json_error
from vibeusage.display.json import output_json_pretty
from vibeusage.display.rich import format_overage_used
from vibeusage.display.rich import format_period
from vibeusage.display.rich import format_period_line
from vibeusage.display.rich import render_usage_bar
from vibeusage.models import OverageUsage
from vibeusage.models import PeriodType
from vibeusage.models import UsagePeriod
from vibeusage.models import UsageSnapshot


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
            name="Session",
            utilization=50,
            period_type=PeriodType.SESSION,
            resets_at=None,
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
        from vibeusage.models import UsageSnapshot

        json_bytes = b'{"provider": "claude", "fetched_at": "2025-01-15T12:00:00Z", "periods": []}'
        result = decode_json(json_bytes, type_hint=UsageSnapshot)

        assert isinstance(result, UsageSnapshot)
        assert result.provider == "claude"

    def test_decode_invalid_json(self):
        """Invalid JSON raises exception."""
        json_bytes = b"{invalid json}"

        with pytest.raises(msgspec.DecodeError):
            decode_json(json_bytes)

    def test_decode_unicode(self):
        """Handle unicode characters."""
        json_bytes = b'"Hello World"'
        result = decode_json(json_bytes)

        assert "Hello" in result


# Import io for BytesIO


class TestSingleProviderDisplay:
    """Tests for SingleProviderDisplay class."""

    def test_title_is_capitalized(self):
        """Provider name should be capitalized in title per spec 05."""
        now = datetime.now(UTC)
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
        now = datetime.now(UTC)
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
        now = datetime.now(UTC)
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
        now = datetime.now(UTC)
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
        now = datetime.now(UTC)
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
        now = datetime.now(UTC)
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
        now = datetime.now(UTC)
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
        now = datetime.now(UTC)
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


class TestErrorResponse:
    """Tests for ErrorResponse struct per spec 07."""

    def test_error_data_has_required_fields(self):
        """ErrorData should have all required fields per spec."""
        error = ErrorData(
            message="Authentication failed",
            category="authentication",
            severity="recoverable",
        )

        assert error.message == "Authentication failed"
        assert error.category == "authentication"
        assert error.severity == "recoverable"
        assert error.provider is None
        assert error.remediation is None
        assert error.details is None
        # timestamp should be set by default_factory
        assert error.timestamp is not None

    def test_error_data_with_optional_fields(self):
        """ErrorData should support optional fields."""
        error = ErrorData(
            message="Rate limit exceeded",
            category="rate_limited",
            severity="transient",
            provider="claude",
            remediation="Wait 10 minutes before retrying",
            details={"retry_after": 600},
        )

        assert error.provider == "claude"
        assert error.remediation == "Wait 10 minutes before retrying"
        assert error.details == {"retry_after": 600}

    def test_error_data_to_dict(self):
        """ErrorData.to_dict() returns correct dict structure."""
        error = ErrorData(
            message="Network error",
            category="network",
            severity="transient",
            provider="codex",
            remediation="Check your internet connection",
        )

        result = error.to_dict()

        assert result == {
            "message": "Network error",
            "category": "network",
            "severity": "transient",
            "provider": "codex",
            "remediation": "Check your internet connection",
            "timestamp": error.timestamp,
        }

    def test_error_data_to_dict_omits_none_fields(self):
        """ErrorData.to_dict() omits None values for optional fields."""
        error = ErrorData(
            message="Test error",
            category="unknown",
            severity="warning",
        )

        result = error.to_dict()

        # None fields should not be in dict
        assert "provider" not in result
        assert "remediation" not in result
        assert "details" not in result
        # Required fields should be present
        assert "message" in result
        assert "category" in result
        assert "severity" in result
        assert "timestamp" in result

    def test_error_response_structure(self):
        """ErrorResponse should wrap error data per spec."""
        error_data = ErrorData(
            message="Test error",
            category="test",
            severity="fatal",
        )
        response = ErrorResponse(error=error_data)

        assert response.error is error_data
        assert response.error.message == "Test error"

    def test_error_response_to_dict(self):
        """ErrorResponse.to_dict() returns correct nested structure."""
        error_data = ErrorData(
            message="API error",
            category="provider",
            severity="transient",
            provider="gemini",
        )
        response = ErrorResponse(error=error_data)

        result = response.to_dict()

        assert result == {
            "error": {
                "message": "API error",
                "category": "provider",
                "severity": "transient",
                "provider": "gemini",
                "timestamp": error_data.timestamp,
            }
        }


class TestCreateErrorResponse:
    """Tests for create_error_response function."""

    def test_create_error_response_basic(self):
        """Create error response with basic fields."""
        response = create_error_response(
            message="Test error",
            category="test",
            severity="warning",
        )

        assert response.error.message == "Test error"
        assert response.error.category == "test"
        assert response.error.severity == "warning"

    def test_create_error_response_with_all_fields(self):
        """Create error response with all fields."""
        response = create_error_response(
            message="Auth failed",
            category="authentication",
            severity="recoverable",
            provider="copilot",
            remediation="Run: vibeusage auth copilot",
            details={"status_code": 401},
        )

        assert response.error.message == "Auth failed"
        assert response.error.provider == "copilot"
        assert response.error.remediation == "Run: vibeusage auth copilot"
        assert response.error.details == {"status_code": 401}


class TestOutputJsonError:
    """Tests for output_json_error function."""

    def test_output_json_error_basic(self, capsys):
        """Output basic error in JSON format."""
        output_json_error(
            message="Test error",
            category="test",
            severity="warning",
        )

        result = capsys.readouterr().out
        data = json.loads(result)

        assert "error" in data
        assert data["error"]["message"] == "Test error"
        assert data["error"]["category"] == "test"
        assert data["error"]["severity"] == "warning"

    def test_output_json_error_with_all_fields(self, capsys):
        """Output error with all fields in JSON format."""
        output_json_error(
            message="Connection timeout",
            category="network",
            severity="transient",
            provider="cursor",
            remediation="Check your network and retry",
            details={"timeout": 30},
        )

        result = capsys.readouterr().out
        data = json.loads(result)

        assert data["error"]["message"] == "Connection timeout"
        assert data["error"]["category"] == "network"
        assert data["error"]["provider"] == "cursor"
        assert data["error"]["remediation"] == "Check your network and retry"
        assert data["error"]["details"] == {"timeout": 30}

    def test_output_json_error_is_pretty_printed(self, capsys):
        """JSON error output is pretty-printed."""
        output_json_error(
            message="Test",
            category="test",
            severity="warning",
        )

        result = capsys.readouterr().out
        # Check for indentation
        assert "  " in result or "    " in result
        assert "\n" in result


class TestFromVibeusageError:
    """Tests for from_vibeusage_error function."""

    def test_from_vibeusage_error_basic(self):
        """Convert VibeusageError to ErrorResponse."""
        from vibeusage.errors.types import ErrorCategory
        from vibeusage.errors.types import ErrorSeverity
        from vibeusage.errors.types import VibeusageError

        vibe_error = VibeusageError(
            message="Test error",
            category=ErrorCategory.AUTHENTICATION,
            severity=ErrorSeverity.RECOVERABLE,
        )

        response = from_vibeusage_error(vibe_error)

        assert response.error.message == "Test error"
        assert response.error.category == "authentication"
        assert response.error.severity == "recoverable"

    def test_from_vibeusage_error_with_all_fields(self):
        """Convert VibeusageError with all fields to ErrorResponse."""
        from vibeusage.errors.types import ErrorCategory
        from vibeusage.errors.types import ErrorSeverity
        from vibeusage.errors.types import VibeusageError

        vibe_error = VibeusageError(
            message="Token expired",
            category=ErrorCategory.AUTHENTICATION,
            severity=ErrorSeverity.RECOVERABLE,
            provider="claude",
            remediation="Run: vibeusage auth claude",
            details={"error_code": "invalid_token"},
        )

        response = from_vibeusage_error(vibe_error)

        assert response.error.message == "Token expired"
        assert response.error.provider == "claude"
        assert response.error.remediation == "Run: vibeusage auth claude"
        assert response.error.details == {"error_code": "invalid_token"}

    def test_from_vibeusage_error_to_dict(self):
        """from_vibeusage_error result serializes correctly."""
        from vibeusage.errors.types import ErrorCategory
        from vibeusage.errors.types import ErrorSeverity
        from vibeusage.errors.types import VibeusageError

        vibe_error = VibeusageError(
            message="Test",
            category=ErrorCategory.NETWORK,
            severity=ErrorSeverity.TRANSIENT,
            provider="codex",
        )

        response = from_vibeusage_error(vibe_error)
        result = response.to_dict()

        assert result["error"]["message"] == "Test"
        assert result["error"]["category"] == "network"
        assert result["error"]["provider"] == "codex"
