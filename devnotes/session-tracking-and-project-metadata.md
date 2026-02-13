# Session Tracking & Project Metadata Enhancements

*2026-02-13T05:02:26Z*

This batch implements 8 features across three domains: session tracking enhancements, project metadata enrichment, and web UI improvements. Together they give much better visibility into agent session state and project health.

## What was built

**Session Tracking (3 features)**
- Last commit hash and message recorded when sessions close
- Last active timestamp tracked on close and resume
- Active sessions auto-transition to idle on app restart (previously stayed stuck as active)

**Project Metadata (3 features)**
- Branch count detected and stored during `pm project refresh`
- Project description always syncs from GitHub repo About section (previously only filled when empty)
- GitHub Pages detection — stores whether Pages is configured and the URL

**Web UI (2 features)**
- Sessions page now shows a Project column for easy filtering
- Clicking a session row navigates to a detail page showing git state (commits, ahead/behind main, dirty status, worktree existence)

## Database migrations

Two new migrations add the required columns:

```bash
cat internal/store/migrations/007_add_session_tracking_fields.sql
```

```output
ALTER TABLE agent_sessions ADD COLUMN last_commit_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_sessions ADD COLUMN last_commit_message TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_sessions ADD COLUMN last_active_at DATETIME;
```

```bash
cat internal/store/migrations/008_add_project_metadata_fields.sql
```

```output
ALTER TABLE projects ADD COLUMN branch_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN has_github_pages INTEGER NOT NULL DEFAULT 0;
ALTER TABLE projects ADD COLUMN pages_url TEXT NOT NULL DEFAULT '';
```

## Tests

All tests pass, including 6 new tests covering the new functionality:

```bash
go test -v -run "TestEnrichSession|TestProjectNewFields|TestSessionNewFields|TestRefreshProject_DetectsGitHubPages|TestReconcileSessions_ExistingWorktree" ./internal/agent/ ./internal/store/ ./cmd/ 2>&1
```

```output
=== RUN   TestEnrichSessionWithGitInfo_SetsFields
--- PASS: TestEnrichSessionWithGitInfo_SetsFields (0.00s)
=== RUN   TestEnrichSessionWithGitInfo_NilClient
--- PASS: TestEnrichSessionWithGitInfo_NilClient (0.00s)
=== RUN   TestEnrichSessionWithGitInfo_EmptyWorktreePath
--- PASS: TestEnrichSessionWithGitInfo_EmptyWorktreePath (0.00s)
=== RUN   TestReconcileSessions_ExistingWorktree_ActiveToIdle
--- PASS: TestReconcileSessions_ExistingWorktree_ActiveToIdle (0.00s)
=== RUN   TestReconcileSessions_ExistingWorktree_IdleStaysIdle
--- PASS: TestReconcileSessions_ExistingWorktree_IdleStaysIdle (0.00s)
PASS
ok  	github.com/joescharf/pm/internal/agent	0.201s
=== RUN   TestProjectNewFields
--- PASS: TestProjectNewFields (0.01s)
=== RUN   TestSessionNewFields
--- PASS: TestSessionNewFields (0.00s)
PASS
ok  	github.com/joescharf/pm/internal/store	0.412s
=== RUN   TestRefreshProject_DetectsGitHubPages
  → BranchCount: 0 -> 1
  → GitHub Pages: https://test.github.io
--- PASS: TestRefreshProject_DetectsGitHubPages (0.01s)
PASS
ok  	github.com/joescharf/pm/cmd	0.361s
```

## Full test suite

```bash
go test -race -count=1 ./... 2>&1
```

```output
?   	github.com/joescharf/pm	[no test files]
ok  	github.com/joescharf/pm/cmd	2.564s
ok  	github.com/joescharf/pm/internal/agent	1.254s
ok  	github.com/joescharf/pm/internal/api	1.920s
ok  	github.com/joescharf/pm/internal/git	2.163s
ok  	github.com/joescharf/pm/internal/golang	2.889s
ok  	github.com/joescharf/pm/internal/health	2.703s
ok  	github.com/joescharf/pm/internal/llm	1.845s
ok  	github.com/joescharf/pm/internal/mcp	2.542s
?   	github.com/joescharf/pm/internal/models	[no test files]
ok  	github.com/joescharf/pm/internal/output	3.057s
ok  	github.com/joescharf/pm/internal/standards	1.643s
ok  	github.com/joescharf/pm/internal/store	2.560s
?   	github.com/joescharf/pm/internal/ui	[no test files]
?   	github.com/joescharf/pm/internal/wt	[no test files]
```

## Files changed

**New files:**
- `internal/store/migrations/007_add_session_tracking_fields.sql` — Session tracking columns
- `internal/store/migrations/008_add_project_metadata_fields.sql` — Project metadata columns
- `internal/agent/git_enrich.go` — Git enrichment helper for sessions
- `internal/agent/git_enrich_test.go` — Tests for git enrichment
- `ui/src/components/sessions/session-detail.tsx` — Session detail page component

**Modified backend files:**
- `internal/models/agent_session.go` — Added LastCommitHash, LastCommitMessage, LastActiveAt fields
- `internal/models/project.go` — Added BranchCount, HasGitHubPages, PagesURL fields
- `internal/store/sqlite.go` — Updated all session and project CRUD for new columns
- `internal/git/git.go` — Added CommitCountSince and AheadBehind to Client interface
- `internal/git/github.go` — Added PagesInfo to GitHubClient interface
- `internal/agent/sync.go` — Active-to-idle reconciliation on app restart
- `internal/api/api.go` — Session list enrichment, GET /sessions/{id} endpoint
- `internal/mcp/server.go` — Enrichment wiring, Pages in project status
- `cmd/agent.go` — Enrichment wiring, updated list/history display
- `cmd/project.go` — Refresh: branch count, description sync, Pages detection

**Modified frontend files:**
- `ui/src/lib/types.ts` — New TypeScript interfaces for session detail
- `ui/src/hooks/use-sessions.ts` — Added useSession hook
- `ui/src/components/sessions/sessions-page.tsx` — Project column, clickable rows
- `ui/src/App.tsx` — Session detail route

**Modified test files:**
- `internal/store/sqlite_test.go` — TestProjectNewFields, TestSessionNewFields
- `internal/agent/sync_test.go` — Active-to-idle and idle-stays-idle tests
- `cmd/project_refresh_test.go` — Updated mocks, GitHub Pages test
- `internal/mcp/server_test.go` — Updated mock clients
