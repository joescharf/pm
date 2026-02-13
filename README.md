# pm

**Program Manager** -- Track projects, issues, and AI agent sessions across multiple repos from the command line.

`pm` is a developer-first CLI for managing parallel development across many repositories. It combines lightweight project tracking, issue management, agent session recording, health scoring, and a web dashboard -- all backed by a local SQLite database.

## Why pm?

When you're developing across many repos with AI coding agents, you need a way to:

- Keep track of which projects are active and their current state
- Manage issues and features without leaving the terminal
- Launch AI agent sessions tied to specific issues and worktrees
- Get a quick health check across all your projects at a glance
- Export data and generate reports for weekly reviews

## Features

- **Project tracking** -- Add, scan, and organize repos by group
- **Issue management** -- Track bugs, features, and chores with priorities and tags
- **Agent sessions** -- Launch Claude Code agents in worktrees, tied to issues
- **Health scoring** -- 0-100 composite score across 5 dimensions (git cleanliness, activity, issues, releases, branches)
- **Standards checking** -- Verify projects follow conventions (Makefile, CLAUDE.md, tests, etc.)
- **Web dashboard** -- Embedded React UI served alongside a REST API
- **Issue import** -- Bulk-import issues from markdown files with LLM-powered extraction
- **Data export** -- JSON, CSV, and Markdown export for projects, issues, and sessions
- **MCP server** -- Model Context Protocol server for Claude Code integration
- **Auto-detect project** -- Running `pm` in a project directory shows its status automatically

## Quick Start

```bash
# Install
go install github.com/joescharf/pm@latest

# Initialize configuration
pm config init

# Add a project (current directory)
pm project add .

# Or scan a parent directory to discover all git repos
pm project scan ~/code

# Create an issue
pm issue add --title "Add user authentication"

# Import a backlog of issues from markdown
pm issue import backlog.md --dry-run

# Check status across all projects
pm status

# Launch the web dashboard
pm serve
```

## Installation

### From source (requires Go 1.22+)

```bash
go install github.com/joescharf/pm@latest
```

### Build locally

```bash
git clone https://github.com/joescharf/pm.git
cd pm
make build
# Binary is at ./bin/pm
```

## Configuration

Configuration lives at `~/.config/pm/config.yaml`. Initialize it with:

```bash
pm config init
```

Key settings:

| Key | Default | Env Var | Description |
|-----|---------|---------|-------------|
| `state_dir` | `~/.config/pm` | `PM_STATE_DIR` | State/data directory |
| `db_path` | `~/.config/pm/pm.db` | `PM_DB_PATH` | SQLite database path |
| `github.default_org` | `""` | `PM_GITHUB_DEFAULT_ORG` | Default GitHub org |
| `agent.model` | `"opus"` | `PM_AGENT_MODEL` | Claude model for agents |
| `agent.auto_launch` | `false` | `PM_AGENT_AUTO_LAUNCH` | Auto-launch agents on worktree create |
| `anthropic.api_key` | `""` | `ANTHROPIC_API_KEY` | API key for LLM features (issue import) |
| `anthropic.model` | `"claude-haiku-4-5-20251001"` | `PM_ANTHROPIC_MODEL` | Model for LLM extraction |

Precedence: flags > environment variables > config file > defaults.

See the [full configuration docs](docs/docs/configuration.md) for details.

## Commands

```
pm project add|remove|list|show|scan|refresh   Manage tracked projects
pm issue add|list|show|update|close|link|import   Manage issues and features
pm status [project]                    Cross-project status dashboard
pm agent launch|list|history           Manage AI agent sessions
pm worktree list|create                Manage git worktrees (alias: wt)
pm tag list|create|delete              Manage issue tags
pm standards [project]                 Check project standardization
pm export                              Export data (JSON/CSV/Markdown)
pm report weekly                       Generate weekly summary
pm serve                               Start web UI + REST API
pm config init|show|edit               Manage configuration
pm mcp                                 Start MCP stdio server
pm version                             Print version info
```

Global flags: `--verbose (-v)`, `--dry-run (-n)`, `--config <path>`

## Web Dashboard

`pm serve` starts an embedded web UI at `http://localhost:8080` with:

- **Dashboard** -- Overview of all projects with health scores, open issue counts, and quick links
- **Projects** -- Detailed project view with git metadata, issues, and health breakdown
- **Issues** -- All issues grouped by project with status/priority/tag filters
- **REST API** -- Full CRUD API at `/api/v1/` for programmatic access

The UI is a React/TypeScript SPA built with Vite, TanStack Query, and shadcn/ui, embedded directly into the Go binary.

## Documentation

Full documentation is available in the [docs site](docs/docs/index.md):

- [Getting Started](docs/docs/getting-started.md)
- [Configuration](docs/docs/configuration.md)
- [Command Reference](docs/docs/commands/index.md)
- [Workflows](docs/docs/workflows.md)
- [REST API](docs/docs/api.md)

Build and serve docs locally:

```bash
make docs-deps   # one-time setup
make docs-serve  # starts at http://localhost:8000
```

## License

See [LICENSE](LICENSE) for details.
