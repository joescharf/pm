package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/store"
	"github.com/joescharf/pm/internal/wt"
	"github.com/joescharf/wt/pkg/gitops"
	"github.com/joescharf/wt/pkg/lifecycle"
)

// ===========================================================================
// testWTClient — real git worktree operations, no iTerm/trust/state
// ===========================================================================

// testWTClient implements wt.Client using real git commands but skips iTerm
// window management, Claude trust, and wt state — making it safe for tests.
type testWTClient struct {
	createCalls []struct{ repo, branch string }
}

func (c *testWTClient) Create(repoPath, branch string) error {
	c.createCalls = append(c.createCalls, struct{ repo, branch string }{repoPath, branch})

	wtDir := repoPath + ".worktrees"
	if err := os.MkdirAll(wtDir, 0o755); err != nil {
		return err
	}

	// Derive directory name from branch (last segment after /)
	parts := strings.Split(branch, "/")
	dirname := parts[len(parts)-1]
	wtPath := filepath.Join(wtDir, dirname)

	// Check if worktree already exists (resume case)
	if _, err := os.Stat(wtPath); err == nil {
		return nil
	}

	// Create a real git worktree
	out, err := exec.Command("git", "-C", repoPath, "worktree", "add", "-b", branch, wtPath, "main").CombinedOutput()
	if err != nil {
		// Branch might already exist — try without -b
		out2, err2 := exec.Command("git", "-C", repoPath, "worktree", "add", wtPath, branch).CombinedOutput()
		if err2 != nil {
			return fmt.Errorf("git worktree add: %s / %s", strings.TrimSpace(string(out)), strings.TrimSpace(string(out2)))
		}
	}
	_ = out
	return nil
}

func (c *testWTClient) List(repoPath string) ([]wt.WorktreeInfo, error) {
	out, err := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %s", strings.TrimSpace(string(out)))
	}

	var result []wt.WorktreeInfo
	var current wt.WorktreeInfo
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "worktree "):
			if current.Path != "" {
				result = append(result, current)
			}
			current = wt.WorktreeInfo{Path: strings.TrimPrefix(line, "worktree "), Repo: repoPath}
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		}
	}
	if current.Path != "" {
		result = append(result, current)
	}
	return result, nil
}

func (c *testWTClient) Delete(repoPath, branch string) error {
	parts := strings.Split(branch, "/")
	dirname := parts[len(parts)-1]
	wtPath := filepath.Join(repoPath+".worktrees", dirname)
	out, err := exec.Command("git", "-C", repoPath, "worktree", "remove", "--force", wtPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *testWTClient) Lifecycle() *lifecycle.Manager { return nil }
func (c *testWTClient) LifecycleForRepo(repoPath string) *lifecycle.Manager {
	git := &testGitopsClient{repoPath: repoPath}
	return lifecycle.NewManager(git, nil, nil, nil, nil)
}

// testGitopsClient implements gitops.Client for a specific repo.
// Only methods used by lifecycle.Delete are implemented; others panic via the
// embedded nil interface, which is fine for tests.
type testGitopsClient struct {
	gitops.Client // unimplemented methods panic — acceptable in tests
	repoPath      string
}

func (c *testGitopsClient) WorktreeRemove(path string, force bool) error {
	args := []string{"-C", c.repoPath, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *testGitopsClient) BranchDelete(branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	out, err := exec.Command("git", "-C", c.repoPath, "branch", flag, branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git branch delete: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ===========================================================================
// Test helpers
// ===========================================================================

// initTestRepo creates a real git repository in a temp dir with an initial
// commit, so worktree operations work. Returns the repo path.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", "-b", "main", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "cmd %v: %s", args, string(out))
	}

	// Create initial file and commit so we have a main branch
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test\n"), 0o644))
	out, err := exec.Command("git", "-C", dir, "add", ".").CombinedOutput()
	require.NoError(t, err, "git add: %s", string(out))
	out, err = exec.Command("git", "-C", dir, "commit", "-m", "initial commit").CombinedOutput()
	require.NoError(t, err, "git commit: %s", string(out))

	// Resolve symlinks (macOS: /var -> /private/var) so paths match git output
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return resolved
}

// gitCommitFile creates a file and commits it in the given repo/worktree path.
func gitCommitFile(t *testing.T, path, filename, content, message string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(path, filename), []byte(content), 0o644))
	out, err := exec.Command("git", "-C", path, "add", filename).CombinedOutput()
	require.NoError(t, err, "git add: %s", string(out))
	out, err = exec.Command("git", "-C", path, "commit", "-m", message).CombinedOutput()
	require.NoError(t, err, "git commit: %s", string(out))
}

func setupE2EServer(t *testing.T) (*Server, store.Store, *testWTClient, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	require.NoError(t, s.Migrate(context.Background()))
	t.Cleanup(func() { _ = s.Close() })

	gc := git.NewClient()
	ghc := git.NewGitHubClient()
	wtc := &testWTClient{}

	repoPath := initTestRepo(t)
	srv := NewServer(s, gc, ghc, wtc, nil)
	// Disable process detection in tests — no real claude processes are running,
	// so OSProcessDetector would incorrectly transition active → idle.
	srv.processDetector = nil

	return srv, s, wtc, repoPath
}

// doJSON is a helper: make a JSON request and return the recorder.
func doJSON(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// decodeJSON is a helper: unmarshal response body.
func decodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &v), "body: %s", w.Body.String())
	return v
}

// ---------------------------------------------------------------------------
// Create fixtures
// ---------------------------------------------------------------------------

func createProject(t *testing.T, s store.Store, name, path string) *models.Project {
	t.Helper()
	p := &models.Project{Name: name, Path: path}
	require.NoError(t, s.CreateProject(context.Background(), p))
	return p
}

