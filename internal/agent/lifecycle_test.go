package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/joescharf/pm/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSessionStore implements SessionStore using in-memory maps.
type mockSessionStore struct {
	sessions map[string]*models.AgentSession
	issues   map[string]*models.Issue
}

func (m *mockSessionStore) GetAgentSession(_ context.Context, id string) (*models.AgentSession, error) {
	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	return s, nil
}

func (m *mockSessionStore) UpdateAgentSession(_ context.Context, session *models.AgentSession) error {
	if _, ok := m.sessions[session.ID]; !ok {
		return fmt.Errorf("session %s not found", session.ID)
	}
	m.sessions[session.ID] = session
	return nil
}

func (m *mockSessionStore) GetIssue(_ context.Context, id string) (*models.Issue, error) {
	i, ok := m.issues[id]
	if !ok {
		return nil, fmt.Errorf("issue %s not found", id)
	}
	return i, nil
}

func (m *mockSessionStore) UpdateIssue(_ context.Context, issue *models.Issue) error {
	if _, ok := m.issues[issue.ID]; !ok {
		return fmt.Errorf("issue %s not found", issue.ID)
	}
	m.issues[issue.ID] = issue
	return nil
}

func newMockStore() *mockSessionStore {
	return &mockSessionStore{
		sessions: make(map[string]*models.AgentSession),
		issues:   make(map[string]*models.Issue),
	}
}

func TestCloseSession_Idle(t *testing.T) {
	store := newMockStore()
	store.sessions["sess-1"] = &models.AgentSession{
		ID:      "sess-1",
		IssueID: "issue-1",
		Status:  models.SessionStatusActive,
	}
	store.issues["issue-1"] = &models.Issue{
		ID:     "issue-1",
		Status: models.IssueStatusInProgress,
	}

	ctx := context.Background()
	session, err := CloseSession(ctx, store, "sess-1", models.SessionStatusIdle)
	require.NoError(t, err)

	assert.Equal(t, models.SessionStatusIdle, session.Status)
	assert.Nil(t, session.EndedAt, "idle sessions should not have EndedAt set")

	// Issue should remain in_progress when session goes idle
	issue := store.issues["issue-1"]
	assert.Equal(t, models.IssueStatusInProgress, issue.Status)
}

func TestCloseSession_Completed(t *testing.T) {
	store := newMockStore()
	store.sessions["sess-2"] = &models.AgentSession{
		ID:      "sess-2",
		IssueID: "issue-2",
		Status:  models.SessionStatusActive,
	}
	store.issues["issue-2"] = &models.Issue{
		ID:     "issue-2",
		Status: models.IssueStatusInProgress,
	}

	ctx := context.Background()
	session, err := CloseSession(ctx, store, "sess-2", models.SessionStatusCompleted)
	require.NoError(t, err)

	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	assert.NotNil(t, session.EndedAt, "completed sessions must have EndedAt set")

	// Issue should cascade to done
	issue := store.issues["issue-2"]
	assert.Equal(t, models.IssueStatusDone, issue.Status)
}

func TestCloseSession_Abandoned(t *testing.T) {
	store := newMockStore()
	store.sessions["sess-3"] = &models.AgentSession{
		ID:      "sess-3",
		IssueID: "issue-3",
		Status:  models.SessionStatusActive,
	}
	store.issues["issue-3"] = &models.Issue{
		ID:     "issue-3",
		Status: models.IssueStatusInProgress,
	}

	ctx := context.Background()
	session, err := CloseSession(ctx, store, "sess-3", models.SessionStatusAbandoned)
	require.NoError(t, err)

	assert.Equal(t, models.SessionStatusAbandoned, session.Status)
	assert.NotNil(t, session.EndedAt, "abandoned sessions must have EndedAt set")

	// Issue should cascade back to open
	issue := store.issues["issue-3"]
	assert.Equal(t, models.IssueStatusOpen, issue.Status)
}

func TestCloseSession_NoIssue(t *testing.T) {
	store := newMockStore()
	store.sessions["sess-4"] = &models.AgentSession{
		ID:      "sess-4",
		IssueID: "", // no linked issue
		Status:  models.SessionStatusActive,
	}

	ctx := context.Background()
	session, err := CloseSession(ctx, store, "sess-4", models.SessionStatusCompleted)
	require.NoError(t, err)

	assert.Equal(t, models.SessionStatusCompleted, session.Status)
	assert.NotNil(t, session.EndedAt, "completed sessions must have EndedAt set")
}

func TestCloseSession_AlreadyClosed(t *testing.T) {
	store := newMockStore()
	store.sessions["sess-5"] = &models.AgentSession{
		ID:     "sess-5",
		Status: models.SessionStatusCompleted, // already closed
	}

	ctx := context.Background()
	_, err := CloseSession(ctx, store, "sess-5", models.SessionStatusCompleted)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already completed")
}
