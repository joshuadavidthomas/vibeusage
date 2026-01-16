# Spec 05: CLI Interface

**Status**: Draft
**Dependencies**: 01-architecture, 02-data-models, 04-providers
**Dependents**: 06-configuration, 07-error-handling

## Overview

This specification defines the command-line interface for vibeusage, including command structure, output formatting, and user interaction patterns. The CLI uses Rich for terminal rendering and Typer for command parsing.

## Design Goals

1. **Terminal-First**: Optimized for shell workflows with clean, scannable output
2. **Pace-Based Feedback**: Visual indicators based on usage pace, not arbitrary thresholds
3. **Progressive Disclosure**: Simple default output, detailed info available via flags
4. **Scriptable**: JSON output mode for integration with other tools
5. **Consistent UX**: Uniform patterns across all providers

## Dependencies

```toml
# pyproject.toml
dependencies = [
    "httpx>=0.27",
    "msgspec>=0.18",
    "rich>=13.0",
    "typer>=0.12",
    "platformdirs>=4.0",
]
```

---

## Command Structure

### Top-Level Commands

```
vibeusage                  # Show usage for all enabled providers
vibeusage <provider>       # Show usage for specific provider
vibeusage auth <provider>  # Authenticate with a provider
vibeusage status           # Show provider health status
vibeusage config           # Manage configuration
vibeusage --version        # Show version
vibeusage --help           # Show help
```

### Provider Commands

```
vibeusage claude           # Claude usage
vibeusage codex            # Codex/OpenAI usage
vibeusage copilot          # GitHub Copilot usage
vibeusage cursor           # Cursor usage
vibeusage gemini           # Gemini usage
```

### Auth Subcommands

```
vibeusage auth claude      # Authenticate with Claude
vibeusage auth codex       # Authenticate with Codex
vibeusage auth copilot     # Start Copilot device flow
vibeusage auth cursor      # Set Cursor session cookie
vibeusage auth gemini      # Authenticate with Gemini
vibeusage auth --status    # Show auth status for all providers
```

### Config Subcommands

```
vibeusage config show      # Show current configuration
vibeusage config path      # Show config/cache paths
vibeusage config reset     # Reset to defaults
```

---

## Global Options

Options available on all commands:

| Option | Short | Description |
|--------|-------|-------------|
| `--json` | `-j` | Output as JSON (for scripting) |
| `--no-color` | | Disable colored output |
| `--verbose` | `-v` | Show detailed output including source |
| `--quiet` | `-q` | Minimal output (errors only) |
| `--help` | `-h` | Show help message |

### Implementation

Typer doesn't natively support async commands. We use `ATyper`, a custom wrapper that automatically handles async functions (see spec 01 for full implementation).

```python
import typer
from rich.console import Console

from vibeusage.cli.atyper import ATyper

app = ATyper(
    name="vibeusage",
    help="Track usage across agentic LLM providers",
    no_args_is_help=False,
    add_completion=True,
)

console = Console()

@app.callback(invoke_without_command=True)
async def main(
    ctx: typer.Context,
    json_output: bool = typer.Option(False, "--json", "-j", help="Output as JSON"),
    no_color: bool = typer.Option(False, "--no-color", help="Disable colors"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Verbose output"),
    quiet: bool = typer.Option(False, "--quiet", "-q", help="Minimal output"),
    version: bool = typer.Option(False, "--version", help="Show version"),
) -> None:
    """Show usage for all enabled providers."""
    if version:
        from vibeusage import __version__
        console.print(f"vibeusage {__version__}")
        raise typer.Exit()

    if ctx.invoked_subcommand is not None:
        return

    # Default action: show all providers
    await show_all_providers(json_output=json_output, verbose=verbose)
```

---

## Output Format

### Usage Display

The primary output shows usage periods with visual progress bars and reset times.

#### Single Provider View