func createIssue(t *testing.T, s store.Store, projectID, title string) *models.Issue {
	t.Helper()
	issue := &models.Issue{
		ProjectID: projectID,
		Title:     title,
		Status:    models.IssueStatusOpen,
		Priority:  models.IssuePriorityMedium,
		Type:      models.IssueTypeFeature,
	}
	require.NoError(t, s.CreateIssue(context.Background(), issue))
	return issue
}

func createSession(t *testing.T, s store.Store, projectID, issueID, branch, wtPath string, status models.SessionStatus) *models.AgentSession {
	t.Helper()
	sess := &models.AgentSession{
		ProjectID:     projectID,
		IssueID:       issueID,
		Branch:        branch,
		WorktreePath:  wtPath,
		Status:        status,
		ConflictState: models.ConflictStateNone,
		ConflictFiles: "[]",
	}
	require.NoError(t, s.CreateAgentSession(context.Background(), sess))
	return sess
}

// ===========================================================================
// E2E Tests — real git repos, real worktrees, real SQLite
// ===========================================================================

// TestSessionLifecycle_E2E walks through the full session lifecycle against
// a real git repo:
//
//  1. Launch agent — creates real worktree + session, marks issue in_progress
//  2. List sessions (active filter) — session appears
//  3. Close agent as idle — session pauses, issue stays in_progress
//  4. Resume agent — session re-activates
//  5. Close agent as completed — session ends, issue cascades to done
//  6. Reactivate — session back to idle, issue to in_progress
//  7. Close agent as abandoned — session ends, issue cascades to open
//  8. Cleanup stale — abandoned 0-commit session is purged
func TestSessionLifecycle_E2E(t *testing.T) {
	srv, s, wtc, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "lifecycle-test", repoPath)
	issue := createIssue(t, s, proj.ID, "Add user login")

	// -----------------------------------------------------------------------
	// Step 1: Launch agent — creates real git worktree
	// -----------------------------------------------------------------------
	w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
		"project_id": proj.ID,
		"issue_ids":  []string{issue.ID},
	})
	require.Equal(t, http.StatusOK, w.Code, "launch body: %s", w.Body.String())
	launchResp := decodeJSON[LaunchAgentResponse](t, w)

	assert.NotEmpty(t, launchResp.SessionID)
	assert.Equal(t, "feature/add-user-login", launchResp.Branch)
	assert.Contains(t, launchResp.WorktreePath, ".worktrees/")
	assert.Contains(t, launchResp.Command, "claude")

	// Verify wt.Create was called with correct args
	require.Len(t, wtc.createCalls, 1)
	assert.Equal(t, repoPath, wtc.createCalls[0].repo)
	assert.Equal(t, "feature/add-user-login", wtc.createCalls[0].branch)

	// Verify the worktree directory actually exists on disk
	assert.DirExists(t, launchResp.WorktreePath)

	// Verify a real git branch was created
	out, err := exec.Command("git", "-C", repoPath, "branch", "--list", "feature/add-user-login").CombinedOutput()
	require.NoError(t, err)
	assert.Contains(t, string(out), "feature/add-user-login")

	// Verify issue cascaded to in_progress
	updIssue, err := s.GetIssue(ctx, issue.ID)
	require.NoError(t, err)
	assert.Equal(t, models.IssueStatusInProgress, updIssue.Status)

	// Verify session is active in the DB
	sess, err := s.GetAgentSession(ctx, launchResp.SessionID)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusActive, sess.Status)
	assert.Equal(t, issue.ID, sess.IssueID)
	assert.Nil(t, sess.EndedAt)

	sessionID := launchResp.SessionID

	// -----------------------------------------------------------------------
	// Step 2: List sessions — active filter should include our session
	// -----------------------------------------------------------------------
	w = doJSON(t, router, "GET", "/api/v1/sessions?status=active,idle", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var listed []sessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listed))
	require.Len(t, listed, 1)
	assert.Equal(t, sessionID, listed[0].ID)
	assert.Equal(t, "lifecycle-test", listed[0].ProjectName)

	// -----------------------------------------------------------------------
	// Step 3: Close agent as idle (pause)
	// -----------------------------------------------------------------------
	w = doJSON(t, router, "POST", "/api/v1/agent/close", map[string]any{
		"session_id": sessionID,
		"status":     "idle",
	})
	require.Equal(t, http.StatusOK, w.Code)
	closeResp := decodeJSON[CloseAgentResponse](t, w)
	assert.Equal(t, "idle", closeResp.Status)
	assert.Empty(t, closeResp.EndedAt, "idle should not set EndedAt")

	// Issue stays in_progress (idle doesn't cascade)
	updIssue, _ = s.GetIssue(ctx, issue.ID)
	assert.Equal(t, models.IssueStatusInProgress, updIssue.Status)

	// -----------------------------------------------------------------------
	// Step 4: Resume agent
	// -----------------------------------------------------------------------
	w = doJSON(t, router, "POST", "/api/v1/agent/resume", map[string]any{
		"session_id": sessionID,
	})
	require.Equal(t, http.StatusOK, w.Code, "resume body: %s", w.Body.String())
	resumeResp := decodeJSON[LaunchAgentResponse](t, w)
	assert.Equal(t, sessionID, resumeResp.SessionID)
	assert.Contains(t, resumeResp.Command, "claude")

	sess, _ = s.GetAgentSession(ctx, sessionID)
	assert.Equal(t, models.SessionStatusActive, sess.Status)

	// -----------------------------------------------------------------------
	// Step 5: Close agent as completed
	// -----------------------------------------------------------------------
	w = doJSON(t, router, "POST", "/api/v1/agent/close", map[string]any{
		"session_id": sessionID,
		"status":     "completed",
	})
	require.Equal(t, http.StatusOK, w.Code)
	closeResp = decodeJSON[CloseAgentResponse](t, w)
	assert.Equal(t, "completed", closeResp.Status)
	assert.NotEmpty(t, closeResp.EndedAt)

	// Issue cascades to done
	updIssue, _ = s.GetIssue(ctx, issue.ID)
	assert.Equal(t, models.IssueStatusDone, updIssue.Status)

	sess, _ = s.GetAgentSession(ctx, sessionID)
	assert.Equal(t, models.SessionStatusCompleted, sess.Status)
	assert.NotNil(t, sess.EndedAt)

	// -----------------------------------------------------------------------
	// Step 6: Reactivate session
	// -----------------------------------------------------------------------
	w = doJSON(t, router, "POST", fmt.Sprintf("/api/v1/sessions/%s/reactivate", sessionID), nil)
	require.Equal(t, http.StatusOK, w.Code, "reactivate body: %s", w.Body.String())

	var reactivateResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &reactivateResp))
	assert.Equal(t, "idle", reactivateResp["status"])

	sess, _ = s.GetAgentSession(ctx, sessionID)
	assert.Equal(t, models.SessionStatusIdle, sess.Status)
	assert.Nil(t, sess.EndedAt)

	// Issue cascades back to in_progress
	updIssue, _ = s.GetIssue(ctx, issue.ID)
	assert.Equal(t, models.IssueStatusInProgress, updIssue.Status)

	// -----------------------------------------------------------------------
	// Step 7: Close agent as abandoned
	// -----------------------------------------------------------------------
	w = doJSON(t, router, "POST", "/api/v1/agent/close", map[string]any{
		"session_id": sessionID,
		"status":     "abandoned",
	})
	require.Equal(t, http.StatusOK, w.Code)
	closeResp = decodeJSON[CloseAgentResponse](t, w)
	assert.Equal(t, "abandoned", closeResp.Status)
	assert.NotEmpty(t, closeResp.EndedAt)

	// Issue cascades back to open
	updIssue, _ = s.GetIssue(ctx, issue.ID)
	assert.Equal(t, models.IssueStatusOpen, updIssue.Status)

	// -----------------------------------------------------------------------
	// Step 8: Cleanup stale
	// -----------------------------------------------------------------------
	w = doJSON(t, router, "DELETE", "/api/v1/sessions/cleanup", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var cleanupResp map[string]int64
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cleanupResp))
	assert.Equal(t, int64(1), cleanupResp["deleted"])

	_, err = s.GetAgentSession(ctx, sessionID)
	assert.Error(t, err, "session should be deleted after cleanup")
}

