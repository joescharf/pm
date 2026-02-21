# Fix: Duplicate Sessions & Idle Status Detection

**Date:** 2026-02-20
**Bugs:** Duplicate sessions created on launch; sessions stuck as idle when actively in use

## Problem Statement

Two related session management bugs:

1. **Duplicate sessions:** `pm agent launch` (CLI and MCP) creates a new session even when an idle session already exists for the same branch/worktree. Root cause: when `UpdateAgentSession()` fails during resume, the loop continues and falls through to session creation.

2. **Idle status detection:** Sessions show `idle` when a Claude process is actively running in the worktree. Root cause: `ReconcileSessions()` only checks filesystem state (worktree directory exists), not runtime state (claude process running). There is no automatic `active`<->`idle` transition.

## Bug 1: Duplicate Sessions

### Root Cause

**CLI** (`cmd/agent.go:206-234`): The resume loop finds a matching idle session, attempts `UpdateAgentSession()`, and if it fails, logs a warning and **continues the loop** instead of returning an error. After the loop, control falls through to `CreateAgentSession()`.

**MCP** (`internal/mcp/server.go:631-666`): Same pattern — if `UpdateAgentSession()` fails inside the `if err == nil` block, the loop silently continues to the next iteration, then falls through to create a new session.

### Fix

**A. Fix control flow (primary):**

In both CLI and MCP launch paths:
1. When a matching idle session is found and resume fails, **return an error** — do not continue the loop.
2. Add a pre-creation guard before `CreateAgentSession()`: query for any existing active/idle session on the same branch. If found, return an error instead of creating a duplicate.

**B. Database constraint (safety net):**

Add a partial unique index on `agent_sessions`:
```sql
CREATE UNIQUE INDEX idx_agent_sessions_active_branch
ON agent_sessions(project_id, branch)
WHERE status IN ('active', 'idle');
```

This prevents any code path from creating duplicate active/idle sessions for the same project+branch. Applied as a migration in store init.

**C. Cleanup existing duplicates:**

Add a one-time migration or startup cleanup that deduplicates existing sessions: for each `(project_id, branch)` group with multiple active/idle sessions, keep the most recent one (by `started_at`) and mark the rest as `abandoned`.

## Bug 2: Idle Status Detection

### Root Cause

`ReconcileSessions()` (`internal/agent/sync.go:17-49`) handles two cases:
- active/idle + worktree gone → abandoned
- abandoned + worktree exists → idle

**Missing:** No check for whether a `claude` process is running in the worktree. The `LastActiveAt` field is written but never read.

### Fix: Process Detection

Add a `DetectClaudeProcess(worktreePath string) bool` function in `internal/agent/` that:

1. Finds `claude` process PIDs via `pgrep -x claude`
2. For each PID, resolves its cwd via `lsof -a -p <pid> -d cwd -Fn`
3. Returns `true` if any claude process has a cwd that is the worktree path or a subdirectory of it

Update `ReconcileSessions()` with two new transitions:
- **idle + claude process detected → active** (set `Status = active`, update `LastActiveAt`)
- **active + no claude process → idle** (set `Status = idle`)

This runs on every `ReconcileSessions()` call, which already fires on session list/status API requests.

### Performance

`pgrep` + `lsof` per-PID is fast for the typical case (0-3 claude processes). Total overhead: ~10-50ms per reconciliation call. Since this only runs on explicit API/CLI queries (not on a timer), this is acceptable.

### Fallback

If `pgrep` or `lsof` aren't available (unlikely on macOS), the function returns `false` (no detection) and behavior degrades gracefully to the current manual model.

## Files Changed

| File | Change |
|------|--------|
| `internal/agent/sync.go` | Add `DetectClaudeProcess()`, expand `ReconcileSessions()` with active/idle transitions |
| `internal/agent/sync_test.go` | Tests for new reconciliation transitions (mock process detection) |
| `cmd/agent.go` | Fix resume loop: return error on update failure, add pre-creation guard |
| `internal/mcp/server.go` | Same resume loop fix for MCP launch handler |
| `internal/store/sqlite.go` | Add partial unique index migration, add dedup cleanup |
| `internal/store/sqlite_test.go` | Test that duplicate insert is rejected by constraint |

## Testing

- Unit test: `ReconcileSessions` transitions idle→active when process detector returns true
- Unit test: `ReconcileSessions` transitions active→idle when process detector returns false
- Unit test: Launch with existing idle session resumes it (no duplicate created)
- Unit test: Launch returns error if resume update fails (no fallthrough to create)
- Unit test: DB constraint rejects duplicate active/idle session for same branch
- Integration: Launch twice for same issue → only one session exists
