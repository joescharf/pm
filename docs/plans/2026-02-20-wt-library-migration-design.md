# wt Library Migration Design

**Date:** 2026-02-20
**Status:** Approved

## Problem

pm currently interacts with `wt` through a mix of approaches:

1. **CLI exec** — `internal/wt/wt.go` shells out to `wt open` and `wt rm` via `exec.Command`
2. **Library (partial)** — `sessions/manager.go` imports `wt/pkg/ops` and `wt/pkg/gitops` for sync/merge/delete
3. **Manual AppleScript** — `sessions/manager.go` has ~80 lines of hand-rolled osascript for iTerm window management
4. **Manual state parsing** — `sessions/manager.go` reads `~/.config/wt/state.json` directly

This causes bugs. When `close_agent(abandoned)` is called:
- Session state transitions to `abandoned` and iTerm is closed (manually)
- But the worktree is NOT deleted
- `ReconcileSessions()` sees the worktree still exists and recovers the session to `idle`
- Issue cascade already fired (→ open) but session is now `idle` — inconsistent state

## Solution

### Changes to `wt` repo

**Move internal packages to pkg/ for library consumption:**
- `internal/iterm` → `pkg/iterm` (iTerm2 client + AppleScript)
- `internal/claude` → `pkg/claude` (Claude Code trust management)
- Delete `internal/state` and `internal/git` (duplicates of `pkg/wtstate` and `pkg/gitops`)
- Update all `wt` internal imports accordingly

**New `pkg/lifecycle` package** — high-level orchestrated workflows:

```go
package lifecycle

type Manager struct {
    git    gitops.Client
    iterm  iterm.Client
    state  *wtstate.Manager
    trust  *claude.TrustManager
    logger ops.Logger
}

func NewManager(git, iterm, state, trust, logger) *Manager

// Create: git worktree add → trust → iTerm window → save state
type CreateOptions struct {
    Branch, Base string
    NoClaude, Existing bool
}
type CreateResult struct {
    WorktreePath, Branch string
    SessionIDs *iterm.SessionIDs
    Existed bool
}
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (*CreateResult, error)

// Delete: close iTerm → remove worktree → untrust → remove state → optionally delete branch
type DeleteOptions struct {
    Force, DeleteBranch, DryRun bool
}
func (m *Manager) Delete(ctx context.Context, worktreePath string, opts DeleteOptions) error

// Open: focus existing iTerm window or create new one
type OpenOptions struct{ NoClaude bool }
type OpenResult struct {
    WorktreePath string
    SessionIDs *iterm.SessionIDs
    Action string // "focused" or "opened"
}
func (m *Manager) Open(ctx context.Context, worktreePath string, opts OpenOptions) (*OpenResult, error)
```

### Changes to `pm` repo

**Replace `internal/wt/wt.go`** — the `Client` interface implementation calls `lifecycle.Manager` instead of `exec.Command("wt", ...)`.

**Replace manual AppleScript in `sessions/manager.go`** — `CloseITermWindow()` and all osascript code replaced by `iterm.Client.CloseWindow(sessionID)`.

**Replace manual state.json reading** — `getWtSessionID()` uses `wtstate.Manager.GetWorktree()` instead of hand-parsing JSON.

**Fix abandon bug** — `close_agent(abandoned)` calls `lifecycle.Delete()` for full teardown (iTerm close + worktree remove + state cleanup).

**Fix merge/delete in sessions manager** — `DeleteWorktree()` and merge cleanup use `lifecycle.Delete()` instead of `ops.Delete()` + manual `CloseITermWindow()`.

### What gets removed from pm

- `internal/sessions/manager.go`: `CloseITermWindow()`, `getWtSessionID()`, `closeITermWindowByID()`, `closeITermWindowByName()`, `nopLogger` — ~80 lines of AppleScript
- `internal/wt/wt.go`: All `exec.Command("wt", ...)` calls replaced by library imports
