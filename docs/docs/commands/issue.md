# pm issue

Manage project issues and features.

```
pm issue add [project]          Add a new issue
pm issue list [project]         List issues (alias: ls)
pm issue show <issue-id>        Show issue details
pm issue update <issue-id>      Update an issue
pm issue close <issue-id>       Close an issue
pm issue link <issue-id>        Link to a GitHub issue
pm issue import <file>          Import issues from markdown
```

## Data Model

### Statuses

| Status | Description |
|--------|-------------|
| `open` | New issue, not yet started |
| `in_progress` | Actively being worked on |
| `done` | Work completed |
| `closed` | Resolved and closed |

### Priorities

| Priority | Description |
|----------|-------------|
| `low` | Low priority |
| `medium` | Default priority |
| `high` | High priority |

### Types

| Type | Description |
|------|-------------|
| `feature` | New functionality (default) |
| `bug` | Bug fix |
| `chore` | Maintenance or infrastructure work |

## issue add

Add a new issue to a project.

```bash
pm issue add [project] [flags]
```

Without `<project>`, auto-detects the project from the current working directory.

**Flags:**

| Flag | Type | Default | Required | Description |
|------|------|---------|----------|-------------|
| `--title` | string | | Yes | Issue title |
| `--desc` | string | `""` | No | Issue description |
| `--priority` | string | `"medium"` | No | Priority: `low`, `medium`, `high` |
| `--type` | string | `"feature"` | No | Type: `feature`, `bug`, `chore` |
| `--tag` | string | `""` | No | Tag to apply (created if it doesn't exist) |

**Examples:**

```bash
# Add to a specific project
pm issue add my-api --title "Add user authentication" --priority high

# Auto-detect project from cwd
cd ~/code/my-api
pm issue add --title "Fix login bug" --type bug

# Add with a tag
pm issue add --title "Improve logging" --tag observability
```

## issue list

List issues, optionally filtered by project or criteria.

```bash
pm issue list [project] [flags]
```

Aliases: `ls`

Without `<project>` and without `--all`, tries to detect the project from the current directory. Falls back to showing all issues.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--status` | string | `""` | Filter by status: `open`, `in_progress`, `done`, `closed` |
| `--priority` | string | `""` | Filter by priority: `low`, `medium`, `high` |
| `--tag` | string | `""` | Filter by tag name |
| `--all` | bool | `false` | Show all issues across all projects |

**Output columns:** ID (short), Project, Title, Status, Priority, Type, GH#

**Examples:**

```bash
# List all issues
pm issue list --all

# List issues for a specific project
pm issue list my-api

# Filter by status
pm issue ls --status open

# Filter by tag
pm issue list --tag backend
```

## issue show

Show detailed information about an issue.

```bash
pm issue show <issue-id>
```

The `<issue-id>` can be a full ULID or a unique prefix (e.g., the 12-character short ID).

Displays: short ID, title, project, status (colored), priority, type, description, GitHub issue number, tags, created date, closed date, and full ULID.

**Example:**

```bash
pm issue show 01J5ABCD1234
```

## issue update

Update fields on an existing issue.

```bash
pm issue update <issue-id> [flags]
```

At least one flag must be specified.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--status` | string | `""` | New status |
| `--priority` | string | `""` | New priority |
| `--title` | string | `""` | New title |
| `--desc` | string | `""` | New description |

**Examples:**

```bash
# Move to in-progress
pm issue update 01J5ABCD1234 --status in_progress

# Change priority
pm issue update 01J5ABCD1234 --priority high
```

## issue close

Close an issue. Sets the status to `closed` and records the close timestamp.

```bash
pm issue close <issue-id>
```

**Example:**

```bash
pm issue close 01J5ABCD1234
```

## issue link

Link a pm issue to a GitHub issue number.

```bash
pm issue link <issue-id> --github <number>
```

**Flags:**

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--github` | int | Yes | GitHub issue number |

**Example:**

```bash
pm issue link 01J5ABCD1234 --github 42
```

After linking, the GitHub issue number appears in issue list and show output as `GH#42`.

## issue import

Bulk-import issues from a markdown file.

```bash
pm issue import <file> [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--project` | string | `""` | Assign all issues to this project (skips LLM) |
| `--dry-run` | bool | `false` | Preview extracted issues without creating them |

There are two import modes:

### With `--project` (simple parser)

Parses numbered (`1.`) and bulleted (`-`, `*`) list items from the markdown file. Issues can optionally be grouped under `## Project <name>` headings.

Type and priority are **automatically classified from the title** using keyword heuristics:

| Classification | Keywords |
|----------------|----------|
| **Bug** | fix, bug, broken, crash, error, regression, fail, fault, defect, "issue with", "not working" |
| **Chore** | refactor, cleanup, "clean up", "update dep", migrate, upgrade, rename, reorganize, chore, lint |
| **Feature** | (default -- no keywords matched) |
| **High priority** | critical, urgent, blocker, crash, security, "data loss", "production down", p0, p1 |
| **Low priority** | minor, "nice to have", cosmetic, trivial, "low priority", cleanup, "clean up" |
| **Medium priority** | (default -- no keywords matched) |

Bug keywords are checked before chore keywords, so "Fix the migration script" is classified as a bug.

### Without `--project` (LLM extraction)

Uses Claude (requires `ANTHROPIC_API_KEY` or `anthropic.api_key` in config) to extract structured issues with project assignment, type, and priority inferred by the model.

**Examples:**

```bash
# Preview with simple parser
pm issue import backlog.md --project my-api --dry-run

# Import with LLM extraction
pm issue import backlog.md

# Import from stdin
echo "1. Fix login crash\n2. Add dark mode" | pm issue import --project myapp /dev/stdin
```
