# Session Screen UI/UX Cleanup

**Date**: 2026-02-20
**Issue**: Session screen is cluttered and has duplicates (`01KHXYMPKEP4`)

## Problem

The sessions page suffers from three UX issues:

1. **Duplicate idle entries** — Two idle sessions for the same calsync branch pointing to the same worktree
2. **Abandoned clutter** — Many abandoned entries with 0 commits and <1 min durations (failed launches) dominate the list
3. **Action button bloat** — 5 stacked text buttons per row make rows very tall

## Design

### 1. Default View: Active + Idle Only

Change default selected tab from "All" to a combined Active+Idle view. Users can still click "All" or "Abandoned" to see everything.

**Files**: `ui/src/components/sessions/sessions-page.tsx`

### 2. Prevent Duplicate Sessions at Creation

Before creating a new session (in `agent launch` and `discover worktrees`), check for an existing active/idle session on the same branch+project. Reuse the existing session instead of creating a new record.

**Files**: `internal/agent/agent.go` (or equivalent launch logic), store queries

### 3. Auto-Purge Failed Launches

When creating a new session, automatically delete abandoned sessions for the same branch that have 0 commits and duration <60 seconds. These are clearly failed launches with no work to preserve.

**Files**: Store layer (new cleanup query), agent launch logic

### 4. Action Buttons → Dropdown Menu

Replace the 5 stacked buttons with a single `⋮` (vertical ellipsis) dropdown menu:

- **Primary group**: Resume, Sync, Merge
- **Separator**
- **Lifecycle group**: Done, Abandon

Completed/abandoned rows show no actions (or minimal: Reactivate only).

**Files**: `ui/src/components/sessions/sessions-page.tsx`, possibly extract to `session-row-menu.tsx`

### 5. Bulk Cleanup Button

Add "Clean up" button in the page header (next to "Discover Worktrees"):

- Shows count of removable sessions (abandoned, 0 commits, <60s duration)
- Confirmation dialog before deletion
- New API endpoint: `DELETE /api/v1/sessions/cleanup`

**Files**: `ui/src/components/sessions/sessions-page.tsx`, `internal/api/api.go`, store layer

## Out of Scope

- Branch-based grouping/collapsing (deferred — data-layer fixes should eliminate most clutter)
- Session detail page changes
- CLI output changes (CLI already filters to active+idle by default)
