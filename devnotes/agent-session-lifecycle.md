# Agent Session Lifecycle

*2026-02-13T04:15:42Z*

## Overview

This feature adds a complete agent session lifecycle to the PM tool. Previously, agent sessions could only be launched — there was no way to close, pause, or resume them. Sessions that were abandoned left orphaned worktrees with no cleanup mechanism.

The new lifecycle supports four session statuses: **active** (Claude is running), **idle** (worktree exists but no active Claude session), **completed** (work finished successfully), and **abandoned** (work stopped without completion).

Key capabilities:
- **Close sessions** via CLI (`pm agent close`), MCP tool (`pm_close_agent`), and REST API (`POST /api/v1/agent/close`)
- **Resume idle sessions** — re-launching on the same branch reactivates an idle session instead of creating a new worktree
- **Issue status cascading** — completing a session marks its issue as "done"; abandoning marks it as "open"
- **Worktree sync** — lazy reconciliation detects when worktree directories have been removed and marks those sessions as abandoned
- **Web UI** — sessions page with Pause/Done/Abandon actions, plus a Worktrees tab on project detail pages

## CLI: Agent Close Command

The `pm agent close` command closes an agent session. By default it transitions to **idle** (worktree preserved). Use `--done` to mark completed or `--abandon` to abandon. It auto-detects the session from the current working directory when no session ID is given.

```bash
go run . agent close --help
```

```output
Close an agent session. Default transitions to idle (worktree preserved).
Use --done to mark completed (issues → done) or --abandon to abandon (issues → open).

When no session_id is given:
  - In a worktree directory: closes the session for that worktree
  - In a project directory: lists active/idle sessions to choose from

Usage:
  pm agent close [session_id] [flags]

Flags:
      --abandon   Mark session as abandoned (issues → open)
      --done      Mark session as completed (issues → done)
  -h, --help      help for close

Global Flags:
      --config string   Config file (default ~/.config/pm/config.yaml)
  -n, --dry-run         Show what would happen without making changes
  -v, --verbose         Verbose output
```

## Session Statuses and Agent List

The agent list command now shows active and idle sessions with their worktree paths. The status column is color-coded: green for active, yellow for idle.

```bash
go run . agent list
```

```output
i No active or idle agent sessions.
```

Session history shows all sessions including completed and abandoned ones:

```bash
go run . agent history
```

```output
ID            PROJECT  BRANCH                                                      STATUS     COMMITS  DURATION  
01KHAEQ11D5Q  gsi      feature/scaffolded-projects-with-a-ui-end-up-seeing-blank-  abandoned  0        1h22m     
```

## MCP Tool: pm_close_agent

A new MCP tool `pm_close_agent` allows Claude agents to close their own sessions programmatically. This is useful when an agent finishes its work and wants to mark the session as completed.

The MCP server now exposes 8 tools total:

```bash
go run . mcp status 2>&1 | head -20
```

```output
✓ ~/.claude.json: pm configured (command: /Users/joescharf/go/bin/pm)
i .mcp.json (cwd): not found
```

## REST API: POST /api/v1/agent/close

The web UI and API now support closing sessions. The endpoint accepts a JSON body with `session_id` (required) and `status` (optional: idle, completed, or abandoned — defaults to idle).

```bash
grep -A5 'POST.*agent/close' internal/api/api.go | head -6
```

```output
	mux.HandleFunc("POST /api/v1/agent/close", s.closeAgent)

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
```

## Idle Session Resume

When launching an agent on a branch that already has an idle session, PM reactivates the existing session instead of creating a new worktree. This works across all three interfaces (CLI, MCP, API). The resume logic finds the idle session, sets it back to active, and returns the existing worktree path.

## Issue Status Cascading

Closing a session cascades its status to the linked issue:
- **completed** → issue status set to **done**
- **abandoned** → issue status set to **open** (so it can be picked up again)
- **idle** → no issue change (work is paused, not finished)

This is handled by `internal/agent/lifecycle.go`:

```bash
grep -A25 'func CloseSession' internal/agent/lifecycle.go | head -30
```

```output
func CloseSession(ctx context.Context, s SessionStore, sessionID string, target models.SessionStatus) (*models.AgentSession, error) {
	session, err := s.GetAgentSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Only active or idle sessions can transition
	if session.Status != models.SessionStatusActive && session.Status != models.SessionStatusIdle {
		return nil, fmt.Errorf("session %s is already %s", sessionID, session.Status)
	}

	session.Status = target

	// Terminal statuses get an end time
	if target == models.SessionStatusCompleted || target == models.SessionStatusAbandoned {
		now := time.Now().UTC()
		session.EndedAt = &now
	}

	if err := s.UpdateAgentSession(ctx, session); err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	// Cascade issue status
	if session.IssueID != "" {
		issue, err := s.GetIssue(ctx, session.IssueID)
```

## Worktree Sync (Lazy Reconciliation)

When listing sessions (CLI or API), PM checks whether each active/idle session's worktree directory still exists on disk. If the directory has been removed (e.g. via `wt remove` or manual deletion), the session is automatically marked as abandoned. This prevents stale sessions from accumulating.

```bash
grep -A15 'func ReconcileSessions' internal/agent/sync.go
```

