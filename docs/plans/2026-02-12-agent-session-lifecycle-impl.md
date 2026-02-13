# Agent Session Lifecycle Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add agent session close/resume lifecycle with worktree sync, across CLI, MCP, API, and web UI.

**Architecture:** New session statuses (active/idle/completed/abandoned) replace the old running/completed/failed/abandoned. A `GetAgentSession` store method enables lookups by ID or worktree path. Close cascades issue status changes. Lazy worktree sync reconciles orphaned sessions. The web UI gets close/resume actions on the sessions tab.

**Tech Stack:** Go (Cobra CLI, SQLite, MCP via mark3labs/mcp-go), React/TypeScript (TanStack Query, shadcn/ui)

---

### Task 1: Update Session Status Constants

**Files:**
- Modify: `internal/models/agent_session.go`

**Step 1: Update the status constants**

Replace the existing constants block:

```go
const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusIdle      SessionStatus = "idle"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusAbandoned SessionStatus = "abandoned"
)
```

Remove `SessionStatusRunning` and `SessionStatusFailed`.

**Step 2: Fix all references to old constants**

Search for `SessionStatusRunning` across the codebase and replace with `SessionStatusActive`. Search for `SessionStatusFailed` and replace with `SessionStatusAbandoned`. Files that will need updating:
- `cmd/agent.go` (lines 125, 159)
- `internal/mcp/server.go` (line 579)
- `internal/api/api.go` (line 528)
- `internal/store/sqlite_test.go` (line ~310)
- `internal/mcp/server_test.go` (any test using `SessionStatusRunning`)

**Step 3: Build to verify**

Run: `go build ./...`
Expected: Clean build, no errors.

**Step 4: Run tests**

Run: `go test ./... -count=1`
Expected: All tests pass (existing status values in DB may differ but tests create fresh DBs).

**Step 5: Commit**

```bash
git add internal/models/agent_session.go cmd/agent.go internal/mcp/server.go internal/api/api.go internal/store/sqlite_test.go internal/mcp/server_test.go
git commit -m "refactor: rename session statuses to active/idle/completed/abandoned"
```

---

### Task 2: Add SQL Migration for Status Rename

**Files:**
- Create: `internal/store/migrations/006_rename_session_statuses.sql`

**Step 1: Create the migration file**

```sql
-- Rename running -> active, failed -> abandoned in existing rows.
UPDATE agent_sessions SET status = 'active' WHERE status = 'running';
UPDATE agent_sessions SET status = 'abandoned' WHERE status = 'failed';
```

**Step 2: Run tests**

Run: `go test ./internal/store/ -count=1 -v`
Expected: All store tests pass (they run Migrate on fresh DBs).

**Step 3: Commit**

```bash
git add internal/store/migrations/006_rename_session_statuses.sql
git commit -m "migrate: rename running/failed session statuses to active/abandoned"
```

---

