package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/joescharf/pm/internal/models"
)

// SessionStore is the subset of store.Store needed for session lifecycle.
type SessionStore interface {
	GetAgentSession(ctx context.Context, id string) (*models.AgentSession, error)
	UpdateAgentSession(ctx context.Context, session *models.AgentSession) error
	GetIssue(ctx context.Context, id string) (*models.Issue, error)
	UpdateIssue(ctx context.Context, issue *models.Issue) error
}

// CloseSession transitions a session to the given status and cascades issue changes.
// Valid target statuses: idle, completed, abandoned.
// Only active or idle sessions can be closed.
func CloseSession(ctx context.Context, s SessionStore, sessionID string, target models.SessionStatus) (*models.AgentSession, error) {
	session, err := s.GetAgentSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	// Only active or idle sessions can transition
	if session.Status != models.SessionStatusActive && session.Status != models.SessionStatusIdle {
		return nil, fmt.Errorf("session %s is already %s", sessionID, session.Status)
	}

	session.Status = target

	// Terminal statuses get an end time
	if target == models.SessionStatusCompleted || target == models.SessionStatusAbandoned {
		now := time.Now().UTC()
		session.EndedAt = &now
	}

	if err := s.UpdateAgentSession(ctx, session); err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	// Cascade issue status
	if session.IssueID != "" {
		issue, err := s.GetIssue(ctx, session.IssueID)
		if err == nil && issue.Status == models.IssueStatusInProgress {
			switch target {
			case models.SessionStatusCompleted:
				issue.Status = models.IssueStatusDone
				_ = s.UpdateIssue(ctx, issue)
			case models.SessionStatusAbandoned:
				issue.Status = models.IssueStatusOpen
				_ = s.UpdateIssue(ctx, issue)
			}
		}
	}

	return session, nil
}
