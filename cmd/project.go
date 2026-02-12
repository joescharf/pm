package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/golang"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/output"
	"github.com/joescharf/pm/internal/store"
)

var (
	projectGroup string
	projectName  string
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage tracked projects",
	Long:  "Add, remove, list, and show tracked development projects.",
}

var projectAddCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a project to tracking",
	Long:  "Add a project directory to pm tracking. Use '.' for the current directory.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return projectAddRun(args[0])
	},
}

var projectRemoveCmd = &cobra.Command{
	Use:     "remove <name-or-path>",
	Aliases: []string{"rm"},
	Short:   "Remove a project from tracking",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return projectRemoveRun(args[0])
	},
}

var projectListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List tracked projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		return projectListRun()
	},
}

var projectShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show detailed project information",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return projectShowRun(args[0])
	},
}

var projectRefreshCmd = &cobra.Command{
	Use:   "refresh [name]",
	Short: "Refresh project metadata",
	Long:  "Re-detect language, remote URL, and fetch GitHub description for one or all projects.",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) > 0 {
			return projectRefreshOneRun(args[0])
		}
		return projectRefreshAllRun()
	},
}

var projectScanCmd = &cobra.Command{
	Use:   "scan <directory>",
	Short: "Auto-discover git repos in a directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return projectScanRun(args[0])
	},
}

func init() {
	projectAddCmd.Flags().StringVar(&projectName, "name", "", "Override project name (default: directory name)")
	projectAddCmd.Flags().StringVar(&projectGroup, "group", "", "Project group name")

	projectListCmd.Flags().StringVar(&projectGroup, "group", "", "Filter by group")

	projectCmd.AddCommand(projectAddCmd)
	projectCmd.AddCommand(projectRemoveCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectShowCmd)
	projectCmd.AddCommand(projectRefreshCmd)
	projectCmd.AddCommand(projectScanCmd)
	rootCmd.AddCommand(projectCmd)
}

func projectAddRun(rawPath string) error {
	s, err := getStore()
	if err != nil {
		return err
	}

	// Resolve path
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	// Verify directory exists
	info, err := os.Stat(absPath)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("not a directory: %s", absPath)
	}

	// Determine name
	name := projectName
	if name == "" {
		name = filepath.Base(absPath)
	}

	// Detect language
	lang := golang.DetectLanguage(absPath)

	// Try to get remote URL
	gc := git.NewClient()
	remoteURL, _ := gc.RemoteURL(absPath)

	p := &models.Project{
		Name:      name,
		Path:      absPath,
		Language:  lang,
		RepoURL:   remoteURL,
		GroupName: projectGroup,
	}

	if dryRun {
		ui.DryRunMsg("Would add project: %s (%s)", name, absPath)
		return nil
	}

	if err := s.CreateProject(context.Background(), p); err != nil {
		return fmt.Errorf("add project: %w", err)
	}

	ui.Success("Added project: %s (%s)", output.Cyan(name), absPath)
	if lang != "" {
		ui.VerboseLog("Language: %s", lang)
	}
	if remoteURL != "" {
		ui.VerboseLog("Remote: %s", remoteURL)
	}
	return nil
}

func projectRemoveRun(nameOrPath string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	p, err := resolveProject(ctx, s, nameOrPath)
	if err != nil {
		return err
	}

	if dryRun {
		ui.DryRunMsg("Would remove project: %s", p.Name)
		return nil
	}

	if err := s.DeleteProject(ctx, p.ID); err != nil {
		return fmt.Errorf("remove project: %w", err)
	}

	ui.Success("Removed project: %s", output.Cyan(p.Name))
	return nil
}

func projectListRun() error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	projects, err := s.ListProjects(ctx, projectGroup)
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		ui.Info("No projects tracked. Use 'pm project add <path>' to get started.")
		return nil
	}

	table := ui.Table([]string{"Name", "Path", "Language", "Group", "Open Issues"})
	for _, p := range projects {
		issues, _ := s.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID, Status: models.IssueStatusOpen})
		openCount := fmt.Sprintf("%d", len(issues))

		table.Append([]string{
			output.Cyan(p.Name),
			p.Path,
			p.Language,
			p.GroupName,
			openCount,
		})
	}
	table.Render()
	return nil
}

