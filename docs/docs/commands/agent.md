# pm agent

Manage Claude Code agent sessions. Agent sessions represent AI coding work tied to specific issues and git worktrees.

```
pm agent launch <project>       Launch a Claude agent in a new worktree
pm agent close [session_id]     Close an agent session (--done, --abandon)
pm agent list [project]         List active/idle agent sessions (alias: ls)
pm agent history [project]      Show agent session history
```

## Concepts

An **agent session** records a Claude Code agent working on a specific task:

- Each session is tied to a **project** and optionally an **issue**
- The agent works in an isolated **git worktree** on a dedicated branch
- Sessions track status (`active`, `idle`, `completed`, `abandoned`), commit count, duration, last commit info, and last active timestamp
- **Resumable**: Launching on a branch with an existing idle session resumes it instead of creating a new worktree
- **Reconciliation**: On startup, active sessions whose worktrees still exist are transitioned to idle; sessions with missing worktrees are abandoned

When launched with `--issue`, the agent session automatically:

1. Generates a branch name from the issue title (e.g., `feature/add-user-auth`)
2. Sets the issue status to `in_progress`
3. Creates a git worktree for isolated development

When closed, the session records the last commit hash and message, and cascades status to linked issues (completed -> done, abandoned -> open).

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

## agent close

Close an agent session. By default transitions to **idle** (worktree preserved). Use `--done` to mark completed or `--abandon` to mark abandoned.

```bash
pm agent close [session_id] [flags]
```

When no session ID is given, the command auto-detects the session from the current working directory (worktree path).

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--done` | bool | `false` | Mark session as completed (linked issues -> done) |
| `--abandon` | bool | `false` | Mark session as abandoned (linked issues -> open) |

On close, the session is enriched with git info from the worktree: last commit hash, last commit message, and commit count.

**Examples:**

```bash
# Pause the current session (auto-detect from cwd)
pm agent close

# Mark a session as completed
pm agent close 01J5ABCD1234 --done

# Abandon a session
pm agent close --abandon
```

## agent list

List active and idle agent sessions.

```bash
pm agent list [project]
```

Aliases: `ls`

Without `<project>`, shows sessions across all projects. With a project name, filters to that project.

**Output columns:** ID (short), Project, Branch, Status, Last Active, Started (relative time)

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

**Output columns:** ID (short), Project, Branch, Status (colored), Commits, Last Commit, Duration

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
| `active` | Agent is currently running |
| `idle` | Worktree exists but no active Claude session |
| `completed` | Agent finished successfully (issues -> done) |
| `abandoned` | Session was abandoned (issues -> open) |
