# Version & Release Tracking Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add latest released version (GitHub release primary, local git tag fallback) to project status everywhere: CLI status table, project show, API endpoints, health scorer, and web UI.

**Architecture:** GitHub releases are the primary version source (via `gh api`), with local git tags as fallback. The `git.Client` interface gains a `LatestTag()` method. The `git.Release` struct is enhanced with asset info. All status display surfaces (CLI, API, UI) show version info and feed it into the health scorer.

**Tech Stack:** Go (Cobra CLI, net/http API), `gh` CLI for GitHub API, `git describe --tags` for local tags, React/TypeScript web UI.

---

### Task 1: Add `LatestTag()` to git.Client

**Files:**
- Modify: `internal/git/git.go:19-29` (Client interface)
- Modify: `internal/git/git.go` (RealClient implementation)
- Modify: `internal/git/git_test.go`

**Step 1: Write the failing test**

Add to `internal/git/git_test.go`:

```go
func TestLatestTag_NoTags(t *testing.T) {
	// Create temp git repo with no tags
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	c := NewClient()
	_, err := c.LatestTag(dir)
	assert.Error(t, err)
}

func TestLatestTag_WithTag(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()
	exec.Command("git", "-C", dir, "tag", "v1.0.0").Run()

	c := NewClient()
	tag, err := c.LatestTag(dir)
	assert.NoError(t, err)
	assert.Equal(t, "v1.0.0", tag)
}

func TestLatestTag_MultipleTagsReturnsLatest(t *testing.T) {
	dir := t.TempDir()
	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "first").Run()
	exec.Command("git", "-C", dir, "tag", "v1.0.0").Run()
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "second").Run()
	exec.Command("git", "-C", dir, "tag", "v2.0.0").Run()

	c := NewClient()
	tag, err := c.LatestTag(dir)
	assert.NoError(t, err)
	assert.Equal(t, "v2.0.0", tag)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/git/ -run TestLatestTag -v`
Expected: FAIL — `LatestTag` method doesn't exist

**Step 3: Add `LatestTag` to interface and implement**

In `internal/git/git.go`, add to `Client` interface:

```go
LatestTag(path string) (string, error)
```

Add implementation to `RealClient`:

```go
func (c *RealClient) LatestTag(path string) (string, error) {
	return gitCmd(path, "describe", "--tags", "--abbrev=0")
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/git/ -run TestLatestTag -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/git/git.go internal/git/git_test.go
git commit -m "feat(git): add LatestTag method to Client interface"
```

---

### Task 2: Enhance GitHub Release with Assets

**Files:**
- Modify: `internal/git/github.go:10-15` (Release struct)
- Modify: `internal/git/github.go:62-76` (LatestRelease method)

**Step 1: Add `ReleaseAsset` struct and update `Release`**

In `internal/git/github.go`, add before `Release`:

```go
// ReleaseAsset represents a file attached to a GitHub release.
type ReleaseAsset struct {
	Name          string `json:"name"`
	DownloadCount int    `json:"downloadCount"`
	Size          int64  `json:"size"`
}
```

Update `Release` struct to add `Assets`:

```go
type Release struct {
	TagName     string         `json:"tagName"`
	PublishedAt string         `json:"publishedAt"`
	IsLatest    bool           `json:"isLatest"`
	Assets      []ReleaseAsset `json:"assets"`
}
```

**Step 2: Update the `LatestRelease` jq query to include assets**

Replace the `LatestRelease` method body's `ghCmd` call:

```go
func (c *RealGitHubClient) LatestRelease(owner, repo string) (*Release, error) {
	out, err := ghCmd("api",
		fmt.Sprintf("repos/%s/%s/releases/latest", owner, repo),
		"--jq", `{tagName: .tag_name, publishedAt: .published_at, isLatest: true, assets: [.assets[] | {name: .name, downloadCount: .download_count, size: .size}]}`,
	)
	if err != nil {
		return nil, err
	}

	var r Release
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	return &r, nil
}
```

**Step 3: Run existing tests**

Run: `go test ./internal/git/ -v`
Expected: PASS (existing tests still pass; LatestRelease tests are integration-only)

**Step 4: Commit**

```bash
git add internal/git/github.go
git commit -m "feat(git): add release assets to GitHub Release struct"
```

---

### Task 3: Add version helper function + populate health metadata

**Files:**
- Modify: `cmd/status.go`

**Step 1: Create `getVersionInfo()` helper and update `gatherMetadata()`**

Add a `versionInfo` struct and helper function to `cmd/status.go`:

```go
type versionInfo struct {
	Version string
	Date    time.Time
	Source  string // "github" or "git-tag"
	Assets  []git.ReleaseAsset
}

func getVersionInfo(gc git.Client, ghClient git.GitHubClient, p *models.Project) *versionInfo {
	// Primary: GitHub release
	if p.RepoURL != "" {
		if owner, repo, err := git.ExtractOwnerRepo(p.RepoURL); err == nil {
			if rel, err := ghClient.LatestRelease(owner, repo); err == nil {
				vi := &versionInfo{
					Version: rel.TagName,
					Source:  "github",
					Assets:  rel.Assets,
				}
				if t, err := time.Parse(time.RFC3339, rel.PublishedAt); err == nil {
					vi.Date = t
				}
				return vi
			}
		}
	}

	// Fallback: local git tag
	if tag, err := gc.LatestTag(p.Path); err == nil {
		return &versionInfo{
			Version: tag,
			Source:  "git-tag",
		}
	}

	return nil
}
```

**Step 2: Update `gatherMetadata()` to accept and populate release info**

Update `gatherMetadata` signature to also accept version info:

```go
func populateReleaseMeta(meta *health.ProjectMetadata, vi *versionInfo) {
	if vi != nil {
		meta.LatestRelease = vi.Version
		meta.ReleaseDate = vi.Date
	}
}
```

**Step 3: Update `statusOverviewRun()` to fetch versions in parallel and add Version column**

```go
func statusOverviewRun() error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	projects, err := s.ListProjects(ctx, statusGroup)
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		ui.Info("No projects tracked. Use 'pm project add <path>' to get started.")
		return nil
	}

	gc := git.NewClient()
	ghClient := git.NewGitHubClient()
	scorer := health.NewScorer()

	// Fetch version info in parallel
	type projectVersion struct {
		index int
		vi    *versionInfo
	}
	versionCh := make(chan projectVersion, len(projects))

	for i, p := range projects {
		go func(idx int, proj *models.Project) {
			vi := getVersionInfo(gc, ghClient, proj)
			versionCh <- projectVersion{index: idx, vi: vi}
		}(i, p)
	}

	versions := make([]*versionInfo, len(projects))
	for range projects {
		pv := <-versionCh
		versions[pv.index] = pv.vi
	}

	table := ui.Table([]string{"Project", "Version", "Branch", "Status", "Issues", "Health", "Activity"})

	for i, p := range projects {
		meta := gatherMetadata(gc, p)
		populateReleaseMeta(meta, versions[i])

		issues, _ := s.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID})

		if statusStale && !meta.LastCommitDate.IsZero() {
			if time.Since(meta.LastCommitDate) < 7*24*time.Hour {
				continue
			}
		}

		h := scorer.Score(p, meta, issues)

		branch := getBranch(gc, p.Path)
		gitStatus := getGitStatus(meta)
		issueStr := formatIssueCounts(issues)
		healthStr := output.HealthColor(h.Total)
		activity := "n/a"
		if !meta.LastCommitDate.IsZero() {
			activity = timeAgo(meta.LastCommitDate)
		}

		versionStr := "-"
		if versions[i] != nil {
			versionStr = versions[i].Version
		}

		table.Append([]string{
			output.Cyan(p.Name),
			versionStr,
			branch,
			gitStatus,
			issueStr,
			healthStr,
			activity,
		})
	}

	table.Render()
	return nil
}
```

**Step 4: Run full test suite**

Run: `go test ./... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/status.go
git commit -m "feat(status): add Version column with parallel GitHub release fetching"
```

---

### Task 4: Enhance `projectShowRun()` with release details

**Files:**
- Modify: `cmd/project.go:204-296`

**Step 1: Update `projectShowRun()` to show assets and use fallback**

Replace the existing GitHub release section (lines ~285-293) with enhanced version:

```go
	// Version / Release info
	gc2 := git.NewGitHubClient()
	vi := getVersionInfo(gc, gc2, p)
	if vi != nil {
		fmt.Fprintln(ui.Out)
		fmt.Fprintf(ui.Out, "  Version:    %s", output.Green(vi.Version))
		if vi.Source == "github" {
			fmt.Fprintf(ui.Out, " (GitHub release)")
		} else {
			fmt.Fprintf(ui.Out, " (git tag)")
		}
		fmt.Fprintln(ui.Out)
		if !vi.Date.IsZero() {
			fmt.Fprintf(ui.Out, "  Released:   %s\n", timeAgo(vi.Date))
		}
		if len(vi.Assets) > 0 {
			fmt.Fprintf(ui.Out, "  Assets:     %d files\n", len(vi.Assets))
			for _, a := range vi.Assets {
				size := formatBytes(a.Size)
				fmt.Fprintf(ui.Out, "              %s (%s, %d downloads)\n", a.Name, size, a.DownloadCount)
			}
		}
	}
```

**Step 2: Add `formatBytes` helper to `cmd/project.go`**

```go
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
```

**Step 3: Run existing tests**

Run: `go test ./cmd/ -v`
Expected: PASS

**Step 4: Commit**

```bash
git add cmd/project.go
git commit -m "feat(project): show release assets and version source in project show"
```

---

### Task 5: Add version fields to API status endpoint

**Files:**
- Modify: `internal/api/api.go:240-316`

**Step 1: Update `statusEntry` struct**

```go
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
```

**Step 2: Update `buildStatusEntry()` to populate version fields**