// TestLaunchAgent_ResumesIdleSession verifies that launching an agent for
// the same issue/branch resumes the existing idle session instead of creating
// a duplicate.
func TestLaunchAgent_ResumesIdleSession(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "resume-test", repoPath)
	issue := createIssue(t, s, proj.ID, "Add user login")

	// Pre-create a real worktree and an idle session for it
	branch := "feature/add-user-login"
	wtDir := filepath.Join(repoPath+".worktrees", "add-user-login")
	out, err := exec.Command("git", "-C", repoPath, "worktree", "add", "-b", branch, wtDir, "main").CombinedOutput()
	require.NoError(t, err, "git worktree add: %s", string(out))

	existingSess := createSession(t, s, proj.ID, issue.ID, branch, wtDir, models.SessionStatusIdle)

	// Launch agent for same issue — should resume the idle session
	w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
		"project_id": proj.ID,
		"issue_ids":  []string{issue.ID},
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	resp := decodeJSON[LaunchAgentResponse](t, w)

	assert.Equal(t, existingSess.ID, resp.SessionID, "should resume existing session")

	sess, _ := s.GetAgentSession(ctx, existingSess.ID)
	assert.Equal(t, models.SessionStatusActive, sess.Status)

	// No duplicate sessions
	sessions, _ := s.ListAgentSessions(ctx, proj.ID, 50)
	assert.Len(t, sessions, 1)
}

// TestLaunchAgent_WorktreePathMatchesConvention verifies the bug fix: the
// worktree path stored in the session uses the .worktrees/<dirname> convention.
func TestLaunchAgent_WorktreePathMatchesConvention(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "path-test", repoPath)
	issue := createIssue(t, s, proj.ID, "Fix database migration")

	w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
		"project_id": proj.ID,
		"issue_ids":  []string{issue.ID},
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	resp := decodeJSON[LaunchAgentResponse](t, w)

	expected := filepath.Join(repoPath+".worktrees", "fix-database-migration")
	assert.Equal(t, expected, resp.WorktreePath)
	assert.DirExists(t, expected, "worktree should actually exist on disk")

	sess, _ := s.GetAgentSession(ctx, resp.SessionID)
	assert.Equal(t, expected, sess.WorktreePath)
}

// TestLaunchAgent_Validation tests error responses for bad requests.
func TestLaunchAgent_Validation(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()

	proj := createProject(t, s, "val-test", repoPath)
	issue := createIssue(t, s, proj.ID, "Foo")

	tests := []struct {
		name   string
		body   map[string]any
		status int
	}{
		{
			name:   "missing project_id",
			body:   map[string]any{"issue_ids": []string{issue.ID}},
			status: http.StatusBadRequest,
		},
		{
			name:   "missing issue_ids",
			body:   map[string]any{"project_id": proj.ID},
			status: http.StatusBadRequest,
		},
		{
			name:   "nonexistent project",
			body:   map[string]any{"project_id": "NONEXISTENT", "issue_ids": []string{issue.ID}},
			status: http.StatusNotFound,
		},
		{
			name:   "nonexistent issue",
			body:   map[string]any{"project_id": proj.ID, "issue_ids": []string{"NONEXISTENT"}},
			status: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := doJSON(t, router, "POST", "/api/v1/agent/launch", tt.body)
			assert.Equal(t, tt.status, w.Code, "body: %s", w.Body.String())
		})
	}
}