### Task 3: Add `GetAgentSession` to Store Interface and SQLite Implementation

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/sqlite.go`
- Modify: `internal/store/sqlite_test.go`

**Step 1: Write the failing test**

Add to `internal/store/sqlite_test.go` after `TestAgentSessionCRUD`:

```go
func TestGetAgentSession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create a project first
	p := &models.Project{Name: "test-proj", Path: "/tmp/test-proj"}
	require.NoError(t, s.CreateProject(ctx, p))

	// Create a session
	session := &models.AgentSession{
		ProjectID:    p.ID,
		IssueID:      "issue-1",
		Branch:       "feature/test",
		WorktreePath: "/tmp/test-proj-feature-test",
		Status:       models.SessionStatusActive,
	}
	require.NoError(t, s.CreateAgentSession(ctx, session))

	// Get by ID
	got, err := s.GetAgentSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, got.ID)
	assert.Equal(t, models.SessionStatusActive, got.Status)
	assert.Equal(t, "/tmp/test-proj-feature-test", got.WorktreePath)

	// Not found
	_, err = s.GetAgentSession(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestGetAgentSessionByWorktreePath(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p := &models.Project{Name: "test-proj", Path: "/tmp/test-proj"}
	require.NoError(t, s.CreateProject(ctx, p))

	session := &models.AgentSession{
		ProjectID:    p.ID,
		Branch:       "feature/test",
		WorktreePath: "/tmp/test-proj-feature-test",
		Status:       models.SessionStatusActive,
	}
	require.NoError(t, s.CreateAgentSession(ctx, session))

	// Find by worktree path
	got, err := s.GetAgentSessionByWorktreePath(ctx, "/tmp/test-proj-feature-test")
	require.NoError(t, err)
	assert.Equal(t, session.ID, got.ID)

	// Not found
	_, err = s.GetAgentSessionByWorktreePath(ctx, "/nonexistent")
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestGetAgentSession -v`
Expected: FAIL — methods don't exist.

**Step 3: Add to Store interface**

In `internal/store/store.go`, add to the Agent Sessions section:

```go
	// Agent Sessions
	CreateAgentSession(ctx context.Context, session *models.AgentSession) error
	GetAgentSession(ctx context.Context, id string) (*models.AgentSession, error)
	GetAgentSessionByWorktreePath(ctx context.Context, path string) (*models.AgentSession, error)
	ListAgentSessions(ctx context.Context, projectID string, limit int) ([]*models.AgentSession, error)
	UpdateAgentSession(ctx context.Context, session *models.AgentSession) error
```

**Step 4: Implement in SQLite store**

Add to `internal/store/sqlite.go` after `CreateAgentSession`:

```go
func (s *SQLiteStore) GetAgentSession(ctx context.Context, id string) (*models.AgentSession, error) {
	session := &models.AgentSession{}
	var status string
	var endedAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, started_at, ended_at
		FROM agent_sessions WHERE id = ?`, id,
	).Scan(&session.ID, &session.ProjectID, &session.IssueID,
		&session.Branch, &session.WorktreePath, &status,
		&session.Outcome, &session.CommitCount, &session.StartedAt, &endedAt)
	if err != nil {
		return nil, fmt.Errorf("agent session not found: %s", id)
	}

	session.Status = models.SessionStatus(status)
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}
	return session, nil
}

func (s *SQLiteStore) GetAgentSessionByWorktreePath(ctx context.Context, path string) (*models.AgentSession, error) {
	session := &models.AgentSession{}
	var status string
	var endedAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, started_at, ended_at
		FROM agent_sessions WHERE worktree_path = ? AND status IN ('active', 'idle')
		ORDER BY started_at DESC LIMIT 1`, path,
	).Scan(&session.ID, &session.ProjectID, &session.IssueID,
		&session.Branch, &session.WorktreePath, &status,
		&session.Outcome, &session.CommitCount, &session.StartedAt, &endedAt)
	if err != nil {
		return nil, fmt.Errorf("no active/idle session for worktree: %s", path)
	}

	session.Status = models.SessionStatus(status)
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}
	return session, nil
}
```

**Step 5: Update mock store in MCP tests**

In `internal/mcp/server_test.go`, add to `mockStore`:

```go
func (m *mockStore) GetAgentSession(_ context.Context, id string) (*models.AgentSession, error) {
	for _, s := range m.sessions {
		if s.ID == id {
			return s, nil
		}
	}
	return nil, fmt.Errorf("agent session not found: %s", id)
}

func (m *mockStore) GetAgentSessionByWorktreePath(_ context.Context, path string) (*models.AgentSession, error) {
	for _, s := range m.sessions {
		if s.WorktreePath == path && (s.Status == models.SessionStatusActive || s.Status == models.SessionStatusIdle) {
			return s, nil
		}
	}
	return nil, fmt.Errorf("no active/idle session for worktree: %s", path)
}
```

**Step 6: Run tests to verify they pass**

Run: `go test ./internal/store/ -run TestGetAgentSession -v && go test ./... -count=1`
Expected: All pass.

**Step 7: Commit**

```bash
git add internal/store/store.go internal/store/sqlite.go internal/store/sqlite_test.go internal/mcp/server_test.go
git commit -m "feat: add GetAgentSession and GetAgentSessionByWorktreePath to store"
```

---

### Task 4: Add `CloseAgentSession` Helper with Issue Cascading

**Files:**
- Create: `internal/agent/lifecycle.go`
- Create: `internal/agent/lifecycle_test.go`

This is a shared helper used by CLI, MCP, and API to close a session with consistent issue status cascading.

**Step 1: Write the failing test**

Create `internal/agent/lifecycle_test.go`:

```go
package agent

import (
	"context"
	"testing"

	"github.com/joescharf/pm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSessionStore struct {
	sessions map[string]*models.AgentSession
	issues   map[string]*models.Issue
}

func (m *mockSessionStore) GetAgentSession(_ context.Context, id string) (*models.AgentSession, error) {
	if s, ok := m.sessions[id]; ok {
		return s, nil
	}
	return nil, fmt.Errorf("not found: %s", id)
}

func (m *mockSessionStore) UpdateAgentSession(_ context.Context, s *models.AgentSession) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *mockSessionStore) GetIssue(_ context.Context, id string) (*models.Issue, error) {
	if i, ok := m.issues[id]; ok {
		return i, nil
	}
	return nil, fmt.Errorf("not found: %s", id)
}

func (m *mockSessionStore) UpdateIssue(_ context.Context, i *models.Issue) error {
	m.issues[i.ID] = i
	return nil
}