Add version fetching at the end of `buildStatusEntry()`:

```go
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
```

Note: The `meta.LatestRelease` / `meta.ReleaseDate` must be populated **before** the health scoring call. Move the health scoring call to after the version fetching block.

**Step 3: Add `"time"` import to api.go**

Add `"time"` to the import block in `api.go`.

**Step 4: Write test for version fields in status endpoint**

Add to `internal/api/api_test.go`:

```go
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

	var entries []map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &entries))
	require.Len(t, entries, 1)

	// Version fields should exist in JSON (may be empty for non-git paths)
	_, hasVersion := entries[0]["latestVersion"]
	assert.True(t, hasVersion || entries[0]["latestVersion"] == nil, "should have latestVersion field")
}
```

**Step 5: Run tests**

Run: `go test ./internal/api/ -run TestStatusOverview -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/api/api.go internal/api/api_test.go
git commit -m "feat(api): add version/release fields to status endpoint"
```

---

### Task 6: Update TypeScript types and web UI

**Files:**
- Modify: `ui/src/lib/types.ts`
- Modify: `ui/src/components/dashboard/status-table.tsx`

**Step 1: Update `StatusEntry` type**

Add to `StatusEntry` interface in `ui/src/lib/types.ts`:

```typescript
export interface ReleaseAsset {
  name: string;
  downloadCount: number;
  size: number;
}

export interface StatusEntry {
  project: Project;
  branch: string;
  isDirty: boolean;
  openIssues: number;
  inProgressIssues: number;
  health: number;
  lastActivity: string;
  latestVersion?: string;
  releaseDate?: string;
  versionSource?: string;
  releaseAssets?: ReleaseAsset[];
}
```

**Step 2: Add Version column to `StatusTable`**

In `ui/src/components/dashboard/status-table.tsx`, add a "Version" column header after "Project" and a corresponding cell:

```tsx
<TableHead>Version</TableHead>
```

And in the row:

```tsx
<TableCell className="font-mono text-xs">
  {e.latestVersion ? (
    <span title={e.versionSource === "github" ? "GitHub release" : "git tag"}>
      {e.latestVersion}
    </span>
  ) : (
    "—"
  )}
</TableCell>
```

**Step 3: Verify UI builds**

Run: `cd ui && bun run build` (or equivalent)
Expected: Build succeeds

**Step 4: Commit**

```bash
git add ui/src/lib/types.ts ui/src/components/dashboard/status-table.tsx
git commit -m "feat(ui): add version column to status table"
```

---

### Task 7: Wire up health scorer with release data in API

**Files:**
- Modify: `internal/api/api.go:278-316` (buildStatusEntry — reorder to populate meta before scoring)

**Step 1: Restructure `buildStatusEntry` to populate version before health scoring**

The version fetching block (from Task 5) must execute **before** the `s.scorer.Score()` call. Reorder `buildStatusEntry` so the flow is:
1. Gather git info (branch, dirty, commit date, branches)
2. Gather issues
3. Gather version info (GitHub/tag) — populating `meta.LatestRelease` and `meta.ReleaseDate`
4. Compute health score with fully populated meta

This is a structural reorder of the code already written in Task 5. Ensure the final method body looks like:

```go
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

	// Version info
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
```

**Step 2: Also clean up duplicate git calls**

The current `buildStatusEntry` calls `IsDirty` twice (once for entry, once for meta). The refactored version above eliminates this duplication.

**Step 3: Run all tests**

Run: `go test ./... -v`
Expected: PASS

**Step 4: Commit**

```bash
git add internal/api/api.go
git commit -m "refactor(api): populate release data before health scoring, remove duplicate git calls"
```

---

### Task 8: Final integration test + manual verification

**Step 1: Run full test suite**

Run: `go test ./... -v`
Expected: All PASS

**Step 2: Build and manual test**

```bash
make build
./pm status
./pm project show <some-project-with-github-releases>
```

Expected:
- `pm status` shows Version column with version tags
- `pm project show` shows version, source, release date, and assets
- Health scores may differ (now incorporating release freshness)

**Step 3: Final commit if any fixups needed**

```bash
git add -A
git commit -m "feat: version and release tracking across CLI, API, and UI"
```

---

## Summary of Changes

| File | Change |
|------|--------|
| `internal/git/git.go` | Add `LatestTag()` to interface + impl |
| `internal/git/git_test.go` | Tests for `LatestTag` |
| `internal/git/github.go` | `ReleaseAsset` struct, assets in `Release`, updated jq |
| `cmd/status.go` | Version column, parallel GitHub fetch, `getVersionInfo()` helper |
| `cmd/project.go` | Enhanced release display with assets, `formatBytes()` |
| `internal/api/api.go` | Version fields in `statusEntry`, refactored `buildStatusEntry` |
| `internal/api/api_test.go` | Test for version fields |
| `ui/src/lib/types.ts` | `ReleaseAsset` interface, version fields on `StatusEntry` |
| `ui/src/components/dashboard/status-table.tsx` | Version column |
