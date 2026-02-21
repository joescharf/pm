package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joescharf/pm/internal/agent"
	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/health"
	"github.com/joescharf/pm/internal/llm"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/refresh"
	"github.com/joescharf/pm/internal/sessions"
	"github.com/joescharf/pm/internal/store"
	"github.com/joescharf/pm/internal/wt"
)

// Server provides the REST API handlers.
type Server struct {
	store    store.Store
	git      git.Client
	gh       git.GitHubClient
	wt       wt.Client
	llm      *llm.Client
	scorer   *health.Scorer
	sessions *sessions.Manager
}

// NewServer creates a new API server.
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

// Router returns an http.Handler for the API routes.
func (s *Server) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/v1/projects", s.listProjects)
	mux.HandleFunc("POST /api/v1/projects", s.createProject)
	mux.HandleFunc("GET /api/v1/projects/{id}", s.getProject)
	mux.HandleFunc("PUT /api/v1/projects/{id}", s.updateProject)
	mux.HandleFunc("DELETE /api/v1/projects/{id}", s.deleteProject)

	mux.HandleFunc("POST /api/v1/projects/refresh", s.refreshAllProjects)

	mux.HandleFunc("GET /api/v1/projects/{id}/issues", s.listProjectIssues)
	mux.HandleFunc("POST /api/v1/projects/{id}/issues", s.createProjectIssue)

	mux.HandleFunc("GET /api/v1/issues", s.listIssues)
	mux.HandleFunc("POST /api/v1/issues/bulk-update", s.bulkUpdateIssues)
	mux.HandleFunc("POST /api/v1/issues/bulk-delete", s.bulkDeleteIssues)
	mux.HandleFunc("GET /api/v1/issues/{id}", s.getIssue)
	mux.HandleFunc("PUT /api/v1/issues/{id}", s.updateIssue)
	mux.HandleFunc("DELETE /api/v1/issues/{id}", s.deleteIssue)
	mux.HandleFunc("POST /api/v1/issues/{id}/enrich", s.enrichIssue)

	mux.HandleFunc("GET /api/v1/issues/{id}/reviews", s.listIssueReviews)
	mux.HandleFunc("POST /api/v1/issues/{id}/reviews", s.createIssueReview)

	mux.HandleFunc("GET /api/v1/status", s.statusOverview)
	mux.HandleFunc("GET /api/v1/status/{id}", s.statusProject)

	mux.HandleFunc("GET /api/v1/sessions", s.listSessions)
	mux.HandleFunc("DELETE /api/v1/sessions/cleanup", s.cleanupSessions)
	mux.HandleFunc("GET /api/v1/sessions/{id}", s.getSession)
	mux.HandleFunc("POST /api/v1/sessions/{id}/sync", s.syncSession)
	mux.HandleFunc("POST /api/v1/sessions/{id}/merge", s.mergeSession)
	mux.HandleFunc("DELETE /api/v1/sessions/{id}/worktree", s.deleteWorktree)
	mux.HandleFunc("GET /api/v1/sessions/{id}/close-check", s.closeCheck)
	mux.HandleFunc("POST /api/v1/sessions/{id}/reactivate", s.reactivateSession)
	mux.HandleFunc("POST /api/v1/sessions/discover", s.discoverWorktrees)

	mux.HandleFunc("GET /api/v1/tags", s.listTags)

	mux.HandleFunc("GET /api/v1/health/{id}", s.projectHealth)

	mux.HandleFunc("POST /api/v1/agent/launch", s.launchAgent)
	mux.HandleFunc("POST /api/v1/agent/resume", s.resumeAgent)
	mux.HandleFunc("POST /api/v1/agent/close", s.closeAgent)

	return corsMiddleware(mux)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// patchString applies a string value from a JSON patch map to the target if the key is present and non-empty.
func patchString(patch map[string]any, key string, target *string) {
	if v, ok := patch[key]; ok {
		if str, ok := v.(string); ok && str != "" {
			*target = str
		}
	}
}

// --- Projects ---

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	group := r.URL.Query().Get("group")
	projects, err := s.store.ListProjects(r.Context(), group)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, projects)
}

