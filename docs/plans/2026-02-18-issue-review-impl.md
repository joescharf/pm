# Issue Review Feature Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add AI-powered issue review with a `pm_prepare_review` / `pm_save_review` MCP workflow that transitions issues from "done" to "closed" (pass) or back to "in_progress" (fail), with historical review tracking and optional UI/UX review via rodney.

**Architecture:** Two new MCP tools gather review context and store results. A new `issue_reviews` table stores review history. Three new project fields (`build_cmd`, `serve_cmd`, `serve_port`) support automated UI testing. The git client gets a `Diff` method. The REST API and React UI are extended to display reviews and edit project build/serve settings.

**Tech Stack:** Go (Cobra/Viper CLI, SQLite, mcp-go), React/TypeScript (Bun, shadcn/ui)

---

### Task 1: Add `Diff` and `DiffStat` methods to git Client

**Files:**
- Modify: `internal/git/git.go` (add to Client interface + RealClient)
- Test: `internal/git/git_test.go`

**Step 1: Write the failing test**

```go
// internal/git/git_test.go
func TestRealClient_Diff(t *testing.T) {
	// Create a temp git repo with a branch
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
	}

	run("init", "-b", "main")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello"), 0644))
	run("add", ".")
	run("commit", "-m", "init")
	run("checkout", "-b", "feature/test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello world"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new file"), 0644))
	run("add", ".")
	run("commit", "-m", "feature changes")

	c := NewClient()

	diff, err := c.Diff(dir, "main", "feature/test")
	require.NoError(t, err)
	assert.Contains(t, diff, "hello world")
	assert.Contains(t, diff, "new.txt")

	stat, err := c.DiffStat(dir, "main", "feature/test")
	require.NoError(t, err)
	assert.Contains(t, stat, "2 files changed")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/git/ -run TestRealClient_Diff -v`
Expected: FAIL — `Diff` and `DiffStat` not defined on Client

**Step 3: Write minimal implementation**

Add to the `Client` interface in `internal/git/git.go`:

```go
Diff(path, base, head string) (string, error)
DiffStat(path, base, head string) (string, error)
DiffNameOnly(path, base, head string) ([]string, error)
```

Add to `RealClient`:

```go
func (c *RealClient) Diff(path, base, head string) (string, error) {
	return gitCmd(path, "diff", base+"..."+head)
}

func (c *RealClient) DiffStat(path, base, head string) (string, error) {
	return gitCmd(path, "diff", "--stat", base+"..."+head)
}

func (c *RealClient) DiffNameOnly(path, base, head string) ([]string, error) {
	out, err := gitCmd(path, "diff", "--name-only", base+"..."+head)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/git/ -run TestRealClient_Diff -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/git.go internal/git/git_test.go
git commit -m "feat: add Diff, DiffStat, DiffNameOnly to git client"
```

---

### Task 2: Add project build/serve fields — migration + model

**Files:**
- Create: `internal/store/migrations/010_add_project_build_fields.sql`
- Modify: `internal/models/project.go`
- Modify: `internal/store/sqlite.go` (all project queries)
- Test: `internal/store/sqlite_test.go`

**Step 1: Write the failing test**

```go
// Add to internal/store/sqlite_test.go
func TestProjectBuildFields(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p := &models.Project{
		Name:      "test-project",
		Path:      "/tmp/test",
		BuildCmd:  "npm run build",
		ServeCmd:  "npm run dev",
		ServePort: 3000,
	}
	require.NoError(t, s.CreateProject(ctx, p))

	got, err := s.GetProject(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "npm run build", got.BuildCmd)
	assert.Equal(t, "npm run dev", got.ServeCmd)
	assert.Equal(t, 3000, got.ServePort)

	// Update
	got.BuildCmd = "make build"
	require.NoError(t, s.UpdateProject(ctx, got))
	got2, _ := s.GetProject(ctx, p.ID)
	assert.Equal(t, "make build", got2.BuildCmd)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestProjectBuildFields -v`
Expected: FAIL — `BuildCmd` not a field on Project

**Step 3: Create migration**

```sql
-- internal/store/migrations/010_add_project_build_fields.sql
ALTER TABLE projects ADD COLUMN build_cmd TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN serve_cmd TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN serve_port INTEGER DEFAULT 0;
```

**Step 4: Update model**

Add to `internal/models/project.go` Project struct:

```go
BuildCmd  string
ServeCmd  string
ServePort int
```

**Step 5: Update all project queries in `internal/store/sqlite.go`**

Every project query that selects/inserts/updates columns must include `build_cmd`, `serve_cmd`, `serve_port`. The affected methods are:
- `CreateProject` — add to INSERT column list + VALUES
- `GetProject` — add to SELECT + Scan
- `GetProjectByName` — add to SELECT + Scan
- `GetProjectByPath` — add to SELECT + Scan
- `ListProjects` (both branches) — add to SELECT + Scan
- `UpdateProject` — add to SET clause

The column list pattern for SELECT becomes:
```
id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, build_cmd, serve_cmd, serve_port, created_at, updated_at
```

And Scan adds `&p.BuildCmd, &p.ServeCmd, &p.ServePort` before `&p.CreatedAt`.

