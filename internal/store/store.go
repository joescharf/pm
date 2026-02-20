package store

import (
	"context"

	"github.com/joescharf/pm/internal/models"
)

// IssueListFilter specifies filters for listing issues.
type IssueListFilter struct {
	ProjectID string
	Status    models.IssueStatus
	Priority  models.IssuePriority
	Type      models.IssueType
	Tag       string
}

// Store defines the persistence interface for pm.
type Store interface {
	// Projects
	CreateProject(ctx context.Context, p *models.Project) error
	GetProject(ctx context.Context, id string) (*models.Project, error)
	GetProjectByName(ctx context.Context, name string) (*models.Project, error)
	GetProjectByPath(ctx context.Context, path string) (*models.Project, error)
	ListProjects(ctx context.Context, group string) ([]*models.Project, error)
	UpdateProject(ctx context.Context, p *models.Project) error
	DeleteProject(ctx context.Context, id string) error

	// Issues
	CreateIssue(ctx context.Context, issue *models.Issue) error
	GetIssue(ctx context.Context, id string) (*models.Issue, error)
	ListIssues(ctx context.Context, filter IssueListFilter) ([]*models.Issue, error)
	UpdateIssue(ctx context.Context, issue *models.Issue) error
	DeleteIssue(ctx context.Context, id string) error
	BulkUpdateIssueStatus(ctx context.Context, ids []string, status models.IssueStatus) (int64, error)
	BulkDeleteIssues(ctx context.Context, ids []string) (int64, error)

	// Tags
	CreateTag(ctx context.Context, tag *models.Tag) error
	ListTags(ctx context.Context) ([]*models.Tag, error)
	DeleteTag(ctx context.Context, id string) error
	TagIssue(ctx context.Context, issueID, tagID string) error
	UntagIssue(ctx context.Context, issueID, tagID string) error
	GetIssueTags(ctx context.Context, issueID string) ([]*models.Tag, error)

	// Agent Sessions
	CreateAgentSession(ctx context.Context, session *models.AgentSession) error
	GetAgentSession(ctx context.Context, id string) (*models.AgentSession, error)
	GetAgentSessionByWorktreePath(ctx context.Context, path string) (*models.AgentSession, error)
	ListAgentSessions(ctx context.Context, projectID string, limit int) ([]*models.AgentSession, error)
	ListAgentSessionsByStatus(ctx context.Context, projectID string, statuses []models.SessionStatus, limit int) ([]*models.AgentSession, error)
	ListAgentSessionsByWorktreePaths(ctx context.Context, paths []string) ([]*models.AgentSession, error)
	UpdateAgentSession(ctx context.Context, session *models.AgentSession) error
	DeleteStaleSessions(ctx context.Context, projectID, branch string) (int64, error)

	// Issue Reviews
	CreateIssueReview(ctx context.Context, review *models.IssueReview) error
	ListIssueReviews(ctx context.Context, issueID string) ([]*models.IssueReview, error)

	// Lifecycle
	Migrate(ctx context.Context) error
	Close() error
}