func (s *Server) getProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, err := s.store.GetProject(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, project)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var p models.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := s.store.CreateProject(r.Context(), &p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.store.GetProject(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Selectively merge only keys present in the patch with non-empty values.
	// Empty strings are treated as "not provided" to avoid wiping existing data.
	patchString(patch, "Name", &existing.Name)
	patchString(patch, "Path", &existing.Path)
	patchString(patch, "Description", &existing.Description)
	patchString(patch, "RepoURL", &existing.RepoURL)
	patchString(patch, "Language", &existing.Language)
	patchString(patch, "GroupName", &existing.GroupName)

	if err := s.store.UpdateProject(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, existing)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteProject(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) refreshAllProjects(w http.ResponseWriter, r *http.Request) {
	result, err := refresh.All(r.Context(), s.store, s.git, s.gh)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// --- Issues ---

func (s *Server) listIssues(w http.ResponseWriter, r *http.Request) {
	filter := store.IssueListFilter{
		Status:   models.IssueStatus(r.URL.Query().Get("status")),
		Priority: models.IssuePriority(r.URL.Query().Get("priority")),
		Tag:      r.URL.Query().Get("tag"),
	}
	issues, err := s.store.ListIssues(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, issues)
}

func (s *Server) listProjectIssues(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	filter := store.IssueListFilter{ProjectID: projectID}
	issues, err := s.store.ListIssues(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, issues)
}

func (s *Server) createProjectIssue(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	var issue models.Issue
	if err := json.NewDecoder(r.Body).Decode(&issue); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	issue.ProjectID = projectID
	if issue.Status == "" {
		issue.Status = models.IssueStatusOpen
	}
	if issue.Priority == "" {
		issue.Priority = models.IssuePriorityMedium
	}
	if issue.Type == "" {
		issue.Type = models.IssueTypeFeature
	}

	// Auto-enrich if LLM available and AIPrompt not already set
	if s.llm != nil && issue.AIPrompt == "" {
		enriched, err := s.llm.EnrichIssue(r.Context(), issue.Title, issue.Body, issue.Description)
		if err == nil {
			if issue.Description == "" && enriched.Description != "" {
				issue.Description = enriched.Description
			}
			if enriched.AIPrompt != "" {
				issue.AIPrompt = enriched.AIPrompt
			}
		}
	}

	if err := s.store.CreateIssue(r.Context(), &issue); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, issue)
}

func (s *Server) getIssue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	issue, err := s.store.GetIssue(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (s *Server) updateIssue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var issue models.Issue
	if err := json.NewDecoder(r.Body).Decode(&issue); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	issue.ID = id
	if err := s.store.UpdateIssue(r.Context(), &issue); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (s *Server) deleteIssue(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteIssue(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) enrichIssue(w http.ResponseWriter, r *http.Request) {
	if s.llm == nil {
		writeError(w, http.StatusServiceUnavailable, "LLM not configured (set ANTHROPIC_API_KEY)")
		return
	}

	id := r.PathValue("id")
	issue, err := s.store.GetIssue(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	enriched, err := s.llm.EnrichIssue(r.Context(), issue.Title, issue.Body, issue.Description)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("LLM enrichment failed: %v", err))
		return
	}

	if enriched.Description != "" {
		issue.Description = enriched.Description
	}
	if enriched.AIPrompt != "" {
		issue.AIPrompt = enriched.AIPrompt
	}

	if err := s.store.UpdateIssue(r.Context(), issue); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, issue)
}

func (s *Server) bulkUpdateIssues(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	if req.Status == "" {
		writeError(w, http.StatusBadRequest, "status is required")
		return
	}
	n, err := s.store.BulkUpdateIssueStatus(r.Context(), req.IDs, models.IssueStatus(req.Status))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"updated": n})
}

