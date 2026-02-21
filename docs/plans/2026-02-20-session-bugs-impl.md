# Session Bugs Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix duplicate session creation on launch and add automatic active/idle detection via claude process detection.

**Architecture:** Two independent bug fixes. Bug 1 fixes control flow in CLI/MCP launch + adds a DB uniqueness constraint. Bug 2 adds a `ProcessDetector` interface to `ReconcileSessions` that checks for running claude processes in worktree paths via `pgrep`/`lsof`.

**Tech Stack:** Go, SQLite migrations, `os/exec` for process detection, `testify` for tests.

---

### Task 1: Add DB Migration — Dedup Existing Sessions + Partial Unique Index

**Files:**
- Create: `internal/store/migrations/014_dedup_sessions_unique_branch.sql`

**Step 1: Write the migration SQL**

Create `internal/store/migrations/014_dedup_sessions_unique_branch.sql`:

```sql
-- Deduplicate: for each (project_id, branch) with multiple active/idle sessions,
-- keep the one with the latest started_at, abandon the rest.
UPDATE agent_sessions
SET status = 'abandoned',
    ended_at = datetime('now')
WHERE id IN (
    SELECT a.id
    FROM agent_sessions a
    INNER JOIN (
        SELECT project_id, branch, MAX(started_at) AS max_started
        FROM agent_sessions
        WHERE status IN ('active', 'idle')
        GROUP BY project_id, branch
        HAVING COUNT(*) > 1
    ) dups ON a.project_id = dups.project_id
         AND a.branch = dups.branch
         AND a.status IN ('active', 'idle')
         AND a.started_at < dups.max_started
);

-- Safety net: prevent future duplicates for same project+branch when active/idle
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_sessions_active_branch
ON agent_sessions(project_id, branch)
WHERE status IN ('active', 'idle');
```

**Step 2: Run tests to verify migration applies cleanly**

Run: `go test ./internal/store/ -run TestMigrate -v -count=1`
Expected: PASS (existing Migrate tests should pick up new migration file)

**Step 3: Commit**

```bash
git add internal/store/migrations/014_dedup_sessions_unique_branch.sql
git commit -m "fix: add migration to dedup sessions and add unique branch constraint"
```

---

### Task 2: Fix CLI Launch Resume Loop

**Files:**
- Modify: `cmd/agent.go:204-234`

**Step 1: Write the failing test**

This is a control-flow fix in the Cobra command handler. The test is best done as an integration test later (Task 5). For now, we fix the code directly since the bug is a clear logic error.

**Step 2: Fix the resume loop control flow**

In `cmd/agent.go`, replace the resume loop (lines ~204-234) with:

```go
	// Check for existing idle session on this branch
	existingSessions, _ := s.ListAgentSessions(ctx, p.ID, 0)
	for _, sess := range existingSessions {
		if sess.Branch == branch && sess.Status == models.SessionStatusIdle {
			if dryRun {
				ui.DryRunMsg("Would resume session %s for %s on branch %s", shortID(sess.ID), p.Name, branch)
				return nil
			}
			// Resume: reactivate existing session, open iTerm window
			wtClient := wt.NewClient()
			ui.Info("Opening worktree for branch: %s", output.Cyan(branch))
			if err := wtClient.Create(p.Path, branch); err != nil {
				return fmt.Errorf("wt open: %w", err)
			}
			sess.Status = models.SessionStatusActive
			now := time.Now().UTC()
			sess.LastActiveAt = &now
			if err := s.UpdateAgentSession(ctx, sess); err != nil {
				return fmt.Errorf("failed to reactivate session %s: %w", shortID(sess.ID), err)
			}
			resumePath := sess.WorktreePath
			ui.Success("Resumed session %s for %s on branch %s", output.Cyan(shortID(sess.ID)), output.Cyan(p.Name), output.Cyan(branch))
			if resolvedIssueID != "" {
				ui.Info("Run: cd %s && claude \"Use pm MCP tools to look up issue %s and implement it. Update the issue status when complete.\"", resumePath, shortID(resolvedIssueID))
			} else {
				ui.Info("Run: cd %s && claude", resumePath)
			}
			return nil
		}
	}
```

Key changes:
- On `UpdateAgentSession` failure: `return fmt.Errorf(...)` instead of `ui.Warning()` + continue
- The `else` block wrapping the success path is removed — success is the only path now (error returns above)

**Step 3: Build to verify compilation**

