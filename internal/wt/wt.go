package wt

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/joescharf/wt/pkg/claude"
	"github.com/joescharf/wt/pkg/gitops"
	"github.com/joescharf/wt/pkg/iterm"
	"github.com/joescharf/wt/pkg/lifecycle"
	"github.com/joescharf/wt/pkg/wtstate"
)

// WorktreeInfo represents a worktree.
type WorktreeInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Repo   string `json:"repo"`
}

// Client wraps the wt lifecycle for worktree operations.
type Client interface {
	Create(repoPath, branch string) error
	List(repoPath string) ([]WorktreeInfo, error)
	Delete(repoPath, branch string) error
	Lifecycle() *lifecycle.Manager
	LifecycleForRepo(repoPath string) *lifecycle.Manager
}

// RealClient implements Client using wt library packages.
type RealClient struct {
	itermClient iterm.Client
	stateMgr    *wtstate.Manager
	trustMgr    *claude.TrustManager
}

// NewClient returns a new wt client backed by library packages.
func NewClient() *RealClient {
	itermClient := iterm.NewClient()

	home, _ := os.UserHomeDir()
	statePath := filepath.Join(home, ".config", "wt", "state.json")
	stateMgr := wtstate.NewManager(statePath)

	claudePath, _ := claude.DefaultPath()
	var trustMgr *claude.TrustManager
	if claudePath != "" {
		trustMgr = claude.NewTrustManager(claudePath)
	}

	return &RealClient{
		itermClient: itermClient,
		stateMgr:    stateMgr,
		trustMgr:    trustMgr,
	}
}

func (c *RealClient) Create(repoPath, branch string) error {
	git := newRepoBoundGitopsClient(repoPath)
	lm := lifecycle.NewManager(git, c.itermClient, c.stateMgr, c.trustMgr, nil)
	_, err := lm.Create(context.Background(), lifecycle.CreateOptions{
		Branch: branch,
	})
	return err
}

func (c *RealClient) List(repoPath string) ([]WorktreeInfo, error) {
	git := newRepoBoundGitopsClient(repoPath)
	worktrees, err := git.WorktreeList()
	if err != nil {
		return nil, err
	}

	var result []WorktreeInfo
	for _, wt := range worktrees {
		result = append(result, WorktreeInfo{
			Path:   wt.Path,
			Branch: wt.Branch,
			Repo:   repoPath,
		})
	}
	return result, nil
}

func (c *RealClient) Delete(repoPath, branch string) error {
	git := newRepoBoundGitopsClient(repoPath)
	// Resolve branch to worktree path
	wtDir := repoPath + ".worktrees"
	dirname := gitops.BranchToDirname(branch)
	wtPath := filepath.Join(wtDir, dirname)

	lm := lifecycle.NewManager(git, c.itermClient, c.stateMgr, c.trustMgr, nil)
	return lm.Delete(context.Background(), wtPath, lifecycle.DeleteOptions{
		Force: true,
	})
}

// Lifecycle returns a lifecycle.Manager with shared iTerm/state/trust dependencies
// but NO git client (caller must provide repo-specific context).
func (c *RealClient) Lifecycle() *lifecycle.Manager {
	return lifecycle.NewManager(nil, c.itermClient, c.stateMgr, c.trustMgr, nil)
}

// LifecycleForRepo returns a lifecycle.Manager bound to a specific repo.
func (c *RealClient) LifecycleForRepo(repoPath string) *lifecycle.Manager {
	git := newRepoBoundGitopsClient(repoPath)
	return lifecycle.NewManager(git, c.itermClient, c.stateMgr, c.trustMgr, nil)
}

// WtState represents the wt state.json file.
type WtState struct {
	Worktrees []WtStateEntry `json:"worktrees"`
}

// WtStateEntry is a single entry from wt state.json.
type WtStateEntry struct {
	Branch string `json:"branch"`
	Path   string `json:"path"`
	Repo   string `json:"repo"`
}

// StateReader reads wt state from its state file.
type StateReader struct {
	statePath string
}

// NewStateReader creates a reader for the wt state file.
func NewStateReader() *StateReader {
	home, _ := os.UserHomeDir()
	return &StateReader{
		statePath: filepath.Join(home, ".config", "wt", "state.json"),
	}
}

// LoadState reads and parses the wt state.json.
func (r *StateReader) LoadState() (*WtState, error) {
	data, err := os.ReadFile(r.statePath)
	if err != nil {
		return nil, err
	}

	var state WtState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}
