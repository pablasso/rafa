# Rafa

A task loop runner for AI coding agents. Implements Geoffrey Huntley's [Ralph Wiggum](https://ghuntley.com/ralph/) technique and Anthropic's [recommendations for long-running agents](https://www.anthropic.com/engineering/effective-harnesses-for-long-running-agents) with structure and monitoring.

## What it does

Rafa takes a technical design, converts it into a sequence of tasks with clear acceptance criteria, and runs AI agents in a loop until each task succeeds. One agent, one task, one loop at a time.

- **Convert** markdown designs into executable JSON plans
- **Run** tasks sequentially with automatic retry on failure
- **Monitor** progress via TUI without babysitting
- **Resume** from where you left off

## Philosophy

Rafa specializes in running and monitoring the loop. It doesn't try to orchestrate everything—you're responsible for creating good designs with strong acceptance criteria. Rafa handles the execution.

Fresh context on every retry. Agents commit after each completed task. Progress is tracked, output is logged, and you can walk away.

## Status

Work in progress.
