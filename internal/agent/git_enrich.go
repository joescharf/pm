package agent

import (
	"time"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/models"
)

// EnrichSessionWithGitInfo populates last commit and activity info on a session
// using the worktree path. Best-effort: errors are silently ignored.
func EnrichSessionWithGitInfo(session *models.AgentSession, gc git.Client) {
	if session.WorktreePath == "" || gc == nil {
		return
	}

	if hash, err := gc.LastCommitHash(session.WorktreePath); err == nil {
		session.LastCommitHash = hash
	}
	if msg, err := gc.LastCommitMessage(session.WorktreePath); err == nil {
		session.LastCommitMessage = msg
	}

	now := time.Now().UTC()
	session.LastActiveAt = &now
}
