# wt Library Migration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace pm's CLI exec / manual AppleScript / manual state parsing with proper wt library calls, fixing the abandon bug and unifying all worktree lifecycle management.

**Architecture:** Two repos change: `wt` gets `internal/iterm` and `internal/claude` promoted to `pkg/`, plus a new `pkg/lifecycle` package that orchestrates create/delete/open workflows. `pm` then replaces its `internal/wt/wt.go` CLI wrapper and manual AppleScript in `sessions/manager.go` with imports from `wt/pkg/lifecycle`, `wt/pkg/iterm`, and `wt/pkg/wtstate`.

**Tech Stack:** Go, `github.com/joescharf/wt` library, `github.com/joescharf/pm` CLI

---

## Phase 1: wt repo — Promote internal packages to pkg/

### Task 1: Move `internal/iterm` → `pkg/iterm`

**Files:**
- Move: `wt/internal/iterm/iterm.go` → `wt/pkg/iterm/iterm.go`
- Move: `wt/internal/iterm/applescript.go` → `wt/pkg/iterm/applescript.go`
- Move: `wt/internal/iterm/applescript_test.go` → `wt/pkg/iterm/applescript_test.go`

**Step 1: Create pkg/iterm directory and copy files**

```bash
mkdir -p /Users/joescharf/app/wt/pkg/iterm
cp /Users/joescharf/app/wt/internal/iterm/iterm.go /Users/joescharf/app/wt/pkg/iterm/iterm.go
cp /Users/joescharf/app/wt/internal/iterm/applescript.go /Users/joescharf/app/wt/pkg/iterm/applescript.go
cp /Users/joescharf/app/wt/internal/iterm/applescript_test.go /Users/joescharf/app/wt/pkg/iterm/applescript_test.go
```

No code changes needed — the package name stays `iterm`, all types/functions are already exported.

**Step 2: Update all wt internal imports from `internal/iterm` → `pkg/iterm`**

Files to update:
- `wt/cmd/root.go` — imports `internal/iterm`
- `wt/cmd/create.go` — may reference iterm types
- `wt/cmd/delete.go` — references `itermClient`
- `wt/cmd/open.go` — references `itermClient`
- `wt/internal/mcp/server.go` — imports `internal/iterm`

In each file, change:
```go
// OLD
"github.com/joescharf/wt/internal/iterm"
// NEW
"github.com/joescharf/wt/pkg/iterm"
```

