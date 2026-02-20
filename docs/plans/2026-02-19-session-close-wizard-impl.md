# Session Close Wizard & Lifecycle Improvements — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace the simple "Done" button with a guided close wizard that checks repo state, offers sync/merge, and optionally deletes the worktree. Also add session reactivation for completed/abandoned sessions.

**Architecture:** New `CloseCheck` method on sessions manager returns repo readiness info. New `ReactivateSession` function in lifecycle.go handles completed/abandoned → idle transitions. Frontend gets a new `CloseWizardDialog` component that replaces direct close calls. All existing sync/merge/delete operations are reused unchanged.

**Tech Stack:** Go (net/http handlers, testify), React (TypeScript, shadcn Dialog, TanStack Query mutations)

---

### Task 1: Add ReactivateSession to lifecycle.go (backend)

**Files:**
- Modify: `internal/agent/lifecycle.go`
- Test: `internal/agent/lifecycle_test.go`

**Step 1: Write failing tests for ReactivateSession**

Add to `internal/agent/lifecycle_test.go`:

```go
func TestReactivateSession_FromCompleted(t *testing.T) {
	store := newMockStore()
	now := time.Now().UTC()
	store.sessions["sess-r1"] = &models.AgentSession{
		ID:           "sess-r1",
		IssueID:      "issue-r1",
		Status:       models.SessionStatusCompleted,
		EndedAt:      &now,
		WorktreePath: "/tmp/test-wt", // pretend it exists
	}
	store.issues["issue-r1"] = &models.Issue{
		ID:     "issue-r1",
		Status: models.IssueStatusDone,
	}

	ctx := context.Background()
	session, err := ReactivateSession(ctx, store, "sess-r1")
	require.NoError(t, err)

	assert.Equal(t, models.SessionStatusIdle, session.Status)
	assert.Nil(t, session.EndedAt, "reactivated sessions should clear EndedAt")

	issue := store.issues["issue-r1"]
	assert.Equal(t, models.IssueStatusInProgress, issue.Status)
}

func TestReactivateSession_FromAbandoned(t *testing.T) {
	store := newMockStore()
	now := time.Now().UTC()
	store.sessions["sess-r2"] = &models.AgentSession{
		ID:           "sess-r2",
		Status:       models.SessionStatusAbandoned,
		EndedAt:      &now,
		WorktreePath: "/tmp/test-wt",
	}

	ctx := context.Background()
	session, err := ReactivateSession(ctx, store, "sess-r2")
	require.NoError(t, err)

	assert.Equal(t, models.SessionStatusIdle, session.Status)
	assert.Nil(t, session.EndedAt)
}

func TestReactivateSession_AlreadyActive(t *testing.T) {
	store := newMockStore()
	store.sessions["sess-r3"] = &models.AgentSession{
		ID:           "sess-r3",
		Status:       models.SessionStatusActive,
		WorktreePath: "/tmp/test-wt",
	}

	ctx := context.Background()
	_, err := ReactivateSession(ctx, store, "sess-r3")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already active")
}
```