```
Claude
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Session (5h)  ████████████░░░░░░░░ 58%    resets in 2h 15m

Weekly
  All Models  ████░░░░░░░░░░░░░░░░ 23%    resets in 4d 12h
  Opus        ██░░░░░░░░░░░░░░░░░░ 12%    resets in 4d 12h
  Sonnet      ██████░░░░░░░░░░░░░░ 31%    resets in 4d 12h

╭─ Overage ──────────────────────────────────────────────╮
│ Extra Usage: $5.50 / $100.00 USD                       │
╰────────────────────────────────────────────────────────╯
```

#### Multi-Provider View

```
╭─ Claude ───────────────────────────────────────────────╮
│ Session (5h)  ████████████░░░░░░░░ 58%   resets in 2h  │
│ Weekly        ████░░░░░░░░░░░░░░░░ 23%   resets in 4d  │
╰────────────────────────────────────────────────────────╯
╭─ Codex ────────────────────────────────────────────────╮
│ Session       ██████████░░░░░░░░░░ 50%   resets in 3h  │
│ Weekly        ██░░░░░░░░░░░░░░░░░░ 12%   resets in 5d  │
╰────────────────────────────────────────────────────────╯
╭─ Copilot ──────────────────────────────────────────────╮
│ Premium       ████████░░░░░░░░░░░░ 42%   resets in 18d │
╰────────────────────────────────────────────────────────╯
```

### Progress Bar

The progress bar uses Unicode block characters:

```python
def render_usage_bar(utilization: int, width: int = 20) -> str:
    """Render a usage bar with filled and empty blocks."""
    filled = utilization * width // 100
    empty = width - filled
    return "█" * filled + "░" * empty
```

### Color Scheme

Colors are determined by pace ratio, not fixed thresholds:

| Pace Ratio | Color | Meaning |
|------------|-------|---------|
| ≤ 1.15 | Green | On or under pace |
| 1.15 - 1.30 | Yellow | Slightly over pace |
| > 1.30 | Red | Significantly over pace |

**Fallback** (early period or missing reset time):

| Utilization | Color |
|-------------|-------|
| < 50% | Green |
| 50-80% | Yellow |
| > 80% | Red |

#### Implementation

```python
from rich.style import Style
from vibeusage.models import UsagePeriod

def get_usage_style(period: UsagePeriod) -> Style:
    """Get Rich style based on usage pace."""
    pace = period.pace_ratio()

    if pace is None:
        # Fallback to threshold-based
        if period.utilization < 50:
            return Style(color="green")
        elif period.utilization < 80:
            return Style(color="yellow")
        else:
            return Style(color="red")

    # Pace-based coloring
    if pace <= 1.15:
        return Style(color="green")
    elif pace <= 1.30:
        return Style(color="yellow")
    else:
        return Style(color="red")

def format_usage_line(period: UsagePeriod) -> Text:
    """Format a usage period as a Rich Text object."""
    style = get_usage_style(period)
    bar = render_usage_bar(period.utilization)

    text = Text()
    text.append(bar, style=style)
    text.append(f" {period.utilization}%", style=style)

    return text
```

### Reset Time Formatting

Reset times are displayed as countdown by default:

| Time Until Reset | Display |
|------------------|---------|
| > 24 hours | `4d 12h` |
| 1-24 hours | `3h 45m` |
| < 1 hour | `45m` |
| < 1 minute | `<1m` |

```python
from datetime import timedelta

def format_reset_time(delta: timedelta | None) -> str:
    """Format reset time as countdown."""
    if delta is None:
        return ""

    total_seconds = int(delta.total_seconds())
    if total_seconds <= 0:
        return "<1m"

    days, remainder = divmod(total_seconds, 86400)
    hours, remainder = divmod(remainder, 3600)
    minutes = remainder // 60

    if days > 0:
        return f"{days}d {hours}h"
    elif hours > 0:
        return f"{hours}h {minutes}m"
    elif minutes > 0:
        return f"{minutes}m"
    else:
        return "<1m"
```

---

## Rich Renderable Components

### UsageDisplay

A Rich-compatible class for rendering usage data:

