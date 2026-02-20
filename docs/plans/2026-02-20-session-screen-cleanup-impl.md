# Session Screen Cleanup Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix the cluttered sessions page by preventing duplicate sessions, auto-purging failed launches, defaulting to active+idle view, replacing stacked action buttons with a dropdown menu, and adding bulk cleanup.

**Architecture:** Five independent changes: (1) new store method `DeleteStaleSessions` + call from `CreateAgentSession`, (2) stronger duplicate check in discover worktrees, (3) default tab change in React, (4) dropdown menu replacement in sessions table, (5) new cleanup API endpoint + UI button. Changes 1-3 are backend (Go), 4-5 are frontend (React/TypeScript) with a small API addition.

**Tech Stack:** Go (SQLite store, Cobra CLI), React + TypeScript (Vite, shadcn/ui, TanStack Query), `lucide-react` icons

---

### Task 1: Add `DeleteStaleSessions` Store Method

**Files:**
- Modify: `internal/store/store.go:46-53` (add interface method)
- Modify: `internal/store/sqlite.go:579-607` (add implementation near CreateAgentSession)
- Test: `internal/store/sqlite_test.go`

**Step 1: Write the failing test**

Add to `internal/store/sqlite_test.go`:

```go
func TestDeleteStaleSessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p := &models.Project{Name: "stale-test", Path: "/tmp/stale-test"}
	require.NoError(t, s.CreateProject(ctx, p))

	// Create a stale abandoned session (0 commits, short duration)
	stale := &models.AgentSession{
		ProjectID:    p.ID,
		Branch:       "feature/test",
		WorktreePath: "/tmp/stale-test-wt",
		Status:       models.SessionStatusAbandoned,
		CommitCount:  0,
	}
	require.NoError(t, s.CreateAgentSession(ctx, stale))
	// Manually set ended_at to 10 seconds after started_at
	tenSec := stale.StartedAt.Add(10 * time.Second)
	stale.EndedAt = &tenSec
	stale.Status = models.SessionStatusAbandoned
	require.NoError(t, s.UpdateAgentSession(ctx, stale))

	// Create a non-stale abandoned session (has commits)
	nonStale := &models.AgentSession{
		ProjectID:    p.ID,
		Branch:       "feature/test",
		WorktreePath: "/tmp/stale-test-wt2",
		Status:       models.SessionStatusAbandoned,
		CommitCount:  3,
	}
	require.NoError(t, s.CreateAgentSession(ctx, nonStale))
	end2 := nonStale.StartedAt.Add(10 * time.Second)
	nonStale.EndedAt = &end2
	require.NoError(t, s.UpdateAgentSession(ctx, nonStale))

	// Delete stale sessions for this branch+project
	count, err := s.DeleteStaleSessions(ctx, p.ID, "feature/test")
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Verify only non-stale remains
	sessions, err := s.ListAgentSessions(ctx, p.ID, 0)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
	assert.Equal(t, nonStale.ID, sessions[0].ID)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestDeleteStaleSessions -v`
Expected: Compilation error — `DeleteStaleSessions` not defined

**Step 3: Add interface method**

In `internal/store/store.go`, add to the `Store` interface inside the `// Agent Sessions` section (after `UpdateAgentSession`):

```go
	DeleteStaleSessions(ctx context.Context, projectID, branch string) (int64, error)
```

**Step 4: Write implementation**

In `internal/store/sqlite.go`, add after `CreateAgentSession`:

