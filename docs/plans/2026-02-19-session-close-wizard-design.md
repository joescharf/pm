# Session Close Wizard & Lifecycle Improvements

**Date**: 2026-02-19
**Status**: Approved

## Problem

Clicking "Done" on a session only sets status to `completed` and cascades the linked issue to `done`. It performs no validation — the worktree stays on disk, dirty repos are ignored, unmerged commits are lost, and there's no way to reactivate a completed session.

## Design

### 1. Pre-Close Check API

**Endpoint**: `GET /api/v1/sessions/{id}/close-check`

Returns structured readiness assessment with live git state:

```json
{
  "session_id": "...",
  "worktree_exists": true,
  "is_dirty": true,
  "ahead_count": 3,
  "behind_count": 1,
  "conflict_state": "none",
  "branch": "feature/foo",
  "base_branch": "main",
  "ready_to_close": false,
  "warnings": [
    { "type": "dirty", "message": "Worktree has uncommitted changes" },
    { "type": "unmerged", "message": "3 commits not merged to main" },
    { "type": "behind", "message": "1 commit behind main" }
  ]
}
```

`ready_to_close` = not dirty AND ahead_count == 0 AND no conflicts.

This endpoint is informational — it never blocks operations.

### 2. Close Wizard Dialog (UI)

Multi-step dialog triggered by the "Done" button.

**Step 1 — Pre-Close Summary**:
- Fetches close-check data on open
- Shows repo state with visual indicators (badges/icons):
  - Dirty/Clean status
  - Commits ahead/behind base
  - Conflict state
- Inline action buttons:
  - **Sync** (shown if behind base) — uses existing `useSyncSession`
  - **Merge** (shown if ahead/unmerged) — uses existing `useMergeSession`
- After each action, re-fetches close-check to update display
- **"Complete"** button (primary when `ready_to_close`, outline/warning when not)
- Clicking Complete when not ready shows the warnings but proceeds

**Step 2 — Worktree Cleanup** (shown after marking complete):
- Prompt: "Delete worktree?" with path displayed
- Disabled with explanation if repo was dirty or unmerged
- **Delete** and **Keep** buttons
- Keep closes the dialog; Delete calls existing `useDeleteWorktree`

### 3. Session Reactivation

**New endpoint**: `POST /api/v1/sessions/{id}/reactivate`

Transitions completed/abandoned sessions back to `idle`:
- Validates worktree still exists on disk (400 if not)
- Sets status to `idle`, clears `EndedAt`
- Cascades linked issue back to `in_progress`

**UI**: "Reactivate" button on completed/abandoned session detail view, only visible when `worktree_exists` is true.

### 4. Backend Changes

| Change | Location | Type |
|--------|----------|------|
| Close-check endpoint | `internal/api/api.go` | New handler |
| Close-check logic | `internal/sessions/manager.go` | New method |
| Reactivate endpoint | `internal/api/api.go` | New handler |
| Reactivate logic | `internal/sessions/manager.go` or `internal/agent/lifecycle.go` | New function |
| MCP close-check tool | `internal/mcp/server.go` | New tool (optional) |
| MCP reactivate tool | `internal/mcp/server.go` | New tool (optional) |

### 5. Frontend Changes

| Change | Location | Type |
|--------|----------|------|
| Close wizard dialog | `ui/src/components/sessions/close-wizard-dialog.tsx` | New component |
| Close-check hook | `ui/src/hooks/use-sessions.ts` | New query hook |
| Reactivate hook | `ui/src/hooks/use-sessions.ts` | New mutation hook |
| Reactivate button | `ui/src/components/sessions/session-detail.tsx` | Modified |
| Done button wiring | `ui/src/components/sessions/sessions-page.tsx` | Modified |

### 6. What Does NOT Change

- The existing `CloseSession` / `POST /api/v1/agent/close` behavior is unchanged
- Sync, Merge, Delete Worktree operations stay as-is
- Session status model stays the same (no new statuses)
- Issue cascade logic stays the same
