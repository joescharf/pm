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
	"github.com/joescharf/pm/internal/wt"
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
	wtc := wt.NewClient()
	srv := NewServer(s, gc, ghc, wtc)

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

func TestUpdateProject_PartialUpdate_PreservesOmittedFields(t *testing.T) {
	srv, s := setupTestServer(t)
	router := srv.Router()
	ctx := context.Background()

	// Create a project with all fields populated
	p := &models.Project{
		Name:        "full-project",
		Path:        "/home/user/projects/full",
		Description: "Original description",
		RepoURL:     "https://github.com/test/full-project",
		Language:    "Go",
		GroupName:   "backend",
	}
	require.NoError(t, s.CreateProject(ctx, p))

	// Verify the project was created with all fields
	original, err := s.GetProject(ctx, p.ID)
	require.NoError(t, err)
	require.Equal(t, "full-project", original.Name)
	require.Equal(t, "/home/user/projects/full", original.Path)
	require.Equal(t, "Original description", original.Description)
	require.Equal(t, "https://github.com/test/full-project", original.RepoURL)
	require.Equal(t, "Go", original.Language)
	require.Equal(t, "backend", original.GroupName)

	// Send a PUT with only Description changed — omit other fields entirely.
	// With patch semantics, omitted keys should NOT overwrite existing values.
	patchBody := `{"Description":"Updated description"}`
	req := httptest.NewRequest("PUT", "/api/v1/projects/"+p.ID, bytes.NewBufferString(patchBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// Decode the response
	var updated models.Project
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))

	// The Description should be updated
	assert.Equal(t, "Updated description", updated.Description)

	// All other fields MUST be preserved (not wiped to zero values)
	assert.Equal(t, "full-project", updated.Name, "Name should be preserved")
	assert.Equal(t, "/home/user/projects/full", updated.Path, "Path should be preserved")
	assert.Equal(t, "https://github.com/test/full-project", updated.RepoURL, "RepoURL should be preserved")
	assert.Equal(t, "Go", updated.Language, "Language should be preserved")
	assert.Equal(t, "backend", updated.GroupName, "GroupName should be preserved")

	// Double-check by reading from the store directly
	fromDB, err := s.GetProject(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated description", fromDB.Description)
	assert.Equal(t, "full-project", fromDB.Name, "Name in DB should be preserved")
	assert.Equal(t, "/home/user/projects/full", fromDB.Path, "Path in DB should be preserved")
	assert.Equal(t, "https://github.com/test/full-project", fromDB.RepoURL, "RepoURL in DB should be preserved")
	assert.Equal(t, "Go", fromDB.Language, "Language in DB should be preserved")
	assert.Equal(t, "backend", fromDB.GroupName, "GroupName in DB should be preserved")
}

func TestUpdateProject_EmptyStringsShouldNotOverwrite(t *testing.T) {
	srv, s := setupTestServer(t)
	router := srv.Router()
	ctx := context.Background()

	// Create a project with all fields populated
	p := &models.Project{
		Name:        "full-project",
		Path:        "/home/user/projects/full",
		Description: "Original description",
		RepoURL:     "https://github.com/test/full-project",
		Language:    "Go",
		GroupName:   "backend",
	}
	require.NoError(t, s.CreateProject(ctx, p))

	// Simulate what the frontend does: send all fields, but some are empty strings.
	// This is the actual bug — the form sends empty strings for fields the user
	// didn't fill in, and the old handler overwrites existing data with "".
	patchBody := `{"Name":"full-project","Description":"Updated description","Path":"","RepoURL":"","Language":"","GroupName":""}`
	req := httptest.NewRequest("PUT", "/api/v1/projects/"+p.ID, bytes.NewBufferString(patchBody))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var updated models.Project
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))

	// Description should be updated
	assert.Equal(t, "Updated description", updated.Description)

	// Fields sent as empty strings should NOT overwrite existing non-empty values
	assert.Equal(t, "/home/user/projects/full", updated.Path, "Path should be preserved when sent as empty string")
	assert.Equal(t, "https://github.com/test/full-project", updated.RepoURL, "RepoURL should be preserved when sent as empty string")
	assert.Equal(t, "Go", updated.Language, "Language should be preserved when sent as empty string")
	assert.Equal(t, "backend", updated.GroupName, "GroupName should be preserved when sent as empty string")

	// Verify in DB too
	fromDB, err := s.GetProject(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "/home/user/projects/full", fromDB.Path, "Path in DB should be preserved")
	assert.Equal(t, "https://github.com/test/full-project", fromDB.RepoURL, "RepoURL in DB should be preserved")
	assert.Equal(t, "Go", fromDB.Language, "Language in DB should be preserved")
	assert.Equal(t, "backend", fromDB.GroupName, "GroupName in DB should be preserved")
}
