# pm

**Program Manager** -- Track projects, issues, and AI agent sessions across multiple repos from the command line.

`pm` is a developer-first CLI for managing parallel development across many repositories. It combines lightweight project tracking, issue management, agent session recording, health scoring, and a web dashboard -- all backed by a local SQLite database.

## Features

| Feature | Description |
|---------|-------------|
| [Project Tracking](commands/project.md) | Add, scan, and organize repos by group |
| [Issue Management](commands/issue.md) | Track bugs, features, and chores with priorities and tags |
| [Status Dashboard](commands/status.md) | Cross-project overview with health scores |
| [Agent Sessions](commands/agent.md) | Launch Claude Code agents in worktrees tied to issues |
| [Worktrees](commands/worktree.md) | Manage git worktrees for isolated development |
| [Standards Checking](commands/other.md#standards) | Verify projects follow conventions |
| [Data Export](commands/other.md#export) | JSON, CSV, and Markdown export |
| [Web Dashboard](commands/other.md#serve) | Embedded React UI with REST API |
| [REST API](api.md) | Programmatic access to all data |

## Quick Links

| Topic | Description |
|-------|-------------|
| [Getting Started](getting-started.md) | Installation, setup, and first steps |
| [Configuration](configuration.md) | Config file, environment variables, and defaults |
| [Command Reference](commands/index.md) | All commands with flags and examples |
| [Workflows](workflows.md) | End-to-end usage patterns |
| [REST API](api.md) | API endpoint reference |
