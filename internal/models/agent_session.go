package models

import "time"

// SessionStatus represents the state of an agent session.
type SessionStatus string

const (
	SessionStatusRunning   SessionStatus = "running"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusFailed    SessionStatus = "failed"
	SessionStatusAbandoned SessionStatus = "abandoned"
)

// AgentSession represents a Claude Code agent session tied to a project and issue.
type AgentSession struct {
	ID           string
	ProjectID    string
	IssueID      string
	Branch       string
	WorktreePath string
	Status       SessionStatus
	Outcome      string
	CommitCount  int
	StartedAt    time.Time
	EndedAt      *time.Time
}
