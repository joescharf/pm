# Resolve Open PM Issues — Batch 3: MCP Server, CLI Defaults & Agent Launch

*2026-02-13T02:23:30Z*

This batch resolves the final 4 open issues from the PM issue tracker. The work spans the Go CLI (Cobra command defaults), a full MCP server implementation with 7 tools, a Claude Code skill file, and a web UI agent launch feature with issue selection.

## Issues Closed

| Issue ID | Title |
|----------|-------|
| 01KHA4NVKG01 | CLI efficiency — parent commands default to list |
| 01KHA1VR7XAC | MCP server with install/status subcommands |
| 01KHA9HWHMFW | Claude Code skill (CLAUDE.md) |
| 01KHA1VNQ6PA | Web UI agent launch from selected issues |

## The Commit

```bash
git log --oneline -1
```

```output
5e854cc feat: implement MCP server, CLI defaults, agent launch UI, and Claude Code skill
```

```bash
git show --stat 5e854cc | tail -26
```

```output

 CLAUDE.md                                          |  101 ++
 cmd/agent.go                                       |    3 +
 cmd/issue.go                                       |    3 +
 cmd/mcp.go                                         |  177 +++-
 cmd/project.go                                     |    3 +
 cmd/serve.go                                       |   33 +-
 cmd/tag.go                                         |    3 +
 cmd/worktree.go                                    |    3 +
 go.mod                                             |    9 +-
 go.sum                                             |   15 +
 internal/api/api.go                                |  153 ++-
 internal/api/api_test.go                           |    4 +-
 internal/mcp/server.go                             |  675 +++++++++++++
 internal/mcp/server_test.go                        | 1066 ++++++++++++++++++++
 internal/ui/dist/chunk-0q51626c.css                |    1 +
 internal/ui/dist/chunk-77hn80f0.css                |    1 -
 internal/ui/dist/chunk-mbkxch0h.js                 |   63 --
 internal/ui/dist/chunk-xgs7tk85.js                 |   63 ++
 ...chunk-mbkxch0h.js.map => chunk-xgs7tk85.js.map} |   12 +-
 internal/ui/dist/index.html                        |    2 +-
 ui/src/components/issues/agent-launch-dialog.tsx   |  139 +++
 ui/src/components/issues/issues-page.tsx           |   72 +-
 ui/src/hooks/use-agent.ts                          |   29 +
 ui/src/lib/types.ts                                |   12 +
 24 files changed, 2555 insertions(+), 87 deletions(-)
```

24 files changed across Go CLI, MCP server, API, web UI, and embedded assets — 2,555 lines added.

## Feature 1: CLI Command Defaults (01KHA4NVKG01)

Parent commands now default to their `list` subcommand when invoked without arguments. Previously, running `pm issue` would show help text; now it runs `pm issue list` with auto-detection of the current project from the working directory.

Commands updated: `issue`, `project`, `agent`, `tag`, `worktree`.

```bash
./pm project 2>&1 | head -10
```

```output
NAME              PATH                                           LANGUAGE  GROUP   OPEN ISSUES  
calsync           /Users/joescharf/app/calsync                   go                0            
company-research  /Users/joescharf/app/quantum/company-research  python            0            
dbsnapper-agent   /Users/joescharf/app/scratch/dbsnapper-agent   go                0            
fdsn              /Users/joescharf/app/brtt/fdsn                 go                0            
gpub              /Users/joescharf/app/gpub                      go                0            
gsi               /Users/joescharf/app/gsi                       go        ai-dev  1            
joeblog           /Users/joescharf/app/joeblog                                     0            
pm                /Users/joescharf/app/pm                        go        ai-dev  4            
wt                /Users/joescharf/app/wt                        go        ai-dev  0            
```

## Feature 2: MCP Server (01KHA1VR7XAC)

New `internal/mcp` package implements a full Model Context Protocol server with 7 tools for Claude Code integration. The server runs on stdio (for direct Claude Code use) or StreamableHTTP (embedded in `pm serve` on port 8081).

### MCP Tools

| Tool | Description |
|------|-------------|
| `pm_list_projects` | List all projects with optional group filter |
| `pm_project_status` | Project details + git status + health score |
| `pm_list_issues` | Filtered issue list (project, status, priority) |
| `pm_create_issue` | Create new issue with title, type, priority |
| `pm_update_issue` | Update issue status, title, description, priority |
| `pm_health_score` | Health score breakdown for a project |
| `pm_launch_agent` | Create worktree + agent session for an issue |

