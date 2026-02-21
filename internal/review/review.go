package review

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/store"
)

// Config holds review agent configuration.
type Config struct {
	MaxAttempts  int
	AllowedTools []string
}

// DefaultConfig returns the default review config, reading from viper when available.
func DefaultConfig() Config {
	maxAttempts := viper.GetInt("review.max_attempts")
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	allowedTools := viper.GetString("review.allowed_tools")
	if allowedTools == "" {
		allowedTools = "Read Write Edit Glob Grep Bash(git:*) Bash(make:*) Bash(go:*) mcp__pm__*"
	}

	var tools []string
	for _, t := range strings.Split(allowedTools, " ") {
		t = strings.TrimSpace(t)
		if t != "" {
			tools = append(tools, t)
		}
	}

	return Config{
		MaxAttempts:  maxAttempts,
		AllowedTools: tools,
	}
}

// LaunchResult holds the result of launching a review agent.
type LaunchResult struct {
	SessionID    string `json:"session_id"`
	IssueID      string `json:"issue_id"`
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktree_path"`
	Command      string `json:"command"`
}

// Launcher orchestrates review agent launches.
type Launcher struct {
	store store.Store
	cfg   Config
}

// NewLauncher creates a review launcher with the given store and config.
func NewLauncher(s store.Store, cfg Config) *Launcher {
	return &Launcher{store: s, cfg: cfg}
}

// Launch validates preconditions and launches a Claude Code review agent in iTerm.
func (l *Launcher) Launch(ctx context.Context, issueID string) (*LaunchResult, error) {
	// 1. Find the issue (support prefix matching)
	issue, err := l.findIssue(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("issue not found: %s", issueID)
	}

	// 2. Validate issue status
	if issue.Status != models.IssueStatusDone {
		return nil, fmt.Errorf("issue must be in 'done' status to review (current: %s)", issue.Status)
	}

	// 3. Find the most recent session for this issue that has a worktree
	sessions, err := l.store.ListAgentSessions(ctx, issue.ProjectID, 0)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}

	var session *models.AgentSession
	for _, sess := range sessions {
		if sess.IssueID == issue.ID && sess.WorktreePath != "" {
			// Check if worktree actually exists on disk
			if _, statErr := os.Stat(sess.WorktreePath); statErr == nil {
				session = sess
				break
			}
		}
	}
	if session == nil {
		return nil, fmt.Errorf("no session with worktree found for issue")
	}

	// 4. Check for existing active review session
	if session.Status == models.SessionStatusActive && session.SessionType == models.SessionTypeReview {
		return nil, fmt.Errorf("review already in progress for issue")
	}

	// 5. Get project for prompt building
	project, err := l.store.GetProject(ctx, issue.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %w", err)
	}

	// 6. Reactivate session as review type
	session.Status = models.SessionStatusActive
	session.SessionType = models.SessionTypeReview
	session.ReviewAttempt++
	now := time.Now().UTC()
	session.LastActiveAt = &now
	if err := l.store.UpdateAgentSession(ctx, session); err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	// 7. Build the claude command
	systemPrompt := BuildReviewPrompt(issue, session, project, l.cfg)
	kickoff := BuildKickoffPrompt(issue)
	claudeCmd := l.buildClaudeCommand(systemPrompt, kickoff)

	// 8. Launch in iTerm
	sessionName := fmt.Sprintf("%s:review", session.Branch)
	if err := LaunchClaudeInITerm(session.WorktreePath, sessionName, claudeCmd); err != nil {
		return nil, fmt.Errorf("launch iTerm: %w", err)
	}

	return &LaunchResult{
		SessionID:    session.ID,
		IssueID:      issue.ID,
		Branch:       session.Branch,
		WorktreePath: session.WorktreePath,
		Command:      claudeCmd,
	}, nil
}

// buildClaudeCommand constructs the full claude CLI command with flags.
func (l *Launcher) buildClaudeCommand(systemPrompt, kickoff string) string {
	var parts []string
	parts = append(parts, "claude")

	// Add allowed tools
	if len(l.cfg.AllowedTools) > 0 {
		for _, tool := range l.cfg.AllowedTools {
			parts = append(parts, fmt.Sprintf("--allowedTools %q", tool))
		}
	}

	// Add system prompt (escaped for shell)
	parts = append(parts, fmt.Sprintf("--append-system-prompt %q", systemPrompt))

	// Add kickoff prompt as positional argument
	parts = append(parts, fmt.Sprintf("%q", kickoff))

	return strings.Join(parts, " ")
}

// findIssue finds an issue by full ID or prefix match.
func (l *Launcher) findIssue(ctx context.Context, id string) (*models.Issue, error) {
	// Try exact match
	if issue, err := l.store.GetIssue(ctx, id); err == nil {
		return issue, nil
	}

	// Try prefix match
	upper := strings.ToUpper(id)
	issues, err := l.store.ListIssues(ctx, store.IssueListFilter{})
	if err != nil {
		return nil, err
	}

	var matches []*models.Issue
	for _, issue := range issues {
		if strings.HasPrefix(issue.ID, upper) {
			matches = append(matches, issue)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("issue not found: %s", id)
	case 1:
		return l.store.GetIssue(ctx, matches[0].ID)
	default:
		return nil, fmt.Errorf("ambiguous issue ID %s: matches %d issues", id, len(matches))
	}
}
