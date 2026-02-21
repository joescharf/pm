package agent

import (
	"context"
	"os"
	"time"

	"github.com/joescharf/pm/internal/models"
)

// ReconcileOption configures ReconcileSessions behavior.
type ReconcileOption func(*reconcileConfig)

type reconcileConfig struct {
	processDetector ProcessDetector
}

// WithProcessDetector enables active/idle transitions based on claude process detection.
func WithProcessDetector(d ProcessDetector) ReconcileOption {
	return func(c *reconcileConfig) {
		c.processDetector = d
	}
}

// ReconcileSessions checks sessions and:
// 1. Marks active/idle sessions with missing worktree directories as abandoned.
// 2. Recovers abandoned sessions whose worktree still exists back to idle.
// 3. If a ProcessDetector is provided:
//   - Transitions idle -> active when a claude process is detected in the worktree.
//   - Transitions active -> idle when no claude process is detected.
//
// Returns the count of sessions updated.
func ReconcileSessions(ctx context.Context, s SessionStore, sessions []*models.AgentSession, opts ...ReconcileOption) int {
	cfg := &reconcileConfig{}
	for _, opt := range opts {
		opt(cfg)
	}

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
		case wtExists && cfg.processDetector != nil && sess.Status == models.SessionStatusIdle:
			// Idle + claude running → active
			if cfg.processDetector.IsClaudeRunning(sess.WorktreePath) {
				now := time.Now().UTC()
				sess.LastActiveAt = &now
				sess.Status = models.SessionStatusActive
				if err := s.UpdateAgentSession(ctx, sess); err == nil {
					cleaned++
				}
			}
		case wtExists && cfg.processDetector != nil && sess.Status == models.SessionStatusActive:
			// Active + no claude running → idle
			if !cfg.processDetector.IsClaudeRunning(sess.WorktreePath) {
				sess.Status = models.SessionStatusIdle
				if err := s.UpdateAgentSession(ctx, sess); err == nil {
					cleaned++
				}
			}
		}
	}
	return cleaned
}
