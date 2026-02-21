package review

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/store"
)

func setupTestStore(t *testing.T) store.Store {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	s, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestLaunch_IssueNotFound(t *testing.T) {
	s := setupTestStore(t)
	cfg := Config{MaxAttempts: 3, AllowedTools: []string{"Read"}}
	launcher := NewLauncher(s, cfg)

	_, err := launcher.Launch(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent issue")
	}
	if got := err.Error(); got != "issue not found: nonexistent" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestLaunch_WrongStatus(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Create project and issue with status=open
	proj := &models.Project{Name: "test", Path: "/tmp/test"}
	if err := s.CreateProject(ctx, proj); err != nil {
		t.Fatal(err)
	}
	issue := &models.Issue{
		ProjectID: proj.ID,
		Title:     "Test Issue",
		Status:    models.IssueStatusOpen,
		Priority:  models.IssuePriorityMedium,
		Type:      models.IssueTypeFeature,
	}
	if err := s.CreateIssue(ctx, issue); err != nil {
		t.Fatal(err)
	}

	cfg := Config{MaxAttempts: 3, AllowedTools: []string{"Read"}}
	launcher := NewLauncher(s, cfg)
	_, err := launcher.Launch(ctx, issue.ID)
	if err == nil {
		t.Fatal("expected error for non-done issue")
	}
	if got := err.Error(); got != "issue must be in 'done' status to review (current: open)" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestLaunch_NoSession(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	proj := &models.Project{Name: "test", Path: "/tmp/test"}
	if err := s.CreateProject(ctx, proj); err != nil {
		t.Fatal(err)
	}
	issue := &models.Issue{
		ProjectID: proj.ID,
		Title:     "Test Issue",
		Status:    models.IssueStatusDone,
		Priority:  models.IssuePriorityMedium,
		Type:      models.IssueTypeFeature,
	}
	if err := s.CreateIssue(ctx, issue); err != nil {
		t.Fatal(err)
	}

	cfg := Config{MaxAttempts: 3, AllowedTools: []string{"Read"}}
	launcher := NewLauncher(s, cfg)
	_, err := launcher.Launch(ctx, issue.ID)
	if err == nil {
		t.Fatal("expected error for missing session")
	}
	if got := err.Error(); got != "no session with worktree found for issue" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestLaunch_AlreadyActive(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	// Create a real temp dir as the worktree path
	wtDir := t.TempDir()

	proj := &models.Project{Name: "test", Path: "/tmp/test"}
	if err := s.CreateProject(ctx, proj); err != nil {
		t.Fatal(err)
	}
	issue := &models.Issue{
		ProjectID: proj.ID,
		Title:     "Test Issue",
		Status:    models.IssueStatusDone,
		Priority:  models.IssuePriorityMedium,
		Type:      models.IssueTypeFeature,
	}
	if err := s.CreateIssue(ctx, issue); err != nil {
		t.Fatal(err)
	}

	sess := &models.AgentSession{
		ProjectID:    proj.ID,
		IssueID:      issue.ID,
		Branch:       "feature/test",
		WorktreePath: wtDir,
		Status:       models.SessionStatusActive,
		SessionType:  models.SessionTypeReview,
	}
	if err := s.CreateAgentSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	cfg := Config{MaxAttempts: 3, AllowedTools: []string{"Read"}}
	launcher := NewLauncher(s, cfg)
	_, err := launcher.Launch(ctx, issue.ID)
	if err == nil {
		t.Fatal("expected error for active review")
	}
	if got := err.Error(); got != "review already in progress for issue" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestBuildReviewPrompt(t *testing.T) {
	issue := &models.Issue{
		ID:          "01TESTISSUE001",
		Title:       "Add review agent",
		Description: "Implement autonomous review",
		ProjectID:   "proj1",
	}
	session := &models.AgentSession{
		ID:     "01TESTSESS001",
		Branch: "feature/review",
	}
	project := &models.Project{
		Name:     "pm",
		Language: "Go",
	}
	cfg := Config{MaxAttempts: 5}

	prompt := BuildReviewPrompt(issue, session, project, cfg)

	// Verify key content is present
	checks := []string{
		"01TESTISSUE0",
		"Add review agent",
		"pm (Go)",
		"5 attempts",
		"pm_prepare_review",
		"pm_save_review",
		"pm_sync_session",
	}
	for _, check := range checks {
		if !contains(prompt, check) {
			t.Errorf("prompt missing expected content: %q", check)
		}
	}
}

func TestBuildKickoffPrompt(t *testing.T) {
	issue := &models.Issue{
		ID:    "01TESTISSUE001",
		Title: "Add review agent",
	}
	prompt := BuildKickoffPrompt(issue)
	if !contains(prompt, "01TESTISSUE0") {
		t.Error("kickoff prompt missing short issue ID")
	}
	if !contains(prompt, "Add review agent") {
		t.Error("kickoff prompt missing issue title")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.MaxAttempts <= 0 {
		t.Errorf("expected positive MaxAttempts, got %d", cfg.MaxAttempts)
	}
	if len(cfg.AllowedTools) == 0 {
		t.Error("expected non-empty AllowedTools")
	}
}

func TestLaunch_WorktreeDirMissing(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	proj := &models.Project{Name: "test", Path: "/tmp/test"}
	if err := s.CreateProject(ctx, proj); err != nil {
		t.Fatal(err)
	}
	issue := &models.Issue{
		ProjectID: proj.ID,
		Title:     "Test Issue",
		Status:    models.IssueStatusDone,
		Priority:  models.IssuePriorityMedium,
		Type:      models.IssueTypeFeature,
	}
	if err := s.CreateIssue(ctx, issue); err != nil {
		t.Fatal(err)
	}

	// Session with nonexistent worktree path
	sess := &models.AgentSession{
		ProjectID:    proj.ID,
		IssueID:      issue.ID,
		Branch:       "feature/test",
		WorktreePath: "/tmp/nonexistent-worktree-" + time.Now().Format("20060102150405"),
		Status:       models.SessionStatusIdle,
	}
	if err := s.CreateAgentSession(ctx, sess); err != nil {
		t.Fatal(err)
	}

	cfg := Config{MaxAttempts: 3, AllowedTools: []string{"Read"}}
	launcher := NewLauncher(s, cfg)
	_, err := launcher.Launch(ctx, issue.ID)
	if err == nil {
		t.Fatal("expected error for missing worktree dir")
	}
	if got := err.Error(); got != "no session with worktree found for issue" {
		t.Errorf("unexpected error: %s", got)
	}
}

func TestLaunch_PrefixMatch(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	proj := &models.Project{Name: "test", Path: "/tmp/test"}
	if err := s.CreateProject(ctx, proj); err != nil {
		t.Fatal(err)
	}
	issue := &models.Issue{
		ProjectID: proj.ID,
		Title:     "Test Issue",
		Status:    models.IssueStatusDone,
		Priority:  models.IssuePriorityMedium,
		Type:      models.IssueTypeFeature,
	}
	if err := s.CreateIssue(ctx, issue); err != nil {
		t.Fatal(err)
	}

	// Use prefix of the issue ID
	prefix := issue.ID[:8]

	cfg := Config{MaxAttempts: 3, AllowedTools: []string{"Read"}}
	launcher := NewLauncher(s, cfg)
	_, err := launcher.Launch(ctx, prefix)
	if err == nil {
		t.Fatal("expected error (no session), but prefix should have matched the issue")
	}
	// Should get past the "issue not found" stage to "no session" error
	if got := err.Error(); got != "no session with worktree found for issue" {
		t.Errorf("expected 'no session' error, got: %s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Verify migration adds the new columns
func TestMigration_SessionTypeFields(t *testing.T) {
	s := setupTestStore(t)
	ctx := context.Background()

	proj := &models.Project{Name: "test", Path: "/tmp/test"}
	if err := s.CreateProject(ctx, proj); err != nil {
		t.Fatal(err)
	}

	// Create session with new fields
	sess := &models.AgentSession{
		ProjectID:     proj.ID,
		Branch:        "feature/test",
		WorktreePath:  "/tmp/test-wt",
		Status:        models.SessionStatusActive,
		SessionType:   models.SessionTypeReview,
		ReviewAttempt: 2,
	}
	if err := s.CreateAgentSession(ctx, sess); err != nil {
		t.Fatalf("create session with new fields: %v", err)
	}

	// Read it back
	got, err := s.GetAgentSession(ctx, sess.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.SessionType != models.SessionTypeReview {
		t.Errorf("SessionType = %q, want %q", got.SessionType, models.SessionTypeReview)
	}
	if got.ReviewAttempt != 2 {
		t.Errorf("ReviewAttempt = %d, want 2", got.ReviewAttempt)
	}

	// Verify defaults for sessions created without explicit values
	sess2 := &models.AgentSession{
		ProjectID:    proj.ID,
		Branch:       "feature/test2",
		WorktreePath: "/tmp/test-wt2",
		Status:       models.SessionStatusActive,
	}
	if err := s.CreateAgentSession(ctx, sess2); err != nil {
		t.Fatalf("create session without new fields: %v", err)
	}
	got2, err := s.GetAgentSession(ctx, sess2.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got2.SessionType != models.SessionTypeImplementation {
		t.Errorf("default SessionType = %q, want %q", got2.SessionType, models.SessionTypeImplementation)
	}
	if got2.ReviewAttempt != 0 {
		t.Errorf("default ReviewAttempt = %d, want 0", got2.ReviewAttempt)
	}

	// Verify listing also returns new fields
	sessions, err := s.ListAgentSessions(ctx, proj.ID, 0)
	if err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	foundReview := false
	for _, listed := range sessions {
		if listed.ID == sess.ID {
			if listed.SessionType != models.SessionTypeReview {
				t.Errorf("listed session SessionType = %q, want %q", listed.SessionType, models.SessionTypeReview)
			}
			foundReview = true
		}
	}
	if !foundReview {
		t.Error("review session not found in list")
	}

	// Verify update
	got.ReviewAttempt = 5
	if err := s.UpdateAgentSession(ctx, got); err != nil {
		t.Fatalf("update session: %v", err)
	}
	updated, _ := s.GetAgentSession(ctx, got.ID)
	if updated.ReviewAttempt != 5 {
		t.Errorf("after update ReviewAttempt = %d, want 5", updated.ReviewAttempt)
	}

	// Cleanup: also test that we can delete
	_ = os.Remove("/tmp/test-wt")
	_ = os.Remove("/tmp/test-wt2")
}