```go
// DeleteStaleSessions removes abandoned sessions with 0 commits and duration < 60s
// for the given project and branch. Returns the number of deleted rows.
func (s *SQLiteStore) DeleteStaleSessions(ctx context.Context, projectID, branch string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM agent_sessions
		WHERE project_id = ? AND branch = ?
		AND status = 'abandoned' AND commit_count = 0
		AND ended_at IS NOT NULL
		AND (julianday(ended_at) - julianday(started_at)) * 86400 < 60`,
		projectID, branch,
	)
	if err != nil {
		return 0, fmt.Errorf("delete stale sessions: %w", err)
	}
	return res.RowsAffected()
}
```

**Step 5: Run test to verify it passes**

Run: `go test ./internal/store/ -run TestDeleteStaleSessions -v`
Expected: PASS

**Step 6: Commit**

```bash
git add internal/store/store.go internal/store/sqlite.go internal/store/sqlite_test.go
git commit -m "feat: add DeleteStaleSessions store method"
```

---

### Task 2: Auto-Purge Stale Sessions on Launch

**Files:**
- Modify: `cmd/agent.go:167-269` (agentLaunchRun — call DeleteStaleSessions before creating)
- Modify: `internal/api/api.go:1029-1155` (launchAgent — same)

**Step 1: Write test for API auto-purge**

In `internal/api/api_test.go`, add:

```go
func TestLaunchAgent_PurgesStaleSessionsOnCreate(t *testing.T) {
	srv, s := setupTestServer(t)
	router := srv.Router()
	ctx := context.Background()

	p := &models.Project{Name: "purge-test", Path: t.TempDir()}
	require.NoError(t, s.CreateProject(ctx, p))

	// Create a stale abandoned session
	stale := &models.AgentSession{
		ProjectID:   p.ID,
		Branch:      "feature/purge-test",
		Status:      models.SessionStatusAbandoned,
		CommitCount: 0,
	}
	require.NoError(t, s.CreateAgentSession(ctx, stale))
	tenSec := stale.StartedAt.Add(10 * time.Second)
	stale.EndedAt = &tenSec
	require.NoError(t, s.UpdateAgentSession(ctx, stale))

	// Verify stale session exists
	sessions, err := s.ListAgentSessions(ctx, p.ID, 0)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	// List sessions via API — stale session should still appear initially
	req := httptest.NewRequest("GET", "/api/v1/sessions?project_id="+p.ID, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
```

Note: Full integration test of launch + purge is hard since launch creates worktrees. The store-level test in Task 1 covers the core logic. Here we just verify the API plumbing doesn't break.

**Step 2: Add auto-purge call in API launch**

In `internal/api/api.go`, inside `launchAgent`, add right before the `// Create new session` section (around line 1113):

```go
	// Auto-purge stale abandoned sessions for this branch
	if _, err := s.store.DeleteStaleSessions(ctx, project.ID, branch); err != nil {
		// Non-fatal: log and continue
		slog.Warn("failed to purge stale sessions", "error", err)
	}
```

**Step 3: Add auto-purge call in CLI launch**

In `cmd/agent.go`, inside `agentLaunchRun`, add right before the worktree creation (around line 236, after the idle-session resume loop):

```go
	// Auto-purge stale abandoned sessions for this branch
	if _, err := s.DeleteStaleSessions(ctx, p.ID, branch); err != nil {
		ui.Warning("Failed to purge stale sessions: %v", err)
	}
```

**Step 4: Run tests**

Run: `go test ./internal/api/ -run TestLaunchAgent -v && go test ./internal/store/ -run TestDeleteStaleSessions -v`
Expected: PASS

**Step 5: Commit**

```bash
git add cmd/agent.go internal/api/api.go internal/api/api_test.go
git commit -m "feat: auto-purge stale sessions on agent launch"
```

---

### Task 3: Add Bulk Cleanup API Endpoint

**Files:**
- Modify: `internal/store/store.go` (add `DeleteAllStaleSessions` to interface)
- Modify: `internal/store/sqlite.go` (implement)
- Modify: `internal/api/api.go` (add `DELETE /api/v1/sessions/cleanup` endpoint)
- Test: `internal/store/sqlite_test.go`, `internal/api/api_test.go`

**Step 1: Write store test**

In `internal/store/sqlite_test.go`:

```go
func TestDeleteAllStaleSessions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	p := &models.Project{Name: "bulk-test", Path: "/tmp/bulk-test"}
	require.NoError(t, s.CreateProject(ctx, p))

	// Create 3 stale sessions
	for i := 0; i < 3; i++ {
		sess := &models.AgentSession{
			ProjectID:   p.ID,
			Branch:      "feature/bulk",
			Status:      models.SessionStatusAbandoned,
			CommitCount: 0,
		}
		require.NoError(t, s.CreateAgentSession(ctx, sess))
		tenSec := sess.StartedAt.Add(10 * time.Second)
		sess.EndedAt = &tenSec
		require.NoError(t, s.UpdateAgentSession(ctx, sess))
	}

	// Create 1 non-stale (has commits)
	good := &models.AgentSession{
		ProjectID:   p.ID,
		Branch:      "feature/good",
		Status:      models.SessionStatusAbandoned,
		CommitCount: 5,
	}
	require.NoError(t, s.CreateAgentSession(ctx, good))
	end := good.StartedAt.Add(10 * time.Second)
	good.EndedAt = &end
	require.NoError(t, s.UpdateAgentSession(ctx, good))

	count, err := s.DeleteAllStaleSessions(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	sessions, err := s.ListAgentSessions(ctx, "", 0)
	require.NoError(t, err)
	assert.Len(t, sessions, 1)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/store/ -run TestDeleteAllStaleSessions -v`
Expected: Compilation error

**Step 3: Add interface method and implementation**

In `internal/store/store.go`, add after `DeleteStaleSessions`:

```go
	DeleteAllStaleSessions(ctx context.Context) (int64, error)
```

In `internal/store/sqlite.go`, add after `DeleteStaleSessions`:

```go
// DeleteAllStaleSessions removes all abandoned sessions with 0 commits and duration < 60s.
func (s *SQLiteStore) DeleteAllStaleSessions(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM agent_sessions
		WHERE status = 'abandoned' AND commit_count = 0
		AND ended_at IS NOT NULL
		AND (julianday(ended_at) - julianday(started_at)) * 86400 < 60`,
	)
	if err != nil {
		return 0, fmt.Errorf("delete all stale sessions: %w", err)
	}
	return res.RowsAffected()
}
```

**Step 4: Run store test**

Run: `go test ./internal/store/ -run TestDeleteAllStaleSessions -v`
Expected: PASS

**Step 5: Add API endpoint**

In `internal/api/api.go`, find the routes section. Add a new route for `DELETE /api/v1/sessions/cleanup`. The route must be registered BEFORE the `GET /api/v1/sessions/{id}` pattern to avoid conflicts.

Add the handler:

```go
func (s *Server) cleanupSessions(w http.ResponseWriter, r *http.Request) {
	count, err := s.store.DeleteAllStaleSessions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"deleted": count})
}
```

Register: `mux.HandleFunc("DELETE /api/v1/sessions/cleanup", s.cleanupSessions)`

**Step 6: Write API test**

In `internal/api/api_test.go`:

```go
func TestCleanupSessions_API(t *testing.T) {
	srv, s := setupTestServer(t)
	router := srv.Router()
	ctx := context.Background()

	p := &models.Project{Name: "cleanup-api", Path: "/tmp/cleanup-api"}
	require.NoError(t, s.CreateProject(ctx, p))

	// Create a stale session
	stale := &models.AgentSession{
		ProjectID:   p.ID,
		Branch:      "feature/stale",
		Status:      models.SessionStatusAbandoned,
		CommitCount: 0,
	}
	require.NoError(t, s.CreateAgentSession(ctx, stale))
	tenSec := stale.StartedAt.Add(10 * time.Second)
	stale.EndedAt = &tenSec
	require.NoError(t, s.UpdateAgentSession(ctx, stale))

	req := httptest.NewRequest("DELETE", "/api/v1/sessions/cleanup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]int64
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, int64(1), result["deleted"])
}
```

**Step 7: Run all tests**

Run: `go test ./internal/... -v -count=1`
Expected: All PASS

**Step 8: Commit**

```bash
git add internal/store/store.go internal/store/sqlite.go internal/store/sqlite_test.go internal/api/api.go internal/api/api_test.go
git commit -m "feat: add bulk cleanup API endpoint for stale sessions"
```

---

### Task 4: Fix Discover Worktrees Duplicate Prevention

The current `DiscoverWorktrees` checks `ListAgentSessionsByWorktreePaths` which returns sessions of ALL statuses. If a worktree path has only abandoned/completed sessions, it won't be re-discovered — but new sessions could be created via other paths. The real bug is that `ListAgentSessionsByWorktreePaths` returns abandoned sessions, so discover thinks the path is already tracked even when all its sessions are terminal.

**Files:**
- Modify: `internal/sessions/manager.go:375-384` (filter to active/idle only in knownPaths)

**Step 1: Fix the duplicate check**

In `internal/sessions/manager.go`, change the `knownPaths` logic (around line 381-384):

```go
	knownPaths := make(map[string]bool)
	for _, s := range existingSessions {
		if s.Status == models.SessionStatusActive || s.Status == models.SessionStatusIdle {
			knownPaths[s.WorktreePath] = true
		}
	}