**Step 6: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestProjectBuildFields -v`
Expected: PASS

**Step 7: Run all store tests to verify no regressions**

Run: `go test ./internal/store/ -v`
Expected: All PASS

**Step 8: Commit**

```bash
git add internal/store/migrations/010_add_project_build_fields.sql internal/models/project.go internal/store/sqlite.go internal/store/sqlite_test.go
git commit -m "feat: add build_cmd, serve_cmd, serve_port to project model"
```

---

### Task 3: Add IssueReview model + migration + store methods

**Files:**
- Create: `internal/models/review.go`
- Create: `internal/store/migrations/011_create_issue_reviews.sql`
- Modify: `internal/store/store.go` (add interface methods)
- Modify: `internal/store/sqlite.go` (implement methods)
- Test: `internal/store/sqlite_test.go`

**Step 1: Write the failing test**

```go
// Add to internal/store/sqlite_test.go
func TestIssueReviewCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create a project + issue first
	p := &models.Project{Name: "test", Path: "/tmp/test"}
	require.NoError(t, s.CreateProject(ctx, p))
	issue := &models.Issue{
		ProjectID: p.ID, Title: "test issue",
		Status: models.IssueStatusDone, Priority: models.IssuePriorityMedium,
		Type: models.IssueTypeFeature,
	}
	require.NoError(t, s.CreateIssue(ctx, issue))

	// Create a review
	now := time.Now().UTC()
	review := &models.IssueReview{
		IssueID:           issue.ID,
		SessionID:         "",
		Verdict:           models.ReviewVerdictFail,
		Summary:           "Missing test coverage for edge cases",
		CodeQuality:       "pass",
		RequirementsMatch: "pass",
		TestCoverage:      "fail",
		UIUX:              "na",
		FailureReasons:    []string{"No tests for empty input", "No tests for error paths"},
		DiffStats:         "3 files changed, 50 insertions(+), 10 deletions(-)",
		ReviewedAt:        now,
	}
	require.NoError(t, s.CreateIssueReview(ctx, review))
	assert.NotEmpty(t, review.ID)

	// List reviews for issue
	reviews, err := s.ListIssueReviews(ctx, issue.ID)
	require.NoError(t, err)
	require.Len(t, reviews, 1)
	assert.Equal(t, models.ReviewVerdictFail, reviews[0].Verdict)
	assert.Equal(t, "Missing test coverage for edge cases", reviews[0].Summary)
	assert.Equal(t, "fail", string(reviews[0].TestCoverage))
	assert.Equal(t, []string{"No tests for empty input", "No tests for error paths"}, reviews[0].FailureReasons)

	// Add a second review (pass)
	review2 := &models.IssueReview{
		IssueID:           issue.ID,
		Verdict:           models.ReviewVerdictPass,
		Summary:           "All checks pass after fixes",
		CodeQuality:       "pass",
		RequirementsMatch: "pass",
		TestCoverage:      "pass",
		UIUX:              "na",
		ReviewedAt:        now.Add(time.Hour),
	}
	require.NoError(t, s.CreateIssueReview(ctx, review2))

	reviews, _ = s.ListIssueReviews(ctx, issue.ID)
	assert.Len(t, reviews, 2)
	// Most recent first
	assert.Equal(t, models.ReviewVerdictPass, reviews[0].Verdict)
	assert.Equal(t, models.ReviewVerdictFail, reviews[1].Verdict)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestIssueReviewCRUD -v`
Expected: FAIL — `IssueReview` type not defined

**Step 3: Create the model**

```go
// internal/models/review.go
package models

import "time"

// ReviewVerdict is the outcome of an issue review.
type ReviewVerdict string

const (
	ReviewVerdictPass ReviewVerdict = "pass"
	ReviewVerdictFail ReviewVerdict = "fail"
)

// ReviewCategory is a per-aspect rating.
type ReviewCategory string