func TestCloseSession_Idle(t *testing.T) {
	session := &models.AgentSession{
		ID:      "sess-1",
		IssueID: "issue-1",
		Status:  models.SessionStatusActive,
	}
	issue := &models.Issue{
		ID:     "issue-1",
		Status: models.IssueStatusInProgress,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{"issue-1": issue},
	}

	result, err := CloseSession(context.Background(), ms, "sess-1", models.SessionStatusIdle)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusIdle, result.Status)
	assert.Nil(t, result.EndedAt)
	// Issue should stay in_progress
	assert.Equal(t, models.IssueStatusInProgress, ms.issues["issue-1"].Status)
}

func TestCloseSession_Completed(t *testing.T) {
	session := &models.AgentSession{
		ID:      "sess-1",
		IssueID: "issue-1",
		Status:  models.SessionStatusActive,
	}
	issue := &models.Issue{
		ID:     "issue-1",
		Status: models.IssueStatusInProgress,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{"issue-1": issue},
	}

	result, err := CloseSession(context.Background(), ms, "sess-1", models.SessionStatusCompleted)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCompleted, result.Status)
	assert.NotNil(t, result.EndedAt)
	assert.Equal(t, models.IssueStatusDone, ms.issues["issue-1"].Status)
}

func TestCloseSession_Abandoned(t *testing.T) {
	session := &models.AgentSession{
		ID:      "sess-1",
		IssueID: "issue-1",
		Status:  models.SessionStatusActive,
	}
	issue := &models.Issue{
		ID:     "issue-1",
		Status: models.IssueStatusInProgress,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{"issue-1": issue},
	}

	result, err := CloseSession(context.Background(), ms, "sess-1", models.SessionStatusAbandoned)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusAbandoned, result.Status)
	assert.NotNil(t, result.EndedAt)
	assert.Equal(t, models.IssueStatusOpen, ms.issues["issue-1"].Status)
}

func TestCloseSession_NoIssue(t *testing.T) {
	session := &models.AgentSession{
		ID:      "sess-1",
		IssueID: "",
		Status:  models.SessionStatusActive,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{},
	}

	result, err := CloseSession(context.Background(), ms, "sess-1", models.SessionStatusCompleted)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCompleted, result.Status)
}

func TestCloseSession_AlreadyClosed(t *testing.T) {
	session := &models.AgentSession{
		ID:     "sess-1",
		Status: models.SessionStatusCompleted,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{},
	}

	_, err := CloseSession(context.Background(), ms, "sess-1", models.SessionStatusIdle)
	assert.Error(t, err)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/ -v`
Expected: FAIL — package doesn't exist.

**Step 3: Write the implementation**

Create `internal/agent/lifecycle.go`:

```go
package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/joescharf/pm/internal/models"
)

// SessionStore is the subset of store.Store needed for session lifecycle.
type SessionStore interface {
	GetAgentSession(ctx context.Context, id string) (*models.AgentSession, error)
	UpdateAgentSession(ctx context.Context, session *models.AgentSession) error
	GetIssue(ctx context.Context, id string) (*models.Issue, error)
	UpdateIssue(ctx context.Context, issue *models.Issue) error
}

// CloseSession transitions a session to the given status and cascades issue changes.
// Valid target statuses: idle, completed, abandoned.
// Only active or idle sessions can be closed.
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
		if err == nil && issue.Status == models.IssueStatusInProgress {
			switch target {
			case models.SessionStatusCompleted:
				issue.Status = models.IssueStatusDone
				_ = s.UpdateIssue(ctx, issue)
			case models.SessionStatusAbandoned:
				issue.Status = models.IssueStatusOpen
				_ = s.UpdateIssue(ctx, issue)
			}
		}
	}

	return session, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ -v`
Expected: All 5 tests pass.

**Step 5: Commit**

```bash
git add internal/agent/lifecycle.go internal/agent/lifecycle_test.go
git commit -m "feat: add CloseSession helper with issue status cascading"
```

---

### Task 5: Add `pm agent close` CLI Command

**Files:**
- Modify: `cmd/agent.go`

**Step 1: Add the command and flags**

In the `var` block at the top, add:

```go
var (
	closeDone    bool
	closeAbandon bool
)
```

In `init()`, add:

```go
	agentCloseCmd.Flags().BoolVar(&closeDone, "done", false, "Mark session as completed (issues → done)")
	agentCloseCmd.Flags().BoolVar(&closeAbandon, "abandon", false, "Mark session as abandoned (issues → open)")
	agentCmd.AddCommand(agentCloseCmd)