// TestLaunchAgent_IssueFromDifferentProject verifies rejection when issue
// doesn't belong to the specified project.
func TestLaunchAgent_IssueFromDifferentProject(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()

	proj1 := createProject(t, s, "proj1", repoPath)
	proj2 := createProject(t, s, "proj2", "/tmp/proj2")
	issue := createIssue(t, s, proj2.ID, "Wrong project issue")

	w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
		"project_id": proj1.ID,
		"issue_ids":  []string{issue.ID},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "should reject issue from different project")
}

// TestLaunchAgent_MultipleIssues verifies launching with multiple issue IDs.
func TestLaunchAgent_MultipleIssues(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "multi-issue", repoPath)
	issue1 := createIssue(t, s, proj.ID, "First issue")
	issue2 := createIssue(t, s, proj.ID, "Second issue")

	w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
		"project_id": proj.ID,
		"issue_ids":  []string{issue1.ID, issue2.ID},
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	resp := decodeJSON[LaunchAgentResponse](t, w)

	assert.Equal(t, "feature/first-issue", resp.Branch)

	iss1, _ := s.GetIssue(ctx, issue1.ID)
	assert.Equal(t, models.IssueStatusInProgress, iss1.Status)
	iss2, _ := s.GetIssue(ctx, issue2.ID)
	assert.Equal(t, models.IssueStatusInProgress, iss2.Status)

	sess, _ := s.GetAgentSession(ctx, resp.SessionID)
	assert.Equal(t, issue1.ID, sess.IssueID)
}

// TestLaunchAgent_AutoPurgesStaleSessions verifies auto-purge of stale
// abandoned sessions on launch.
func TestLaunchAgent_AutoPurgesStaleSessions(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "purge-test", repoPath)
	issue := createIssue(t, s, proj.ID, "Purge target")

	branch := "feature/purge-target"
	stale := createSession(t, s, proj.ID, issue.ID, branch, "/tmp/old-wt", models.SessionStatusAbandoned)
	tenSec := stale.StartedAt.Add(10 * time.Second)
	stale.EndedAt = &tenSec
	require.NoError(t, s.UpdateAgentSession(ctx, stale))

	w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
		"project_id": proj.ID,
		"issue_ids":  []string{issue.ID},
	})
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	_, err := s.GetAgentSession(ctx, stale.ID)
	assert.Error(t, err, "stale session should be purged on launch")
}

