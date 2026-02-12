# Command Reference

## Global Flags

These flags are available on all commands:

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--verbose` | `-v` | bool | `false` | Enable verbose output |
| `--dry-run` | `-n` | bool | `false` | Show what would happen without making changes |
| `--config` | | string | `""` | Path to config file (default: `~/.config/pm/config.yaml`) |

## Commands

| Command | Description |
|---------|-------------|
| [`pm project`](project.md) | Add, remove, list, show, and scan tracked projects |
| [`pm issue`](issue.md) | Add, list, show, update, close, and link issues |
| [`pm status`](status.md) | Cross-project status dashboard with health scores |
| [`pm agent`](agent.md) | Launch, list, and review Claude Code agent sessions |
| [`pm worktree`](worktree.md) | List and create git worktrees (alias: `wt`) |
| [`pm tag`](other.md#tag) | Create, list, and delete issue tags |
| [`pm standards`](other.md#standards) | Check project standardization |
| [`pm export`](other.md#export) | Export data as JSON, CSV, or Markdown |
| [`pm report`](other.md#report) | Generate activity reports |
| [`pm serve`](other.md#serve) | Start the web UI and REST API server |
| [`pm config`](other.md#config) | Show and manage configuration |
| [`pm mcp`](other.md#mcp) | MCP server for Claude Code (coming soon) |
| [`pm version`](other.md#version) | Print version information |

## ID Format

pm uses ULIDs (Universally Unique Lexicographically Sortable Identifiers) as primary keys. In command output, IDs are displayed as 12-character short IDs (the first 12 characters of the full 26-character ULID).

When referencing an issue by ID in commands (e.g., `pm issue show`), you can use:

- The full 26-character ULID
- A prefix that uniquely matches one issue (e.g., the 12-character short ID)

If a prefix matches multiple issues, pm will report an error and ask you to be more specific.