```

Add the command definition:

```go
var agentCloseCmd = &cobra.Command{
	Use:   "close [session_id]",
	Short: "Close an agent session",
	Long: `Close an agent session. Default transitions to idle (worktree preserved).
Use --done to mark completed (issues → done) or --abandon to abandon (issues → open).

When no session_id is given:
  - In a worktree directory: closes the session for that worktree
  - In a project directory: lists active/idle sessions to choose from`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var sessionRef string
		if len(args) > 0 {
			sessionRef = args[0]
		}
		return agentCloseRun(sessionRef)
	},
}
```

**Step 2: Implement `agentCloseRun`**

```go
func agentCloseRun(sessionRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	// Determine target status
	target := models.SessionStatusIdle
	if closeDone {
		target = models.SessionStatusCompleted
	} else if closeAbandon {
		target = models.SessionStatusAbandoned
	}

	// Resolve session ID
	sessionID := sessionRef
	if sessionID == "" {
		sessionID, err = resolveSessionFromCwd(ctx, s)
		if err != nil {
			return err
		}
	}

	// Use the shared lifecycle helper
	session, err := agent.CloseSession(ctx, s, sessionID, target)
	if err != nil {
		return err
	}

	ui.Success("Session %s → %s", output.Cyan(shortID(session.ID)), output.Cyan(string(session.Status)))
	return nil
}
```

**Step 3: Implement `resolveSessionFromCwd`**

```go
func resolveSessionFromCwd(ctx context.Context, s store.Store) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	// Try matching cwd as a worktree path
	session, err := s.GetAgentSessionByWorktreePath(ctx, cwd)
	if err == nil {
		return session.ID, nil
	}

	// Try matching cwd as a project directory
	p, err := s.GetProjectByPath(ctx, cwd)
	if err != nil {
		return "", fmt.Errorf("no session found for current directory; specify a session ID")
	}

	// List active/idle sessions for this project
	sessions, err := s.ListAgentSessions(ctx, p.ID, 0)
	if err != nil {
		return "", err
	}

	var live []*models.AgentSession
	for _, sess := range sessions {
		if sess.Status == models.SessionStatusActive || sess.Status == models.SessionStatusIdle {
			live = append(live, sess)
		}
	}

	if len(live) == 0 {
		return "", fmt.Errorf("no active/idle sessions for project %s", p.Name)
	}
	if len(live) == 1 {
		return live[0].ID, nil
	}

	// Multiple sessions — list them for the user
	fmt.Println("Multiple active sessions. Specify a session ID:")
	table := ui.Table([]string{"ID", "Branch", "Status", "Started"})
	for _, sess := range live {
		table.Append([]string{
			shortID(sess.ID),
			sess.Branch,
			string(sess.Status),
			timeAgo(sess.StartedAt),
		})
	}
	table.Render()
	return "", fmt.Errorf("ambiguous: multiple sessions found")
}
```

Add `"os"` and the agent package to imports:
```go
import (
	// ... existing imports ...
	"os"

	"github.com/joescharf/pm/internal/agent"
	"github.com/joescharf/pm/internal/store"
)
```

**Step 4: Build and test**

Run: `go build ./... && go test ./cmd/ -count=1 -v`
Expected: Clean build, all tests pass.

**Step 5: Commit**

```bash
git add cmd/agent.go
git commit -m "feat: add pm agent close command with cwd detection"
```

---

### Task 6: Enhance `pm agent launch` to Resume Idle Sessions

**Files:**
- Modify: `cmd/agent.go` (the `agentLaunchRun` function)
- Modify: `internal/mcp/server.go` (the `handleLaunchAgent` handler)
- Modify: `internal/api/api.go` (the `launchAgent` handler)

**Step 1: Update CLI launch**

In `agentLaunchRun`, after computing `worktreePath` and before the `dryRun` check, add idle session detection:

```go
	// Check for existing idle session on this branch
	existingSessions, _ := s.ListAgentSessions(ctx, p.ID, 0)
	for _, sess := range existingSessions {
		if sess.Branch == branch && sess.Status == models.SessionStatusIdle {
			// Resume: reactivate existing session, skip worktree creation
			sess.Status = models.SessionStatusActive
			if err := s.UpdateAgentSession(ctx, sess); err != nil {
				ui.Warning("Failed to reactivate session: %v", err)
			} else {
				ui.Success("Resumed session %s for %s on branch %s", output.Cyan(shortID(sess.ID)), output.Cyan(p.Name), output.Cyan(branch))
				if resolvedIssueID != "" {
					ui.Info("Run: cd %s && claude \"Use pm MCP tools to look up issue %s and implement it. Update the issue status when complete.\"", worktreePath, shortID(resolvedIssueID))
				} else {
					ui.Info("Run: cd %s && claude", worktreePath)
				}
				return nil
			}
		}
	}
```

**Step 2: Update MCP launch**

In `handleLaunchAgent` in `internal/mcp/server.go`, after computing `worktreePath` and before `// Create worktree`, add:

```go
	// Check for existing idle session on this branch
	existingSessions, _ := s.store.ListAgentSessions(ctx, p.ID, 0)
	for _, sess := range existingSessions {
		if sess.Branch == branch && sess.Status == models.SessionStatusIdle {
			sess.Status = models.SessionStatusActive
			if err := s.store.UpdateAgentSession(ctx, sess); err == nil {
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
	}
```

**Step 3: Update API launch**

In `launchAgent` in `internal/api/api.go`, after computing `worktreePath` and before `// Create worktree`, add:

```go
	// Check for existing idle session on this branch
	existingSessions, _ := s.store.ListAgentSessions(ctx, project.ID, 0)
	for _, sess := range existingSessions {
		if sess.Branch == branch && sess.Status == models.SessionStatusIdle {
			sess.Status = models.SessionStatusActive
			if err := s.store.UpdateAgentSession(ctx, sess); err == nil {
				var issueRefs []string
				for _, issue := range issues {
					id := issue.ID
					if len(id) > 12 {
						id = id[:12]
					}
					issueRefs = append(issueRefs, id)
				}
				prompt := fmt.Sprintf("Use pm MCP tools to look up issue(s) %s and implement them. Update issue status when complete.", strings.Join(issueRefs, ", "))
				command := fmt.Sprintf(`cd %s && claude "%s"`, sess.WorktreePath, prompt)
				writeJSON(w, http.StatusOK, LaunchAgentResponse{
					SessionID:    sess.ID,
					Branch:       branch,
					WorktreePath: sess.WorktreePath,
					Command:      command,
				})
				return
			}
		}
	}
```

**Step 4: Build and test**

Run: `go build ./... && go test ./... -count=1`
Expected: All pass.

**Step 5: Commit**

```bash
git add cmd/agent.go internal/mcp/server.go internal/api/api.go
git commit -m "feat: resume idle sessions on agent launch instead of creating new worktree"
```

---

### Task 7: Add `pm_close_agent` MCP Tool

**Files:**
- Modify: `internal/mcp/server.go`
- Modify: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

Add to `internal/mcp/server_test.go`:

```go
func TestCloseAgentTool_Idle(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	seedProject(t, ms, "myapp", "/tmp/myapp")

	// Create a running session
	ms.sessions = append(ms.sessions, &models.AgentSession{
		ID:        "sess-123",
		ProjectID: ms.projects[0].ID,
		IssueID:   "",
		Branch:    "feature/test",
		Status:    models.SessionStatusActive,
	})

	result, err := srv.MCPServer().CallTool(context.Background(),
		callToolReq("pm_close_agent", map[string]any{"session_id": "sess-123"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	assert.Contains(t, text, `"status":"idle"`)
}

func TestCloseAgentTool_Completed(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	issue := seedIssue(t, ms, p.ID, "Fix bug", models.IssueStatusInProgress)

	ms.sessions = append(ms.sessions, &models.AgentSession{
		ID:        "sess-123",
		ProjectID: p.ID,
		IssueID:   issue.ID,
		Branch:    "feature/fix-bug",
		Status:    models.SessionStatusActive,
	})

	result, err := srv.MCPServer().CallTool(context.Background(),
		callToolReq("pm_close_agent", map[string]any{"session_id": "sess-123", "status": "completed"}))
	require.NoError(t, err)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	assert.Contains(t, text, `"status":"completed"`)
	// Verify issue was cascaded to done
	assert.Equal(t, models.IssueStatusDone, ms.issues[0].Status)
}

func TestCloseAgentTool_MissingSessionID(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)

	result, err := srv.MCPServer().CallTool(context.Background(),
		callToolReq("pm_close_agent", map[string]any{}))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/mcp/ -run TestCloseAgent -v`
Expected: FAIL — tool not registered.

**Step 3: Implement the tool**

In `internal/mcp/server.go`, add tool registration in `MCPServer()`:

```go
	srv.AddTool(s.closeAgentTool())
```

Add the tool definition and handler:

```go
func (s *Server) closeAgentTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_close_agent",
		mcp.WithDescription("Close an agent session. Default transitions to idle. Use status=completed to mark done (issues → done) or status=abandoned to abandon (issues → open)."),
		mcp.WithString("session_id", mcp.Required(), mcp.Description("Session ID to close")),
		mcp.WithString("status", mcp.Description("Target status: idle (default), completed, abandoned")),
	)
	return tool, s.handleCloseAgent
}

func (s *Server) handleCloseAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: session_id"), nil
	}

	targetStr := request.GetString("status", "idle")
	target := models.SessionStatus(targetStr)

	// Validate target status
	switch target {
	case models.SessionStatusIdle, models.SessionStatusCompleted, models.SessionStatusAbandoned:
		// valid
	default:
		return mcp.NewToolResultError(fmt.Sprintf("invalid status: %s (must be idle, completed, or abandoned)", targetStr)), nil
	}

	session, err := agent.CloseSession(ctx, s.store, sessionID, target)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	result := map[string]any{
		"session_id": session.ID,
		"status":     string(session.Status),
	}
	if session.EndedAt != nil {
		result["ended_at"] = session.EndedAt.Format(time.RFC3339)
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}
```

Add import: `"github.com/joescharf/pm/internal/agent"`

**Step 4: Update the mock store's `UpdateAgentSession` to actually persist**

