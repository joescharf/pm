# Workflows

End-to-end usage patterns for common tasks.

## Onboarding Existing Projects

Quickly add all your existing repositories:

```bash
# Scan a parent directory
pm project scan ~/code

# Organize into groups
pm project add ~/code/api-gateway --group backend
pm project add ~/code/web-app --group frontend
pm project add ~/code/shared-lib --group libraries
```

Check the baseline state of everything:

```bash
pm status
pm standards
```

## Issue-Driven Development with AI Agents

The full cycle from creating an issue to completing work with an AI agent:

```bash
# 1. Create an issue
pm issue add my-api --title "Add rate limiting middleware" --type feature --priority high

# 2. Note the issue ID from the output (e.g., 01J5ABCD1234)

# 3. Launch an agent session tied to the issue
#    This creates a worktree and sets the issue to in_progress
pm agent launch my-api --issue 01J5ABCD1234

# 4. Monitor active sessions
pm agent list

# 5. Review session history after completion
pm agent history my-api

# 6. Close the issue when the work is merged
pm issue close 01J5ABCD1234
```

The agent launch automatically:

- Generates a branch name from the issue title (e.g., `feature/add-rate-limiting-middleware`)
- Creates a git worktree for isolated development
- Records the session for tracking

## GitHub Integration

Link pm issues to GitHub issues for cross-referencing:

```bash
# Create a pm issue
pm issue add my-api --title "Fix auth token expiry" --type bug

# Link it to a GitHub issue
pm issue link 01J5ABCD1234 --github 42

# The GitHub issue number shows up in listings
pm issue list my-api
```

## Standards Auditing

Check all projects against standard conventions:

```bash
# Audit everything
pm standards

# Audit a single project
pm standards my-api
```

The 9 checks cover: `.goreleaser.yml`, `Makefile`, `CLAUDE.md`, `.mockery.yml`, `LICENSE`, `README.md`, `go.mod`, `internal/` directory, and test files.

## Weekly Review

A typical weekly review workflow:

```bash
# 1. Check which projects are stale (no activity in 7+ days)
pm status --stale

# 2. Generate a weekly summary report
pm report weekly

# 3. Export data for record-keeping
pm export --type issues --format csv > ~/reports/issues-$(date +%Y-%m-%d).csv
pm export --type projects --format json > ~/reports/projects-$(date +%Y-%m-%d).json

# 4. Review health scores
pm status
```

## Using the Web Dashboard

Launch the embedded web UI for a visual overview:

```bash
pm serve
# Open http://localhost:8080 in your browser
```

The dashboard provides the same data as the CLI but in a visual format. The REST API is also available for custom integrations at `http://localhost:8080/api/v1/`.

Use a custom port if 8080 is taken:

```bash
pm serve --port 3000
```

## Dry Run / Preview Mode

Most commands that modify state support `--dry-run` to preview what would happen:

```bash
# Preview adding a project
pm project add ~/code/new-repo --dry-run

# Preview scanning for projects
pm project scan ~/code --dry-run

# Preview creating a config file
pm config init --dry-run

# Preview launching an agent
pm agent launch my-api --issue 01J5ABCD1234 --dry-run
```

Dry-run output is prefixed with `[DRY-RUN]` and no changes are made.

## Filtering and Organizing

### By project group

```bash
# List projects in a group
pm project list --group backend

# Status for a group
pm status --group backend
```

### By issue criteria

```bash
# Open high-priority issues
pm issue list --status open --priority high

# All issues tagged "backend"
pm issue list --tag backend --all

# Issues for a specific project
pm issue list my-api
```

### By staleness

```bash
# Projects with no recent activity
pm status --stale
```
