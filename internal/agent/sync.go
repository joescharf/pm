package agent

import (
	"context"
	"os"

	"github.com/joescharf/pm/internal/models"
)

// ReconcileSessions checks active/idle sessions and marks any with missing
// worktree directories as abandoned. Returns the count of sessions cleaned up.
func ReconcileSessions(ctx context.Context, s SessionStore, sessions []*models.AgentSession) int {
	cleaned := 0
	for _, sess := range sessions {
		if sess.Status != models.SessionStatusActive && sess.Status != models.SessionStatusIdle {
			continue
		}
		if sess.WorktreePath == "" {
			continue
		}
		if _, err := os.Stat(sess.WorktreePath); err == nil {
			continue
		}
		// Worktree is gone — abandon the session
		if _, err := CloseSession(ctx, s, sess.ID, models.SessionStatusAbandoned); err == nil {
			cleaned++
		}
	}
	return cleaned
}
