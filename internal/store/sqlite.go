package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/joescharf/pm/internal/models"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// SQLiteStore implements Store using modernc.org/sqlite (pure Go, no CGO).
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at the given path.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Enable WAL mode for concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	return &SQLiteStore{db: db}, nil
}

// boolToInt converts a bool to 0 or 1 for SQLite storage.
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// newULID generates a new ULID string.
func newULID() string {
	entropy := rand.New(rand.NewSource(time.Now().UnixNano()))
	return ulid.MustNew(ulid.Timestamp(time.Now()), ulid.Monotonic(entropy, 0)).String()
}

// Migrate runs all embedded SQL migration files in order.
func (s *SQLiteStore) Migrate(ctx context.Context) error {
	// Create migrations tracking table
	_, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		filename TEXT PRIMARY KEY,
		applied_at DATETIME NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Sort by filename
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// Check if already applied
		var count int
		err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE filename = ?", name).Scan(&count)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if count > 0 {
			continue
		}

		data, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if _, err := s.db.ExecContext(ctx, string(data)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := s.db.ExecContext(ctx, "INSERT INTO schema_migrations (filename) VALUES (?)", name); err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}
	}

	return nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

// --- Projects ---

func (s *SQLiteStore) CreateProject(ctx context.Context, p *models.Project) error {
	if p.ID == "" {
		p.ID = newULID()
	}
	now := time.Now().UTC()
	p.CreatedAt = now
	p.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO projects (id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Path, p.Description, p.RepoURL, p.Language, p.GroupName,
		p.BranchCount, boolToInt(p.HasGitHubPages), p.PagesURL, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetProject(ctx context.Context, id string) (*models.Project, error) {
	p := &models.Project{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, created_at, updated_at
		FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Path, &p.Description, &p.RepoURL, &p.Language, &p.GroupName, &p.BranchCount, &p.HasGitHubPages, &p.PagesURL, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}
	return p, nil
}

func (s *SQLiteStore) GetProjectByName(ctx context.Context, name string) (*models.Project, error) {
	p := &models.Project{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, created_at, updated_at
		FROM projects WHERE name = ?`, name,
	).Scan(&p.ID, &p.Name, &p.Path, &p.Description, &p.RepoURL, &p.Language, &p.GroupName, &p.BranchCount, &p.HasGitHubPages, &p.PagesURL, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found: %s", name)
	}
	if err != nil {
		return nil, fmt.Errorf("get project by name: %w", err)
	}
	return p, nil
}

func (s *SQLiteStore) GetProjectByPath(ctx context.Context, path string) (*models.Project, error) {
	p := &models.Project{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, created_at, updated_at
		FROM projects WHERE path = ?`, path,
	).Scan(&p.ID, &p.Name, &p.Path, &p.Description, &p.RepoURL, &p.Language, &p.GroupName, &p.BranchCount, &p.HasGitHubPages, &p.PagesURL, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found at path: %s", path)
	}
	if err != nil {
		return nil, fmt.Errorf("get project by path: %w", err)
	}
	return p, nil
}

func (s *SQLiteStore) ListProjects(ctx context.Context, group string) ([]*models.Project, error) {
	var rows *sql.Rows
	var err error
	if group != "" {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, created_at, updated_at
			FROM projects WHERE group_name = ? ORDER BY name`, group)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, created_at, updated_at
			FROM projects ORDER BY name`)
	}
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		p := &models.Project{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Description, &p.RepoURL, &p.Language, &p.GroupName, &p.BranchCount, &p.HasGitHubPages, &p.PagesURL, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *SQLiteStore) UpdateProject(ctx context.Context, p *models.Project) error {
	p.UpdatedAt = time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE projects SET name=?, path=?, description=?, repo_url=?, language=?, group_name=?, branch_count=?, has_github_pages=?, pages_url=?, updated_at=?
		WHERE id=?`,
		p.Name, p.Path, p.Description, p.RepoURL, p.Language, p.GroupName,
		p.BranchCount, boolToInt(p.HasGitHubPages), p.PagesURL, p.UpdatedAt, p.ID,
	)
	if err != nil {
		return fmt.Errorf("update project: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("project not found: %s", p.ID)
	}
	return nil
}

func (s *SQLiteStore) DeleteProject(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("project not found: %s", id)
	}
	return nil
}

// --- Issues ---

func (s *SQLiteStore) CreateIssue(ctx context.Context, issue *models.Issue) error {
	if issue.ID == "" {
		issue.ID = newULID()
	}
	now := time.Now().UTC()
	issue.CreatedAt = now
	issue.UpdatedAt = now

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO issues (id, project_id, title, description, status, priority, type, github_issue, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue.ID, issue.ProjectID, issue.Title, issue.Description,
		string(issue.Status), string(issue.Priority), string(issue.Type),
		issue.GitHubIssue, issue.CreatedAt, issue.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create issue: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetIssue(ctx context.Context, id string) (*models.Issue, error) {
	issue := &models.Issue{}
	var status, priority, issueType string
	var closedAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, title, description, status, priority, type, github_issue, created_at, updated_at, closed_at
		FROM issues WHERE id = ?`, id,
	).Scan(&issue.ID, &issue.ProjectID, &issue.Title, &issue.Description,
		&status, &priority, &issueType,
		&issue.GitHubIssue, &issue.CreatedAt, &issue.UpdatedAt, &closedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("issue not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}

	issue.Status = models.IssueStatus(status)
	issue.Priority = models.IssuePriority(priority)
	issue.Type = models.IssueType(issueType)
	if closedAt.Valid {
		issue.ClosedAt = &closedAt.Time
	}

	// Load tags
	tags, err := s.GetIssueTags(ctx, issue.ID)
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		issue.Tags = append(issue.Tags, t.Name)
	}

	return issue, nil
}

func (s *SQLiteStore) ListIssues(ctx context.Context, filter IssueListFilter) ([]*models.Issue, error) {
	query := `SELECT id, project_id, title, description, status, priority, type, github_issue, created_at, updated_at, closed_at FROM issues`
	var conditions []string
	var args []any

	if filter.ProjectID != "" {
		conditions = append(conditions, "project_id = ?")
		args = append(args, filter.ProjectID)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, string(filter.Status))
	}
	if filter.Priority != "" {
		conditions = append(conditions, "priority = ?")
		args = append(args, string(filter.Priority))
	}
	if filter.Type != "" {
		conditions = append(conditions, "type = ?")
		args = append(args, string(filter.Type))
	}
	if filter.Tag != "" {
		conditions = append(conditions, "id IN (SELECT issue_id FROM issue_tags JOIN tags ON tags.id = issue_tags.tag_id WHERE tags.name = ?)")
		args = append(args, filter.Tag)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += ` ORDER BY
		CASE status WHEN 'open' THEN 0 WHEN 'in_progress' THEN 1 WHEN 'done' THEN 2 WHEN 'closed' THEN 3 ELSE 4 END,
		CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 3 END,
		created_at DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	var issues []*models.Issue
	for rows.Next() {
		issue := &models.Issue{}
		var status, priority, issueType string
		var closedAt sql.NullTime

		if err := rows.Scan(&issue.ID, &issue.ProjectID, &issue.Title, &issue.Description,
			&status, &priority, &issueType,
			&issue.GitHubIssue, &issue.CreatedAt, &issue.UpdatedAt, &closedAt); err != nil {
			return nil, fmt.Errorf("scan issue: %w", err)
		}

		issue.Status = models.IssueStatus(status)
		issue.Priority = models.IssuePriority(priority)
		issue.Type = models.IssueType(issueType)
		if closedAt.Valid {
			issue.ClosedAt = &closedAt.Time
		}

		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func (s *SQLiteStore) UpdateIssue(ctx context.Context, issue *models.Issue) error {
	issue.UpdatedAt = time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE issues SET title=?, description=?, status=?, priority=?, type=?, github_issue=?, updated_at=?, closed_at=?
		WHERE id=?`,
		issue.Title, issue.Description, string(issue.Status), string(issue.Priority), string(issue.Type),
		issue.GitHubIssue, issue.UpdatedAt, issue.ClosedAt, issue.ID,
	)
	if err != nil {
		return fmt.Errorf("update issue: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("issue not found: %s", issue.ID)
	}
	return nil
}

func (s *SQLiteStore) DeleteIssue(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM issues WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete issue: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("issue not found: %s", id)
	}
	return nil
}

// --- Tags ---

func (s *SQLiteStore) CreateTag(ctx context.Context, tag *models.Tag) error {
	if tag.ID == "" {
		tag.ID = newULID()
	}
	tag.CreatedAt = time.Now().UTC()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tags (id, name, created_at) VALUES (?, ?, ?)`,
		tag.ID, tag.Name, tag.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create tag: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListTags(ctx context.Context) ([]*models.Tag, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, created_at FROM tags ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	var tags []*models.Tag
	for rows.Next() {
		t := &models.Tag{}
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

func (s *SQLiteStore) DeleteTag(ctx context.Context, id string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM tags WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("tag not found: %s", id)
	}
	return nil
}

func (s *SQLiteStore) TagIssue(ctx context.Context, issueID, tagID string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT OR IGNORE INTO issue_tags (issue_id, tag_id) VALUES (?, ?)", issueID, tagID)
	if err != nil {
		return fmt.Errorf("tag issue: %w", err)
	}
	return nil
}

func (s *SQLiteStore) UntagIssue(ctx context.Context, issueID, tagID string) error {
	_, err := s.db.ExecContext(ctx,
		"DELETE FROM issue_tags WHERE issue_id = ? AND tag_id = ?", issueID, tagID)
	if err != nil {
		return fmt.Errorf("untag issue: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetIssueTags(ctx context.Context, issueID string) ([]*models.Tag, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT t.id, t.name, t.created_at FROM tags t
		JOIN issue_tags it ON t.id = it.tag_id
		WHERE it.issue_id = ? ORDER BY t.name`, issueID)
	if err != nil {
		return nil, fmt.Errorf("get issue tags: %w", err)
	}
	defer rows.Close()

	var tags []*models.Tag
	for rows.Next() {
		t := &models.Tag{}
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// --- Agent Sessions ---

func (s *SQLiteStore) CreateAgentSession(ctx context.Context, session *models.AgentSession) error {
	if session.ID == "" {
		session.ID = newULID()
	}
	session.StartedAt = time.Now().UTC()

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_sessions (id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, last_commit_hash, last_commit_message, last_active_at, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.ProjectID, session.IssueID, session.Branch,
		session.WorktreePath, string(session.Status), session.Outcome,
		session.CommitCount, session.LastCommitHash, session.LastCommitMessage,
		session.LastActiveAt, session.StartedAt,
	)
	if err != nil {
		return fmt.Errorf("create agent session: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetAgentSession(ctx context.Context, id string) (*models.AgentSession, error) {
	session := &models.AgentSession{}
	var status string
	var endedAt, lastActiveAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, last_commit_hash, last_commit_message, last_active_at, started_at, ended_at
		FROM agent_sessions WHERE id = ?`, id,
	).Scan(&session.ID, &session.ProjectID, &session.IssueID,
		&session.Branch, &session.WorktreePath, &status,
		&session.Outcome, &session.CommitCount,
		&session.LastCommitHash, &session.LastCommitMessage, &lastActiveAt,
		&session.StartedAt, &endedAt)
	if err != nil {
		return nil, fmt.Errorf("agent session not found: %s", id)
	}

	session.Status = models.SessionStatus(status)
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}
	if lastActiveAt.Valid {
		session.LastActiveAt = &lastActiveAt.Time
	}
	return session, nil
}

func (s *SQLiteStore) GetAgentSessionByWorktreePath(ctx context.Context, path string) (*models.AgentSession, error) {
	session := &models.AgentSession{}
	var status string
	var endedAt, lastActiveAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, last_commit_hash, last_commit_message, last_active_at, started_at, ended_at
		FROM agent_sessions WHERE worktree_path = ? AND status IN ('active', 'idle')
		ORDER BY started_at DESC LIMIT 1`, path,
	).Scan(&session.ID, &session.ProjectID, &session.IssueID,
		&session.Branch, &session.WorktreePath, &status,
		&session.Outcome, &session.CommitCount,
		&session.LastCommitHash, &session.LastCommitMessage, &lastActiveAt,
		&session.StartedAt, &endedAt)
	if err != nil {
		return nil, fmt.Errorf("no active/idle session for worktree: %s", path)
	}

	session.Status = models.SessionStatus(status)
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}
	if lastActiveAt.Valid {
		session.LastActiveAt = &lastActiveAt.Time
	}
	return session, nil
}

func (s *SQLiteStore) ListAgentSessions(ctx context.Context, projectID string, limit int) ([]*models.AgentSession, error) {
	query := `SELECT id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, last_commit_hash, last_commit_message, last_active_at, started_at, ended_at
		FROM agent_sessions`
	var args []any

	if projectID != "" {
		query += " WHERE project_id = ?"
		args = append(args, projectID)
	}
	query += " ORDER BY started_at DESC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agent sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.AgentSession
	for rows.Next() {
		session := &models.AgentSession{}
		var status string
		var endedAt, lastActiveAt sql.NullTime

		if err := rows.Scan(&session.ID, &session.ProjectID, &session.IssueID,
			&session.Branch, &session.WorktreePath, &status,
			&session.Outcome, &session.CommitCount,
			&session.LastCommitHash, &session.LastCommitMessage, &lastActiveAt,
			&session.StartedAt, &endedAt); err != nil {
			return nil, fmt.Errorf("scan agent session: %w", err)
		}

		session.Status = models.SessionStatus(status)
		if endedAt.Valid {
			session.EndedAt = &endedAt.Time
		}
		if lastActiveAt.Valid {
			session.LastActiveAt = &lastActiveAt.Time
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (s *SQLiteStore) UpdateAgentSession(ctx context.Context, session *models.AgentSession) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE agent_sessions SET status=?, outcome=?, commit_count=?, last_commit_hash=?, last_commit_message=?, last_active_at=?, ended_at=? WHERE id=?`,
		string(session.Status), session.Outcome, session.CommitCount,
		session.LastCommitHash, session.LastCommitMessage, session.LastActiveAt,
		session.EndedAt, session.ID,
	)
	if err != nil {
		return fmt.Errorf("update agent session: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("agent session not found: %s", session.ID)
	}
	return nil
}
