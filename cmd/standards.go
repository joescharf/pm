package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/joescharf/pm/internal/output"
	"github.com/joescharf/pm/internal/standards"
)

var standardsCmd = &cobra.Command{
	Use:   "standards [project]",
	Short: "Check project standardization",
	Long:  "Check if a project follows standard conventions (Makefile, CLAUDE.md, tests, etc.).",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectRef string
		if len(args) > 0 {
			projectRef = args[0]
		}
		return standardsCheckRun(projectRef)
	},
}

func init() {
	rootCmd.AddCommand(standardsCmd)
}

func standardsCheckRun(projectRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	var projects []*struct{ name, path string }

	if projectRef != "" {
		p, err := resolveProject(ctx, s, projectRef)
		if err != nil {
			return err
		}
		projects = append(projects, &struct{ name, path string }{p.Name, p.Path})
	} else {
		// Check all projects
		all, err := s.ListProjects(ctx, "")
		if err != nil {
			return err
		}
		for _, p := range all {
			projects = append(projects, &struct{ name, path string }{p.Name, p.Path})
		}
	}

	if len(projects) == 0 {
		ui.Info("No projects to check.")
		return nil
	}

	checker := standards.NewChecker()

	for _, proj := range projects {
		fmt.Fprintf(ui.Out, "\n%s\n", output.Cyan(proj.name))
		checks := checker.Run(proj.path)

		passed, total := 0, len(checks)
		for _, c := range checks {
			icon := output.Red("\u2717")
			if c.Passed {
				icon = output.Green("\u2713")
				passed++
			}
			fmt.Fprintf(ui.Out, "  %s %-20s %s\n", icon, c.Name, c.Detail)
		}
		fmt.Fprintf(ui.Out, "  Score: %d/%d\n", passed, total)
	}

	return nil
}