func projectShowRun(name string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	p, err := resolveProject(ctx, s, name)
	if err != nil {
		return err
	}

	gc := git.NewClient()
	ga := golang.NewAnalyzer()

	// Header
	fmt.Fprintf(ui.Out, "%s\n", output.Cyan(p.Name))
	fmt.Fprintf(ui.Out, "  Path:       %s\n", p.Path)
	if p.Description != "" {
		fmt.Fprintf(ui.Out, "  Desc:       %s\n", p.Description)
	}
	if p.GroupName != "" {
		fmt.Fprintf(ui.Out, "  Group:      %s\n", p.GroupName)
	}
	if p.Language != "" {
		fmt.Fprintf(ui.Out, "  Language:   %s\n", p.Language)
	}
	if p.RepoURL != "" {
		fmt.Fprintf(ui.Out, "  Remote:     %s\n", p.RepoURL)
	}
	fmt.Fprintln(ui.Out)

	// Git info
	if branch, err := gc.CurrentBranch(p.Path); err == nil {
		fmt.Fprintf(ui.Out, "  Branch:     %s\n", branch)
	}
	if dirty, err := gc.IsDirty(p.Path); err == nil {
		status := output.Green("clean")
		if dirty {
			status = output.Red("dirty")
		}
		fmt.Fprintf(ui.Out, "  Status:     %s\n", status)
	}
	if hash, err := gc.LastCommitHash(p.Path); err == nil {
		msg, _ := gc.LastCommitMessage(p.Path)
		fmt.Fprintf(ui.Out, "  Last commit: %s %s\n", hash, msg)
	}
	if date, err := gc.LastCommitDate(p.Path); err == nil {
		fmt.Fprintf(ui.Out, "  Activity:   %s\n", timeAgo(date))
	}

	// Worktrees
	if wts, err := gc.WorktreeList(p.Path); err == nil && len(wts) > 1 {
		fmt.Fprintf(ui.Out, "  Worktrees:  %d\n", len(wts)-1) // exclude main
	}

	// Go-specific
	if golang.IsGoProject(p.Path) {
		if ver, err := ga.GoVersion(p.Path); err == nil {
			fmt.Fprintf(ui.Out, "  Go version: %s\n", ver)
		}
		if mod, err := ga.ModulePath(p.Path); err == nil {
			fmt.Fprintf(ui.Out, "  Module:     %s\n", mod)
		}
	}

	// Issue counts
	issues, err := s.ListIssues(ctx, store.IssueListFilter{ProjectID: p.ID})
	if err == nil && len(issues) > 0 {
		open, inProg := 0, 0
		for _, i := range issues {
			switch i.Status {
			case models.IssueStatusOpen:
				open++
			case models.IssueStatusInProgress:
				inProg++
			}
		}
		fmt.Fprintf(ui.Out, "  Issues:     %d open, %d in-progress\n", open, inProg)
	}

	// Version / Release info
	ghClient := git.NewGitHubClient()
	vi := getVersionInfo(gc, ghClient, p)
	if vi != nil {
		fmt.Fprintln(ui.Out)
		fmt.Fprintf(ui.Out, "  Version:    %s", output.Green(vi.Version))
		if vi.Source == "github" {
			fmt.Fprintf(ui.Out, " (GitHub release)")
		} else {
			fmt.Fprintf(ui.Out, " (git tag)")
		}
		fmt.Fprintln(ui.Out)
		if !vi.Date.IsZero() {
			fmt.Fprintf(ui.Out, "  Released:   %s\n", timeAgo(vi.Date))
		}
		if len(vi.Assets) > 0 {
			fmt.Fprintf(ui.Out, "  Assets:     %d files\n", len(vi.Assets))
			for _, a := range vi.Assets {
				size := formatBytes(a.Size)
				fmt.Fprintf(ui.Out, "              %s (%s, %d downloads)\n", a.Name, size, a.DownloadCount)
			}
		}
	}

	return nil
}

func projectScanRun(dir string) error {
	s, err := getStore()
	if err != nil {
		return err
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return fmt.Errorf("read directory: %w", err)
	}

	gc := git.NewClient()
	ctx := context.Background()
	added := 0

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		entryPath := filepath.Join(absDir, entry.Name())

		// Check if it's a git repo
		if _, err := gc.RepoRoot(entryPath); err != nil {
			continue
		}

		// Check if already tracked
		if _, err := s.GetProjectByPath(ctx, entryPath); err == nil {
			ui.VerboseLog("Already tracked: %s", entry.Name())
			continue
		}

		lang := golang.DetectLanguage(entryPath)
		remoteURL, _ := gc.RemoteURL(entryPath)

		p := &models.Project{
			Name:     entry.Name(),
			Path:     entryPath,
			Language: lang,
			RepoURL:  remoteURL,
		}

		if dryRun {
			ui.DryRunMsg("Would add: %s (%s)", entry.Name(), entryPath)
			added++
			continue
		}

		if err := s.CreateProject(ctx, p); err != nil {
			ui.Warning("Skipped %s: %v", entry.Name(), err)
			continue
		}

		ui.Success("Added: %s", output.Cyan(entry.Name()))
		added++
	}

	if added == 0 {
		ui.Info("No new projects found in %s", absDir)
	} else {
		ui.Info("Discovered %d project(s)", added)
	}
	return nil
}

