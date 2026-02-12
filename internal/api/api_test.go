package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/store"
)

func setupTestServer(t *testing.T) (*Server, store.Store) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	s, err := store.NewSQLiteStore(dbPath)
	require.NoError(t, err)
	require.NoError(t, s.Migrate(context.Background()))
	t.Cleanup(func() { s.Close() })

	gc := git.NewClient()
	ghc := git.NewGitHubClient()
	srv := NewServer(s, gc, ghc)

	return srv, s
}

func TestListProjects_Empty(t *testing.T) {
	srv, _ := setupTestServer(t)
	router := srv.Router()

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var projects []*models.Project
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &projects))
	assert.Nil(t, projects)
}

func TestProjectCRUD_API(t *testing.T) {
	srv, _ := setupTestServer(t)
	router := srv.Router()

	// Create
	body := `{"name":"test-proj","path":"/tmp/test","language":"go"}`
	req := httptest.NewRequest("POST", "/api/v1/projects", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var created models.Project
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "test-proj", created.Name)
	assert.NotEmpty(t, created.ID)

	// Get
	req = httptest.NewRequest("GET", "/api/v1/projects/"+created.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// List
	req = httptest.NewRequest("GET", "/api/v1/projects", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var projects []*models.Project
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &projects))
	assert.Len(t, projects, 1)

	// Delete
	req = httptest.NewRequest("DELETE", "/api/v1/projects/"+created.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestIssuesCRUD_API(t *testing.T) {
	srv, s := setupTestServer(t)
	router := srv.Router()
	ctx := context.Background()

	// Create project first
	p := &models.Project{Name: "proj", Path: "/tmp/proj"}
	require.NoError(t, s.CreateProject(ctx, p))

	// Create issue via API
	body := `{"title":"test issue","priority":"high","type":"bug"}`
	req := httptest.NewRequest("POST", "/api/v1/projects/"+p.ID+"/issues", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)

	var created models.Issue
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, "test issue", created.Title)
	assert.Equal(t, models.IssuePriorityHigh, created.Priority)
	assert.Equal(t, models.IssueStatusOpen, created.Status)

	// List project issues
	req = httptest.NewRequest("GET", "/api/v1/projects/"+p.ID+"/issues", nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Get issue
	req = httptest.NewRequest("GET", "/api/v1/issues/"+created.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Delete issue
	req = httptest.NewRequest("DELETE", "/api/v1/issues/"+created.ID, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestTags_API(t *testing.T) {
	srv, _ := setupTestServer(t)
	router := srv.Router()

	req := httptest.NewRequest("GET", "/api/v1/tags", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestSessions_API(t *testing.T) {
	srv, _ := setupTestServer(t)
	router := srv.Router()

	req := httptest.NewRequest("GET", "/api/v1/sessions", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCORS(t *testing.T) {
	srv, _ := setupTestServer(t)
	router := srv.Router()

	req := httptest.NewRequest("OPTIONS", "/api/v1/projects", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
}

func TestStatusOverview_HasVersionFields(t *testing.T) {
	srv, s := setupTestServer(t)
	router := srv.Router()
	ctx := context.Background()

	p := &models.Project{Name: "status-test", Path: "/tmp/status-test"}
	require.NoError(t, s.CreateProject(ctx, p))

	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify JSON structure includes version fields
	var entries []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entries))
	require.Len(t, entries, 1)

	// The entry should have standard fields
	_, hasProject := entries[0]["project"]
	assert.True(t, hasProject, "should have project field")
	_, hasBranch := entries[0]["branch"]
	assert.True(t, hasBranch, "should have branch field")
	_, hasHealth := entries[0]["health"]
	assert.True(t, hasHealth, "should have health field")
}

func TestGetProject_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)
	router := srv.Router()

	req := httptest.NewRequest("GET", "/api/v1/projects/nonexistent", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
