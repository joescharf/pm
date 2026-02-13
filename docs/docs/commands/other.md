# Other Commands

## tag

Manage issue tags for organizing and filtering issues.

```
pm tag list              List all tags (alias: ls)
pm tag create <name>     Create a new tag
pm tag delete <name>     Delete a tag (alias: rm)
```

Tags are created automatically when using `pm issue add --tag <name>` if the tag doesn't exist yet. Use these commands for explicit tag management.

**Examples:**

```bash
# List all tags
pm tag list

# Create a tag
pm tag create backend

# Delete a tag
pm tag delete obsolete-tag
```

---

## standards

Check if a project follows standard conventions.

```bash
pm standards [project]
```

Without `<project>`, checks all tracked projects.

Runs 9 checks against the project directory:

| # | Check | What It Looks For |
|---|-------|-------------------|
| 1 | GoReleaser config | `.goreleaser.yml` in project root |
| 2 | Makefile | `Makefile` in project root |
| 3 | CLAUDE.md | `CLAUDE.md` in project root |
| 4 | Mockery config | `.mockery.yml` in project root |
| 5 | LICENSE file | `LICENSE` in project root |
| 6 | README | `README.md` in project root |
| 7 | Go module | `go.mod` in project root |
| 8 | internal/ directory | `internal/` directory exists |
| 9 | Tests | Any `_test.go` files anywhere in the project tree |

Each check shows a green checkmark or red X with the check name and detail. A summary score is printed as `passed/total`.

**Examples:**

```bash
# Check a specific project
pm standards my-api

# Check all projects
pm standards
```

---

## export

Export data as JSON, CSV, or Markdown.

```bash
pm export [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--format` | string | `"json"` | Output format: `json`, `csv`, `markdown` |
| `--type` | string | `"projects"` | Data type: `projects`, `issues`, `sessions` |

**Examples:**

```bash
# Export projects as JSON (default)
pm export

# Export issues as CSV
pm export --type issues --format csv

# Export sessions as Markdown
pm export --type sessions --format markdown

# Pipe to a file
pm export --type issues --format json > issues.json
```

---

## report

Generate summary reports of project activity.

### report weekly

Generate a weekly activity summary in Markdown format.

```bash
pm report weekly
```

For each tracked project, outputs:

- Project name as a heading
- Issue counts: open, in-progress, and closed
- Active agent session branches

**Example:**

```bash
pm report weekly

# Save to a file
pm report weekly > weekly-review.md
```

---

## serve

Start the web UI and REST API server.

```bash
pm serve [flags]
```

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--port` | `-p` | int | `8080` | Port to listen on |

Starts an HTTP server that hosts:

- **Web UI** at `http://localhost:<port>/` -- an embedded React dashboard
- **REST API** at `http://localhost:<port>/api/v1/` -- see [REST API reference](../api.md)

On startup, all projects are automatically refreshed in the background to ensure the dashboard shows up-to-date metadata (language, GitHub description, Pages status, branch counts, etc.). The dashboard also includes a **Refresh All** button for on-demand refreshing.

**Examples:**

```bash
# Default port 8080
pm serve

# Custom port
pm serve --port 3000
```

---

## config

Show or manage pm configuration. Running bare `pm config` is equivalent to `pm config show`.

```
pm config init     Create config file with commented defaults
pm config show     Show effective configuration with sources
pm config edit     Open config file in $EDITOR
```

### config init

Create the config file at `~/.config/pm/config.yaml` with commented defaults.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--force` | bool | `false` | Overwrite existing config file |

```bash
pm config init
pm config init --force  # overwrite existing
```

### config show

Show all configuration keys with their current values and where each value comes from (default, file, or environment variable).

```bash
pm config show
```

### config edit

Open the config file in your `$EDITOR` (or `$VISUAL`). The config file must exist first.

```bash
pm config edit
```

See [Configuration](../configuration.md) for full details.

---

## mcp

Start an MCP (Model Context Protocol) server on stdio for Claude Code integration.

```bash
pm mcp
```

!!! note "Coming Soon"
    The MCP server is not yet implemented. It will provide tools for Claude Code to query pm natively: `pm_list_projects`, `pm_project_status`, `pm_list_issues`, `pm_create_issue`, `pm_update_issue`, `pm_launch_agent`, `pm_health_score`.

    Configure in Claude Code with:

    ```json
    {
      "mcpServers": {
        "pm": { "command": "pm", "args": ["mcp"] }
      }
    }
    ```

---

## version

Print version information.

```bash
pm version
```

Output format:

```
pm version <version> (commit: <commit>, built: <date>)
```

Version, commit hash, and build date are set at build time via linker flags.