func (s *Server) bulkDeleteIssues(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	n, err := s.store.BulkDeleteIssues(r.Context(), req.IDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": n})
}

// --- Issue Reviews ---

func (s *Server) listIssueReviews(w http.ResponseWriter, r *http.Request) {
	issueID := r.PathValue("id")
	reviews, err := s.store.ListIssueReviews(r.Context(), issueID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if reviews == nil {
		reviews = []*models.IssueReview{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reviews)
}

func (s *Server) createIssueReview(w http.ResponseWriter, r *http.Request) {
	issueID := r.PathValue("id")

	var body struct {
		Verdict           string   `json:"verdict"`
		Summary           string   `json:"summary"`
		CodeQuality       string   `json:"code_quality"`
		RequirementsMatch string   `json:"requirements_match"`
		TestCoverage      string   `json:"test_coverage"`
		UIUX              string   `json:"ui_ux"`
		FailureReasons    []string `json:"failure_reasons"`
		DiffStats         string   `json:"diff_stats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if body.Verdict != "pass" && body.Verdict != "fail" {
		writeError(w, http.StatusBadRequest, "verdict must be 'pass' or 'fail'")
		return
	}
	if body.Summary == "" {
		writeError(w, http.StatusBadRequest, "summary is required")
		return
	}

	review := &models.IssueReview{
		IssueID:           issueID,
		Verdict:           models.ReviewVerdict(body.Verdict),
		Summary:           body.Summary,
		CodeQuality:       models.ReviewCategory(body.CodeQuality),
		RequirementsMatch: models.ReviewCategory(body.RequirementsMatch),
		TestCoverage:      models.ReviewCategory(body.TestCoverage),
		UIUX:              models.ReviewCategory(body.UIUX),
		FailureReasons:    body.FailureReasons,
		DiffStats:         body.DiffStats,
		ReviewedAt:        time.Now().UTC(),
	}

	if err := s.store.CreateIssueReview(r.Context(), review); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Transition issue status based on verdict (matches MCP pm_save_review behavior)
	if issue, err := s.store.GetIssue(r.Context(), issueID); err == nil {
		if body.Verdict == "pass" {
			issue.Status = models.IssueStatusClosed
			now := time.Now().UTC()
			issue.ClosedAt = &now
		} else {
			issue.Status = models.IssueStatusInProgress
			issue.ClosedAt = nil
		}
		_ = s.store.UpdateIssue(r.Context(), issue)
	}

	writeJSON(w, http.StatusCreated, review)
}

// --- Status ---

type statusEntry struct {
	Project       *models.Project    `json:"project"`
	Branch        string             `json:"branch"`
	IsDirty       bool               `json:"isDirty"`
	OpenIssues    int                `json:"openIssues"`
	InProgress    int                `json:"inProgressIssues"`
	Health        int                `json:"health"`
	LastActivity  string             `json:"lastActivity"`
	LatestVersion string             `json:"latestVersion,omitempty"`
	ReleaseDate   string             `json:"releaseDate,omitempty"`
	VersionSource string             `json:"versionSource,omitempty"`
	ReleaseAssets []git.ReleaseAsset `json:"releaseAssets,omitempty"`
}

func (s *Server) statusOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projects, err := s.store.ListProjects(ctx, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var entries []statusEntry
	for _, p := range projects {
		entry := s.buildStatusEntry(ctx, p)
		entries = append(entries, entry)
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) statusProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	entry := s.buildStatusEntry(ctx, p)
	writeJSON(w, http.StatusOK, entry)
}

func (s *Server) buildStatusEntry(ctx context.Context, p *models.Project) statusEntry {
	entry := statusEntry{Project: p}
	meta := &health.ProjectMetadata{}

	// Git info
	if branch, err := s.git.CurrentBranch(p.Path); err == nil {
		entry.Branch = branch
	}
	if dirty, err := s.git.IsDirty(p.Path); err == nil {
		entry.IsDirty = dirty
		meta.IsDirty = dirty
	}
	if date, err := s.git.LastCommitDate(p.Path); err == nil {
		entry.LastActivity = date.Format("2006-01-02T15:04:05Z")
		meta.LastCommitDate = date
	}
	if branches, err := s.git.BranchList(p.Path); err == nil {
		meta.BranchCount = len(branches)
	}

	// Issues
	issues, _ := s.store.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID})
	for _, i := range issues {
		switch i.Status {
		case models.IssueStatusOpen:
			entry.OpenIssues++
		case models.IssueStatusInProgress:
			entry.InProgress++
		}
	}

	// Version info: GitHub release primary, local git tag fallback
	if p.RepoURL != "" {
		if owner, repo, err := git.ExtractOwnerRepo(p.RepoURL); err == nil {
			if rel, err := s.gh.LatestRelease(owner, repo); err == nil {
				entry.LatestVersion = rel.TagName
				entry.ReleaseDate = rel.PublishedAt
				entry.VersionSource = "github"
				entry.ReleaseAssets = rel.Assets
				if t, parseErr := time.Parse(time.RFC3339, rel.PublishedAt); parseErr == nil {
					meta.LatestRelease = rel.TagName
					meta.ReleaseDate = t
				}
			}
		}
	}
	if entry.LatestVersion == "" {
		if tag, err := s.git.LatestTag(p.Path); err == nil {
			entry.LatestVersion = tag
			entry.VersionSource = "git-tag"
			meta.LatestRelease = tag
		}
	}

	// Health score (with fully populated meta)
	h := s.scorer.Score(p, meta, issues)
	entry.Health = h.Total

	return entry
}

// --- Sessions ---

type sessionResponse struct {
	*models.AgentSession
	ProjectName string `json:"ProjectName"`
}

type sessionDetailResponse struct {
	*models.AgentSession
	ProjectName    string `json:"ProjectName"`
	WorktreeExists bool   `json:"WorktreeExists"`
	IsDirty        bool   `json:"IsDirty,omitempty"`
	CurrentBranch  string `json:"CurrentBranch,omitempty"`
	AheadCount     int    `json:"AheadCount,omitempty"`
	BehindCount    int    `json:"BehindCount,omitempty"`
}

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	statusFilter := r.URL.Query().Get("status")

	var allSessions []*models.AgentSession
	var err error

	if statusFilter != "" {
		// Parse comma-separated statuses
		var statuses []models.SessionStatus
		for _, st := range strings.Split(statusFilter, ",") {
			st = strings.TrimSpace(st)
			if st != "" {
				statuses = append(statuses, models.SessionStatus(st))
			}
		}
		allSessions, err = s.store.ListAgentSessionsByStatus(r.Context(), projectID, statuses, 50)
	} else {
		allSessions, err = s.store.ListAgentSessions(r.Context(), projectID, 50)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Lightweight reconcile: check worktree status for returned sessions only
	agent.ReconcileSessions(r.Context(), s.store, allSessions)
	sessions := allSessions

	// Build enriched responses with project names (cached by project ID)
	nameCache := make(map[string]string)
	result := make([]sessionResponse, 0, len(sessions))
	for _, sess := range sessions {
		name, ok := nameCache[sess.ProjectID]
		if !ok {
			if p, err := s.store.GetProject(r.Context(), sess.ProjectID); err == nil {
				name = p.Name
			}
			nameCache[sess.ProjectID] = name
		}
		result = append(result, sessionResponse{
			AgentSession: sess,
			ProjectName:  name,
		})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sess, err := s.store.GetAgentSession(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Resolve project name
	var projectName string
	if p, err := s.store.GetProject(r.Context(), sess.ProjectID); err == nil {
		projectName = p.Name
	}

	resp := sessionDetailResponse{
		AgentSession: sess,
		ProjectName:  projectName,
	}

	// Check if worktree path exists and enrich with live git data
	if _, err := os.Stat(sess.WorktreePath); err == nil {
		resp.WorktreeExists = true

		if dirty, err := s.git.IsDirty(sess.WorktreePath); err == nil {
			resp.IsDirty = dirty
		}
		if branch, err := s.git.CurrentBranch(sess.WorktreePath); err == nil {
			resp.CurrentBranch = branch
		}
		if ahead, behind, err := s.git.AheadBehind(sess.WorktreePath, "main"); err == nil {
			resp.AheadCount = ahead
			resp.BehindCount = behind
			// Use ahead count as commit count when stored value is stale
			if ahead > sess.CommitCount {
				sess.CommitCount = ahead
			}
		}
		if hash, err := s.git.LastCommitHash(sess.WorktreePath); err == nil {
			sess.LastCommitHash = hash
		}
		if msg, err := s.git.LastCommitMessage(sess.WorktreePath); err == nil {
			sess.LastCommitMessage = msg
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Session Operations ---

func (s *Server) syncSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Rebase bool `json:"rebase"`
		Force  bool `json:"force"`
		DryRun bool `json:"dry_run"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
	}

	result, err := s.sessions.SyncSession(r.Context(), id, sessions.SyncOptions{
		Rebase: req.Rebase,
		Force:  req.Force,
		DryRun: req.DryRun,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) mergeSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		BaseBranch string `json:"base_branch"`
		Rebase     bool   `json:"rebase"`
		CreatePR   bool   `json:"create_pr"`
		PRTitle    string `json:"pr_title"`
		PRBody     string `json:"pr_body"`
		PRDraft    bool   `json:"pr_draft"`
		Force      bool   `json:"force"`
		DryRun     bool   `json:"dry_run"`
		Cleanup    *bool  `json:"cleanup,omitempty"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
	}

	// Default cleanup to true when not specified
	cleanup := true
	if req.Cleanup != nil {
		cleanup = *req.Cleanup
	}

	result, err := s.sessions.MergeSession(r.Context(), id, sessions.MergeOptions{
		BaseBranch: req.BaseBranch,
		Rebase:     req.Rebase,
		CreatePR:   req.CreatePR,
		PRTitle:    req.PRTitle,
		PRBody:     req.PRBody,
		PRDraft:    req.PRDraft,
		Force:      req.Force,
		DryRun:     req.DryRun,
		Cleanup:    cleanup,
	})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) deleteWorktree(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Force bool `json:"force"`
	}
	if r.Body != nil && r.ContentLength > 0 {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if err := s.sessions.DeleteWorktree(r.Context(), id, req.Force); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Close Check ---

type closeCheckWarning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type closeCheckResponse struct {
	SessionID      string              `json:"session_id"`
	WorktreeExists bool                `json:"worktree_exists"`
	IsDirty        bool                `json:"is_dirty"`
	AheadCount     int                 `json:"ahead_count"`
	BehindCount    int                 `json:"behind_count"`
	ConflictState  string              `json:"conflict_state"`
	Branch         string              `json:"branch"`
	BaseBranch     string              `json:"base_branch"`
	ReadyToClose   bool                `json:"ready_to_close"`
	Warnings       []closeCheckWarning `json:"warnings"`
}

func (s *Server) closeCheck(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	sess, err := s.store.GetAgentSession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	resp := closeCheckResponse{
		SessionID:     sess.ID,
		Branch:        sess.Branch,
		BaseBranch:    "main",
		ConflictState: string(sess.ConflictState),
	}

	if sess.WorktreePath != "" {
		if _, err := os.Stat(sess.WorktreePath); err == nil {
			resp.WorktreeExists = true

			if dirty, err := s.git.IsDirty(sess.WorktreePath); err == nil {
				resp.IsDirty = dirty
			}
			if ahead, behind, err := s.git.AheadBehind(sess.WorktreePath, "main"); err == nil {
				resp.AheadCount = ahead
				resp.BehindCount = behind
			}
		}
	}

	// Build warnings
	if resp.IsDirty {
		resp.Warnings = append(resp.Warnings, closeCheckWarning{
			Type:    "dirty",
			Message: "Worktree has uncommitted changes",
		})
	}
	if resp.AheadCount > 0 {
		resp.Warnings = append(resp.Warnings, closeCheckWarning{
			Type:    "unmerged",
			Message: fmt.Sprintf("%d commit(s) not merged to main", resp.AheadCount),
		})
	}
	if resp.BehindCount > 0 {
		resp.Warnings = append(resp.Warnings, closeCheckWarning{
			Type:    "behind",
			Message: fmt.Sprintf("%d commit(s) behind main", resp.BehindCount),
		})
	}
	if sess.ConflictState != models.ConflictStateNone {
		resp.Warnings = append(resp.Warnings, closeCheckWarning{
			Type:    "conflict",
			Message: fmt.Sprintf("Session has %s", sess.ConflictState),
		})
	}

	resp.ReadyToClose = !resp.IsDirty && resp.AheadCount == 0 && sess.ConflictState == models.ConflictStateNone

	if resp.Warnings == nil {
		resp.Warnings = []closeCheckWarning{}
	}

	writeJSON(w, http.StatusOK, resp)
}

// --- Reactivate Session ---

func (s *Server) reactivateSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	sess, err := s.store.GetAgentSession(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Verify worktree exists
	if sess.WorktreePath == "" {
		writeError(w, http.StatusBadRequest, "session has no worktree path")
		return
	}
	if _, err := os.Stat(sess.WorktreePath); err != nil {
		writeError(w, http.StatusBadRequest, "worktree no longer exists on disk")
		return
	}

	session, err := agent.ReactivateSession(r.Context(), s.store, id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": session.ID,
		"status":     session.Status,
	})
}

func (s *Server) discoverWorktrees(w http.ResponseWriter, r *http.Request) {
	// Accept project_id from query param or JSON body
	projectID := r.URL.Query().Get("project_id")
	if projectID == "" && r.Body != nil && r.ContentLength > 0 {
		var req struct {
			ProjectID string `json:"project_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			projectID = req.ProjectID
		}
	}

	var allDiscovered []*models.AgentSession

	if projectID != "" {
		// Discover for a specific project
		discovered, err := s.sessions.DiscoverWorktrees(r.Context(), projectID)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		allDiscovered = discovered
	} else {
		// Discover across all projects
		projects, err := s.store.ListProjects(r.Context(), "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		for _, p := range projects {
			discovered, err := s.sessions.DiscoverWorktrees(r.Context(), p.ID)
			if err != nil {
				// Skip projects that fail (e.g., missing repo)
				continue
			}
			allDiscovered = append(allDiscovered, discovered...)
		}
	}

	if allDiscovered == nil {
		allDiscovered = []*models.AgentSession{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"discovered": allDiscovered,
		"count":      len(allDiscovered),
	})
}

// --- Cleanup ---

func (s *Server) cleanupSessions(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.DeleteAllStaleSessions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": count})
}

// --- Tags ---

func (s *Server) listTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.store.ListTags(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, tags)
}

// --- Health ---

func (s *Server) projectHealth(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()

	p, err := s.store.GetProject(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	meta := &health.ProjectMetadata{}
	if dirty, err := s.git.IsDirty(p.Path); err == nil {
		meta.IsDirty = dirty
	}
	if date, err := s.git.LastCommitDate(p.Path); err == nil {
		meta.LastCommitDate = date
	}
	if branches, err := s.git.BranchList(p.Path); err == nil {
		meta.BranchCount = len(branches)
	}

	// Version info for release freshness scoring
	if p.RepoURL != "" {
		if owner, repo, err := git.ExtractOwnerRepo(p.RepoURL); err == nil {
			if rel, err := s.gh.LatestRelease(owner, repo); err == nil {
				meta.LatestRelease = rel.TagName
				if t, parseErr := time.Parse(time.RFC3339, rel.PublishedAt); parseErr == nil {
					meta.ReleaseDate = t
				}
			}
		}
	}
	if meta.LatestRelease == "" {
		if tag, err := s.git.LatestTag(p.Path); err == nil {
			meta.LatestRelease = tag
		}
	}

	issues, _ := s.store.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID})
	h := s.scorer.Score(p, meta, issues)
	writeJSON(w, http.StatusOK, h)
}

