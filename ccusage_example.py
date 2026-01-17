#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "httpx",
#     "platformdirs",
#     "rich",
#     "typer",
# ]
# ///
from __future__ import annotations

import json
from dataclasses import dataclass
from datetime import datetime
from datetime import timedelta
from decimal import Decimal
from enum import StrEnum
from pathlib import Path
from typing import ClassVar

import httpx
import platformdirs
import typer
from rich.console import Console
from rich.console import ConsoleOptions
from rich.console import RenderResult
from rich.panel import Panel
from rich.table import Table

APP_NAME = "ccusage"
CONFIG_DIR = Path(platformdirs.user_config_dir(APP_NAME))
CACHE_DIR = Path(platformdirs.user_cache_dir(APP_NAME))

SESSION_KEY_FILE = CONFIG_DIR / "session-key"
ORG_ID_CACHE = CACHE_DIR / "org-id"


console = Console()


class PeriodType(StrEnum):
    """Usage period types with their durations."""
    SESSION = "session"  # 5 hours
    WEEKLY = "weekly"    # 7 days


@dataclass
class UsagePeriod:
    """A usage period (5-hour session or 7-day weekly)."""

    name: str
    utilization: int  # 0-100 percentage
    resets_at: datetime | None = None
    period_type: PeriodType | None = None

    # Duration constants
    SESSION_HOURS: ClassVar[float] = 5.0
    WEEKLY_HOURS: ClassVar[float] = 7.0 * 24.0

    def _elapsed_hours(self) -> float | None:
        """Calculate hours elapsed since period started."""
        if not self.resets_at or not self.period_type:
            return None
        now = datetime.now(self.resets_at.tzinfo)
        total_hours = (
            self.SESSION_HOURS if self.period_type == PeriodType.SESSION
            else self.WEEKLY_HOURS
        )
        start_time = self.resets_at - timedelta(hours=total_hours)
        elapsed = (now - start_time).total_seconds() / 3600.0
        return max(0.0, min(elapsed, total_hours))

    def _elapsed_ratio(self) -> float | None:
        """Calculate ratio of time elapsed (0.0 to 1.0)."""
        if not self.period_type:
            return None
        elapsed = self._elapsed_hours()
        if elapsed is None:
            return None
        total_hours = (
            self.SESSION_HOURS if self.period_type == PeriodType.SESSION
            else self.WEEKLY_HOURS
        )
        return elapsed / total_hours

    def _pace_ratio(self) -> float | None:
        """Calculate usage pace ratio."""
        elapsed_ratio = self._elapsed_ratio()
        if elapsed_ratio is None or elapsed_ratio <= 0:
            return None
        expected_utilization = elapsed_ratio * 100.0
        if expected_utilization <= 0:
            return None
        return self.utilization / expected_utilization

    @classmethod
    def from_data(cls, name: str, data: dict | None, period_type: PeriodType | None = None) -> UsagePeriod | None:
        if data is None:
            return None
        resets_at = None
        if reset_str := data.get("resets_at"):
            resets_at = datetime.fromisoformat(reset_str.replace("Z", "+00:00"))
        return cls(
            name=name,
            utilization=int(data["utilization"]),
            resets_at=resets_at,
            period_type=period_type,
        )


@dataclass
class OverageUsage:
    """Extra/overage usage tracking."""

    used_credits: Decimal
    monthly_limit: Decimal
    currency: str
    is_enabled: bool

    @classmethod
    def from_data(cls, data: dict | None) -> OverageUsage | None:
        if not data:
            return None
        return cls(
            used_credits=Decimal(str(data["used_credits"])),
            monthly_limit=Decimal(str(data["monthly_credit_limit"])),
            currency=data["currency"],
            is_enabled=data["is_enabled"],
        )


