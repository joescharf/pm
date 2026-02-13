package git

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTestRepo creates a git repo in dir with a user config so commits work on CI.
func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "-C", dir, "init"},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		require.NoError(t, exec.Command(args[0], args[1:]...).Run())
	}
}

func TestParseWorktreeListPorcelain(t *testing.T) {
	input := `worktree /Users/joe/projects/myrepo
HEAD abc123def456
branch refs/heads/main

worktree /Users/joe/projects/myrepo.worktrees/feature-x
HEAD def789abc012
branch refs/heads/feature/x

`
	worktrees := ParseWorktreeListPorcelain(input)
	assert.Len(t, worktrees, 2)

	assert.Equal(t, "/Users/joe/projects/myrepo", worktrees[0].Path)
	assert.Equal(t, "main", worktrees[0].Branch)
	assert.Equal(t, "abc123def456", worktrees[0].HEAD)

	assert.Equal(t, "/Users/joe/projects/myrepo.worktrees/feature-x", worktrees[1].Path)
	assert.Equal(t, "feature/x", worktrees[1].Branch)
}

func TestParseWorktreeListPorcelain_Empty(t *testing.T) {
	worktrees := ParseWorktreeListPorcelain("")
	assert.Nil(t, worktrees)
}

func TestExtractOwnerRepo_SSH(t *testing.T) {
	owner, repo, err := ExtractOwnerRepo("git@github.com:joescharf/pm.git")
	assert.NoError(t, err)
	assert.Equal(t, "joescharf", owner)
	assert.Equal(t, "pm", repo)
}

func TestExtractOwnerRepo_HTTPS(t *testing.T) {
	owner, repo, err := ExtractOwnerRepo("https://github.com/joescharf/pm.git")
	assert.NoError(t, err)
	assert.Equal(t, "joescharf", owner)
	assert.Equal(t, "pm", repo)
}

func TestExtractOwnerRepo_HTTPSNoGit(t *testing.T) {
	owner, repo, err := ExtractOwnerRepo("https://github.com/joescharf/pm")
	assert.NoError(t, err)
	assert.Equal(t, "joescharf", owner)
	assert.Equal(t, "pm", repo)
}

func TestExtractOwnerRepo_Invalid(t *testing.T) {
	_, _, err := ExtractOwnerRepo("not-a-url")
	assert.Error(t, err)
}

func TestLatestTag_NoTags(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run())

	c := NewClient()
	_, err := c.LatestTag(dir)
	assert.Error(t, err)
}

func TestLatestTag_WithTag(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "tag", "v1.0.0").Run())

	c := NewClient()
	tag, err := c.LatestTag(dir)
	assert.NoError(t, err)
	assert.Equal(t, "v1.0.0", tag)
}

func TestLatestTag_MultipleTagsReturnsLatest(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "first").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "tag", "v1.0.0").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "second").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "tag", "v2.0.0").Run())

	c := NewClient()
	tag, err := c.LatestTag(dir)
	assert.NoError(t, err)
	assert.Equal(t, "v2.0.0", tag)
}
