# Fix Open Issues (Batches 1-3) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix 6 open issues: 2 bugs (project edit data loss, blank web UI pages) and 4 UI improvements (issue sorting, open issue counts, dashboard links, health card prominence).

**Architecture:** Backend changes in Go (SQLite query ordering, API merge-update logic, CLI table columns). Frontend changes in React/TypeScript (dashboard links, health card reorder). Rebuild embedded UI at the end.

**Tech Stack:** Go (Cobra CLI, net/http API, SQLite), React (react-router, TanStack Query, shadcn/ui), Bun (build tooling)

---

### Task 1: Bug — Fix "Editing project loses information" (API merge update)

**Issue:** `01KH9ZZYFHXT` — When editing a project in the web UI, empty form fields overwrite existing data because the API handler blindly decodes the JSON body onto the existing project.

**Root cause:** `internal/api/api.go:126-143` — `updateProject` does `json.NewDecoder(r.Body).Decode(existing)` which overwrites all fields including those the frontend sends as empty strings. The Go `Project` struct has no `json:"omitempty"` tags, so empty strings replace real values.

**Fix:** Decode into a `map[string]any`, then selectively merge only the keys the client actually sent.

**Files:**
- Modify: `internal/api/api.go:126-143` (updateProject handler)
- Test: `internal/api/api_test.go` (new file)

**Step 1: Write the failing test**

Create `internal/api/api_test.go`:

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/store"
)