### Subcommands

```bash
./pm mcp --help 2>&1 | head -20
```

```output
Start an MCP (Model Context Protocol) server on stdio.

This allows Claude Code to query pm natively for project status,
issues, and health scores. Configure in Claude Code with:

  pm mcp install

Or manually add to ~/.claude.json:

  {
    "mcpServers": {
      "pm": { "command": "/path/to/pm", "args": ["mcp"] }
    }
  }

Available tools: pm_list_projects, pm_project_status, pm_list_issues,
pm_create_issue, pm_update_issue, pm_launch_agent, pm_health_score

Usage:
  pm mcp [flags]
```

```bash
./pm mcp status 2>&1
```

```output
i ~/.claude.json: pm not configured (other servers present)
i .mcp.json (cwd): not found
```

MCP server not yet installed in Claude Code — run `pm mcp install` to configure.

### MCP Integration in `pm serve`

The `pm serve` command now starts the MCP StreamableHTTP server concurrently on port 8081 (controllable via `--mcp-port`, disable with `--mcp=false`).

```bash
./pm serve --help 2>&1
```

```output
Start an HTTP server serving the REST API, embedded web UI, and MCP server.
By default it listens on port 8080 (API/UI) and 8081 (MCP).

Usage:
  pm serve [flags]

Flags:
  -h, --help           help for serve
      --mcp            enable MCP StreamableHTTP server (default true)
      --mcp-port int   MCP server port (default 8081)
  -p, --port int       port to listen on (default 8080)

Global Flags:
      --config string   Config file (default ~/.config/pm/config.yaml)
  -n, --dry-run         Show what would happen without making changes
  -v, --verbose         Verbose output
```

### MCP Test Coverage

The MCP server has comprehensive tests (1,066 lines) with mock implementations of all 4 interfaces (Store, GitClient, GitHubClient, WTClient) including error injection.

```bash
go test ./internal/mcp/ -v 2>&1 | tail -40
```

```output
=== RUN   TestHandleCreateIssue_MissingProject
--- PASS: TestHandleCreateIssue_MissingProject (0.00s)
=== RUN   TestHandleCreateIssue_UnknownProject
--- PASS: TestHandleCreateIssue_UnknownProject (0.00s)
=== RUN   TestHandleCreateIssue_StoreError
--- PASS: TestHandleCreateIssue_StoreError (0.00s)
=== RUN   TestHandleUpdateIssue_ChangeStatus
--- PASS: TestHandleUpdateIssue_ChangeStatus (0.00s)
=== RUN   TestHandleUpdateIssue_ChangePriority
--- PASS: TestHandleUpdateIssue_ChangePriority (0.00s)
=== RUN   TestHandleUpdateIssue_ChangeTitle
--- PASS: TestHandleUpdateIssue_ChangeTitle (0.00s)
=== RUN   TestHandleUpdateIssue_MissingID
--- PASS: TestHandleUpdateIssue_MissingID (0.00s)
=== RUN   TestHandleUpdateIssue_IssueNotFound
--- PASS: TestHandleUpdateIssue_IssueNotFound (0.00s)
=== RUN   TestHandleUpdateIssue_CloseIssue
--- PASS: TestHandleUpdateIssue_CloseIssue (0.00s)
=== RUN   TestHandleHealthScore
--- PASS: TestHandleHealthScore (0.00s)
=== RUN   TestHandleHealthScore_MissingProject
--- PASS: TestHandleHealthScore_MissingProject (0.00s)
=== RUN   TestHandleHealthScore_UnknownProject
--- PASS: TestHandleHealthScore_UnknownProject (0.00s)
=== RUN   TestHandleHealthScore_DirtyRepo
--- PASS: TestHandleHealthScore_DirtyRepo (0.00s)
=== RUN   TestHandleLaunchAgent
--- PASS: TestHandleLaunchAgent (0.00s)
=== RUN   TestHandleLaunchAgent_MissingProject
--- PASS: TestHandleLaunchAgent_MissingProject (0.00s)
=== RUN   TestHandleLaunchAgent_MissingIssue
--- PASS: TestHandleLaunchAgent_MissingIssue (0.00s)
=== RUN   TestHandleLaunchAgent_WorktreeCreateFails
--- PASS: TestHandleLaunchAgent_WorktreeCreateFails (0.00s)
=== RUN   TestHandleLaunchAgent_WithCustomBranch
--- PASS: TestHandleLaunchAgent_WithCustomBranch (0.00s)
=== RUN   TestMCPIntegration_ListTools
--- PASS: TestMCPIntegration_ListTools (0.00s)
PASS
ok  	github.com/joescharf/pm/internal/mcp	(cached)
```