// IssueReview records a single AI review of an issue's implementation.
type IssueReview struct {
	ID                string
	IssueID           string
	SessionID         string
	Verdict           ReviewVerdict
	Summary           string
	CodeQuality       ReviewCategory
	RequirementsMatch ReviewCategory
	TestCoverage      ReviewCategory
	UIUX              ReviewCategory
	FailureReasons    []string
	DiffStats         string
	ReviewedAt        time.Time
	CreatedAt         time.Time
}
```

**Step 4: Create migration**

```sql
-- internal/store/migrations/011_create_issue_reviews.sql
CREATE TABLE IF NOT EXISTS issue_reviews (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    session_id TEXT DEFAULT '',
    verdict TEXT NOT NULL,
    summary TEXT NOT NULL,
    code_quality TEXT DEFAULT '',
    requirements_match TEXT DEFAULT '',
    test_coverage TEXT DEFAULT '',
    ui_ux TEXT DEFAULT '',
    failure_reasons TEXT DEFAULT '[]',
    diff_stats TEXT DEFAULT '',
    reviewed_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_issue_reviews_issue_id ON issue_reviews(issue_id);
```

**Step 5: Add to store interface**

Add to `internal/store/store.go` Store interface, after Agent Sessions section:

```go
// Issue Reviews
CreateIssueReview(ctx context.Context, review *models.IssueReview) error
ListIssueReviews(ctx context.Context, issueID string) ([]*models.IssueReview, error)
```

**Step 6: Implement store methods**

Add to `internal/store/sqlite.go`:

```go
// --- Issue Reviews ---

func (s *SQLiteStore) CreateIssueReview(ctx context.Context, review *models.IssueReview) error {
	if review.ID == "" {
		review.ID = newULID()
	}
	review.CreatedAt = time.Now().UTC()

	failureJSON, err := json.Marshal(review.FailureReasons)
	if err != nil {
		failureJSON = []byte("[]")
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO issue_reviews (id, issue_id, session_id, verdict, summary, code_quality, requirements_match, test_coverage, ui_ux, failure_reasons, diff_stats, reviewed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		review.ID, review.IssueID, review.SessionID,
		string(review.Verdict), review.Summary,
		string(review.CodeQuality), string(review.RequirementsMatch),
		string(review.TestCoverage), string(review.UIUX),
		string(failureJSON), review.DiffStats,
		review.ReviewedAt, review.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create issue review: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListIssueReviews(ctx context.Context, issueID string) ([]*models.IssueReview, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, issue_id, session_id, verdict, summary, code_quality, requirements_match, test_coverage, ui_ux, failure_reasons, diff_stats, reviewed_at, created_at
		FROM issue_reviews WHERE issue_id = ? ORDER BY reviewed_at DESC`, issueID)
	if err != nil {
		return nil, fmt.Errorf("list issue reviews: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var reviews []*models.IssueReview
	for rows.Next() {
		r := &models.IssueReview{}
		var failureJSON string
		if err := rows.Scan(&r.ID, &r.IssueID, &r.SessionID,
			&r.Verdict, &r.Summary,
			&r.CodeQuality, &r.RequirementsMatch,
			&r.TestCoverage, &r.UIUX,
			&failureJSON, &r.DiffStats,
			&r.ReviewedAt, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan issue review: %w", err)
		}
		_ = json.Unmarshal([]byte(failureJSON), &r.FailureReasons)
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}
```

Note: Add `"encoding/json"` to the import block in `sqlite.go` if not already present.

**Step 7: Update mock store in `internal/mcp/server_test.go`**

Add to the `mockStore` struct:

```go
reviews []*models.IssueReview
createdReviews []*models.IssueReview
```

Add methods:

```go
func (m *mockStore) CreateIssueReview(_ context.Context, review *models.IssueReview) error {
	if review.ID == "" {
		review.ID = fmt.Sprintf("review-%d", len(m.reviews)+1)
	}
	review.CreatedAt = time.Now()
	m.reviews = append(m.reviews, review)
	m.createdReviews = append(m.createdReviews, review)
	return nil
}

func (m *mockStore) ListIssueReviews(_ context.Context, issueID string) ([]*models.IssueReview, error) {
	var result []*models.IssueReview
	for _, r := range m.reviews {
		if r.IssueID == issueID {
			result = append(result, r)
		}
	}
	return result, nil
}
```

**Step 8: Run tests**

Run: `go test ./internal/store/ -run TestIssueReviewCRUD -v`
Expected: PASS

Run: `go test ./... -count=1`
Expected: All PASS (mock store satisfies interface)

**Step 9: Commit**

```bash
git add internal/models/review.go internal/store/migrations/011_create_issue_reviews.sql internal/store/store.go internal/store/sqlite.go internal/store/sqlite_test.go internal/mcp/server_test.go
git commit -m "feat: add issue_reviews table, model, and store methods"
```

---

### Task 4: Add `pm_prepare_review` MCP tool

**Files:**
- Modify: `internal/mcp/server.go`
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
// Add to internal/mcp/server_test.go
func TestPrepareReview(t *testing.T) {
	ms := &mockStore{
		projects: []*models.Project{{ID: "p1", Name: "myproject", Path: "/tmp/myproject", Language: "go"}},
		issues: []*models.Issue{{
			ID: "ISSUE001", ProjectID: "p1", Title: "Add login",
			Description: "Implement login page", Body: "Full requirements...",
			Status: models.IssueStatusDone, Priority: models.IssuePriorityMedium,
			Type: models.IssueTypeFeature,
		}},
		sessions: []*models.AgentSession{{
			ID: "s1", ProjectID: "p1", IssueID: "ISSUE001",
			Branch: "feature/add-login", WorktreePath: "/tmp/myproject-feature-add-login",
			Status: models.SessionStatusCompleted, CommitCount: 3,
		}},
	}

	mg := &mockGit{
		diff:     "diff --git a/cmd/login.go b/cmd/login.go\n+func Login() {}",
		diffStat: " 2 files changed, 50 insertions(+), 5 deletions(-)",
		diffNames: []string{"cmd/login.go", "internal/auth/auth.go"},
	}

	srv := NewServer(ms, mg, nil, nil)
	mcpSrv := srv.MCPServer()

	result, err := mcpSrv.CallTool(context.Background(), "pm_prepare_review", map[string]any{
		"issue_id": "ISSUE001",
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "expected success")

	var out map[string]any
	text := result.Content[0].(mcpgo.TextContent).Text
	require.NoError(t, json.Unmarshal([]byte(text), &out))

	assert.Equal(t, "Add login", out["issue"].(map[string]any)["title"])
	assert.Contains(t, out["diff"], "Login")
	assert.Equal(t, false, out["ui_review_needed"])
}
```

Note: The mock git client needs `Diff`, `DiffStat`, `DiffNameOnly` methods added. Update the `mockGit` struct in the test file to include:

```go
type mockGit struct {
	// existing fields...
	diff      string
	diffStat  string
	diffNames []string
}

func (m *mockGit) Diff(path, base, head string) (string, error) { return m.diff, nil }
func (m *mockGit) DiffStat(path, base, head string) (string, error) { return m.diffStat, nil }
func (m *mockGit) DiffNameOnly(path, base, head string) ([]string, error) { return m.diffNames, nil }
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run TestPrepareReview -v`
Expected: FAIL — `pm_prepare_review` tool not registered

**Step 3: Implement `pm_prepare_review`**

Add to `internal/mcp/server.go`:

Register in `MCPServer()`:
```go
srv.AddTool(s.prepareReviewTool())
srv.AddTool(s.saveReviewTool())
```

Tool definition:
```go
func (s *Server) prepareReviewTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_prepare_review",
		mcp.WithDescription("Gather all context needed to review an issue's implementation. Returns issue requirements, git diff, changed files, UI review flags, and review history. The calling agent should analyze this context and then call pm_save_review with the verdict."),
		mcp.WithString("issue_id", mcp.Required(), mcp.Description("Issue ID (full ULID or unique prefix)")),
		mcp.WithString("base_ref", mcp.Description("Base ref for diff (default: main, or auto-detected from session branch)")),
		mcp.WithString("head_ref", mcp.Description("Head ref for diff (default: session branch, or HEAD)")),
		mcp.WithString("app_url", mcp.Description("URL of running app for UI/UX review via rodney (e.g. http://localhost:3000)")),
	)
	return tool, s.handlePrepareReview
}
```

Handler:
```go
func (s *Server) handlePrepareReview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	issueID, err := request.RequireString("issue_id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: issue_id"), nil
	}

	issue, err := s.findIssue(ctx, issueID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("issue not found: %s", issueID)), nil
	}

	// Resolve project
	project, err := s.store.GetProject(ctx, issue.ProjectID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found for issue: %v", err)), nil
	}

	// Find linked session (most recent for this issue)
	var session *models.AgentSession
	sessions, _ := s.store.ListAgentSessions(ctx, project.ID, 0)
	for _, sess := range sessions {
		if sess.IssueID == issue.ID {
			session = sess
			break
		}
	}

	// Determine diff refs
	baseRef := request.GetString("base_ref", "main")
	headRef := request.GetString("head_ref", "")
	if headRef == "" && session != nil && session.Branch != "" {
		headRef = session.Branch
	}
	if headRef == "" {
		headRef = "HEAD"
	}

	// Get diff (best-effort)
	var diff, diffStat string
	var filesChanged []string
	if s.git != nil && project.Path != "" {
		diff, _ = s.git.Diff(project.Path, baseRef, headRef)
		diffStat, _ = s.git.DiffStat(project.Path, baseRef, headRef)
		filesChanged, _ = s.git.DiffNameOnly(project.Path, baseRef, headRef)
	}

	// Check if UI review is needed
	uiReviewNeeded := false
	for _, f := range filesChanged {
		if strings.HasPrefix(f, "ui/") || strings.HasPrefix(f, "internal/ui/") {
			uiReviewNeeded = true
			break
		}
	}

	// Build UI context
	appURL := request.GetString("app_url", "")
	uiContext := map[string]any{
		"build_cmd":  project.BuildCmd,
		"serve_cmd":  project.ServeCmd,
		"serve_port": project.ServePort,
		"app_url":    appURL,
	}

	// Fetch review history
	reviews, _ := s.store.ListIssueReviews(ctx, issue.ID)
	var reviewHistory []map[string]any
	for _, r := range reviews {
		reviewHistory = append(reviewHistory, map[string]any{
			"verdict":    string(r.Verdict),
			"summary":    r.Summary,
			"reviewed_at": r.ReviewedAt.Format(time.RFC3339),
		})
	}

	// Build session info
	var sessionOut map[string]any
	if session != nil {
		sessionOut = map[string]any{
			"id":            session.ID,
			"branch":        session.Branch,
			"worktree_path": session.WorktreePath,
			"commit_count":  session.CommitCount,
		}
	}

	result := map[string]any{
		"issue": map[string]any{
			"id":          issue.ID,
			"title":       issue.Title,
			"description": issue.Description,
			"body":        issue.Body,
			"type":        string(issue.Type),
			"priority":    string(issue.Priority),
			"status":      string(issue.Status),
		},
		"session":          sessionOut,
		"diff":             diff,
		"diff_stats":       diffStat,
		"files_changed":    filesChanged,
		"ui_review_needed": uiReviewNeeded,
		"ui_context":       uiContext,
		"review_history":   reviewHistory,
		"project": map[string]any{
			"name":     project.Name,
			"path":     project.Path,
			"language": project.Language,
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal review context: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/mcp/ -run TestPrepareReview -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat: add pm_prepare_review MCP tool"
```

---

### Task 5: Add `pm_save_review` MCP tool

**Files:**
- Modify: `internal/mcp/server.go`
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing tests**

```go
func TestSaveReview_Pass(t *testing.T) {
	issue := &models.Issue{
		ID: "ISSUE001", ProjectID: "p1", Title: "Add login",
		Status: models.IssueStatusDone, Priority: models.IssuePriorityMedium,
		Type: models.IssueTypeFeature,
	}
	ms := &mockStore{
		projects: []*models.Project{{ID: "p1", Name: "myproject"}},
		issues:   []*models.Issue{issue},
	}
	srv := NewServer(ms, nil, nil, nil)
	mcpSrv := srv.MCPServer()

	result, err := mcpSrv.CallTool(context.Background(), "pm_save_review", map[string]any{
		"issue_id":           "ISSUE001",
		"verdict":            "pass",
		"summary":            "All checks pass. Code is clean and requirements are met.",
		"code_quality":       "pass",
		"requirements_match": "pass",
		"test_coverage":      "pass",
		"ui_ux":              "na",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	// Verify review was created
	require.Len(t, ms.createdReviews, 1)
	assert.Equal(t, models.ReviewVerdictPass, ms.createdReviews[0].Verdict)

	// Verify issue transitioned to closed
	require.Len(t, ms.updatedIssues, 1)
	assert.Equal(t, models.IssueStatusClosed, ms.updatedIssues[0].Status)
	assert.NotNil(t, ms.updatedIssues[0].ClosedAt)
}

func TestSaveReview_Fail(t *testing.T) {
	issue := &models.Issue{
		ID: "ISSUE002", ProjectID: "p1", Title: "Add search",
		Status: models.IssueStatusDone, Priority: models.IssuePriorityMedium,
		Type: models.IssueTypeFeature,
	}
	ms := &mockStore{
		projects: []*models.Project{{ID: "p1", Name: "myproject"}},
		issues:   []*models.Issue{issue},
	}
	srv := NewServer(ms, nil, nil, nil)
	mcpSrv := srv.MCPServer()

	result, err := mcpSrv.CallTool(context.Background(), "pm_save_review", map[string]any{
		"issue_id":           "ISSUE002",
		"verdict":            "fail",
		"summary":            "Missing test coverage for edge cases",
		"code_quality":       "pass",
		"requirements_match": "pass",
		"test_coverage":      "fail",
		"ui_ux":              "na",
		"failure_reasons":    "No tests for empty input\nNo tests for error paths",
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	// Verify issue transitioned to in_progress
	require.Len(t, ms.updatedIssues, 1)
	assert.Equal(t, models.IssueStatusInProgress, ms.updatedIssues[0].Status)
	assert.Nil(t, ms.updatedIssues[0].ClosedAt)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/mcp/ -run TestSaveReview -v`
Expected: FAIL — `pm_save_review` not registered

**Step 3: Implement `pm_save_review`**

```go
func (s *Server) saveReviewTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_save_review",
		mcp.WithDescription("Save the result of an issue review. On pass, transitions issue to closed. On fail, transitions issue to in_progress with failure reasons. Creates a historical review record."),
		mcp.WithString("issue_id", mcp.Required(), mcp.Description("Issue ID (full ULID or unique prefix)")),
		mcp.WithString("verdict", mcp.Required(), mcp.Description("Review verdict: pass or fail")),
		mcp.WithString("summary", mcp.Required(), mcp.Description("Narrative review summary")),
		mcp.WithString("code_quality", mcp.Description("Code quality assessment: pass, fail, or skip")),
		mcp.WithString("requirements_match", mcp.Description("Requirements match assessment: pass, fail, or skip")),
		mcp.WithString("test_coverage", mcp.Description("Test coverage assessment: pass, fail, or skip")),
		mcp.WithString("ui_ux", mcp.Description("UI/UX assessment: pass, fail, skip, or na")),
		mcp.WithString("failure_reasons", mcp.Description("Newline-separated list of failure reasons (for fail verdicts)")),
		mcp.WithString("diff_stats", mcp.Description("Diff statistics string")),
	)
	return tool, s.handleSaveReview
}

func (s *Server) handleSaveReview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	issueID, err := request.RequireString("issue_id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: issue_id"), nil
	}
	verdict, err := request.RequireString("verdict")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: verdict"), nil
	}
	summary, err := request.RequireString("summary")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: summary"), nil
	}

	if verdict != "pass" && verdict != "fail" {
		return mcp.NewToolResultError("verdict must be 'pass' or 'fail'"), nil
	}

	issue, err := s.findIssue(ctx, issueID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("issue not found: %s", issueID)), nil
	}

	// Find linked session
	var sessionID string
	sessions, _ := s.store.ListAgentSessions(ctx, issue.ProjectID, 0)
	for _, sess := range sessions {
		if sess.IssueID == issue.ID {
			sessionID = sess.ID
			break
		}
	}

	// Parse failure reasons
	var failureReasons []string
	if fr := request.GetString("failure_reasons", ""); fr != "" {
		for _, line := range strings.Split(fr, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				failureReasons = append(failureReasons, line)
			}
		}
	}

	review := &models.IssueReview{
		IssueID:           issue.ID,
		SessionID:         sessionID,
		Verdict:           models.ReviewVerdict(verdict),
		Summary:           summary,
		CodeQuality:       models.ReviewCategory(request.GetString("code_quality", "skip")),
		RequirementsMatch: models.ReviewCategory(request.GetString("requirements_match", "skip")),
		TestCoverage:      models.ReviewCategory(request.GetString("test_coverage", "skip")),
		UIUX:              models.ReviewCategory(request.GetString("ui_ux", "na")),
		FailureReasons:    failureReasons,
		DiffStats:         request.GetString("diff_stats", ""),
		ReviewedAt:        time.Now().UTC(),
	}

	if err := s.store.CreateIssueReview(ctx, review); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save review: %v", err)), nil
	}

	// Transition issue status
	if verdict == "pass" {
		issue.Status = models.IssueStatusClosed
		now := time.Now().UTC()
		issue.ClosedAt = &now
	} else {
		issue.Status = models.IssueStatusInProgress
		issue.ClosedAt = nil
	}
	if err := s.store.UpdateIssue(ctx, issue); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("review saved but issue update failed: %v", err)), nil
	}

	result := map[string]any{
		"review_id":    review.ID,
		"issue_id":     issue.ID,
		"verdict":      verdict,
		"issue_status": string(issue.Status),
		"summary":      summary,
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}
```

**Step 4: Run tests**

Run: `go test ./internal/mcp/ -run TestSaveReview -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat: add pm_save_review MCP tool with issue status transitions"
```

---

### Task 6: Add `pm_update_project` MCP tool

**Files:**
- Modify: `internal/mcp/server.go`
- Test: `internal/mcp/server_test.go`

**Step 1: Write the failing test**

```go
func TestUpdateProject(t *testing.T) {
	ms := &mockStore{
		projects: []*models.Project{{ID: "p1", Name: "myproject", Path: "/tmp/myproject"}},
	}
	srv := NewServer(ms, nil, nil, nil)
	mcpSrv := srv.MCPServer()

	result, err := mcpSrv.CallTool(context.Background(), "pm_update_project", map[string]any{
		"project":    "myproject",
		"build_cmd":  "npm run build",
		"serve_cmd":  "npm run dev",
		"serve_port": float64(3000),
	})
	require.NoError(t, err)
	require.False(t, result.IsError)

	var out map[string]any
	text := result.Content[0].(mcpgo.TextContent).Text
	require.NoError(t, json.Unmarshal([]byte(text), &out))
	assert.Equal(t, "npm run build", out["build_cmd"])
	assert.Equal(t, "npm run dev", out["serve_cmd"])
	assert.Equal(t, float64(3000), out["serve_port"])
}
```

Note: The `mockStore.UpdateProject` needs to actually store the changes. Update it:

```go
func (m *mockStore) UpdateProject(_ context.Context, p *models.Project) error {
	for i, existing := range m.projects {
		if existing.ID == p.ID {
			m.projects[i] = p
			return nil
		}
	}
	return fmt.Errorf("project not found: %s", p.ID)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/mcp/ -run TestUpdateProject -v`
Expected: FAIL — `pm_update_project` not registered

**Step 3: Implement**

Register in `MCPServer()`:
```go
srv.AddTool(s.updateProjectTool())
```

```go
func (s *Server) updateProjectTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_update_project",
		mcp.WithDescription("Update project metadata. Use this to persist discovered build/serve commands for automated UI review."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name")),
		mcp.WithString("description", mcp.Description("New project description")),
		mcp.WithString("build_cmd", mcp.Description("Build command (e.g. 'npm run build', 'make ui-build')")),
		mcp.WithString("serve_cmd", mcp.Description("Dev server command (e.g. 'npm run dev', 'bun run dev')")),
		mcp.WithNumber("serve_port", mcp.Description("Dev server port (e.g. 3000, 5173)")),
	)
	return tool, s.handleUpdateProject
}

