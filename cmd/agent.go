package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joescharf/pm/internal/agent"
	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/output"
	"github.com/joescharf/pm/internal/sessions"
	"github.com/joescharf/pm/internal/store"
	"github.com/joescharf/pm/internal/wt"
)

var (
	agentIssue   string
	agentBranch  string
	agentLimit   int
	closeDone    bool
	closeAbandon bool
	syncRebase   bool
	syncForce    bool
	mergeRebase    bool
	mergeForce     bool
	mergeNoCleanup bool
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage Claude Code agent sessions",
	Long:  "Launch Claude agents via worktrees and track their sessions.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return agentListRun("")
	},
}

var agentLaunchCmd = &cobra.Command{
	Use:   "launch <project>",
	Short: "Launch a Claude agent in a new worktree",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return agentLaunchRun(args[0])
	},
}

var agentListCmd = &cobra.Command{
	Use:     "list [project]",
	Aliases: []string{"ls"},
	Short:   "List active agent sessions",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectRef string
		if len(args) > 0 {
			projectRef = args[0]
		}
		return agentListRun(projectRef)
	},
}

var agentHistoryCmd = &cobra.Command{
	Use:   "history [project]",
	Short: "Show agent session history",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectRef string
		if len(args) > 0 {
			projectRef = args[0]
		}
		return agentHistoryRun(projectRef)
	},
}

var agentCloseCmd = &cobra.Command{
	Use:   "close [session_id]",
	Short: "Close an agent session",
	Long: `Close an agent session. Default transitions to idle (worktree preserved).
Use --done to mark completed (issues → done) or --abandon to abandon (issues → open).

When no session_id is given:
  - In a worktree directory: closes the session for that worktree
  - In a project directory: lists active/idle sessions to choose from`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var sessionRef string
		if len(args) > 0 {
			sessionRef = args[0]
		}
		return agentCloseRun(sessionRef)
	},
}

var agentSyncCmd = &cobra.Command{
	Use:   "sync [session_id]",
	Short: "Sync a session's worktree with the base branch",
	Long:  "Fetches latest changes and merges/rebases the base branch into the feature branch.\nAuto-detects session from cwd if no session_id is given.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var sessionRef string
		if len(args) > 0 {
			sessionRef = args[0]
		}
		return agentSyncRun(sessionRef)
	},
}

var agentMergeCmd = &cobra.Command{
	Use:   "merge [session_id]",
	Short: "Merge a session's branch into the base branch",
	Long:  "Merges or rebases the feature branch into the base branch (default: main).\nAuto-detects session from cwd if no session_id is given.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var sessionRef string
		if len(args) > 0 {
			sessionRef = args[0]
		}
		return agentMergeRun(sessionRef)
	},
}

var agentDiscoverCmd = &cobra.Command{
	Use:   "discover [project]",
	Short: "Discover worktrees not tracked by pm",
	Long:  "Scans a project's git repo for worktrees and creates idle session records for untracked ones.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectRef string
		if len(args) > 0 {
			projectRef = args[0]
		}
		return agentDiscoverRun(projectRef)
	},
}

func init() {
	agentLaunchCmd.Flags().StringVar(&agentIssue, "issue", "", "Issue ID to work on")
	agentLaunchCmd.Flags().StringVar(&agentBranch, "branch", "", "Branch name (auto-generated from issue if not specified)")

	agentHistoryCmd.Flags().IntVar(&agentLimit, "limit", 20, "Max sessions to show")

	agentCloseCmd.Flags().BoolVar(&closeDone, "done", false, "Mark session as completed (issues → done)")
	agentCloseCmd.Flags().BoolVar(&closeAbandon, "abandon", false, "Mark session as abandoned (issues → open)")

	agentSyncCmd.Flags().BoolVar(&syncRebase, "rebase", false, "Use rebase instead of merge")
	agentSyncCmd.Flags().BoolVar(&syncForce, "force", false, "Skip dirty worktree check")

	agentMergeCmd.Flags().BoolVar(&mergeRebase, "rebase", false, "Use rebase instead of merge")
	agentMergeCmd.Flags().BoolVar(&mergeForce, "force", false, "Skip dirty worktree check")
	agentMergeCmd.Flags().BoolVar(&mergeNoCleanup, "no-cleanup", false, "Skip post-merge cleanup (worktree removal, branch deletion, iTerm close)")

	agentCmd.AddCommand(agentLaunchCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentHistoryCmd)
	agentCmd.AddCommand(agentCloseCmd)
	agentCmd.AddCommand(agentSyncCmd)
	agentCmd.AddCommand(agentMergeCmd)
	agentCmd.AddCommand(agentDiscoverCmd)
	rootCmd.AddCommand(agentCmd)
}

