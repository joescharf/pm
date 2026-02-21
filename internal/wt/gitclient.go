package wt

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joescharf/wt/pkg/gitops"
)

// repoBoundGitopsClient implements gitops.Client for a specific repository path.
type repoBoundGitopsClient struct {
	repoPath string
}

func newRepoBoundGitopsClient(repoPath string) *repoBoundGitopsClient {
	return &repoBoundGitopsClient{repoPath: repoPath}
}

func (c *repoBoundGitopsClient) git(args ...string) (string, error) {
	fullArgs := append([]string{"-C", c.repoPath}, args...)
	out, err := exec.Command("git", fullArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *repoBoundGitopsClient) gitAt(path string, args ...string) (string, error) {
	fullArgs := append([]string{"-C", path}, args...)
	out, err := exec.Command("git", fullArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *repoBoundGitopsClient) RepoRoot() (string, error) { return c.repoPath, nil }
func (c *repoBoundGitopsClient) RepoName() (string, error) { return filepath.Base(c.repoPath), nil }
func (c *repoBoundGitopsClient) WorktreesDir() (string, error) {
	return c.repoPath + ".worktrees", nil
}

func (c *repoBoundGitopsClient) WorktreeList() ([]gitops.WorktreeInfo, error) {
	out, err := c.git("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return gitops.ParseWorktreeListPorcelain(out), nil
}

func (c *repoBoundGitopsClient) WorktreeAdd(path, branch, base string, newBranch bool) error {
	var args []string
	if newBranch {
		args = []string{"-C", c.repoPath, "worktree", "add", "-b", branch, path, base}
	} else {
		args = []string{"-C", c.repoPath, "worktree", "add", path, branch}
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *repoBoundGitopsClient) WorktreeRemove(path string, force bool) error {
	args := []string{"-C", c.repoPath, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *repoBoundGitopsClient) BranchExists(branch string) (bool, error) {
	err := exec.Command("git", "-C", c.repoPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branch).Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *repoBoundGitopsClient) BranchDelete(branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := c.git("branch", flag, branch)
	return err
}

func (c *repoBoundGitopsClient) CurrentBranch(worktreePath string) (string, error) {
	return c.gitAt(worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
}

func (c *repoBoundGitopsClient) ResolveWorktree(input string) (string, error) {
	wtDir := c.repoPath + ".worktrees"
	return gitops.ResolveWorktreePath(input, wtDir)
}

// Methods below implement the full gitops.Client interface.
// Some are used by lifecycle/ops, others are stubs for interface compliance.

func (c *repoBoundGitopsClient) BranchList() ([]string, error) {
	out, err := c.git("branch", "--format=%(refname:short)")
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

func (c *repoBoundGitopsClient) IsWorktreeDirty(path string) (bool, error) {
	out, err := c.gitAt(path, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (c *repoBoundGitopsClient) HasUnpushedCommits(path, base string) (bool, error) {
	out, err := c.gitAt(path, "log", base+"..HEAD", "--oneline")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (c *repoBoundGitopsClient) WorktreePrune() error {
	_, err := c.git("worktree", "prune")
	return err
}

func (c *repoBoundGitopsClient) Merge(repoPath, branch string) error {
	out, err := exec.Command("git", "-C", repoPath, "merge", branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("merge failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *repoBoundGitopsClient) MergeContinue(repoPath string) error {
	out, err := exec.Command("git", "-C", repoPath, "merge", "--continue").CombinedOutput()
	if err != nil {
		return fmt.Errorf("merge --continue failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *repoBoundGitopsClient) IsMergeInProgress(repoPath string) (bool, error) {
	_, err := c.gitAt(repoPath, "rev-parse", "MERGE_HEAD")
	return err == nil, nil
}

func (c *repoBoundGitopsClient) HasConflicts(repoPath string) (bool, error) {
	out, err := c.gitAt(repoPath, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (c *repoBoundGitopsClient) Rebase(repoPath, branch string) error {
	out, err := exec.Command("git", "-C", repoPath, "rebase", branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebase failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *repoBoundGitopsClient) RebaseContinue(repoPath string) error {
	out, err := exec.Command("git", "-C", repoPath, "rebase", "--continue").CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebase --continue failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *repoBoundGitopsClient) RebaseAbort(repoPath string) error {
	out, err := exec.Command("git", "-C", repoPath, "rebase", "--abort").CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebase --abort failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *repoBoundGitopsClient) IsRebaseInProgress(repoPath string) (bool, error) {
	return false, nil
}

func (c *repoBoundGitopsClient) Pull(repoPath string) error {
	out, err := exec.Command("git", "-C", repoPath, "pull").CombinedOutput()
	if err != nil {
		return fmt.Errorf("pull failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *repoBoundGitopsClient) Push(path, branch string, upstream bool) error {
	args := []string{"-C", path, "push"}
	if upstream {
		args = append(args, "-u", "origin", branch)
	} else {
		args = append(args, "origin", branch)
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("push failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *repoBoundGitopsClient) HasRemote() (bool, error) {
	out, err := c.git("remote")
	if err != nil {
		return false, nil
	}
	return out != "", nil
}

func (c *repoBoundGitopsClient) Fetch(repoPath string) error {
	out, err := exec.Command("git", "-C", repoPath, "fetch").CombinedOutput()
	if err != nil {
		return fmt.Errorf("fetch failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *repoBoundGitopsClient) CommitsAhead(path, base string) (int, error) { return 0, nil }
func (c *repoBoundGitopsClient) CommitsBehind(path, base string) (int, error) {
	return 0, nil
}