Run: `go build .`
Expected: Success, no errors

**Step 4: Commit**

```bash
git add cmd/agent.go
git commit -m "fix: return error on session resume failure instead of creating duplicate"
```

---

### Task 3: Fix MCP Launch Resume Loop

**Files:**
- Modify: `internal/mcp/server.go:630-666`

**Step 1: Fix the resume loop control flow**

In `internal/mcp/server.go`, replace the resume loop (lines ~630-666) with:

```go
	// Check for existing idle session on this branch
	existingSessions, _ := s.store.ListAgentSessions(ctx, p.ID, 0)
	for _, sess := range existingSessions {
		if sess.Branch == branch && sess.Status == models.SessionStatusIdle {
			// Open iTerm window via wt open
			if s.wt != nil {
				if err := s.wt.Create(p.Path, branch); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("wt open: %v", err)), nil
				}
			}
			sess.Status = models.SessionStatusActive
			now := time.Now().UTC()
			sess.LastActiveAt = &now
			if err := s.store.UpdateAgentSession(ctx, sess); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("failed to reactivate session %s: %v", sess.ID, err)), nil
			}
			command := fmt.Sprintf("cd %s && claude", sess.WorktreePath)
			if issueID != "" {
				shortIssueID := issueID
				if len(shortIssueID) > 12 {
					shortIssueID = shortIssueID[:12]
				}
				command = fmt.Sprintf(`cd %s && claude "Use pm MCP tools to look up issue %s and implement it. Update the issue status when complete."`, sess.WorktreePath, shortIssueID)
			}
			result := map[string]any{
				"session_id":    sess.ID,
				"project":       p.Name,
				"branch":        branch,
				"worktree_path": sess.WorktreePath,
				"issue_id":      issueID,
				"status":        string(sess.Status),
				"resumed":       true,
				"command":       command,
			}
			data, _ := json.Marshal(result)
			return mcp.NewToolResultText(string(data)), nil
		}
	}
```

Key change:
- On `UpdateAgentSession` failure: `return mcp.NewToolResultError(...)` instead of silently falling through

**Step 2: Build to verify compilation**

Run: `go build .`
Expected: Success, no errors

**Step 3: Commit**

```bash
git add internal/mcp/server.go
git commit -m "fix: MCP launch returns error on session resume failure instead of creating duplicate"
```

---

### Task 4: Add Process Detection and Update ReconcileSessions

**Files:**
- Create: `internal/agent/process.go`
- Modify: `internal/agent/sync.go`

**Step 1: Write the failing tests**

Add to `internal/agent/sync_test.go`:

```go
// mockProcessDetector implements ProcessDetector for testing.
type mockProcessDetector struct {
	activePaths map[string]bool // worktree paths that have a claude process
}

func (m *mockProcessDetector) IsClaudeRunning(worktreePath string) bool {
	return m.activePaths[worktreePath]
}

func TestReconcileSessions_IdleToActive_WhenClaudeRunning(t *testing.T) {
	dir := t.TempDir()
	session := &models.AgentSession{
		ID:           "sess-1",
		WorktreePath: dir,
		Status:       models.SessionStatusIdle,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{},
	}
	detector := &mockProcessDetector{activePaths: map[string]bool{dir: true}}

	cleaned := ReconcileSessions(context.Background(), ms, []*models.AgentSession{session}, WithProcessDetector(detector))
	assert.Equal(t, 1, cleaned)
	assert.Equal(t, models.SessionStatusActive, ms.sessions["sess-1"].Status)
	assert.NotNil(t, ms.sessions["sess-1"].LastActiveAt)
}

func TestReconcileSessions_ActiveToIdle_WhenClaudeNotRunning(t *testing.T) {
	dir := t.TempDir()
	session := &models.AgentSession{
		ID:           "sess-1",
		WorktreePath: dir,
		Status:       models.SessionStatusActive,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{},
	}
	detector := &mockProcessDetector{activePaths: map[string]bool{}} // no claude running

	cleaned := ReconcileSessions(context.Background(), ms, []*models.AgentSession{session}, WithProcessDetector(detector))
	assert.Equal(t, 1, cleaned)
	assert.Equal(t, models.SessionStatusIdle, ms.sessions["sess-1"].Status)
}

func TestReconcileSessions_ActiveStaysActive_WhenClaudeRunning(t *testing.T) {
	dir := t.TempDir()
	session := &models.AgentSession{
		ID:           "sess-1",
		WorktreePath: dir,
		Status:       models.SessionStatusActive,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{},
	}
	detector := &mockProcessDetector{activePaths: map[string]bool{dir: true}}

	cleaned := ReconcileSessions(context.Background(), ms, []*models.AgentSession{session}, WithProcessDetector(detector))
	assert.Equal(t, 0, cleaned)
	assert.Equal(t, models.SessionStatusActive, ms.sessions["sess-1"].Status)
}

func TestReconcileSessions_IdleStaysIdle_NoDetector(t *testing.T) {
	// Without a detector, existing behavior: idle stays idle
	dir := t.TempDir()
	session := &models.AgentSession{
		ID:           "sess-1",
		WorktreePath: dir,
		Status:       models.SessionStatusIdle,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{},
	}

	cleaned := ReconcileSessions(context.Background(), ms, []*models.AgentSession{session})
	assert.Equal(t, 0, cleaned)
	assert.Equal(t, models.SessionStatusIdle, ms.sessions["sess-1"].Status)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run "TestReconcileSessions_(IdleToActive|ActiveToIdle|ActiveStaysActive_WhenClaude|IdleStaysIdle_NoDetector)" -v -count=1`
