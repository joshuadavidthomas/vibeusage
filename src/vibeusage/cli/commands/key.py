"""Key management commands for vibeusage."""

from __future__ import annotations

import json
from pathlib import Path
from typing import TYPE_CHECKING

import typer
from rich.console import Console
from rich.table import Table

from vibeusage.cli.app import ExitCode
from vibeusage.cli.atyper import ATyper
from vibeusage.config.credentials import credential_path
from vibeusage.config.credentials import delete_credential
from vibeusage.config.credentials import find_provider_credential
from vibeusage.config.credentials import get_all_credential_status
from vibeusage.config.credentials import write_credential
from vibeusage.config.paths import credentials_dir
from vibeusage.providers import list_provider_ids

if TYPE_CHECKING:
    from collections.abc import Callable

    from click import Typer as TyperType

# Create key group
key_app = ATyper(help="Manage credentials for providers.")


def create_key_command(
    provider_id: str,
    credential_type: str = "session",
    credential_prefix: str | None = None,
) -> "TyperType":
    """Factory for provider-specific key commands.

    Args:
        provider_id: The provider identifier (e.g., "claude", "copilot")
        credential_type: The type of credential (session, oauth, apikey)
        credential_prefix: Optional prefix to validate (e.g., "sk-ant-sid01-")

    Returns:
        A Typer app with set/delete commands for the provider
    """

    provider_app = ATyper(help=f"Manage {provider_id.title()} credentials")

    @provider_app.callback(invoke_without_command=True)
    def show(
        ctx: typer.Context,
        json_mode: bool = typer.Option(False, "--json", "-j", help="Enable JSON output mode"),
        quiet: bool = typer.Option(False, "--quiet", "-q", help="Enable quiet mode"),
    ) -> None:
        """Show credential status for this provider."""
        if ctx.invoked_subcommand is not None:
            # Store options for subcommands to access
            ctx.meta["json"] = json_mode
            ctx.meta["quiet"] = quiet
            return

        console = Console()

        # Check context chain for global options (parent -> grandparent)
        # Local options (from this callback's parameters) take precedence
        current_ctx = ctx.parent
        while current_ctx:
            if not json_mode:
                json_mode = current_ctx.meta.get("json", False)
            if not quiet:
                quiet = current_ctx.meta.get("quiet", False)
            current_ctx = current_ctx.parent if current_ctx.parent else None

        # Check for any credential
        found, source, path = find_provider_credential(provider_id)

        if json_mode:
            from vibeusage.display.json import output_json_pretty

            output_json_pretty(
                {
                    "provider": provider_id,
                    "configured": found,
                    "source": source,
                    "path": str(path) if path else None,
                }
            )
            return

        if quiet:
            console.print(f"{provider_id}: {'configured' if found else 'not configured'}")
            return

        if found:
            source_label = {
                "vibeusage": "vibeusage storage",
                "provider_cli": "provider CLI",
                "env": "environment variable",
            }.get(source or "", source or "unknown")

            console.print(f"[green]✓[/green] {provider_id.title()} credentials configured ({source_label})")
            if path:
                console.print(f"  Location: {path}")
        else:
            console.print(f"[yellow]✗[/yellow] {provider_id.title()} credentials not configured")
            console.print(f"\n[dim]Run 'vibeusage key {provider_id} set' to configure[/dim]")

    @provider_app.command("set")
    def set_key(
        value: str = typer.Argument(
            None, help="Credential value (or enter interactively)"
        ),
        type_override: str = typer.Option(
            None,
            "--type",
            "-t",
            help=f"Credential type (default: {credential_type})",
        ),
    ) -> None:
        """Set a credential for this provider."""
        console = Console()
        cred_type = type_override or credential_type

        if value is None:
            console.print(f"[bold]Set {provider_id.title()} Credential[/bold]")
            console.print()
            value = typer.prompt(f"Enter {provider_id} {cred_type} credential", hide_input=True)

        if not value:
            console.print("[red]Credential cannot be empty[/red]")
            raise typer.Exit(ExitCode.CONFIG_ERROR)

        # Validate format if prefix specified
        if credential_prefix and not value.startswith(credential_prefix):
            console.print(f"[yellow]Warning: doesn't start with '{credential_prefix}'[/yellow]")
            if not typer.confirm("Save anyway?"):
                raise typer.Abort()

        # Save credential
        cred_path = credential_path(provider_id, cred_type)
        cred_data = {"credential": value}
        content = json.dumps(cred_data).encode()

        try:
            write_credential(cred_path, content)

            # Clear cached data for this provider
            from vibeusage.config.cache import clear_provider_cache as _clear

            _clear(provider_id)

            console.print(f"[green]✓[/green] Credential saved for {provider_id}")
        except Exception as e:
            console.print(f"[red]Error saving credential:[/red] {e}")
            raise typer.Exit(ExitCode.GENERAL_ERROR) from e

    @provider_app.command("delete")
    def delete_key(
        force: bool = typer.Option(False, "--force", "-f", help="Skip confirmation"),
        type_override: str = typer.Option(
            None,
            "--type",
            "-t",
            help=f"Credential type to delete (default: {credential_type})",
        ),
    ) -> None:
        """Delete a credential for this provider."""
        console = Console()
        cred_type = type_override or credential_type

        if not force:
            if not typer.confirm(f"Delete {provider_id.title()} {cred_type} credential?"):
                raise typer.Abort()

        if type_override or credential_type:
            # Delete specific credential type
            cred_path = credential_path(provider_id, cred_type)
            if delete_credential(cred_path):
                console.print(f"[green]✓[/green] Deleted {cred_type} credential for {provider_id}")
            else:
                console.print(f"[yellow]No {cred_type} credential found for {provider_id}[/yellow]")
        else:
            # Delete all credentials for provider
            provider_dir = credentials_dir() / provider_id
            if provider_dir.exists():
                deleted = 0
                for cred_file in provider_dir.glob("*"):
                    if delete_credential(cred_file):
                        deleted += 1

                if deleted > 0:
                    console.print(f"[green]✓[/green] Deleted {deleted} credential(s) for {provider_id}")
                else:
                    console.print(f"[yellow]No credentials found for {provider_id}[/yellow]")
            else:
                console.print(f"[yellow]No credentials found for {provider_id}[/yellow]")

    return provider_app