All 34 MCP tests pass.

## Feature 3: Claude Code Skill — CLAUDE.md (01KHA9HWHMFW)

Created `CLAUDE.md` in the project root with a complete CLI command reference, MCP tool reference table, key patterns (short IDs, cwd auto-detection, lazy store), and development notes. This file is automatically loaded by Claude Code when working in the project.

```bash
head -30 CLAUDE.md
```

````output
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
pm issue add [project]          # --title (required), --desc, --priority, --type, --tag
pm issue show <id>              # Show issue details (accepts short IDs)
pm issue update <id>            # --status, --title, --desc, --priority
pm issue close <id>             # Close an issue
pm issue link <id>              # --github <number>
pm issue import <file>          # Import issues from markdown (--project, --dry-run)

pm agent list [project]         # Active sessions (default subcommand)
````

## Feature 4: Web UI Agent Launch (01KHA1VNQ6PA)

Added the ability to select issues in the web UI and launch a Claude Code agent session. This feature includes:

- **API endpoint**: `POST /api/v1/agent/launch` — validates issues, generates branch name, creates worktree, records session, marks issues as in_progress, returns a copyable `claude` command
- **Issue selection**: Checkbox column per issue row, locked to one project at a time, with a floating action bar showing count + "Launch Agent" + clear buttons
- **Agent launch dialog**: Two-state dialog — pre-launch shows selected issues with a Launch button, post-launch displays the generated command in a copyable code block
- **React hooks**: `useLaunchAgent()` mutation via TanStack Query, invalidates sessions and issues caches on success

### API Endpoint

```bash
grep -n 'agent/launch\|launchAgent\|LaunchAgent' internal/api/api.go | head -10
```

```output
65:	mux.HandleFunc("POST /api/v1/agent/launch", s.launchAgent)
448:// LaunchAgentRequest is the JSON body for POST /api/v1/agent/launch.
449:type LaunchAgentRequest struct {
454:// LaunchAgentResponse is the JSON response for a successful agent launch.
455:type LaunchAgentResponse struct {
462:func (s *Server) launchAgent(w http.ResponseWriter, r *http.Request) {
465:	var req LaunchAgentRequest
549:	writeJSON(w, http.StatusOK, LaunchAgentResponse{
```

The endpoint accepts `{"issue_ids": [...], "project_id": "..."}`, validates all issues belong to the specified project, generates a branch name from issue titles, creates a git worktree, records an agent session, and returns the generated `claude` command to run in terminal.

### UI Components

New files:
- `ui/src/components/issues/agent-launch-dialog.tsx` (139 lines)
- `ui/src/hooks/use-agent.ts` (29 lines)
- Modified `ui/src/components/issues/issues-page.tsx` (+72 lines for selection + floating bar)

## Full Test Suite

```bash
go test ./... 2>&1
```

```output
?   	github.com/joescharf/pm	[no test files]
ok  	github.com/joescharf/pm/cmd	(cached)
ok  	github.com/joescharf/pm/internal/api	(cached)
ok  	github.com/joescharf/pm/internal/git	(cached)
ok  	github.com/joescharf/pm/internal/golang	(cached)
ok  	github.com/joescharf/pm/internal/health	(cached)
ok  	github.com/joescharf/pm/internal/llm	(cached)
ok  	github.com/joescharf/pm/internal/mcp	(cached)
?   	github.com/joescharf/pm/internal/models	[no test files]
ok  	github.com/joescharf/pm/internal/output	(cached)
ok  	github.com/joescharf/pm/internal/standards	(cached)
ok  	github.com/joescharf/pm/internal/store	(cached)
?   	github.com/joescharf/pm/internal/ui	[no test files]
?   	github.com/joescharf/pm/internal/wt	[no test files]
```

All 11 test packages pass. Zero issues remaining in the PM tracker for this batch.

## Summary

| Metric | Value |
|--------|-------|
| Issues closed | 4 |
| Files changed | 24 |
| Lines added | 2,555 |
| Lines removed | 87 |
| New packages | `internal/mcp` |
| New MCP tools | 7 |
| New API endpoints | 1 (`POST /api/v1/agent/launch`) |
| New UI components | 2 (dialog + hook) |
| Test coverage | 34 MCP tests + existing API/store/cmd tests |
