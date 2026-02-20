package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/joescharf/pm/internal/agent"
	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/health"
	"github.com/joescharf/pm/internal/llm"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/sessions"
	"github.com/joescharf/pm/internal/store"
	"github.com/joescharf/pm/internal/wt"
)

// Server wraps the pm data layer and exposes it as MCP tools.
type Server struct {
	store    store.Store
	git      git.Client
	gh       git.GitHubClient
	wt       wt.Client
	llm      *llm.Client
	scorer   *health.Scorer
	sessions *sessions.Manager
}

// NewServer creates the MCP server wrapper with all required dependencies.
// The llmClient may be nil if no API key is configured.
func NewServer(s store.Store, gc git.Client, ghc git.GitHubClient, wtc wt.Client, llmClient *llm.Client) *Server {
	return &Server{
		store:    s,
		git:      gc,
		gh:       ghc,
		wt:       wtc,
		llm:      llmClient,
		scorer:   health.NewScorer(),
		sessions: sessions.NewManager(s),
	}
}

// MCPServer returns a configured mcp-go server with all tools registered.
func (s *Server) MCPServer() *server.MCPServer {
	srv := server.NewMCPServer("pm", "1.0.0", server.WithToolCapabilities(true))

	// Register all tools
	srv.AddTool(s.listProjectsTool())
	srv.AddTool(s.projectStatusTool())
	srv.AddTool(s.listIssuesTool())
	srv.AddTool(s.createIssueTool())
	srv.AddTool(s.updateIssueTool())
	srv.AddTool(s.healthScoreTool())
	srv.AddTool(s.launchAgentTool())
	srv.AddTool(s.closeAgentTool())
	srv.AddTool(s.syncSessionTool())
	srv.AddTool(s.mergeSessionTool())
	srv.AddTool(s.deleteWorktreeTool())
	srv.AddTool(s.discoverWorktreesTool())
	srv.AddTool(s.prepareReviewTool())
	srv.AddTool(s.saveReviewTool())
	srv.AddTool(s.updateProjectTool())

	return srv
}

// ServeStdio starts the stdio transport, blocking until ctx is cancelled.
func (s *Server) ServeStdio(ctx context.Context) error {
	srv := s.MCPServer()
	stdioServer := server.NewStdioServer(srv)
	return stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}

// ---------------------------------------------------------------------------
// Tool definitions and handlers
// ---------------------------------------------------------------------------

// pm_list_projects
func (s *Server) listProjectsTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_list_projects",
		mcp.WithDescription("List all tracked projects. Returns a JSON array of projects with id, name, path, description, language, and group."),
		mcp.WithString("group", mcp.Description("Filter by project group name")),
	)
	return tool, s.handleListProjects
}

func (s *Server) handleListProjects(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	group := request.GetString("group", "")
	projects, err := s.store.ListProjects(ctx, group)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list projects: %v", err)), nil
	}

	type projectOut struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Path        string `json:"path"`
		Description string `json:"description"`
		Language    string `json:"language"`
		Group       string `json:"group"`
	}

	out := make([]projectOut, len(projects))
	for i, p := range projects {
		out[i] = projectOut{
			ID:          p.ID,
			Name:        p.Name,
			Path:        p.Path,
			Description: p.Description,
			Language:    p.Language,
			Group:       p.GroupName,
		}
	}

	data, err := json.Marshal(out)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal projects: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// pm_project_status
func (s *Server) projectStatusTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_project_status",
		mcp.WithDescription("Get detailed project status including git info, health score, and issue counts. Resolves project by name."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name")),
	)
	return tool, s.handleProjectStatus
}

