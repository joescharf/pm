package git

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// WorktreeInfo holds parsed worktree metadata from `git worktree list --porcelain`.
type WorktreeInfo struct {
	Path   string
	Branch string
	HEAD   string
}

// Client defines the interface for git operations on arbitrary repos.
// All methods take a path parameter since pm operates on multiple repos.
type Client interface {
	RepoRoot(path string) (string, error)
	CurrentBranch(path string) (string, error)
	LastCommitDate(path string) (time.Time, error)
	LastCommitMessage(path string) (string, error)
	LastCommitHash(path string) (string, error)
	BranchList(path string) ([]string, error)
	IsDirty(path string) (bool, error)
	WorktreeList(path string) ([]WorktreeInfo, error)
	RemoteURL(path string) (string, error)
	LatestTag(path string) (string, error)
}

// RealClient implements Client using real git commands.
type RealClient struct{}

// NewClient returns a new RealClient.
func NewClient() *RealClient {
	return &RealClient{}
}

func gitCmd(path string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", path}, args...)
	out, err := exec.Command("git", fullArgs...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *RealClient) RepoRoot(path string) (string, error) {
	return gitCmd(path, "rev-parse", "--show-toplevel")
}

func (c *RealClient) CurrentBranch(path string) (string, error) {
	return gitCmd(path, "rev-parse", "--abbrev-ref", "HEAD")
}

func (c *RealClient) LastCommitDate(path string) (time.Time, error) {
	out, err := gitCmd(path, "log", "-1", "--format=%aI")
	if err != nil {
		return time.Time{}, err
	}
	return time.Parse(time.RFC3339, out)
}

func (c *RealClient) LastCommitMessage(path string) (string, error) {
	return gitCmd(path, "log", "-1", "--format=%s")
}

func (c *RealClient) LastCommitHash(path string) (string, error) {
	return gitCmd(path, "log", "-1", "--format=%h")
}

func (c *RealClient) BranchList(path string) ([]string, error) {
	out, err := gitCmd(path, "branch", "--format=%(refname:short)")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	var branches []string
	for _, line := range strings.Split(out, "\n") {
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

func (c *RealClient) IsDirty(path string) (bool, error) {
	out, err := gitCmd(path, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (c *RealClient) WorktreeList(path string) ([]WorktreeInfo, error) {
	out, err := gitCmd(path, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return ParseWorktreeListPorcelain(out), nil
}

func (c *RealClient) RemoteURL(path string) (string, error) {
	out, err := gitCmd(path, "remote", "get-url", "origin")
	if err != nil {
		return "", nil // no remote is not an error
	}
	return out, nil
}

func (c *RealClient) LatestTag(path string) (string, error) {
	return gitCmd(path, "describe", "--tags", "--abbrev=0")
}

// ParseWorktreeListPorcelain parses the output of `git worktree list --porcelain`.
func ParseWorktreeListPorcelain(output string) []WorktreeInfo {
	var worktrees []WorktreeInfo
	var current WorktreeInfo

	for _, line := range strings.Split(output, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			branch := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(branch, "refs/heads/")
		case line == "":
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = WorktreeInfo{}
			}
		}
	}
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}
	return worktrees
}

// ExtractOwnerRepo parses a GitHub remote URL and returns owner/repo.
func ExtractOwnerRepo(remoteURL string) (owner, repo string, err error) {
	// Handle SSH: git@github.com:owner/repo.git
	if strings.HasPrefix(remoteURL, "git@") {
		parts := strings.SplitN(remoteURL, ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("cannot parse SSH remote: %s", remoteURL)
		}
		path := strings.TrimSuffix(parts[1], ".git")
		segments := strings.SplitN(path, "/", 2)
		if len(segments) != 2 {
			return "", "", fmt.Errorf("cannot parse owner/repo from: %s", remoteURL)
		}
		return segments[0], segments[1], nil
	}

	// Handle HTTPS: https://github.com/owner/repo.git
	trimmed := strings.TrimSuffix(remoteURL, ".git")
	trimmed = strings.TrimPrefix(trimmed, "https://github.com/")
	trimmed = strings.TrimPrefix(trimmed, "http://github.com/")
	segments := strings.SplitN(trimmed, "/", 2)
	if len(segments) != 2 || segments[0] == "" || segments[1] == "" {
		return "", "", fmt.Errorf("cannot parse owner/repo from: %s", remoteURL)
	}
	return segments[0], segments[1], nil
}
