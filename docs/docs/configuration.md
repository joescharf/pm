# Configuration

## Config File

The configuration file lives at `~/.config/pm/config.yaml`. Create it with:

```bash
pm config init
```

### Example config file

```yaml
# pm configuration
# See: pm config show (for effective values and sources)

# State/data directory (default: ~/.config/pm)
# state_dir: /home/user/.config/pm

# SQLite database path (default: ~/.config/pm/pm.db)
# db_path: /home/user/.config/pm/pm.db

# GitHub
github:
  # Default GitHub organization for project lookups
  default_org: "my-org"

# Agent settings
agent:
  # Claude model to use (default: "opus")
  model: "opus"

  # Auto-launch Claude agent when creating worktrees (default: false)
  auto_launch: false
```

## Config Keys

| Key | Default | Env Var | Description |
|-----|---------|---------|-------------|
| `state_dir` | `~/.config/pm` | `PM_STATE_DIR` | Directory for pm state and data files |
| `db_path` | `~/.config/pm/pm.db` | `PM_DB_PATH` | Path to the SQLite database file |
| `github.default_org` | `""` | `PM_GITHUB_DEFAULT_ORG` | Default GitHub organization for project lookups |
| `agent.model` | `"opus"` | `PM_AGENT_MODEL` | Claude model to use for agent sessions |
| `agent.auto_launch` | `false` | `PM_AGENT_AUTO_LAUNCH` | Auto-launch Claude agent when creating worktrees |

## Precedence

Configuration values are resolved in the following order (highest priority first):

1. **Command-line flags** -- e.g., `--port 3000`
2. **Environment variables** -- prefixed with `PM_`, e.g., `PM_DB_PATH`
3. **Config file** -- `~/.config/pm/config.yaml`
4. **Defaults** -- built-in default values

## Managing Configuration

### View effective config

```bash
pm config show
```

Shows all configuration keys with their current values and sources:

```
Config file: /home/user/.config/pm/config.yaml

  state_dir              /home/user/.config/pm  (default)
  db_path                /home/user/.config/pm/pm.db  (default)
  github.default_org     my-org                 (file)
  agent.model            opus                   (default)
  agent.auto_launch      false                  (default)
```

Sources:

- `(default)` -- using the built-in default value
- `(file)` -- set in the config file
- `(env: PM_XXX)` -- set via environment variable

### Edit config

```bash
pm config edit
```

Opens the config file in `$EDITOR` (or `$VISUAL`). The config file must exist first -- run `pm config init` if needed.

### Reinitialize config

```bash
pm config init --force
```

Overwrites the existing config file with a fresh template populated from current effective values.

## Database

pm uses SQLite (via `modernc.org/sqlite`, a pure-Go driver) with WAL mode enabled for concurrent reads.

- **Default location:** `~/.config/pm/pm.db`
- **ID format:** ULIDs (Universally Unique Lexicographically Sortable Identifiers)
- **Migrations:** Applied automatically on first use

The database schema includes tables for:

- `projects` -- tracked repositories
- `issues` -- bugs, features, and chores linked to projects
- `tags` and `issue_tags` -- tag-based organization for issues
- `agent_sessions` -- Claude Code agent session records

### Custom database location

Set via config file, environment variable, or flag:

```bash
# Environment variable
export PM_DB_PATH=/path/to/custom/pm.db
pm status

# Config file
# db_path: /path/to/custom/pm.db
```