Expected: FAIL — `ReconcileSessions` doesn't accept `WithProcessDetector` option yet

**Step 3: Create the ProcessDetector interface and real implementation**

Create `internal/agent/process.go`:

```go
package agent

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// ProcessDetector checks whether a Claude process is running in a directory.
type ProcessDetector interface {
	IsClaudeRunning(worktreePath string) bool
}

// OSProcessDetector detects Claude processes using pgrep + lsof (macOS/Linux).
type OSProcessDetector struct{}

// IsClaudeRunning returns true if a `claude` process has its cwd at or under worktreePath.
func (d *OSProcessDetector) IsClaudeRunning(worktreePath string) bool {
	absWT, err := filepath.Abs(worktreePath)
	if err != nil {
		return false
	}

	// Find claude PIDs
	out, err := exec.Command("pgrep", "-x", "claude").Output()
	if err != nil {
		return false // pgrep not found or no matches
	}

	pids := strings.Fields(strings.TrimSpace(string(out)))
	for _, pid := range pids {
		cwd := getCwd(pid)
		if cwd == "" {
			continue
		}
		absCwd, err := filepath.Abs(cwd)
		if err != nil {
			continue
		}
		if absCwd == absWT || strings.HasPrefix(absCwd, absWT+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// getCwd resolves the current working directory of a process via lsof.
func getCwd(pid string) string {
	out, err := exec.Command("lsof", "-a", "-p", pid, "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") && !strings.HasPrefix(line, "n ") {
			return line[1:]
		}
	}
	return ""
}
```

**Step 4: Update ReconcileSessions with functional options**

Replace `internal/agent/sync.go` entirely:

```go
package agent

import (
	"context"
	"os"
	"time"

	"github.com/joescharf/pm/internal/models"
)

// ReconcileOption configures ReconcileSessions behavior.
type ReconcileOption func(*reconcileConfig)

type reconcileConfig struct {
	processDetector ProcessDetector
}

// WithProcessDetector enables active/idle transitions based on claude process detection.
func WithProcessDetector(d ProcessDetector) ReconcileOption {
	return func(c *reconcileConfig) {
		c.processDetector = d
	}
}

// ReconcileSessions checks sessions and:
// 1. Marks active/idle sessions with missing worktree directories as abandoned.
// 2. Recovers abandoned sessions whose worktree still exists back to idle.
// 3. If a ProcessDetector is provided:
//   - Transitions idle → active when a claude process is detected in the worktree.
//   - Transitions active → idle when no claude process is detected.
//
// Returns the count of sessions updated.
func ReconcileSessions(ctx context.Context, s SessionStore, sessions []*models.AgentSession, opts ...ReconcileOption) int {
	cfg := &reconcileConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

	cleaned := 0
	for _, sess := range sessions {
		if sess.Status == models.SessionStatusCompleted {
			continue
		}
		if sess.WorktreePath == "" {
			continue
		}
		wtExists := true
		if _, err := os.Stat(sess.WorktreePath); err != nil {
			wtExists = false
		}

		switch {
		case !wtExists && (sess.Status == models.SessionStatusActive || sess.Status == models.SessionStatusIdle):
			// Worktree is gone — abandon the session
			if _, err := CloseSession(ctx, s, sess.ID, models.SessionStatusAbandoned); err == nil {
				cleaned++
			}
		case wtExists && sess.Status == models.SessionStatusAbandoned:
			// Worktree recovered/still exists — transition back to idle
			now := time.Now().UTC()
			sess.LastActiveAt = &now
			sess.Status = models.SessionStatusIdle
			sess.EndedAt = nil
			if err := s.UpdateAgentSession(ctx, sess); err == nil {
				cleaned++
			}
		case wtExists && cfg.processDetector != nil && sess.Status == models.SessionStatusIdle:
			// Idle + claude running → active
			if cfg.processDetector.IsClaudeRunning(sess.WorktreePath) {
				now := time.Now().UTC()
				sess.LastActiveAt = &now
				sess.Status = models.SessionStatusActive
				if err := s.UpdateAgentSession(ctx, sess); err == nil {
					cleaned++
				}
			}
		case wtExists && cfg.processDetector != nil && sess.Status == models.SessionStatusActive:
			// Active + no claude running → idle
			if !cfg.processDetector.IsClaudeRunning(sess.WorktreePath) {
				sess.Status = models.SessionStatusIdle
				if err := s.UpdateAgentSession(ctx, sess); err == nil {
					cleaned++
				}
			}
		}
	}
	return cleaned
}
```

