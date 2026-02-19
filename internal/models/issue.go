package models

import "time"

// IssueStatus represents the state of an issue.
type IssueStatus string

const (
	IssueStatusOpen       IssueStatus = "open"
	IssueStatusInProgress IssueStatus = "in_progress"
	IssueStatusDone       IssueStatus = "done"
	IssueStatusClosed     IssueStatus = "closed"
)

// IssuePriority represents the urgency of an issue.
type IssuePriority string

const (
	IssuePriorityLow    IssuePriority = "low"
	IssuePriorityMedium IssuePriority = "medium"
	IssuePriorityHigh   IssuePriority = "high"
)

// IssueType represents the kind of work an issue tracks.
type IssueType string

const (
	IssueTypeFeature IssueType = "feature"
	IssueTypeBug     IssueType = "bug"
	IssueTypeChore   IssueType = "chore"
)

// Issue represents a tracked issue/feature for a project.
type Issue struct {
	ID          string
	ProjectID   string
	Title       string
	Description string
	Body        string // raw/original text preserved from import
	AIPrompt    string // LLM-generated guidance for AI agents working on this issue
	Status      IssueStatus
	Priority    IssuePriority
	Type        IssueType
	Tags        []string
	GitHubIssue int // linked GitHub issue number (0 = none)
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ClosedAt    *time.Time
}