func (s *Server) handleUpdateProject(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectName, err := request.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: project"), nil
	}

	p, err := s.resolveProject(ctx, projectName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", projectName)), nil
	}

	updated := false

	if desc := request.GetString("description", ""); desc != "" {
		p.Description = desc
		updated = true
	}
	if cmd := request.GetString("build_cmd", ""); cmd != "" {
		p.BuildCmd = cmd
		updated = true
	}
	if cmd := request.GetString("serve_cmd", ""); cmd != "" {
		p.ServeCmd = cmd
		updated = true
	}
	if port := request.GetNumber("serve_port", 0); port > 0 {
		p.ServePort = int(port)
		updated = true
	}

	if !updated {
		return mcp.NewToolResultError("no fields provided to update"), nil
	}

	if err := s.store.UpdateProject(ctx, p); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update project: %v", err)), nil
	}

	result := map[string]any{
		"id":          p.ID,
		"name":        p.Name,
		"description": p.Description,
		"build_cmd":   p.BuildCmd,
		"serve_cmd":   p.ServeCmd,
		"serve_port":  p.ServePort,
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}
```

Note: Check if `mcp.WithNumber` and `request.GetNumber` exist in the mcp-go library. If not, use `mcp.WithString` for serve_port and parse with `strconv.Atoi`. The test would then pass `"serve_port": "3000"` as a string.

**Step 4: Run test**

Run: `go test ./internal/mcp/ -run TestUpdateProject -v`
Expected: PASS

**Step 5: Run all MCP tests**

Run: `go test ./internal/mcp/ -v`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/mcp/server.go internal/mcp/server_test.go
git commit -m "feat: add pm_update_project MCP tool for build/serve settings"
```