```output
func ReconcileSessions(ctx context.Context, s SessionStore, sessions []*models.AgentSession) int {
	cleaned := 0
	for _, sess := range sessions {
		if sess.Status != models.SessionStatusActive && sess.Status != models.SessionStatusIdle {
			continue
		}
		if sess.WorktreePath == "" {
			continue
		}
		if _, err := os.Stat(sess.WorktreePath); err == nil {
			continue
		}
		// Worktree is gone — abandon the session
		if _, err := CloseSession(ctx, s, sess.ID, models.SessionStatusAbandoned); err == nil {
			cleaned++
		}
```

## Tests

```bash
go test ./internal/agent/... -v -count=1 2>&1
```

```output
=== RUN   TestCloseSession_Idle
--- PASS: TestCloseSession_Idle (0.00s)
=== RUN   TestCloseSession_Completed
--- PASS: TestCloseSession_Completed (0.00s)
=== RUN   TestCloseSession_Abandoned
--- PASS: TestCloseSession_Abandoned (0.00s)
=== RUN   TestCloseSession_NoIssue
--- PASS: TestCloseSession_NoIssue (0.00s)
=== RUN   TestCloseSession_AlreadyClosed
--- PASS: TestCloseSession_AlreadyClosed (0.00s)
=== RUN   TestReconcileSessions_OrphanedWorktree
--- PASS: TestReconcileSessions_OrphanedWorktree (0.00s)
=== RUN   TestReconcileSessions_ExistingWorktree
--- PASS: TestReconcileSessions_ExistingWorktree (0.00s)
=== RUN   TestReconcileSessions_SkipsTerminal
--- PASS: TestReconcileSessions_SkipsTerminal (0.00s)
PASS
ok  	github.com/joescharf/pm/internal/agent	0.338s
```

```bash
go test ./internal/mcp/... -run 'Close|Integration' -v -count=1 2>&1
```

```output
=== RUN   TestHandleUpdateIssue_CloseIssue
--- PASS: TestHandleUpdateIssue_CloseIssue (0.00s)
=== RUN   TestCloseAgentTool_Idle
--- PASS: TestCloseAgentTool_Idle (0.00s)
=== RUN   TestCloseAgentTool_Completed
--- PASS: TestCloseAgentTool_Completed (0.00s)
=== RUN   TestCloseAgentTool_MissingSessionID
--- PASS: TestCloseAgentTool_MissingSessionID (0.00s)
=== RUN   TestMCPIntegration_ListTools
--- PASS: TestMCPIntegration_ListTools (0.00s)
PASS
ok  	github.com/joescharf/pm/internal/mcp	0.257s
```

```bash
go test ./internal/api/... -run CloseAgent -v -count=1 2>&1
```

```output
=== RUN   TestCloseAgent
--- PASS: TestCloseAgent (0.01s)
PASS
ok  	github.com/joescharf/pm/internal/api	0.265s
```

## Full Build Verification

```bash
go build . && echo 'Build OK'
```

```output
Build OK
```

```bash
go test ./... 2>&1 | tail -15
```

```output
?   	github.com/joescharf/pm	[no test files]
ok  	github.com/joescharf/pm/cmd	0.458s
ok  	github.com/joescharf/pm/internal/agent	0.165s
ok  	github.com/joescharf/pm/internal/api	0.952s
ok  	github.com/joescharf/pm/internal/git	(cached)
ok  	github.com/joescharf/pm/internal/golang	(cached)
ok  	github.com/joescharf/pm/internal/health	(cached)
ok  	github.com/joescharf/pm/internal/llm	(cached)
ok  	github.com/joescharf/pm/internal/mcp	0.566s
?   	github.com/joescharf/pm/internal/models	[no test files]
ok  	github.com/joescharf/pm/internal/output	(cached)
ok  	github.com/joescharf/pm/internal/standards	(cached)
ok  	github.com/joescharf/pm/internal/store	0.824s
?   	github.com/joescharf/pm/internal/ui	[no test files]
?   	github.com/joescharf/pm/internal/wt	[no test files]
```

## Files Changed

| Area | Files |
|------|-------|
| Models | `internal/models/agent_session.go` — status constants: active, idle, completed, abandoned |
| Migration | `internal/store/migrations/006_rename_session_statuses.sql` |
| Store | `internal/store/store.go`, `internal/store/sqlite.go` — GetAgentSession, GetAgentSessionByWorktreePath |
| Lifecycle | `internal/agent/lifecycle.go`, `internal/agent/lifecycle_test.go` — CloseSession with issue cascading |
| Sync | `internal/agent/sync.go`, `internal/agent/sync_test.go` — ReconcileSessions worktree check |
| CLI | `cmd/agent.go` — agent close command, idle resume on launch, updated list |
| MCP | `internal/mcp/server.go`, `internal/mcp/server_test.go` — pm_close_agent tool, idle resume |
| API | `internal/api/api.go`, `internal/api/api_test.go` — POST /api/v1/agent/close, idle resume, reconcile on list |
| UI Types | `ui/src/lib/types.ts`, `ui/src/hooks/use-agent.ts` — updated statuses, useCloseAgent hook |
| UI Pages | `ui/src/components/sessions/sessions-page.tsx` — action buttons |
| UI Pages | `ui/src/components/projects/project-detail.tsx` — Worktrees tab |
| Embed | `internal/ui/dist/` — rebuilt embedded UI |
| Docs | `CLAUDE.md` — updated CLI, MCP, and patterns sections |
