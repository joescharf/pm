package sessions

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joescharf/wt/pkg/gitops"
)

// repoBoundClient implements gitops.Client for a specific repository path.
// This allows pm (which manages multiple repos) to use wt's ops package
// which expects a single-repo gitops.Client.
type repoBoundClient struct {
	repoPath string
}

// newRepoBoundClient creates a gitops.Client bound to the given repo path.
func newRepoBoundClient(repoPath string) gitops.Client {
	return &repoBoundClient{repoPath: repoPath}
}

func (c *repoBoundClient) git(args ...string) (string, error) {
	fullArgs := append([]string{"-C", c.repoPath}, args...)
	out, err := exec.Command("git", fullArgs...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *repoBoundClient) gitAt(path string, args ...string) (string, error) {
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

func (c *repoBoundClient) RepoRoot() (string, error) {
	return c.repoPath, nil
}

func (c *repoBoundClient) RepoName() (string, error) {
	return filepath.Base(c.repoPath), nil
}

func (c *repoBoundClient) WorktreesDir() (string, error) {
	return c.repoPath + ".worktrees", nil
}

func (c *repoBoundClient) WorktreeList() ([]gitops.WorktreeInfo, error) {
	out, err := c.git("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return gitops.ParseWorktreeListPorcelain(out), nil
}

func (c *repoBoundClient) WorktreeAdd(path, branch, base string, newBranch bool) error {
	var args []string
	if newBranch {
		args = []string{"-C", c.repoPath, "worktree", "add", "-b", branch, path, base}
	} else {
		args = []string{"-C", c.repoPath, "worktree", "add", path, branch}
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (c *repoBoundClient) WorktreeRemove(path string, force bool) error {
	args := []string{"-C", c.repoPath, "worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (c *repoBoundClient) BranchExists(branch string) (bool, error) {
	err := exec.Command("git", "-C", c.repoPath, "show-ref", "--verify", "--quiet", "refs/heads/"+branch).Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *repoBoundClient) BranchDelete(branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := c.git("branch", flag, branch)
	return err
}

func (c *repoBoundClient) CurrentBranch(worktreePath string) (string, error) {
	return c.gitAt(worktreePath, "rev-parse", "--abbrev-ref", "HEAD")
}

func (c *repoBoundClient) ResolveWorktree(input string) (string, error) {
	wtDir := c.repoPath + ".worktrees"
	return gitops.ResolveWorktreePath(input, wtDir)
}

func (c *repoBoundClient) BranchList() ([]string, error) {
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

func (c *repoBoundClient) IsWorktreeDirty(path string) (bool, error) {
	out, err := c.gitAt(path, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (c *repoBoundClient) HasUnpushedCommits(path, baseBranch string) (bool, error) {
	out, err := c.gitAt(path, "log", baseBranch+"..HEAD", "--oneline")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (c *repoBoundClient) WorktreePrune() error {
	_, err := c.git("worktree", "prune")
	return err
}

func (c *repoBoundClient) Merge(repoPath, branch string) error {
	out, err := exec.Command("git", "-C", repoPath, "merge", branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("merge failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (c *repoBoundClient) MergeContinue(repoPath string) error {
	out, err := exec.Command("git", "-C", repoPath, "merge", "--continue").CombinedOutput()
	if err != nil {
		return fmt.Errorf("merge --continue failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (c *repoBoundClient) IsMergeInProgress(repoPath string) (bool, error) {
	_, err := c.gitAt(repoPath, "rev-parse", "MERGE_HEAD")
	return err == nil, nil
}

func (c *repoBoundClient) HasConflicts(repoPath string) (bool, error) {
	out, err := c.gitAt(repoPath, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

func (c *repoBoundClient) Rebase(repoPath, branch string) error {
	out, err := exec.Command("git", "-C", repoPath, "rebase", branch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebase failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (c *repoBoundClient) RebaseContinue(repoPath string) error {
	out, err := exec.Command("git", "-C", repoPath, "rebase", "--continue").CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebase --continue failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (c *repoBoundClient) RebaseAbort(repoPath string) error {
	out, err := exec.Command("git", "-C", repoPath, "rebase", "--abort").CombinedOutput()
	if err != nil {
		return fmt.Errorf("rebase --abort failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (c *repoBoundClient) IsRebaseInProgress(repoPath string) (bool, error) {
	// Check for rebase-merge or rebase-apply directories
	gitDir, err := c.gitAt(repoPath, "rev-parse", "--git-dir")
	if err != nil {
		return false, err
	}
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(repoPath, gitDir)
	}
	for _, dir := range []string{"rebase-merge", "rebase-apply"} {
		if _, err := exec.Command("test", "-d", filepath.Join(gitDir, dir)).Output(); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func (c *repoBoundClient) Pull(repoPath string) error {
	out, err := exec.Command("git", "-C", repoPath, "pull").CombinedOutput()
	if err != nil {
		return fmt.Errorf("pull failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (c *repoBoundClient) Push(worktreePath, branch string, setUpstream bool) error {
	args := []string{"-C", worktreePath, "push"}
	if setUpstream {
		args = append(args, "-u", "origin", branch)
	} else {
		args = append(args, "origin", branch)
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("push failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (c *repoBoundClient) HasRemote() (bool, error) {
	out, err := c.git("remote")
	if err != nil {
		return false, nil
	}
	return out != "", nil
}

func (c *repoBoundClient) Fetch(repoPath string) error {
	out, err := exec.Command("git", "-C", repoPath, "fetch").CombinedOutput()
	if err != nil {
		return fmt.Errorf("fetch failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (c *repoBoundClient) CommitsAhead(worktreePath, baseBranch string) (int, error) {
	out, err := c.gitAt(worktreePath, "rev-list", "--count", baseBranch+"..HEAD")
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

func (c *repoBoundClient) CommitsBehind(worktreePath, baseBranch string) (int, error) {
	out, err := c.gitAt(worktreePath, "rev-list", "--count", "HEAD.."+baseBranch)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}