// --- Agent Launch ---

// LaunchAgentRequest is the JSON body for POST /api/v1/agent/launch.
type LaunchAgentRequest struct {
	IssueIDs  []string `json:"issue_ids"`
	ProjectID string   `json:"project_id"`
}

// LaunchAgentResponse is the JSON response for a successful agent launch.
type LaunchAgentResponse struct {
	SessionID    string `json:"session_id"`
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktree_path"`
	Command      string `json:"command"`
}

func (s *Server) launchAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req LaunchAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.ProjectID == "" {
		writeError(w, http.StatusBadRequest, "project_id is required")
		return
	}
	if len(req.IssueIDs) == 0 {
		writeError(w, http.StatusBadRequest, "issue_ids is required")
		return
	}

	// Validate project exists
	project, err := s.store.GetProject(ctx, req.ProjectID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Validate all issues exist and belong to the project
	var issues []*models.Issue
	for _, id := range req.IssueIDs {
		issue, err := s.store.GetIssue(ctx, id)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeError(w, http.StatusNotFound, fmt.Sprintf("issue %s not found", id))
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if issue.ProjectID != req.ProjectID {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("issue %s does not belong to project %s", id, req.ProjectID))
			return
		}
		issues = append(issues, issue)
	}

	// Generate branch name from first issue title
	branch := issueToBranch(issues[0].Title)

	// Worktree path: <project.Path>-<branch_with_slashes_replaced_by_hyphens>
	worktreePath := project.Path + "-" + strings.ReplaceAll(branch, "/", "-")

	// Check for existing idle session on this branch
	existingSessions, _ := s.store.ListAgentSessions(ctx, project.ID, 0)
	for _, sess := range existingSessions {
		if sess.Branch == branch && sess.Status == models.SessionStatusIdle {
			sess.Status = models.SessionStatusActive
			now := time.Now().UTC()
			sess.LastActiveAt = &now
			if err := s.store.UpdateAgentSession(ctx, sess); err == nil {
				var issueRefs []string
				for _, issue := range issues {
					id := issue.ID
					if len(id) > 12 {
						id = id[:12]
					}
					issueRefs = append(issueRefs, id)
				}
				prompt := fmt.Sprintf("Use pm MCP tools to look up issue(s) %s and implement them. Update issue status when complete.", strings.Join(issueRefs, ", "))
				command := fmt.Sprintf(`cd %s && claude "%s"`, sess.WorktreePath, prompt)
				writeJSON(w, http.StatusOK, LaunchAgentResponse{
					SessionID:    sess.ID,
					Branch:       branch,
					WorktreePath: sess.WorktreePath,
					Command:      command,
				})
				return
			}
		}
	}

	// Auto-purge stale abandoned sessions for this branch
	if _, err := s.store.DeleteStaleSessions(ctx, project.ID, branch); err != nil {
		slog.Warn("failed to purge stale sessions", "error", err)
	}

	// Create worktree
	if err := s.wt.Create(project.Path, branch); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create worktree: %v", err))
		return
	}

	// Record agent session (use first issue ID for the session record)
	session := &models.AgentSession{
		ProjectID:    project.ID,
		IssueID:      req.IssueIDs[0],
		Branch:       branch,
		WorktreePath: worktreePath,
		Status:       models.SessionStatusActive,
	}
	if err := s.store.CreateAgentSession(ctx, session); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("create session: %v", err))
		return
	}

	// Mark all issues as in_progress
	for _, issue := range issues {
		issue.Status = models.IssueStatusInProgress
		_ = s.store.UpdateIssue(ctx, issue)
	}

	// Build command prompt with issue IDs for MCP lookup
	var issueRefs []string
	for _, issue := range issues {
		id := issue.ID
		if len(id) > 12 {
			id = id[:12]
		}
		issueRefs = append(issueRefs, id)
	}
	prompt := fmt.Sprintf("Use pm MCP tools to look up issue(s) %s and implement them. Update issue status when complete.", strings.Join(issueRefs, ", "))
	command := fmt.Sprintf(`cd %s && claude "%s"`, worktreePath, prompt)

	writeJSON(w, http.StatusOK, LaunchAgentResponse{
		SessionID:    session.ID,
		Branch:       branch,
		WorktreePath: worktreePath,
		Command:      command,
	})
}

