package review

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// LaunchClaudeInITerm opens a new iTerm2 window and runs the given claude command
// in the specified worktree directory. The window/tab is named with sessionName.
func LaunchClaudeInITerm(worktreePath, sessionName, claudeCmd string) error {
	// Write command to a temp script to avoid AppleScript quoting issues
	scriptFile, err := os.CreateTemp("", "pm-review-*.sh")
	if err != nil {
		return fmt.Errorf("create temp script: %w", err)
	}
	scriptPath := scriptFile.Name()

	scriptContent := fmt.Sprintf("#!/bin/bash\ncd %s && %s\nrm -f %s\n",
		shellQuote(worktreePath), claudeCmd, shellQuote(scriptPath))
	if _, err := scriptFile.WriteString(scriptContent); err != nil {
		scriptFile.Close()
		os.Remove(scriptPath)
		return fmt.Errorf("write temp script: %w", err)
	}
	scriptFile.Close()
	os.Chmod(scriptPath, 0700)

	escapedName := strings.ReplaceAll(sessionName, `"`, `\"`)
	escapedScript := strings.ReplaceAll(scriptPath, `"`, `\"`)

	appleScript := fmt.Sprintf(`tell application "iTerm2"
	activate
	set newWindow to (create window with default profile)
	tell current session of newWindow
		set name to "%s"
		write text "%s"
	end tell
end tell`, escapedName, escapedScript)

	cmd := exec.Command("osascript", "-e", appleScript)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(scriptPath)
		return fmt.Errorf("launch iTerm: %w (output: %s)", err, string(out))
	}
	return nil
}

// shellQuote wraps a string in single quotes for safe shell embedding.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// AbsScriptPath returns the absolute path for a review script in the worktree's .claude dir.
func AbsScriptPath(worktreePath, name string) string {
	return filepath.Join(worktreePath, ".claude", name)
}