@key_app.callback(invoke_without_command=True)
def key_callback(
    ctx: typer.Context,
) -> None:
    """Show credential status for all providers.

    If no subcommand is provided, shows credential status for all providers.
    Use `vibeusage auth <provider>` to see detailed status for a specific provider.
    """
    console = Console()

    # If a subcommand was invoked, don't run the default behavior
    if ctx.invoked_subcommand is not None:
        return

    # Show all providers
    json_mode = ctx.meta.get("json", False)
    verbose = ctx.meta.get("verbose", False)
    quiet = ctx.meta.get("quiet", False)
    display_all_credential_status(
        console, json_mode=json_mode, verbose=verbose, quiet=quiet
    )


def display_all_credential_status(
    console: Console,
    json_mode: bool = False,
    verbose: bool = False,
    quiet: bool = False,
) -> None:
    """Display credential status for all providers."""
    all_status = get_all_credential_status()

    if json_mode:
        from vibeusage.display.json import output_json_pretty

        data = {
            provider_id: {
                "configured": status_info["has_credentials"],
                "source": status_info.get("source")
                if status_info["has_credentials"]
                else None,
            }
            for provider_id, status_info in all_status.items()
        }
        output_json_pretty(data)
        return

    # Quiet mode: minimal output
    if quiet:
        for provider_id, status_info in all_status.items():
            status = (
                "configured" if status_info["has_credentials"] else "not configured"
            )
            console.print(f"{provider_id}: {status}")
        return

    table = Table(title="Credential Status", show_header=True, header_style="bold")
    table.add_column("Provider", style="cyan")
    table.add_column("Status", style="bold")
    table.add_column("Source", style="dim")

    for provider_id, status_info in all_status.items():
        if status_info["has_credentials"]:
            status_text = "[green]✓ Configured[/green]"
            source_text = status_info.get("source", "unknown")
        else:
            status_text = "[yellow]✗ Not configured[/yellow]"
            source_text = "—"

        table.add_row(provider_id, status_text, source_text)

    console.print(table)
    console.print("\nSet credentials with:")
    console.print("  vibeusage key <provider> set")

    # Verbose: show credential paths
    if verbose:
        console.print("\n[bold]Credential Paths:[/bold]")
        for provider_id in all_status.keys():
            found, src, path = find_provider_credential(provider_id)
            if path:
                console.print(f"  {provider_id}: {path}")
            else:
                console.print(f"  {provider_id}: [dim]none[/dim]")


# Register provider-specific key commands
# This is done at import time to ensure all providers are available
def _register_provider_key_commands() -> None:
    """Register provider-specific key commands dynamically."""
    from vibeusage.providers import _PROVIDERS

    # Default credential types for each provider
    credential_types: dict[str, tuple[str, str | None]] = {
        "claude": ("session", None),
        "codex": ("oauth", None),
        "copilot": ("oauth", None),
        "cursor": ("session", None),
        "gemini": ("oauth", None),
    }

    for provider_id in list_provider_ids():
        if provider_id in credential_types:
            cred_type, cred_prefix = credential_types[provider_id]
        else:
            cred_type, cred_prefix = "session", None

        provider_cmd = create_key_command(provider_id, cred_type, cred_prefix)
        key_app.add_typer(provider_cmd, name=provider_id)


# Register commands on module import
_register_provider_key_commands()
