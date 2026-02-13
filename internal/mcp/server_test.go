package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/store"
	"github.com/joescharf/pm/internal/wt"
)

// ---------------------------------------------------------------------------
// Mock implementations
// ---------------------------------------------------------------------------

// mockStore implements store.Store for testing.
type mockStore struct {
	projects []*models.Project
	issues   []*models.Issue
	sessions []*models.AgentSession
	tags     []*models.Tag

	// Track calls for verification.
	createdIssues   []*models.Issue
	updatedIssues   []*models.Issue
	createdSessions []*models.AgentSession

	// Optional error injection.
	listProjectsErr error
	listIssuesErr   error
	createIssueErr  error
	getIssueErr     error
	updateIssueErr  error
}

func (m *mockStore) CreateProject(_ context.Context, p *models.Project) error {
	m.projects = append(m.projects, p)
	return nil
}
func (m *mockStore) GetProject(_ context.Context, id string) (*models.Project, error) {
	for _, p := range m.projects {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, fmt.Errorf("project not found: %s", id)
}
func (m *mockStore) GetProjectByName(_ context.Context, name string) (*models.Project, error) {
	for _, p := range m.projects {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, fmt.Errorf("project not found: %s", name)
}
func (m *mockStore) GetProjectByPath(_ context.Context, path string) (*models.Project, error) {
	for _, p := range m.projects {
		if p.Path == path {
			return p, nil
		}
	}
	return nil, fmt.Errorf("project not found by path: %s", path)
}
func (m *mockStore) ListProjects(_ context.Context, group string) ([]*models.Project, error) {
	if m.listProjectsErr != nil {
		return nil, m.listProjectsErr
	}
	if group == "" {
		return m.projects, nil
	}
	var filtered []*models.Project
	for _, p := range m.projects {
		if p.GroupName == group {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}
func (m *mockStore) UpdateProject(_ context.Context, _ *models.Project) error { return nil }
func (m *mockStore) DeleteProject(_ context.Context, _ string) error          { return nil }

func (m *mockStore) CreateIssue(_ context.Context, issue *models.Issue) error {
	if m.createIssueErr != nil {
		return m.createIssueErr
	}
	if issue.ID == "" {
		issue.ID = fmt.Sprintf("issue-%d", len(m.issues)+1)
	}
	issue.CreatedAt = time.Now()
	issue.UpdatedAt = time.Now()
	m.issues = append(m.issues, issue)
	m.createdIssues = append(m.createdIssues, issue)
	return nil
}
func (m *mockStore) GetIssue(_ context.Context, id string) (*models.Issue, error) {
	if m.getIssueErr != nil {
		return nil, m.getIssueErr
	}
	for _, i := range m.issues {
		if i.ID == id {
			return i, nil
		}
	}
	return nil, fmt.Errorf("issue not found: %s", id)
}
func (m *mockStore) ListIssues(_ context.Context, filter store.IssueListFilter) ([]*models.Issue, error) {
	if m.listIssuesErr != nil {
		return nil, m.listIssuesErr
	}
	var result []*models.Issue
	for _, i := range m.issues {
		if filter.ProjectID != "" && i.ProjectID != filter.ProjectID {
			continue
		}
		if filter.Status != "" && i.Status != filter.Status {
			continue
		}
		if filter.Priority != "" && i.Priority != filter.Priority {
			continue
		}
		if filter.Type != "" && i.Type != filter.Type {
			continue
		}
		result = append(result, i)
	}
	return result, nil
}
func (m *mockStore) UpdateIssue(_ context.Context, issue *models.Issue) error {
	if m.updateIssueErr != nil {
		return m.updateIssueErr
	}
	for idx, i := range m.issues {
		if i.ID == issue.ID {
			m.issues[idx] = issue
			m.updatedIssues = append(m.updatedIssues, issue)
			return nil
		}
	}
	return fmt.Errorf("issue not found: %s", issue.ID)
}
func (m *mockStore) DeleteIssue(_ context.Context, _ string) error { return nil }

func (m *mockStore) CreateTag(_ context.Context, tag *models.Tag) error {
	m.tags = append(m.tags, tag)
	return nil
}
func (m *mockStore) ListTags(_ context.Context) ([]*models.Tag, error) { return m.tags, nil }
func (m *mockStore) DeleteTag(_ context.Context, _ string) error       { return nil }
func (m *mockStore) TagIssue(_ context.Context, _, _ string) error     { return nil }
func (m *mockStore) UntagIssue(_ context.Context, _, _ string) error   { return nil }
func (m *mockStore) GetIssueTags(_ context.Context, _ string) ([]*models.Tag, error) {
	return nil, nil
}

func (m *mockStore) CreateAgentSession(_ context.Context, session *models.AgentSession) error {
	if session.ID == "" {
		session.ID = fmt.Sprintf("session-%d", len(m.sessions)+1)
	}
	m.sessions = append(m.sessions, session)
	m.createdSessions = append(m.createdSessions, session)
	return nil
}
func (m *mockStore) ListAgentSessions(_ context.Context, projectID string, limit int) ([]*models.AgentSession, error) {
	var result []*models.AgentSession
	for _, s := range m.sessions {
		if projectID != "" && s.ProjectID != projectID {
			continue
		}
		result = append(result, s)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}
func (m *mockStore) UpdateAgentSession(_ context.Context, _ *models.AgentSession) error { return nil }
func (m *mockStore) Migrate(_ context.Context) error                                    { return nil }
func (m *mockStore) Close() error                                                       { return nil }

// mockGitClient implements git.Client for testing.
type mockGitClient struct {
	branch     string
	dirty      bool
	lastCommit time.Time
	commitMsg  string
	commitHash string
	branches   []string
	remoteURL  string
	latestTag  string

	// Error injection.
	currentBranchErr error
}

func (m *mockGitClient) RepoRoot(_ string) (string, error) { return "/mock/repo", nil }
func (m *mockGitClient) CurrentBranch(_ string) (string, error) {
	if m.currentBranchErr != nil {
		return "", m.currentBranchErr
	}
	return m.branch, nil
}
func (m *mockGitClient) LastCommitDate(_ string) (time.Time, error) { return m.lastCommit, nil }
func (m *mockGitClient) LastCommitMessage(_ string) (string, error) { return m.commitMsg, nil }
func (m *mockGitClient) LastCommitHash(_ string) (string, error)    { return m.commitHash, nil }
func (m *mockGitClient) BranchList(_ string) ([]string, error)      { return m.branches, nil }
func (m *mockGitClient) IsDirty(_ string) (bool, error)             { return m.dirty, nil }
func (m *mockGitClient) WorktreeList(_ string) ([]git.WorktreeInfo, error) {
	return nil, nil
}
func (m *mockGitClient) RemoteURL(_ string) (string, error) { return m.remoteURL, nil }
func (m *mockGitClient) LatestTag(_ string) (string, error) { return m.latestTag, nil }

// mockGHClient implements git.GitHubClient for testing.
type mockGHClient struct {
	release  *git.Release
	prs      []git.PullRequest
	repoInfo *git.RepoInfo
}

func (m *mockGHClient) LatestRelease(_, _ string) (*git.Release, error) {
	if m.release == nil {
		return nil, fmt.Errorf("no release")
	}
	return m.release, nil
}
func (m *mockGHClient) OpenPRs(_, _ string) ([]git.PullRequest, error) { return m.prs, nil }
func (m *mockGHClient) RepoInfo(_, _ string) (*git.RepoInfo, error) {
	if m.repoInfo == nil {
		return nil, fmt.Errorf("no repo info")
	}
	return m.repoInfo, nil
}

// mockWTClient implements wt.Client for testing.
type mockWTClient struct {
	created   []struct{ repo, branch string }
	worktrees []wt.WorktreeInfo
	createErr error
}

func (m *mockWTClient) Create(repoPath, branch string) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, struct{ repo, branch string }{repoPath, branch})
	return nil
}
func (m *mockWTClient) List(_ string) ([]wt.WorktreeInfo, error) { return m.worktrees, nil }
func (m *mockWTClient) Delete(_, _ string) error                 { return nil }

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestServer creates a Server with mock dependencies and seed data.
func newTestServer(t *testing.T) (*Server, *mockStore, *mockGitClient, *mockGHClient, *mockWTClient) {
	t.Helper()

	ms := &mockStore{}
	gc := &mockGitClient{
		branch:     "main",
		dirty:      false,
		lastCommit: time.Now().Add(-2 * time.Hour),
		commitMsg:  "feat: add new feature",
		commitHash: "abc1234",
		branches:   []string{"main", "develop"},
		remoteURL:  "https://github.com/testorg/testrepo.git",
		latestTag:  "v1.2.0",
	}
	ghc := &mockGHClient{}
	wtc := &mockWTClient{}

	srv := NewServer(ms, gc, ghc, wtc)
	require.NotNil(t, srv)

	return srv, ms, gc, ghc, wtc
}

// callToolReq builds a mcpgo.CallToolRequest with the given name and arguments.
func callToolReq(name string, args map[string]any) mcpgo.CallToolRequest {
	return mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      name,
			Arguments: args,
		},
	}
}

// resultText extracts the concatenated text from a CallToolResult.
func resultText(t *testing.T, result *mcpgo.CallToolResult) string {
	t.Helper()
	var b strings.Builder
	for _, c := range result.Content {
		tc, ok := c.(mcpgo.TextContent)
		if ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// resultJSON parses the text result as JSON into the provided target.
func resultJSON(t *testing.T, result *mcpgo.CallToolResult, target any) {
	t.Helper()
	text := resultText(t, result)
	err := json.Unmarshal([]byte(text), target)
	require.NoError(t, err, "failed to parse result JSON: %s", text)
}

// seedProject adds a project to the mock store and returns it.
func seedProject(t *testing.T, ms *mockStore, name, path string) *models.Project {
	t.Helper()
	p := &models.Project{
		ID:        fmt.Sprintf("proj-%s", name),
		Name:      name,
		Path:      path,
		Language:  "go",
		GroupName: "default",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	ms.projects = append(ms.projects, p)
	return p
}

// seedIssue adds an issue to the mock store and returns it.
func seedIssue(t *testing.T, ms *mockStore, projectID, title string, status models.IssueStatus) *models.Issue {
	t.Helper()
	i := &models.Issue{
		ID:        fmt.Sprintf("issue-%s-%d", title, len(ms.issues)+1),
		ProjectID: projectID,
		Title:     title,
		Status:    status,
		Priority:  models.IssuePriorityMedium,
		Type:      models.IssueTypeFeature,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	ms.issues = append(ms.issues, i)
	return i
}

// ---------------------------------------------------------------------------
// Tests: MCPServer registration
// ---------------------------------------------------------------------------

func TestNewServer(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	mcpSrv := srv.MCPServer()
	require.NotNil(t, mcpSrv, "MCPServer() should return non-nil")
}

// ---------------------------------------------------------------------------
// Tests: pm_list_projects
// ---------------------------------------------------------------------------

func TestHandleListProjects_Empty(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	ctx := context.Background()

	req := callToolReq("pm_list_projects", nil)
	result, err := srv.handleListProjects(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	assert.NotEmpty(t, text, "should return some output even with no projects")
}

func TestHandleListProjects_WithProjects(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	seedProject(t, ms, "alpha", "/tmp/alpha")
	seedProject(t, ms, "beta", "/tmp/beta")

	req := callToolReq("pm_list_projects", nil)
	result, err := srv.handleListProjects(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	assert.Contains(t, text, "alpha")
	assert.Contains(t, text, "beta")
}

func TestHandleListProjects_WithGroupFilter(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	p1 := seedProject(t, ms, "alpha", "/tmp/alpha")
	p1.GroupName = "team-a"
	p2 := seedProject(t, ms, "beta", "/tmp/beta")
	p2.GroupName = "team-b"

	req := callToolReq("pm_list_projects", map[string]any{"group": "team-a"})
	result, err := srv.handleListProjects(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	assert.Contains(t, text, "alpha")
	assert.NotContains(t, text, "beta")
}

func TestHandleListProjects_StoreError(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	ms.listProjectsErr = fmt.Errorf("db connection failed")

	req := callToolReq("pm_list_projects", nil)
	result, err := srv.handleListProjects(ctx, req)
	require.NoError(t, err, "handler should not return Go error; should wrap in result")
	require.NotNil(t, result)
	assert.True(t, result.IsError)

	text := resultText(t, result)
	assert.Contains(t, text, "db connection failed")
}

// ---------------------------------------------------------------------------
// Tests: pm_project_status
// ---------------------------------------------------------------------------

func TestHandleProjectStatus(t *testing.T) {
	srv, ms, gc, _, _ := newTestServer(t)
	ctx := context.Background()

	seedProject(t, ms, "myapp", "/tmp/myapp")
	gc.branch = "feature/login"
	gc.dirty = true
	gc.commitHash = "deadbeef"
	gc.commitMsg = "wip: login page"

	req := callToolReq("pm_project_status", map[string]any{"project": "myapp"})
	result, err := srv.handleProjectStatus(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	assert.Contains(t, text, "myapp")
	assert.Contains(t, text, "feature/login")
}

func TestHandleProjectStatus_MissingProject(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	ctx := context.Background()

	req := callToolReq("pm_project_status", map[string]any{"project": "nonexistent"})
	result, err := srv.handleProjectStatus(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestHandleProjectStatus_NoProjectArg(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	ctx := context.Background()

	req := callToolReq("pm_project_status", nil)
	result, err := srv.handleProjectStatus(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should error when project argument is missing")
}

// ---------------------------------------------------------------------------
// Tests: pm_list_issues
// ---------------------------------------------------------------------------

func TestHandleListIssues_All(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	seedIssue(t, ms, p.ID, "Fix login bug", models.IssueStatusOpen)
	seedIssue(t, ms, p.ID, "Add dark mode", models.IssueStatusInProgress)

	req := callToolReq("pm_list_issues", nil)
	result, err := srv.handleListIssues(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	assert.Contains(t, text, "Fix login bug")
	assert.Contains(t, text, "Add dark mode")
}

func TestHandleListIssues_FilterByProject(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	p1 := seedProject(t, ms, "app-a", "/tmp/app-a")
	p2 := seedProject(t, ms, "app-b", "/tmp/app-b")
	seedIssue(t, ms, p1.ID, "Issue A", models.IssueStatusOpen)
	seedIssue(t, ms, p2.ID, "Issue B", models.IssueStatusOpen)

	req := callToolReq("pm_list_issues", map[string]any{"project": "app-a"})
	result, err := srv.handleListIssues(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	assert.Contains(t, text, "Issue A")
	assert.NotContains(t, text, "Issue B")
}

func TestHandleListIssues_FilterByStatus(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	seedIssue(t, ms, p.ID, "Open issue", models.IssueStatusOpen)
	seedIssue(t, ms, p.ID, "Done issue", models.IssueStatusDone)

	req := callToolReq("pm_list_issues", map[string]any{"status": "open"})
	result, err := srv.handleListIssues(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	assert.Contains(t, text, "Open issue")
	assert.NotContains(t, text, "Done issue")
}

func TestHandleListIssues_FilterByPriority(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	i1 := seedIssue(t, ms, p.ID, "High priority", models.IssueStatusOpen)
	i1.Priority = models.IssuePriorityHigh
	i2 := seedIssue(t, ms, p.ID, "Low priority", models.IssueStatusOpen)
	i2.Priority = models.IssuePriorityLow

	req := callToolReq("pm_list_issues", map[string]any{"priority": "high"})
	result, err := srv.handleListIssues(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	assert.Contains(t, text, "High priority")
	assert.NotContains(t, text, "Low priority")
}

func TestHandleListIssues_Empty(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	ctx := context.Background()

	req := callToolReq("pm_list_issues", nil)
	result, err := srv.handleListIssues(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestHandleListIssues_StoreError(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	ms.listIssuesErr = fmt.Errorf("database locked")

	req := callToolReq("pm_list_issues", nil)
	result, err := srv.handleListIssues(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "database locked")
}

// ---------------------------------------------------------------------------
// Tests: pm_create_issue
// ---------------------------------------------------------------------------

func TestHandleCreateIssue(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	seedProject(t, ms, "myapp", "/tmp/myapp")

	req := callToolReq("pm_create_issue", map[string]any{
		"project":     "myapp",
		"title":       "Implement caching",
		"description": "Add Redis caching layer",
		"priority":    "high",
		"type":        "feature",
	})

	result, err := srv.handleCreateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Verify issue was created in store.
	require.Len(t, ms.createdIssues, 1)
	created := ms.createdIssues[0]
	assert.Equal(t, "Implement caching", created.Title)
	assert.Equal(t, "Add Redis caching layer", created.Description)
	assert.Equal(t, models.IssuePriorityHigh, created.Priority)
	assert.Equal(t, models.IssueTypeFeature, created.Type)
	assert.Equal(t, models.IssueStatusOpen, created.Status)
}

func TestHandleCreateIssue_DefaultPriorityAndType(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	seedProject(t, ms, "myapp", "/tmp/myapp")

	req := callToolReq("pm_create_issue", map[string]any{
		"project": "myapp",
		"title":   "Quick fix",
	})

	result, err := srv.handleCreateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	require.Len(t, ms.createdIssues, 1)
	created := ms.createdIssues[0]
	assert.Equal(t, "Quick fix", created.Title)
	// Default values should be applied.
	assert.Equal(t, models.IssuePriorityMedium, created.Priority)
	assert.Equal(t, models.IssueTypeFeature, created.Type)
}

func TestHandleCreateIssue_MissingTitle(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	seedProject(t, ms, "myapp", "/tmp/myapp")

	req := callToolReq("pm_create_issue", map[string]any{
		"project": "myapp",
	})

	result, err := srv.handleCreateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should error when title is missing")
}

func TestHandleCreateIssue_MissingProject(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	ctx := context.Background()

	req := callToolReq("pm_create_issue", map[string]any{
		"title": "Some issue",
	})

	result, err := srv.handleCreateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should error when project is missing")
}

func TestHandleCreateIssue_UnknownProject(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	ctx := context.Background()

	req := callToolReq("pm_create_issue", map[string]any{
		"project": "nonexistent",
		"title":   "Some issue",
	})

	result, err := srv.handleCreateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestHandleCreateIssue_StoreError(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	seedProject(t, ms, "myapp", "/tmp/myapp")
	ms.createIssueErr = fmt.Errorf("disk full")

	req := callToolReq("pm_create_issue", map[string]any{
		"project": "myapp",
		"title":   "Some issue",
	})

	result, err := srv.handleCreateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "disk full")
}

// ---------------------------------------------------------------------------
// Tests: pm_update_issue
// ---------------------------------------------------------------------------

func TestHandleUpdateIssue_ChangeStatus(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	issue := seedIssue(t, ms, p.ID, "Fix bug", models.IssueStatusOpen)

	req := callToolReq("pm_update_issue", map[string]any{
		"issue_id": issue.ID,
		"status":   "in_progress",
	})

	result, err := srv.handleUpdateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	require.Len(t, ms.updatedIssues, 1)
	assert.Equal(t, models.IssueStatusInProgress, ms.updatedIssues[0].Status)
}

func TestHandleUpdateIssue_ChangePriority(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	issue := seedIssue(t, ms, p.ID, "Feature X", models.IssueStatusOpen)

	req := callToolReq("pm_update_issue", map[string]any{
		"issue_id": issue.ID,
		"priority": "high",
	})

	result, err := srv.handleUpdateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	require.Len(t, ms.updatedIssues, 1)
	assert.Equal(t, models.IssuePriorityHigh, ms.updatedIssues[0].Priority)
}

func TestHandleUpdateIssue_ChangeTitle(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	issue := seedIssue(t, ms, p.ID, "Old title", models.IssueStatusOpen)

	req := callToolReq("pm_update_issue", map[string]any{
		"issue_id": issue.ID,
		"title":    "New title",
	})

	result, err := srv.handleUpdateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	require.Len(t, ms.updatedIssues, 1)
	assert.Equal(t, "New title", ms.updatedIssues[0].Title)
}

func TestHandleUpdateIssue_MissingID(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	ctx := context.Background()

	req := callToolReq("pm_update_issue", map[string]any{
		"status": "done",
	})

	result, err := srv.handleUpdateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should error when issue ID is missing")
}

func TestHandleUpdateIssue_IssueNotFound(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	ctx := context.Background()

	req := callToolReq("pm_update_issue", map[string]any{
		"issue_id": "nonexistent-id",
		"status":   "done",
	})

	result, err := srv.handleUpdateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestHandleUpdateIssue_CloseIssue(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	issue := seedIssue(t, ms, p.ID, "To close", models.IssueStatusOpen)

	req := callToolReq("pm_update_issue", map[string]any{
		"issue_id": issue.ID,
		"status":   "closed",
	})

	result, err := srv.handleUpdateIssue(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	require.Len(t, ms.updatedIssues, 1)
	assert.Equal(t, models.IssueStatusClosed, ms.updatedIssues[0].Status)
}

// ---------------------------------------------------------------------------
// Tests: pm_health_score
// ---------------------------------------------------------------------------

func TestHandleHealthScore(t *testing.T) {
	srv, ms, gc, _, _ := newTestServer(t)
	ctx := context.Background()

	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	seedIssue(t, ms, p.ID, "Open bug", models.IssueStatusOpen)
	seedIssue(t, ms, p.ID, "Done feature", models.IssueStatusDone)

	gc.dirty = false
	gc.lastCommit = time.Now().Add(-1 * time.Hour)
	gc.branches = []string{"main"}
	gc.latestTag = "v1.0.0"

	req := callToolReq("pm_health_score", map[string]any{"project": "myapp"})
	result, err := srv.handleHealthScore(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	// Should contain score information.
	assert.Contains(t, text, "myapp")
	// The output should mention the total score or numeric value.
	assert.Regexp(t, `\d+`, text, "should contain a numeric score")
}

func TestHandleHealthScore_MissingProject(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	ctx := context.Background()

	req := callToolReq("pm_health_score", nil)
	result, err := srv.handleHealthScore(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should error when project is missing")
}

func TestHandleHealthScore_UnknownProject(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	ctx := context.Background()

	req := callToolReq("pm_health_score", map[string]any{"project": "ghost"})
	result, err := srv.handleHealthScore(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
}

func TestHandleHealthScore_DirtyRepo(t *testing.T) {
	srv, ms, gc, _, _ := newTestServer(t)
	ctx := context.Background()

	seedProject(t, ms, "myapp", "/tmp/myapp")

	gc.dirty = true
	gc.lastCommit = time.Now().Add(-1 * time.Hour)
	gc.branches = []string{"main", "feat-a", "feat-b", "feat-c", "fix-d", "fix-e"}

	req := callToolReq("pm_health_score", map[string]any{"project": "myapp"})
	result, err := srv.handleHealthScore(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	text := resultText(t, result)
	assert.Regexp(t, `\d+`, text, "should contain numeric score")
}

// ---------------------------------------------------------------------------
// Tests: pm_launch_agent
// ---------------------------------------------------------------------------

func TestHandleLaunchAgent(t *testing.T) {
	srv, ms, _, _, wtc := newTestServer(t)
	ctx := context.Background()

	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	issue := seedIssue(t, ms, p.ID, "Implement feature X", models.IssueStatusOpen)

	req := callToolReq("pm_launch_agent", map[string]any{
		"project": "myapp",
		"issue_id": issue.ID,
	})

	result, err := srv.handleLaunchAgent(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	// Verify worktree creation was requested.
	require.Len(t, wtc.created, 1)
	assert.Equal(t, "/tmp/myapp", wtc.created[0].repo)

	// Verify agent session was recorded.
	require.Len(t, ms.createdSessions, 1)
	session := ms.createdSessions[0]
	assert.Equal(t, p.ID, session.ProjectID)
	assert.Equal(t, issue.ID, session.IssueID)
	assert.Equal(t, models.SessionStatusActive, session.Status)
}

func TestHandleLaunchAgent_MissingProject(t *testing.T) {
	srv, _, _, _, _ := newTestServer(t)
	ctx := context.Background()

	req := callToolReq("pm_launch_agent", map[string]any{
		"issue_id": "some-issue-id",
	})

	result, err := srv.handleLaunchAgent(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should error when project is missing")
}

func TestHandleLaunchAgent_MissingIssue(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)
	ctx := context.Background()

	seedProject(t, ms, "myapp", "/tmp/myapp")

	req := callToolReq("pm_launch_agent", map[string]any{
		"project": "myapp",
		"issue_id": "nonexistent-issue",
	})

	result, err := srv.handleLaunchAgent(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "should error when issue is not found")
}

func TestHandleLaunchAgent_WorktreeCreateFails(t *testing.T) {
	srv, ms, _, _, wtc := newTestServer(t)
	ctx := context.Background()

	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	issue := seedIssue(t, ms, p.ID, "Some issue", models.IssueStatusOpen)

	wtc.createErr = fmt.Errorf("branch already exists")

	req := callToolReq("pm_launch_agent", map[string]any{
		"project": "myapp",
		"issue_id": issue.ID,
	})

	result, err := srv.handleLaunchAgent(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	assert.Contains(t, resultText(t, result), "branch already exists")
}

func TestHandleLaunchAgent_WithCustomBranch(t *testing.T) {
	srv, ms, _, _, wtc := newTestServer(t)
	ctx := context.Background()

	p := seedProject(t, ms, "myapp", "/tmp/myapp")
	issue := seedIssue(t, ms, p.ID, "Custom branch issue", models.IssueStatusOpen)

	req := callToolReq("pm_launch_agent", map[string]any{
		"project": "myapp",
		"issue_id": issue.ID,
		"branch":  "custom/my-branch",
	})

	result, err := srv.handleLaunchAgent(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError)

	require.Len(t, wtc.created, 1)
	assert.Equal(t, "custom/my-branch", wtc.created[0].branch)
}

// ---------------------------------------------------------------------------
// Tests: Integration -- verify all tools are registered via HandleMessage
// ---------------------------------------------------------------------------

func TestMCPIntegration_ListTools(t *testing.T) {
	srv, ms, _, _, _ := newTestServer(t)

	seedProject(t, ms, "demo", "/tmp/demo")

	mcpSrv := srv.MCPServer()
	require.NotNil(t, mcpSrv)

	// Call tools/list via HandleMessage to verify registration.
	ctx := context.Background()
	reqJSON := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	respMsg := mcpSrv.HandleMessage(ctx, reqJSON)
	require.NotNil(t, respMsg)

	respBytes, err := json.Marshal(respMsg)
	require.NoError(t, err)

	var rpcResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	err = json.Unmarshal(respBytes, &rpcResp)
	require.NoError(t, err)

	toolNames := make(map[string]bool)
	for _, tool := range rpcResp.Result.Tools {
		toolNames[tool.Name] = true
	}

	expectedTools := []string{
		"pm_list_projects",
		"pm_project_status",
		"pm_list_issues",
		"pm_create_issue",
		"pm_update_issue",
		"pm_health_score",
		"pm_launch_agent",
	}
	for _, name := range expectedTools {
		assert.True(t, toolNames[name], "expected tool %q to be registered", name)
	}
}

// Compile-time interface checks for mocks.
var (
	_ store.Store        = (*mockStore)(nil)
	_ git.Client         = (*mockGitClient)(nil)
	_ git.GitHubClient   = (*mockGHClient)(nil)
	_ wt.Client          = (*mockWTClient)(nil)
)

// Reference mcpserver to keep the import active (used by MCPServer return type).
var _ = (*mcpserver.MCPServer)(nil)