```

This means if a worktree only has abandoned/completed sessions, it can be re-discovered as a new idle session — which is the correct behavior.

**Step 2: Run tests**

Run: `go test ./internal/... -v -count=1`
Expected: All PASS

**Step 3: Commit**

```bash
git add internal/sessions/manager.go
git commit -m "fix: discover worktrees ignores terminal sessions when checking duplicates"
```

---

### Task 5: Default Sessions Tab to Active + Idle

**Files:**
- Modify: `ui/src/components/sessions/sessions-page.tsx:33-39,80`

**Step 1: Add a new default tab**

Change `STATUS_TABS` and default state:

Replace line 33-39:
```typescript
const STATUS_TABS: { label: string; value: string; statuses?: SessionStatus[] }[] = [
  { label: "Active", value: "active_idle", statuses: ["active", "idle"] },
  { label: "Completed", value: "completed", statuses: ["completed"] },
  { label: "Abandoned", value: "abandoned", statuses: ["abandoned"] },
  { label: "All", value: "all" },
];
```

Replace line 80:
```typescript
const [statusTab, setStatusTab] = useState("active_idle");
```

**Step 2: Build and verify**

Run: `cd ui && bun run build`
Expected: Build succeeds

**Step 3: Commit**

```bash
git add ui/src/components/sessions/sessions-page.tsx
git commit -m "feat: default sessions tab to active+idle view"
```

---

### Task 6: Replace Action Buttons with Dropdown Menu

**Files:**
- Modify: `ui/src/components/sessions/sessions-page.tsx:260-309` (replace button block with dropdown)

**Step 1: Add imports**

Add to the imports in `sessions-page.tsx`:

```typescript
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { MoreVertical } from "lucide-react";
```

**Step 2: Replace the action buttons block**

Replace the entire `<TableCell>` for actions (lines 260-309) with:

```tsx
<TableCell>
  {(s.Status === "active" || s.Status === "idle") && (
    <div onClick={(e) => e.stopPropagation()}>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="ghost" size="sm" className="h-7 w-7 p-0">
            <MoreVertical className="size-4" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          <DropdownMenuGroup>
            {s.Status === "idle" && (
              <DropdownMenuItem
                onClick={() => handleResume(s.ID)}
                disabled={resumeAgent.isPending}
              >
                Resume
              </DropdownMenuItem>
            )}
            {s.Status === "active" && (
              <DropdownMenuItem
                onClick={() => handleClose(s.ID, "idle")}
                disabled={closeAgent.isPending}
              >
                Pause
              </DropdownMenuItem>
            )}
            <DropdownMenuItem
              onClick={() => {
                // Open sync dialog — use ref or state
                setSyncSession(s);
              }}
            >
              Sync
            </DropdownMenuItem>
            <DropdownMenuItem
              onClick={() => {
                setMergeSession(s);
              }}
            >
              Merge
            </DropdownMenuItem>
          </DropdownMenuGroup>
          <DropdownMenuSeparator />
          <DropdownMenuGroup>
            <DropdownMenuItem
              onClick={() => setCloseWizardSession(s)}
            >
              Done
            </DropdownMenuItem>
            <DropdownMenuItem
              variant="destructive"
              onClick={() => handleClose(s.ID, "abandoned")}
              disabled={closeAgent.isPending}
            >
              Abandon
            </DropdownMenuItem>
          </DropdownMenuGroup>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  )}
