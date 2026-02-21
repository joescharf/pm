package review

import (
	"fmt"
	"os/exec"
	"strings"
)

// LaunchClaudeInITerm opens a new iTerm2 window and runs the given claude command
// in the specified worktree directory. The window/tab is named with sessionName.
func LaunchClaudeInITerm(worktreePath, sessionName, claudeCmd string) error {
	// Escape double quotes in the command for AppleScript
	escapedCmd := strings.ReplaceAll(claudeCmd, `"`, `\"`)
	escapedPath := strings.ReplaceAll(worktreePath, `"`, `\"`)
	escapedName := strings.ReplaceAll(sessionName, `"`, `\"`)

	script := fmt.Sprintf(`tell application "iTerm2"
	activate
	set newWindow to (create window with default profile)
	tell current session of newWindow
		set name to "%s"
		write text "cd \"%s\" && %s"
	end tell
end tell`, escapedName, escapedPath, escapedCmd)

	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("launch iTerm: %w (output: %s)", err, string(out))
	}
	return nil
}
