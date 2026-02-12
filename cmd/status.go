package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/health"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/output"
	"github.com/joescharf/pm/internal/store"
)

var (
	statusStale bool
	statusGroup string
)

var statusCmd = &cobra.Command{
	Use:   "status [project]",
	Short: "Show project status dashboard",
	Long: `Show a cross-project status overview or detailed status for one project.

Without arguments, shows a summary table of all tracked projects.
With a project name, shows detailed status for that project.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 1 {
			return projectShowRun(args[0]) // reuse project show for detail
		}
		return statusOverviewRun()
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusStale, "stale", false, "Show only stale projects (no activity in 7+ days)")
	statusCmd.Flags().StringVar(&statusGroup, "group", "", "Filter by project group")
	rootCmd.AddCommand(statusCmd)
}

func statusOverviewRun() error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	projects, err := s.ListProjects(ctx, statusGroup)
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		ui.Info("No projects tracked. Use 'pm project add <path>' to get started.")
		return nil
	}

	gc := git.NewClient()
	scorer := health.NewScorer()

	table := ui.Table([]string{"Project", "Branch", "Status", "Issues", "Health", "Activity"})

	for _, p := range projects {
		meta := gatherMetadata(gc, p)

		// Get issues
		issues, _ := s.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID})

		// Skip non-stale if --stale flag
		if statusStale && !meta.LastCommitDate.IsZero() {
			if time.Since(meta.LastCommitDate) < 7*24*time.Hour {
				continue
			}
		}

		// Compute health
		h := scorer.Score(p, meta, issues)

		// Format fields
		branch := getBranch(gc, p.Path)
		gitStatus := getGitStatus(meta)
		issueStr := formatIssueCounts(issues)
		healthStr := output.HealthColor(h.Total)
		activity := "n/a"
		if !meta.LastCommitDate.IsZero() {
			activity = timeAgo(meta.LastCommitDate)
		}

		table.Append([]string{
			output.Cyan(p.Name),
			branch,
			gitStatus,
			issueStr,
			healthStr,
			activity,
		})
	}

	table.Render()
	return nil
}

func gatherMetadata(gc git.Client, p *models.Project) *health.ProjectMetadata {
	meta := &health.ProjectMetadata{}

	if dirty, err := gc.IsDirty(p.Path); err == nil {
		meta.IsDirty = dirty
	}
	if date, err := gc.LastCommitDate(p.Path); err == nil {
		meta.LastCommitDate = date
	}
	if branches, err := gc.BranchList(p.Path); err == nil {
		meta.BranchCount = len(branches)
	}
	if wts, err := gc.WorktreeList(p.Path); err == nil {
		meta.WorktreeCount = len(wts)
	}

	return meta
}

func getBranch(gc git.Client, path string) string {
	branch, err := gc.CurrentBranch(path)
	if err != nil {
		return "?"
	}
	return branch
}

func getGitStatus(meta *health.ProjectMetadata) string {
	if meta.IsDirty {
		return output.Red("dirty")
	}
	return output.Green("clean")
}

func formatIssueCounts(issues []*models.Issue) string {
	if len(issues) == 0 {
		return "-"
	}
	open, inProg := 0, 0
	for _, i := range issues {
		switch i.Status {
		case models.IssueStatusOpen:
			open++
		case models.IssueStatusInProgress:
			inProg++
		}
	}
	return fmt.Sprintf("%d/%d", open, inProg)
}
