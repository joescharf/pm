---
name: pm-workflow
description: Use when working on projects tracked by PM, managing issues, launching agent sessions, or checking project status. Provides workflows for the PM project management system.
---

# PM Workflow Skill

## Overview
PM is a project management CLI and MCP server for tracking projects, issues, and agent sessions.

## MCP Tools (Preferred)
When the PM MCP server is available, use these tools for programmatic access:

| Tool | Usage |
|------|-------|
| `pm_list_projects` | List all tracked projects. Optional: `group` filter |
| `pm_project_status` | Get project details, git info, health score. Required: `project` name |
| `pm_list_issues` | List issues. Optional: `project`, `status` (open/in_progress/done/closed), `priority` (low/medium/high) |
| `pm_create_issue` | Create an issue. Required: `project`, `title`. Optional: `description`, `type` (feature/bug/chore), `priority` |
| `pm_update_issue` | Update issue fields. Required: `issue_id` (full ULID or 12-char prefix). Optional: `status`, `title`, `description`, `priority` |
| `pm_health_score` | Get health score breakdown. Required: `project` name |
| `pm_launch_agent` | Create worktree + agent session. Required: `project`. Optional: `issue_id`, `branch` |
| `pm_close_agent` | Close session. Required: `session_id`. Optional: `status` (idle/completed/abandoned) |

## Common Workflows

### Starting work on an issue
1. `pm_list_issues` with `status: "open"` to find available issues
2. `pm_launch_agent` with `project` and `issue_id` to create a worktree and session
3. Work on the issue in the worktree
4. `pm_close_agent` with `status: "completed"` when done (auto-marks issue as done)

### Creating and tracking issues
1. `pm_create_issue` with project name, title, and optional description/type/priority
2. Use `pm_update_issue` to change status as work progresses
3. Issue lifecycle: open -> in_progress -> done -> closed

### Checking project health
1. `pm_project_status` for a quick overview including git info and issue counts
2. `pm_health_score` for detailed breakdown (git cleanliness, activity, issues, releases, branches)

### Session management
- Sessions auto-detect idle state when worktree exists but no active Claude session
- `pm_close_agent` with `status: "abandoned"` reopens the linked issue
- `pm_launch_agent` resumes idle sessions for the same project

## CLI Fallback
If MCP tools are unavailable, use the `pm` CLI via Bash:
```bash
pm project list                    # List projects
pm issue list <project>            # List issues
pm issue add <project> --title "..." --type bug --priority high
pm issue update <id> --status done
pm agent launch <project> --issue <id>
pm agent close --done
pm status <project>                # Dashboard overview
```

## Key Concepts
- **Short IDs**: First 12 chars of ULID work for all ID-accepting commands
- **Auto-detection**: Running `pm` or `pm issue` in a tracked project directory auto-detects the project
- **Priorities**: low, medium, high
- **Types**: feature, bug, chore