**Step 3: Delete internal/iterm/**

```bash
rm -rf /Users/joescharf/app/wt/internal/iterm
```

**Step 4: Run tests**

```bash
cd /Users/joescharf/app/wt && go build ./... && go test ./...
```

**Step 5: Commit**

```bash
cd /Users/joescharf/app/wt
git add -A && git commit -m "refactor: promote internal/iterm to pkg/iterm for library use"
```

---

### Task 2: Move `internal/claude` → `pkg/claude`

**Files:**
- Move: `wt/internal/claude/trust.go` → `wt/pkg/claude/trust.go`
- Move: `wt/internal/claude/trust_test.go` → `wt/pkg/claude/trust_test.go`

**Step 1: Create pkg/claude directory and copy files**

```bash
mkdir -p /Users/joescharf/app/wt/pkg/claude
cp /Users/joescharf/app/wt/internal/claude/trust.go /Users/joescharf/app/wt/pkg/claude/trust.go
cp /Users/joescharf/app/wt/internal/claude/trust_test.go /Users/joescharf/app/wt/pkg/claude/trust_test.go
```

**Step 2: Update wt imports from `internal/claude` → `pkg/claude`**

Files to update:
- `wt/cmd/root.go` — imports `internal/claude`
- Any other cmd/ files referencing `claudeTrust`

Change:
```go
// OLD
"github.com/joescharf/wt/internal/claude"
// NEW
"github.com/joescharf/wt/pkg/claude"
```

**Step 3: Delete internal/claude/**

```bash
rm -rf /Users/joescharf/app/wt/internal/claude
```

**Step 4: Run tests**

```bash
cd /Users/joescharf/app/wt && go build ./... && go test ./...
```

**Step 5: Commit**

```bash
cd /Users/joescharf/app/wt
git add -A && git commit -m "refactor: promote internal/claude to pkg/claude for library use"
```

---

### Task 3: Delete duplicate internal/state and internal/git

These are duplicates of `pkg/wtstate` and `pkg/gitops` respectively.

**Step 1: Check imports that reference internal/state**

```bash
cd /Users/joescharf/app/wt && grep -r '"github.com/joescharf/wt/internal/state"' --include='*.go'
```

Update any remaining references to use `pkg/wtstate`:
```go
// OLD
"github.com/joescharf/wt/internal/state"
// or
state "github.com/joescharf/wt/internal/state"
// NEW
"github.com/joescharf/wt/pkg/wtstate"
```

Note: The MCP server (`internal/mcp/server.go`) imports `internal/state` — update to `pkg/wtstate`.

**Step 2: Check imports that reference internal/git**

```bash
cd /Users/joescharf/app/wt && grep -r '"github.com/joescharf/wt/internal/git"' --include='*.go'
```

Update references to `pkg/gitops`. Note the MCP server uses a separate `GitClient` interface (`internal/mcp/gitclient.go`) that wraps `gitops` — this stays in internal/mcp.

**Step 3: Delete duplicates**

```bash
rm -rf /Users/joescharf/app/wt/internal/state
rm -rf /Users/joescharf/app/wt/internal/git
```

**Step 4: Run tests**

```bash
cd /Users/joescharf/app/wt && go build ./... && go test ./...
```

**Step 5: Commit**

```bash
cd /Users/joescharf/app/wt
git add -A && git commit -m "refactor: remove duplicate internal/state and internal/git packages"
```

---

## Phase 2: wt repo — Create pkg/lifecycle

### Task 4: Create `pkg/lifecycle` package with types and Manager

**Files:**
- Create: `wt/pkg/lifecycle/lifecycle.go`
- Test: `wt/pkg/lifecycle/lifecycle_test.go`

**Step 1: Write the test file with mock interfaces**

Create `wt/pkg/lifecycle/lifecycle_test.go`:

```go
package lifecycle

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	m := NewManager(nil, nil, nil, nil, nil)
	assert.NotNil(t, m)
}
```

**Step 2: Run test to verify it fails**

```bash
cd /Users/joescharf/app/wt && go test ./pkg/lifecycle/...
```

Expected: FAIL — package doesn't exist yet.

**Step 3: Write lifecycle.go with Manager struct and constructor**

Create `wt/pkg/lifecycle/lifecycle.go`:

```go
package lifecycle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/joescharf/wt/pkg/claude"
	"github.com/joescharf/wt/pkg/gitops"
	"github.com/joescharf/wt/pkg/iterm"
	"github.com/joescharf/wt/pkg/ops"
	"github.com/joescharf/wt/pkg/wtstate"
)

// Manager orchestrates worktree lifecycle operations:
// create (git worktree + trust + iTerm), open (focus/create iTerm),
// and delete (close iTerm + remove worktree + untrust + cleanup state).
type Manager struct {
	git    gitops.Client
	iterm  iterm.Client
	state  *wtstate.Manager
	trust  *claude.TrustManager
	logger ops.Logger
}

// NewManager creates a lifecycle manager. Any dependency can be nil
// (operations requiring it will be skipped or return an error).
func NewManager(git gitops.Client, itermClient iterm.Client, state *wtstate.Manager, trust *claude.TrustManager, logger ops.Logger) *Manager {
	if logger == nil {
		logger = &nopLogger{}
	}
	return &Manager{
		git:    git,
		iterm:  itermClient,
		state:  state,
		trust:  trust,
		logger: logger,
	}
}

// CreateOptions configures a Create operation.
type CreateOptions struct {
	Branch   string // Required: branch name
	Base     string // Base branch (default: "main")
	NoClaude bool   // Don't auto-launch claude in top pane
	Existing bool   // Use existing branch instead of creating new
}

// CreateResult holds the result of creating a worktree.
type CreateResult struct {
	WorktreePath string
	Branch       string
	RepoName     string
	SessionIDs   *iterm.SessionIDs
	Existed      bool // true if worktree already existed, just opened window
}

// Create creates a worktree with iTerm window, trust, and state management.
// If the worktree already exists, delegates to Open.
func (m *Manager) Create(ctx context.Context, opts CreateOptions) (*CreateResult, error) {
	if m.git == nil {
		return nil, fmt.Errorf("git client required for Create")
	}
	if opts.Branch == "" {
		return nil, fmt.Errorf("branch is required")
	}

	repoName, err := m.git.RepoName()
	if err != nil {
		return nil, fmt.Errorf("get repo name: %w", err)
	}

	wtDir, err := m.git.WorktreesDir()
	if err != nil {
		return nil, fmt.Errorf("get worktrees dir: %w", err)
	}

	dirname := gitops.BranchToDirname(opts.Branch)
	wtPath := filepath.Join(wtDir, dirname)

	// If worktree already exists, delegate to Open
	if info, err := os.Stat(wtPath); err == nil && info.IsDir() {
		m.logger.Info("Worktree already exists, opening window")
		openResult, err := m.Open(ctx, wtPath, OpenOptions{NoClaude: opts.NoClaude})
		if err != nil {
			return nil, err
		}
		return &CreateResult{
			WorktreePath: wtPath,
			Branch:       opts.Branch,
			RepoName:     repoName,
			SessionIDs:   openResult.SessionIDs,
			Existed:      true,
		}, nil
	}

	// Create worktrees directory if needed
	if _, err := os.Stat(wtDir); os.IsNotExist(err) {
		if err := os.MkdirAll(wtDir, 0755); err != nil {
			return nil, fmt.Errorf("create worktrees directory: %w", err)
		}
	}

	// Check if branch already exists
	branchExists, err := m.git.BranchExists(opts.Branch)
	if err != nil {
		return nil, fmt.Errorf("check branch: %w", err)
	}

	useExisting := opts.Existing || branchExists

	// Create git worktree
	base := opts.Base
	if base == "" {
		base = "main"
	}

	if useExisting {
		m.logger.Info("Creating worktree from existing branch '%s'", opts.Branch)
		err = m.git.WorktreeAdd(wtPath, opts.Branch, "", false)
	} else {
		m.logger.Info("Creating worktree with new branch '%s' from '%s'", opts.Branch, base)
		err = m.git.WorktreeAdd(wtPath, opts.Branch, base, true)
	}
	if err != nil {
		return nil, fmt.Errorf("create worktree: %w", err)
	}
	m.logger.Success("Git worktree created")

	// Pre-approve Claude Code trust
	if m.trust != nil {
		if added, trustErr := m.trust.TrustProject(wtPath); trustErr != nil {
			m.logger.Warning("Failed to set Claude trust: %v", trustErr)
		} else if added {
			m.logger.Verbose("Claude trust set for %s", wtPath)
		}
	}

	// Create iTerm2 window
	var sessionIDs *iterm.SessionIDs
	if m.iterm != nil {
		sessionName := fmt.Sprintf("wt:%s:%s", repoName, dirname)
		m.logger.Info("Creating iTerm2 window (session: %s)", sessionName)

		sessionIDs, err = m.iterm.CreateWorktreeWindow(wtPath, sessionName, opts.NoClaude)
		if err != nil {
			m.logger.Warning("Worktree created but failed to open iTerm2 window: %v", err)
		}
	}

	// Save wt state
	if m.state != nil && sessionIDs != nil {
		stateErr := m.state.SetWorktree(wtPath, &wtstate.WorktreeState{
			Repo:            repoName,
			Branch:          opts.Branch,
			ClaudeSessionID: sessionIDs.ClaudeSessionID,
			ShellSessionID:  sessionIDs.ShellSessionID,
			CreatedAt:       wtstate.FlexTime{Time: time.Now().UTC()},
		})
		if stateErr != nil {
			m.logger.Warning("Failed to save state: %v", stateErr)
		}
	}

	return &CreateResult{
		WorktreePath: wtPath,
		Branch:       opts.Branch,
		RepoName:     repoName,
		SessionIDs:   sessionIDs,
	}, nil
}

// DeleteOptions configures a Delete operation.
type DeleteOptions struct {
	Force        bool // Skip safety checks (dirty worktree, etc.)
	DeleteBranch bool // Also delete the git branch
	DryRun       bool // Show what would happen without doing it
}

// Delete performs full worktree cleanup: close iTerm → remove git worktree →
// untrust Claude → remove wt state → optionally delete branch.
func (m *Manager) Delete(ctx context.Context, worktreePath string, opts DeleteOptions) error {
	if m.git == nil {
		return fmt.Errorf("git client required for Delete")
	}

	dirname := filepath.Base(worktreePath)

	// 1. Close iTerm2 window (best-effort, before removing worktree)
	if m.iterm != nil && m.state != nil {
		ws, _ := m.state.GetWorktree(worktreePath)
		if ws != nil && ws.ClaudeSessionID != "" {
			if m.iterm.SessionExists(ws.ClaudeSessionID) {
				if err := m.iterm.CloseWindow(ws.ClaudeSessionID); err != nil {
					m.logger.Warning("Failed to close iTerm2 window: %v", err)
				} else {
					m.logger.Success("Closed iTerm2 window")
				}
			}
		}
	}

	// 2. Get branch name from state before removing
	branchName := dirname
	if m.state != nil {
		ws, _ := m.state.GetWorktree(worktreePath)
		if ws != nil && ws.Branch != "" {
			branchName = ws.Branch
		}
	}

	// 3. Remove git worktree
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		// Already gone — just clean up state
		m.logger.Info("Worktree already removed: %s", worktreePath)
	} else if opts.DryRun {
		m.logger.Info("Would remove git worktree: %s", worktreePath)
	} else {
		if err := m.git.WorktreeRemove(worktreePath, opts.Force); err != nil {
			return fmt.Errorf("remove worktree: %w", err)
		}
		m.logger.Success("Removed git worktree '%s'", dirname)
	}

	// 4. Delete branch if requested
	if opts.DeleteBranch && !opts.DryRun {
		err := m.git.BranchDelete(branchName, false)
		if err != nil && opts.Force {
			err = m.git.BranchDelete(branchName, true)
		}
		if err != nil {
			m.logger.Warning("Could not delete branch '%s': %v", branchName, err)
		} else {
			m.logger.Success("Deleted branch '%s'", branchName)
		}
	}

	// 5. Remove wt state entry
	if !opts.DryRun && m.state != nil {
		_ = m.state.RemoveWorktree(worktreePath)
	}

	// 6. Remove Claude trust
	if !opts.DryRun && m.trust != nil {
		if err := m.trust.UntrustProject(worktreePath); err != nil {
			m.logger.Warning("Failed to remove Claude trust: %v", err)
		}
	}

	return nil
}

// OpenOptions configures an Open operation.
type OpenOptions struct {
	NoClaude bool // Don't auto-launch claude in top pane
}

// OpenResult holds the result of opening/focusing a worktree window.
type OpenResult struct {
	WorktreePath string
	SessionIDs   *iterm.SessionIDs
	Action       string // "focused" or "opened"
}

// Open focuses an existing iTerm window or creates a new one for the worktree.
func (m *Manager) Open(ctx context.Context, worktreePath string, opts OpenOptions) (*OpenResult, error) {
	if m.iterm == nil {
		return nil, fmt.Errorf("iTerm client required for Open")
	}

	dirname := filepath.Base(worktreePath)

	// Check if window already exists via state
	if m.state != nil {
		ws, _ := m.state.GetWorktree(worktreePath)
		if ws != nil && ws.ClaudeSessionID != "" {
			if m.iterm.SessionExists(ws.ClaudeSessionID) {
				m.logger.Info("iTerm2 window already open, focusing it")
				if err := m.iterm.FocusWindow(ws.ClaudeSessionID); err != nil {
					return nil, fmt.Errorf("focus window: %w", err)
				}
				return &OpenResult{
					WorktreePath: worktreePath,
					SessionIDs: &iterm.SessionIDs{
						ClaudeSessionID: ws.ClaudeSessionID,
						ShellSessionID:  ws.ShellSessionID,
					},
					Action: "focused",
				}, nil
			}
		}
	}

	// Pre-approve Claude trust
	if m.trust != nil {
		if added, err := m.trust.TrustProject(worktreePath); err != nil {
			m.logger.Warning("Failed to set Claude trust: %v", err)
		} else if added {
			m.logger.Verbose("Claude trust set for %s", worktreePath)
		}
	}

	// Create new iTerm window
	repoName := dirname // fallback
	if m.git != nil {
		if name, err := m.git.RepoName(); err == nil {
			repoName = name
		}
	}
	sessionName := fmt.Sprintf("wt:%s:%s", repoName, dirname)
	m.logger.Info("Opening iTerm2 window for '%s'", dirname)

	sessions, err := m.iterm.CreateWorktreeWindow(worktreePath, sessionName, opts.NoClaude)
	if err != nil {
		return nil, fmt.Errorf("create iTerm2 window: %w", err)
	}

	// Update wt state with new session IDs
	if m.state != nil {
		// Get branch from git
		branchName := dirname
		if m.git != nil {
			if b, bErr := m.git.CurrentBranch(worktreePath); bErr == nil {
				branchName = b
			}
		}

		stateErr := m.state.SetWorktree(worktreePath, &wtstate.WorktreeState{
			Repo:            repoName,
			Branch:          branchName,
			ClaudeSessionID: sessions.ClaudeSessionID,
			ShellSessionID:  sessions.ShellSessionID,
			CreatedAt:       wtstate.FlexTime{Time: time.Now().UTC()},
		})
		if stateErr != nil {
			m.logger.Warning("Failed to save state: %v", stateErr)
		}
	}

	return &OpenResult{
		WorktreePath: worktreePath,
		SessionIDs:   sessions,
		Action:       "opened",
	}, nil
}

// nopLogger discards all log output.
type nopLogger struct{}

func (l *nopLogger) Info(format string, args ...interface{})    {}
func (l *nopLogger) Success(format string, args ...interface{}) {}
func (l *nopLogger) Warning(format string, args ...interface{}) {}
func (l *nopLogger) Error(format string, args ...interface{})   {}
func (l *nopLogger) Verbose(format string, args ...interface{}) {}
```

**Step 4: Run tests**

```bash
cd /Users/joescharf/app/wt && go test ./pkg/lifecycle/...
```

Expected: PASS

**Step 5: Commit**

```bash
cd /Users/joescharf/app/wt
git add -A && git commit -m "feat: add pkg/lifecycle for orchestrated worktree create/delete/open"
```

---

### Task 5: Update wt's cmd/ to use pkg/lifecycle

**Files:**
- Modify: `wt/cmd/create.go` — use `lifecycle.Manager.Create()`
- Modify: `wt/cmd/delete.go` — use `lifecycle.Manager.Delete()`
- Modify: `wt/cmd/open.go` — use `lifecycle.Manager.Open()`
- Modify: `wt/cmd/root.go` — add lifecycle Manager as package-level dependency

**Step 1: Add lifecycle Manager to root.go**

In `wt/cmd/root.go`, add:
```go
import "github.com/joescharf/wt/pkg/lifecycle"

var lifecycleMgr *lifecycle.Manager
```

Initialize in `initDeps()` (or wherever gitClient/itermClient/stateMgr are initialized):
```go
lifecycleMgr = lifecycle.NewManager(gitClient, itermClient, stateMgr, claudeTrust, output)
```

**Step 2: Simplify create.go to delegate to lifecycle**

Replace the body of `createRun()` with a call to `lifecycleMgr.Create()`. Map flags to `lifecycle.CreateOptions`.

**Step 3: Simplify delete.go to delegate to lifecycle**

Replace `cleanupWorktree()` with a call to `lifecycleMgr.Delete()`. Keep the safety check prompt logic in cmd/ (it's UI concern).

**Step 4: Simplify open.go to delegate to lifecycle**

Replace `openRun()` with a call to `lifecycleMgr.Open()`.

**Step 5: Run tests**

```bash
cd /Users/joescharf/app/wt && go build ./... && go test ./...
```

**Step 6: Commit**

```bash
cd /Users/joescharf/app/wt
git add -A && git commit -m "refactor: cmd/ uses pkg/lifecycle for create/delete/open"
```

---

### Task 6: Tag wt release

**Step 1: Tag and push**

```bash
cd /Users/joescharf/app/wt
git tag v0.7.0
git push origin main --tags
```

---

## Phase 3: pm repo — Replace CLI exec and manual code with library imports

### Task 7: Update pm's go.mod to use new wt version

**Files:**
- Modify: `pm/go.mod`

**Step 1: Update dependency**

```bash
cd /Users/joescharf/app/pm
go get github.com/joescharf/wt@v0.7.0
go mod tidy
```

**Step 2: Verify build**

```bash
go build ./...
```

**Step 3: Commit**

```bash
git add go.mod go.sum && git commit -m "deps: update wt to v0.7.0 with pkg/lifecycle"
```

---

### Task 8: Replace pm's `internal/wt/wt.go` with lifecycle-backed implementation

**Files:**
- Modify: `pm/internal/wt/wt.go` — replace CLI exec with lifecycle.Manager calls

**Step 1: Rewrite internal/wt/wt.go**

The `Client` interface stays the same (pm depends on it). The `RealClient` implementation changes from exec.Command("wt", ...) to lifecycle.Manager calls:

```go
package wt

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/joescharf/wt/pkg/claude"
	"github.com/joescharf/wt/pkg/gitops"
	"github.com/joescharf/wt/pkg/iterm"
	"github.com/joescharf/wt/pkg/lifecycle"
	"github.com/joescharf/wt/pkg/wtstate"
)

// WorktreeInfo represents a worktree from wt state.
type WorktreeInfo struct {
	Path   string `json:"path"`
	Branch string `json:"branch"`
	Repo   string `json:"repo"`
}

// Client wraps the wt lifecycle for worktree operations.
type Client interface {
	Create(repoPath, branch string) error
	List(repoPath string) ([]WorktreeInfo, error)
	Delete(repoPath, branch string) error
	Lifecycle() *lifecycle.Manager
}

// RealClient implements Client using wt library packages.
type RealClient struct {
	lm *lifecycle.Manager
}

// NewClient returns a new wt client backed by lifecycle.Manager.
func NewClient() *RealClient {
	// Create dependencies
	itermClient := iterm.NewClient()

	home, _ := os.UserHomeDir()
	statePath := filepath.Join(home, ".config", "wt", "state.json")
	stateMgr := wtstate.NewManager(statePath)

	claudePath, _ := claude.DefaultPath()
	var trustMgr *claude.TrustManager
	if claudePath != "" {
		trustMgr = claude.NewTrustManager(claudePath)
	}

	lm := lifecycle.NewManager(nil, itermClient, stateMgr, trustMgr, nil)
	return &RealClient{lm: lm}
}

func (c *RealClient) Create(repoPath, branch string) error {
	// Create a repo-bound git client for this specific repo
	git := newRepoBoundGitClient(repoPath)
	lm := lifecycle.NewManager(git, c.lm.ITerm(), c.lm.State(), c.lm.Trust(), nil)
	_, err := lm.Create(context.Background(), lifecycle.CreateOptions{
		Branch: branch,
	})
	return err
}

func (c *RealClient) List(repoPath string) ([]WorktreeInfo, error) {
	git := newRepoBoundGitClient(repoPath)
	worktrees, err := git.WorktreeList()
	if err != nil {
		return nil, err
	}

	var result []WorktreeInfo
	for _, wt := range worktrees {
		result = append(result, WorktreeInfo{
			Path:   wt.Path,
			Branch: wt.Branch,
			Repo:   repoPath,
		})
	}
	return result, nil
}

func (c *RealClient) Delete(repoPath, branch string) error {
	git := newRepoBoundGitClient(repoPath)
	wtPath, err := git.ResolveWorktree(branch)
	if err != nil {
		// Try dirname convention
		wtDir, _ := git.WorktreesDir()
		dirname := gitops.BranchToDirname(branch)
		wtPath = filepath.Join(wtDir, dirname)
	}
	lm := lifecycle.NewManager(git, c.lm.ITerm(), c.lm.State(), c.lm.Trust(), nil)
	return lm.Delete(context.Background(), wtPath, lifecycle.DeleteOptions{
		Force: true,
	})
}

func (c *RealClient) Lifecycle() *lifecycle.Manager {
	return c.lm
}
```

Note: This requires adding accessor methods to lifecycle.Manager:
```go
func (m *Manager) ITerm() iterm.Client       { return m.iterm }
func (m *Manager) State() *wtstate.Manager   { return m.state }
func (m *Manager) Trust() *claude.TrustManager { return m.trust }
```

Add these to `wt/pkg/lifecycle/lifecycle.go`.

**Step 2: Create `pm/internal/wt/gitclient.go`** for the repo-bound helper

This is a minimal gitops.Client that wraps git commands for a specific repo path. It only needs the methods used by lifecycle.Manager (WorktreeAdd, WorktreeRemove, BranchExists, BranchDelete, RepoName, WorktreesDir, WorktreeList, CurrentBranch, ResolveWorktree). pm already has `sessions/gitadapter.go` with this exact pattern — extract a shared helper or import from there.

**Step 3: Run tests**

```bash
cd /Users/joescharf/app/pm && go build ./...
```

**Step 4: Commit**

```bash
git add internal/wt/ && git commit -m "refactor: replace wt CLI exec with lifecycle library calls"
```

---

### Task 9: Replace manual AppleScript and state parsing in sessions/manager.go

**Files:**
- Modify: `pm/internal/sessions/manager.go`
- Modify: `pm/internal/mcp/server.go`

**Step 1: Remove all manual iTerm/state code from manager.go**

Delete these functions from `pm/internal/sessions/manager.go`:
- `CloseITermWindow()` (lines 476-487)
- `getWtSessionID()` (lines 489-514)
- `closeITermWindowByID()` (lines 516-533)
- `closeITermWindowByName()` (lines 535-553)

The `nopLogger` struct (lines 555-562) can stay since it implements `ops.Logger` for use with the ops package.

**Step 2: Update Manager to accept a lifecycle.Manager**

```go
type Manager struct {
	store     store.Store
	lifecycle *lifecycle.Manager
}

func NewManager(s store.Store, lm *lifecycle.Manager) *Manager {
	return &Manager{store: s, lifecycle: lm}
}
```

**Step 3: Update MergeSession cleanup to use lifecycle.Delete**

Replace lines 263-280 in manager.go (the post-merge cleanup block):

```go
// Post-merge cleanup
if result.Success && !opts.CreatePR && opts.Cleanup && !opts.DryRun && session.WorktreePath != "" {
    if m.lifecycle != nil {
        if delErr := m.lifecycle.Delete(ctx, session.WorktreePath, lifecycle.DeleteOptions{
            Force:        true,
            DeleteBranch: true,
        }); delErr == nil {
            session.WorktreePath = ""
            _ = m.store.UpdateAgentSession(ctx, session)
            result.Cleaned = true
        }
    }
}
```

**Step 4: Update DeleteWorktree to use lifecycle.Delete**

Replace lines 285-338 in manager.go:

```go
func (m *Manager) DeleteWorktree(ctx context.Context, sessionID string, force bool) error {
	session, err := m.store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	if session.WorktreePath == "" {
		return fmt.Errorf("session %s has no worktree path", sessionID)
	}

	if m.lifecycle != nil {
		if err := m.lifecycle.Delete(ctx, session.WorktreePath, lifecycle.DeleteOptions{
			Force: force,
		}); err != nil {
			return fmt.Errorf("delete worktree: %w", err)
		}
	}

	// Update session
	now := time.Now().UTC()
	session.Status = models.SessionStatusAbandoned
	session.EndedAt = &now
	session.WorktreePath = ""

	if err := m.store.UpdateAgentSession(ctx, session); err != nil {
		return fmt.Errorf("update session: %w", err)
	}

	// Cascade issue status
	if session.IssueID != "" {
		issue, issErr := m.store.GetIssue(ctx, session.IssueID)
		if issErr == nil && issue.Status == models.IssueStatusInProgress {
			issue.Status = models.IssueStatusOpen
			_ = m.store.UpdateIssue(ctx, issue)
		}
	}

	return nil
}
```

**Step 5: Update MCP server.go close_agent handler**

In `pm/internal/mcp/server.go` `handleCloseAgent()`, when target is `abandoned`, call lifecycle.Delete to remove the worktree:

Replace lines 740-756:

```go
// Enrich session with git info before closing; capture session for cleanup
var worktreePath string
var branch string
if sess, err := s.store.GetAgentSession(ctx, sessionID); err == nil {
    worktreePath = sess.WorktreePath
    branch = sess.Branch
    agent.EnrichSessionWithGitInfo(sess, s.git)
    _ = s.store.UpdateAgentSession(ctx, sess)
}

session, err := agent.CloseSession(ctx, s.store, sessionID, target)
if err != nil {
    return mcp.NewToolResultError(err.Error()), nil
}

// For abandoned: full worktree teardown via lifecycle
// For completed/idle: just close iTerm window
if worktreePath != "" && target == models.SessionStatusAbandoned {
    if s.lifecycle != nil {
        _ = s.lifecycle.Delete(ctx, worktreePath, lifecycle.DeleteOptions{Force: true})
    }
    // Clear worktree path since it's been deleted
    session.WorktreePath = ""
    _ = s.store.UpdateAgentSession(ctx, session)
} else if worktreePath != "" && target == models.SessionStatusCompleted {
    // Just close iTerm for completed (worktree may still be needed for review)
    if s.lifecycle != nil {
        // Close iTerm only — use iterm client directly or lifecycle.CloseWindow helper
    }
}
```

**Step 6: Update MCP NewServer to accept lifecycle.Manager**

In `pm/internal/mcp/server.go`:

```go
type Server struct {
	store     store.Store
	git       git.Client
	gh        git.GitHubClient
	wt        wt.Client
	llm       *llm.Client
	scorer    *health.Scorer
	sessions  *sessions.Manager
	lifecycle *lifecycle.Manager
}

func NewServer(s store.Store, gc git.Client, ghc git.GitHubClient, wtc wt.Client, llmClient *llm.Client) *Server {
	return &Server{
		store:     s,
		git:       gc,
		gh:        ghc,
		wt:        wtc,
		llm:       llmClient,
		scorer:    health.NewScorer(),
		sessions:  sessions.NewManager(s, wtc.Lifecycle()),
		lifecycle: wtc.Lifecycle(),
	}
}
```

**Step 7: Update CLI agent close to use lifecycle for abandon**

In `pm/cmd/agent.go` `agentCloseRun()`, add worktree deletion for abandoned:

```go
// After agent.CloseSession()...
if worktreePath != "" && target == models.SessionStatusAbandoned {
    wtClient := wt.NewClient()
    lm := wtClient.Lifecycle()
    // Need repo-bound lifecycle for this project
    _ = lm.Delete(context.Background(), worktreePath, lifecycle.DeleteOptions{Force: true})
}
```

**Step 8: Run tests**

```bash
cd /Users/joescharf/app/pm && go build ./... && go test ./...
```

**Step 9: Commit**

```bash
git add -A && git commit -m "refactor: replace manual AppleScript/state with wt lifecycle library"
```

---

### Task 10: Remove dead code and clean up imports

**Files:**
- Modify: `pm/internal/sessions/manager.go` — remove `os/exec` import, dead functions
- Modify: `pm/internal/wt/wt.go` — remove old StateReader if unused
- Verify: `pm/go.mod` — ensure clean `go mod tidy`

**Step 1: Remove unused imports and dead code**

- `sessions/manager.go`: Remove `os/exec` import (no longer used)
- `internal/wt/wt.go`: Remove old `StateReader`, `WtState`, `WtStateEntry` types (if still present)

**Step 2: Run go mod tidy**

```bash
cd /Users/joescharf/app/pm && go mod tidy
```

**Step 3: Run full test suite**

```bash
cd /Users/joescharf/app/pm && make test
```

**Step 4: Commit**

```bash
git add -A && git commit -m "chore: remove dead code from wt library migration"
```

---

## Phase 4: Validation

### Task 11: End-to-end validation of agent launch + abandon cycle

**Step 1: Launch an agent**

```bash
pm agent launch pm --issue 01KHV8JF02C3
```

Verify:
- Worktree created at `pm.worktrees/<dirname>`
- iTerm window opened with claude + shell panes
- Session status = active
- Issue status = in_progress
- wt state.json has entry for the worktree

**Step 2: Close as abandoned**

```bash
pm agent close <session_id> --abandon
```

Verify:
- iTerm window closed
- Worktree directory deleted
- Session status = abandoned (stays abandoned because worktree is gone)
- Issue status = open (cascade)
- wt state.json entry removed

**Step 3: Run agent list (reconciliation check)**

```bash
pm agent list pm
```

Verify: Session does NOT reappear as idle (the abandon bug is fixed).

**Step 4: Test MCP path**

Use `pm_launch_agent` and `pm_close_agent` MCP tools to repeat the same flow and verify identical behavior.

**Step 5: Commit validation notes if needed**

---

### Task 12: Validate merge cleanup path

**Step 1: Create a worktree with a test commit**

```bash
pm agent launch pm --branch test-merge-cleanup
# In worktree: touch test.txt && git add test.txt && git commit -m "test"
```

**Step 2: Merge the session**

```bash
pm agent merge <session_id>
```

Verify:
- iTerm window closed (via lifecycle.Delete)
- Worktree removed
- Branch deleted
- Session status = completed
- wt state.json cleaned up