func projectRefreshOneRun(nameOrPath string) error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	p, err := resolveProject(ctx, s, nameOrPath)
	if err != nil {
		return err
	}

	gc := git.NewClient()
	ghc := git.NewGitHubClient()

	if dryRun {
		ui.DryRunMsg("Would refresh project: %s", p.Name)
		return nil
	}

	changed, err := refreshProject(ctx, s, p, gc, ghc)
	if err != nil {
		return fmt.Errorf("refresh %s: %w", p.Name, err)
	}

	if changed {
		ui.Success("Refreshed project: %s", output.Cyan(p.Name))
	} else {
		ui.Info("No changes for project: %s", p.Name)
	}
	return nil
}

func projectRefreshAllRun() error {
	s, err := getStore()
	if err != nil {
		return err
	}
	ctx := context.Background()

	projects, err := s.ListProjects(ctx, "")
	if err != nil {
		return err
	}

	if len(projects) == 0 {
		ui.Info("No projects tracked.")
		return nil
	}

	gc := git.NewClient()
	ghc := git.NewGitHubClient()
	refreshed := 0

	for _, p := range projects {
		if dryRun {
			ui.DryRunMsg("Would refresh: %s", p.Name)
			refreshed++
			continue
		}

		changed, err := refreshProject(ctx, s, p, gc, ghc)
		if err != nil {
			ui.Warning("Failed to refresh %s: %v", p.Name, err)
			continue
		}
		if changed {
			ui.Success("Refreshed: %s", output.Cyan(p.Name))
			refreshed++
		} else {
			ui.VerboseLog("No changes: %s", p.Name)
		}
	}

	ui.Info("Refreshed %d of %d project(s)", refreshed, len(projects))
	return nil
}

// refreshProject re-detects metadata for a project and persists changes.
// Returns true if any field was updated.
func refreshProject(ctx context.Context, s store.Store, p *models.Project, gc git.Client, ghc git.GitHubClient) (bool, error) {
	changed := false

	// Validate path still exists
	if _, err := os.Stat(p.Path); err != nil {
		ui.Warning("Project path missing: %s", p.Path)
	}

	// Re-detect language
	if lang := golang.DetectLanguage(p.Path); lang != "" && lang != p.Language {
		ui.VerboseLog("Language: %s -> %s", p.Language, lang)
		p.Language = lang
		changed = true
	}

	// Re-detect remote URL
	if url, _ := gc.RemoteURL(p.Path); url != "" && url != p.RepoURL {
		ui.VerboseLog("RepoURL: %s -> %s", p.RepoURL, url)
		p.RepoURL = url
		changed = true
	}

	// Fetch GitHub metadata if we have a repo URL
	if p.RepoURL != "" {
		if owner, repo, err := git.ExtractOwnerRepo(p.RepoURL); err == nil {
			if info, err := ghc.RepoInfo(owner, repo); err == nil && info != nil {
				// Fill description only if currently empty
				if p.Description == "" && info.Description != "" {
					ui.VerboseLog("Description: (empty) -> %s", info.Description)
					p.Description = info.Description
					changed = true
				}
				// Use GitHub language as fallback if local detection returned empty
				if p.Language == "" && info.Language != "" {
					ui.VerboseLog("Language (GitHub): -> %s", info.Language)
					p.Language = info.Language
					changed = true
				}
			}
		}
	}

	if changed {
		if err := s.UpdateProject(ctx, p); err != nil {
			return false, fmt.Errorf("update project: %w", err)
		}
	}

	return changed, nil
}

// resolveProject finds a project by name or path.
func resolveProject(ctx context.Context, s store.Store, nameOrPath string) (*models.Project, error) {
	// Try by name first
	if p, err := s.GetProjectByName(ctx, nameOrPath); err == nil {
		return p, nil
	}

	// Try by path
	absPath, _ := filepath.Abs(nameOrPath)
	if p, err := s.GetProjectByPath(ctx, absPath); err == nil {
		return p, nil
	}

	return nil, fmt.Errorf("project not found: %s", nameOrPath)
}

// timeAgo returns a human-readable duration from a time.
func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}

// formatBytes returns a human-readable byte size string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