---

### Task 7: Add REST API endpoints for reviews

**Files:**
- Modify: `internal/api/api.go` (add routes + handlers)
- Test: `internal/api/api_test.go` (if exists, else create)

**Step 1: Add routes to Router()**

In `internal/api/api.go`, add to the `Router()` function:

```go
mux.HandleFunc("GET /api/v1/issues/{id}/reviews", s.listIssueReviews)
mux.HandleFunc("POST /api/v1/issues/{id}/reviews", s.createIssueReview)
```

**Step 2: Implement handlers**

```go
func (s *Server) listIssueReviews(w http.ResponseWriter, r *http.Request) {
	issueID := r.PathValue("id")
	reviews, err := s.store.ListIssueReviews(r.Context(), issueID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if reviews == nil {
		reviews = []*models.IssueReview{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reviews)
}

func (s *Server) createIssueReview(w http.ResponseWriter, r *http.Request) {
	issueID := r.PathValue("id")

	var body struct {
		Verdict           string   `json:"verdict"`
		Summary           string   `json:"summary"`
		CodeQuality       string   `json:"code_quality"`
		RequirementsMatch string   `json:"requirements_match"`
		TestCoverage      string   `json:"test_coverage"`
		UIUX              string   `json:"ui_ux"`
		FailureReasons    []string `json:"failure_reasons"`
		DiffStats         string   `json:"diff_stats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON body", http.StatusBadRequest)
		return
	}

	review := &models.IssueReview{
		IssueID:           issueID,
		Verdict:           models.ReviewVerdict(body.Verdict),
		Summary:           body.Summary,
		CodeQuality:       models.ReviewCategory(body.CodeQuality),
		RequirementsMatch: models.ReviewCategory(body.RequirementsMatch),
		TestCoverage:      models.ReviewCategory(body.TestCoverage),
		UIUX:              models.ReviewCategory(body.UIUX),
		FailureReasons:    body.FailureReasons,
		DiffStats:         body.DiffStats,
		ReviewedAt:        time.Now().UTC(),
	}

	if err := s.store.CreateIssueReview(r.Context(), review); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(review)
}
```

**Step 3: Run full test suite**

Run: `go test ./... -count=1`
Expected: All PASS

**Step 4: Commit**

```bash
git add internal/api/api.go
git commit -m "feat: add REST API endpoints for issue reviews"
```

---

### Task 8: Add `pm issue review` CLI command

**Files:**
- Modify: `cmd/issue.go`

**Step 1: Add the command**

Add a new `issueReviewCmd` cobra command to `cmd/issue.go`:

```go
var (
	reviewBaseRef string
	reviewHeadRef string
	reviewAppURL  string
)