func (s *Server) handleProjectStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectName, err := request.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: project"), nil
	}

	p, err := s.resolveProject(ctx, projectName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", projectName)), nil
	}

	// Gather git info (best-effort)
	var branch, lastCommitHash, lastCommitMsg string
	var dirty bool
	var lastCommitDate time.Time
	var branchCount int

	if s.git != nil && p.Path != "" {
		branch, _ = s.git.CurrentBranch(p.Path)
		dirty, _ = s.git.IsDirty(p.Path)
		lastCommitDate, _ = s.git.LastCommitDate(p.Path)
		lastCommitHash, _ = s.git.LastCommitHash(p.Path)
		lastCommitMsg, _ = s.git.LastCommitMessage(p.Path)
		branches, _ := s.git.BranchList(p.Path)
		branchCount = len(branches)
	}

	// Gather issue counts
	allIssues, _ := s.store.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID})
	openCount, inProgressCount, doneCount, closedCount := 0, 0, 0, 0
	for _, issue := range allIssues {
		switch issue.Status {
		case models.IssueStatusOpen:
			openCount++
		case models.IssueStatusInProgress:
			inProgressCount++
		case models.IssueStatusDone:
			doneCount++
		case models.IssueStatusClosed:
			closedCount++
		}
	}

	// Compute health score
	meta := &health.ProjectMetadata{
		IsDirty:        dirty,
		LastCommitDate: lastCommitDate,
		BranchCount:    branchCount,
	}

	// Try to get release info for health score
	if s.gh != nil && p.RepoURL != "" {
		if owner, repo, err := git.ExtractOwnerRepo(p.RepoURL); err == nil {
			if rel, err := s.gh.LatestRelease(owner, repo); err == nil {
				meta.LatestRelease = rel.TagName
				if t, err := time.Parse(time.RFC3339, rel.PublishedAt); err == nil {
					meta.ReleaseDate = t
				}
			}
		}
	}

	hscore := s.scorer.Score(p, meta, allIssues)

	result := map[string]any{
		"project": map[string]any{
			"id":               p.ID,
			"name":             p.Name,
			"path":             p.Path,
			"description":      p.Description,
			"language":         p.Language,
			"group":            p.GroupName,
			"repo_url":         p.RepoURL,
			"has_github_pages": p.HasGitHubPages,
			"pages_url":        p.PagesURL,
		},
		"git": map[string]any{
			"branch":          branch,
			"dirty":           dirty,
			"last_commit_date": lastCommitDate.Format(time.RFC3339),
			"last_commit_hash": lastCommitHash,
			"last_commit_msg":  lastCommitMsg,
			"branch_count":     branchCount,
		},
		"issues": map[string]any{
			"total":       len(allIssues),
			"open":        openCount,
			"in_progress": inProgressCount,
			"done":        doneCount,
			"closed":      closedCount,
		},
		"health": map[string]any{
			"total":             hscore.Total,
			"git_cleanliness":   hscore.GitCleanliness,
			"activity_recency":  hscore.ActivityRecency,
			"issue_health":      hscore.IssueHealth,
			"release_freshness": hscore.ReleaseFreshness,
			"branch_hygiene":    hscore.BranchHygiene,
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal status: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// pm_list_issues
func (s *Server) listIssuesTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_list_issues",
		mcp.WithDescription("List issues, optionally filtered by project, status, and/or priority. Returns a JSON array of issues. Each issue has: title, description (short summary), body (raw original text with full context — use this for implementation details), ai_prompt (LLM-generated guidance for AI agents), status (open/in_progress/done/closed), priority (low/medium/high), type (feature/bug/chore), and tags."),
		mcp.WithString("project", mcp.Description("Project name to filter by")),
		mcp.WithString("status", mcp.Description("Status filter: open, in_progress, done, closed")),
		mcp.WithString("priority", mcp.Description("Priority filter: low, medium, high")),
	)
	return tool, s.handleListIssues
}

func (s *Server) handleListIssues(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filter := store.IssueListFilter{}

	projectName := request.GetString("project", "")
	if projectName != "" {
		p, err := s.resolveProject(ctx, projectName)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", projectName)), nil
		}
		filter.ProjectID = p.ID
	}

	status := request.GetString("status", "")
	if status != "" {
		filter.Status = models.IssueStatus(status)
	}

	priority := request.GetString("priority", "")
	if priority != "" {
		filter.Priority = models.IssuePriority(priority)
	}

	issues, err := s.store.ListIssues(ctx, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to list issues: %v", err)), nil
	}

	type issueOut struct {
		ID          string   `json:"id"`
		ProjectID   string   `json:"project_id"`
		Title       string   `json:"title"`
		Description string   `json:"description"`
		Body        string   `json:"body,omitempty"`
		AIPrompt    string   `json:"ai_prompt,omitempty"`
		Status      string   `json:"status"`
		Priority    string   `json:"priority"`
		Type        string   `json:"type"`
		Tags        []string `json:"tags"`
		GitHubIssue int      `json:"github_issue,omitempty"`
		CreatedAt   string   `json:"created_at"`
		UpdatedAt   string   `json:"updated_at"`
	}

	out := make([]issueOut, len(issues))
	for i, issue := range issues {
		out[i] = issueOut{
			ID:          issue.ID,
			ProjectID:   issue.ProjectID,
			Title:       issue.Title,
			Description: issue.Description,
			Body:        issue.Body,
			AIPrompt:    issue.AIPrompt,
			Status:      string(issue.Status),
			Priority:    string(issue.Priority),
			Type:        string(issue.Type),
			Tags:        issue.Tags,
			GitHubIssue: issue.GitHubIssue,
			CreatedAt:   issue.CreatedAt.Format(time.RFC3339),
			UpdatedAt:   issue.UpdatedAt.Format(time.RFC3339),
		}
	}

	data, err := json.Marshal(out)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal issues: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// pm_create_issue
