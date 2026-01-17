"""Config management commands for vibeusage."""

import os
import subprocess
from pathlib import Path

import typer
from rich.console import Console
from rich.panel import Panel
from rich.syntax import Syntax

from vibeusage.cli.app import app, ExitCode
from vibeusage.cli.atyper import ATyper
from vibeusage.config.paths import cache_dir, config_dir, config_file, credentials_dir
from vibeusage.config.settings import Config, get_config

# Create config group
config_app = ATyper(help="Manage configuration settings.")


@config_app.command("show")
def config_show_command() -> None:
    """Display current settings."""
    console = Console()

    config = get_config()
    config_path = config_file()

    # Format as TOML
    import msgspec.toml

    toml_data = msgspec.toml.encode(config)
    console.print(Panel(Syntax(toml_data.decode(), "toml"), title=f"Config: {config_path}"))


@config_app.command("path")
def config_path_command(
    cache: bool = typer.Option(False, "--cache", "-c", help="Show cache directory"),
    credentials: bool = typer.Option(
        False, "--credentials", "-r", help="Show credentials directory"
    ),
) -> None:
    """Show directory paths used by vibeusage."""
    console = Console()

    if cache:
        console.print(str(cache_dir()))
    elif credentials:
        console.print(str(credentials_dir()))
    else:
        console.print(f"Config dir:    {config_dir()}")
        console.print(f"Config file:   {config_file()}")
        console.print(f"Cache dir:     {cache_dir()}")
        console.print(f"Credentials:   {credentials_dir()}")


@config_app.command("reset")
def config_reset_command(
    confirm: bool = typer.Option(
        False,
        "--confirm",
        "-y",
        help="Skip confirmation prompt",
    ),
) -> None:
    """Reset configuration to defaults."""
    console = Console()

    if not confirm:
        confirm = typer.confirm(
            "This will reset your configuration to defaults. Continue?",
            default=False,
        )

    if not confirm:
        console.print("Reset cancelled")
        raise typer.Exit()

    # Delete config file
    cfg_path = config_file()
    if cfg_path.exists():
        cfg_path.unlink()
        console.print(f"[green]✓[/green] Configuration reset to defaults")
        console.print(f"\nDeleted: {cfg_path}")
    else:
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
        import msgspec.toml

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
        raise typer.Exit(ExitCode.GENERAL_ERROR)
    except FileNotFoundError:
        console.print(f"[red]Editor not found: {editor}[/red]")
        console.print("Set EDITOR environment variable to your preferred editor")
        raise typer.Exit(ExitCode.CONFIG_ERROR)


# Register the config group with the main app
app.add_typer(config_app, name="config")
