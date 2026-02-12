package wt

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeInfo represents a worktree from wt state.
type WorktreeInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Repo   string `json:"repo"`
}

// Client wraps the wt CLI for worktree operations.
type Client interface {
	Create(repoPath, branch string) error
	List(repoPath string) ([]WorktreeInfo, error)
	Delete(repoPath, branch string) error
}

// RealClient implements Client using the wt CLI.
type RealClient struct{}

// NewClient returns a new wt client.
func NewClient() *RealClient {
	return &RealClient{}
}

func (c *RealClient) Create(repoPath, branch string) error {
	cmd := exec.Command("wt", "open", branch)
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wt open %s: %w", branch, err)
	}
	return nil
}

func (c *RealClient) List(repoPath string) ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "-C", repoPath, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	var worktrees []WorktreeInfo
	var current WorktreeInfo

	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "branch "):
			branch := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
		case line == "":
			if current.Path != "" {
				current.Repo = repoPath
				worktrees = append(worktrees, current)
				current = WorktreeInfo{}
			}
		}
	}
	if current.Path != "" {
		current.Repo = repoPath
		worktrees = append(worktrees, current)
	}

	return worktrees, nil
}

func (c *RealClient) Delete(repoPath, branch string) error {
	cmd := exec.Command("wt", "rm", branch)
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wt rm %s: %w", branch, err)
	}
	return nil
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
		return nil, fmt.Errorf("read wt state: %w", err)
	}

	var state WtState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse wt state: %w", err)
	}
	return &state, nil
}
