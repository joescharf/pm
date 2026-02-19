# Issue Review Feature Design

## Overview

Add AI-powered issue review that validates completed work against requirements, providing a quality gate between "done" (work complete) and "closed" (verified).

**Issue lifecycle with review**:
```
open → in_progress → done → [AI review] → closed (pass) / in_progress (fail)
```

## Approach: Hybrid MCP

The MCP tool `pm_prepare_review` gathers and structures all review context. The calling agent (Claude Code) performs the actual review — code review + optional UI/UX review — then stores the result via `pm_save_review`. This avoids a separate Claude API integration and leverages the agent already running.

## Data Model

### New Table: `issue_reviews`

```sql
CREATE TABLE issue_reviews (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    session_id TEXT,
    verdict TEXT NOT NULL,          -- 'pass' or 'fail'
    summary TEXT NOT NULL,          -- narrative review
    code_quality TEXT,              -- 'pass'/'fail'/'skip'
    requirements_match TEXT,        -- 'pass'/'fail'/'skip'
    test_coverage TEXT,             -- 'pass'/'fail'/'skip'
    ui_ux TEXT,                     -- 'pass'/'fail'/'skip'/'na'
    failure_reasons TEXT,           -- JSON array of strings
    diff_stats TEXT,                -- e.g. "12 files changed, 340+, 45-"
    reviewed_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);
CREATE INDEX idx_issue_reviews_issue_id ON issue_reviews(issue_id);
```

### New Model: `IssueReview`

```go
type ReviewVerdict string
const (
    ReviewVerdictPass ReviewVerdict = "pass"
    ReviewVerdictFail ReviewVerdict = "fail"
)

type ReviewCategory string // "pass", "fail", "skip", "na"

type IssueReview struct {
    ID                string
    IssueID           string
    SessionID         string
    Verdict           ReviewVerdict
    Summary           string
    CodeQuality       ReviewCategory
    RequirementsMatch ReviewCategory
    TestCoverage      ReviewCategory
    UIUX              ReviewCategory
    FailureReasons    []string
    DiffStats         string
    ReviewedAt        time.Time
    CreatedAt         time.Time
}
```

### Project Model Additions

Three new optional fields on the `projects` table:

```sql
ALTER TABLE projects ADD COLUMN build_cmd TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN serve_cmd TEXT DEFAULT '';
ALTER TABLE projects ADD COLUMN serve_port INTEGER DEFAULT 0;
```

These are discovered by the AI agent during its first review of a project and persisted via `pm_update_project` for future reviews. Also editable via CLI (`pm project update`) and the web UI.

## MCP Tools

### `pm_prepare_review`

Gathers all context needed for the calling agent to perform the review.

**Input**:
- `issue_id` (required) — full ULID or prefix
- `base_ref` (optional) — defaults to "main", or auto-detected from session
- `head_ref` (optional) — defaults to session branch, or "HEAD"
- `app_url` (optional) — URL for rodney UI review if app is already running

**Behavior**:
1. Resolve issue → find linked session → get branch name
2. Determine diff range:
   - Session with branch → `git diff main...<branch>`
   - Explicit refs provided → `git diff <base_ref>...<head_ref>`
   - Fallback → require explicit refs
3. Compute diff stats and list of changed files
4. Check if diff touches `ui/` or `internal/ui/` → set `ui_review_needed`
5. If UI review needed:
   - If project has `build_cmd`/`serve_cmd`/`serve_port` → include in response for agent to use
   - If `app_url` provided → include for agent to use with rodney
   - Otherwise → flag that UI changed but no server info available
6. Fetch previous review history for this issue
7. Return structured context

**Output**:
```json
{
  "issue": {
    "id": "...",
    "title": "...",
    "description": "...",
    "body": "...",
    "type": "feature",
    "priority": "medium"
  },
  "session": {
    "id": "...",
    "branch": "feature/...",
    "worktree_path": "...",
    "commit_count": 5
  },
  "diff": "<full git diff output>",
  "diff_stats": "12 files changed, 340 insertions(+), 45 deletions(-)",
  "files_changed": ["cmd/issue.go", "internal/store/sqlite.go"],
  "ui_review_needed": true,
  "ui_context": {
    "build_cmd": "npm run build",
    "serve_cmd": "npm run dev",
    "serve_port": 3000,
    "app_url": null
  },
  "review_history": [
    { "verdict": "fail", "summary": "...", "reviewed_at": "..." }
  ],
  "project": {
    "name": "...",
    "path": "...",
    "language": "go"
  }
}
```