**Step 5: Run tests to verify they pass**

Run: `go test ./internal/agent/ -v -count=1`
Expected: ALL PASS — including both new and existing tests (existing tests pass no options, so no detector, backward-compatible)

**Step 6: Commit**

```bash
git add internal/agent/process.go internal/agent/sync.go internal/agent/sync_test.go
git commit -m "feat: add claude process detection for automatic active/idle session transitions"
```

---

### Task 5: Wire ProcessDetector Into API and CLI Callers

**Files:**
- Modify: `internal/api/api.go` (where ReconcileSessions is called)
- Modify: `cmd/agent.go` (if ReconcileSessions is called there)

**Step 1: Find all ReconcileSessions call sites**

Search for `agent.ReconcileSessions` and add `agent.WithProcessDetector(&agent.OSProcessDetector{})` to each call.

**Step 2: Update API call site**

In `internal/api/api.go`, find the `ReconcileSessions` call and add the option:

```go
// Before:
if changed := agent.ReconcileSessions(r.Context(), s.store, allSessions); changed > 0 && statusFilter != "" {

// After:
detector := &agent.OSProcessDetector{}
if changed := agent.ReconcileSessions(r.Context(), s.store, allSessions, agent.WithProcessDetector(detector)); changed > 0 && statusFilter != "" {
```

Do the same for any other call sites (grep for `ReconcileSessions`).

**Step 3: Build to verify compilation**

Run: `go build .`
Expected: Success

**Step 4: Run full test suite**

Run: `go test ./... -count=1`
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/api/api.go cmd/agent.go
git commit -m "feat: wire process detector into API and CLI reconciliation"
```

---

### Task 6: Manual Verification

**Step 1: Rebuild and install**

Run: `make build && make install` (or `go install .`)

**Step 2: Check that the existing duplicates were cleaned up**

Run: `curl -s http://localhost:8080/api/v1/sessions | python3 -c "import sys,json; sessions=json.load(sys.stdin); [print(f'{s[\"ID\"][:12]} {s[\"Status\"]:10} {s[\"Branch\"]}') for s in sessions]"`

Expected: Only one active/idle session per branch. Duplicate should now be `abandoned`.

**Step 3: Verify active detection**

With a Claude session running in a worktree, hit the sessions API:

Run: `curl -s http://localhost:8080/api/v1/sessions | python3 -c "import sys,json; [print(f'{s[\"ID\"][:12]} {s[\"Status\"]}') for s in json.load(sys.stdin) if s['Status'] in ('active','idle')]"`

Expected: Sessions with a running Claude process show `active`; sessions without show `idle`.

**Step 4: Verify duplicate prevention**

Try launching the same issue twice via MCP:

Run: `pm agent launch pm --issue <issue_id>`
Run: `pm agent launch pm --issue <issue_id>` (again)

Expected: Second launch resumes the existing session, does NOT create a duplicate.

**Step 5: Final commit (if any manual fixes needed)**

```bash
git add -A
git commit -m "fix: manual verification adjustments"
```