Note: add `"time"` to the imports if not already present.

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/agent/ -run TestReactivateSession -v`
Expected: FAIL — `ReactivateSession` undefined

**Step 3: Implement ReactivateSession**

Add to `internal/agent/lifecycle.go`:

```go
// ReactivateSession transitions a completed or abandoned session back to idle.
// Only works if the session is in a terminal state (completed or abandoned).
func ReactivateSession(ctx context.Context, s SessionStore, sessionID string) (*models.AgentSession, error) {
	session, err := s.GetAgentSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if session.Status != models.SessionStatusCompleted && session.Status != models.SessionStatusAbandoned {
		return nil, fmt.Errorf("session %s is already %s", sessionID, session.Status)
	}

	session.Status = models.SessionStatusIdle
	session.EndedAt = nil

	if err := s.UpdateAgentSession(ctx, session); err != nil {
		return nil, fmt.Errorf("update session: %w", err)
	}

	// Cascade issue back to in_progress
	if session.IssueID != "" {
		issue, err := s.GetIssue(ctx, session.IssueID)
		if err == nil {
			issue.Status = models.IssueStatusInProgress
			_ = s.UpdateIssue(ctx, issue)
		}
	}

	return session, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/agent/ -run TestReactivateSession -v`
Expected: PASS (all 3 tests)

**Step 5: Run all lifecycle tests**

Run: `go test ./internal/agent/ -v -count=1`
Expected: All tests pass

**Step 6: Commit**

```bash
git add internal/agent/lifecycle.go internal/agent/lifecycle_test.go
git commit -m "feat: add ReactivateSession for completed/abandoned sessions"
```

---

### Task 2: Add close-check endpoint (backend)

**Files:**
- Modify: `internal/api/api.go` (add route + handler)
- Test: `internal/api/api_test.go`

**Step 1: Add the route registration**

In `api.go`, in the `Router()` function, add after line 81 (`DELETE /api/v1/sessions/{id}/worktree`):

```go
mux.HandleFunc("GET /api/v1/sessions/{id}/close-check", s.closeCheck)
```

**Step 2: Add the close-check handler**

Add after the `deleteWorktree` handler (around line 770, before `discoverWorktrees`):

```go
// --- Close Check ---

type closeCheckWarning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type closeCheckResponse struct {
	SessionID    string             `json:"session_id"`
	WorktreeExists bool            `json:"worktree_exists"`
	IsDirty      bool              `json:"is_dirty"`
	AheadCount   int               `json:"ahead_count"`
	BehindCount  int               `json:"behind_count"`
	ConflictState string           `json:"conflict_state"`
	Branch       string             `json:"branch"`
	BaseBranch   string             `json:"base_branch"`
	ReadyToClose bool              `json:"ready_to_close"`
	Warnings     []closeCheckWarning `json:"warnings"`
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
```

**Step 3: Run build**

Run: `go build ./...`
Expected: Success

**Step 4: Run API tests**

Run: `go test ./internal/api/ -v -count=1`
Expected: All existing tests pass

**Step 5: Commit**

```bash
git add internal/api/api.go
git commit -m "feat: add GET /sessions/{id}/close-check endpoint"
```

---

### Task 3: Add reactivate endpoint (backend)

**Files:**
- Modify: `internal/api/api.go` (add route + handler)

**Step 1: Add the route registration**

In the `Router()` function, add after the close-check route:

```go
mux.HandleFunc("POST /api/v1/sessions/{id}/reactivate", s.reactivateSession)
```

**Step 2: Add the reactivate handler**

Add after the `closeCheck` handler:

```go
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
```

**Step 3: Run build**

Run: `go build ./...`
Expected: Success

**Step 4: Make sure `agent` package is imported**

The `internal/api/api.go` file already imports `"github.com/joescharf/pm/internal/agent"` (used in `closeAgent` handler). Verify this import exists.

**Step 5: Run all tests**

Run: `go test ./... -count=1 2>&1 | tail -20`
Expected: All pass

**Step 6: Commit**

```bash
git add internal/api/api.go
git commit -m "feat: add POST /sessions/{id}/reactivate endpoint"
```

---

### Task 4: Add frontend hooks (close-check + reactivate)

**Files:**
- Modify: `ui/src/hooks/use-sessions.ts`
- Modify: `ui/src/lib/types.ts`

**Step 1: Add TypeScript types**

Add to `ui/src/lib/types.ts` before the `Tag` interface:

```typescript
export interface CloseCheckWarning {
  type: string;
  message: string;
}

export interface CloseCheckResponse {
  session_id: string;
  worktree_exists: boolean;
  is_dirty: boolean;
  ahead_count: number;
  behind_count: number;
  conflict_state: string;
  branch: string;
  base_branch: string;
  ready_to_close: boolean;
  warnings: CloseCheckWarning[];
}

export interface ReactivateResponse {
  session_id: string;
  status: string;
}
```

**Step 2: Add useCloseCheck hook**

Add to `ui/src/hooks/use-sessions.ts`:

```typescript
import type {
  AgentSession,
  SessionDetail,
  SessionStatus,
  SyncSessionRequest,
  SyncSessionResponse,
  MergeSessionRequest,
  MergeSessionResponse,
  DiscoverWorktreesResponse,
  CloseCheckResponse,
  ReactivateResponse,
} from "@/lib/types";

export function useCloseCheck(sessionId: string, enabled: boolean) {
  return useQuery({
    queryKey: ["close-check", sessionId],
    queryFn: () => apiFetch<CloseCheckResponse>(`/api/v1/sessions/${sessionId}/close-check`),
    enabled,
    refetchInterval: false,
  });
}
```

**Step 3: Add useReactivateSession hook**

Add to `ui/src/hooks/use-sessions.ts`:

```typescript
export function useReactivateSession() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (sessionId: string) =>
      apiFetch<ReactivateResponse>(`/api/v1/sessions/${sessionId}/reactivate`, {
        method: "POST",
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["sessions"] });
      qc.invalidateQueries({ queryKey: ["session"] });
      qc.invalidateQueries({ queryKey: ["issues"] });
    },
  });
}
```

**Step 4: Verify UI builds**

Run: `cd ui && bun run build`
Expected: Success (types are exported but not yet consumed by components)

**Step 5: Commit**

```bash
git add ui/src/lib/types.ts ui/src/hooks/use-sessions.ts
git commit -m "feat: add close-check and reactivate hooks"
```

---

### Task 5: Build CloseWizardDialog component

**Files:**
- Create: `ui/src/components/sessions/close-wizard-dialog.tsx`

**Step 1: Create the close wizard dialog**

Create `ui/src/components/sessions/close-wizard-dialog.tsx`:

```tsx
import { useState, useCallback } from "react";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import {
  useCloseCheck,
  useSyncSession,
  useMergeSession,
  useDeleteWorktree,
} from "@/hooks/use-sessions";
import { useCloseAgent } from "@/hooks/use-agent";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { cn } from "@/lib/utils";
import type { AgentSession } from "@/lib/types";

