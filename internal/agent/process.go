package agent

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// ProcessDetector checks whether a Claude process is running in a directory.
type ProcessDetector interface {
	IsClaudeRunning(worktreePath string) bool
}

// OSProcessDetector detects Claude processes using pgrep + lsof (macOS/Linux).
type OSProcessDetector struct{}

// IsClaudeRunning returns true if a `claude` process has its cwd at or under worktreePath.
func (d *OSProcessDetector) IsClaudeRunning(worktreePath string) bool {
	absWT, err := filepath.Abs(worktreePath)
	if err != nil {
		return false
	}

	// Find claude PIDs
	out, err := exec.Command("pgrep", "-x", "claude").Output()
	if err != nil {
		return false // pgrep not found or no matches
	}

	for pid := range strings.FieldsSeq(strings.TrimSpace(string(out))) {
		cwd := getCwd(pid)
		if cwd == "" {
			continue
		}
		absCwd, err := filepath.Abs(cwd)
		if err != nil {
			continue
		}
		if absCwd == absWT || strings.HasPrefix(absCwd, absWT+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// getCwd resolves the current working directory of a process via lsof.
func getCwd(pid string) string {
	out, err := exec.Command("lsof", "-a", "-p", pid, "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if strings.HasPrefix(line, "n") && !strings.HasPrefix(line, "n ") {
			return line[1:]
		}
	}
	return ""
}
