package refresh

import (
	"context"
	"fmt"
	"os"

	"github.com/joescharf/pm/internal/git"
	"github.com/joescharf/pm/internal/golang"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/store"
)

// Result holds the outcome of refreshing a single project.
type Result struct {
	Name    string `json:"name"`
	Changed bool   `json:"changed"`
	Error   string `json:"error,omitempty"`
}

// AllResult holds the outcome of refreshing all projects.
type AllResult struct {
	Refreshed int      `json:"refreshed"`
	Total     int      `json:"total"`
	Failed    int      `json:"failed"`
	Results   []Result `json:"results"`
}

// Project re-detects metadata for a single project and persists changes.
// Returns true if any field was updated.
func Project(ctx context.Context, s store.Store, p *models.Project, gc git.Client, ghc git.GitHubClient) (bool, error) {
	changed := false

	// Validate path still exists
	if _, err := os.Stat(p.Path); err != nil {
		return false, fmt.Errorf("project path missing: %s", p.Path)
	}

	// Re-detect language
	if lang := golang.DetectLanguage(p.Path); lang != "" && lang != p.Language {
		p.Language = lang
		changed = true
	}

	// Re-detect remote URL
	if url, _ := gc.RemoteURL(p.Path); url != "" && url != p.RepoURL {
		p.RepoURL = url
		changed = true
	}

	// Update branch count
	if branches, err := gc.BranchList(p.Path); err == nil {
		count := len(branches)
		if count != p.BranchCount {
			p.BranchCount = count
			changed = true
		}
	}

	// Fetch GitHub metadata if we have a repo URL
	if p.RepoURL != "" {
		if owner, repo, err := git.ExtractOwnerRepo(p.RepoURL); err == nil {
			if info, err := ghc.RepoInfo(owner, repo); err == nil && info != nil {
				if info.Description != "" && info.Description != p.Description {
					p.Description = info.Description
					changed = true
				}
				if p.Language == "" && info.Language != "" {
					p.Language = info.Language
					changed = true
				}
			}

			// Check GitHub Pages configuration
			if pages, err := ghc.PagesInfo(owner, repo); err == nil && pages != nil {
				if !p.HasGitHubPages || p.PagesURL != pages.URL {
					p.HasGitHubPages = true
					p.PagesURL = pages.URL
					changed = true
				}
			} else if p.HasGitHubPages {
				p.HasGitHubPages = false
				p.PagesURL = ""
				changed = true
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

// All refreshes metadata for all tracked projects.
func All(ctx context.Context, s store.Store, gc git.Client, ghc git.GitHubClient) (*AllResult, error) {
	projects, err := s.ListProjects(ctx, "")
	if err != nil {
		return nil, err
	}

	result := &AllResult{Total: len(projects)}
	for _, p := range projects {
		r := Result{Name: p.Name}
		changed, err := Project(ctx, s, p, gc, ghc)
		if err != nil {
			r.Error = err.Error()
			result.Failed++
		} else {
			r.Changed = changed
			if changed {
				result.Refreshed++
			}
		}
		result.Results = append(result.Results, r)
	}

	return result, nil
}
