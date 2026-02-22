package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ConflictChecker checks whether a worktree has unresolved conflicts.
type ConflictChecker interface {
	HasConflicts(worktreePath string) (bool, error)
}

// GitConflictChecker checks for unresolved git conflicts by looking for
// conflict markers (MERGE_HEAD, rebase-merge) and unmerged files.
// Works correctly with both regular repos and git worktrees.
type GitConflictChecker struct{}

// HasConflicts returns true if the worktree has unresolved merge/rebase conflicts.
func (g *GitConflictChecker) HasConflicts(worktreePath string) (bool, error) {
	// Resolve the actual git directory (handles worktrees where .git is a file)
	gitDirOut, err := exec.Command("git", "-C", worktreePath, "rev-parse", "--git-dir").Output()
	if err != nil {
		// If git command fails, assume no conflicts
		return false, nil
	}
	gitDir := strings.TrimSpace(string(gitDirOut))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(worktreePath, gitDir)
	}

	// Check for MERGE_HEAD (indicates an in-progress merge)
	if _, err := os.Stat(filepath.Join(gitDir, "MERGE_HEAD")); err == nil {
		return true, nil
	}

	// Check for rebase-merge (indicates an in-progress rebase)
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-merge")); err == nil {
		return true, nil
	}

	// Check for rebase-apply (indicates an in-progress rebase/am)
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-apply")); err == nil {
		return true, nil
	}

	// Check for unmerged files via git
	cmd := exec.Command("git", "-C", worktreePath, "diff", "--name-only", "--diff-filter=U")
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}

	return strings.TrimSpace(string(out)) != "", nil
}
