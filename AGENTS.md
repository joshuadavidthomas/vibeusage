## Commands
- `uv sync`: Sync the project's dependencies with the environment.
- `uv run vibeusage --help`: Show CLI help

## Validation

Run these after implementing to get immediate feedback:

- Tests: No tests exist yet (Phase 6)
- Typecheck: `uv run python -c "from vibeusage.errors.classify import classify_exception"` (or similar import checks)
- Lint: Not configured

## Operational Notes

Succinct learnings about how to RUN the project:

- CLI entry point: `uv run vibeusage [OPTIONS] COMMAND [ARGS]`
- No test suite exists yet - validate by importing modules and running the CLI
- All imports should work: `from vibeusage.display import ...`, `from vibeusage.errors.classify import ...`

### Codebase Patterns

- msgspec.Struct for all data models - frozen=True for immutability
- ATyper (cli/atyper.py) - async wrapper around Typer
- Provider registry via decorator pattern in providers/__init__.py
- Error classification through errors/classify.py for all exceptions
- Display utilities in display/rich.py and display/json.py