```python
from rich.console import Console, ConsoleOptions, RenderResult
from rich.table import Table
from rich.panel import Panel
from rich.text import Text
from vibeusage.models import UsageSnapshot

class UsageDisplay:
    """Rich-renderable usage display."""

    def __init__(self, snapshot: UsageSnapshot, verbose: bool = False):
        self.snapshot = snapshot
        self.verbose = verbose

    def __rich_console__(
        self, console: Console, options: ConsoleOptions
    ) -> RenderResult:
        # Create usage grid
        grid = Table.grid(padding=(0, 2))
        grid.add_column(min_width=14)  # Label
        grid.add_column(min_width=24)  # Usage bar + percentage
        grid.add_column(justify="right")  # Reset time

        # Primary period (session)
        primary = self.snapshot.primary_period()
        if primary:
            grid.add_row(
                Text(primary.name, style="bold"),
                format_usage_line(primary),
                Text(f"resets in {format_reset_time(primary.time_until_reset())}", style="dim"),
            )
            grid.add_row("")  # Spacer

        # Secondary period (weekly) with model-specific
        secondary = self.snapshot.secondary_period()
        model_periods = self.snapshot.model_periods()

        if secondary or model_periods:
            grid.add_row(Text("Weekly", style="bold"), "", "")

            if secondary:
                grid.add_row(
                    "  All Models",
                    format_usage_line(secondary),
                    Text(f"resets in {format_reset_time(secondary.time_until_reset())}", style="dim"),
                )

            for period in model_periods:
                grid.add_row(
                    f"  {period.name}",
                    format_usage_line(period),
                    Text(f"resets in {format_reset_time(period.time_until_reset())}", style="dim"),
                )

        yield ""
        yield grid

        # Overage panel
        if self.snapshot.overage and self.snapshot.overage.is_enabled:
            overage = self.snapshot.overage
            yield ""
            yield Panel(
                f"Extra Usage: ${overage.used:.2f} / ${overage.limit:.2f} {overage.currency}",
                title="Overage",
                border_style="blue",
            )

        # Verbose: show source and fetch time
        if self.verbose:
            yield ""
            info = []
            if self.snapshot.source:
                info.append(f"Source: {self.snapshot.source}")
            info.append(f"Fetched: {self.snapshot.fetched_at.strftime('%H:%M:%S')}")
            if self.snapshot.identity and self.snapshot.identity.email:
                info.append(f"Account: {self.snapshot.identity.email}")
            yield Text(" | ".join(info), style="dim")
```

### ProviderPanel

Wraps provider usage in a panel for multi-provider view:

```python
class ProviderPanel:
    """Provider usage wrapped in a panel."""

    def __init__(self, snapshot: UsageSnapshot):
        self.snapshot = snapshot

    def __rich_console__(
        self, console: Console, options: ConsoleOptions
    ) -> RenderResult:
        # Compact grid for panel view
        grid = Table.grid(padding=(0, 2))
        grid.add_column(min_width=12)
        grid.add_column(min_width=22)
        grid.add_column(justify="right")

        for period in self.snapshot.periods:
            if period.model is None:  # Skip model-specific in compact view
                grid.add_row(
                    period.name,
                    format_usage_line(period),
                    Text(f"resets in {format_reset_time(period.time_until_reset())}", style="dim"),
                )

        # Provider name from metadata
        from vibeusage.providers import get_provider
        provider = get_provider(self.snapshot.provider)
        title = provider.metadata.name if provider else self.snapshot.provider.title()

        yield Panel(grid, title=title, border_style="dim")
```

### StatusDisplay

For showing provider health status:

```python
from vibeusage.models import ProviderStatus, StatusLevel

STATUS_SYMBOLS = {
    StatusLevel.OPERATIONAL: ("●", "green"),
    StatusLevel.DEGRADED: ("◐", "yellow"),
    StatusLevel.PARTIAL_OUTAGE: ("◑", "yellow"),
    StatusLevel.MAJOR_OUTAGE: ("○", "red"),
    StatusLevel.UNKNOWN: ("?", "dim"),
}

class StatusDisplay:
    """Rich-renderable provider status."""

    def __init__(self, statuses: dict[str, ProviderStatus]):
        self.statuses = statuses

    def __rich_console__(
        self, console: Console, options: ConsoleOptions
    ) -> RenderResult:
        table = Table(title="Provider Status", box=None)
        table.add_column("Provider", style="bold")
        table.add_column("Status")
        table.add_column("Details", style="dim")

        for provider_id, status in self.statuses.items():
            symbol, color = STATUS_SYMBOLS.get(
                status.level, STATUS_SYMBOLS[StatusLevel.UNKNOWN]
            )
            provider = get_provider(provider_id)
            name = provider.metadata.name if provider else provider_id.title()

            table.add_row(
                name,
                Text(f"{symbol} {status.level.value.replace('_', ' ').title()}", style=color),
                status.description or "",
            )

        yield table
```

---

## JSON Output

When `--json` is specified, output raw JSON suitable for parsing:

### Single Provider

```json
{
  "provider": "claude",
  "fetched_at": "2026-01-16T10:30:00+00:00",
  "periods": [
    {
      "name": "Session (5h)",
      "utilization": 58,
      "period_type": "session",
      "resets_at": "2026-01-16T12:45:00+00:00"
    },
    {
      "name": "Weekly",
      "utilization": 23,
      "period_type": "weekly",
      "resets_at": "2026-01-20T00:00:00+00:00"
    }
  ],
  "overage": {
    "used": "5.50",
    "limit": "100.00",
    "currency": "USD",
    "is_enabled": true
  },
  "source": "oauth"
}
```

### Multiple Providers

```json
{
  "providers": {
    "claude": { ... },
    "codex": { ... },
    "copilot": { ... }
  },
  "fetched_at": "2026-01-16T10:30:00+00:00",
  "errors": {
    "cursor": "Session expired"
  }
}
```

### Implementation

msgspec provides native JSON serialization for all data structures. No manual `to_dict()` conversion needed.

```python
import msgspec

def output_json(data: object) -> None:
    """Print JSON output to stdout using msgspec."""
    # msgspec.json.encode returns bytes, decode to str for printing
    json_bytes = msgspec.json.encode(data)
    print(json_bytes.decode("utf-8"))

@app.command()
async def claude(
    ctx: typer.Context,
    json_output: bool = typer.Option(False, "--json", "-j"),
) -> None:
    """Show Claude usage."""
    from vibeusage.providers import get_provider

    provider = get_provider("claude")
    result = await provider.fetch()

    if json_output:
        if result.success:
            output_json(result.snapshot)
        else:
            output_json({"error": result.error})
            raise typer.Exit(1)
    else:
        if result.success:
            console.print(UsageDisplay(result.snapshot))
        else:
            console.print(f"[red]Error:[/red] {result.error}")
            raise typer.Exit(1)
```

### Pretty-Printed JSON

For human-readable JSON output, use a custom encoder:

```python
# Create encoder for pretty-printed output
_pretty_encoder = msgspec.json.Encoder(indent=2)

def output_json_pretty(data: object) -> None:
    """Print pretty-printed JSON output."""
    json_bytes = _pretty_encoder.encode(data)
    print(json_bytes.decode("utf-8"))
```

**Note**: msgspec's `indent` option is available in msgspec >= 0.18.

---

## Interactive Authentication

### Device Flow (Copilot)

```python
from rich.progress import Progress, SpinnerColumn, TextColumn

async def device_flow_auth(provider_id: str) -> None:
    """Handle GitHub device flow authentication."""
    from vibeusage.auth.device_flow import start_device_flow

    console.print(f"[bold]Authenticating with {provider_id}...[/bold]")
    console.print()

    # Start device flow
    flow = await start_device_flow(provider_id)

    # Show user code
    console.print(f"Please visit: [link={flow.verification_uri}]{flow.verification_uri}[/link]")
    console.print(f"Enter code: [bold cyan]{flow.user_code}[/bold cyan]")
    console.print()

    # Poll for completion
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        console=console,
    ) as progress:
        task = progress.add_task("Waiting for authorization...", total=None)

        try:
            token = await flow.wait_for_token()
            progress.update(task, description="[green]Authorized!")
            console.print()
            console.print(f"[green]Successfully authenticated with {provider_id}![/green]")
        except TimeoutError:
            progress.update(task, description="[red]Timed out")
            console.print()
            console.print("[red]Authorization timed out. Please try again.[/red]")
            raise typer.Exit(1)
```

