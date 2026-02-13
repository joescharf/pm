package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyIssueType(t *testing.T) {
	tests := []struct {
		title    string
		expected string
	}{
		// Bug keywords
		{"Fix login bug", "bug"},
		{"fix broken authentication", "bug"},
		{"Crash on startup", "bug"},
		{"Error handling in API", "bug"},
		{"Regression in search results", "bug"},
		{"Login fails intermittently", "bug"},
		{"Fault in payment processing", "bug"},
		{"Defect in report generation", "bug"},
		{"Issue with dashboard loading", "bug"},
		{"Upload not working", "bug"},

		// Chore keywords
		{"Refactor database layer", "chore"},
		{"Cleanup old migrations", "chore"},
		{"Clean up test fixtures", "chore"},
		{"Update dependencies to latest", "chore"},
		{"Migrate to new API version", "chore"},
		{"Upgrade Go to 1.23", "chore"},
		{"Rename user service", "chore"},
		{"Reorganize project structure", "chore"},
		{"Lint configuration updates", "chore"},

		// Feature (default)
		{"Add dark mode", "feature"},
		{"Implement user profiles", "feature"},
		{"Dashboard: click on project name should view the project", "feature"},
		{"Add search functionality", "feature"},
		{"Support CSV export", "feature"},

		// Case insensitivity
		{"FIX the broken thing", "bug"},
		{"REFACTOR the module", "chore"},

		// "fix" at end of string
		{"Minor cosmetic button fix", "bug"},
		// "fix:" variant
		{"Fix: broken auth flow", "bug"},

		// No false positive on "fixtures"
		{"Clean up test fixtures", "chore"},

		// Bug takes precedence over chore
		{"Fix the migration script", "bug"},
		{"Fix cleanup task", "bug"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			assert.Equal(t, tt.expected, classifyIssueType(tt.title))
		})
	}
}

func TestClassifyIssuePriority(t *testing.T) {
	tests := []struct {
		title    string
		expected string
	}{
		// High priority
		{"Critical: database corruption", "high"},
		{"Urgent fix needed for auth", "high"},
		{"Blocker for release", "high"},
		{"App crash on login", "high"},
		{"Security vulnerability in API", "high"},
		{"Data loss when saving forms", "high"},
		{"Production down", "high"},
		{"P0: system outage", "high"},
		{"P1: degraded performance", "high"},

		// Low priority
		{"Minor UI alignment issue", "low"},
		{"Nice to have: dark mode toggle animation", "low"},
		{"Cosmetic fix for button color", "low"},
		{"Trivial typo in tooltip", "low"},
		{"Low priority: update footer text", "low"},
		{"Cleanup unused CSS classes", "low"},
		{"Clean up old log files", "low"},

		// Medium (default)
		{"Add user profiles", "medium"},
		{"Implement search", "medium"},
		{"Refactor auth module", "medium"},
		{"Update documentation", "medium"},

		// Case insensitivity
		{"CRITICAL outage", "high"},
		{"MINOR text change", "low"},

		// High takes precedence over low
		{"Critical cleanup needed", "high"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			assert.Equal(t, tt.expected, classifyIssuePriority(tt.title))
		})
	}
}