// TestCloseAgent_Validation tests error responses for bad close requests.
func TestCloseAgent_Validation(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()

	proj := createProject(t, s, "close-val", repoPath)
	issue := createIssue(t, s, proj.ID, "Bar")

	t.Run("missing session_id", func(t *testing.T) {
		w := doJSON(t, router, "POST", "/api/v1/agent/close", map[string]any{
			"status": "completed",
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("nonexistent session", func(t *testing.T) {
		w := doJSON(t, router, "POST", "/api/v1/agent/close", map[string]any{
			"session_id": "NONEXISTENT",
			"status":     "completed",
		})
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid status", func(t *testing.T) {
		sess := createSession(t, s, proj.ID, issue.ID, "feature/bar", "/tmp/wt", models.SessionStatusActive)
		w := doJSON(t, router, "POST", "/api/v1/agent/close", map[string]any{
			"session_id": sess.ID,
			"status":     "invalid_status",
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("already completed", func(t *testing.T) {
		sess := createSession(t, s, proj.ID, issue.ID, "feature/already-done", "/tmp/wt2", models.SessionStatusActive)
		doJSON(t, router, "POST", "/api/v1/agent/close", map[string]any{
			"session_id": sess.ID,
			"status":     "completed",
		})
		w := doJSON(t, router, "POST", "/api/v1/agent/close", map[string]any{
			"session_id": sess.ID,
			"status":     "abandoned",
		})
		assert.Equal(t, http.StatusBadRequest, w.Code, "should not close an already-completed session")
	})
}

// TestCloseAgent_DefaultsToIdle verifies omitting status defaults to idle.
func TestCloseAgent_DefaultsToIdle(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "default-idle", repoPath)
	sess := createSession(t, s, proj.ID, "", "feature/test", "/tmp/wt", models.SessionStatusActive)

	w := doJSON(t, router, "POST", "/api/v1/agent/close", map[string]any{
		"session_id": sess.ID,
	})
	require.Equal(t, http.StatusOK, w.Code)
	resp := decodeJSON[CloseAgentResponse](t, w)
	assert.Equal(t, "idle", resp.Status)

	dbSess, _ := s.GetAgentSession(ctx, sess.ID)
	assert.Equal(t, models.SessionStatusIdle, dbSess.Status)
	assert.Nil(t, dbSess.EndedAt)
}

// TestResumeAgent_Validation tests error responses for bad resume requests.
func TestResumeAgent_Validation(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()

	proj := createProject(t, s, "resume-val", repoPath)

	t.Run("missing session_id", func(t *testing.T) {
		w := doJSON(t, router, "POST", "/api/v1/agent/resume", map[string]any{})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("nonexistent session", func(t *testing.T) {
		w := doJSON(t, router, "POST", "/api/v1/agent/resume", map[string]any{
			"session_id": "NONEXISTENT",
		})
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("session not idle", func(t *testing.T) {
		sess := createSession(t, s, proj.ID, "", "feature/not-idle", "/tmp/wt-ni", models.SessionStatusActive)
		w := doJSON(t, router, "POST", "/api/v1/agent/resume", map[string]any{
			"session_id": sess.ID,
		})
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestGetSession_Detail verifies the session detail endpoint with a real
// worktree providing live git enrichment data.
func TestGetSession_Detail(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()

	proj := createProject(t, s, "detail-test", repoPath)
	issue := createIssue(t, s, proj.ID, "Detail issue")

	// Launch to create a real worktree
	w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
		"project_id": proj.ID,
		"issue_ids":  []string{issue.ID},
	})
	require.Equal(t, http.StatusOK, w.Code)
	launchResp := decodeJSON[LaunchAgentResponse](t, w)

	// Make a commit in the worktree
	gitCommitFile(t, launchResp.WorktreePath, "new.go", "package main\n", "add new file")

	// Get session detail — should show enriched git data
	w = doJSON(t, router, "GET", fmt.Sprintf("/api/v1/sessions/%s", launchResp.SessionID), nil)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.Equal(t, launchResp.SessionID, resp["ID"])
	assert.Equal(t, "detail-test", resp["ProjectName"])
	assert.Equal(t, true, resp["WorktreeExists"])
	assert.Equal(t, "feature/detail-issue", resp["CurrentBranch"])
	assert.NotEmpty(t, resp["LastCommitHash"])
	assert.Equal(t, "add new file", resp["LastCommitMessage"])

	// Should be 1 commit ahead of main
	aheadCount, _ := resp["AheadCount"].(float64)
	assert.Equal(t, float64(1), aheadCount)
}

// TestGetSession_NotFound verifies 404 for unknown session ID.
func TestGetSession_NotFound(t *testing.T) {
	srv, _, _, _ := setupE2EServer(t)
	router := srv.Router()

	w := doJSON(t, router, "GET", "/api/v1/sessions/NONEXISTENT", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestListSessions_StatusFilter verifies status and project filtering.
func TestListSessions_StatusFilter(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()

	proj := createProject(t, s, "filter-test", repoPath)

	// Create worktrees so reconciliation doesn't abandon them
	wtDirA := t.TempDir()
	wtDirB := t.TempDir()

	active := createSession(t, s, proj.ID, "", "feature/a", wtDirA, models.SessionStatusActive)
	idle := createSession(t, s, proj.ID, "", "feature/b", wtDirB, models.SessionStatusIdle)
	_ = createSession(t, s, proj.ID, "", "feature/c", "/tmp/nonexistent-c", models.SessionStatusAbandoned)

	t.Run("active_idle filter", func(t *testing.T) {
		w := doJSON(t, router, "GET", "/api/v1/sessions?status=active,idle", nil)
		require.Equal(t, http.StatusOK, w.Code)
		var sessions []sessionResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sessions))
		assert.Len(t, sessions, 2)
		ids := map[string]bool{sessions[0].ID: true, sessions[1].ID: true}
		assert.True(t, ids[active.ID])
		assert.True(t, ids[idle.ID])
	})

	t.Run("abandoned filter", func(t *testing.T) {
		w := doJSON(t, router, "GET", "/api/v1/sessions?status=abandoned", nil)
		require.Equal(t, http.StatusOK, w.Code)
		var sessions []sessionResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sessions))
		assert.Len(t, sessions, 1)
		assert.Equal(t, "abandoned", string(sessions[0].Status))
	})

	t.Run("all sessions", func(t *testing.T) {
		w := doJSON(t, router, "GET", "/api/v1/sessions", nil)
		require.Equal(t, http.StatusOK, w.Code)
		var sessions []sessionResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sessions))
		assert.Len(t, sessions, 3)
	})

	t.Run("project filter", func(t *testing.T) {
		proj2 := createProject(t, s, "other-proj", "/tmp/other")
		createSession(t, s, proj2.ID, "", "feature/x", t.TempDir(), models.SessionStatusActive)

		w := doJSON(t, router, "GET", fmt.Sprintf("/api/v1/sessions?project_id=%s&status=active", proj.ID), nil)
		require.Equal(t, http.StatusOK, w.Code)
		var sessions []sessionResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sessions))
		assert.Len(t, sessions, 1)
		assert.Equal(t, active.ID, sessions[0].ID)
	})
}

// TestListSessions_ReconciliationRefilters verifies that reconciliation
// doesn't leak abandoned sessions into active/idle filtered results.
func TestListSessions_ReconciliationRefilters(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()

	proj := createProject(t, s, "reconcile-test", repoPath)

	// Idle session with missing worktree — reconciliation will abandon it
	_ = createSession(t, s, proj.ID, "", "feature/ghost", "/tmp/nonexistent-worktree-path", models.SessionStatusIdle)

	// Idle session with valid worktree — should survive
	validSess := createSession(t, s, proj.ID, "", "feature/valid", t.TempDir(), models.SessionStatusIdle)

	w := doJSON(t, router, "GET", "/api/v1/sessions?status=active,idle", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var sessions []sessionResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &sessions))

	assert.Len(t, sessions, 1, "ghost session should be excluded after reconciliation")
	assert.Equal(t, validSess.ID, sessions[0].ID)
}

// TestCloseCheck verifies the close-check endpoint returns correct warnings.
func TestCloseCheck(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "check-test", repoPath)

	t.Run("session with real clean worktree", func(t *testing.T) {
		issue := createIssue(t, s, proj.ID, "Close check clean")
		w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
			"project_id": proj.ID,
			"issue_ids":  []string{issue.ID},
		})
		require.Equal(t, http.StatusOK, w.Code)
		launchResp := decodeJSON[LaunchAgentResponse](t, w)

		w = doJSON(t, router, "GET", fmt.Sprintf("/api/v1/sessions/%s/close-check", launchResp.SessionID), nil)
		require.Equal(t, http.StatusOK, w.Code)

		var resp closeCheckResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.True(t, resp.WorktreeExists)
		assert.False(t, resp.IsDirty)
		assert.True(t, resp.ReadyToClose, "clean worktree with no ahead commits should be ready")
		assert.Equal(t, "main", resp.BaseBranch)
	})

	t.Run("session with dirty worktree", func(t *testing.T) {
		issue := createIssue(t, s, proj.ID, "Close check dirty")
		w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
			"project_id": proj.ID,
			"issue_ids":  []string{issue.ID},
		})
		require.Equal(t, http.StatusOK, w.Code)
		launchResp := decodeJSON[LaunchAgentResponse](t, w)

		// Add an uncommitted file to make it dirty
		require.NoError(t, os.WriteFile(filepath.Join(launchResp.WorktreePath, "dirty.txt"), []byte("dirty"), 0o644))

		w = doJSON(t, router, "GET", fmt.Sprintf("/api/v1/sessions/%s/close-check", launchResp.SessionID), nil)
		require.Equal(t, http.StatusOK, w.Code)

		var resp closeCheckResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.True(t, resp.IsDirty)
		assert.False(t, resp.ReadyToClose)

		// Should have a "dirty" warning
		var hasWarning bool
		for _, warn := range resp.Warnings {
			if warn.Type == "dirty" {
				hasWarning = true
			}
		}
		assert.True(t, hasWarning, "should have a dirty warning")
	})

	t.Run("session with unmerged commits", func(t *testing.T) {
		issue := createIssue(t, s, proj.ID, "Close check ahead")
		w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
			"project_id": proj.ID,
			"issue_ids":  []string{issue.ID},
		})
		require.Equal(t, http.StatusOK, w.Code)
		launchResp := decodeJSON[LaunchAgentResponse](t, w)

		gitCommitFile(t, launchResp.WorktreePath, "ahead.go", "package main\n", "ahead commit")

		w = doJSON(t, router, "GET", fmt.Sprintf("/api/v1/sessions/%s/close-check", launchResp.SessionID), nil)
		require.Equal(t, http.StatusOK, w.Code)

		var resp closeCheckResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, 1, resp.AheadCount)
		assert.False(t, resp.ReadyToClose)

		var hasWarning bool
		for _, warn := range resp.Warnings {
			if warn.Type == "unmerged" {
				hasWarning = true
			}
		}
		assert.True(t, hasWarning, "should have an unmerged warning")
	})

	t.Run("session with conflict state", func(t *testing.T) {
		sess := createSession(t, s, proj.ID, "", "feature/conf-check", "/tmp/conf", models.SessionStatusActive)
		sess.ConflictState = models.ConflictStateSyncConflict
		require.NoError(t, s.UpdateAgentSession(ctx, sess))

		w := doJSON(t, router, "GET", fmt.Sprintf("/api/v1/sessions/%s/close-check", sess.ID), nil)
		require.Equal(t, http.StatusOK, w.Code)

		var resp closeCheckResponse
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.False(t, resp.ReadyToClose)

		var hasWarning bool
		for _, warn := range resp.Warnings {
			if warn.Type == "conflict" {
				hasWarning = true
			}
		}
		assert.True(t, hasWarning, "should have a conflict warning")
	})

	t.Run("not found", func(t *testing.T) {
		w := doJSON(t, router, "GET", "/api/v1/sessions/NONEXISTENT/close-check", nil)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// TestReactivateSession tests reactivation from terminal states.
func TestReactivateSession(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "react-test", repoPath)
	issue := createIssue(t, s, proj.ID, "Reactivate me")

	for _, fromStatus := range []models.SessionStatus{models.SessionStatusCompleted, models.SessionStatusAbandoned} {
		t.Run(string(fromStatus), func(t *testing.T) {
			wtDir := t.TempDir()
			sess := createSession(t, s, proj.ID, issue.ID, "feature/react-"+string(fromStatus), wtDir, models.SessionStatusActive)

			sess.Status = fromStatus
			now := time.Now().UTC()
			sess.EndedAt = &now
			require.NoError(t, s.UpdateAgentSession(ctx, sess))

			// Set issue to cascaded state
			iss, _ := s.GetIssue(ctx, issue.ID)
			if fromStatus == models.SessionStatusCompleted {
				iss.Status = models.IssueStatusDone
			} else {
				iss.Status = models.IssueStatusOpen
			}
			require.NoError(t, s.UpdateIssue(ctx, iss))

			w := doJSON(t, router, "POST", fmt.Sprintf("/api/v1/sessions/%s/reactivate", sess.ID), nil)
			require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

			dbSess, _ := s.GetAgentSession(ctx, sess.ID)
			assert.Equal(t, models.SessionStatusIdle, dbSess.Status)
			assert.Nil(t, dbSess.EndedAt)

			dbIssue, _ := s.GetIssue(ctx, issue.ID)
			assert.Equal(t, models.IssueStatusInProgress, dbIssue.Status)
		})
	}

	t.Run("missing worktree on disk", func(t *testing.T) {
		sess := createSession(t, s, proj.ID, "", "feature/missing-wt", "/tmp/gone-forever", models.SessionStatusAbandoned)
		now := time.Now().UTC()
		sess.EndedAt = &now
		require.NoError(t, s.UpdateAgentSession(ctx, sess))

		w := doJSON(t, router, "POST", fmt.Sprintf("/api/v1/sessions/%s/reactivate", sess.ID), nil)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("session not in terminal state", func(t *testing.T) {
		sess := createSession(t, s, proj.ID, "", "feature/active-react", t.TempDir(), models.SessionStatusActive)
		w := doJSON(t, router, "POST", fmt.Sprintf("/api/v1/sessions/%s/reactivate", sess.ID), nil)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestDeleteWorktree verifies worktree deletion transitions.
func TestDeleteWorktree(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "del-wt", repoPath)
	issue := createIssue(t, s, proj.ID, "Delete wt test")

	// Launch a real session (creates a real git worktree)
	w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
		"project_id": proj.ID,
		"issue_ids":  []string{issue.ID},
	})
	require.Equal(t, http.StatusOK, w.Code)
	launchResp := decodeJSON[LaunchAgentResponse](t, w)

	// Close to idle first
	w = doJSON(t, router, "POST", "/api/v1/agent/close", map[string]any{
		"session_id": launchResp.SessionID,
		"status":     "idle",
	})
	require.Equal(t, http.StatusOK, w.Code)

	// Verify worktree exists on disk
	_, err := os.Stat(launchResp.WorktreePath)
	require.NoError(t, err, "worktree should exist before delete")

	// Delete the worktree
	w = doJSON(t, router, "DELETE", fmt.Sprintf("/api/v1/sessions/%s/worktree", launchResp.SessionID), nil)
	require.Equal(t, http.StatusNoContent, w.Code)

	dbSess, _ := s.GetAgentSession(ctx, launchResp.SessionID)
	assert.Equal(t, models.SessionStatusAbandoned, dbSess.Status)
	assert.Empty(t, dbSess.WorktreePath)
	assert.NotNil(t, dbSess.EndedAt)

	dbIssue, _ := s.GetIssue(ctx, issue.ID)
	assert.Equal(t, models.IssueStatusOpen, dbIssue.Status)

	// Verify worktree is actually gone from disk
	_, err = os.Stat(launchResp.WorktreePath)
	assert.True(t, os.IsNotExist(err), "worktree should be removed from disk")
}

// TestDeleteWorktree_NotFound verifies 404.
func TestDeleteWorktree_NotFound(t *testing.T) {
	srv, _, _, _ := setupE2EServer(t)
	router := srv.Router()

	w := doJSON(t, router, "DELETE", "/api/v1/sessions/NONEXISTENT/worktree", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestCleanupSessions_PreservesNonStale verifies cleanup selectivity.
func TestCleanupSessions_PreservesNonStale(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "cleanup-preserve", repoPath)

	// Stale: abandoned, 0 commits, short duration — should be deleted
	stale := createSession(t, s, proj.ID, "", "feature/stale", "/tmp/s1", models.SessionStatusAbandoned)
	tenSec := stale.StartedAt.Add(10 * time.Second)
	stale.EndedAt = &tenSec
	require.NoError(t, s.UpdateAgentSession(ctx, stale))

	// Not stale: has commits
	withCommits := createSession(t, s, proj.ID, "", "feature/commits", "/tmp/s2", models.SessionStatusAbandoned)
	withCommits.CommitCount = 5
	shortEnd := withCommits.StartedAt.Add(10 * time.Second)
	withCommits.EndedAt = &shortEnd
	require.NoError(t, s.UpdateAgentSession(ctx, withCommits))

	// Not stale: long duration
	longDuration := createSession(t, s, proj.ID, "", "feature/long", "/tmp/s3", models.SessionStatusAbandoned)
	longEnd := longDuration.StartedAt.Add(5 * time.Minute)
	longDuration.EndedAt = &longEnd
	require.NoError(t, s.UpdateAgentSession(ctx, longDuration))

	// Not stale: active
	active := createSession(t, s, proj.ID, "", "feature/active-s", t.TempDir(), models.SessionStatusActive)

	w := doJSON(t, router, "DELETE", "/api/v1/sessions/cleanup", nil)
	require.Equal(t, http.StatusOK, w.Code)
	var result map[string]int64
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, int64(1), result["deleted"])

	_, err := s.GetAgentSession(ctx, stale.ID)
	assert.Error(t, err, "stale session should be deleted")

	_, err = s.GetAgentSession(ctx, withCommits.ID)
	assert.NoError(t, err, "session with commits should survive")

	_, err = s.GetAgentSession(ctx, longDuration.ID)
	assert.NoError(t, err, "long-duration session should survive")

	_, err = s.GetAgentSession(ctx, active.ID)
	assert.NoError(t, err, "active session should survive")
}

// TestReconcileSessions_Flows tests reconciliation through the list API.
func TestReconcileSessions_Flows(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "reconcile-flows", repoPath)

	t.Run("idle with missing worktree gets abandoned", func(t *testing.T) {
		sess := createSession(t, s, proj.ID, "", "feature/missing-r", "/tmp/does-not-exist", models.SessionStatusIdle)
		doJSON(t, router, "GET", "/api/v1/sessions", nil)

		dbSess, _ := s.GetAgentSession(ctx, sess.ID)
		assert.Equal(t, models.SessionStatusAbandoned, dbSess.Status)
	})

	t.Run("abandoned with existing worktree recovers to idle", func(t *testing.T) {
		wtDir := t.TempDir()
		sess := createSession(t, s, proj.ID, "", "feature/recovered", wtDir, models.SessionStatusAbandoned)
		now := time.Now().UTC()
		sess.EndedAt = &now
		require.NoError(t, s.UpdateAgentSession(ctx, sess))

		doJSON(t, router, "GET", "/api/v1/sessions", nil)

		dbSess, _ := s.GetAgentSession(ctx, sess.ID)
		assert.Equal(t, models.SessionStatusIdle, dbSess.Status)
		assert.Nil(t, dbSess.EndedAt)
	})

	t.Run("completed is never reconciled", func(t *testing.T) {
		sess := createSession(t, s, proj.ID, "", "feature/completed-r", "/tmp/also-missing", models.SessionStatusActive)
		sess.Status = models.SessionStatusCompleted
		now := time.Now().UTC()
		sess.EndedAt = &now
		require.NoError(t, s.UpdateAgentSession(ctx, sess))

		doJSON(t, router, "GET", "/api/v1/sessions", nil)

		dbSess, _ := s.GetAgentSession(ctx, sess.ID)
		assert.Equal(t, models.SessionStatusCompleted, dbSess.Status)
	})
}

// TestSyncSession_RealGit tests session sync against a real git repo.
func TestSyncSession_RealGit(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()
	ctx := context.Background()

	proj := createProject(t, s, "sync-test", repoPath)
	issue := createIssue(t, s, proj.ID, "Sync test issue")

	// Launch to create real worktree
	w := doJSON(t, router, "POST", "/api/v1/agent/launch", map[string]any{
		"project_id": proj.ID,
		"issue_ids":  []string{issue.ID},
	})
	require.Equal(t, http.StatusOK, w.Code)
	launchResp := decodeJSON[LaunchAgentResponse](t, w)

	// Add a commit on main so the feature branch is behind
	gitCommitFile(t, repoPath, "main-update.txt", "from main\n", "main branch update")

	// Sync the session worktree
	w = doJSON(t, router, "POST", fmt.Sprintf("/api/v1/sessions/%s/sync", launchResp.SessionID), map[string]any{})
	require.Equal(t, http.StatusOK, w.Code, "sync body: %s", w.Body.String())

	var syncResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &syncResp))
	assert.Equal(t, true, syncResp["Success"])

	// Verify the sync was recorded in the session
	sess, _ := s.GetAgentSession(ctx, launchResp.SessionID)
	assert.NotNil(t, sess.LastSyncAt)
	assert.Equal(t, models.ConflictStateNone, sess.ConflictState)

	// The main-update.txt file should now be in the worktree
	_, err := os.Stat(filepath.Join(launchResp.WorktreePath, "main-update.txt"))
	assert.NoError(t, err, "synced file should exist in worktree")
}