### Session Key Input

```python
@auth_app.command("claude")
async def auth_claude(
    session_key: str = typer.Argument(None, help="Session key (or enter interactively)"),
) -> None:
    """Authenticate with Claude using a session key."""
    if session_key is None:
        console.print("[bold]Claude Authentication[/bold]")
        console.print()
        console.print("Get your session key from browser cookies:")
        console.print("  1. Open [link=https://claude.ai]claude.ai[/link] in your browser")
        console.print("  2. Open Developer Tools → Application → Cookies")
        console.print("  3. Copy the value of [cyan]sessionKey[/cyan]")
        console.print()
        session_key = typer.prompt("Session key", hide_input=True)

    # Validate format
    if not session_key.startswith("sk-ant-sid01-"):
        console.print("[yellow]Warning: Key doesn't look like a Claude session key[/yellow]")
        if not typer.confirm("Save anyway?"):
            raise typer.Abort()

    # Save credential
    from vibeusage.auth.storage import save_credential
    save_credential("claude", "session_key", session_key)
    console.print("[green]Session key saved![/green]")
```

---

## Loading States

### Fetching Spinner

```python
from rich.status import Status

def fetch_with_spinner(provider_id: str) -> FetchResult:
    """Fetch usage with a spinner indicator."""
    with console.status(f"[bold]Fetching {provider_id} usage...[/bold]"):
        provider = get_provider(provider_id)
        return provider.fetch()
```

### Multiple Providers Progress

```python
from rich.progress import Progress, TaskID
import asyncio

async def fetch_all_providers() -> dict[str, FetchResult]:
    """Fetch all enabled providers concurrently with progress."""
    from vibeusage.providers import list_enabled_providers

    providers = list_enabled_providers()
    results = {}

    with Progress(console=console) as progress:
        task = progress.add_task("Fetching usage...", total=len(providers))

        async def fetch_one(provider_id: str) -> None:
            provider = get_provider(provider_id)
            results[provider_id] = await provider.fetch()
            progress.advance(task)

        await asyncio.gather(*[fetch_one(p) for p in providers])

    return results
```

---

## Error Display

### User-Friendly Error Messages

```python
from rich.panel import Panel

def show_error(error: str, provider_id: str | None = None) -> None:
    """Display a user-friendly error message."""
    title = f"{provider_id.title()} Error" if provider_id else "Error"

    console.print(Panel(
        error,
        title=title,
        border_style="red",
    ))

def show_auth_error(provider_id: str) -> None:
    """Display authentication error with fix instructions."""
    from vibeusage.providers import PROVIDER_ERROR_MESSAGES

    messages = PROVIDER_ERROR_MESSAGES.get(provider_id, {})
    message = messages.get("401", f"Authentication failed for {provider_id}")

    console.print(f"[red]Error:[/red] {message}")
    console.print()
    console.print(f"[dim]Run 'vibeusage auth {provider_id}' to re-authenticate.[/dim]")
```

### Stale Data Indicator

When showing cached data:

```python
def show_stale_warning(snapshot: UsageSnapshot) -> None:
    """Show warning for stale cached data."""
    age_minutes = (datetime.now(snapshot.fetched_at.tzinfo) - snapshot.fetched_at).total_seconds() / 60

    console.print(
        f"[yellow]Showing cached data from {int(age_minutes)} minutes ago[/yellow]"
    )
    console.print("[dim]Run with --refresh to fetch fresh data[/dim]")
    console.print()
```

---

## Command Reference

### `vibeusage` (default)

Show usage for all enabled providers.

```
Usage: vibeusage [OPTIONS]

Options:
  -j, --json      Output as JSON
  -v, --verbose   Show detailed output
  -q, --quiet     Minimal output
  --refresh       Force refresh (ignore cache)
  --help          Show help
```

