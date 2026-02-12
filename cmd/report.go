package cmd

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joescharf/pm/internal/store"
)

var (
	reportFormat string
	exportType   string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export data as JSON, CSV, or Markdown",
	Long:  "Export projects, issues, or sessions in various formats.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return exportRun()
	},
}

func init() {
	exportCmd.Flags().StringVar(&reportFormat, "format", "json", "Output format: json, csv, markdown")
	exportCmd.Flags().StringVar(&exportType, "type", "projects", "Data type: projects, issues, sessions")
	rootCmd.AddCommand(exportCmd)
}

func exportRun() error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	switch exportType {
	case "projects":
		return exportProjects(ctx, s)
	case "issues":
		return exportIssues(ctx, s)
	case "sessions":
		return exportSessions(ctx, s)
	default:
		return fmt.Errorf("unknown export type: %s (use: projects, issues, sessions)", exportType)
	}
}

func exportProjects(ctx context.Context, s store.Store) error {
	projects, err := s.ListProjects(ctx, "")
	if err != nil {
		return err
	}

	switch reportFormat {
	case "json":
		enc := json.NewEncoder(ui.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(projects)
	case "csv":
		w := csv.NewWriter(ui.Out)
		w.Write([]string{"ID", "Name", "Path", "Language", "Group", "Created"})
		for _, p := range projects {
			w.Write([]string{p.ID, p.Name, p.Path, p.Language, p.GroupName, p.CreatedAt.Format("2006-01-02")})
		}
		w.Flush()
		return w.Error()
	case "markdown":
		fmt.Fprintln(ui.Out, "# Projects")
		fmt.Fprintln(ui.Out)
		fmt.Fprintln(ui.Out, "| Name | Path | Language | Group |")
		fmt.Fprintln(ui.Out, "|------|------|----------|-------|")
		for _, p := range projects {
			fmt.Fprintf(ui.Out, "| %s | %s | %s | %s |\n", p.Name, p.Path, p.Language, p.GroupName)
		}
		return nil
	default:
		return fmt.Errorf("unknown format: %s", reportFormat)
	}
}

func exportIssues(ctx context.Context, s store.Store) error {
	issues, err := s.ListIssues(ctx, store.IssueListFilter{})
	if err != nil {
		return err
	}

	switch reportFormat {
	case "json":
		enc := json.NewEncoder(ui.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(issues)
	case "csv":
		w := csv.NewWriter(ui.Out)
		w.Write([]string{"ID", "ProjectID", "Title", "Status", "Priority", "Type", "GitHub#", "Created"})
		for _, i := range issues {
			gh := ""
			if i.GitHubIssue > 0 {
				gh = fmt.Sprintf("%d", i.GitHubIssue)
			}
			w.Write([]string{i.ID, i.ProjectID, i.Title, string(i.Status), string(i.Priority), string(i.Type), gh, i.CreatedAt.Format("2006-01-02")})
		}
		w.Flush()
		return w.Error()
	case "markdown":
		fmt.Fprintln(ui.Out, "# Issues")
		fmt.Fprintln(ui.Out)
		fmt.Fprintln(ui.Out, "| Title | Status | Priority | Type |")
		fmt.Fprintln(ui.Out, "|-------|--------|----------|------|")
		for _, i := range issues {
			fmt.Fprintf(ui.Out, "| %s | %s | %s | %s |\n", i.Title, i.Status, i.Priority, i.Type)
		}
		return nil
	default:
		return fmt.Errorf("unknown format: %s", reportFormat)
	}
}

func exportSessions(ctx context.Context, s store.Store) error {
	sessions, err := s.ListAgentSessions(ctx, "", 0)
	if err != nil {
		return err
	}

	switch reportFormat {
	case "json":
		enc := json.NewEncoder(ui.Out)
		enc.SetIndent("", "  ")
		return enc.Encode(sessions)
	case "csv":
		w := csv.NewWriter(ui.Out)
		w.Write([]string{"ID", "ProjectID", "Branch", "Status", "Commits", "Started"})
		for _, sess := range sessions {
			w.Write([]string{sess.ID, sess.ProjectID, sess.Branch, string(sess.Status),
				fmt.Sprintf("%d", sess.CommitCount), sess.StartedAt.Format("2006-01-02T15:04:05Z")})
		}
		w.Flush()
		return w.Error()
	case "markdown":
		fmt.Fprintln(ui.Out, "# Agent Sessions")
		fmt.Fprintln(ui.Out)
		fmt.Fprintln(ui.Out, "| Branch | Status | Commits |")
		fmt.Fprintln(ui.Out, "|--------|--------|---------|")
		for _, sess := range sessions {
			fmt.Fprintf(ui.Out, "| %s | %s | %d |\n", sess.Branch, sess.Status, sess.CommitCount)
		}
		return nil
	default:
		return fmt.Errorf("unknown format: %s", reportFormat)
	}
}

// reportWeeklyRun generates a summary for the past week. Added as convenience.
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate reports",
	Long:  "Generate summary reports of project activity.",
}

var reportWeeklyCmd = &cobra.Command{
	Use:   "weekly",
	Short: "Generate weekly activity summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		return reportWeeklyRun()
	},
}

func init() {
	reportCmd.AddCommand(reportWeeklyCmd)
	rootCmd.AddCommand(reportCmd)
}

func reportWeeklyRun() error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	projects, err := s.ListProjects(ctx, "")
	if err != nil {
		return err
	}

	fmt.Fprintln(ui.Out, "# Weekly Report")
	fmt.Fprintln(ui.Out)

	for _, p := range projects {
		issues, _ := s.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID})
		sessions, _ := s.ListAgentSessions(ctx, p.ID, 0)

		open, closed, inProg := 0, 0, 0
		for _, i := range issues {
			switch i.Status {
			case "open":
				open++
			case "closed", "done":
				closed++
			case "in_progress":
				inProg++
			}
		}

		var sessionBranches []string
		for _, sess := range sessions {
			sessionBranches = append(sessionBranches, sess.Branch)
		}

		fmt.Fprintf(ui.Out, "## %s\n", p.Name)
		fmt.Fprintf(ui.Out, "- Issues: %d open, %d in-progress, %d closed\n", open, inProg, closed)
		if len(sessionBranches) > 0 {
			fmt.Fprintf(ui.Out, "- Agent sessions: %s\n", strings.Join(sessionBranches, ", "))
		}
		fmt.Fprintln(ui.Out)
	}

	return nil
}
