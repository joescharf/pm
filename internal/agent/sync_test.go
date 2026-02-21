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

func TestReconcileSessions_ExistingWorktree_ActiveStaysActive(t *testing.T) {
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

func TestReconcileSessions_ExistingWorktree_IdleStaysIdle(t *testing.T) {
	session := &models.AgentSession{
		ID:           "sess-1",
		WorktreePath: t.TempDir(),
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

func TestReconcileSessions_AbandonedWithWorktree_RecoversToIdle(t *testing.T) {
	session := &models.AgentSession{
		ID:           "sess-1",
		WorktreePath: t.TempDir(),
		Status:       models.SessionStatusAbandoned,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{},
	}

	cleaned := ReconcileSessions(context.Background(), ms, []*models.AgentSession{session})
	assert.Equal(t, 1, cleaned)
	assert.Equal(t, models.SessionStatusIdle, ms.sessions["sess-1"].Status)
	assert.NotNil(t, ms.sessions["sess-1"].LastActiveAt)
}

func TestReconcileSessions_AbandonedWithMissingWorktree_StaysAbandoned(t *testing.T) {
	session := &models.AgentSession{
		ID:           "sess-1",
		WorktreePath: "/nonexistent/path",
		Status:       models.SessionStatusAbandoned,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{"sess-1": session},
		issues:   map[string]*models.Issue{},
	}

	cleaned := ReconcileSessions(context.Background(), ms, []*models.AgentSession{session})
	assert.Equal(t, 0, cleaned)
	assert.Equal(t, models.SessionStatusAbandoned, ms.sessions["sess-1"].Status)
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

// mockProcessDetector implements ProcessDetector for testing.
type mockProcessDetector struct {
	activePaths map[string]bool
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
	detector := &mockProcessDetector{activePaths: map[string]bool{}}

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

func TestReconcileSessions_AbandonedNotRecovered_WhenBranchHasLiveSession(t *testing.T) {
	dir := t.TempDir()
	idleSess := &models.AgentSession{
		ID:           "sess-live",
		ProjectID:    "proj-1",
		Branch:       "feature/foo",
		WorktreePath: dir,
		Status:       models.SessionStatusIdle,
	}
	abandonedSess := &models.AgentSession{
		ID:           "sess-dup",
		ProjectID:    "proj-1",
		Branch:       "feature/foo",
		WorktreePath: dir,
		Status:       models.SessionStatusAbandoned,
	}
	ms := &mockSessionStore{
		sessions: map[string]*models.AgentSession{
			"sess-live": idleSess,
			"sess-dup":  abandonedSess,
		},
		issues: map[string]*models.Issue{},
	}

	sessions := []*models.AgentSession{idleSess, abandonedSess}
	cleaned := ReconcileSessions(context.Background(), ms, sessions)
	assert.Equal(t, 0, cleaned)
	assert.Equal(t, models.SessionStatusIdle, ms.sessions["sess-live"].Status)
	assert.Equal(t, models.SessionStatusAbandoned, ms.sessions["sess-dup"].Status)
}