func (s *Server) createIssueTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_create_issue",
		mcp.WithDescription("Create a new issue for a project. By default, uses LLM to generate a description and ai_prompt if not provided. Returns the created issue as JSON."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Issue title")),
		mcp.WithString("description", mcp.Description("Issue description")),
		mcp.WithString("body", mcp.Description("Raw body text (e.g. original issue text for full context)")),
		mcp.WithString("ai_prompt", mcp.Description("AI prompt providing guidance for AI agents working on this issue")),
		mcp.WithString("type", mcp.Description("Issue type: feature, bug, chore (default: feature)")),
		mcp.WithString("priority", mcp.Description("Issue priority: low, medium, high (default: medium)")),
		mcp.WithString("enrich", mcp.Description("Set to 'false' to skip LLM enrichment (default: true)")),
	)
	return tool, s.handleCreateIssue
}

func (s *Server) handleCreateIssue(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectName, err := request.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: project"), nil
	}
	title, err := request.RequireString("title")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: title"), nil
	}

	p, err := s.resolveProject(ctx, projectName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", projectName)), nil
	}

	issueType := request.GetString("type", "feature")
	priority := request.GetString("priority", "medium")
	description := request.GetString("description", "")
	body := request.GetString("body", "")
	aiPrompt := request.GetString("ai_prompt", "")
	enrich := request.GetString("enrich", "true")

	issue := &models.Issue{
		ProjectID:   p.ID,
		Title:       title,
		Description: description,
		Body:        body,
		AIPrompt:    aiPrompt,
		Status:      models.IssueStatusOpen,
		Priority:    models.IssuePriority(priority),
		Type:        models.IssueType(issueType),
	}

	// LLM enrichment (non-fatal)
	if enrich != "false" && s.llm != nil {
		enriched, enrichErr := s.llm.EnrichIssue(ctx, issue.Title, issue.Body, issue.Description)
		if enrichErr == nil {
			if issue.Description == "" && enriched.Description != "" {
				issue.Description = enriched.Description
			}
			if issue.AIPrompt == "" && enriched.AIPrompt != "" {
				issue.AIPrompt = enriched.AIPrompt
			}
		}
		// Silently ignore enrichment errors — issue will still be created
	}

	if err := s.store.CreateIssue(ctx, issue); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create issue: %v", err)), nil
	}

	result := map[string]any{
		"id":          issue.ID,
		"project_id":  p.ID,
		"project":     p.Name,
		"title":       issue.Title,
		"description": issue.Description,
		"body":        issue.Body,
		"ai_prompt":   issue.AIPrompt,
		"status":      string(issue.Status),
		"priority":    string(issue.Priority),
		"type":        string(issue.Type),
		"created_at":  issue.CreatedAt.Format(time.RFC3339),
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal issue: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// pm_update_issue
func (s *Server) updateIssueTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_update_issue",
		mcp.WithDescription("Update an existing issue. Provide the issue ID (full or prefix) and at least one field to update. Returns the updated issue as JSON."),
		mcp.WithString("issue_id", mcp.Required(), mcp.Description("Issue ID (full ULID or unique prefix)")),
		mcp.WithString("status", mcp.Description("New status: open, in_progress, done, closed")),
		mcp.WithString("title", mcp.Description("New title")),
		mcp.WithString("description", mcp.Description("New description")),
		mcp.WithString("body", mcp.Description("New body text")),
		mcp.WithString("ai_prompt", mcp.Description("New AI prompt (guidance for AI agents)")),
		mcp.WithString("priority", mcp.Description("New priority: low, medium, high")),
	)
	return tool, s.handleUpdateIssue
}

func (s *Server) handleUpdateIssue(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	issueID, err := request.RequireString("issue_id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: issue_id"), nil
	}

	issue, err := s.findIssue(ctx, issueID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("issue not found: %s", issueID)), nil
	}

	// Track whether any field was updated
	updated := false

	if status := request.GetString("status", ""); status != "" {
		issue.Status = models.IssueStatus(status)
		updated = true
		// Set ClosedAt when closing
		if status == string(models.IssueStatusClosed) || status == string(models.IssueStatusDone) {
			now := time.Now()
			issue.ClosedAt = &now
		}
	}
	if title := request.GetString("title", ""); title != "" {
		issue.Title = title
		updated = true
	}
	if desc := request.GetString("description", ""); desc != "" {
		issue.Description = desc
		updated = true
	}
	if body := request.GetString("body", ""); body != "" {
		issue.Body = body
		updated = true
	}
	if aiPrompt := request.GetString("ai_prompt", ""); aiPrompt != "" {
		issue.AIPrompt = aiPrompt
		updated = true
	}
	if priority := request.GetString("priority", ""); priority != "" {
		issue.Priority = models.IssuePriority(priority)
		updated = true
	}

	if !updated {
		return mcp.NewToolResultError("no fields provided to update; specify at least one of: status, title, description, body, ai_prompt, priority"), nil
	}

	if err := s.store.UpdateIssue(ctx, issue); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update issue: %v", err)), nil
	}

	result := map[string]any{
		"id":          issue.ID,
		"project_id":  issue.ProjectID,
		"title":       issue.Title,
		"description": issue.Description,
		"body":        issue.Body,
		"ai_prompt":   issue.AIPrompt,
		"status":      string(issue.Status),
		"priority":    string(issue.Priority),
		"type":        string(issue.Type),
		"updated_at":  issue.UpdatedAt.Format(time.RFC3339),
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal issue: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// pm_health_score
func (s *Server) healthScoreTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_health_score",
		mcp.WithDescription("Get the full health score breakdown for a project. Includes git cleanliness, activity recency, issue health, release freshness, and branch hygiene."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name")),
	)
	return tool, s.handleHealthScore
}