In `internal/mcp/server_test.go`, replace the one-liner:

```go
func (m *mockStore) UpdateAgentSession(_ context.Context, session *models.AgentSession) error {
	for idx, s := range m.sessions {
		if s.ID == session.ID {
			m.sessions[idx] = session
			return nil
		}
	}
	return fmt.Errorf("session not found: %s", session.ID)
}
```

**Step 5: Run tests**

Run: `go test ./internal/mcp/ -v -count=1`
Expected: All pass, including the integration test (should now find 8 tools).

**Step 6: Update integration test tool count**

In `TestMCPIntegration_ListTools`, update the expected tool count from 7 to 8.

**Step 7: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat: add pm_close_agent MCP tool"
```

---

### Task 8: Add `POST /api/v1/agent/close` API Endpoint

**Files:**
- Modify: `internal/api/api.go`
- Modify: `internal/api/api_test.go`

**Step 1: Write the failing test**

Add to `internal/api/api_test.go`:

```go
func TestCloseAgent(t *testing.T) {
	srv, s := setupTestServer(t)
	ctx := context.Background()

	// Create project and issue
	p := &models.Project{Name: "test-close", Path: "/tmp/test-close"}
	require.NoError(t, s.CreateProject(ctx, p))
	issue := &models.Issue{ProjectID: p.ID, Title: "Test issue", Status: models.IssueStatusInProgress, Priority: models.IssuePriorityMedium, Type: models.IssueTypeFeature}
	require.NoError(t, s.CreateIssue(ctx, issue))

	// Create a session
	session := &models.AgentSession{
		ProjectID:    p.ID,
		IssueID:      issue.ID,
		Branch:       "feature/test",
		WorktreePath: "/tmp/test-close-feature-test",
		Status:       models.SessionStatusActive,
	}
	require.NoError(t, s.CreateAgentSession(ctx, session))

	// Close as completed
	body := fmt.Sprintf(`{"session_id":"%s","status":"completed"}`, session.ID)
	req := httptest.NewRequest("POST", "/api/v1/agent/close", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify issue was cascaded
	updatedIssue, _ := s.GetIssue(ctx, issue.ID)
	assert.Equal(t, models.IssueStatusDone, updatedIssue.Status)
}
```

**Step 2: Run to verify it fails**

Run: `go test ./internal/api/ -run TestCloseAgent -v`
Expected: FAIL — 404, route doesn't exist.

**Step 3: Implement**

Add route in `Router()`:

```go
	mux.HandleFunc("POST /api/v1/agent/close", s.closeAgent)
```

Add types and handler:

```go
// CloseAgentRequest is the JSON body for POST /api/v1/agent/close.
type CloseAgentRequest struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"` // idle, completed, abandoned
}

// CloseAgentResponse is the JSON response for closing an agent session.
type CloseAgentResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	EndedAt   string `json:"ended_at,omitempty"`
}

