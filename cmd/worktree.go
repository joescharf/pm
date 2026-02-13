package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/joescharf/pm/internal/output"
	"github.com/joescharf/pm/internal/wt"
)

var worktreeCmd = &cobra.Command{
	Use:     "worktree",
	Aliases: []string{"wt"},
	Short:   "Manage project worktrees",
	Long:    "List and create git worktrees for tracked projects.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return worktreeListRun("")
	},
}

var worktreeListCmd = &cobra.Command{
	Use:     "list [project]",
	Aliases: []string{"ls"},
	Short:   "List worktrees for a project",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectRef string
		if len(args) > 0 {
			projectRef = args[0]
		}
		return worktreeListRun(projectRef)
	},
}

var worktreeCreateCmd = &cobra.Command{
	Use:   "create <project> <branch>",
	Short: "Create a worktree for a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return worktreeCreateRun(args[0], args[1])
	},
}

func init() {
	worktreeCmd.AddCommand(worktreeListCmd)
	worktreeCmd.AddCommand(worktreeCreateCmd)
	rootCmd.AddCommand(worktreeCmd)
}

func worktreeListRun(projectRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	// If project specified, list that one
	if projectRef != "" {
		p, err := resolveProject(ctx, s, projectRef)
		if err != nil {
			return err
		}
		return listWorktreesForPath(p.Name, p.Path)
	}

	// Otherwise list all projects' worktrees
	projects, err := s.ListProjects(ctx, "")
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		ui.Info("No projects tracked.")
		return nil
	}

	wtClient := wt.NewClient()
	table := ui.Table([]string{"Project", "Branch", "Path"})
	count := 0

	for _, p := range projects {
		wts, err := wtClient.List(p.Path)
		if err != nil {
			continue
		}
		for _, w := range wts {
			_ = table.Append([]string{
				output.Cyan(p.Name),
				w.Branch,
				w.Path,
			})
			count++
		}
	}

	if count == 0 {
		ui.Info("No worktrees found.")
		return nil
	}

	_ = table.Render()
	return nil
}

func listWorktreesForPath(name, path string) error {
	wtClient := wt.NewClient()
	wts, err := wtClient.List(path)
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}

	if len(wts) == 0 {
		ui.Info("No worktrees for %s", name)
		return nil
	}

	table := ui.Table([]string{"Branch", "Path"})
	for _, w := range wts {
		_ = table.Append([]string{w.Branch, w.Path})
	}
	_ = table.Render()
	return nil
}

func worktreeCreateRun(projectRef, branch string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	p, err := resolveProject(ctx, s, projectRef)
	if err != nil {
		return err
	}

	if dryRun {
		ui.DryRunMsg("Would create worktree %s for %s", branch, p.Name)
		return nil
	}

	wtClient := wt.NewClient()
	ui.Info("Creating worktree %s for %s...", output.Cyan(branch), output.Cyan(p.Name))
	if err := wtClient.Create(p.Path, branch); err != nil {
		return fmt.Errorf("create worktree: %w", err)
	}

	ui.Success("Created worktree %s", output.Cyan(branch))
	return nil
}