func (s *Server) handleHealthScore(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectName, err := request.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: project"), nil
	}

	p, err := s.resolveProject(ctx, projectName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", projectName)), nil
	}

	// Gather metadata
	meta := &health.ProjectMetadata{}

	if s.git != nil && p.Path != "" {
		meta.IsDirty, _ = s.git.IsDirty(p.Path)
		meta.LastCommitDate, _ = s.git.LastCommitDate(p.Path)
		branches, _ := s.git.BranchList(p.Path)
		meta.BranchCount = len(branches)
		worktrees, _ := s.git.WorktreeList(p.Path)
		meta.WorktreeCount = len(worktrees)
	}

	// Try to get release info
	if s.gh != nil && p.RepoURL != "" {
		if owner, repo, err := git.ExtractOwnerRepo(p.RepoURL); err == nil {
			if rel, err := s.gh.LatestRelease(owner, repo); err == nil {
				meta.LatestRelease = rel.TagName
				if t, err := time.Parse(time.RFC3339, rel.PublishedAt); err == nil {
					meta.ReleaseDate = t
				}
			}
		}
	}

	issues, _ := s.store.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID})
	hscore := s.scorer.Score(p, meta, issues)

	result := map[string]any{
		"project": p.Name,
		"health": map[string]any{
			"total":             hscore.Total,
			"git_cleanliness":   hscore.GitCleanliness,
			"activity_recency":  hscore.ActivityRecency,
			"issue_health":      hscore.IssueHealth,
			"release_freshness": hscore.ReleaseFreshness,
			"branch_hygiene":    hscore.BranchHygiene,
		},
		"metadata": map[string]any{
			"is_dirty":        meta.IsDirty,
			"last_commit":     meta.LastCommitDate.Format(time.RFC3339),
			"branch_count":    meta.BranchCount,
			"worktree_count":  meta.WorktreeCount,
			"latest_release":  meta.LatestRelease,
			"release_date":    meta.ReleaseDate.Format(time.RFC3339),
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal health score: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// pm_launch_agent
func (s *Server) launchAgentTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_launch_agent",
		mcp.WithDescription("Launch a Claude Code agent session in a new worktree. Creates a worktree, records the session, and returns the command to run. If an issue_id is provided, the issue is marked as in_progress."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name")),
		mcp.WithString("issue_id", mcp.Description("Issue ID to work on (generates branch name from title)")),
		mcp.WithString("branch", mcp.Description("Branch name (auto-generated from issue if not specified)")),
	)
	return tool, s.handleLaunchAgent
}

func (s *Server) handleLaunchAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectName, err := request.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: project"), nil
	}

	p, err := s.resolveProject(ctx, projectName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", projectName)), nil
	}

	issueID := request.GetString("issue_id", "")
	branch := request.GetString("branch", "")

	// If issue_id is provided, resolve the issue and optionally derive the branch name
	if issueID != "" {
		issue, err := s.findIssue(ctx, issueID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("issue not found: %s", issueID)), nil
		}
		issueID = issue.ID // normalize to full ID

		if branch == "" {
			branch = issueToBranch(issue.Title)
		}

		// Mark issue as in_progress
		issue.Status = models.IssueStatusInProgress
		if err := s.store.UpdateIssue(ctx, issue); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to update issue status: %v", err)), nil
		}
	}

	if branch == "" {
		return mcp.NewToolResultError("specify branch or issue_id to generate a branch name"), nil
	}

	// Determine worktree path to match wt's convention: {project}.worktrees/{last-branch-segment}
	branchParts := strings.Split(branch, "/")
	worktreeDirname := branchParts[len(branchParts)-1]
	worktreePath := filepath.Join(p.Path+".worktrees", worktreeDirname)

	// Check for existing idle session on this branch
	existingSessions, _ := s.store.ListAgentSessions(ctx, p.ID, 0)
	for _, sess := range existingSessions {
		if sess.Branch == branch && sess.Status == models.SessionStatusIdle {
			// Open iTerm window via wt open
			if s.wt != nil {
				if err := s.wt.Create(p.Path, branch); err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("wt open: %v", err)), nil
				}
			}
			sess.Status = models.SessionStatusActive
			now := time.Now().UTC()
			sess.LastActiveAt = &now
			if err := s.store.UpdateAgentSession(ctx, sess); err == nil {
				command := fmt.Sprintf("cd %s && claude", sess.WorktreePath)
				if issueID != "" {
					shortIssueID := issueID
					if len(shortIssueID) > 12 {
						shortIssueID = shortIssueID[:12]
					}
					command = fmt.Sprintf(`cd %s && claude "Use pm MCP tools to look up issue %s and implement it. Update the issue status when complete."`, sess.WorktreePath, shortIssueID)
				}
				result := map[string]any{
					"session_id":    sess.ID,
					"project":       p.Name,
					"branch":        branch,
					"worktree_path": sess.WorktreePath,
					"issue_id":      issueID,
					"status":        string(sess.Status),
					"resumed":       true,
					"command":       command,
				}
				data, _ := json.Marshal(result)
				return mcp.NewToolResultText(string(data)), nil
			}
		}
	}

	// Create worktree
	if s.wt == nil {
		return mcp.NewToolResultError("worktree client not available"), nil
	}
	if err := s.wt.Create(p.Path, branch); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to create worktree: %v", err)), nil
	}

	// Record agent session
	session := &models.AgentSession{
		ProjectID:    p.ID,
		IssueID:      issueID,
		Branch:       branch,
		WorktreePath: worktreePath,
		Status:       models.SessionStatusActive,
		StartedAt:    time.Now(),
	}
	if err := s.store.CreateAgentSession(ctx, session); err != nil {
		// Non-fatal: worktree was already created
		return mcp.NewToolResultError(fmt.Sprintf("worktree created but session recording failed: %v", err)), nil
	}

	command := fmt.Sprintf("cd %s && claude", worktreePath)
	if issueID != "" {
		shortIssueID := issueID
		if len(shortIssueID) > 12 {
			shortIssueID = shortIssueID[:12]
		}
		command = fmt.Sprintf(`cd %s && claude "Use pm MCP tools to look up issue %s and implement it. Update the issue status when complete."`, worktreePath, shortIssueID)
	}

	result := map[string]any{
		"session_id":    session.ID,
		"project":       p.Name,
		"branch":        branch,
		"worktree_path": worktreePath,
		"issue_id":      issueID,
		"status":        string(session.Status),
		"command":        command,
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal session: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// pm_close_agent
func (s *Server) closeAgentTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_close_agent",
		mcp.WithDescription("Close an agent session. Default transitions to idle. Use status=completed to mark done (issues → done) or status=abandoned to abandon (issues → open)."),
		mcp.WithString("session_id", mcp.Required(), mcp.Description("Session ID to close")),
		mcp.WithString("status", mcp.Description("Target status: idle (default), completed, abandoned")),
	)
	return tool, s.handleCloseAgent
}

