package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/output"
	"github.com/joescharf/pm/internal/store"
)

var (
	issueTitle    string
	issueDesc     string
	issueBody     string
	issuePriority string
	issueType     string
	issueStatus   string
	issueTag      string
	issueAll      bool
	issueGitHub   int
)

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Manage project issues and features",
	Long:  "Track issues, features, and bugs for your projects.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return issueListRun("")
	},
}

var issueAddCmd = &cobra.Command{
	Use:   "add [project]",
	Short: "Add a new issue",
	Long:  "Add a new issue to a project. Without <project>, auto-detects from cwd.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectRef string
		if len(args) > 0 {
			projectRef = args[0]
		}
		return issueAddRun(projectRef)
	},
}

var issueListCmd = &cobra.Command{
	Use:     "list [project]",
	Aliases: []string{"ls"},
	Short:   "List issues",
	Long:    "List issues. Without <project>, shows all or auto-detects from cwd.",
	Args:    cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var projectRef string
		if len(args) > 0 {
			projectRef = args[0]
		}
		return issueListRun(projectRef)
	},
}

var issueShowCmd = &cobra.Command{
	Use:   "show <issue-id>",
	Short: "Show issue details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return issueShowRun(args[0])
	},
}

var issueUpdateCmd = &cobra.Command{
	Use:   "update <issue-id>",
	Short: "Update an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return issueUpdateRun(args[0])
	},
}

var issueCloseCmd = &cobra.Command{
	Use:   "close <issue-id>",
	Short: "Close an issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return issueCloseRun(args[0])
	},
}

var issueLinkCmd = &cobra.Command{
	Use:   "link <issue-id>",
	Short: "Link to a GitHub issue",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return issueLinkRun(args[0])
	},
}

func init() {
	issueAddCmd.Flags().StringVar(&issueTitle, "title", "", "Issue title (required)")
	issueAddCmd.Flags().StringVar(&issueDesc, "desc", "", "Issue description")
	issueAddCmd.Flags().StringVar(&issueBody, "body", "", "Raw body text (e.g. original issue text)")
	issueAddCmd.Flags().StringVar(&issuePriority, "priority", "medium", "Priority: low, medium, high")
	issueAddCmd.Flags().StringVar(&issueType, "type", "feature", "Type: feature, bug, chore")
	issueAddCmd.Flags().StringVar(&issueTag, "tag", "", "Tag to apply")
	_ = issueAddCmd.MarkFlagRequired("title")

	issueListCmd.Flags().StringVar(&issueStatus, "status", "", "Filter by status: open, in_progress, done, closed")
	issueListCmd.Flags().StringVar(&issuePriority, "priority", "", "Filter by priority")
	issueListCmd.Flags().StringVar(&issueTag, "tag", "", "Filter by tag")
	issueListCmd.Flags().BoolVar(&issueAll, "all", false, "Show all issues across projects")

	issueUpdateCmd.Flags().StringVar(&issueStatus, "status", "", "New status")
	issueUpdateCmd.Flags().StringVar(&issuePriority, "priority", "", "New priority")
	issueUpdateCmd.Flags().StringVar(&issueTitle, "title", "", "New title")
	issueUpdateCmd.Flags().StringVar(&issueDesc, "desc", "", "New description")
	issueUpdateCmd.Flags().StringVar(&issueBody, "body", "", "New body text")

	issueLinkCmd.Flags().IntVar(&issueGitHub, "github", 0, "GitHub issue number")
	_ = issueLinkCmd.MarkFlagRequired("github")

	issueCmd.AddCommand(issueAddCmd)
	issueCmd.AddCommand(issueListCmd)
	issueCmd.AddCommand(issueShowCmd)
	issueCmd.AddCommand(issueUpdateCmd)
	issueCmd.AddCommand(issueCloseCmd)
	issueCmd.AddCommand(issueLinkCmd)
	rootCmd.AddCommand(issueCmd)
}

