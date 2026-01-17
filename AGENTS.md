## Commands
- `uv sync`: Sync the project's dependencies with the environment.
- `uv run vibeusage --help`: Show CLI help
- `uv run pytest`: Run all tests
- `uv run pytest tests/ -v`: Run tests with verbose output
- `uv run pytest tests/ --cov`: Run tests with coverage report

## Validation

Run these after implementing to get immediate feedback:

- Tests: `uv run pytest tests/ -v`
- Coverage: `uv run pytest tests/ --cov`
- Typecheck: `uvx ty check`
- Lint: `uvx ruff format`

## Operational Notes

Succinct learnings about how to RUN the project:

- CLI entry point: `uv run vibeusage [OPTIONS] COMMAND [ARGS]`
- Test suite: 308 passing tests covering models, errors, config, display, core, CLI, providers
- Coverage: 44% (target: 80%+)
- Use `uv run pytest -x` to stop at first failure
- Use `uv run pytest -k "test_name"` to run specific tests

### Codebase Patterns

- msgspec.Struct for all data models - frozen=True for immutability
- ATyper (cli/atyper.py) - async wrapper around Typer
- Provider registry via decorator pattern in providers/__init__.py
- Error classification through errors/classify.py for all exceptions
- Display utilities in display/rich.py and display/json.py
