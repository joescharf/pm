package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
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

	// SQLite only supports one concurrent writer. Limiting to a single connection
	// serializes all DB access through Go's connection pool, preventing
	// "database is locked" errors from concurrent HTTP requests.
	db.SetMaxOpenConns(1)

	// Enable WAL mode for concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable WAL mode: %w", err)
	}

	// Set busy timeout so concurrent writes wait instead of failing immediately
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
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
		`INSERT INTO projects (id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, build_cmd, serve_cmd, serve_port, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.Path, p.Description, p.RepoURL, p.Language, p.GroupName,
		p.BranchCount, boolToInt(p.HasGitHubPages), p.PagesURL, p.BuildCmd, p.ServeCmd, p.ServePort, p.CreatedAt, p.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create project: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetProject(ctx context.Context, id string) (*models.Project, error) {
	p := &models.Project{}
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, build_cmd, serve_cmd, serve_port, created_at, updated_at
		FROM projects WHERE id = ?`, id,
	).Scan(&p.ID, &p.Name, &p.Path, &p.Description, &p.RepoURL, &p.Language, &p.GroupName, &p.BranchCount, &p.HasGitHubPages, &p.PagesURL, &p.BuildCmd, &p.ServeCmd, &p.ServePort, &p.CreatedAt, &p.UpdatedAt)
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
		`SELECT id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, build_cmd, serve_cmd, serve_port, created_at, updated_at
		FROM projects WHERE name = ?`, name,
	).Scan(&p.ID, &p.Name, &p.Path, &p.Description, &p.RepoURL, &p.Language, &p.GroupName, &p.BranchCount, &p.HasGitHubPages, &p.PagesURL, &p.BuildCmd, &p.ServeCmd, &p.ServePort, &p.CreatedAt, &p.UpdatedAt)
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
		`SELECT id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, build_cmd, serve_cmd, serve_port, created_at, updated_at
		FROM projects WHERE path = ?`, path,
	).Scan(&p.ID, &p.Name, &p.Path, &p.Description, &p.RepoURL, &p.Language, &p.GroupName, &p.BranchCount, &p.HasGitHubPages, &p.PagesURL, &p.BuildCmd, &p.ServeCmd, &p.ServePort, &p.CreatedAt, &p.UpdatedAt)
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
			`SELECT id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, build_cmd, serve_cmd, serve_port, created_at, updated_at
			FROM projects WHERE group_name = ? ORDER BY name`, group)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, name, path, description, repo_url, language, group_name, branch_count, has_github_pages, pages_url, build_cmd, serve_cmd, serve_port, created_at, updated_at
			FROM projects ORDER BY name`)
	}
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var projects []*models.Project
	for rows.Next() {
		p := &models.Project{}
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.Description, &p.RepoURL, &p.Language, &p.GroupName, &p.BranchCount, &p.HasGitHubPages, &p.PagesURL, &p.BuildCmd, &p.ServeCmd, &p.ServePort, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

func (s *SQLiteStore) UpdateProject(ctx context.Context, p *models.Project) error {
	p.UpdatedAt = time.Now().UTC()
	result, err := s.db.ExecContext(ctx,
		`UPDATE projects SET name=?, path=?, description=?, repo_url=?, language=?, group_name=?, branch_count=?, has_github_pages=?, pages_url=?, build_cmd=?, serve_cmd=?, serve_port=?, updated_at=?
		WHERE id=?`,
		p.Name, p.Path, p.Description, p.RepoURL, p.Language, p.GroupName,
		p.BranchCount, boolToInt(p.HasGitHubPages), p.PagesURL, p.BuildCmd, p.ServeCmd, p.ServePort, p.UpdatedAt, p.ID,
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
		`INSERT INTO issues (id, project_id, title, description, body, ai_prompt, status, priority, type, github_issue, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		issue.ID, issue.ProjectID, issue.Title, issue.Description, issue.Body, issue.AIPrompt,
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
		`SELECT id, project_id, title, description, body, ai_prompt, status, priority, type, github_issue, created_at, updated_at, closed_at
		FROM issues WHERE id = ?`, id,
	).Scan(&issue.ID, &issue.ProjectID, &issue.Title, &issue.Description, &issue.Body, &issue.AIPrompt,
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
	query := `SELECT id, project_id, title, description, body, ai_prompt, status, priority, type, github_issue, created_at, updated_at, closed_at FROM issues`
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
	defer func() { _ = rows.Close() }()

	var issues []*models.Issue
	for rows.Next() {
		issue := &models.Issue{}
		var status, priority, issueType string
		var closedAt sql.NullTime

		if err := rows.Scan(&issue.ID, &issue.ProjectID, &issue.Title, &issue.Description, &issue.Body, &issue.AIPrompt,
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
		`UPDATE issues SET title=?, description=?, body=?, ai_prompt=?, status=?, priority=?, type=?, github_issue=?, updated_at=?, closed_at=?
		WHERE id=?`,
		issue.Title, issue.Description, issue.Body, issue.AIPrompt, string(issue.Status), string(issue.Priority), string(issue.Type),
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

func (s *SQLiteStore) BulkUpdateIssueStatus(ctx context.Context, ids []string, status models.IssueStatus) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+2)
	args = append(args, string(status), time.Now().UTC())
	for i, id := range ids {
		placeholders[i] = "?"
		args = append(args, id)
	}

	query := fmt.Sprintf(
		"UPDATE issues SET status=?, updated_at=? WHERE id IN (%s)",
		strings.Join(placeholders, ","),
	)
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("bulk update issue status: %w", err)
	}
	n, _ := result.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return n, nil
}