func issueAddRun(projectRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	p, err := resolveProjectOrCwd(ctx, s, projectRef)
	if err != nil {
		return err
	}

	issue := &models.Issue{
		ProjectID:   p.ID,
		Title:       issueTitle,
		Description: issueDesc,
		Body:        issueBody,
		Status:      models.IssueStatusOpen,
		Priority:    models.IssuePriority(issuePriority),
		Type:        models.IssueType(issueType),
	}

	if dryRun {
		ui.DryRunMsg("Would add issue: %s [%s/%s] to %s", issueTitle, issuePriority, issueType, p.Name)
		return nil
	}

	if err := s.CreateIssue(ctx, issue); err != nil {
		return fmt.Errorf("create issue: %w", err)
	}

	// Apply tag if specified
	if issueTag != "" {
		if err := applyTag(ctx, s, issue.ID, issueTag); err != nil {
			ui.Warning("Issue created but tag failed: %v", err)
		}
	}

	ui.Success("Created issue %s: %s", output.Cyan(shortID(issue.ID)), issueTitle)
	return nil
}

func issueListRun(projectRef string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	filter := store.IssueListFilter{
		Status:   models.IssueStatus(issueStatus),
		Priority: models.IssuePriority(issuePriority),
		Tag:      issueTag,
	}

	// Resolve project if specified, otherwise show all if --all or cwd
	if projectRef != "" {
		p, err := resolveProject(ctx, s, projectRef)
		if err != nil {
			return err
		}
		filter.ProjectID = p.ID
	} else if !issueAll {
		// Try to detect project from cwd
		if p, err := resolveProjectFromCwd(ctx, s); err == nil {
			filter.ProjectID = p.ID
		}
		// If cwd detection fails, show all issues
	}

	issues, err := s.ListIssues(ctx, filter)
	if err != nil {
		return err
	}

	if len(issues) == 0 {
		ui.Info("No issues found.")
		return nil
	}

	// Build a project name cache for display
	projectNames := make(map[string]string)

	table := ui.Table([]string{"ID", "Project", "Title", "Status", "Priority", "Type", "GH#"})
	for _, issue := range issues {
		projName := projectNames[issue.ProjectID]
		if projName == "" {
			if p, err := s.GetProject(ctx, issue.ProjectID); err == nil {
				projName = p.Name
				projectNames[issue.ProjectID] = projName
			}
		}

		ghStr := ""
		if issue.GitHubIssue > 0 {
			ghStr = fmt.Sprintf("#%d", issue.GitHubIssue)
		}

		_ = table.Append([]string{
			shortID(issue.ID),
			projName,
			issue.Title,
			output.StatusColor(string(issue.Status)),
			string(issue.Priority),
			string(issue.Type),
			ghStr,
		})
	}
	_ = table.Render()
	return nil
}

func issueShowRun(id string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	issue, err := findIssue(ctx, s, id)
	if err != nil {
		return err
	}

	projName := ""
	if p, err := s.GetProject(ctx, issue.ProjectID); err == nil {
		projName = p.Name
	}

	fmt.Fprintf(ui.Out, "%s  %s\n", output.Cyan(shortID(issue.ID)), issue.Title)
	fmt.Fprintf(ui.Out, "  Project:    %s\n", projName)
	fmt.Fprintf(ui.Out, "  Status:     %s\n", output.StatusColor(string(issue.Status)))
	fmt.Fprintf(ui.Out, "  Priority:   %s\n", issue.Priority)
	fmt.Fprintf(ui.Out, "  Type:       %s\n", issue.Type)
	if issue.Description != "" {
		fmt.Fprintf(ui.Out, "  Desc:       %s\n", issue.Description)
	}
	if issue.Body != "" {
		fmt.Fprintf(ui.Out, "  Body:       %s\n", issue.Body)
	}
	if issue.GitHubIssue > 0 {
		fmt.Fprintf(ui.Out, "  GitHub:     #%d\n", issue.GitHubIssue)
	}
	if len(issue.Tags) > 0 {
		fmt.Fprintf(ui.Out, "  Tags:       %s\n", strings.Join(issue.Tags, ", "))
	}
	fmt.Fprintf(ui.Out, "  Created:    %s\n", issue.CreatedAt.Format(time.RFC3339))
	if issue.ClosedAt != nil {
		fmt.Fprintf(ui.Out, "  Closed:     %s\n", issue.ClosedAt.Format(time.RFC3339))
	}
	fmt.Fprintf(ui.Out, "  Full ID:    %s\n", issue.ID)

	return nil
}