func (s *Server) handleCloseAgent(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: session_id"), nil
	}

	targetStr := request.GetString("status", "idle")
	target := models.SessionStatus(targetStr)

	switch target {
	case models.SessionStatusIdle, models.SessionStatusCompleted, models.SessionStatusAbandoned:
	default:
		return mcp.NewToolResultError(fmt.Sprintf("invalid status: %s (must be idle, completed, or abandoned)", targetStr)), nil
	}

	// Enrich session with git info before closing; capture worktree path for iTerm cleanup
	var worktreePath string
	if sess, err := s.store.GetAgentSession(ctx, sessionID); err == nil {
		worktreePath = sess.WorktreePath
		agent.EnrichSessionWithGitInfo(sess, s.git)
		_ = s.store.UpdateAgentSession(ctx, sess)
	}

	session, err := agent.CloseSession(ctx, s.store, sessionID, target)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// Close iTerm window for terminal states (completed/abandoned)
	if worktreePath != "" && target != models.SessionStatusIdle {
		wtDir := filepath.Base(worktreePath)
		sessions.CloseITermWindowByName(wtDir)
	}

	result := map[string]any{
		"session_id": session.ID,
		"status":     string(session.Status),
	}
	if session.EndedAt != nil {
		result["ended_at"] = session.EndedAt.Format(time.RFC3339)
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

// pm_sync_session
func (s *Server) syncSessionTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_sync_session",
		mcp.WithDescription("Sync a session's worktree with the base branch. Fetches latest changes and merges/rebases the base branch into the feature branch."),
		mcp.WithString("session_id", mcp.Required(), mcp.Description("Session ID to sync")),
		mcp.WithString("rebase", mcp.Description("Set to 'true' to rebase instead of merge (default: false)")),
		mcp.WithString("force", mcp.Description("Set to 'true' to skip dirty worktree check (default: false)")),
		mcp.WithString("dry_run", mcp.Description("Set to 'true' for dry-run mode (default: false)")),
	)
	return tool, s.handleSyncSession
}

func (s *Server) handleSyncSession(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: session_id"), nil
	}

	opts := sessions.SyncOptions{
		Rebase: request.GetString("rebase", "") == "true",
		Force:  request.GetString("force", "") == "true",
		DryRun: request.GetString("dry_run", "") == "true",
	}

	result, err := s.sessions.SyncSession(ctx, sessionID, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("sync failed: %v", err)), nil
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