var issueReviewCmd = &cobra.Command{
	Use:   "review <issue-id>",
	Short: "Show review context for an issue",
	Long:  "Gathers diff, requirements, and review history for AI-powered review. Outputs structured JSON context.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return issueReviewRun(args[0])
	},
}
```

Register in `init()`:
```go
issueCmd.AddCommand(issueReviewCmd)
issueReviewCmd.Flags().StringVar(&reviewBaseRef, "base-ref", "main", "Base ref for diff")
issueReviewCmd.Flags().StringVar(&reviewHeadRef, "head-ref", "", "Head ref for diff (default: session branch or HEAD)")
issueReviewCmd.Flags().StringVar(&reviewAppURL, "app-url", "", "URL of running app for UI review")
```

Implement `issueReviewRun`:

```go
func issueReviewRun(id string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	issue, err := findIssue(ctx, s, id)
	if err != nil {
		return err
	}

	// Show review history
	reviews, err := s.ListIssueReviews(ctx, issue.ID)
	if err != nil {
		return fmt.Errorf("list reviews: %w", err)
	}

	if len(reviews) == 0 {
		ui.Info("No reviews yet for issue %s: %s", output.Cyan(shortID(issue.ID)), issue.Title)
	} else {
		ui.Header("Review History")
		for _, r := range reviews {
			verdict := output.Green("PASS")
			if r.Verdict == models.ReviewVerdictFail {
				verdict = output.Red("FAIL")
			}
			fmt.Fprintf(ui.Out, "  %s  %s  %s\n", verdict, r.ReviewedAt.Format("2006-01-02 15:04"), r.Summary)
			if len(r.FailureReasons) > 0 {
				for _, reason := range r.FailureReasons {
					fmt.Fprintf(ui.Out, "    - %s\n", reason)
				}
			}
		}
		fmt.Fprintln(ui.Out)
	}

	ui.Info("Issue %s (%s) — status: %s", output.Cyan(shortID(issue.ID)), issue.Title, string(issue.Status))
	ui.Info("Use MCP tool pm_prepare_review to gather full review context")
	return nil
}
```

**Step 2: Build and verify**

Run: `go build . && ./pm issue review --help`
Expected: Shows help text for the review command

**Step 3: Commit**

```bash
git add cmd/issue.go
git commit -m "feat: add pm issue review CLI command"
```

---

### Task 9: Update TypeScript types + API hooks for reviews

**Files:**
- Modify: `ui/src/lib/types.ts`
- Modify: `ui/src/hooks/use-issues.ts` (add review hooks)
- Modify: `ui/src/hooks/use-projects.ts` (add build/serve fields)

**Step 1: Add TypeScript types**

Add to `ui/src/lib/types.ts`:

```typescript
export type ReviewVerdict = "pass" | "fail";
export type ReviewCategory = "pass" | "fail" | "skip" | "na";

