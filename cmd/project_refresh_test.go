package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/output"
	"github.com/joescharf/pm/internal/refresh"
	"github.com/joescharf/pm/internal/store"
)

// mockGitClient implements git.Client for testing.
type mockGitClient struct {
	remoteURL string
}

func (m *mockGitClient) RepoRoot(path string) (string, error)          { return path, nil }
func (m *mockGitClient) CurrentBranch(path string) (string, error)     { return "main", nil }
func (m *mockGitClient) LastCommitDate(path string) (time.Time, error) { return time.Now(), nil }
func (m *mockGitClient) LastCommitMessage(path string) (string, error) { return "msg", nil }
func (m *mockGitClient) LastCommitHash(path string) (string, error)    { return "abc123", nil }
func (m *mockGitClient) BranchList(path string) ([]string, error)      { return []string{"main"}, nil }
func (m *mockGitClient) IsDirty(path string) (bool, error)             { return false, nil }
func (m *mockGitClient) WorktreeList(path string) ([]git.WorktreeInfo, error) {
	return nil, nil
}
func (m *mockGitClient) RemoteURL(path string) (string, error) { return m.remoteURL, nil }
func (m *mockGitClient) LatestTag(path string) (string, error) { return "", nil }
func (m *mockGitClient) CommitCountSince(path, base string) (int, error) { return 0, nil }
func (m *mockGitClient) AheadBehind(path, base string) (int, int, error)         { return 0, 0, nil }
func (m *mockGitClient) Diff(path, base, head string) (string, error)            { return "", nil }
func (m *mockGitClient) DiffStat(path, base, head string) (string, error)        { return "", nil }
func (m *mockGitClient) DiffNameOnly(path, base, head string) ([]string, error)  { return nil, nil }

// mockGitHubClient implements git.GitHubClient for testing.
type mockGitHubClient struct {
	repoInfo  *git.RepoInfo
	pagesInfo *git.PagesResult
}

func (m *mockGitHubClient) LatestRelease(owner, repo string) (*git.Release, error) {
	return nil, nil
}
func (m *mockGitHubClient) OpenPRs(owner, repo string) ([]git.PullRequest, error) {
	return nil, nil
}
func (m *mockGitHubClient) RepoInfo(owner, repo string) (*git.RepoInfo, error) {
	if m.repoInfo != nil {
		return m.repoInfo, nil
	}
	return nil, nil
}
func (m *mockGitHubClient) PagesInfo(owner, repo string) (*git.PagesResult, error) {
	if m.pagesInfo != nil {
		return m.pagesInfo, nil
	}
	return nil, nil
}

// refreshTestEnv sets up a store and UI for refresh tests.
func refreshTestEnv(t *testing.T) store.Store {
	t.Helper()
	dir := t.TempDir()

	viper.Reset()
	viper.SetDefault("db_path", filepath.Join(dir, "pm.db"))

	ui = output.New()
	ui.Verbose = true

	s, err := store.NewSQLiteStore(filepath.Join(dir, "pm.db"))
	require.NoError(t, err)
	require.NoError(t, s.Migrate(context.Background()))
	t.Cleanup(func() { _ = s.Close() })

	return s
}

func TestRefreshProject_UpdatesLanguage(t *testing.T) {
	s := refreshTestEnv(t)
	ctx := context.Background()

	projDir := t.TempDir()
	p := &models.Project{Name: "test", Path: projDir, Language: "python"}
	require.NoError(t, s.CreateProject(ctx, p))

	// Create go.mod so DetectLanguage returns "go"
	require.NoError(t, os.WriteFile(filepath.Join(projDir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644))

	gc := &mockGitClient{remoteURL: ""}
	ghc := &mockGitHubClient{}

	changed, err := refresh.Project(ctx, s, p, gc, ghc)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "go", p.Language)

	// Verify persisted
	got, err := s.GetProject(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "go", got.Language)
}