@dataclass
class ClaudeUsage:
    """Complete usage data from Claude API."""

    session: UsagePeriod | None  # 5-hour window
    weekly: UsagePeriod | None  # 7-day all models
    opus_weekly: UsagePeriod | None = None  # 7-day Opus only
    sonnet_weekly: UsagePeriod | None = None  # 7-day Sonnet only
    overage: OverageUsage | None = None

    @classmethod
    def from_data(
        cls, usage_data: dict, overage_data: dict | None = None
    ) -> ClaudeUsage:
        return cls(
            session=UsagePeriod.from_data("Session (5h)", usage_data.get("five_hour"), PeriodType.SESSION),
            weekly=UsagePeriod.from_data("Weekly", usage_data.get("seven_day"), PeriodType.WEEKLY),
            opus_weekly=UsagePeriod.from_data(
                "Opus Weekly", usage_data.get("seven_day_opus"), PeriodType.WEEKLY
            ),
            sonnet_weekly=UsagePeriod.from_data(
                "Sonnet Weekly", usage_data.get("seven_day_sonnet"), PeriodType.WEEKLY
            ),
            overage=OverageUsage.from_data(overage_data),
        )

    def _format_reset_time(self, period: UsagePeriod) -> str:
        if not period.resets_at:
            return ""
        now = datetime.now(period.resets_at.tzinfo)
        delta = period.resets_at - now
        total_hours = int(delta.total_seconds() // 3600)
        minutes = int((delta.total_seconds() % 3600) // 60)

        if total_hours >= 24:
            days = total_hours // 24
            hours = total_hours % 24
            return f"resets in {days}d {hours}h"
        elif total_hours > 0:
            return f"resets in {total_hours}h {minutes}m"
        else:
            return f"resets in {minutes}m"

    def _get_usage_color(self, period: UsagePeriod) -> str:
        """Get color based on usage pace or fixed thresholds for early periods."""
        # Fallback: missing period type or resets_at
        if not period.period_type or not period.resets_at:
            return "green" if period.utilization < 50 else "yellow" if period.utilization < 80 else "red"

        # Edge case: less than 10% elapsed (too volatile)
        elapsed_ratio = period._elapsed_ratio()
        if elapsed_ratio is None or elapsed_ratio < 0.10:
            return "green" if period.utilization < 50 else "yellow" if period.utilization < 80 else "red"

        # Pace-based coloring
        pace_ratio = period._pace_ratio()
        if pace_ratio is None:
            return "green"

        if pace_ratio <= 1.15:
            return "green"
        elif pace_ratio <= 1.30:
            return "yellow"
        else:
            return "red"

    def _format_usage(self, period: UsagePeriod) -> str:
        color = self._get_usage_color(period)
        bar = "█" * (period.utilization // 5) + "░" * (20 - period.utilization // 5)
        return f"[{color}]{bar} {period.utilization}%[/{color}]"

    def _make_usage_grid(self) -> Table:
        """Create a grid with consistent column widths."""
        grid = Table.grid(padding=(0, 2))
        grid.add_column(min_width=12)  # Label
        grid.add_column(min_width=24)  # Usage bar
        grid.add_column(justify="right")  # Reset time
        return grid

    def __rich_console__(
        self, console: Console, options: ConsoleOptions
    ) -> RenderResult:
        grid = self._make_usage_grid()

        if self.session:
            grid.add_row(
                "[bold]Session (5h)[/bold]",
                self._format_usage(self.session),
                f"[dim]{self._format_reset_time(self.session)}[/dim]",
            )
            grid.add_row("")

        weekly_periods = [
            ("All Models", self.weekly),
            ("Opus", self.opus_weekly),
            ("Sonnet", self.sonnet_weekly),
        ]
        weekly_periods = [(name, p) for name, p in weekly_periods if p is not None]

        if weekly_periods:
            grid.add_row("[bold]Weekly[/bold]", "", "")
            for name, period in weekly_periods:
                grid.add_row(
                    f"  {name}",
                    self._format_usage(period),
                    f"[dim]{self._format_reset_time(period)}[/dim]",
                )

        yield ""
        yield grid

        if self.overage:
            yield ""
            yield Panel(
                f"Extra Usage: ${self.overage.used_credits:.2f} / "
                f"${self.overage.monthly_limit:.2f} {self.overage.currency}",
                title="Overage",
                border_style="blue",
            )


class ClaudeAI:
    """Claude AI API client."""

    def __init__(self, session_key: str):
        self.client = httpx.Client(
            base_url="https://claude.ai/api",
            headers={
                "Cookie": f"sessionKey={session_key}",
                "User-Agent": "Mozilla/5.0 (X11; Linux x86_64; rv:137.0) Gecko/20100101 Firefox/137.0",
                "Accept": "application/json",
                "Accept-Language": "en-US,en;q=0.5",
            },
            timeout=10.0,
        )
        self.org_id: str | None = None

    def _get(self, url: str, allow_404: bool = False) -> dict | None:
        """Make a GET request with error handling."""
        try:
            response = self.client.get(url)
            response.raise_for_status()
            return response.json()
        except httpx.HTTPStatusError as e:
            status = e.response.status_code
            if status == 404 and allow_404:
                return None
            elif status == 401:
                raise SystemExit(
                    "Error: Invalid or expired session key.\n\n"
                    "Get a new session key from browser cookies and update:\n"
                    "  ccusage key set"
                ) from e
            elif status == 403:
                raise SystemExit(
                    "Error: Access denied. Your account may not have API access."
                ) from e
            elif status == 429:
                raise SystemExit(
                    "Error: Rate limited. Please wait a moment and try again."
                ) from e
            else:
                raise SystemExit(f"Error: API request failed ({status})") from e

    def get_org_id(self) -> str:
        """Get org ID from cache or fetch from API."""
        if self.org_id:
            return self.org_id

        if ORG_ID_CACHE.exists():
            self.org_id = ORG_ID_CACHE.read_text().strip()
            return self.org_id

        orgs = self._get("/organizations")
        if not orgs:
            raise SystemExit("Error: No organizations found")

        # Find org with chat capability (Claude Max subscription)
        for org in orgs:
            if "chat" in org.get("capabilities", []):
                self.org_id = org["uuid"]
                break
        else:
            # Fall back to first org
            self.org_id = orgs[0]["uuid"]

        CACHE_DIR.mkdir(parents=True, exist_ok=True)
        ORG_ID_CACHE.write_text(self.org_id)

        return self.org_id

    def fetch_usage(self) -> dict:
        """Fetch usage data for the organization."""
        org_id = self.get_org_id()
        data = self._get(f"/organizations/{org_id}/usage")
        assert data is not None  # Usage endpoint should always return data
        return data

    def fetch_overage(self) -> dict | None:
        """Fetch overage/extra usage data. Returns None if not available."""
        org_id = self.get_org_id()
        data = self._get(f"/organizations/{org_id}/overage_spend_limit", allow_404=True)
        if data and data.get("is_enabled"):
            return data
        return None


app = typer.Typer(
    help="Claude Code usage tracker",
    invoke_without_command=True,
    no_args_is_help=False,
)


@app.callback(invoke_without_command=True)
def main(
    ctx: typer.Context,
    clear_cache: bool = typer.Option(
        False, "--clear-cache", help="Clear cached org ID"
    ),
    output_json: bool = typer.Option(False, "--json", help="Output as JSON"),
) -> None:
    """Show Claude Code subscription usage."""
    if ctx.invoked_subcommand is not None:
        return

    if clear_cache and ORG_ID_CACHE.exists():
        ORG_ID_CACHE.unlink()
        if not output_json:
            console.print("[dim]Cache cleared[/dim]")

    if not SESSION_KEY_FILE.exists():
        raise SystemExit(
            "Error: No session key found.\n\n"
            "Run 'ccusage key set' to configure your session key.\n"
            "See 'ccusage --help' for instructions on getting your session key."
        )

    api = ClaudeAI(SESSION_KEY_FILE.read_text().strip())

    if output_json:
        usage_data = api.fetch_usage()
        overage_data = api.fetch_overage()
        output = {"usage": usage_data, "overage": overage_data}
        print(json.dumps(output))
    else:
        with console.status("[bold]Fetching usage...[/bold]"):
            usage_data = api.fetch_usage()
            overage_data = api.fetch_overage()

        usage = ClaudeUsage.from_data(usage_data, overage_data)
        console.print(usage)


key_app = typer.Typer(help="Manage the Claude session key")
app.add_typer(key_app, name="key")


@key_app.callback(invoke_without_command=True)
def key_read(ctx: typer.Context) -> None:
    """Show the current session key (masked)."""
    if ctx.invoked_subcommand is not None:
        return

    if SESSION_KEY_FILE.exists():
        console.print(f"[dim]Key stored in {SESSION_KEY_FILE}[/dim]")
        key = SESSION_KEY_FILE.read_text().strip()
        console.print(f"{key[:20]}...{key[-10:]}")
    else:
        console.print("[yellow]No session key configured[/yellow]")
        console.print("[dim]Run 'ccusage key set' to set your key[/dim]")


@key_app.command(name="set")
def key_set(
    value: str = typer.Argument(None, help="Session key to save"),
) -> None:
    """Set the Claude session key."""
    if value is None:
        value = typer.prompt("Enter your Claude session key", hide_input=True)

    if not value.startswith("sk-ant-sid01-"):
        console.print(
            "[yellow]Warning: Key doesn't start with 'sk-ant-sid01-'[/yellow]"
        )
        if not typer.confirm("Save anyway?"):
            raise typer.Abort()

    CONFIG_DIR.mkdir(parents=True, exist_ok=True)
    SESSION_KEY_FILE.write_text(value)
    SESSION_KEY_FILE.chmod(0o600)
    console.print(f"[green]Session key saved to {SESSION_KEY_FILE}[/green]")


@key_app.command(name="delete")
def key_delete() -> None:
    """Delete the saved session key."""
    if SESSION_KEY_FILE.exists():
        SESSION_KEY_FILE.unlink()
        console.print("[green]Session key deleted[/green]")
    else:
        console.print("[dim]No session key file to delete[/dim]")


if __name__ == "__main__":
    app()