export interface IssueReview {
  ID: string;
  IssueID: string;
  SessionID: string;
  Verdict: ReviewVerdict;
  Summary: string;
  CodeQuality: ReviewCategory;
  RequirementsMatch: ReviewCategory;
  TestCoverage: ReviewCategory;
  UIUX: ReviewCategory;
  FailureReasons: string[] | null;
  DiffStats: string;
  ReviewedAt: string;
  CreatedAt: string;
}
```

Update the `Project` interface to add:

```typescript
BuildCmd: string;
ServeCmd: string;
ServePort: number;
```

**Step 2: Add React Query hooks**

Add to `ui/src/hooks/use-issues.ts` (or create `ui/src/hooks/use-reviews.ts`):

```typescript
export function useIssueReviews(issueId: string) {
  return useQuery<IssueReview[]>({
    queryKey: ["issue-reviews", issueId],
    queryFn: () => fetch(`/api/v1/issues/${issueId}/reviews`).then(r => r.json()),
    enabled: !!issueId,
  });
}
```

**Step 3: Commit**

```bash
git add ui/src/lib/types.ts ui/src/hooks/
git commit -m "feat: add TypeScript types and hooks for issue reviews"
```

---

### Task 10: Add review history to issue detail in web UI

**Files:**
- Modify: `ui/src/components/issues/issues-page.tsx` (or relevant detail component)

**Step 1: Add review history section**

When an issue is expanded/shown in detail, display its review history. Show:
- Verdict badge (green "PASS" / red "FAIL")
- Review date
- Summary text
- Category breakdown (code quality, requirements, tests, UI/UX)
- Failure reasons as a list

Follow existing shadcn/ui patterns in the codebase (Badge, Card, etc.).

**Step 2: Build and verify**

Run: `cd ui && bun run build`
Expected: Builds successfully

**Step 3: Commit**

```bash
git add ui/src/components/
git commit -m "feat: add review history display to issue detail UI"
```

---

### Task 11: Add build/serve fields to project edit in web UI

**Files:**
- Modify: `ui/src/components/projects/project-form.tsx` (or relevant form component)

**Step 1: Add form fields**

Add three fields to the project edit form:
- Build Command (text input)
- Serve Command (text input)
- Serve Port (number input)

Follow existing form patterns in the codebase.

**Step 2: Build and verify**

Run: `cd ui && bun run build`
Expected: Builds successfully

**Step 3: Embed UI**

Run: `make ui-embed` (copies `ui/dist` → `internal/ui/dist`)

**Step 4: Commit**

```bash
git add ui/src/components/projects/ internal/ui/dist/
git commit -m "feat: add build/serve fields to project edit form"
```

---

### Task 12: Update MCP tool descriptions in CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Update the MCP Tools table**

Add the three new tools to the MCP Tools table in CLAUDE.md:

```markdown
| `pm_prepare_review` | Gather review context for an issue (issue_id required; opt: base_ref, head_ref, app_url) |
| `pm_save_review` | Save review verdict and transition issue (issue_id + verdict + summary required; opt: categories, failure_reasons) |
| `pm_update_project` | Update project metadata (project required; opt: description, build_cmd, serve_cmd, serve_port) |
```

Update the issue lifecycle documentation:

```markdown
- **Issue lifecycle**: open -> in_progress -> done -> [review] -> closed (pass) / in_progress (fail)
```

**Step 2: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md with review MCP tools and lifecycle"
```

---

### Task 13: Full integration test

**Step 1: Run full test suite**

Run: `make test`
Expected: All tests pass

**Step 2: Build the binary**

Run: `make build`
Expected: Clean build

**Step 3: Manual smoke test**

Run: `./pm issue review --help`
Expected: Shows help for the review command

**Step 4: Verify MCP tool registration**

Run: `echo '{"jsonrpc":"2.0","method":"tools/list","id":1}' | ./pm mcp 2>/dev/null | python3 -m json.tool | grep -E "pm_(prepare|save)_review|pm_update_project"`
Expected: All three new tools appear in the list