### `vibeusage <provider>`

Show usage for a specific provider.

```
Usage: vibeusage claude [OPTIONS]

Options:
  -j, --json      Output as JSON
  -v, --verbose   Show source and account info
  --help          Show help
```

### `vibeusage auth`

Manage provider authentication.

```
Usage: vibeusage auth [OPTIONS] COMMAND [ARGS]...

Commands:
  claude    Authenticate with Claude
  codex     Authenticate with Codex
  copilot   Authenticate with GitHub Copilot
  cursor    Authenticate with Cursor
  gemini    Authenticate with Gemini
  --status  Show auth status for all providers
```

### `vibeusage status`

Show provider health status.

```
Usage: vibeusage status [OPTIONS]

Options:
  -j, --json   Output as JSON
  --help       Show help
```

### `vibeusage config`

Manage configuration.

```
Usage: vibeusage config [OPTIONS] COMMAND [ARGS]...

Commands:
  show    Show current configuration
  path    Show config/cache directory paths
  reset   Reset to default configuration
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Authentication error |
| 3 | Network error |
| 4 | Invalid input |

```python
class ExitCode:
    SUCCESS = 0
    ERROR = 1
    AUTH_ERROR = 2
    NETWORK_ERROR = 3
    INVALID_INPUT = 4
```

---

## Shell Completion

Typer provides automatic shell completion generation:

```bash
# Install completions
vibeusage --install-completion

# Generate completion script
vibeusage --show-completion
```

Provider names are completed automatically:

```python
from typing import Annotated
from vibeusage.providers import list_providers

def complete_provider(incomplete: str) -> list[str]:
    """Complete provider names."""
    providers = list_providers()
    return [p for p in providers if p.startswith(incomplete)]

@app.command()
def show(
    provider: Annotated[
        str,
        typer.Argument(autocompletion=complete_provider)
    ],
) -> None:
    ...
```

---

## Entry Point

```python
# vibeusage/__main__.py
from vibeusage.cli import app

def main() -> None:
    app()

if __name__ == "__main__":
    main()
```

```toml
# pyproject.toml
[project.scripts]
vibeusage = "vibeusage.__main__:main"
```

---

## Implementation Checklist

- [ ] `vibeusage/cli/__init__.py` - Main app and callback
- [ ] `vibeusage/cli/atyper.py` - ATyper async wrapper for Typer
- [ ] `vibeusage/cli/commands/usage.py` - Usage display commands
- [ ] `vibeusage/cli/commands/auth.py` - Authentication commands
- [ ] `vibeusage/cli/commands/status.py` - Status command
- [ ] `vibeusage/cli/commands/config.py` - Config commands
- [ ] `vibeusage/cli/display.py` - Rich renderables (UsageDisplay, ProviderPanel, etc.)
- [ ] `vibeusage/cli/formatters.py` - Bar rendering, time formatting
- [ ] `vibeusage/cli/json.py` - msgspec JSON output helpers
- [ ] `vibeusage/__main__.py` - Entry point

---

## Open Questions

1. **Color themes**: Should we support custom color themes for accessibility (e.g., colorblind-friendly)?

2. **Watch mode**: Should we support `vibeusage --watch` to continuously refresh and update display?

3. **Notifications**: Should we support desktop notifications when usage exceeds thresholds?

4. **Compact vs expanded**: Should there be a `--compact` mode for minimal output?

5. **Table vs panels**: Should multi-provider view use a table instead of panels for denser display?

## Implementation Notes

- Use `ATyper` instead of `Typer` to enable async command support
- Use `msgspec.json.encode()` for JSON output - no manual serialization needed
- Use `Console(force_terminal=True)` in tests for consistent output
- Test with `NO_COLOR=1` environment variable for colorless output
- Consider `Console(width=80)` for predictable line wrapping in tests
- The `--json` flag should suppress all non-JSON output including spinners
- Errors in JSON mode should still output valid JSON with an `error` field
- All commands should be `async def` to enable concurrent provider fetches