func (s *Server) closeAgent(w http.ResponseWriter, r *http.Request) {
	var req CloseAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	target := models.SessionStatusIdle
	if req.Status != "" {
		target = models.SessionStatus(req.Status)
	}

	switch target {
	case models.SessionStatusIdle, models.SessionStatusCompleted, models.SessionStatusAbandoned:
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid status: %s", req.Status))
		return
	}

	session, err := agent.CloseSession(r.Context(), s.store, req.SessionID, target)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := CloseAgentResponse{
		SessionID: session.ID,
		Status:    string(session.Status),
	}
	if session.EndedAt != nil {
		resp.EndedAt = session.EndedAt.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}
```

Add import: `"github.com/joescharf/pm/internal/agent"`

**Step 4: Run tests**

Run: `go test ./internal/api/ -v -count=1`
Expected: All pass.

**Step 5: Commit**

```bash
git add internal/api/api.go internal/api/api_test.go
git commit -m "feat: add POST /api/v1/agent/close endpoint"
```

---

### Task 9: Worktree Sync (Lazy Reconciliation)

**Files:**
- Create: `internal/agent/sync.go`
- Create: `internal/agent/sync_test.go`
- Modify: `cmd/agent.go` (call sync before listing)
- Modify: `internal/api/api.go` (call sync before listing sessions)

**Step 1: Write the failing test**

Create `internal/agent/sync_test.go`:

```go
package agent

import (
	"context"
	"testing"

	"github.com/joescharf/pm/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestReconcileSessions_OrphanedWorktree(t *testing.T) {
	session := &models.AgentSession{
		ID:           "sess-1",
		IssueID:      "issue-1",
		WorktreePath: "/nonexistent/path",
		Status:       models.SessionStatusActive,
	}
	issue := &models.Issue{
		ID:     "issue-1",
		Status: models.IssueStatusInProgress,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{"issue-1": issue},
	}

	cleaned := ReconcileSessions(context.Background(), ms, []*models.AgentSession{session})
	assert.Equal(t, 1, cleaned)
	assert.Equal(t, models.SessionStatusAbandoned, ms.sessions["sess-1"].Status)
	assert.Equal(t, models.IssueStatusOpen, ms.issues["issue-1"].Status)
}

func TestReconcileSessions_ExistingWorktree(t *testing.T) {
	// Use a path that exists (the temp dir itself)
	session := &models.AgentSession{
		ID:           "sess-1",
		WorktreePath: t.TempDir(),
		Status:       models.SessionStatusActive,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{},
	}

	cleaned := ReconcileSessions(context.Background(), ms, []*models.AgentSession{session})
	assert.Equal(t, 0, cleaned)
	assert.Equal(t, models.SessionStatusActive, ms.sessions["sess-1"].Status)
}

func TestReconcileSessions_SkipsTerminal(t *testing.T) {
	session := &models.AgentSession{
		ID:           "sess-1",
		WorktreePath: "/nonexistent",
		Status:       models.SessionStatusCompleted,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{},
	}

	cleaned := ReconcileSessions(context.Background(), ms, []*models.AgentSession{session})
	assert.Equal(t, 0, cleaned)
	assert.Equal(t, models.SessionStatusCompleted, ms.sessions["sess-1"].Status)
}
```

**Step 2: Run to verify it fails**

Run: `go test ./internal/agent/ -run TestReconcile -v`
Expected: FAIL — function doesn't exist.

**Step 3: Implement**

Create `internal/agent/sync.go`:

```go
package agent

import (
	"context"
	"os"

	"github.com/joescharf/pm/internal/models"
)

// ReconcileSessions checks active/idle sessions and marks any with missing
// worktree directories as abandoned. Returns the count of sessions cleaned up.
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
	}
	return cleaned
}
```

**Step 4: Run tests**

Run: `go test ./internal/agent/ -v -count=1`
Expected: All pass.

**Step 5: Wire into CLI agent list**

In `cmd/agent.go`, in `agentListRun`, after fetching sessions and before filtering to active, add:

```go
	// Reconcile orphaned worktrees
	agent.ReconcileSessions(ctx, s, sessions)
```

Add import: `"github.com/joescharf/pm/internal/agent"`

**Step 6: Wire into API listSessions**

In `internal/api/api.go`, in `listSessions`, after fetching sessions and before `writeJSON`:

```go
	agent.ReconcileSessions(r.Context(), s.store, sessions)
```

Add import: `"github.com/joescharf/pm/internal/agent"`

**Step 7: Build and test**

Run: `go build ./... && go test ./... -count=1`
Expected: All pass.

**Step 8: Commit**

```bash
git add internal/agent/sync.go internal/agent/sync_test.go cmd/agent.go internal/api/api.go
git commit -m "feat: add lazy worktree sync to reconcile orphaned sessions"
```

---

### Task 10: Update CLI `agent list` to Show New Statuses

**Files:**
- Modify: `cmd/agent.go`

**Step 1: Update the filter in `agentListRun`**

Replace the filter that only shows `running` sessions to show `active` and `idle`:

```go
	var live []*models.AgentSession
	for _, sess := range sessions {
		if sess.Status == models.SessionStatusActive || sess.Status == models.SessionStatusIdle {
			live = append(live, sess)
		}
	}
```

Update the table to include the status column and worktree path:

```go
	table := ui.Table([]string{"ID", "Project", "Branch", "Status", "Worktree", "Started"})
	for _, sess := range live {
		// ... existing project name lookup ...
		table.Append([]string{
			shortID(sess.ID),
			projName,
			sess.Branch,
			output.StatusColor(string(sess.Status)),
			sess.WorktreePath,
			timeAgo(sess.StartedAt),
		})
	}
```

**Step 2: Build and test**

Run: `go build ./... && go test ./cmd/ -count=1`
Expected: All pass.

**Step 3: Commit**

```bash
git add cmd/agent.go
git commit -m "feat: update agent list to show active/idle statuses with worktree paths"
```

---

### Task 11: Update Web UI Types and Hooks

**Files:**
- Modify: `ui/src/lib/types.ts`
- Modify: `ui/src/hooks/use-agent.ts`

**Step 1: Update types**

In `ui/src/lib/types.ts`, update `SessionStatus`:

```typescript
export type SessionStatus = "active" | "idle" | "completed" | "abandoned";
```

Add close types:

```typescript
export interface CloseAgentRequest {
  session_id: string;
  status?: "idle" | "completed" | "abandoned";
}

export interface CloseAgentResponse {
  session_id: string;
  status: string;
  ended_at?: string;
}
```

**Step 2: Add close hook**

In `ui/src/hooks/use-agent.ts`, add:

```typescript
interface CloseAgentRequest {
  session_id: string;
  status?: "idle" | "completed" | "abandoned";
}

interface CloseAgentResponse {
  session_id: string;
  status: string;
  ended_at?: string;
}

