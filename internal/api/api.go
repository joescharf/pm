package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/health"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/store"
)

// Server provides the REST API handlers.
type Server struct {
	store    store.Store
	git      git.Client
	gh       git.GitHubClient
	scorer   *health.Scorer
}

// NewServer creates a new API server.
func NewServer(s store.Store, gc git.Client, ghc git.GitHubClient) *Server {
	return &Server{
		store:  s,
		git:    gc,
		gh:     ghc,
		scorer: health.NewScorer(),
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

	mux.HandleFunc("GET /api/v1/projects/{id}/issues", s.listProjectIssues)
	mux.HandleFunc("POST /api/v1/projects/{id}/issues", s.createProjectIssue)

	mux.HandleFunc("GET /api/v1/issues", s.listIssues)
	mux.HandleFunc("GET /api/v1/issues/{id}", s.getIssue)
	mux.HandleFunc("PUT /api/v1/issues/{id}", s.updateIssue)
	mux.HandleFunc("DELETE /api/v1/issues/{id}", s.deleteIssue)

	mux.HandleFunc("GET /api/v1/status", s.statusOverview)
	mux.HandleFunc("GET /api/v1/status/{id}", s.statusProject)

	mux.HandleFunc("GET /api/v1/sessions", s.listSessions)

	mux.HandleFunc("GET /api/v1/tags", s.listTags)

	mux.HandleFunc("GET /api/v1/health/{id}", s.projectHealth)

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
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
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
	var p models.Project
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	p.ID = id
	if err := s.store.UpdateProject(r.Context(), &p); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (s *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteProject(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

// --- Status ---

type statusEntry struct {
	Project  *models.Project `json:"project"`
	Branch   string          `json:"branch"`
	IsDirty  bool            `json:"isDirty"`
	OpenIssues    int        `json:"openIssues"`
	InProgress    int        `json:"inProgressIssues"`
	Health        int        `json:"health"`
	LastActivity  string     `json:"lastActivity"`
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

	if branch, err := s.git.CurrentBranch(p.Path); err == nil {
		entry.Branch = branch
	}
	if dirty, err := s.git.IsDirty(p.Path); err == nil {
		entry.IsDirty = dirty
	}
	if date, err := s.git.LastCommitDate(p.Path); err == nil {
		entry.LastActivity = date.Format("2006-01-02T15:04:05Z")
	}

	issues, _ := s.store.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID})
	for _, i := range issues {
		switch i.Status {
		case models.IssueStatusOpen:
			entry.OpenIssues++
		case models.IssueStatusInProgress:
			entry.InProgress++
		}
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

	h := s.scorer.Score(p, meta, issues)
	entry.Health = h.Total

	return entry
}

// --- Sessions ---

func (s *Server) listSessions(w http.ResponseWriter, r *http.Request) {
	projectID := r.URL.Query().Get("project_id")
	sessions, err := s.store.ListAgentSessions(r.Context(), projectID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sessions)
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

	issues, _ := s.store.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID})
	h := s.scorer.Score(p, meta, issues)
	writeJSON(w, http.StatusOK, h)
}