func (s *SQLiteStore) BulkDeleteIssues(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	// Delete issue_tags first (foreign key)
	tagQuery := fmt.Sprintf("DELETE FROM issue_tags WHERE issue_id IN (%s)", strings.Join(placeholders, ","))
	if _, err := tx.ExecContext(ctx, tagQuery, args...); err != nil {
		return 0, fmt.Errorf("bulk delete issue tags: %w", err)
	}

	query := fmt.Sprintf("DELETE FROM issues WHERE id IN (%s)", strings.Join(placeholders, ","))
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("bulk delete issues: %w", err)
	}
	n, _ := result.RowsAffected()
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return n, nil
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
	defer func() { _ = rows.Close() }()

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
	defer func() { _ = rows.Close() }()

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
	if session.ConflictState == "" {
		session.ConflictState = models.ConflictStateNone
	}
	if session.ConflictFiles == "" {
		session.ConflictFiles = "[]"
	}
	if session.SessionType == "" {
		session.SessionType = models.SessionTypeImplementation
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_sessions (id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, last_commit_hash, last_commit_message, last_active_at, started_at, last_error, last_sync_at, conflict_state, conflict_files, discovered, session_type, review_attempt)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.ProjectID, session.IssueID, session.Branch,
		session.WorktreePath, string(session.Status), session.Outcome,
		session.CommitCount, session.LastCommitHash, session.LastCommitMessage,
		session.LastActiveAt, session.StartedAt,
		session.LastError, session.LastSyncAt, string(session.ConflictState),
		session.ConflictFiles, session.Discovered,
		string(session.SessionType), session.ReviewAttempt,
	)
	if err != nil {
		return fmt.Errorf("create agent session: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetAgentSession(ctx context.Context, id string) (*models.AgentSession, error) {
	session := &models.AgentSession{}
	var status, conflictState, sessionType string
	var endedAt, lastActiveAt, lastSyncAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, last_commit_hash, last_commit_message, last_active_at, started_at, ended_at, last_error, last_sync_at, conflict_state, conflict_files, discovered, session_type, review_attempt
		FROM agent_sessions WHERE id = ?`, id,
	).Scan(&session.ID, &session.ProjectID, &session.IssueID,
		&session.Branch, &session.WorktreePath, &status,
		&session.Outcome, &session.CommitCount,
		&session.LastCommitHash, &session.LastCommitMessage, &lastActiveAt,
		&session.StartedAt, &endedAt,
		&session.LastError, &lastSyncAt, &conflictState,
		&session.ConflictFiles, &session.Discovered,
		&sessionType, &session.ReviewAttempt)
	if err != nil {
		return nil, fmt.Errorf("agent session not found: %s", id)
	}

	session.Status = models.SessionStatus(status)
	session.ConflictState = models.ConflictState(conflictState)
	session.SessionType = models.SessionType(sessionType)
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}
	if lastActiveAt.Valid {
		session.LastActiveAt = &lastActiveAt.Time
	}
	if lastSyncAt.Valid {
		session.LastSyncAt = &lastSyncAt.Time
	}
	return session, nil
}

func (s *SQLiteStore) GetAgentSessionByWorktreePath(ctx context.Context, path string) (*models.AgentSession, error) {
	session := &models.AgentSession{}
	var status, conflictState, sessionType string
	var endedAt, lastActiveAt, lastSyncAt sql.NullTime

	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, last_commit_hash, last_commit_message, last_active_at, started_at, ended_at, last_error, last_sync_at, conflict_state, conflict_files, discovered, session_type, review_attempt
		FROM agent_sessions WHERE worktree_path = ? AND status IN ('active', 'idle')
		ORDER BY started_at DESC LIMIT 1`, path,
	).Scan(&session.ID, &session.ProjectID, &session.IssueID,
		&session.Branch, &session.WorktreePath, &status,
		&session.Outcome, &session.CommitCount,
		&session.LastCommitHash, &session.LastCommitMessage, &lastActiveAt,
		&session.StartedAt, &endedAt,
		&session.LastError, &lastSyncAt, &conflictState,
		&session.ConflictFiles, &session.Discovered,
		&sessionType, &session.ReviewAttempt)
	if err != nil {
		return nil, fmt.Errorf("no active/idle session for worktree: %s", path)
	}

	session.Status = models.SessionStatus(status)
	session.ConflictState = models.ConflictState(conflictState)
	session.SessionType = models.SessionType(sessionType)
	if endedAt.Valid {
		session.EndedAt = &endedAt.Time
	}
	if lastActiveAt.Valid {
		session.LastActiveAt = &lastActiveAt.Time
	}
	if lastSyncAt.Valid {
		session.LastSyncAt = &lastSyncAt.Time
	}
	return session, nil
}

