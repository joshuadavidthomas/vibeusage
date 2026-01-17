"""Config management commands for vibeusage."""

from __future__ import annotations

import os
import subprocess

import msgspec.toml
import typer
from rich.console import Console
from rich.panel import Panel
from rich.syntax import Syntax

from vibeusage.cli.app import ExitCode
from vibeusage.cli.app import app
from vibeusage.cli.atyper import ATyper
from vibeusage.config.paths import cache_dir
from vibeusage.config.paths import config_dir
from vibeusage.config.paths import config_file
from vibeusage.config.paths import credentials_dir
from vibeusage.config.settings import Config
from vibeusage.config.settings import get_config

# Create config group
config_app = ATyper(help="Manage configuration settings.")


@config_app.callback(invoke_without_command=True)
def config_callback(
    ctx: typer.Context,
) -> None:
    """Config management commands.

    If no subcommand is provided, shows current configuration.
    """
    # Store context for subcommands
    pass


@config_app.command("show")
def config_show_command(
    ctx: typer.Context,
) -> None:
    """Display current settings."""
    console = Console()

    config = get_config()
    config_path = config_file()
    json_mode = ctx.meta.get("json", False)
    verbose = ctx.meta.get("verbose", False)
    quiet = ctx.meta.get("quiet", False)

    if json_mode:
        from vibeusage.display.json import output_json_pretty

        # Convert Config to dict for JSON output
        config_dict = {
            "fetch": {
                "timeout": config.fetch.timeout,
                "stale_threshold_minutes": config.fetch.stale_threshold_minutes,
                "max_concurrent": config.fetch.max_concurrent,
            },
            "enabled_providers": config.enabled_providers,
            "display": {
                "show_remaining": config.display.show_remaining,
                "pace_colors": config.display.pace_colors,
                "reset_format": config.display.reset_format,
            },
            "credentials": {
                "use_keyring": config.credentials.use_keyring,
                "reuse_provider_credentials": config.credentials.reuse_provider_credentials,
            },
            "path": str(config_path),
        }
        output_json_pretty(config_dict)
        return

    # Quiet mode: minimal output
    if quiet:
        console.print(str(config_path))
        return

    # Format as TOML
    toml_data = msgspec.toml.encode(config)
    console.print(
        Panel(Syntax(toml_data.decode(), "toml"), title=f"Config: {config_path}")
    )

    # Verbose: show additional info
    if verbose:
        console.print(f"\n[dim]Config file location: {config_path}[/dim]")
        if config_path.exists():
            console.print(f"[dim]File size: {config_path.stat().st_size} bytes[/dim]")
        else:
            console.print(
                "[dim]Using default configuration (file not created yet)[/dim]"
            )


@config_app.command("path")
def config_path_command(
    ctx: typer.Context,
    cache: bool = typer.Option(False, "--cache", "-c", help="Show cache directory"),
    credentials: bool = typer.Option(
        False, "--credentials", "-r", help="Show credentials directory"
    ),
) -> None:
    """Show directory paths used by vibeusage."""
    console = Console()
    json_mode = ctx.meta.get("json", False)
    verbose = ctx.meta.get("verbose", False)
    quiet = ctx.meta.get("quiet", False)

    if json_mode:
        from vibeusage.display.json import output_json_pretty

        paths = {
            "config_dir": str(config_dir()),
            "config_file": str(config_file()),
            "cache_dir": str(cache_dir()),
            "credentials_dir": str(credentials_dir()),
        }
        if cache:
            output_json_pretty({"cache_dir": str(cache_dir())})
        elif credentials:
            output_json_pretty({"credentials_dir": str(credentials_dir())})
        else:
            output_json_pretty(paths)
        return

    # Quiet mode: just the path, no labels
    if quiet:
        if cache:
            console.print(str(cache_dir()))
        elif credentials:
            console.print(str(credentials_dir()))
        else:
            console.print(str(config_dir()))
        return

    # Verbose mode: show existence info
    if cache:
        console.print(str(cache_dir()))
        if verbose:
            console.print(f"[dim]Exists: {cache_dir().exists()}[/dim]")
    elif credentials:
        console.print(str(credentials_dir()))
        if verbose:
            console.print(f"[dim]Exists: {credentials_dir().exists()}[/dim]")
    else:
        console.print(f"Config dir:    {config_dir()}")
        console.print(f"Config file:   {config_file()}")
        console.print(f"Cache dir:     {cache_dir()}")
        console.print(f"Credentials:   {credentials_dir()}")
        if verbose:
            console.print("\n[dim]Directory status:[/dim]")
            console.print(f"  Config dir exists: {config_dir().exists()}")
            console.print(f"  Cache dir exists: {cache_dir().exists()}")
            console.print(f"  Credentials dir exists: {credentials_dir().exists()}")


@config_app.command("reset")
def config_reset_command(
    ctx: typer.Context,
    confirm: bool = typer.Option(
        False,
        "--confirm",
        "-y",
        help="Skip confirmation prompt",
    ),
) -> None:
    """Reset configuration to defaults."""
    console = Console()
    json_mode = ctx.meta.get("json", False)

    result = {"success": False, "reset": False}

    if not confirm:
        # In JSON mode, auto-confirm to avoid hanging
        if json_mode:
            confirm = True
        else:
            confirm = typer.confirm(
                "This will reset your configuration to defaults. Continue?",
                default=False,
            )

    if not confirm:
        result["message"] = "Reset cancelled"
        if json_mode:
            from vibeusage.display.json import output_json_pretty

            output_json_pretty(result)
        console.print("Reset cancelled")
        raise typer.Exit()

    # Delete config file
    cfg_path = config_file()
    if cfg_path.exists():
        cfg_path.unlink()
        result["success"] = True
        result["reset"] = True
        result["message"] = "Configuration reset to defaults"
        result["deleted"] = str(cfg_path)

        if json_mode:
            from vibeusage.display.json import output_json_pretty

            output_json_pretty(result)
            return

        console.print("[green]✓[/green] Configuration reset to defaults")
        console.print(f"\nDeleted: {cfg_path}")
    else:
        result["message"] = "No custom configuration to reset"
        if json_mode:
            from vibeusage.display.json import output_json_pretty

            output_json_pretty(result)
            return
        console.print("[yellow]No custom configuration to reset[/yellow]")


@config_app.command("edit")
def config_edit_command() -> None:
    """Open configuration in editor."""
    console = Console()

    cfg_path = config_file()
    cfg_dir = config_dir()

    # Ensure config directory exists
    cfg_dir.mkdir(parents=True, exist_ok=True)

    # Create default config if it doesn't exist
    if not cfg_path.exists():
        default_config = Config()
        toml_data = msgspec.toml.encode(default_config)
        cfg_path.write_bytes(toml_data)
        console.print(f"[dim]Created default config: {cfg_path}[/dim]")

    # Get editor
    editor = os.environ.get("EDITOR", "vi")

    # Open editor
    try:
        subprocess.run([editor, str(cfg_path)], check=True)
        console.print(f"[green]✓[/green] Configuration file: {cfg_path}")
    except subprocess.CalledProcessError as e:
        console.print(f"[red]Editor exited with error: {e}[/red]")
        raise typer.Exit(ExitCode.GENERAL_ERROR) from e
    except FileNotFoundError:
        console.print(f"[red]Editor not found: {editor}[/red]")
        console.print("Set EDITOR environment variable to your preferred editor")
        raise typer.Exit(ExitCode.CONFIG_ERROR) from None


# Register the config group with the main app
app.add_typer(config_app, name="config")
