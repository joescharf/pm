package agent

import (
	"context"
	"os"
	"time"

	"github.com/joescharf/pm/internal/models"
)

// ReconcileSessions checks active/idle sessions and:
// 1. Marks sessions with missing worktree directories as abandoned.
// 2. Transitions active sessions (whose worktree exists) to idle, since
//    if pm is running reconciliation the Claude process has stopped.
// Returns the count of sessions cleaned up.
func ReconcileSessions(ctx context.Context, s SessionStore, sessions []*models.AgentSession) int {
	cleaned := 0
	for _, sess := range sessions {
		if sess.Status != models.SessionStatusActive && sess.Status != models.SessionStatusIdle {
			continue
		}
		if sess.WorktreePath == "" {
			continue
		}
		if _, err := os.Stat(sess.WorktreePath); err != nil {
			// Worktree is gone — abandon the session
			if _, err := CloseSession(ctx, s, sess.ID, models.SessionStatusAbandoned); err == nil {
				cleaned++
			}
			continue
		}
		// Worktree exists but session is active — transition to idle
		if sess.Status == models.SessionStatusActive {
			now := time.Now().UTC()
			sess.LastActiveAt = &now
			sess.Status = models.SessionStatusIdle
			if err := s.UpdateAgentSession(ctx, sess); err == nil {
				cleaned++
			}
		}
	}
	return cleaned
}
