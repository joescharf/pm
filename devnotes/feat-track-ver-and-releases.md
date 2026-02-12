# Version & Release Tracking for PM

*2026-02-12T20:42:15Z*

PM now tracks the latest released version of every project — using GitHub releases as the primary source and local git tags as a fallback. Version info appears everywhere: the CLI status table, project detail view, REST API, health scoring, and the web UI.

## What Changed — 7 Commits

```bash
git log --oneline be248f9..HEAD
```

```output
a1ada73 fix(api): populate release data in projectHealth endpoint for accurate scoring
b489456 feat(ui): add version column to status table
7325ee2 feat(project): show release assets and version source in project show
6a00184 feat(status): add Version column with parallel GitHub release fetching
7fff95e feat(api): add version fields to status endpoint
3388d58 feat(git): add LatestTag method to Client interface
79a3cdf feat(git): add release assets to GitHub Release struct
```

## New Git Client Method: LatestTag

A new `LatestTag(path)` method was added to the `git.Client` interface. It wraps `git describe --tags --abbrev=0` to retrieve the most recent tag reachable from HEAD — used as the fallback when a project has no GitHub releases.

```bash
sed -n '117,119p' internal/git/git.go
```

```output
func (c *RealClient) LatestTag(path string) (string, error) {
	return gitCmd(path, "describe", "--tags", "--abbrev=0")
}
```

Tests cover three cases: no tags (error), single tag, and multiple tags returning the latest.

```bash
go test ./internal/git/ -run TestLatestTag -v
```

```output
=== RUN   TestLatestTag_NoTags
--- PASS: TestLatestTag_NoTags (0.04s)
=== RUN   TestLatestTag_WithTag
--- PASS: TestLatestTag_WithTag (0.04s)
=== RUN   TestLatestTag_MultipleTagsReturnsLatest
--- PASS: TestLatestTag_MultipleTagsReturnsLatest (0.06s)
PASS
ok  	github.com/joescharf/pm/internal/git	(cached)
```

## Enhanced GitHub Release Struct

The `Release` struct now includes a `ReleaseAsset` slice, capturing each artifact's name, download count, and file size. The `gh api` jq query was updated to extract this data.

```bash
sed -n '10,23p' internal/git/github.go
```

```output
// ReleaseAsset represents a file attached to a GitHub release.
type ReleaseAsset struct {
	Name          string `json:"name"`
	DownloadCount int    `json:"downloadCount"`
	Size          int64  `json:"size"`
}

// Release represents a GitHub release.
type Release struct {
	TagName     string         `json:"tagName"`
	PublishedAt string         `json:"publishedAt"`
	IsLatest    bool           `json:"isLatest"`
	Assets      []ReleaseAsset `json:"assets"`
}
```

## CLI: Version Column in `pm status`

The status overview table now includes a **Version** column. Version info for all projects is fetched in parallel using goroutines — GitHub releases are tried first, with local git tags as fallback.

```bash
sed -n '85,85p' cmd/status.go
```

```output
	table := ui.Table([]string{"Project", "Version", "Branch", "Status", "Issues", "Health", "Activity"})
```

The parallel fetching pattern uses a buffered channel indexed by project position:

```bash
sed -n '65,83p' cmd/status.go
```

```output
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
```

## CLI: Enhanced `pm project show`

The detail view now shows version source, release date, and asset details with file sizes and download counts:

```bash
sed -n '286,310p' cmd/project.go
```

```output
	ghClient := git.NewGitHubClient()
	vi := getVersionInfo(gc, ghClient, p)
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

	return nil
}
```

## REST API: Version Fields on Status Endpoint

The `/api/v1/status` endpoint now returns four new fields per project: `latestVersion`, `releaseDate`, `versionSource`, and `releaseAssets`. The `buildStatusEntry()` method was refactored to populate version data before health scoring — eliminating duplicate git calls in the process.

```bash
sed -n '240,253p' internal/api/api.go
```

```output

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

## Health Scoring: Release Freshness Now Accurate

The `/api/v1/health/{id}` endpoint was also updated to populate release data before scoring. Previously, `ReleaseFreshness` always scored 5 (minimum). Now it correctly reflects actual release recency — up to 20 points.

```bash
sed -n '54,62p' internal/health/health.go
```

```output
	// Release freshness (20 pts) - recent release = more points
	if !meta.ReleaseDate.IsZero() {
		h.ReleaseFreshness = scoreRecency(meta.ReleaseDate, 20)
	} else if meta.LatestRelease != "" {
		h.ReleaseFreshness = 10 // has releases but date unknown
	} else {
		h.ReleaseFreshness = 5 // no releases
	}

```

## Web UI: Version Column in Dashboard

The TypeScript `StatusEntry` type was extended with optional version fields, and the dashboard status table now displays a Version column with a tooltip indicating the source.

```bash
sed -n '30,37p' ui/src/components/dashboard/status-table.tsx
```

```output
          <TableHead>Version</TableHead>
          <TableHead>Branch</TableHead>
          <TableHead>Status</TableHead>
          <TableHead className="text-center">Open</TableHead>
          <TableHead className="text-center">In Progress</TableHead>
          <TableHead className="text-center">Health</TableHead>
          <TableHead>Last Activity</TableHead>
        </TableRow>
```

## Full Test Suite: All Passing

```bash
go test ./... -count=1
```

```output
?   	github.com/joescharf/pm	[no test files]
ok  	github.com/joescharf/pm/cmd	0.206s
ok  	github.com/joescharf/pm/internal/api	0.765s
ok  	github.com/joescharf/pm/internal/git	0.969s
ok  	github.com/joescharf/pm/internal/golang	0.566s
ok  	github.com/joescharf/pm/internal/health	0.936s
?   	github.com/joescharf/pm/internal/models	[no test files]
ok  	github.com/joescharf/pm/internal/output	1.057s
ok  	github.com/joescharf/pm/internal/standards	0.313s
ok  	github.com/joescharf/pm/internal/store	0.479s
?   	github.com/joescharf/pm/internal/ui	[no test files]
?   	github.com/joescharf/pm/internal/wt	[no test files]
```

## Build Verification

```bash
go build -o /dev/null ./... && echo 'Build: OK'
```

```output
Build: OK
```

```bash
cd /Users/joescharf/app/pm/ui && bun run build 2>&1 | tail -3
```

```output

✅ Build completed in 213.88ms

```

## Summary

**Files changed:** 9 files across 7 commits

| Layer | File | Change |
|-------|------|--------|
| Git client | `internal/git/git.go` | `LatestTag()` on Client interface |
| Git client | `internal/git/git_test.go` | 3 tests for LatestTag |
| GitHub | `internal/git/github.go` | `ReleaseAsset` struct, assets in Release, updated jq |
| CLI | `cmd/status.go` | Version column, parallel fetch, helpers |
| CLI | `cmd/project.go` | Asset details, `formatBytes()` |
| API | `internal/api/api.go` | Version fields, refactored `buildStatusEntry`, fixed `projectHealth` |
| API | `internal/api/api_test.go` | Status version fields test |
| UI | `ui/src/lib/types.ts` | `ReleaseAsset`, version fields on `StatusEntry` |
| UI | `ui/src/components/dashboard/status-table.tsx` | Version column with source tooltip |
