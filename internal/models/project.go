package models

import "time"

// Project represents a tracked development project/repository.
type Project struct {
	ID             string
	Name           string
	Path           string
	Description    string
	RepoURL        string
	Language       string
	GroupName      string
	BranchCount    int
	HasGitHubPages bool
	PagesURL       string
	BuildCmd       string
	ServeCmd       string
	ServePort      int
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