The calling agent then:
- Reviews the diff against the issue requirements
- Evaluates code quality
- If `ui_review_needed` and server info available: starts the server, uses rodney for screenshots + accessibility tree analysis
- If it discovers the build/serve commands during exploration, persists them via `pm_update_project`

### `pm_save_review`

Stores the review verdict and transitions the issue.

**Input**:
- `issue_id` (required)
- `verdict` (required) — "pass" or "fail"
- `summary` (required) — narrative review text
- `code_quality` (optional) — "pass"/"fail"/"skip"
- `requirements_match` (optional) — "pass"/"fail"/"skip"
- `test_coverage` (optional) — "pass"/"fail"/"skip"
- `ui_ux` (optional) — "pass"/"fail"/"skip"/"na"
- `failure_reasons` (optional) — JSON array of strings

**Behavior**:
1. Create `issue_reviews` row with all fields
2. Populate `diff_stats` from the issue's most recent prepare call (or accept as param)
3. Transition issue:
   - Pass → status `done` → `closed`, set `closed_at`
   - Fail → status → `in_progress`, clear `closed_at`
4. Return the saved review

### `pm_update_project`

New MCP tool to update project metadata.

**Input**:
- `project` (required) — project name
- `build_cmd` (optional)
- `serve_cmd` (optional)
- `serve_port` (optional)
- `description` (optional)

**Output**: Updated project JSON.

## Store Interface Additions

```go
CreateIssueReview(ctx context.Context, review *IssueReview) error
ListIssueReviews(ctx context.Context, issueID string) ([]*IssueReview, error)
```

## CLI Command

```
pm issue review <id> [--base-ref REF] [--head-ref REF] [--app-url URL]
```

For CLI usage, outputs the structured review context to stdout (useful for piping to an agent or manual inspection). When running under an MCP agent, the agent uses the MCP tools directly.

## UI/UX Review via Rodney

When `ui_review_needed` is true, the agent:

1. Determines how to run the app:
   - Project has `serve_cmd`/`serve_port` → use those
   - `app_url` provided → use directly
   - Neither → explore project (Makefile, package.json, README) to discover commands, then persist via `pm_update_project`
2. Builds the project (`build_cmd` if set)
3. Starts the dev server in background
4. Uses rodney:
   - `rodney start` — launch headless Chrome
   - `rodney open http://localhost:<port>`
   - `rodney waitstable`
   - `rodney screenshot` — capture main view
   - `rodney ax-tree --json` — accessibility tree for semantic analysis
   - Navigate to relevant pages based on what changed
   - `rodney stop`
5. Kills the dev server
6. Includes screenshots and accessibility analysis in the review

**Graceful degradation**: If the app can't be started, UI/UX category is set to "skip" with a note. Code review still proceeds.

## Web UI

- Project edit page: add fields for `build_cmd`, `serve_cmd`, `serve_port`
- Issue detail page: show review history with verdict badges, expandable summaries
- Issue list: visual indicator for reviewed/unreviewed done issues

## Review Categories

| Category | What it evaluates |
|----------|-------------------|
| `code_quality` | Clean code, no obvious bugs, follows project patterns, no security issues |
| `requirements_match` | Changes satisfy the issue's description/body requirements |
| `test_coverage` | New/changed code has appropriate tests |
| `ui_ux` | Visual correctness, accessibility, responsive behavior (rodney-based) |

## Status Transitions

| Current Status | Review Verdict | New Status |
|----------------|---------------|------------|
| done | pass | closed |
| done | fail | in_progress |
| in_progress | pass | closed (allows re-review after fix) |
| in_progress | fail | in_progress (stays) |