func (s *SQLiteStore) ListAgentSessions(ctx context.Context, projectID string, limit int) ([]*models.AgentSession, error) {
	query := `SELECT id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, last_commit_hash, last_commit_message, last_active_at, started_at, ended_at, last_error, last_sync_at, conflict_state, conflict_files, discovered, session_type, review_attempt
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

	return s.scanAgentSessions(ctx, query, args...)
}

func (s *SQLiteStore) ListAgentSessionsByStatus(ctx context.Context, projectID string, statuses []models.SessionStatus, limit int) ([]*models.AgentSession, error) {
	query := `SELECT id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, last_commit_hash, last_commit_message, last_active_at, started_at, ended_at, last_error, last_sync_at, conflict_state, conflict_files, discovered, session_type, review_attempt
		FROM agent_sessions WHERE 1=1`
	var args []any

	if projectID != "" {
		query += " AND project_id = ?"
		args = append(args, projectID)
	}
	if len(statuses) > 0 {
		placeholders := ""
		for i, st := range statuses {
			if i > 0 {
				placeholders += ", "
			}
			placeholders += "?"
			args = append(args, string(st))
		}
		query += " AND status IN (" + placeholders + ")"
	}
	query += " ORDER BY started_at DESC"
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}

	return s.scanAgentSessions(ctx, query, args...)
}

func (s *SQLiteStore) ListAgentSessionsByWorktreePaths(ctx context.Context, paths []string) ([]*models.AgentSession, error) {
	if len(paths) == 0 {
		return nil, nil
	}

	placeholders := ""
	var args []any
	for i, p := range paths {
		if i > 0 {
			placeholders += ", "
		}
		placeholders += "?"
		args = append(args, p)
	}

	query := `SELECT id, project_id, issue_id, branch, worktree_path, status, outcome, commit_count, last_commit_hash, last_commit_message, last_active_at, started_at, ended_at, last_error, last_sync_at, conflict_state, conflict_files, discovered, session_type, review_attempt
		FROM agent_sessions WHERE worktree_path IN (` + placeholders + `) ORDER BY started_at DESC`

	return s.scanAgentSessions(ctx, query, args...)
}

// scanAgentSessions is a shared helper for scanning agent session rows.
func (s *SQLiteStore) scanAgentSessions(ctx context.Context, query string, args ...any) ([]*models.AgentSession, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list agent sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var sessions []*models.AgentSession
	for rows.Next() {
		session := &models.AgentSession{}
		var status, conflictState, sessionType string
		var endedAt, lastActiveAt, lastSyncAt sql.NullTime

		if err := rows.Scan(&session.ID, &session.ProjectID, &session.IssueID,
			&session.Branch, &session.WorktreePath, &status,
			&session.Outcome, &session.CommitCount,
			&session.LastCommitHash, &session.LastCommitMessage, &lastActiveAt,
			&session.StartedAt, &endedAt,
			&session.LastError, &lastSyncAt, &conflictState,
			&session.ConflictFiles, &session.Discovered,
			&sessionType, &session.ReviewAttempt); err != nil {
			return nil, fmt.Errorf("scan agent session: %w", err)
		}

		session.Status = models.SessionStatus(status)
		session.ConflictState = models.ConflictState(conflictState)
		session.SessionType = models.SessionType(sessionType)
		if endedAt.Valid {
			session.EndedAt = &endedAt.Time
		}
		if lastActiveAt.Valid {
			session.LastActiveAt = &lastActiveAt.Time
		}
		if lastSyncAt.Valid {
			session.LastSyncAt = &lastSyncAt.Time
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (s *SQLiteStore) UpdateAgentSession(ctx context.Context, session *models.AgentSession) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE agent_sessions SET status=?, outcome=?, commit_count=?, last_commit_hash=?, last_commit_message=?, last_active_at=?, ended_at=?, last_error=?, last_sync_at=?, conflict_state=?, conflict_files=?, discovered=?, session_type=?, review_attempt=? WHERE id=?`,
		string(session.Status), session.Outcome, session.CommitCount,
		session.LastCommitHash, session.LastCommitMessage, session.LastActiveAt,
		session.EndedAt,
		session.LastError, session.LastSyncAt, string(session.ConflictState),
		session.ConflictFiles, session.Discovered,
		string(session.SessionType), session.ReviewAttempt,
		session.ID,
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

func (s *SQLiteStore) DeleteStaleSessions(ctx context.Context, projectID, branch string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM agent_sessions
		WHERE project_id = ? AND branch = ?
		AND status = 'abandoned' AND commit_count = 0
		AND ended_at IS NOT NULL
		AND (julianday(substr(ended_at, 1, 19)) - julianday(substr(started_at, 1, 19))) * 86400 < 60`,
		projectID, branch,
	)
	if err != nil {
		return 0, fmt.Errorf("delete stale sessions: %w", err)
	}
	return res.RowsAffected()
}

// DeleteAllStaleSessions removes all abandoned sessions with 0 commits and duration < 60s.
func (s *SQLiteStore) DeleteAllStaleSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM agent_sessions
		WHERE status = 'abandoned' AND commit_count = 0
		AND ended_at IS NOT NULL
		AND (julianday(substr(ended_at, 1, 19)) - julianday(substr(started_at, 1, 19))) * 86400 < 60`,
	)
	if err != nil {
		return 0, fmt.Errorf("delete all stale sessions: %w", err)
	}
	return res.RowsAffected()
}

