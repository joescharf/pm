# Getting Started

## Prerequisites

- **Go 1.22+** -- required for installation and building
- **Git** -- required for project tracking and worktrees
- **gh CLI** (optional) -- for GitHub release information in `pm project show`
- **wt CLI** (optional) -- for advanced worktree management via `pm worktree`

## Installation

### Via `go install`

```bash
go install github.com/joescharf/pm@latest
```

### Build from source

```bash
git clone https://github.com/joescharf/pm.git
cd pm
make build
# Binary is at ./bin/pm
```

To install to your `$GOPATH/bin`:

```bash
make install
```

## Initial Setup

### Create a configuration file

```bash
pm config init
```

This creates `~/.config/pm/config.yaml` with commented defaults. Review and edit it:

```bash
pm config edit
```

### Verify your configuration

```bash
pm config show
```

This displays all effective values and where each comes from (default, file, or environment variable):

```
Config file: /home/user/.config/pm/config.yaml

  state_dir              /home/user/.config/pm  (default)
  db_path                /home/user/.config/pm/pm.db  (default)
  github.default_org                            (default)
  agent.model            opus                   (default)
  agent.auto_launch      false                  (default)
```

## Add Your First Project

Add the current directory as a tracked project:

```bash
cd ~/code/my-project
pm project add .
```

Or add a specific path with a group:

```bash
pm project add ~/code/my-api --group backend
```

### Auto-discover projects

Scan a parent directory to find and add all git repos at once:

```bash
pm project scan ~/code
```

This finds every top-level git repository under `~/code` and adds it to tracking.

### List tracked projects

```bash
pm project list
```

## Create an Issue

```bash
pm issue add my-project --title "Add user authentication" --priority high --type feature
```

If you're inside a tracked project directory, you can omit the project name:

```bash
cd ~/code/my-project
pm issue add --title "Fix login bug" --type bug
```

## Check Status

View a dashboard across all projects:

```bash
pm status
```

This shows each project's current branch, dirty/clean status, open issue counts, health score, and last activity time.

Filter to stale projects (no activity in 7+ days):

```bash
pm status --stale
```

## Launch the Web Dashboard

```bash
pm serve
```

This starts the embedded web UI at `http://localhost:8080` and the REST API at `http://localhost:8080/api/v1/`.

Use `--port` to change the port:

```bash
pm serve --port 3000
```

## Next Steps

- [Configuration](configuration.md) -- Customize settings, environment variables, and database location
- [Command Reference](commands/index.md) -- Full details on every command and flag
- [Workflows](workflows.md) -- End-to-end usage patterns for common tasks
- [REST API](api.md) -- Programmatic access via HTTP endpoints