</TableCell>
```

**Step 3: Handle Sync/Merge dialogs from dropdown**

The existing `SyncButton` and `MergeButton` components are self-contained with their own dialog state. To trigger them from the dropdown, add state to the page:

```typescript
const [syncSession, setSyncSession] = useState<AgentSession | null>(null);
const [mergeSession, setMergeSession] = useState<AgentSession | null>(null);
```

Then render the dialog components at the bottom of the page (near the CloseWizardDialog):

```tsx
{syncSession && (
  <SyncButton session={syncSession} open onOpenChange={(open) => { if (!open) setSyncSession(null); }} />
)}
{mergeSession && (
  <MergeButton session={mergeSession} open onOpenChange={(open) => { if (!open) setMergeSession(null); }} />
)}
```

**Important:** The existing `SyncButton` and `MergeButton` in `session-actions.tsx` manage their own dialog open state internally. They need to be refactored to accept optional `open`/`onOpenChange` props for external control. Alternatively, the simpler approach: keep them as inline buttons in the dropdown by rendering them directly as menu items. However, since they have complex dialog UIs, the cleanest approach is to add an `externalOpen` prop pattern.

If the `SyncButton`/`MergeButton` refactor is complex, a simpler approach: render them hidden with `className="hidden"` and use refs to trigger clicks. But the cleanest path is:

**Alternative Step 3 (simpler):** Keep SyncButton/MergeButton as-is but render them inside the dropdown items. Since they're `<Button>` wrappers that open dialogs, we can render them directly:

```tsx
<DropdownMenuItem asChild onSelect={(e) => e.preventDefault()}>
  <SyncButton session={s} />
