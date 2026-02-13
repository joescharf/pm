package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/output"
	"github.com/joescharf/pm/internal/wt"
)

var (
	agentIssue  string
	agentBranch string
	agentLimit  int
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

func init() {
	agentLaunchCmd.Flags().StringVar(&agentIssue, "issue", "", "Issue ID to work on")
	agentLaunchCmd.Flags().StringVar(&agentBranch, "branch", "", "Branch name (auto-generated from issue if not specified)")

	agentHistoryCmd.Flags().IntVar(&agentLimit, "limit", 20, "Max sessions to show")

	agentCmd.AddCommand(agentLaunchCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentHistoryCmd)
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
	if branch == "" && agentIssue != "" {
		issue, err := findIssue(ctx, s, agentIssue)
		if err != nil {
			return fmt.Errorf("find issue: %w", err)
		}
		branch = issueToBranch(issue.Title)

		// Update issue status to in_progress
		issue.Status = models.IssueStatusInProgress
		_ = s.UpdateIssue(ctx, issue)
	}
	if branch == "" {
		return fmt.Errorf("specify --branch or --issue to generate a branch name")
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
		ProjectID: p.ID,
		IssueID:   agentIssue,
		Branch:    branch,
		Status:    models.SessionStatusRunning,
	}
	if err := s.CreateAgentSession(ctx, session); err != nil {
		ui.Warning("Session recording failed: %v", err)
	}

	ui.Success("Agent launched for %s on branch %s", output.Cyan(p.Name), output.Cyan(branch))
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

	// Filter to running only
	var active []*models.AgentSession
	for _, sess := range sessions {
		if sess.Status == models.SessionStatusRunning {
			active = append(active, sess)
		}
	}

	if len(active) == 0 {
		ui.Info("No active agent sessions.")
		return nil
	}

	projectNames := make(map[string]string)
	table := ui.Table([]string{"ID", "Project", "Branch", "Started"})
	for _, sess := range active {
		projName := projectNames[sess.ProjectID]
		if projName == "" {
			if p, err := s.GetProject(ctx, sess.ProjectID); err == nil {
				projName = p.Name
				projectNames[sess.ProjectID] = projName
			}
		}

		table.Append([]string{
			shortID(sess.ID),
			projName,
			sess.Branch,
			timeAgo(sess.StartedAt),
		})
	}
	table.Render()
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
	table := ui.Table([]string{"ID", "Project", "Branch", "Status", "Commits", "Duration"})
	for _, sess := range sessions {
		projName := projectNames[sess.ProjectID]
		if projName == "" {
			if p, err := s.GetProject(ctx, sess.ProjectID); err == nil {
				projName = p.Name
				projectNames[sess.ProjectID] = projName
			}
		}

		duration := "running"
		if sess.EndedAt != nil {
			d := sess.EndedAt.Sub(sess.StartedAt)
			duration = formatDuration(d)
		}

		table.Append([]string{
			shortID(sess.ID),
			projName,
			sess.Branch,
			output.StatusColor(string(sess.Status)),
			fmt.Sprintf("%d", sess.CommitCount),
			duration,
		})
	}
	table.Render()
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
