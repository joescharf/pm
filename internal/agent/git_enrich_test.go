package agent

import (
	"testing"
	"time"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/models"
	"github.com/stretchr/testify/assert"
)

// mockGitClient implements git.Client for testing EnrichSessionWithGitInfo.
type mockGitClient struct {
	lastCommitHash    string
	lastCommitMessage string
}

func (m *mockGitClient) RepoRoot(path string) (string, error)          { return path, nil }
func (m *mockGitClient) CurrentBranch(path string) (string, error)     { return "main", nil }
func (m *mockGitClient) LastCommitDate(path string) (time.Time, error) { return time.Now(), nil }
func (m *mockGitClient) LastCommitMessage(path string) (string, error) {
	return m.lastCommitMessage, nil
}
func (m *mockGitClient) LastCommitHash(path string) (string, error) {
	return m.lastCommitHash, nil
}
func (m *mockGitClient) BranchList(path string) ([]string, error)                { return nil, nil }
func (m *mockGitClient) IsDirty(path string) (bool, error)                       { return false, nil }
func (m *mockGitClient) WorktreeList(path string) ([]git.WorktreeInfo, error)     { return nil, nil }
func (m *mockGitClient) RemoteURL(path string) (string, error)                   { return "", nil }
func (m *mockGitClient) LatestTag(path string) (string, error)                   { return "", nil }
func (m *mockGitClient) CommitCountSince(path, base string) (int, error)         { return 0, nil }
func (m *mockGitClient) AheadBehind(path, base string) (int, int, error)         { return 0, 0, nil }
func (m *mockGitClient) Diff(path, base, head string) (string, error)           { return "", nil }
func (m *mockGitClient) DiffStat(path, base, head string) (string, error)       { return "", nil }
func (m *mockGitClient) DiffNameOnly(path, base, head string) ([]string, error) { return nil, nil }

func TestEnrichSessionWithGitInfo_SetsFields(t *testing.T) {
	session := &models.AgentSession{
		ID:           "sess-1",
		WorktreePath: "/tmp/some-worktree",
		Status:       models.SessionStatusActive,
	}

	gc := &mockGitClient{
		lastCommitHash:    "abc1234",
		lastCommitMessage: "feat: add new feature",
	}

	before := time.Now().UTC()
	EnrichSessionWithGitInfo(session, gc)
	after := time.Now().UTC()

	assert.Equal(t, "abc1234", session.LastCommitHash)
	assert.Equal(t, "feat: add new feature", session.LastCommitMessage)
	assert.NotNil(t, session.LastActiveAt)
	assert.False(t, session.LastActiveAt.Before(before), "LastActiveAt should be >= before")
	assert.False(t, session.LastActiveAt.After(after), "LastActiveAt should be <= after")
}

func TestEnrichSessionWithGitInfo_NilClient(t *testing.T) {
	session := &models.AgentSession{
		ID:           "sess-2",
		WorktreePath: "/tmp/some-worktree",
		Status:       models.SessionStatusActive,
	}

	// Pass nil git client — should return early without panic
	EnrichSessionWithGitInfo(session, nil)

	assert.Empty(t, session.LastCommitHash)
	assert.Empty(t, session.LastCommitMessage)
	assert.Nil(t, session.LastActiveAt)
}

func TestEnrichSessionWithGitInfo_EmptyWorktreePath(t *testing.T) {
	session := &models.AgentSession{
		ID:           "sess-3",
		WorktreePath: "", // empty
		Status:       models.SessionStatusActive,
	}

	gc := &mockGitClient{
		lastCommitHash:    "abc1234",
		lastCommitMessage: "feat: add new feature",
	}

	// Should return early without modifying session
	EnrichSessionWithGitInfo(session, gc)

	assert.Empty(t, session.LastCommitHash)
	assert.Empty(t, session.LastCommitMessage)
	assert.Nil(t, session.LastActiveAt)
}