// pm_merge_session
func (s *Server) mergeSessionTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_merge_session",
		mcp.WithDescription("Merge a session's feature branch into the base branch. Can perform local merge or create a PR. After a successful local merge, automatically cleans up the worktree, branch, and iTerm window unless cleanup is disabled."),
		mcp.WithString("session_id", mcp.Required(), mcp.Description("Session ID to merge")),
		mcp.WithString("base_branch", mcp.Description("Target branch (default: main)")),
		mcp.WithString("rebase", mcp.Description("Set to 'true' to rebase instead of merge")),
		mcp.WithString("create_pr", mcp.Description("Set to 'true' to create a PR instead of local merge")),
		mcp.WithString("force", mcp.Description("Set to 'true' to skip safety checks")),
		mcp.WithString("dry_run", mcp.Description("Set to 'true' for dry-run mode")),
		mcp.WithString("cleanup", mcp.Description("Set to 'false' to skip post-merge cleanup of worktree, branch, and iTerm window (default: true)")),
	)
	return tool, s.handleMergeSession
}

func (s *Server) handleMergeSession(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: session_id"), nil
	}

	// Default cleanup to true unless explicitly set to "false"
	cleanup := request.GetString("cleanup", "") != "false"

	opts := sessions.MergeOptions{
		BaseBranch: request.GetString("base_branch", ""),
		Rebase:     request.GetString("rebase", "") == "true",
		CreatePR:   request.GetString("create_pr", "") == "true",
		Force:      request.GetString("force", "") == "true",
		DryRun:     request.GetString("dry_run", "") == "true",
		Cleanup:    cleanup,
	}

	result, err := s.sessions.MergeSession(ctx, sessionID, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("merge failed: %v", err)), nil
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

// pm_delete_worktree
func (s *Server) deleteWorktreeTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_delete_worktree",
		mcp.WithDescription("Delete a session's worktree. Marks the session as abandoned."),
		mcp.WithString("session_id", mcp.Required(), mcp.Description("Session ID whose worktree to delete")),
		mcp.WithString("force", mcp.Description("Set to 'true' to force removal even with uncommitted changes")),
	)
	return tool, s.handleDeleteWorktree
}

func (s *Server) handleDeleteWorktree(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sessionID, err := request.RequireString("session_id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: session_id"), nil
	}

	force := request.GetString("force", "") == "true"

	if err := s.sessions.DeleteWorktree(ctx, sessionID, force); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("delete worktree failed: %v", err)), nil
	}

	result := map[string]any{
		"session_id": sessionID,
		"status":     "abandoned",
		"message":    "worktree deleted successfully",
	}
	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

// pm_discover_worktrees
func (s *Server) discoverWorktreesTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_discover_worktrees",
		mcp.WithDescription("Scan a project's git repo for worktrees not tracked by pm. Creates idle session records for discovered worktrees."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name")),
	)
	return tool, s.handleDiscoverWorktrees
}

func (s *Server) handleDiscoverWorktrees(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectName, err := request.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: project"), nil
	}

	p, err := s.resolveProject(ctx, projectName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", projectName)), nil
	}

	discovered, err := s.sessions.DiscoverWorktrees(ctx, p.ID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("discovery failed: %v", err)), nil
	}

	var out []map[string]any
	for _, sess := range discovered {
		out = append(out, map[string]any{
			"session_id":    sess.ID,
			"branch":        sess.Branch,
			"worktree_path": sess.WorktreePath,
			"status":        string(sess.Status),
		})
	}

	result := map[string]any{
		"project":    projectName,
		"discovered": out,
		"count":      len(discovered),
	}
	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

// pm_prepare_review
func (s *Server) prepareReviewTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_prepare_review",
		mcp.WithDescription("Gather all context needed to review an issue's implementation. Returns issue requirements, git diff, changed files, UI review flags, and review history. The calling agent should analyze this context and then call pm_save_review with the verdict."),
		mcp.WithString("issue_id", mcp.Required(), mcp.Description("Issue ID (full ULID or unique prefix)")),
		mcp.WithString("base_ref", mcp.Description("Base ref for diff (default: main, or auto-detected from session branch)")),
		mcp.WithString("head_ref", mcp.Description("Head ref for diff (default: session branch, or HEAD)")),
		mcp.WithString("app_url", mcp.Description("URL of running app for UI/UX review via rodney (e.g. http://localhost:3000)")),
	)
	return tool, s.handlePrepareReview
}