export function useCloseAgent() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: CloseAgentRequest) =>
      apiFetch<CloseAgentResponse>("/api/v1/agent/close", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
      qc.invalidateQueries({ queryKey: ["issues"] });
    },
  });
}
```

**Step 3: Commit**

```bash
git add ui/src/lib/types.ts ui/src/hooks/use-agent.ts
git commit -m "feat(ui): add close agent types and hook"
```

---

### Task 12: Update Sessions Page with New Statuses and Actions

**Files:**
- Modify: `ui/src/components/sessions/sessions-page.tsx`

**Step 1: Update `sessionColor` function**

```typescript
function sessionColor(status: SessionStatus): string {
  switch (status) {
    case "active":
      return "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300";
    case "idle":
      return "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300";
    case "completed":
      return "bg-emerald-100 text-emerald-800 dark:bg-emerald-900/40 dark:text-emerald-300";
    case "abandoned":
      return "bg-gray-100 text-gray-800 dark:bg-gray-900/40 dark:text-gray-300";
    default:
      return "";
  }
}
```

**Step 2: Add close/done/abandon action buttons**

Add imports for `useCloseAgent` and action buttons. Add an Actions column to the table with buttons:
- For `active` sessions: Close (→ idle), Done, Abandon
- For `idle` sessions: Done, Abandon
- For terminal statuses: no actions

```typescript
import { useCloseAgent } from "@/hooks/use-agent";
import { Button } from "@/components/ui/button";
import { toast } from "sonner";

// Inside the component:
const closeAgent = useCloseAgent();

const handleClose = (sessionId: string, status: "idle" | "completed" | "abandoned") => {
  closeAgent.mutate(
    { session_id: sessionId, status },
    {
      onSuccess: () => toast.success(`Session ${status}`),
      onError: (err) => toast.error(`Failed: ${(err as Error).message}`),
    }
  );
};
```

Add Actions column to the table header and body with appropriate buttons.

**Step 3: Build UI**

Run: `cd /Users/joescharf/app/pm/ui && bun run build`
Expected: Clean build.

**Step 4: Commit**

```bash
git add ui/src/components/sessions/sessions-page.tsx
git commit -m "feat(ui): update sessions page with new statuses and close actions"
```

---

### Task 13: Add Worktrees Section to Project Detail Page

**Files:**
- Modify: `ui/src/components/projects/project-detail.tsx`

**Step 1: Add a Worktrees tab**

Add a third tab "Worktrees" alongside Issues and Sessions. Filter sessions to only `active`/`idle` for the worktrees tab:

```typescript
const worktrees = sessions.filter(
  (s) => s.Status === "active" || s.Status === "idle"
);
```

Add a `TabsTrigger` for worktrees:
```tsx
<TabsTrigger value="worktrees">
  Worktrees{worktrees.length > 0 && ` (${worktrees.length})`}
</TabsTrigger>
```

Add the `TabsContent` with a table showing Branch, Issue ID, Status, Started, and Actions (Close/Done/Abandon/Resume buttons).

Import and use `useCloseAgent` for the action buttons. For Resume, show a copy-command dialog reusing the pattern from `AgentLaunchDialog`.

**Step 2: Build UI**

Run: `cd /Users/joescharf/app/pm/ui && bun run build`
Expected: Clean build.

**Step 3: Commit**

```bash
git add ui/src/components/projects/project-detail.tsx
git commit -m "feat(ui): add worktrees tab to project detail with close/resume actions"
```

---

### Task 14: Embed Updated UI and Final Build

**Files:**
- Run: `make ui-embed`

**Step 1: Build and embed the UI**

Run: `cd /Users/joescharf/app/pm && make ui-build && make ui-embed`

**Step 2: Full build and test**

Run: `make build && make test`
Expected: All pass.

**Step 3: Update CLAUDE.md**

Update the MCP Tools table in `/Users/joescharf/app/pm/CLAUDE.md` to add `pm_close_agent` and update the CLI Commands section to include `pm agent close`.

**Step 4: Commit**

```bash
git add internal/ui/dist/ CLAUDE.md
git commit -m "build: embed updated UI, update docs for agent close"
```

---

### Task 15: Manual Smoke Test

Run through the full lifecycle manually:

1. `pm agent launch <project> --issue <id>` — verify session created as `active`
2. `pm agent list` — verify session shows as `active`
3. `pm agent close` (from worktree dir) — verify session moves to `idle`
4. `pm agent launch <project> --branch <same-branch>` — verify session resumes to `active`
5. `pm agent close --done` — verify session moves to `completed`, issue moves to `done`
6. Test abandon: launch new session, `pm agent close --abandon` — verify issue returns to `open`
7. Test worktree sync: delete a worktree dir manually, run `pm agent list` — verify session auto-abandoned
8. Test web UI: open dashboard, verify worktrees tab, click Done/Abandon buttons
