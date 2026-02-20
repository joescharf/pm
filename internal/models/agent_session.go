package models

import "time"

// SessionStatus represents the state of an agent session.
type SessionStatus string

const (
	SessionStatusActive    SessionStatus = "active"
	SessionStatusIdle      SessionStatus = "idle"
	SessionStatusCompleted SessionStatus = "completed"
	SessionStatusAbandoned SessionStatus = "abandoned"
)

// ConflictState represents the conflict state of a session's worktree.
type ConflictState string

const (
	ConflictStateNone         ConflictState = "none"
	ConflictStateSyncConflict ConflictState = "sync_conflict"
	ConflictStateMergeConflict ConflictState = "merge_conflict"
)

// AgentSession represents a Claude Code agent session tied to a project and issue.
type AgentSession struct {
	ID                string
	ProjectID         string
	IssueID           string
	Branch            string
	WorktreePath      string
	Status            SessionStatus
	Outcome           string
	CommitCount       int
	LastCommitHash    string
	LastCommitMessage string
	LastActiveAt      *time.Time
	StartedAt         time.Time
	EndedAt           *time.Time

	// Session operations fields
	LastError     string        // Last operation error message
	LastSyncAt    *time.Time    // When last synced with base
	ConflictState ConflictState // "none", "sync_conflict", "merge_conflict"
	ConflictFiles string        // JSON array of conflicting file paths
	Discovered    bool          // true if auto-discovered (not created by pm)
}