// testStore creates a real SQLite store in a temp directory.
func testStore(t *testing.T) store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := store.NewSQLiteStore(dir + "/test.db")
	require.NoError(t, err)
	require.NoError(t, s.Migrate(context.Background()))
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpdateProject_PartialUpdate_PreservesOmittedFields(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()

	// Create a project with all fields populated
	p := &models.Project{
		Name:        "test-proj",
		Path:        "/tmp/test",
		Description: "Original description",
		RepoURL:     "https://github.com/org/repo",
		Language:    "Go",
		GroupName:   "backend",
	}
	require.NoError(t, s.CreateProject(ctx, p))

	// Send a partial update — only change the description
	body, _ := json.Marshal(map[string]string{
		"Description": "Updated description",
	})

	srv := NewServer(s, &nullGitClient{}, &nullGitHubClient{})
	req := httptest.NewRequest("PUT", "/api/v1/projects/"+p.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Router().ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Verify the omitted fields were NOT wiped out
	got, err := s.GetProject(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated description", got.Description)
	assert.Equal(t, "https://github.com/org/repo", got.RepoURL, "RepoURL should be preserved")
	assert.Equal(t, "Go", got.Language, "Language should be preserved")
	assert.Equal(t, "backend", got.GroupName, "GroupName should be preserved")
	assert.Equal(t, "/tmp/test", got.Path, "Path should be preserved")
	assert.Equal(t, "test-proj", got.Name, "Name should be preserved")
}

// Null implementations for git interfaces (not needed for this test).
type nullGitClient struct{}

func (n *nullGitClient) RepoRoot(string) (string, error)                     { return "", nil }
func (n *nullGitClient) CurrentBranch(string) (string, error)                { return "", nil }
func (n *nullGitClient) LastCommitDate(string) (time.Time, error)            { return time.Time{}, nil }
func (n *nullGitClient) LastCommitMessage(string) (string, error)            { return "", nil }
func (n *nullGitClient) LastCommitHash(string) (string, error)               { return "", nil }
func (n *nullGitClient) BranchList(string) ([]string, error)                 { return nil, nil }
func (n *nullGitClient) IsDirty(string) (bool, error)                        { return false, nil }
func (n *nullGitClient) WorktreeList(string) ([]git.WorktreeInfo, error)     { return nil, nil }
func (n *nullGitClient) RemoteURL(string) (string, error)                    { return "", nil }
func (n *nullGitClient) LatestTag(string) (string, error)                    { return "", nil }

type nullGitHubClient struct{}

func (n *nullGitHubClient) LatestRelease(string, string) (*git.Release, error) { return nil, nil }
func (n *nullGitHubClient) OpenPRs(string, string) ([]git.PullRequest, error)  { return nil, nil }
func (n *nullGitHubClient) RepoInfo(string, string) (*git.RepoInfo, error)     { return nil, nil }
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestUpdateProject_PartialUpdate ./internal/api/`
Expected: FAIL — RepoURL, Language, GroupName will be empty strings

**Step 3: Fix the updateProject handler**

In `internal/api/api.go`, replace the `updateProject` function (lines 126-143) with:

```go
func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.store.GetProject(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Decode into a map so we only update fields the client actually sent.
	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if v, ok := patch["Name"]; ok {
		if s, ok := v.(string); ok {
			existing.Name = s
		}
	}
	if v, ok := patch["Path"]; ok {
		if s, ok := v.(string); ok {
			existing.Path = s
		}
	}
	if v, ok := patch["Description"]; ok {
		if s, ok := v.(string); ok {
			existing.Description = s
		}
	}
	if v, ok := patch["RepoURL"]; ok {
		if s, ok := v.(string); ok {
			existing.RepoURL = s
		}
	}
	if v, ok := patch["Language"]; ok {
		if s, ok := v.(string); ok {
			existing.Language = s
		}
	}
	if v, ok := patch["GroupName"]; ok {
		if s, ok := v.(string); ok {
			existing.GroupName = s
		}
	}

	if err := s.store.UpdateProject(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, existing)
}
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestUpdateProject_PartialUpdate ./internal/api/`
Expected: PASS

**Step 5: Run all tests**

Run: `go test -v -race -count=1 ./...`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/api/api.go internal/api/api_test.go
git commit -m "fix: project update API uses patch semantics to preserve omitted fields

Fixes issue 01KH9ZZYFHXT - editing a project no longer wipes fields
that weren't included in the update payload."
```

---

### Task 2: Feature — Sort issues by status then priority

**Issue:** `01KHA26138JJ` — Issues should sort: open at top, then by priority (high > medium > low), then by created date. Closed issues at the bottom.

**Files:**
- Modify: `internal/store/sqlite.go:337` (ORDER BY clause in ListIssues)
- Test: `internal/store/sqlite_test.go` (add test)

**Step 1: Write the failing test**

Add to `internal/store/sqlite_test.go`:

```go
func TestListIssues_SortedByStatusThenPriority(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p := &models.Project{Name: "sort-test", Path: "/tmp/sort"}
	require.NoError(t, s.CreateProject(ctx, p))

	// Create issues in random order
	issues := []struct {
		title    string
		status   models.IssueStatus
		priority models.IssuePriority
	}{
		{"closed-low", models.IssueStatusClosed, models.IssuePriorityLow},
		{"open-low", models.IssueStatusOpen, models.IssuePriorityLow},
		{"open-high", models.IssueStatusOpen, models.IssuePriorityHigh},
		{"in-progress-medium", models.IssueStatusInProgress, models.IssuePriorityMedium},
		{"open-medium", models.IssueStatusOpen, models.IssuePriorityMedium},
		{"done-high", models.IssueStatusDone, models.IssuePriorityHigh},
	}

	for _, iss := range issues {
		i := &models.Issue{
			ProjectID: p.ID,
			Title:     iss.title,
			Status:    iss.status,
			Priority:  iss.priority,
			Type:      models.IssueTypeFeature,
		}
		require.NoError(t, s.CreateIssue(ctx, i))
		// Small sleep so created_at differs
		time.Sleep(5 * time.Millisecond)
	}

	result, err := s.ListIssues(ctx, IssueListFilter{ProjectID: p.ID})
	require.NoError(t, err)
	require.Len(t, result, 6)

	// Expected order: open issues first (high, medium, low), then in_progress, then done, then closed
	titles := make([]string, len(result))
	for i, r := range result {
		titles[i] = r.Title
	}

	assert.Equal(t, "open-high", titles[0], "open+high should be first")
	assert.Equal(t, "open-medium", titles[1], "open+medium second")
	assert.Equal(t, "open-low", titles[2], "open+low third")
	assert.Equal(t, "in-progress-medium", titles[3], "in_progress next")
	assert.Equal(t, "done-high", titles[4], "done next")
	assert.Equal(t, "closed-low", titles[5], "closed last")
}
```

**Step 2: Run test to verify it fails**

Run: `go test -v -run TestListIssues_SortedByStatusThenPriority ./internal/store/`
Expected: FAIL — current order is by `created_at DESC`

**Step 3: Change the ORDER BY clause**

In `internal/store/sqlite.go`, replace line 337:

```go
// OLD:
query += " ORDER BY created_at DESC"

// NEW:
query += ` ORDER BY
	CASE status WHEN 'open' THEN 0 WHEN 'in_progress' THEN 1 WHEN 'done' THEN 2 WHEN 'closed' THEN 3 ELSE 4 END,
	CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 3 END,
	created_at DESC`
```

**Step 4: Run test to verify it passes**

Run: `go test -v -run TestListIssues_SortedByStatusThenPriority ./internal/store/`
Expected: PASS

**Step 5: Run all store tests**

Run: `go test -v -race -count=1 ./internal/store/`
Expected: All PASS

**Step 6: Commit**

```bash
git add internal/store/sqlite.go internal/store/sqlite_test.go
git commit -m "feat: sort issues by status then priority

Open issues appear first (sorted high > medium > low priority),
followed by in_progress, done, and closed.

Fixes issue 01KHA26138JJ."
```

---

### Task 3: Feature — Show open issue counts on project list

**Issue:** `01KHA262C3D0` — Add open issue counts to `pm project list` output.

**Files:**
- Modify: `cmd/project.go:189-216` (projectListRun function)

**Step 1: Update projectListRun to include issue counts**

In `cmd/project.go`, replace the `projectListRun` function (lines 189-216):

```go
func projectListRun() error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	projects, err := s.ListProjects(ctx, projectGroup)
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		ui.Info("No projects tracked. Use 'pm project add <path>' to get started.")
		return nil
	}

	table := ui.Table([]string{"Name", "Path", "Language", "Group", "Open Issues"})
	for _, p := range projects {
		// Count open issues for this project
		issues, _ := s.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID, Status: models.IssueStatusOpen})
		openCount := fmt.Sprintf("%d", len(issues))

		table.Append([]string{
			output.Cyan(p.Name),
			p.Path,
			p.Language,
			p.GroupName,
			openCount,
		})
	}
	table.Render()
	return nil
}
```

Note: This requires adding `"fmt"` and `"github.com/joescharf/pm/internal/models"` and `"github.com/joescharf/pm/internal/store"` to the imports if not already present. Check imports at top of file — `store` and `models` are already imported. `fmt` is already imported.

**Step 2: Build and verify manually**

Run: `go build -o bin/pm . && bin/pm project list`
Expected: Table now has an "Open Issues" column with counts

**Step 3: Commit**

```bash
git add cmd/project.go
git commit -m "feat: show open issue counts in project list

Adds an 'Open Issues' column to pm project list output.

Fixes issue 01KHA262C3D0."
```

---

### Task 4: Feature — Dashboard project name links to project detail, branch links to GitHub

**Issue:** `01KHA06QDMVV` — In the web UI dashboard, clicking project name should navigate to `/projects/{id}`, not to GitHub. Branch should link to the GitHub repo.

**Files:**
- Modify: `ui/src/components/dashboard/status-table.tsx:46-71` (Project name cell) and line 92 (Branch cell)

**Step 1: Update the project name cell**

In `ui/src/components/dashboard/status-table.tsx`, replace the project name `<TableCell>` (lines 47-71) so the project name always links to the project detail page. Remove the GitHub external link from the name:

```tsx
<TableCell>
  <Link
    to={`/projects/${e.project.ID}`}
    className="font-medium hover:underline"
  >
    {e.project.Name}
  </Link>
  {e.project.GroupName && (
    <span className="ml-2 text-xs text-muted-foreground">
      {e.project.GroupName}
    </span>
  )}
</TableCell>
```

**Step 2: Update the branch cell to link to GitHub**

Replace line 92 (the branch `<TableCell>`):

```tsx
<TableCell className="font-mono text-xs">
  {e.branch ? (
    e.project.RepoURL ? (
      <a
        href={`${repoURL(e.project.RepoURL)}/tree/${e.branch}`}
        target="_blank"
        rel="noopener noreferrer"
        className="inline-flex items-center gap-1 hover:underline"
      >
        {e.branch}
        <ExternalLink className="size-3" />
      </a>
    ) : (
      e.branch
    )
  ) : (
    "—"
  )}
</TableCell>
```

**Step 3: Verify `ExternalLink` is still imported and `Link` is imported**

Both are already imported at the top of the file. No changes needed.

**Step 4: Commit**

```bash
git add ui/src/components/dashboard/status-table.tsx
git commit -m "feat: dashboard project name links to detail, branch links to GitHub

Project name navigates to /projects/{id} instead of GitHub.
Branch column now links to the repo branch on GitHub.

Fixes issue 01KHA06QDMVV."
```

---

### Task 5: Feature — Reduce prominence of health score on project detail page

**Issue:** `01KHA25ZY7G4` — Move the health score card below the Issues/Sessions tabs section.

**Files:**
- Modify: `ui/src/components/projects/project-detail.tsx:175-195` (Health Chart card) — move it after the Tabs block (after line 299)

**Step 1: Move the Health Score card**

In `ui/src/components/projects/project-detail.tsx`, remove the Health Score `<Card>` block (lines 175-195) from its current position and place it after the closing `</Tabs>` tag (after line 299, before the Edit Dialog).

The moved block:

```tsx
{/* Health Score (lower prominence) */}
<Card>
  <CardHeader>
    <CardTitle>Health Score</CardTitle>
  </CardHeader>
  <CardContent>
    {healthLoading ? (
      <div className="space-y-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-6 rounded" />
        ))}
      </div>
    ) : healthData ? (
      <HealthChart health={healthData} />
    ) : (
      <p className="text-muted-foreground text-sm">
        No health data available.
      </p>
    )}
  </CardContent>
</Card>
```

**Step 2: Commit**

```bash
git add ui/src/components/projects/project-detail.tsx
git commit -m "feat: move health score below issues on project detail page

Reduces prominence of health score breakdown by placing it after
the more useful Issues and Sessions tabs.

Fixes issue 01KHA25ZY7G4."
```

---

### Task 6: Rebuild embedded UI and verify SPA routing

**Issue:** `01KHA1Y4GEQT` — Web UI shows blank pages on many routes. The SPA fallback handler in `internal/ui/embed.go` looks correct, so likely the embedded dist is stale. Rebuild after UI changes.

**Files:**
- Build: `ui/` (bun run build)
- Copy: `internal/ui/dist/` (make ui-embed)
- Build: Go binary (make build)

**Step 1: Install UI dependencies (if needed)**

Run: `cd /Users/joescharf/app/pm/ui && bun install`

**Step 2: Build the UI**

Run: `cd /Users/joescharf/app/pm && make ui-build`
Expected: Build succeeds, `ui/dist/` is populated

**Step 3: Embed the built UI**

Run: `make ui-embed`
Expected: `internal/ui/dist/` is updated with fresh build

**Step 4: Build the Go binary**

Run: `make build`
Expected: `bin/pm` is built

**Step 5: Test SPA routing manually**

Run: `bin/pm serve` and verify:
- Navigate to `/projects` — should show projects page (not blank)
- Navigate to `/issues` — should show issues page (not blank)
- Refresh browser on `/projects/some-id` — should show project detail (not blank)
- Click on a project name in dashboard — should navigate to project detail

**Step 6: Commit the embedded UI**

```bash
git add internal/ui/dist/
git commit -m "build: rebuild embedded UI with dashboard and project detail fixes

Includes: project name links to detail view, branch links to GitHub,
health score moved below issues. Fixes SPA routing with fresh build.

Fixes issue 01KHA1Y4GEQT."
```

---

### Task 7: Update all issue statuses

After all implementations are verified:

```bash
# Close the 6 fixed issues
pm issue update 01KH9ZZYFHXT --status closed   # Editing project loses info
pm issue close 01KH9ZZYFHXT

pm issue close 01KHA26138JJ   # Sort issues by status/priority

pm issue close 01KHA262C3D0   # Open issue counts on project list

pm issue close 01KHA06QDMVV   # Dashboard links

pm issue close 01KHA25ZY7G4   # Health score prominence

pm issue close 01KHA1Y4GEQT   # Web UI blank pages

# Verify
pm issue list
```

Expected: 6 issues now show as closed, 4 remaining open issues are the larger features deferred to a future plan.

---