// --- Issue Reviews ---

func (s *SQLiteStore) CreateIssueReview(ctx context.Context, review *models.IssueReview) error {
	if review.ID == "" {
		review.ID = newULID()
	}
	review.CreatedAt = time.Now().UTC()

	failureJSON, err := json.Marshal(review.FailureReasons)
	if err != nil {
		failureJSON = []byte("[]")
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO issue_reviews (id, issue_id, session_id, verdict, summary, code_quality, requirements_match, test_coverage, ui_ux, failure_reasons, diff_stats, reviewed_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		review.ID, review.IssueID, review.SessionID,
		string(review.Verdict), review.Summary,
		string(review.CodeQuality), string(review.RequirementsMatch),
		string(review.TestCoverage), string(review.UIUX),
		string(failureJSON), review.DiffStats,
		review.ReviewedAt, review.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create issue review: %w", err)
	}
	return nil
}

func (s *SQLiteStore) ListIssueReviews(ctx context.Context, issueID string) ([]*models.IssueReview, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, issue_id, session_id, verdict, summary, code_quality, requirements_match, test_coverage, ui_ux, failure_reasons, diff_stats, reviewed_at, created_at
		FROM issue_reviews WHERE issue_id = ? ORDER BY reviewed_at DESC`, issueID)
	if err != nil {
		return nil, fmt.Errorf("list issue reviews: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var reviews []*models.IssueReview
	for rows.Next() {
		r := &models.IssueReview{}
		var failureJSON string
		if err := rows.Scan(&r.ID, &r.IssueID, &r.SessionID,
			&r.Verdict, &r.Summary,
			&r.CodeQuality, &r.RequirementsMatch,
			&r.TestCoverage, &r.UIUX,
			&failureJSON, &r.DiffStats,
			&r.ReviewedAt, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan issue review: %w", err)
		}
		_ = json.Unmarshal([]byte(failureJSON), &r.FailureReasons)
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}
