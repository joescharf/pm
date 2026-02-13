package git

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ReleaseAsset represents a file attached to a GitHub release.
type ReleaseAsset struct {
	Name          string `json:"name"`
	DownloadCount int    `json:"downloadCount"`
	Size          int64  `json:"size"`
}

// Release represents a GitHub release.
type Release struct {
	TagName     string         `json:"tagName"`
	PublishedAt string         `json:"publishedAt"`
	IsLatest    bool           `json:"isLatest"`
	Assets      []ReleaseAsset `json:"assets"`
}

// PullRequest represents a GitHub pull request.
type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Branch string `json:"headRefName"`
	URL    string `json:"url"`
}

// RepoInfo represents basic GitHub repository information.
type RepoInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Stars       int    `json:"stargazerCount"`
	Language    string `json:"primaryLanguage"`
	IsPrivate   bool   `json:"isPrivate"`
	URL         string `json:"url"`
}

// PagesResult holds GitHub Pages configuration status.
type PagesResult struct {
	URL string `json:"html_url"`
}

// GitHubClient wraps the gh CLI for GitHub metadata.
type GitHubClient interface {
	LatestRelease(owner, repo string) (*Release, error)
	OpenPRs(owner, repo string) ([]PullRequest, error)
	RepoInfo(owner, repo string) (*RepoInfo, error)
	PagesInfo(owner, repo string) (*PagesResult, error)
}

// RealGitHubClient implements GitHubClient using the gh CLI.
type RealGitHubClient struct{}

// NewGitHubClient returns a new RealGitHubClient.
func NewGitHubClient() *RealGitHubClient {
	return &RealGitHubClient{}
}

func ghCmd(args ...string) (string, error) {
	out, err := exec.Command("gh", args...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("gh %s: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *RealGitHubClient) LatestRelease(owner, repo string) (*Release, error) {
	out, err := ghCmd("api",
		fmt.Sprintf("repos/%s/%s/releases/latest", owner, repo),
		"--jq", `{tagName: .tag_name, publishedAt: .published_at, isLatest: true, assets: [.assets[] | {name: .name, downloadCount: .download_count, size: .size}]}`,
	)
	if err != nil {
		return nil, err
	}

	var r Release
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		return nil, fmt.Errorf("parse release: %w", err)
	}
	return &r, nil
}

func (c *RealGitHubClient) OpenPRs(owner, repo string) ([]PullRequest, error) {
	out, err := ghCmd("pr", "list",
		"--repo", fmt.Sprintf("%s/%s", owner, repo),
		"--state", "open",
		"--json", "number,title,state,headRefName,url",
	)
	if err != nil {
		return nil, err
	}

	var prs []PullRequest
	if err := json.Unmarshal([]byte(out), &prs); err != nil {
		return nil, fmt.Errorf("parse PRs: %w", err)
	}
	return prs, nil
}

type repoInfoRaw struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	StargazerCount  int    `json:"stargazerCount"`
	PrimaryLanguage struct {
		Name string `json:"name"`
	} `json:"primaryLanguage"`
	IsPrivate bool   `json:"isPrivate"`
	URL       string `json:"url"`
}

func (c *RealGitHubClient) PagesInfo(owner, repo string) (*PagesResult, error) {
	out, err := ghCmd("api", fmt.Sprintf("repos/%s/%s/pages", owner, repo))
	if err != nil {
		// 404 means no pages configured — not an error
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
			return nil, nil
		}
		return nil, err
	}

	var result PagesResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		return nil, fmt.Errorf("parse pages info: %w", err)
	}
	return &result, nil
}

func (c *RealGitHubClient) RepoInfo(owner, repo string) (*RepoInfo, error) {
	out, err := ghCmd("repo", "view",
		fmt.Sprintf("%s/%s", owner, repo),
		"--json", "name,description,stargazerCount,primaryLanguage,isPrivate,url",
	)
	if err != nil {
		return nil, err
	}

	var raw repoInfoRaw
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("parse repo info: %w", err)
	}
	return &RepoInfo{
		Name:        raw.Name,
		Description: raw.Description,
		Stars:       raw.StargazerCount,
		Language:    raw.PrimaryLanguage.Name,
		IsPrivate:   raw.IsPrivate,
		URL:         raw.URL,
	}, nil
}
