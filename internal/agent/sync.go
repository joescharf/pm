package agent

import (
	"context"
	"os"
	"time"

	"github.com/joescharf/pm/internal/models"
)

// ReconcileSessions checks sessions and:
// 1. Marks active/idle sessions with missing worktree directories as abandoned.
// 2. Recovers abandoned sessions whose worktree still exists back to idle.
// Note: active sessions are NOT automatically transitioned to idle here.
// That transition should happen explicitly via agent close/pause.
// Returns the count of sessions updated.
func ReconcileSessions(ctx context.Context, s SessionStore, sessions []*models.AgentSession) int {
	cleaned := 0
	for _, sess := range sessions {
		if sess.Status == models.SessionStatusCompleted {
			continue
		}
		if sess.WorktreePath == "" {
			continue
		}
		wtExists := true
		if _, err := os.Stat(sess.WorktreePath); err != nil {
			wtExists = false
		}

		switch {
		case !wtExists && (sess.Status == models.SessionStatusActive || sess.Status == models.SessionStatusIdle):
			// Worktree is gone — abandon the session
			if _, err := CloseSession(ctx, s, sess.ID, models.SessionStatusAbandoned); err == nil {
				cleaned++
			}
		case wtExists && sess.Status == models.SessionStatusAbandoned:
			// Worktree recovered/still exists — transition back to idle
			now := time.Now().UTC()
			sess.LastActiveAt = &now
			sess.Status = models.SessionStatusIdle
			sess.EndedAt = nil
			if err := s.UpdateAgentSession(ctx, sess); err == nil {
				cleaned++
			}
		}
	}
	return cleaned
}
