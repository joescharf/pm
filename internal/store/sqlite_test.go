package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joescharf/pm/internal/models"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)

	err = s.Migrate(context.Background())
	require.NoError(t, err)

	t.Cleanup(func() { s.Close() })
	return s
}

func TestNewSQLiteStore_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "subdir", "test.db")

	s, err := NewSQLiteStore(dbPath)
	require.NoError(t, err)
	defer s.Close()

	_, err = os.Stat(filepath.Join(dir, "subdir"))
	assert.NoError(t, err, "should create parent directory")
}

func TestMigrate_Idempotent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Running migrate again should be a no-op
	err := s.Migrate(ctx)
	assert.NoError(t, err)
}

// --- Project CRUD ---

func TestProjectCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create
	p := &models.Project{
		Name:        "test-project",
		Path:        "/tmp/test-project",
		Description: "A test project",
		RepoURL:     "https://github.com/test/test",
		Language:    "go",
		GroupName:   "testing",
	}
	err := s.CreateProject(ctx, p)
	require.NoError(t, err)
	assert.NotEmpty(t, p.ID)
	assert.False(t, p.CreatedAt.IsZero())

	// Get by ID
	got, err := s.GetProject(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, p.Name, got.Name)
	assert.Equal(t, p.Path, got.Path)
	assert.Equal(t, p.Description, got.Description)
	assert.Equal(t, p.Language, got.Language)

	// Get by Name
	got, err = s.GetProjectByName(ctx, "test-project")
	require.NoError(t, err)
	assert.Equal(t, p.ID, got.ID)

	// Get by Path
	got, err = s.GetProjectByPath(ctx, "/tmp/test-project")
	require.NoError(t, err)
	assert.Equal(t, p.ID, got.ID)

	// Update
	got.Description = "Updated description"
	err = s.UpdateProject(ctx, got)
	require.NoError(t, err)

	got2, err := s.GetProject(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated description", got2.Description)

	// List
	projects, err := s.ListProjects(ctx, "")
	require.NoError(t, err)
	assert.Len(t, projects, 1)

	// List by group
	projects, err = s.ListProjects(ctx, "testing")
	require.NoError(t, err)
	assert.Len(t, projects, 1)

	projects, err = s.ListProjects(ctx, "nonexistent")
	require.NoError(t, err)
	assert.Len(t, projects, 0)

	// Delete
	err = s.DeleteProject(ctx, p.ID)
	require.NoError(t, err)

	_, err = s.GetProject(ctx, p.ID)
	assert.Error(t, err)
}

func TestProjectUniqueConstraints(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p1 := &models.Project{Name: "dup", Path: "/tmp/dup1"}
	require.NoError(t, s.CreateProject(ctx, p1))

	// Duplicate name
	p2 := &models.Project{Name: "dup", Path: "/tmp/dup2"}
	err := s.CreateProject(ctx, p2)
	assert.Error(t, err)

	// Duplicate path
	p3 := &models.Project{Name: "diff", Path: "/tmp/dup1"}
	err = s.CreateProject(ctx, p3)
	assert.Error(t, err)
}

// --- Issue CRUD ---

func TestIssueCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create project first
	p := &models.Project{Name: "proj", Path: "/tmp/proj"}
	require.NoError(t, s.CreateProject(ctx, p))

	// Create issue
	issue := &models.Issue{
		ProjectID:   p.ID,
		Title:       "Fix bug",
		Description: "Something is broken",
		Status:      models.IssueStatusOpen,
		Priority:    models.IssuePriorityHigh,
		Type:        models.IssueTypeBug,
		GitHubIssue: 42,
	}
	err := s.CreateIssue(ctx, issue)
	require.NoError(t, err)
	assert.NotEmpty(t, issue.ID)

	// Get
	got, err := s.GetIssue(ctx, issue.ID)
	require.NoError(t, err)
	assert.Equal(t, "Fix bug", got.Title)
	assert.Equal(t, models.IssueStatusOpen, got.Status)
	assert.Equal(t, models.IssuePriorityHigh, got.Priority)
	assert.Equal(t, models.IssueTypeBug, got.Type)
	assert.Equal(t, 42, got.GitHubIssue)

	// Update
	got.Status = models.IssueStatusInProgress
	err = s.UpdateIssue(ctx, got)
	require.NoError(t, err)

	got2, err := s.GetIssue(ctx, issue.ID)
	require.NoError(t, err)
	assert.Equal(t, models.IssueStatusInProgress, got2.Status)

	// List with filter
	issues, err := s.ListIssues(ctx, IssueListFilter{ProjectID: p.ID})
	require.NoError(t, err)
	assert.Len(t, issues, 1)

	issues, err = s.ListIssues(ctx, IssueListFilter{Status: models.IssueStatusOpen})
	require.NoError(t, err)
	assert.Len(t, issues, 0) // We changed to in_progress

	issues, err = s.ListIssues(ctx, IssueListFilter{Status: models.IssueStatusInProgress})
	require.NoError(t, err)
	assert.Len(t, issues, 1)

	issues, err = s.ListIssues(ctx, IssueListFilter{Priority: models.IssuePriorityHigh})
	require.NoError(t, err)
	assert.Len(t, issues, 1)

	// Delete
	err = s.DeleteIssue(ctx, issue.ID)
	require.NoError(t, err)

	_, err = s.GetIssue(ctx, issue.ID)
	assert.Error(t, err)
}

func TestIssueCascadeDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p := &models.Project{Name: "proj", Path: "/tmp/proj"}
	require.NoError(t, s.CreateProject(ctx, p))

	issue := &models.Issue{
		ProjectID: p.ID,
		Title:     "Test",
		Status:    models.IssueStatusOpen,
		Priority:  models.IssuePriorityMedium,
		Type:      models.IssueTypeFeature,
	}
	require.NoError(t, s.CreateIssue(ctx, issue))

	// Deleting project should cascade to issues
	require.NoError(t, s.DeleteProject(ctx, p.ID))

	_, err := s.GetIssue(ctx, issue.ID)
	assert.Error(t, err)
}

// --- Tag Operations ---

func TestTagOperations(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create tags
	tag1 := &models.Tag{Name: "v1"}
	require.NoError(t, s.CreateTag(ctx, tag1))

	tag2 := &models.Tag{Name: "urgent"}
	require.NoError(t, s.CreateTag(ctx, tag2))

	// List
	tags, err := s.ListTags(ctx)
	require.NoError(t, err)
	assert.Len(t, tags, 2)

	// Create project and issue
	p := &models.Project{Name: "proj", Path: "/tmp/proj"}
	require.NoError(t, s.CreateProject(ctx, p))

	issue := &models.Issue{
		ProjectID: p.ID,
		Title:     "Tagged issue",
		Status:    models.IssueStatusOpen,
		Priority:  models.IssuePriorityMedium,
		Type:      models.IssueTypeFeature,
	}
	require.NoError(t, s.CreateIssue(ctx, issue))

	// Tag issue
	require.NoError(t, s.TagIssue(ctx, issue.ID, tag1.ID))
	require.NoError(t, s.TagIssue(ctx, issue.ID, tag2.ID))

	// Get issue tags
	issueTags, err := s.GetIssueTags(ctx, issue.ID)
	require.NoError(t, err)
	assert.Len(t, issueTags, 2)

	// TagIssue is idempotent
	require.NoError(t, s.TagIssue(ctx, issue.ID, tag1.ID))

	// Untag
	require.NoError(t, s.UntagIssue(ctx, issue.ID, tag1.ID))
	issueTags, err = s.GetIssueTags(ctx, issue.ID)
	require.NoError(t, err)
	assert.Len(t, issueTags, 1)

	// List issues by tag
	issues, err := s.ListIssues(ctx, IssueListFilter{Tag: "urgent"})
	require.NoError(t, err)
	assert.Len(t, issues, 1)

	issues, err = s.ListIssues(ctx, IssueListFilter{Tag: "v1"})
	require.NoError(t, err)
	assert.Len(t, issues, 0) // We removed the v1 tag

	// Delete tag
	require.NoError(t, s.DeleteTag(ctx, tag1.ID))
	tags, err = s.ListTags(ctx)
	require.NoError(t, err)
	assert.Len(t, tags, 1)
}

// --- Agent Sessions ---

func TestAgentSessionCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create project
	p := &models.Project{Name: "proj", Path: "/tmp/proj"}
	require.NoError(t, s.CreateProject(ctx, p))

	// Create session
	session := &models.AgentSession{
		ProjectID:    p.ID,
		IssueID:      "",
		Branch:       "feature/test",
		WorktreePath: "/tmp/proj-wt/feature-test",
		Status:       models.SessionStatusActive,
	}
	err := s.CreateAgentSession(ctx, session)
	require.NoError(t, err)
	assert.NotEmpty(t, session.ID)

	// List
	sessions, err := s.ListAgentSessions(ctx, p.ID, 10)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, models.SessionStatusActive, sessions[0].Status)

	// Update
	now := time.Now().UTC()
	session.Status = models.SessionStatusCompleted
	session.Outcome = "All tests passed"
	session.CommitCount = 3
	session.EndedAt = &now
	err = s.UpdateAgentSession(ctx, session)
	require.NoError(t, err)

	sessions, err = s.ListAgentSessions(ctx, p.ID, 10)
	require.NoError(t, err)
	assert.Equal(t, models.SessionStatusCompleted, sessions[0].Status)
	assert.Equal(t, 3, sessions[0].CommitCount)
	assert.NotNil(t, sessions[0].EndedAt)

	// List with limit
	session2 := &models.AgentSession{
		ProjectID: p.ID,
		Branch:    "feature/other",
		Status:    models.SessionStatusActive,
	}
	require.NoError(t, s.CreateAgentSession(ctx, session2))

	sessions, err = s.ListAgentSessions(ctx, p.ID, 1)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)

	// List all
	sessions, err = s.ListAgentSessions(ctx, "", 0)
	require.NoError(t, err)
	assert.Len(t, sessions, 2)
}

func TestGetAgentSession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p := &models.Project{Name: "test-proj", Path: "/tmp/test-proj"}
	require.NoError(t, s.CreateProject(ctx, p))

	session := &models.AgentSession{
		ProjectID:    p.ID,
		IssueID:      "issue-1",
		Branch:       "feature/test",
		WorktreePath: "/tmp/test-proj-feature-test",
		Status:       models.SessionStatusActive,
	}
	require.NoError(t, s.CreateAgentSession(ctx, session))

	got, err := s.GetAgentSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, session.ID, got.ID)
	assert.Equal(t, models.SessionStatusActive, got.Status)
	assert.Equal(t, "/tmp/test-proj-feature-test", got.WorktreePath)

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

	got, err := s.GetAgentSessionByWorktreePath(ctx, "/tmp/test-proj-feature-test")
	require.NoError(t, err)
	assert.Equal(t, session.ID, got.ID)

	_, err = s.GetAgentSessionByWorktreePath(ctx, "/nonexistent")
	assert.Error(t, err)
}

func TestGetProject_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, err := s.GetProject(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDeleteProject_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	err := s.DeleteProject(ctx, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUpdateProject_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p := &models.Project{ID: "nonexistent", Name: "test", Path: "/tmp/test"}
	err := s.UpdateProject(ctx, p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestListIssues_SortedByStatusThenPriority(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p := &models.Project{Name: "sort-test", Path: "/tmp/sort"}
	require.NoError(t, s.CreateProject(ctx, p))

	// Create issues in deliberate disorder
	issues := []struct {
		title    string
		status   models.IssueStatus
		priority models.IssuePriority
	}{
		{"closed-low", models.IssueStatusClosed, models.IssuePriorityLow},
		{"open-low", models.IssueStatusOpen, models.IssuePriorityLow},
		{"open-high", models.IssueStatusOpen, models.IssuePriorityHigh},
		{"in-progress-medium", models.IssueStatusInProgress, models.IssuePriorityMedium},
		{"open-medium", models.IssueStatusOpen, models.IssuePriorityMedium},
		{"done-high", models.IssueStatusDone, models.IssuePriorityHigh},
	}

	for _, iss := range issues {
		i := &models.Issue{
			ProjectID: p.ID,
			Title:     iss.title,
			Status:    iss.status,
			Priority:  iss.priority,
			Type:      models.IssueTypeFeature,
		}
		require.NoError(t, s.CreateIssue(ctx, i))
		time.Sleep(5 * time.Millisecond) // ensure distinct created_at
	}

	result, err := s.ListIssues(ctx, IssueListFilter{ProjectID: p.ID})
	require.NoError(t, err)
	require.Len(t, result, 6)

	titles := make([]string, len(result))
	for i, r := range result {
		titles[i] = r.Title
	}

	// Expected: open (high, medium, low) -> in_progress -> done -> closed
	assert.Equal(t, "open-high", titles[0])
	assert.Equal(t, "open-medium", titles[1])
	assert.Equal(t, "open-low", titles[2])
	assert.Equal(t, "in-progress-medium", titles[3])
	assert.Equal(t, "done-high", titles[4])
	assert.Equal(t, "closed-low", titles[5])
}