func (s *Server) handlePrepareReview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	issueID, err := request.RequireString("issue_id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: issue_id"), nil
	}

	issue, err := s.findIssue(ctx, issueID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("issue not found: %s", issueID)), nil
	}

	project, err := s.store.GetProject(ctx, issue.ProjectID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found for issue: %v", err)), nil
	}

	// Find linked session (most recent for this issue)
	var session *models.AgentSession
	sessions, _ := s.store.ListAgentSessions(ctx, project.ID, 0)
	for _, sess := range sessions {
		if sess.IssueID == issue.ID {
			session = sess
			break
		}
	}

	// Determine diff refs
	baseRef := request.GetString("base_ref", "main")
	headRef := request.GetString("head_ref", "")
	if headRef == "" && session != nil && session.Branch != "" {
		headRef = session.Branch
	}
	if headRef == "" {
		headRef = "HEAD"
	}

	// Get diff (best-effort)
	var diff, diffStat string
	var filesChanged []string
	if s.git != nil && project.Path != "" {
		diff, _ = s.git.Diff(project.Path, baseRef, headRef)
		diffStat, _ = s.git.DiffStat(project.Path, baseRef, headRef)
		filesChanged, _ = s.git.DiffNameOnly(project.Path, baseRef, headRef)
	}

	// Check if UI review is needed
	uiReviewNeeded := false
	for _, f := range filesChanged {
		if strings.HasPrefix(f, "ui/") || strings.HasPrefix(f, "internal/ui/") {
			uiReviewNeeded = true
			break
		}
	}

	// Build UI context
	appURL := request.GetString("app_url", "")
	uiContext := map[string]any{
		"build_cmd":  project.BuildCmd,
		"serve_cmd":  project.ServeCmd,
		"serve_port": project.ServePort,
		"app_url":    appURL,
	}

	// Fetch review history
	reviews, _ := s.store.ListIssueReviews(ctx, issue.ID)
	var reviewHistory []map[string]any
	for _, r := range reviews {
		reviewHistory = append(reviewHistory, map[string]any{
			"verdict":     string(r.Verdict),
			"summary":     r.Summary,
			"reviewed_at": r.ReviewedAt.Format(time.RFC3339),
		})
	}

	// Build session info
	var sessionOut map[string]any
	if session != nil {
		sessionOut = map[string]any{
			"id":            session.ID,
			"branch":        session.Branch,
			"worktree_path": session.WorktreePath,
			"commit_count":  session.CommitCount,
		}
	}

	result := map[string]any{
		"issue": map[string]any{
			"id":          issue.ID,
			"title":       issue.Title,
			"description": issue.Description,
			"body":        issue.Body,
			"ai_prompt":   issue.AIPrompt,
			"type":        string(issue.Type),
			"priority":    string(issue.Priority),
			"status":      string(issue.Status),
		},
		"session":          sessionOut,
		"diff":             diff,
		"diff_stats":       diffStat,
		"files_changed":    filesChanged,
		"ui_review_needed": uiReviewNeeded,
		"ui_context":       uiContext,
		"review_history":   reviewHistory,
		"project": map[string]any{
			"name":     project.Name,
			"path":     project.Path,
			"language": project.Language,
		},
	}

	data, err := json.Marshal(result)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to marshal review context: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// pm_save_review
func (s *Server) saveReviewTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_save_review",
		mcp.WithDescription("Save the result of an issue review. On pass, transitions issue to closed. On fail, transitions issue to in_progress with failure reasons. Creates a historical review record."),
		mcp.WithString("issue_id", mcp.Required(), mcp.Description("Issue ID (full ULID or unique prefix)")),
		mcp.WithString("verdict", mcp.Required(), mcp.Description("Review verdict: pass or fail")),
		mcp.WithString("summary", mcp.Required(), mcp.Description("Narrative review summary")),
		mcp.WithString("code_quality", mcp.Description("Code quality assessment: pass, fail, or skip")),
		mcp.WithString("requirements_match", mcp.Description("Requirements match assessment: pass, fail, or skip")),
		mcp.WithString("test_coverage", mcp.Description("Test coverage assessment: pass, fail, or skip")),
		mcp.WithString("ui_ux", mcp.Description("UI/UX assessment: pass, fail, skip, or na")),
		mcp.WithString("failure_reasons", mcp.Description("Newline-separated list of failure reasons (for fail verdicts)")),
		mcp.WithString("diff_stats", mcp.Description("Diff statistics string")),
	)
	return tool, s.handleSaveReview
}