func agentLaunchRun(projectRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	p, err := resolveProject(ctx, s, projectRef)
	if err != nil {
		return err
	}

	// Determine branch name
	branch := agentBranch
	resolvedIssueID := agentIssue
	if branch == "" && agentIssue != "" {
		issue, err := findIssue(ctx, s, agentIssue)
		if err != nil {
			return fmt.Errorf("find issue: %w", err)
		}
		branch = issueToBranch(issue.Title)
		resolvedIssueID = issue.ID

		// Update issue status to in_progress
		issue.Status = models.IssueStatusInProgress
		_ = s.UpdateIssue(ctx, issue)
	}
	if branch == "" {
		return fmt.Errorf("specify --branch or --issue to generate a branch name")
	}

	// Compute worktree path to match wt's convention: {project}.worktrees/{last-branch-segment}
	branchParts := strings.Split(branch, "/")
	worktreeDirname := branchParts[len(branchParts)-1]
	worktreePath := filepath.Join(p.Path+".worktrees", worktreeDirname)

	// Check for existing idle session on this branch
	existingSessions, _ := s.ListAgentSessions(ctx, p.ID, 0)
	for _, sess := range existingSessions {
		if sess.Branch == branch && sess.Status == models.SessionStatusIdle {
			if dryRun {
				ui.DryRunMsg("Would resume session %s for %s on branch %s", shortID(sess.ID), p.Name, branch)
				return nil
			}
			// Resume: reactivate existing session, open iTerm window
			wtClient := wt.NewClient()
			ui.Info("Opening worktree for branch: %s", output.Cyan(branch))
			if err := wtClient.Create(p.Path, branch); err != nil {
				return fmt.Errorf("wt open: %w", err)
			}
			sess.Status = models.SessionStatusActive
			now := time.Now().UTC()
			sess.LastActiveAt = &now
			if err := s.UpdateAgentSession(ctx, sess); err != nil {
				ui.Warning("Failed to reactivate session: %v", err)
			} else {
				resumePath := sess.WorktreePath
				ui.Success("Resumed session %s for %s on branch %s", output.Cyan(shortID(sess.ID)), output.Cyan(p.Name), output.Cyan(branch))
				if resolvedIssueID != "" {
					ui.Info("Run: cd %s && claude \"Use pm MCP tools to look up issue %s and implement it. Update the issue status when complete.\"", resumePath, shortID(resolvedIssueID))
				} else {
					ui.Info("Run: cd %s && claude", resumePath)
				}
				return nil
			}
		}
	}

	if dryRun {
		ui.DryRunMsg("Would launch agent for %s on branch %s", p.Name, branch)
		return nil
	}

	// Create worktree via wt CLI
	wtClient := wt.NewClient()
	ui.Info("Creating worktree for branch: %s", output.Cyan(branch))
	if err := wtClient.Create(p.Path, branch); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	// Record session
	session := &models.AgentSession{
		ProjectID:    p.ID,
		IssueID:      resolvedIssueID,
		Branch:       branch,
		WorktreePath: worktreePath,
		Status:       models.SessionStatusActive,
	}
	if err := s.CreateAgentSession(ctx, session); err != nil {
		ui.Warning("Session recording failed: %v", err)
	}

	ui.Success("Agent launched for %s on branch %s", output.Cyan(p.Name), output.Cyan(branch))

	// Show the command to run
	if resolvedIssueID != "" {
		shortIssueID := shortID(resolvedIssueID)
		ui.Info("Run: cd %s && claude \"Use pm MCP tools to look up issue %s and implement it. Update the issue status when complete.\"", worktreePath, shortIssueID)
	} else {
		ui.Info("Run: cd %s && claude", worktreePath)
	}
	return nil
}

func agentListRun(projectRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	var projectID string
	if projectRef != "" {
		p, err := resolveProject(ctx, s, projectRef)
		if err != nil {
			return err
		}
		projectID = p.ID
	}

	sessions, err := s.ListAgentSessions(ctx, projectID, 0)
	if err != nil {
		return err
	}

	// Reconcile orphaned worktrees
	agent.ReconcileSessions(ctx, s, sessions)

	// Filter to active/idle
	var live []*models.AgentSession
	for _, sess := range sessions {
		if sess.Status == models.SessionStatusActive || sess.Status == models.SessionStatusIdle {
			live = append(live, sess)
		}
	}

	if len(live) == 0 {
		ui.Info("No active or idle agent sessions.")
		return nil
	}

	projectNames := make(map[string]string)
	table := ui.Table([]string{"ID", "Project", "Branch", "Status", "Worktree", "Last Active", "Started"})
	for _, sess := range live {
		projName := projectNames[sess.ProjectID]
		if projName == "" {
			if p, err := s.GetProject(ctx, sess.ProjectID); err == nil {
				projName = p.Name
				projectNames[sess.ProjectID] = projName
			}
		}

		lastActive := "—"
		if sess.LastActiveAt != nil {
			lastActive = timeAgo(*sess.LastActiveAt)
		}

		_ = table.Append([]string{
			shortID(sess.ID),
			projName,
			sess.Branch,
			output.StatusColor(string(sess.Status)),
			sess.WorktreePath,
			lastActive,
			timeAgo(sess.StartedAt),
		})
	}
	_ = table.Render()
	return nil
}

