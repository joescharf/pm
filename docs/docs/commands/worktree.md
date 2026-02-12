# pm worktree

Manage git worktrees for tracked projects.

```
pm worktree list [project]      List worktrees (alias: ls)
pm worktree create <project> <branch>  Create a worktree
```

Alias: `pm wt`

## worktree list

List git worktrees for one or all tracked projects.

```bash
pm worktree list [project]
```

Aliases: `ls`

**Without `<project>`:** Lists worktrees for all tracked projects. Output columns: Project, Branch, Path.

**With `<project>`:** Lists worktrees for that project only. Output columns: Branch, Path.

**Examples:**

```bash
# All worktrees across all projects
pm wt list

# Worktrees for a specific project
pm wt ls my-api
```

## worktree create

Create a new git worktree for a project on a given branch.

```bash
pm worktree create <project> <branch>
```

Creates the worktree using the external `wt` CLI. The worktree path is determined by the `wt` tool's conventions.

**Examples:**

```bash
pm worktree create my-api feature/new-endpoint
pm wt create my-api bugfix/auth-fix

# Preview without creating
pm wt create my-api feature/test --dry-run
```

## Integration with Agent Sessions

When using `pm agent launch`, a worktree is created automatically. You typically only need `pm worktree create` for manual worktree management outside of agent sessions.
