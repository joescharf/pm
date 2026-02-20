package sessions

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/store"
	"github.com/joescharf/wt/pkg/ops"
)

// Manager orchestrates wt ops with pm's session store.
type Manager struct {
	store store.Store
}

// NewManager creates a new sessions manager.
func NewManager(s store.Store) *Manager {
	return &Manager{store: s}
}

// SyncOptions configures a session sync operation.
type SyncOptions struct {
	Rebase bool
	Force  bool
	DryRun bool
}

// SyncResult holds the result of syncing a session's worktree.
type SyncResult struct {
	SessionID string
	Branch    string
	Success   bool
	Ahead     int
	Behind    int
	Synced    bool // true if already in sync
	Conflicts []string
	Error     string
}

// MergeOptions configures a session merge operation.
type MergeOptions struct {
	BaseBranch string
	CreatePR   bool
	PRTitle    string
	PRBody     string
	PRDraft    bool
	Force      bool
	DryRun     bool
}

// MergeResult holds the result of merging a session's worktree.
type MergeResult struct {
	SessionID string
	Branch    string
	Success   bool
	PRCreated bool
	PRURL     string
	Conflicts []string
	Error     string
}

// SyncSession syncs a session's worktree with the base branch.
func (m *Manager) SyncSession(ctx context.Context, sessionID string, opts SyncOptions) (*SyncResult, error) {
	session, err := m.store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	if session.WorktreePath == "" {
		return nil, fmt.Errorf("session %s has no worktree path", sessionID)
	}

	if _, err := os.Stat(session.WorktreePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("worktree directory does not exist: %s", session.WorktreePath)
	}

	project, err := m.store.GetProject(ctx, session.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	// Create gitops client bound to the project's repo
	gitClient := newRepoBoundClient(project.Path)

	strategy := "merge"
	if opts.Rebase {
		strategy = "rebase"
	}

	syncOpts := ops.SyncOptions{
		BaseBranch: "main",
		Strategy:   strategy,
		Force:      opts.Force,
		DryRun:     opts.DryRun,
	}

	logger := &nopLogger{}
	syncResult, err := ops.Sync(ctx, gitClient, nil, logger, session.WorktreePath, syncOpts)

	result := &SyncResult{
		SessionID: sessionID,
		Branch:    session.Branch,
	}

	if syncResult != nil {
		result.Ahead = syncResult.Ahead
		result.Behind = syncResult.Behind
		result.Success = syncResult.Success
		result.Synced = syncResult.AlreadySynced

		if syncResult.HasConflicts {
			result.Conflicts = syncResult.ConflictFiles
		}
		if syncResult.Error != nil {
			result.Error = syncResult.Error.Error()
		}
	}

	// Update session state
	now := time.Now().UTC()
	if !opts.DryRun {
		session.LastSyncAt = &now
		if syncResult != nil && syncResult.HasConflicts {
			session.ConflictState = models.ConflictStateSyncConflict
			files := syncResult.ConflictFiles
			if files == nil {
				files = []string{}
			}
			conflictJSON, _ := json.Marshal(files)
			session.ConflictFiles = string(conflictJSON)
			session.LastError = syncResult.Error.Error()
		} else if err != nil {
			session.LastError = err.Error()
		} else {
			session.ConflictState = models.ConflictStateNone
			session.ConflictFiles = "[]"
			session.LastError = ""
		}
		_ = m.store.UpdateAgentSession(ctx, session)
	}

	if err != nil && (syncResult == nil || !syncResult.HasConflicts) {
		return result, err
	}

	return result, nil
}

// MergeSession merges a session's worktree branch into the base branch.
func (m *Manager) MergeSession(ctx context.Context, sessionID string, opts MergeOptions) (*MergeResult, error) {
	session, err := m.store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	if session.WorktreePath == "" {
		return nil, fmt.Errorf("session %s has no worktree path", sessionID)
	}

	if _, err := os.Stat(session.WorktreePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("worktree directory does not exist: %s", session.WorktreePath)
	}

	project, err := m.store.GetProject(ctx, session.ProjectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	gitClient := newRepoBoundClient(project.Path)

	baseBranch := opts.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	mergeOpts := ops.MergeOptions{
		BaseBranch: baseBranch,
		Strategy:   "merge",
		Force:      opts.Force,
		DryRun:     opts.DryRun,
		CreatePR:   opts.CreatePR,
		PRTitle:    opts.PRTitle,
		PRBody:     opts.PRBody,
		PRDraft:    opts.PRDraft,
	}

	logger := &nopLogger{}
	mergeResult, err := ops.Merge(ctx, gitClient, nil, logger, session.WorktreePath, mergeOpts, nil)

	result := &MergeResult{
		SessionID: sessionID,
		Branch:    session.Branch,
	}

	if mergeResult != nil {
		result.Success = mergeResult.Success
		result.PRCreated = mergeResult.PRCreated
		result.PRURL = mergeResult.PRURL

		if mergeResult.HasConflicts {
			result.Conflicts = mergeResult.ConflictFiles
		}
		if mergeResult.Error != nil {
			result.Error = mergeResult.Error.Error()
		}
	}

	// Update session state
	if !opts.DryRun {
		if mergeResult != nil && mergeResult.HasConflicts {
			session.ConflictState = models.ConflictStateMergeConflict
			files := mergeResult.ConflictFiles
			if files == nil {
				files = []string{}
			}
			conflictJSON, _ := json.Marshal(files)
			session.ConflictFiles = string(conflictJSON)
			session.LastError = mergeResult.Error.Error()
		} else if mergeResult != nil && mergeResult.Success {
			session.ConflictState = models.ConflictStateNone
			session.ConflictFiles = "[]"
			session.LastError = ""
			// Mark session as completed on successful merge
			now := time.Now().UTC()
			session.Status = models.SessionStatusCompleted
			session.EndedAt = &now
			// Cascade issue status
			if session.IssueID != "" {
				issue, issErr := m.store.GetIssue(ctx, session.IssueID)
				if issErr == nil && issue.Status == models.IssueStatusInProgress {
					issue.Status = models.IssueStatusDone
					_ = m.store.UpdateIssue(ctx, issue)
				}
			}
		} else if err != nil {
			session.LastError = err.Error()
		}
		_ = m.store.UpdateAgentSession(ctx, session)
	}

	if err != nil && (mergeResult == nil || !mergeResult.HasConflicts) {
		return result, err
	}

	return result, nil
}

// DeleteWorktree removes a session's worktree.
func (m *Manager) DeleteWorktree(ctx context.Context, sessionID string, force bool) error {
	session, err := m.store.GetAgentSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	if session.WorktreePath == "" {
		return fmt.Errorf("session %s has no worktree path", sessionID)
	}

	project, err := m.store.GetProject(ctx, session.ProjectID)
	if err != nil {
		return fmt.Errorf("get project: %w", err)
	}

	gitClient := newRepoBoundClient(project.Path)

	logger := &nopLogger{}
	deleteOpts := ops.DeleteOptions{
		Force:        force,
		DeleteBranch: false,
		DryRun:       false,
	}

	err = ops.Delete(ctx, gitClient, nil, logger, session.WorktreePath, deleteOpts, nil, nil)
	if err != nil {
		return fmt.Errorf("delete worktree: %w", err)
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

// DiscoverWorktrees scans a project's git repo for worktrees not tracked by pm.
// Returns newly created session records for discovered worktrees.
func (m *Manager) DiscoverWorktrees(ctx context.Context, projectID string) ([]*models.AgentSession, error) {
	project, err := m.store.GetProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get project: %w", err)
	}

	gitClient := newRepoBoundClient(project.Path)

	worktrees, err := gitClient.WorktreeList()
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	repoRoot, err := gitClient.RepoRoot()
	if err != nil {
		return nil, fmt.Errorf("get repo root: %w", err)
	}

	// Collect worktree paths (excluding main repo)
	var wtPaths []string
	pathToWT := make(map[string]struct{ branch string })
	for _, wt := range worktrees {
		if wt.Path == repoRoot {
			continue
		}
		wtPaths = append(wtPaths, wt.Path)
		pathToWT[wt.Path] = struct{ branch string }{branch: wt.Branch}
	}

	if len(wtPaths) == 0 {
		return nil, nil
	}

	// Find which paths already have sessions
	existingSessions, err := m.store.ListAgentSessionsByWorktreePaths(ctx, wtPaths)
	if err != nil {
		return nil, fmt.Errorf("list sessions by paths: %w", err)
	}

	knownPaths := make(map[string]bool)
	for _, s := range existingSessions {
		knownPaths[s.WorktreePath] = true
	}

	// Create sessions for untracked worktrees
	var discovered []*models.AgentSession
	now := time.Now().UTC()
	for _, path := range wtPaths {
		if knownPaths[path] {
			continue
		}
		info := pathToWT[path]
		session := &models.AgentSession{
			ProjectID:     projectID,
			Branch:        info.branch,
			WorktreePath:  path,
			Status:        models.SessionStatusIdle,
			ConflictState: models.ConflictStateNone,
			ConflictFiles: "[]",
			Discovered:    true,
			StartedAt:     now,
		}
		if err := m.store.CreateAgentSession(ctx, session); err != nil {
			continue
		}
		discovered = append(discovered, session)
	}

	return discovered, nil
}

// Reconcile runs enhanced reconciliation: existing reconcile logic + discovery + conflict checks.
func (m *Manager) Reconcile(ctx context.Context) (int, error) {
	projects, err := m.store.ListProjects(ctx, "")
	if err != nil {
		return 0, fmt.Errorf("list projects: %w", err)
	}

	totalUpdated := 0

	for _, project := range projects {
		// Discover untracked worktrees
		discovered, err := m.DiscoverWorktrees(ctx, project.ID)
		if err == nil {
			totalUpdated += len(discovered)
		}

		// Reconcile existing sessions
		sessions, err := m.store.ListAgentSessions(ctx, project.ID, 0)
		if err != nil {
			continue
		}

		for _, sess := range sessions {
			if sess.Status == models.SessionStatusCompleted {
				continue
			}
			if sess.WorktreePath == "" {
				continue
			}

			wtExists := true
			if _, err := os.Stat(sess.WorktreePath); err != nil {
				wtExists = false
			}

			updated := false
			switch {
			case !wtExists && (sess.Status == models.SessionStatusActive || sess.Status == models.SessionStatusIdle):
				now := time.Now().UTC()
				sess.Status = models.SessionStatusAbandoned
				sess.EndedAt = &now
				updated = true
			case wtExists && sess.Status == models.SessionStatusAbandoned:
				now := time.Now().UTC()
				sess.LastActiveAt = &now
				sess.Status = models.SessionStatusIdle
				sess.EndedAt = nil
				updated = true
			}

			if updated {
				if err := m.store.UpdateAgentSession(ctx, sess); err == nil {
					totalUpdated++
				}
			}
		}
	}

	return totalUpdated, nil
}

// nopLogger discards all log output.
type nopLogger struct{}

func (l *nopLogger) Info(format string, args ...interface{})    {}
func (l *nopLogger) Success(format string, args ...interface{}) {}
func (l *nopLogger) Warning(format string, args ...interface{}) {}
func (l *nopLogger) Error(format string, args ...interface{})   {}
func (l *nopLogger) Verbose(format string, args ...interface{}) {}