func agentHistoryRun(projectRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	var projectID string
	if projectRef != "" {
		p, err := resolveProject(ctx, s, projectRef)
		if err != nil {
			return err
		}
		projectID = p.ID
	}

	sessions, err := s.ListAgentSessions(ctx, projectID, agentLimit)
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		ui.Info("No agent session history.")
		return nil
	}

	projectNames := make(map[string]string)
	table := ui.Table([]string{"ID", "Project", "Branch", "Status", "Commits", "Last Commit", "Duration"})
	for _, sess := range sessions {
		projName := projectNames[sess.ProjectID]
		if projName == "" {
			if p, err := s.GetProject(ctx, sess.ProjectID); err == nil {
				projName = p.Name
				projectNames[sess.ProjectID] = projName
			}
		}

		duration := "active"
		if sess.EndedAt != nil {
			d := sess.EndedAt.Sub(sess.StartedAt)
			duration = formatDuration(d)
		}

		lastCommit := "—"
		if sess.LastCommitHash != "" {
			msg := sess.LastCommitMessage
			if len(msg) > 40 {
				msg = msg[:40] + "..."
			}
			lastCommit = fmt.Sprintf("%s %s", sess.LastCommitHash, msg)
		}

		_ = table.Append([]string{
			shortID(sess.ID),
			projName,
			sess.Branch,
			output.StatusColor(string(sess.Status)),
			fmt.Sprintf("%d", sess.CommitCount),
			lastCommit,
			duration,
		})
	}
	_ = table.Render()
	return nil
}

func agentCloseRun(sessionRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	// Determine target status
	target := models.SessionStatusIdle
	if closeDone {
		target = models.SessionStatusCompleted
	} else if closeAbandon {
		target = models.SessionStatusAbandoned
	}

	// Resolve session ID
	sessionID := sessionRef
	if sessionID == "" {
		sessionID, err = resolveSessionFromCwd(ctx, s)
		if err != nil {
			return err
		}
	}

	// Enrich session with git info before closing
	gc := git.NewClient()
	if sess, err := s.GetAgentSession(ctx, sessionID); err == nil {
		agent.EnrichSessionWithGitInfo(sess, gc)
		_ = s.UpdateAgentSession(ctx, sess)
	}

	// Get worktree path before closing (for iTerm cleanup)
	var worktreePath string
	if sess, lookupErr := s.GetAgentSession(ctx, sessionID); lookupErr == nil {
		worktreePath = sess.WorktreePath
	}

	session, err := agent.CloseSession(ctx, s, sessionID, target)
	if err != nil {
		return err
	}

	// Close iTerm window for terminal states (completed/abandoned)
	if worktreePath != "" && target != models.SessionStatusIdle {
		_ = sessions.CloseITermWindow(worktreePath)
	}

	ui.Success("Session %s → %s", output.Cyan(shortID(session.ID)), output.Cyan(string(session.Status)))
	return nil
}

func resolveSessionFromCwd(ctx context.Context, s store.Store) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}

	// Try matching cwd as a worktree path
	session, err := s.GetAgentSessionByWorktreePath(ctx, cwd)
	if err == nil {
		return session.ID, nil
	}

	// Try matching cwd as a project directory
	p, err := s.GetProjectByPath(ctx, cwd)
	if err != nil {
		return "", fmt.Errorf("no session found for current directory; specify a session ID")
	}

	// List active/idle sessions for this project
	sessions, err := s.ListAgentSessions(ctx, p.ID, 0)
	if err != nil {
		return "", err
	}

	var live []*models.AgentSession
	for _, sess := range sessions {
		if sess.Status == models.SessionStatusActive || sess.Status == models.SessionStatusIdle {
			live = append(live, sess)
		}
	}

	if len(live) == 0 {
		return "", fmt.Errorf("no active/idle sessions for project %s", p.Name)
	}
	if len(live) == 1 {
		return live[0].ID, nil
	}

	// Multiple sessions — list them for the user
	fmt.Println("Multiple active sessions. Specify a session ID:")
	table := ui.Table([]string{"ID", "Branch", "Status", "Started"})
	for _, sess := range live {
		_ = table.Append([]string{
			shortID(sess.ID),
			sess.Branch,
			string(sess.Status),
			timeAgo(sess.StartedAt),
		})
	}
	_ = table.Render()
	return "", fmt.Errorf("ambiguous: multiple sessions found")
}