// TestSyncSession_NotFound verifies 404 for unknown session.
func TestSyncSession_NotFound(t *testing.T) {
	srv, _, _, _ := setupE2EServer(t)
	router := srv.Router()

	w := doJSON(t, router, "POST", "/api/v1/sessions/NONEXISTENT/sync", map[string]any{})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestMergeSession_NotFound verifies 404 for unknown session.
func TestMergeSession_NotFound(t *testing.T) {
	srv, _, _, _ := setupE2EServer(t)
	router := srv.Router()

	w := doJSON(t, router, "POST", "/api/v1/sessions/NONEXISTENT/merge", map[string]any{})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestDiscoverWorktrees_RealGit tests discovery of untracked worktrees
// against a real git repo.
func TestDiscoverWorktrees_RealGit(t *testing.T) {
	srv, s, _, repoPath := setupE2EServer(t)
	router := srv.Router()

	proj := createProject(t, s, "discover-test", repoPath)

	// Create a worktree outside of pm's knowledge
	wtDir := filepath.Join(repoPath+".worktrees", "untracked-feature")
	require.NoError(t, os.MkdirAll(repoPath+".worktrees", 0o755))
	out, err := exec.Command("git", "-C", repoPath, "worktree", "add", "-b", "feature/untracked", wtDir, "main").CombinedOutput()
	require.NoError(t, err, "git worktree add: %s", string(out))

	// Discover
	w := doJSON(t, router, "POST", fmt.Sprintf("/api/v1/sessions/discover?project_id=%s", proj.ID), nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	count, _ := resp["count"].(float64)
	assert.Equal(t, float64(1), count, "should discover 1 untracked worktree")

	discovered, _ := resp["discovered"].([]any)
	require.Len(t, discovered, 1)
	disc := discovered[0].(map[string]any)
	assert.Equal(t, true, disc["Discovered"])
	assert.Equal(t, "idle", disc["Status"])
	assert.Contains(t, disc["WorktreePath"], "untracked-feature")
}

// TestDiscoverWorktrees_NotFound verifies 404.
func TestDiscoverWorktrees_NotFound(t *testing.T) {
	srv, _, _, _ := setupE2EServer(t)
	router := srv.Router()

	w := doJSON(t, router, "POST", "/api/v1/sessions/discover?project_id=NONEXISTENT", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// TestSessionConflictState_Persistence verifies conflict state round-trips.
func TestSessionConflictState_Persistence(t *testing.T) {
	_, s, _, repoPath := setupE2EServer(t)
	ctx := context.Background()

	proj := createProject(t, s, "conflict-persist", repoPath)
	sess := createSession(t, s, proj.ID, "", "feature/conflict", "/tmp/conf", models.SessionStatusActive)

	sess.ConflictState = models.ConflictStateSyncConflict
	sess.ConflictFiles = `["file1.go","file2.go"]`
	sess.LastError = "merge conflict in file1.go"
	require.NoError(t, s.UpdateAgentSession(ctx, sess))

	dbSess, err := s.GetAgentSession(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ConflictStateSyncConflict, dbSess.ConflictState)
	assert.Equal(t, `["file1.go","file2.go"]`, dbSess.ConflictFiles)
	assert.Equal(t, "merge conflict in file1.go", dbSess.LastError)

	dbSess.ConflictState = models.ConflictStateNone
	dbSess.ConflictFiles = "[]"
	dbSess.LastError = ""
	require.NoError(t, s.UpdateAgentSession(ctx, dbSess))

	dbSess2, _ := s.GetAgentSession(ctx, sess.ID)
	assert.Equal(t, models.ConflictStateNone, dbSess2.ConflictState)
	assert.Empty(t, dbSess2.LastError)
}

// TestIssueToBranch_Formatting verifies branch name generation.
func TestIssueToBranch_Formatting(t *testing.T) {
	tests := []struct {
		title    string
		expected string
	}{
		{"Add user login", "feature/add-user-login"},
		{"Fix BUG #123!", "feature/fix-bug-123"},
		{"  Multiple   Spaces  ", "feature/multiple-spaces"},
		{"Very Long Title That Exceeds The Fifty Character Limit For Branch Names", "feature/very-long-title-that-exceeds-the-fifty-character-l"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			assert.Equal(t, tt.expected, issueToBranch(tt.title))
		})
	}
}
