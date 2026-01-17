"""CLI strategy for Claude provider."""
from __future__ import annotations

import asyncio
import re
import shutil
from datetime import UTC
from datetime import datetime

from vibeusage.models import PeriodType
from vibeusage.models import UsagePeriod
from vibeusage.models import UsageSnapshot
from vibeusage.strategies.base import FetchResult
from vibeusage.strategies.base import FetchStrategy

# ANSI escape code pattern for stripping
ANSI_PATTERN = re.compile(r"\x1b\[[0-9;]*m")

# Pattern to match usage output like "█ 45.2% (5-hour session)"
USAGE_PATTERN = re.compile(r"█\s*([\d.]+)%\s*(?:\(([^)]+)\)|\[([^\]]+)\])")


class ClaudeCLIStrategy(FetchStrategy):
    """Fetch Claude usage by delegating to the claude CLI tool."""

    name = "cli"

    # CLI command
    COMMAND = "claude"
    USAGE_ARGS = ["/usage"]

    def is_available(self) -> bool:
        """Check if claude CLI is available."""
        return shutil.which(self.COMMAND) is not None

    async def fetch(self) -> FetchResult:
        """Fetch usage by running claude CLI."""
        if not self.is_available():
            return FetchResult.fail("claude CLI not found in PATH")

        try:
            # Run claude /usage command
            process = await asyncio.create_subprocess_exec(
                self.COMMAND,
                *self.USAGE_ARGS,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            stdout, stderr = await process.communicate()

            if process.returncode != 0:
                error_msg = stderr.decode() if stderr else "Unknown error"
                return FetchResult.fail(f"claude CLI failed: {error_msg}")

            output = stdout.decode()
        except FileNotFoundError:
            return FetchResult.fail("claude CLI not found")
        except Exception as e:
            return FetchResult.fail(f"Failed to run claude CLI: {e}")

        # Parse output
        snapshot = self._parse_cli_output(output)
        if snapshot is None:
            return FetchResult.fail("Failed to parse claude CLI output")

        return FetchResult.ok(snapshot)

    def _parse_cli_output(self, output: str) -> UsageSnapshot | None:
        """Parse usage output from claude CLI.

        Expected format:
        █ 45.2% (5-hour session)
        █ 32.0% (7-day period)
        """
        # Strip ANSI codes
        clean_output = ANSI_PATTERN.sub("", output)

        periods = []
        lines = clean_output.splitlines()

        for line in lines:
            line = line.strip()
            if not line or not line.startswith("█"):
                continue

            match = USAGE_PATTERN.search(line)
            if match:
                try:
                    utilization = int(float(match.group(1)))
                except ValueError:
                    continue

                # Extract period name from parentheses or brackets
                period_name = match.group(2) or match.group(3) or "Usage"
                period_name = period_name.strip()

                # Map to period type
                period_type = self._classify_period(period_name)

                periods.append(
                    UsagePeriod(
                        name=period_name,
                        utilization=utilization,
                        period_type=period_type,
                        resets_at=None,  # CLI doesn't provide reset time
                    )
                )

        if not periods:
            return None

        return UsageSnapshot(
            provider="claude",
            fetched_at=datetime.now(UTC),
            periods=periods,
            overage=None,  # CLI doesn't provide overage
            identity=None,
            status=None,
            source="cli",
        )

    def _classify_period(self, name: str) -> PeriodType:
        """Classify a period name to PeriodType."""
        name_lower = name.lower()

        if "hour" in name_lower or "session" in name_lower:
            return PeriodType.SESSION
        elif "day" in name_lower:
            return PeriodType.DAILY
        elif "week" in name_lower:
            return PeriodType.WEEKLY
        elif "month" in name_lower or "billing" in name_lower:
            return PeriodType.MONTHLY

        return PeriodType.DAILY  # Default
