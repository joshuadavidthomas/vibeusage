"""Tests for JSON output on CLI commands."""
from __future__ import annotations

import json
from datetime import datetime
from datetime import timezone
from decimal import Decimal
from io import StringIO

from vibeusage.cli.commands.usage import output_single_provider_json
from vibeusage.models import OverageUsage
from vibeusage.models import PeriodType
from vibeusage.models import ProviderIdentity
from vibeusage.models import UsagePeriod
from vibeusage.models import UsageSnapshot
from vibeusage.strategies.base import FetchOutcome


class TestSingleProviderJsonOutput:
    """Tests for output_single_provider_json function."""

    def test_single_provider_success(self):
        """output_single_provider_json outputs correct JSON for successful fetch."""
        period = UsagePeriod(
            name="Session",
            utilization=50,
            resets_at=datetime.now(timezone.utc),
            period_type=PeriodType.SESSION,
        )
        identity = ProviderIdentity(
            email="test@example.com",
            organization="Test Org",
            plan="pro",
        )
        snapshot = UsageSnapshot(
            provider="claude",
            fetched_at=datetime.now(timezone.utc),
            identity=identity,
            periods=[period],
            overage=None,
        )
        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=snapshot,
            source="oauth",
            attempts=[],
        )

        # Capture stdout
        import sys

        old_stdout = sys.stdout
        sys.stdout = StringIO()

        try:
            output_single_provider_json(outcome)
            output = sys.stdout.getvalue()
        finally:
            sys.stdout = old_stdout

        # Verify JSON is valid
        data = json.loads(output)
        assert data["provider"] == "claude"
        assert data["source"] == "oauth"
        assert data["cached"] is False
        assert "identity" in data
        assert data["identity"]["email"] == "test@example.com"
        assert "periods" in data
        assert len(data["periods"]) == 1

    def test_single_provider_with_overage(self):
        """output_single_provider_json includes overage data when present."""
        period = UsagePeriod(
            name="Monthly",
            utilization=120,
            resets_at=datetime.now(timezone.utc),
            period_type=PeriodType.MONTHLY,
        )
        overage = OverageUsage(
            used=Decimal("20.0"),
            limit=Decimal("50.0"),
            currency="USD",
            is_enabled=True,
        )
        snapshot = UsageSnapshot(
            provider="claude",
            fetched_at=datetime.now(timezone.utc),
            identity=None,
            periods=[period],
            overage=overage,
        )
        outcome = FetchOutcome(
            provider_id="claude",
            success=True,
            snapshot=snapshot,
            source="web",
            attempts=[],
        )

        # Capture stdout
        import sys

        old_stdout = sys.stdout
        sys.stdout = StringIO()

        try:
            output_single_provider_json(outcome)
            output = sys.stdout.getvalue()
        finally:
            sys.stdout = old_stdout

        # Verify JSON includes overage
        data = json.loads(output)
        assert data["overage"] is not None
        assert data["overage"]["limit"] == 50.0
        assert data["overage"]["used"] == 20.0
        assert data["overage"]["remaining"] == 30.0

    def test_single_provider_error(self):
        """output_single_provider_json outputs error for failed fetch."""
        outcome = FetchOutcome(
            provider_id="claude",
            success=False,
            snapshot=None,
            source=None,
            attempts=[],
            error=Exception("Invalid credentials"),
        )

        # Capture stdout
        import sys

        old_stdout = sys.stdout
        sys.stdout = StringIO()

        try:
            output_single_provider_json(outcome)
            output = sys.stdout.getvalue()
        finally:
            sys.stdout = old_stdout

        # Verify JSON includes error with new ErrorResponse format
        data = json.loads(output)
        assert "error" in data
        assert data["error"]["message"] == "Invalid credentials"
        assert data["error"]["category"] == "unknown"
        assert data["error"]["severity"] == "recoverable"
        # Note: provider field is only included when set on the error
        # Generic Exception doesn't have provider, so it may be omitted


class TestJsonCommandOptions:
    """Tests for --json option on CLI commands."""

    def test_usage_command_accepts_json_option(self):
        """usage command accepts --json option."""
        from typer.testing import CliRunner

        from vibeusage.cli.app import app

        runner = CliRunner()
        result = runner.invoke(app, ["usage", "--help"])
        assert result.exit_code == 0
        assert "--json" in result.output or "-j" in result.output

    def test_status_command_accepts_json_option(self):
        """status command accepts --json option."""
        from typer.testing import CliRunner

        from vibeusage.cli.app import app

        runner = CliRunner()
        result = runner.invoke(app, ["status", "--help"])
        assert result.exit_code == 0
        assert "--json" in result.output or "-j" in result.output

    def test_auth_command_accepts_json_option(self):
        """auth command accepts --json option."""
        from typer.testing import CliRunner

        from vibeusage.cli.app import app

        runner = CliRunner()
        result = runner.invoke(app, ["auth", "--help"])
        assert result.exit_code == 0
        assert "--json" in result.output or "-j" in result.output


class TestJsonOutputFormat:
    """Tests for JSON output format consistency."""

    def test_json_output_is_valid_json(self):
        """JSON output is parseable as valid JSON."""
        period = UsagePeriod(
            name="Test",
            utilization=75,
            resets_at=datetime.now(timezone.utc),
            period_type=PeriodType.DAILY,
        )
        snapshot = UsageSnapshot(
            provider="test",
            fetched_at=datetime.now(timezone.utc),
            identity=None,
            periods=[period],
            overage=None,
        )
        outcome = FetchOutcome(
            provider_id="test",
            success=True,
            snapshot=snapshot,
            source="test",
            attempts=[],
        )

        # Capture stdout
        import sys

        old_stdout = sys.stdout
        sys.stdout = StringIO()

        try:
            output_single_provider_json(outcome)
            output = sys.stdout.getvalue()
        finally:
            sys.stdout = old_stdout

        # Should be valid JSON
        json.loads(output)

    def test_json_output_is_pretty_printed(self):
        """JSON output is pretty-printed with indentation."""
        period = UsagePeriod(
            name="Test",
            utilization=75,
            resets_at=datetime.now(timezone.utc),
            period_type=PeriodType.DAILY,
        )
        snapshot = UsageSnapshot(
            provider="test",
            fetched_at=datetime.now(timezone.utc),
            identity=None,
            periods=[period],
            overage=None,
        )
        outcome = FetchOutcome(
            provider_id="test",
            success=True,
            snapshot=snapshot,
            source="test",
            attempts=[],
        )

        # Capture stdout
        import sys

        old_stdout = sys.stdout
        sys.stdout = StringIO()

        try:
            output_single_provider_json(outcome)
            output = sys.stdout.getvalue()
        finally:
            sys.stdout = old_stdout

        # Check for indentation (pretty-printed)
        assert "  " in output or "    " in output
        assert "\n" in output