</DropdownMenuItem>
<DropdownMenuItem asChild onSelect={(e) => e.preventDefault()}>
  <MergeButton session={s} />
</DropdownMenuItem>
```

Test which approach works by checking how `SyncButton` renders (it returns a `<>` fragment with Button + Dialog). If it returns a fragment, the `asChild` approach won't work and we'll need the state-lifting approach. Read `session-actions.tsx` to decide at implementation time.

**Step 4: Build and verify**

Run: `cd ui && bun run build`
Expected: Build succeeds, no type errors

**Step 5: Commit**

```bash
git add ui/src/components/sessions/sessions-page.tsx ui/src/components/sessions/session-actions.tsx
git commit -m "feat: replace session action buttons with dropdown menu"
```

---

### Task 7: Add Bulk Cleanup Button to UI

**Files:**
- Modify: `ui/src/hooks/use-sessions.ts` (add `useCleanupSessions` hook)
- Modify: `ui/src/components/sessions/sessions-page.tsx` (add button + confirmation)

**Step 1: Add the React Query mutation hook**

In `ui/src/hooks/use-sessions.ts`, add:

```typescript
export function useCleanupSessions() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: () =>
      apiFetch<{ deleted: number }>("/api/v1/sessions/cleanup", {
        method: "DELETE",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
    },
  });
}
```

**Step 2: Add the cleanup button to the page header**

In `sessions-page.tsx`, add the hook and state:

```typescript
const cleanup = useCleanupSessions();
const [showCleanupConfirm, setShowCleanupConfirm] = useState(false);
```

Add a button next to "Discover Worktrees" in the header:

```tsx
<div className="flex items-center gap-2">
  <Button
    variant="outline"
    size="sm"
    onClick={() => {
      cleanup.mutate(undefined, {
        onSuccess: (data) => {
          if (data.deleted > 0) {
            toast.success(`Cleaned up ${data.deleted} stale session(s)`);
          } else {
            toast.info("No stale sessions to clean up");
          }
        },
        onError: (err) => toast.error(`Cleanup failed: ${(err as Error).message}`),
      });
    }}
    disabled={cleanup.isPending}
  >
    {cleanup.isPending ? "Cleaning..." : "Clean Up"}
  </Button>
  <Button variant="outline" size="sm" onClick={handleDiscover} disabled={discover.isPending}>
    {discover.isPending ? "Discovering..." : "Discover Worktrees"}
  </Button>
</div>
```

**Step 3: Update import**

Add `useCleanupSessions` to the import from `@/hooks/use-sessions`.

**Step 4: Build and verify**

Run: `cd ui && bun run build`
Expected: Build succeeds

**Step 5: Commit**

```bash
git add ui/src/hooks/use-sessions.ts ui/src/components/sessions/sessions-page.tsx
git commit -m "feat: add bulk cleanup button for stale sessions"
```

---

### Task 8: Build UI, Embed, and Verify

**Files:**
- Build: `ui/dist/` via `make ui-build`
- Copy: `make ui-embed`

**Step 1: Build the UI**

Run: `make ui-build`
Expected: Build succeeds

**Step 2: Embed into Go binary**

Run: `make ui-embed`
Expected: `internal/ui/dist/` updated

**Step 3: Build Go binary**

Run: `make build`
Expected: Binary compiles

**Step 4: Run all Go tests**

Run: `make test`
Expected: All pass

**Step 5: Run lint**

Run: `make lint`
Expected: No errors

**Step 6: Visual verification**

Run: `pm serve restart`
Then: `uvx rodney screenshot http://localhost:8080/sessions --output /tmp/pm-sessions-after.png`
Verify: Default view shows Active+Idle, action column is compact dropdown, no stale sessions visible.

**Step 7: Commit embedded UI**

```bash
git add internal/ui/dist/
git commit -m "build: embed updated UI with session screen improvements"
```

---

### Task 9: Run Cleanup on Live Data

**Step 1: Clean up stale sessions**

Run: `curl -X DELETE http://localhost:8080/api/v1/sessions/cleanup`
Expected: Response shows count of deleted stale sessions

**Step 2: Take final screenshot**

Run: `uvx rodney screenshot http://localhost:8080/sessions --output /tmp/pm-sessions-final.png`
Verify: Clean session list, no duplicates, compact rows

**Step 3: No commit needed — this is live data cleanup**