func agentSyncRun(sessionRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	sessionID := sessionRef
	if sessionID == "" {
		sessionID, err = resolveSessionFromCwd(ctx, s)
		if err != nil {
			return err
		}
	}

	mgr := sessions.NewManager(s)
	opts := sessions.SyncOptions{
		Rebase: syncRebase,
		Force:  syncForce,
		DryRun: dryRun,
	}

	result, err := mgr.SyncSession(ctx, sessionID, opts)
	if err != nil {
		return err
	}

	if result.Synced {
		ui.Success("Already in sync (↑%d)", result.Ahead)
	} else if result.Success {
		strategy := "merged"
		if syncRebase {
			strategy = "rebased"
		}
		ui.Success("Synced (%s) — ↑%d ↓%d", strategy, result.Ahead, result.Behind)
	} else if len(result.Conflicts) > 0 {
		ui.Error("Sync conflicts detected:")
		for _, f := range result.Conflicts {
			ui.Info("  %s", f)
		}
		return fmt.Errorf("resolve conflicts, then sync again")
	} else if result.Error != "" {
		return fmt.Errorf("sync: %s", result.Error)
	}

	return nil
}

func agentMergeRun(sessionRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	sessionID := sessionRef
	if sessionID == "" {
		sessionID, err = resolveSessionFromCwd(ctx, s)
		if err != nil {
			return err
		}
	}

	mgr := sessions.NewManager(s)
	opts := sessions.MergeOptions{
		Rebase:  mergeRebase,
		Force:   mergeForce,
		DryRun:  dryRun,
		Cleanup: !mergeNoCleanup,
	}

	result, err := mgr.MergeSession(ctx, sessionID, opts)
	if err != nil {
		return err
	}

	if result.Success {
		if result.PRCreated {
			ui.Success("PR created: %s", result.PRURL)
		} else {
			strategy := "Merged"
			if mergeRebase {
				strategy = "Rebased"
			}
			ui.Success("%s '%s' into base branch", strategy, result.Branch)
			if result.Cleaned {
				ui.Success("Cleaned up worktree and branch")
			}
		}
	} else if len(result.Conflicts) > 0 {
		ui.Error("Merge conflicts detected:")
		for _, f := range result.Conflicts {
			ui.Info("  %s", f)
		}
		return fmt.Errorf("resolve conflicts, then merge again")
	} else if result.Error != "" {
		return fmt.Errorf("merge: %s", result.Error)
	}

	return nil
}

func agentDiscoverRun(projectRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	if projectRef == "" {
		// Try auto-detect from cwd
		cwd, cwdErr := os.Getwd()
		if cwdErr == nil {
			if p, pErr := s.GetProjectByPath(ctx, cwd); pErr == nil {
				projectRef = p.Name
			}
		}
		if projectRef == "" {
			return fmt.Errorf("specify a project name or run from a project directory")
		}
	}

	p, err := resolveProject(ctx, s, projectRef)
	if err != nil {
		return err
	}

	mgr := sessions.NewManager(s)
	discovered, err := mgr.DiscoverWorktrees(ctx, p.ID)
	if err != nil {
		return err
	}

	if len(discovered) == 0 {
		ui.Info("No untracked worktrees found for %s", output.Cyan(p.Name))
		return nil
	}

	ui.Success("Discovered %d worktree(s) for %s:", len(discovered), output.Cyan(p.Name))
	for _, sess := range discovered {
		ui.Info("  %s → %s", output.Cyan(sess.Branch), sess.WorktreePath)
	}
	return nil
}

// issueToBranch converts an issue title to a branch name.
func issueToBranch(title string) string {
	// Lowercase, replace spaces with hyphens, remove special chars
	s := strings.ToLower(title)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			return r
		}
		if r == ' ' {
			return '-'
		}
		return -1
	}, s)
	// Trim leading/trailing hyphens and collapse multiples
	parts := strings.Split(s, "-")
	var clean []string
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}
	result := strings.Join(clean, "-")
	if len(result) > 50 {
		result = result[:50]
	}
	return "feature/" + result
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}
