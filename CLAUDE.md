# PM - Project Manager CLI

pm is a CLI tool for managing multiple development projects, issues, and agent sessions.
Built with Go (Cobra + Viper CLI, SQLite storage, embedded React web UI).

- DB: `~/.config/pm/pm.db` | Config: `~/.config/pm/config.yaml`
- Store interface: `internal/store/store.go`
- Output helper: `internal/output/output.go` (the `ui` var in cmd/ is `*output.UI`)
- Embedded web UI source: `ui/` | Embedded at: `internal/ui/`

## CLI Commands

```
pm                              # Auto-detect project from cwd, show details
pm project list                 # List all tracked projects (default subcommand)
pm project add <path>           # Add project (--name, --group)
pm project show <name>          # Detailed project info with git status
pm project remove <name>        # Remove from tracking
pm project scan <dir>           # Auto-discover git repos
pm project refresh [name]       # Re-detect metadata

pm issue list [project]         # List issues (default subcommand)
pm issue add [project]          # --title (required), --desc, --priority, --type, --tag, --ai-prompt, --no-enrich
pm issue show <id>              # Show issue details (accepts short IDs); displays AI Prompt
pm issue update <id>            # --status, --title, --desc, --priority, --ai-prompt
pm issue close <id>             # Close an issue
pm issue link <id>              # --github <number>
pm issue import <file>          # Import issues from markdown (--project, --dry-run); auto-classifies type/priority
pm issue review <id>            # Show review history (--base-ref, --head-ref, --app-url)

pm agent list [project]         # Active/idle sessions (default subcommand)
pm agent launch <project>       # --issue, --branch (resumes idle sessions)
pm agent close [session_id]     # Close session (--done, --abandon; auto-detects from cwd)
pm agent sync [session_id]      # Sync worktree with base branch (--rebase, --force; auto-detects from cwd)
pm agent merge [session_id]     # Merge branch into base (auto-detects from cwd)
pm agent discover [project]     # Discover untracked worktrees
pm agent history [project]      # Session history

pm tag list                     # List tags (default subcommand)
pm tag create <name>            # Create a tag
pm tag delete <name>            # Delete a tag

pm worktree list [project]      # List worktrees (default; aliases: pm wt)
pm worktree create <proj> <br>  # Create worktree

pm status [project]             # Dashboard overview
pm standards [project]          # Project standards
pm serve                        # Start web UI + API (--port, --mcp, --mcp-port, --daemon/-d)
pm serve start                  # Start server in the background
pm serve stop                   # Stop background server
pm serve restart                # Restart background server
pm serve status                 # Show background server status
pm export                       # Export data (--format json|csv|md)
pm report weekly                # Weekly report

pm mcp                          # Start MCP stdio server
pm mcp serve                    # Start MCP SSE server
pm mcp install                  # Install pm in ~/.claude.json
pm mcp status                   # Check MCP installation

pm config show                  # Show current config
pm config init                  # Initialize config
pm config edit                  # Open config in editor
pm version                      # Show version info
```

## MCP Tools

When the MCP server is available, prefer MCP tools over CLI for programmatic access:

| Tool | Description |
|------|-------------|
| `pm_list_projects` | List all projects (opt: group filter) |
| `pm_project_status` | Full project status with git info + health (project required) |
| `pm_list_issues` | List issues with ai_prompt in output (opt: project, status, priority) |
| `pm_create_issue` | Create issue with auto LLM enrichment (project + title required; opt: description, type, priority, ai_prompt, enrich) |
| `pm_update_issue` | Update issue fields (issue_id required; opt: status, title, description, priority, ai_prompt) |
| `pm_health_score` | Health score breakdown for a project (project required) |
| `pm_launch_agent` | Create worktree + agent session, or resume idle session (project required; opt: issue_id, branch) |
| `pm_close_agent` | Close agent session (session_id required; opt: status — idle/completed/abandoned) |
| `pm_sync_session` | Sync session worktree with base branch (session_id required; opt: rebase, force, dry_run) |
| `pm_merge_session` | Merge session branch into base (session_id required; opt: base_branch, create_pr, force, dry_run) |
| `pm_delete_worktree` | Delete session worktree and abandon session (session_id required; opt: force) |
| `pm_discover_worktrees` | Discover untracked worktrees and create session records (opt: project) |
| `pm_prepare_review` | Gather review context for an issue (issue_id required; opt: base_ref, head_ref, app_url) |
| `pm_save_review` | Save review verdict and transition issue (issue_id + verdict + summary required; opt: categories, failure_reasons) |
| `pm_update_project` | Update project metadata (project required; opt: description, build_cmd, serve_cmd, serve_port) |

## Key Patterns

- **Short IDs**: First 12 chars of ULID (e.g., `01KHA4NVKG01`)
- **Auto-detection**: `pm` and `pm issue` auto-detect project from cwd
- **Issue lifecycle**: open -> in_progress -> done -> [AI review] -> closed (pass) / in_progress (fail)
- **Session lifecycle**: active -> idle -> completed/abandoned (idle = worktree exists, no active Claude session)
- **Session operations**: sync (pull base into feature), merge (feature into base), delete worktree, discover untracked worktrees
- **Conflict states**: none, sync_conflict, merge_conflict — tracked on sessions with conflict file list
- **Issue cascading**: session completed -> issue done; session abandoned -> issue open; review pass -> closed; review fail -> in_progress
- **Priorities**: low, medium, high
- **Types**: feature, bug, chore
- **Config**: Uses `viper.SetDefault()` with nested keys like `github.default_org`
- **Store init**: Lazy via `getStore()` -- only when commands need DB
- **ULID keys**: All entities use ULID primary keys
- **LLM enrichment**: Issues are auto-enriched on creation (CLI, MCP, API) when an Anthropic API key is configured. Generates `Description` (summary) and `AIPrompt` (agent guidance). Skip with `--no-enrich` (CLI) or `enrich=false` (MCP). Manual enrichment via `POST /api/v1/issues/{id}/enrich` or UI Enrich button.
- **AI Prompt field**: `AIPrompt` on issues provides structured guidance for AI agents working on the issue. Agents should read this field for implementation context.

## Development

```bash
go build .                    # Quick build
make build                    # Build with version ldflags
make test                     # go test -v -race -count=1 ./...
make lint                     # golangci-lint
go test ./...                 # Fast test run

cd ui && bun install          # Install UI deps
cd ui && bun run dev          # UI dev server
make ui-build                 # Build UI for production
make ui-embed                 # Copy ui/dist -> internal/ui/dist
```

- LDFLAGS target `main.version`, `main.commit`, `main.date` (not `cmd.*`)
- tablewriter v1.1 API: `NewTable()` with options, `Header()`, `Append()`, `Render()` (no `SetHeader`/`SetBorder`)
- SQLite via `modernc.org/sqlite` (pure Go), WAL mode
- MCP via `mark3labs/mcp-go`
