"""Key management commands for vibeusage."""

import typer
from rich.console import Console
from rich.table import Table

from vibeusage.cli.app import app, ExitCode
from vibeusage.cli.atyper import ATyper
from vibeusage.config.credentials import (
    check_provider_credentials,
    credential_path,
    delete_credential,
    find_provider_credential,
    get_all_credential_status,
    write_credential,
)
from vibeusage.providers import list_provider_ids

# Create key group
key_app = ATyper(help="Manage credentials for providers.")


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
    display_all_credential_status(console, json_mode=json_mode, verbose=verbose, quiet=quiet)


@key_app.command("set")
def key_set_command(
    provider: str = typer.Argument(
        ...,
        help="Provider to set credential for",
    ),
    credential_type: str = typer.Option(
        "session",
        "--type",
        "-t",
        help="Credential type (session, oauth, apikey)",
    ),
) -> None:
    """Set a credential for a provider."""
    console = Console()

    if provider not in list_provider_ids():
        console.print(f"[red]Unknown provider:[/red] {provider}")
        console.print(f"Available providers: {', '.join(list_provider_ids())}")
        raise typer.Exit(ExitCode.CONFIG_ERROR)

    # Prompt for credential value
    credential_value = typer.prompt(f"Enter {provider} {credential_type} credential", hide_input=True)

    if not credential_value:
        console.print("[red]Credential cannot be empty[/red]")
        raise typer.Exit(ExitCode.CONFIG_ERROR)

    # Write credential
    import json

    cred_path = credential_path(provider, credential_type)
    cred_data = {"credential": credential_value}
    content = json.dumps(cred_data).encode()

    try:
        write_credential(cred_path, content)
        console.print(f"[green]✓[/green] Credential saved for {provider}")
    except Exception as e:
        console.print(f"[red]Error saving credential:[/red] {e}")
        raise typer.Exit(ExitCode.GENERAL_ERROR)


@key_app.command("delete")
def key_delete_command(
    provider: str = typer.Argument(
        ...,
        help="Provider to delete credential for",
    ),
    credential_type: str = typer.Option(
        None,
        "--type",
        "-t",
        help="Credential type to delete (default: all)",
    ),
) -> None:
    """Delete a credential for a provider."""
    console = Console()

    if provider not in list_provider_ids():
        console.print(f"[red]Unknown provider:[/red] {provider}")
        console.print(f"Available providers: {', '.join(list_provider_ids())}")
        raise typer.Exit(ExitCode.CONFIG_ERROR)

    if credential_type:
        # Delete specific credential type
        cred_path = credential_path(provider, credential_type)
        if delete_credential(cred_path):
            console.print(f"[green]✓[/green] Deleted {credential_type} credential for {provider}")
        else:
            console.print(f"[yellow]No {credential_type} credential found for {provider}[/yellow]")
    else:
        # Delete all credentials for provider
        from pathlib import Path

        from vibeusage.config.paths import credentials_dir

        provider_dir = credentials_dir() / provider
        if provider_dir.exists():
            deleted = 0
            for cred_file in provider_dir.glob("*"):
                if delete_credential(cred_file):
                    deleted += 1

            if deleted > 0:
                console.print(f"[green]✓[/green] Deleted {deleted} credential(s) for {provider}")
            else:
                console.print(f"[yellow]No credentials found for {provider}[/yellow]")
        else:
            console.print(f"[yellow]No credentials found for {provider}[/yellow]")


def display_all_credential_status(console: Console, json_mode: bool = False, verbose: bool = False, quiet: bool = False) -> None:
    """Display credential status for all providers."""
    all_status = get_all_credential_status()

    if json_mode:
        from vibeusage.display.json import output_json_pretty

        data = {
            provider_id: {
                "configured": status_info["has_credentials"],
                "source": status_info.get("source") if status_info["has_credentials"] else None,
            }
            for provider_id, status_info in all_status.items()
        }
        output_json_pretty(data)
        return

    # Quiet mode: minimal output
    if quiet:
        for provider_id, status_info in all_status.items():
            status = "configured" if status_info["has_credentials"] else "not configured"
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
    console.print("  vibeusage key set <provider>")

    # Verbose: show credential paths
    if verbose:
        console.print("\n[bold]Credential Paths:[/bold]")
        for provider_id in all_status.keys():
            found, src, path = find_provider_credential(provider_id)
            if path:
                console.print(f"  {provider_id}: {path}")
            else:
                console.print(f"  {provider_id}: [dim]none[/dim]")


def display_provider_credential_status(
    console: Console,
    provider_id: str,
    has_creds: bool,
    source: str | None,
) -> None:
    """Display credential status for a single provider."""
    if has_creds:
        source_label = {
            "vibeusage": "vibeusage storage",
            "provider_cli": "provider CLI",
            "env": "environment variable",
        }.get(source or "", source or "unknown")

        console.print(f"[green]✓[/green] {provider_id} credentials configured ({source_label})")

        # Show details
        found, src, path = find_provider_credential(provider_id)
        if path:
            console.print(f"  Location: {path}")
    else:
        console.print(f"[yellow]✗[/yellow] {provider_id} not configured")
        console.print("\nSet credentials with:")
        console.print(f"  vibeusage key set {provider_id}")


# Register the key group with the main app
app.add_typer(key_app, name="key")