func TestRefreshProject_UpdatesRepoURL(t *testing.T) {
	s := refreshTestEnv(t)
	ctx := context.Background()

	p := &models.Project{Name: "test", Path: t.TempDir(), RepoURL: "https://github.com/old/repo"}
	require.NoError(t, s.CreateProject(ctx, p))

	gc := &mockGitClient{remoteURL: "https://github.com/new/repo"}
	ghc := &mockGitHubClient{}

	changed, err := refresh.Project(ctx, s, p, gc, ghc)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "https://github.com/new/repo", p.RepoURL)

	got, err := s.GetProject(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/new/repo", got.RepoURL)
}

func TestRefreshProject_FillsEmptyDescription(t *testing.T) {
	s := refreshTestEnv(t)
	ctx := context.Background()

	p := &models.Project{
		Name:        "test",
		Path:        t.TempDir(),
		RepoURL:     "https://github.com/owner/repo",
		Description: "",
	}
	require.NoError(t, s.CreateProject(ctx, p))

	gc := &mockGitClient{remoteURL: "https://github.com/owner/repo"}
	ghc := &mockGitHubClient{
		repoInfo: &git.RepoInfo{Description: "A cool project", Language: "Go"},
	}

	changed, err := refresh.Project(ctx, s, p, gc, ghc)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "A cool project", p.Description)
}

func TestRefreshProject_SyncsDescriptionFromGitHub(t *testing.T) {
	s := refreshTestEnv(t)
	ctx := context.Background()

	p := &models.Project{
		Name:        "test",
		Path:        t.TempDir(),
		RepoURL:     "https://github.com/owner/repo",
		Description: "My custom description",
	}
	require.NoError(t, s.CreateProject(ctx, p))

	gc := &mockGitClient{remoteURL: "https://github.com/owner/repo"}
	ghc := &mockGitHubClient{
		repoInfo: &git.RepoInfo{Description: "GitHub description"},
	}

	changed, err := refresh.Project(ctx, s, p, gc, ghc)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "GitHub description", p.Description)
}

func TestRefreshProject_NoChanges(t *testing.T) {
	s := refreshTestEnv(t)
	ctx := context.Background()

	p := &models.Project{
		Name:        "test",
		Path:        t.TempDir(),
		Description: "existing",
		BranchCount: 1, // matches mock's BranchList return of ["main"]
	}
	require.NoError(t, s.CreateProject(ctx, p))

	gc := &mockGitClient{remoteURL: ""}
	ghc := &mockGitHubClient{}

	changed, err := refresh.Project(ctx, s, p, gc, ghc)
	require.NoError(t, err)
	assert.False(t, changed)
}

func TestRefreshProject_GitHubLanguageFallback(t *testing.T) {
	s := refreshTestEnv(t)
	ctx := context.Background()

	p := &models.Project{
		Name:    "test",
		Path:    t.TempDir(),
		RepoURL: "https://github.com/owner/repo",
	}
	require.NoError(t, s.CreateProject(ctx, p))

	gc := &mockGitClient{remoteURL: "https://github.com/owner/repo"}
	ghc := &mockGitHubClient{
		repoInfo: &git.RepoInfo{Language: "Rust"},
	}

	changed, err := refresh.Project(ctx, s, p, gc, ghc)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "Rust", p.Language)
}

func TestRefreshProject_DetectsGitHubPages(t *testing.T) {
	s := refreshTestEnv(t)
	ctx := context.Background()

	p := &models.Project{
		Name:    "test",
		Path:    t.TempDir(),
		RepoURL: "https://github.com/owner/repo",
	}
	require.NoError(t, s.CreateProject(ctx, p))

	gc := &mockGitClient{remoteURL: "https://github.com/owner/repo"}
	ghc := &mockGitHubClient{
		pagesInfo: &git.PagesResult{URL: "https://test.github.io"},
	}

	changed, err := refresh.Project(ctx, s, p, gc, ghc)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.True(t, p.HasGitHubPages)
	assert.Equal(t, "https://test.github.io", p.PagesURL)

	// Verify persisted
	got, err := s.GetProject(ctx, p.ID)
	require.NoError(t, err)
	assert.True(t, got.HasGitHubPages)
	assert.Equal(t, "https://test.github.io", got.PagesURL)
}
