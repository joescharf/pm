# pm agent

Manage Claude Code agent sessions. Agent sessions represent AI coding work tied to specific issues and git worktrees.

```
pm agent launch <project>       Launch a Claude agent in a new worktree
pm agent list [project]         List active agent sessions (alias: ls)
pm agent history [project]      Show agent session history
```

## Concepts

An **agent session** records a Claude Code agent working on a specific task:

- Each session is tied to a **project** and optionally an **issue**
- The agent works in an isolated **git worktree** on a dedicated branch
- Sessions track status (`running`, `completed`, `failed`, `abandoned`), commit count, and duration

When launched with `--issue`, the agent session automatically:

1. Generates a branch name from the issue title (e.g., `feature/add-user-auth`)
2. Sets the issue status to `in_progress`
3. Creates a git worktree for isolated development

## agent launch

Launch a Claude agent in a new worktree for a project.

```bash
pm agent launch <project> [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--issue` | string | `""` | Issue ID to work on |
| `--branch` | string | `""` | Branch name (auto-generated from issue title if not specified) |

Either `--issue` or `--branch` must be provided.

**Branch name generation:** When `--issue` is specified without `--branch`, the branch name is derived from the issue title: lowercased, non-alphanumeric characters replaced with hyphens, collapsed, truncated to 50 characters, and prefixed with `feature/`.

**Examples:**

```bash
# Launch with an issue (auto-generates branch)
pm agent launch my-api --issue 01J5ABCD1234

# Launch with a specific branch
pm agent launch my-api --branch feature/custom-branch

# Preview what would happen
pm agent launch my-api --issue 01J5ABCD1234 --dry-run
```

## agent list

List currently active (running) agent sessions.

```bash
pm agent list [project]
```

Aliases: `ls`

Without `<project>`, shows sessions across all projects. With a project name, filters to that project.

**Output columns:** ID (short), Project, Branch, Started (relative time)

**Example:**

```bash
pm agent list
pm agent ls my-api
```

## agent history

Show agent session history including completed and failed sessions.

```bash
pm agent history [project] [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--limit` | int | `20` | Maximum number of sessions to show |

**Output columns:** ID (short), Project, Branch, Status (colored), Commits, Duration

**Examples:**

```bash
# Recent history across all projects
pm agent history

# History for one project
pm agent history my-api

# Show more results
pm agent history --limit 50
```

## Session Statuses

| Status | Description |
|--------|-------------|
| `running` | Agent is currently active |
| `completed` | Agent finished successfully |
| `failed` | Agent encountered an error |
| `abandoned` | Session was abandoned |