type Step = "check" | "cleanup";

interface CloseWizardDialogProps {
  session: AgentSession;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

export function CloseWizardDialog({ session, open, onOpenChange }: CloseWizardDialogProps) {
  const [step, setStep] = useState<Step>("check");
  const [syncStrategy, setSyncStrategy] = useState<"merge" | "rebase">("merge");
  const [mergeMethod, setMergeMethod] = useState<"pr" | "local">("pr");
  const [showSyncOptions, setShowSyncOptions] = useState(false);
  const [showMergeOptions, setShowMergeOptions] = useState(false);

  const qc = useQueryClient();
  const { data: check, isLoading, refetch } = useCloseCheck(session.ID, open);
  const closeAgent = useCloseAgent();
  const sync = useSyncSession();
  const merge = useMergeSession();
  const del = useDeleteWorktree();

  const handleClose = useCallback(
    (isOpen: boolean) => {
      if (!isOpen) {
        setStep("check");
        setShowSyncOptions(false);
        setShowMergeOptions(false);
      }
      onOpenChange(isOpen);
    },
    [onOpenChange],
  );

  const handleSync = () => {
    sync.mutate(
      { sessionId: session.ID, rebase: syncStrategy === "rebase" },
      {
        onSuccess: (data) => {
          setShowSyncOptions(false);
          if (data.Conflicts?.length) {
            toast.warning(`Sync completed with ${data.Conflicts.length} conflict(s)`);
          } else {
            toast.success("Synced with base branch");
          }
          refetch();
        },
        onError: (err) => toast.error(`Sync failed: ${(err as Error).message}`),
      },
    );
  };

  const handleMerge = () => {
    merge.mutate(
      { sessionId: session.ID, create_pr: mergeMethod === "pr" },
      {
        onSuccess: (data) => {
          setShowMergeOptions(false);
          if (data.Conflicts?.length) {
            toast.warning(`Merge has ${data.Conflicts.length} conflict(s)`);
          } else if (data.PRCreated && data.PRURL) {
            toast.success("PR created", {
              description: data.PRURL,
              action: { label: "Open", onClick: () => window.open(data.PRURL, "_blank") },
            });
          } else if (data.Success) {
            toast.success("Merged successfully");
          }
          refetch();
        },
        onError: (err) => toast.error(`Merge failed: ${(err as Error).message}`),
      },
    );
  };

  const handleComplete = () => {
    closeAgent.mutate(
      { session_id: session.ID, status: "completed" },
      {
        onSuccess: () => {
          toast.success("Session completed");
          if (check?.worktree_exists) {
            setStep("cleanup");
          } else {
            handleClose(false);
          }
        },
        onError: (err) => toast.error(`Failed: ${(err as Error).message}`),
      },
    );
  };

  const handleDeleteWorktree = () => {
    del.mutate(
      { sessionId: session.ID, force: true },
      {
        onSuccess: () => {
          toast.success("Worktree deleted");
          handleClose(false);
        },
        onError: (err) => toast.error(`Delete failed: ${(err as Error).message}`),
      },
    );
  };

  const isActionPending = sync.isPending || merge.isPending || closeAgent.isPending || del.isPending;

  return (
    <Dialog open={open} onOpenChange={handleClose}>
      <DialogContent className="sm:max-w-lg">
        {step === "check" && (
          <>
            <DialogHeader>
              <DialogTitle>Complete Session</DialogTitle>
              <DialogDescription>
                Review the state of{" "}
                <code className="text-xs font-mono">{session.Branch}</code>{" "}
                before closing.
              </DialogDescription>
            </DialogHeader>

            {isLoading ? (
              <div className="space-y-3">
                <Skeleton className="h-6 w-full" />
                <Skeleton className="h-6 w-3/4" />
                <Skeleton className="h-6 w-1/2" />
              </div>
            ) : check ? (
              <div className="space-y-4">
                {/* Status indicators */}
                <div className="grid grid-cols-2 gap-3 text-sm">
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">Working tree</span>
                    <Badge
                      variant="outline"
                      className={cn(
                        "text-xs",
                        check.is_dirty
                          ? "bg-yellow-100 text-yellow-800 dark:bg-yellow-900/40 dark:text-yellow-300"
                          : "bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300",
                      )}
                    >
                      {check.is_dirty ? "dirty" : "clean"}
                    </Badge>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground">Ahead / Behind</span>
                    <span className="text-xs font-mono">
                      <span className={check.ahead_count > 0 ? "text-amber-600 dark:text-amber-400" : "text-green-600 dark:text-green-400"}>
                        +{check.ahead_count}
                      </span>
                      {" / "}
                      <span className={check.behind_count > 0 ? "text-red-600 dark:text-red-400" : "text-green-600 dark:text-green-400"}>
                        -{check.behind_count}
                      </span>
                    </span>
                  </div>
                  {check.conflict_state !== "none" && (
                    <div className="col-span-2 flex items-center justify-between">
                      <span className="text-muted-foreground">Conflicts</span>
                      <Badge
                        variant="outline"
                        className="text-xs bg-red-100 text-red-800 dark:bg-red-900/40 dark:text-red-300"
                      >
                        {check.conflict_state}
                      </Badge>
                    </div>
                  )}
                </div>

                {/* Warnings */}
                {check.warnings.length > 0 && (
                  <div className="rounded-md border border-amber-200 bg-amber-50 dark:border-amber-900 dark:bg-amber-950/30 p-3 space-y-1.5">
                    {check.warnings.map((w, i) => (
                      <p key={i} className="text-xs text-amber-800 dark:text-amber-300">
                        {w.message}
                      </p>
                    ))}
                  </div>
                )}

                {/* Actions */}
                <div className="space-y-3">
                  {check.behind_count > 0 && (
                    <div className="space-y-2">
                      <div className="flex items-center gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setShowSyncOptions(!showSyncOptions)}
                          disabled={isActionPending}
                        >
                          Sync with base
                        </Button>
                        {showSyncOptions && (
                          <>
                            <Select value={syncStrategy} onValueChange={(v) => setSyncStrategy(v as "merge" | "rebase")}>
                              <SelectTrigger className="w-[120px] h-8">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="merge">Merge</SelectItem>
                                <SelectItem value="rebase">Rebase</SelectItem>
                              </SelectContent>
                            </Select>
                            <Button size="sm" onClick={handleSync} disabled={isActionPending}>
                              {sync.isPending ? "Syncing..." : "Go"}
                            </Button>
                          </>
                        )}
                      </div>
                    </div>
                  )}
                  {check.ahead_count > 0 && (
                    <div className="space-y-2">
                      <div className="flex items-center gap-2">
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => setShowMergeOptions(!showMergeOptions)}
                          disabled={isActionPending}
                        >
                          Merge to base
                        </Button>
                        {showMergeOptions && (
                          <>
                            <Select value={mergeMethod} onValueChange={(v) => setMergeMethod(v as "pr" | "local")}>
                              <SelectTrigger className="w-[160px] h-8">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                <SelectItem value="pr">Pull Request</SelectItem>
                                <SelectItem value="local">Local merge</SelectItem>
                              </SelectContent>
                            </Select>
                            <Button size="sm" onClick={handleMerge} disabled={isActionPending}>
                              {merge.isPending ? "Merging..." : "Go"}
                            </Button>
                          </>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            ) : null}

            <DialogFooter>
              <Button variant="outline" onClick={() => handleClose(false)}>
                Cancel
              </Button>
              <Button
                variant={check?.ready_to_close ? "default" : "outline"}
                onClick={handleComplete}
                disabled={isLoading || isActionPending}
              >
                {closeAgent.isPending
                  ? "Completing..."
                  : check?.ready_to_close
                    ? "Complete"
                    : "Complete Anyway"}
              </Button>
            </DialogFooter>
          </>
        )}

        {step === "cleanup" && (
          <>
            <DialogHeader>
              <DialogTitle>Delete Worktree?</DialogTitle>
              <DialogDescription>
                Session is now completed. Would you like to remove the worktree?
              </DialogDescription>
            </DialogHeader>

            {session.WorktreePath && (
              <p className="text-xs font-mono text-muted-foreground break-all">
                {session.WorktreePath}
              </p>
            )}

            <DialogFooter>
              <Button variant="outline" onClick={() => handleClose(false)}>
                Keep
              </Button>
              <Button
                variant="destructive"
                onClick={handleDeleteWorktree}
                disabled={del.isPending}
              >
                {del.isPending ? "Deleting..." : "Delete Worktree"}
              </Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
```

**Step 2: Verify UI builds**

Run: `cd ui && bun run build`
Expected: Success

**Step 3: Commit**

```bash
git add ui/src/components/sessions/close-wizard-dialog.tsx
git commit -m "feat: add CloseWizardDialog component"
```

---

### Task 6: Wire up the close wizard and reactivation in UI

**Files:**
- Modify: `ui/src/components/sessions/sessions-page.tsx`
- Modify: `ui/src/components/sessions/session-detail.tsx`

**Step 1: Replace Done button in sessions-page.tsx**

Add import at top of `sessions-page.tsx`:

```typescript
import { CloseWizardDialog } from "./close-wizard-dialog";
```

Add state for the wizard dialog inside `SessionsPage` component (after the hook declarations):

```typescript
const [closeWizardSession, setCloseWizardSession] = useState<AgentSession | null>(null);
```

Also import `AgentSession` type (add to existing type import):

```typescript
import type { SessionStatus, AgentSession } from "@/lib/types";
```

Replace the "Done" button in the table actions (the `<Button>` with `onClick={() => handleClose(s.ID, "completed")}`) with:

```tsx
<Button
  variant="outline"
  size="sm"
  className="h-7 text-xs"
  onClick={() => setCloseWizardSession(s)}
>
  Done
</Button>
```

Add the dialog render at the end of the component return, just before the closing `</div>`:

```tsx
{closeWizardSession && (
  <CloseWizardDialog
    session={closeWizardSession}
    open={!!closeWizardSession}
    onOpenChange={(open) => { if (!open) setCloseWizardSession(null); }}
  />
)}
```

**Step 2: Replace Done button and add Reactivate in session-detail.tsx**

Add imports at top of `session-detail.tsx`:

```typescript
import { useReactivateSession } from "@/hooks/use-sessions";
import { CloseWizardDialog } from "./close-wizard-dialog";
```

Inside the `SessionDetail` component, add state and hook:

```typescript
const [closeWizardOpen, setCloseWizardOpen] = useState(false);
const reactivate = useReactivateSession();
```

Add `useState` to the React import if not already there.

Add a `handleReactivate` function:

```typescript
const handleReactivate = () => {
  reactivate.mutate(session.ID, {
    onSuccess: () => toast.success("Session reactivated"),
    onError: (err) => toast.error(`Failed: ${(err as Error).message}`),
  });
};
```

In the header buttons section, replace the existing Done button:

```tsx
<Button
  variant="outline"
  onClick={() => handleClose("completed")}
  disabled={closeAgent.isPending}
>
  Done
</Button>
```

with:

```tsx
<Button
  variant="outline"
  onClick={() => setCloseWizardOpen(true)}
>
  Done
</Button>
```

After the `{isLive && (...)}` block in the header, add the reactivate button for terminal sessions:

```tsx
{!isLive && session.WorktreeExists && (
  <div className="flex items-center gap-2">
    <Button
      onClick={handleReactivate}
      disabled={reactivate.isPending}
    >
      {reactivate.isPending ? "Reactivating..." : "Reactivate"}
    </Button>
  </div>
)}
```

Add the dialog render at the bottom of the component return, just before the closing `</div>` of the root element:

```tsx
{session && (
  <CloseWizardDialog
    session={session}
    open={closeWizardOpen}
    onOpenChange={setCloseWizardOpen}
  />
)}
```

**Step 3: Verify UI builds**

Run: `cd ui && bun run build`
Expected: Success

**Step 4: Commit**

```bash
git add ui/src/components/sessions/sessions-page.tsx ui/src/components/sessions/session-detail.tsx
git commit -m "feat: wire close wizard and reactivate button into sessions UI"
```

---

### Task 7: Build UI, embed, and test end-to-end

**Files:**
- Modify: `internal/ui/dist/` (rebuilt assets)

**Step 1: Build UI for production**

Run: `make ui-build`
Expected: Success

**Step 2: Embed UI into Go binary**

Run: `make ui-embed`
Expected: Files copied to `internal/ui/dist/`

**Step 3: Build Go binary**

Run: `go build ./...`
Expected: Success

**Step 4: Run all tests**

Run: `make test`
Expected: All pass

**Step 5: Restart server and test manually**

Restart the running `pm serve` to pick up changes. Verify:
1. Sessions page: "Done" button opens the wizard dialog
2. Wizard shows repo state (dirty/clean, ahead/behind)
3. Sync/Merge buttons work within the wizard
4. "Complete" marks session completed and offers worktree cleanup
5. Completed session detail page shows "Reactivate" button (if worktree exists)
6. Reactivate transitions session back to idle

**Step 6: Commit embedded assets**

```bash
git add internal/ui/dist/
git commit -m "chore: rebuild embedded UI with close wizard and reactivate"
```
