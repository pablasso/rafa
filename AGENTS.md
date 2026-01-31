# AGENTS.md

This file provides guidance to agents when working with code in this repository.

## Project Context

Rafa is a task loop runner for AI coding agents. It executes tasks extracted from technical designs, running each in a fresh agent session and committing after success. Key principle: fresh context per task, fresh context per retry.

## Building

Verify code compiles after changes:

```bash
make build
```

## Go Formatting

Always run the formatter after modifying Go files:

```bash
make fmt
```

To check if files are properly formatted (without modifying):

```bash
make check-fmt
```

## Testing

Run tests after making changes:

```bash
make test
```

## Project Structure

- `internal/` - Core packages (cli, executor, planner, tui)
- `.rafa/plans/<id>-<name>/` - Plan storage per repo (plan.json, progress.log, output.log)

## Debugging

For crash diagnostics (stack traces, lock files, recovery), see CONTRIBUTING.md.