func (s *Server) handleSaveReview(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	issueID, err := request.RequireString("issue_id")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: issue_id"), nil
	}
	verdict, err := request.RequireString("verdict")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: verdict"), nil
	}
	summary, err := request.RequireString("summary")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: summary"), nil
	}

	if verdict != "pass" && verdict != "fail" {
		return mcp.NewToolResultError("verdict must be 'pass' or 'fail'"), nil
	}

	issue, err := s.findIssue(ctx, issueID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("issue not found: %s", issueID)), nil
	}

	// Find linked session
	var sessionID string
	sessions, _ := s.store.ListAgentSessions(ctx, issue.ProjectID, 0)
	for _, sess := range sessions {
		if sess.IssueID == issue.ID {
			sessionID = sess.ID
			break
		}
	}

	// Parse failure reasons
	var failureReasons []string
	if fr := request.GetString("failure_reasons", ""); fr != "" {
		for _, line := range strings.Split(fr, "\n") {
			line = strings.TrimSpace(line)
			if line != "" {
				failureReasons = append(failureReasons, line)
			}
		}
	}

	review := &models.IssueReview{
		IssueID:           issue.ID,
		SessionID:         sessionID,
		Verdict:           models.ReviewVerdict(verdict),
		Summary:           summary,
		CodeQuality:       models.ReviewCategory(request.GetString("code_quality", "skip")),
		RequirementsMatch: models.ReviewCategory(request.GetString("requirements_match", "skip")),
		TestCoverage:      models.ReviewCategory(request.GetString("test_coverage", "skip")),
		UIUX:              models.ReviewCategory(request.GetString("ui_ux", "na")),
		FailureReasons:    failureReasons,
		DiffStats:         request.GetString("diff_stats", ""),
		ReviewedAt:        time.Now().UTC(),
	}

	if err := s.store.CreateIssueReview(ctx, review); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to save review: %v", err)), nil
	}

	// Transition issue status
	if verdict == "pass" {
		issue.Status = models.IssueStatusClosed
		now := time.Now().UTC()
		issue.ClosedAt = &now
	} else {
		issue.Status = models.IssueStatusInProgress
		issue.ClosedAt = nil
	}
	if err := s.store.UpdateIssue(ctx, issue); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("review saved but issue update failed: %v", err)), nil
	}

	result := map[string]any{
		"review_id":    review.ID,
		"issue_id":     issue.ID,
		"verdict":      verdict,
		"issue_status": string(issue.Status),
		"summary":      summary,
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

// pm_update_project
func (s *Server) updateProjectTool() (mcp.Tool, server.ToolHandlerFunc) {
	tool := mcp.NewTool("pm_update_project",
		mcp.WithDescription("Update project metadata. Use this to persist discovered build/serve commands for automated UI review."),
		mcp.WithString("project", mcp.Required(), mcp.Description("Project name")),
		mcp.WithString("description", mcp.Description("New project description")),
		mcp.WithString("build_cmd", mcp.Description("Build command (e.g. 'npm run build', 'make ui-build')")),
		mcp.WithString("serve_cmd", mcp.Description("Dev server command (e.g. 'npm run dev', 'bun run dev')")),
		mcp.WithString("serve_port", mcp.Description("Dev server port as string (e.g. '3000', '5173')")),
	)
	return tool, s.handleUpdateProject
}

func (s *Server) handleUpdateProject(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectName, err := request.RequireString("project")
	if err != nil {
		return mcp.NewToolResultError("missing required parameter: project"), nil
	}

	p, err := s.resolveProject(ctx, projectName)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %s", projectName)), nil
	}

	updated := false

	if desc := request.GetString("description", ""); desc != "" {
		p.Description = desc
		updated = true
	}
	if cmd := request.GetString("build_cmd", ""); cmd != "" {
		p.BuildCmd = cmd
		updated = true
	}
	if cmd := request.GetString("serve_cmd", ""); cmd != "" {
		p.ServeCmd = cmd
		updated = true
	}
	if portStr := request.GetString("serve_port", ""); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
			p.ServePort = port
			updated = true
		}
	}

	if !updated {
		return mcp.NewToolResultError("no fields provided to update"), nil
	}

	if err := s.store.UpdateProject(ctx, p); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed to update project: %v", err)), nil
	}

	result := map[string]any{
		"id":          p.ID,
		"name":        p.Name,
		"description": p.Description,
		"build_cmd":   p.BuildCmd,
		"serve_cmd":   p.ServeCmd,
		"serve_port":  p.ServePort,
	}

	data, _ := json.Marshal(result)
	return mcp.NewToolResultText(string(data)), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// resolveProject tries to find a project by name first, then by ID.
func (s *Server) resolveProject(ctx context.Context, name string) (*models.Project, error) {
	if p, err := s.store.GetProjectByName(ctx, name); err == nil {
		return p, nil
	}
	if p, err := s.store.GetProject(ctx, name); err == nil {
		return p, nil
	}
	return nil, fmt.Errorf("project not found: %s", name)
}

// findIssue finds an issue by full ID or unique prefix.
func (s *Server) findIssue(ctx context.Context, id string) (*models.Issue, error) {
	// Try exact match first
	if issue, err := s.store.GetIssue(ctx, id); err == nil {
		return issue, nil
	}

	// Try prefix match - list all and filter
	upper := strings.ToUpper(id)
	issues, err := s.store.ListIssues(ctx, store.IssueListFilter{})
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
		// Re-fetch to get tags loaded
		return s.store.GetIssue(ctx, matches[0].ID)
	default:
		return nil, fmt.Errorf("ambiguous issue ID %s: matches %d issues", id, len(matches))
	}
}

// issueToBranch converts an issue title to a branch name.
func issueToBranch(title string) string {
	s := strings.ToLower(title)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, s)
	parts := strings.Split(s, "-")
	var clean []string
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}
	result := strings.Join(clean, "-")
	if len(result) > 50 {
		result = result[:50]
	}
	return "feature/" + result
}
