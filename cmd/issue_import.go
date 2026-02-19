package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joescharf/pm/internal/llm"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/store"
)

var (
	importProject string
	importDryRun  bool
)

var importCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Import issues from a markdown file",
	Long: `Import issues from a markdown file using an LLM to extract structured data.

The markdown file should contain issues as numbered or bulleted lists,
optionally grouped under "## Project <name>" headings.

Requires ANTHROPIC_API_KEY environment variable or anthropic.api_key in config.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return issueImportRun(args[0])
	},
}

func init() {
	importCmd.Flags().StringVar(&importProject, "project", "", "Assign all issues to this project (skip LLM project inference)")
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "Preview extracted issues without creating them")
	rootCmd.AddCommand(importCmd)
}

func issueImportRun(file string) error {
	// Read the markdown file
	data, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}
	content := string(data)
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("file is empty: %s", file)
	}

	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	// If --project is specified, try a simple parse first without LLM
	if importProject != "" {
		return importWithProject(ctx, s, content, importProject)
	}

	return importWithLLM(ctx, s, content)
}

// importWithLLM uses Claude to extract and assign issues to projects.
func importWithLLM(ctx context.Context, s store.Store, content string) error {
	client := newLLMClient()
	if client == nil {
		return fmt.Errorf("ANTHROPIC_API_KEY not set (set env var or anthropic.api_key in config)")
	}

	// Get known project names for the LLM
	projects, err := s.ListProjects(ctx, "")
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}
	projectNames := make([]string, len(projects))
	for i, p := range projects {
		projectNames[i] = p.Name
	}

	ui.Info("Extracting issues with LLM...")
	extracted, err := client.ExtractIssues(ctx, content, projectNames)
	if err != nil {
		return fmt.Errorf("extract issues: %w", err)
	}

	if len(extracted) == 0 {
		ui.Info("No issues extracted from file.")
		return nil
	}

	// Preview table with duplicate detection
	dupsInDryRun := 0
	table := ui.Table([]string{"#", "Project", "Title", "Type", "Priority", "Status"})
	dryRunTitleCache := make(map[string]map[string]bool)
	for i, e := range extracted {
		if isPlaceholderTitle(e.Title) {
			continue
		}
		status := "new"
		if importDryRun || dryRun {
			// Check for duplicates in dry-run mode
			if _, ok := dryRunTitleCache[e.Project]; !ok {
				proj, err := s.GetProjectByName(ctx, e.Project)
				if err == nil {
					titles, err := existingTitlesForProject(ctx, s, proj.ID)
					if err == nil {
						dryRunTitleCache[e.Project] = titles
					}
				}
				if dryRunTitleCache[e.Project] == nil {
					dryRunTitleCache[e.Project] = make(map[string]bool)
				}
			}
			if dryRunTitleCache[e.Project][e.Title] {
				status = "duplicate"
				dupsInDryRun++
			} else {
				dryRunTitleCache[e.Project][e.Title] = true
			}
		}
		_ = table.Append([]string{
			fmt.Sprintf("%d", i+1),
			e.Project,
			e.Title,
			e.Type,
			e.Priority,
			status,
		})
	}
	_ = table.Render()

	if importDryRun || dryRun {
		newCount := len(extracted) - dupsInDryRun
		ui.DryRunMsg("Would create %d issues, skip %d duplicates", newCount, dupsInDryRun)
		return nil
	}

	// Create issues
	return createExtractedIssues(ctx, s, extracted)
}

// importWithProject assigns all issues from a simple parse to the given project.
func importWithProject(ctx context.Context, s store.Store, content, projectName string) error {
	p, err := resolveProject(ctx, s, projectName)
	if err != nil {
		return fmt.Errorf("project %q: %w", projectName, err)
	}

	issues := parseMarkdownIssues(content)
	if len(issues) == 0 {
		ui.Info("No issues found in file.")
		return nil
	}

	// Assign all to the specified project
	for i := range issues {
		issues[i].Project = p.Name
	}

	// Preview table with duplicate detection
	dupsInDryRun := 0
	existingTitles, _ := existingTitlesForProject(ctx, s, p.ID)
	if existingTitles == nil {
		existingTitles = make(map[string]bool)
	}
	table := ui.Table([]string{"#", "Project", "Title", "Type", "Priority", "Status"})
	for i, e := range issues {
		if isPlaceholderTitle(e.Title) {
			continue
		}
		status := "new"
		if importDryRun || dryRun {
			if existingTitles[e.Title] {
				status = "duplicate"
				dupsInDryRun++
			} else {
				existingTitles[e.Title] = true
			}
		}
		_ = table.Append([]string{
			fmt.Sprintf("%d", i+1),
			e.Project,
			e.Title,
			e.Type,
			e.Priority,
			status,
		})
	}
	_ = table.Render()

	if importDryRun || dryRun {
		newCount := len(issues) - dupsInDryRun
		ui.DryRunMsg("Would create %d issues for project %s, skip %d duplicates", newCount, p.Name, dupsInDryRun)
		return nil
	}

	return createExtractedIssues(ctx, s, issues)
}

// parseSubIssueNumber checks if a line starts with a sub-issue number like "1.1" or "2.3."
// Returns the title text and true if it's a sub-issue, or empty and false otherwise.
func parseSubIssueNumber(line string) (title string, ok bool) {
	// Pattern: digits.digits[.] space text (e.g., "1.1 text" or "1.1. text")
	i := 0
	// First number
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(line) || line[i] != '.' {
		return "", false
	}
	i++ // skip first dot
	// Second number (must have at least one digit after the dot)
	start := i
	for i < len(line) && line[i] >= '0' && line[i] <= '9' {
		i++
	}
	if i == start {
		return "", false // no digits after dot — this is a regular "1. text" item
	}
	// Optional trailing dot (e.g., "1.1. text")
	if i < len(line) && line[i] == '.' {
		i++
	}
	// Must be followed by a space and text
	if i >= len(line) || line[i] != ' ' {
		return "", false
	}
	title = strings.TrimSpace(line[i:])
	if title == "" {
		return "", false
	}
	return title, true
}

// parseMarkdownIssues does a simple parse of markdown to extract numbered/bulleted items.
func parseMarkdownIssues(content string) []llm.ExtractedIssue {
	var issues []llm.ExtractedIssue
	currentProject := ""
	lastParentLine := "" // raw line of the last top-level numbered item

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)

		// Check for project heading: ## Project <name>
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimPrefix(line, "## ")
			heading = strings.TrimSpace(heading)
			if strings.HasPrefix(strings.ToLower(heading), "project ") {
				currentProject = strings.TrimSpace(heading[8:])
			}
			lastParentLine = ""
			continue
		}

		// Check for sub-issue first (e.g., "1.1 text", "2.3. text")
		if subTitle, ok := parseSubIssueNumber(line); ok {
			body := line
			if lastParentLine != "" {
				body = lastParentLine + "\n" + line
			}
			issues = append(issues, llm.ExtractedIssue{
				Project:  currentProject,
				Title:    subTitle,
				Type:     classifyIssueType(subTitle),
				Priority: classifyIssuePriority(subTitle),
				Body:     body,
			})
			continue
		}

		// Check for numbered list item: "1. Title" or "- Title"
		title := ""
		if len(line) > 2 {
			// Numbered: "1. text", "12. text"
			for i, c := range line {
				if c == '.' && i > 0 && i < 4 {
					rest := strings.TrimSpace(line[i+1:])
					if rest != "" {
						title = rest
					}
					break
				}
				if c < '0' || c > '9' {
					break
				}
			}
			// Bulleted: "- text"
			if title == "" && (strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ")) {
				title = strings.TrimSpace(line[2:])
			}
		}

		if title != "" {
			// Track top-level numbered items as potential parents for sub-issues
			// (only numbered items, not bullets, can be parents)
			if !strings.HasPrefix(line, "- ") && !strings.HasPrefix(line, "* ") {
				lastParentLine = line
			}
			issues = append(issues, llm.ExtractedIssue{
				Project:  currentProject,
				Title:    title,
				Type:     classifyIssueType(title),
				Priority: classifyIssuePriority(title),
				Body:     line,
			})
		}
	}

	return issues
}

// existingTitlesForProject returns the set of existing issue titles for a project.
func existingTitlesForProject(ctx context.Context, s store.Store, projectID string) (map[string]bool, error) {
	issues, err := s.ListIssues(ctx, store.IssueListFilter{ProjectID: projectID})
	if err != nil {
		return nil, err
	}
	titles := make(map[string]bool, len(issues))
	for _, issue := range issues {
		titles[issue.Title] = true
	}
	return titles, nil
}

// isPlaceholderTitle returns true if the title is empty or a known placeholder
// that LLMs sometimes generate for empty project sections.
func isPlaceholderTitle(title string) bool {
	t := strings.TrimSpace(strings.ToLower(title))
	if t == "" {
		return true
	}
	placeholders := []string{
		"no issues specified",
		"no issues found",
		"no issues",
		"none",
		"n/a",
		"na",
		"no issues listed",
		"no issues identified",
		"no issues to import",
	}
	for _, p := range placeholders {
		if t == p {
			return true
		}
	}
	return false
}

// createExtractedIssues resolves projects and creates issues in the store.
// It skips issues whose title already exists in the same project (duplicate detection).
func createExtractedIssues(ctx context.Context, s store.Store, extracted []llm.ExtractedIssue) error {
	// Cache project lookups and existing titles per project
	projectCache := make(map[string]*models.Project)
	titleCache := make(map[string]map[string]bool) // projectName -> set of titles
	created := 0
	duplicates := 0
	skipped := 0

	for _, e := range extracted {
		// Skip empty or placeholder titles (e.g., LLM generating "no issues specified")
		if isPlaceholderTitle(e.Title) {
			skipped++
			continue
		}

		proj, ok := projectCache[e.Project]
		if !ok {
			p, err := s.GetProjectByName(ctx, e.Project)
			if err != nil {
				ui.Warning("Skipping issue %q: project %q not found", e.Title, e.Project)
				skipped++
				continue
			}
			projectCache[e.Project] = p
			proj = p

			// Load existing titles for this project
			titles, err := existingTitlesForProject(ctx, s, proj.ID)
			if err != nil {
				return fmt.Errorf("load existing issues for project %q: %w", e.Project, err)
			}
			titleCache[e.Project] = titles
		}

		// Check for duplicate
		if titleCache[e.Project][e.Title] {
			duplicates++
			continue
		}

		issueType := models.IssueType(e.Type)
		if issueType != models.IssueTypeFeature && issueType != models.IssueTypeBug && issueType != models.IssueTypeChore {
			issueType = models.IssueTypeFeature
		}

		issuePriority := models.IssuePriority(e.Priority)
		if issuePriority != models.IssuePriorityLow && issuePriority != models.IssuePriorityMedium && issuePriority != models.IssuePriorityHigh {
			issuePriority = models.IssuePriorityMedium
		}

		issue := &models.Issue{
			ProjectID:   proj.ID,
			Title:       e.Title,
			Description: e.Description,
			Body:        e.Body,
			Status:      models.IssueStatusOpen,
			Priority:    issuePriority,
			Type:        issueType,
		}

		if err := s.CreateIssue(ctx, issue); err != nil {
			ui.Warning("Failed to create issue %q: %v", e.Title, err)
			skipped++
			continue
		}
		created++
		// Add to title cache so subsequent duplicates within this batch are caught
		titleCache[e.Project][e.Title] = true
	}

	// Count unique projects
	projectSet := make(map[string]bool)
	for name := range projectCache {
		projectSet[name] = true
	}

	ui.Success("Created %d issues across %d projects", created, len(projectSet))
	if duplicates > 0 {
		ui.Info("Skipped %d duplicate issues", duplicates)
	}
	if skipped > 0 {
		ui.Warning("Skipped %d issues (errors)", skipped)
	}

	return nil
}