func (s *Server) resumeAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	sess, err := s.store.GetAgentSession(ctx, req.SessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	if sess.Status != models.SessionStatusIdle {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("session is %s, not idle", sess.Status))
		return
	}

	// Look up project to get repo path for wt open
	project, err := s.store.GetProject(ctx, sess.ProjectID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "project not found for session")
		return
	}

	// Open iTerm window via wt open
	if err := s.wt.Create(project.Path, sess.Branch); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("wt open: %v", err))
		return
	}

	sess.Status = models.SessionStatusActive
	now := time.Now().UTC()
	sess.LastActiveAt = &now
	if err := s.store.UpdateAgentSession(ctx, sess); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	command := fmt.Sprintf("cd %s && claude", sess.WorktreePath)
	if sess.IssueID != "" {
		shortID := sess.IssueID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		command = fmt.Sprintf(`cd %s && claude "Use pm MCP tools to look up issue %s and implement it. Update the issue status when complete."`, sess.WorktreePath, shortID)
	}

	writeJSON(w, http.StatusOK, LaunchAgentResponse{
		SessionID:    sess.ID,
		Branch:       sess.Branch,
		WorktreePath: sess.WorktreePath,
		Command:      command,
	})
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

// --- Agent Close ---

// CloseAgentRequest is the JSON body for POST /api/v1/agent/close.
type CloseAgentRequest struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"` // idle, completed, abandoned
}

// CloseAgentResponse is the JSON response for closing an agent session.
type CloseAgentResponse struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	EndedAt   string `json:"ended_at,omitempty"`
}

func (s *Server) closeAgent(w http.ResponseWriter, r *http.Request) {
	var req CloseAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	target := models.SessionStatusIdle
	if req.Status != "" {
		target = models.SessionStatus(req.Status)
	}

	switch target {
	case models.SessionStatusIdle, models.SessionStatusCompleted, models.SessionStatusAbandoned:
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid status: %s", req.Status))
		return
	}

	// Enrich session with git info before closing
	if sess, err := s.store.GetAgentSession(r.Context(), req.SessionID); err == nil {
		agent.EnrichSessionWithGitInfo(sess, s.git)
		_ = s.store.UpdateAgentSession(r.Context(), sess)
	}

	session, err := agent.CloseSession(r.Context(), s.store, req.SessionID, target)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := CloseAgentResponse{
		SessionID: session.ID,
		Status:    string(session.Status),
	}
	if session.EndedAt != nil {
		resp.EndedAt = session.EndedAt.Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}