func issueUpdateRun(id string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	issue, err := findIssue(ctx, s, id)
	if err != nil {
		return err
	}

	changed := false
	if issueStatus != "" {
		issue.Status = models.IssueStatus(issueStatus)
		changed = true
	}
	if issuePriority != "" {
		issue.Priority = models.IssuePriority(issuePriority)
		changed = true
	}
	if issueTitle != "" {
		issue.Title = issueTitle
		changed = true
	}
	if issueDesc != "" {
		issue.Description = issueDesc
		changed = true
	}
	if issueBody != "" {
		issue.Body = issueBody
		changed = true
	}

	if !changed {
		return fmt.Errorf("no updates specified (use --status, --priority, --title, --desc, or --body)")
	}

	if dryRun {
		ui.DryRunMsg("Would update issue %s", shortID(issue.ID))
		return nil
	}

	if err := s.UpdateIssue(ctx, issue); err != nil {
		return fmt.Errorf("update issue: %w", err)
	}

	ui.Success("Updated issue %s", output.Cyan(shortID(issue.ID)))
	return nil
}

func issueCloseRun(id string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	issue, err := findIssue(ctx, s, id)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	issue.Status = models.IssueStatusClosed
	issue.ClosedAt = &now

	if dryRun {
		ui.DryRunMsg("Would close issue %s: %s", shortID(issue.ID), issue.Title)
		return nil
	}

	if err := s.UpdateIssue(ctx, issue); err != nil {
		return fmt.Errorf("close issue: %w", err)
	}

	ui.Success("Closed issue %s: %s", output.Cyan(shortID(issue.ID)), issue.Title)
	return nil
}

func issueLinkRun(id string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	issue, err := findIssue(ctx, s, id)
	if err != nil {
		return err
	}

	issue.GitHubIssue = issueGitHub

	if dryRun {
		ui.DryRunMsg("Would link issue %s to GitHub #%d", shortID(issue.ID), issueGitHub)
		return nil
	}

	if err := s.UpdateIssue(ctx, issue); err != nil {
		return fmt.Errorf("link issue: %w", err)
	}

	ui.Success("Linked issue %s to GitHub #%d", output.Cyan(shortID(issue.ID)), issueGitHub)
	return nil
}

// resolveProjectOrCwd resolves a project by name/path or auto-detects from cwd.
func resolveProjectOrCwd(ctx context.Context, s store.Store, ref string) (*models.Project, error) {
	if ref != "" {
		return resolveProject(ctx, s, ref)
	}
	return resolveProjectFromCwd(ctx, s)
}

// resolveProjectFromCwd tries to find a tracked project matching the current directory.
// It first checks the exact cwd path, then falls back to the git repo root.
func resolveProjectFromCwd(ctx context.Context, s store.Store) (*models.Project, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get working directory: %w", err)
	}

	// Try exact path match
	if p, err := s.GetProjectByPath(ctx, cwd); err == nil {
		return p, nil
	}

	// Try git repo root (supports subdirectories)
	gc := git.NewClient()
	if root, err := gc.RepoRoot(cwd); err == nil && root != cwd {
		if p, err := s.GetProjectByPath(ctx, root); err == nil {
			return p, nil
		}
	}

	return nil, fmt.Errorf("no tracked project found for current directory: %s\nSpecify a project name or run from a tracked project directory", cwd)
}

// findIssue finds an issue by full ID or prefix match.
func findIssue(ctx context.Context, s store.Store, id string) (*models.Issue, error) {
	// Try exact match first
	if issue, err := s.GetIssue(ctx, id); err == nil {
		return issue, nil
	}

	// Try prefix match - list all and filter
	upper := strings.ToUpper(id)
	issues, err := s.ListIssues(ctx, store.IssueListFilter{})
	if err != nil {
		return nil, err
	}

	var matches []*models.Issue
	for _, issue := range issues {
		if strings.HasPrefix(issue.ID, upper) {
			matches = append(matches, issue)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("issue not found: %s", id)
	case 1:
		// Re-fetch to get tags loaded
		return s.GetIssue(ctx, matches[0].ID)
	default:
		return nil, fmt.Errorf("ambiguous issue ID %s: matches %d issues", id, len(matches))
	}
}

// shortID returns a truncated ULID for display (first 12 chars).
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// applyTag creates a tag if needed and applies it to an issue.
func applyTag(ctx context.Context, s store.Store, issueID, tagName string) error {
	// Find or create the tag
	tags, err := s.ListTags(ctx)
	if err != nil {
		return err
	}

	var tagID string
	for _, t := range tags {
		if t.Name == tagName {
			tagID = t.ID
			break
		}
	}

	if tagID == "" {
		tag := &models.Tag{Name: tagName}
		if err := s.CreateTag(ctx, tag); err != nil {
			return err
		}
		tagID = tag.ID
	}

	return s.TagIssue(ctx, issueID, tagID)
}
