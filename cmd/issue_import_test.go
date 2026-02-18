package cmd

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/joescharf/pm/internal/llm"
	"github.com/joescharf/pm/internal/models"
	"github.com/joescharf/pm/internal/output"
	"github.com/joescharf/pm/internal/store"
)

func TestParseMarkdownIssues(t *testing.T) {
	t.Run("basic numbered list with project heading", func(t *testing.T) {
		md := `# Quick Issues

## Project pm

1. Dashboard: click on project name should view the project
2. Import issues from a markdown file
3. Review the PROMPT.md and update README

## Project gsi

1. Add deployment pipeline
2. Fix login bug
`
		issues := parseMarkdownIssues(md)
		require.Len(t, issues, 5)

		assert.Equal(t, "pm", issues[0].Project)
		assert.Equal(t, "Dashboard: click on project name should view the project", issues[0].Title)
		assert.Equal(t, "feature", issues[0].Type)
		assert.Equal(t, "medium", issues[0].Priority)

		assert.Equal(t, "pm", issues[1].Project)
		assert.Equal(t, "Import issues from a markdown file", issues[1].Title)

		assert.Equal(t, "pm", issues[2].Project)

		assert.Equal(t, "gsi", issues[3].Project)
		assert.Equal(t, "Add deployment pipeline", issues[3].Title)

		assert.Equal(t, "gsi", issues[4].Project)
		assert.Equal(t, "Fix login bug", issues[4].Title)
		assert.Equal(t, "bug", issues[4].Type)
		assert.Equal(t, "medium", issues[4].Priority)
	})

	t.Run("classification through parser", func(t *testing.T) {
		md := `## Project test

1. Fix critical crash on startup
2. Refactor database layer
3. Add dark mode support
4. Minor cosmetic button fix
`
		issues := parseMarkdownIssues(md)
		require.Len(t, issues, 4)

		// bug + high priority (crash + critical)
		assert.Equal(t, "bug", issues[0].Type)
		assert.Equal(t, "high", issues[0].Priority)

		// chore + medium priority
		assert.Equal(t, "chore", issues[1].Type)
		assert.Equal(t, "medium", issues[1].Priority)

		// feature + medium priority
		assert.Equal(t, "feature", issues[2].Type)
		assert.Equal(t, "medium", issues[2].Priority)

		// bug (fix) + low priority (minor + cosmetic)
		assert.Equal(t, "bug", issues[3].Type)
		assert.Equal(t, "low", issues[3].Priority)
	})

	t.Run("bulleted list", func(t *testing.T) {
		md := `## Project test

- Item one
- Item two
* Item three
`
		issues := parseMarkdownIssues(md)
		require.Len(t, issues, 3)
		assert.Equal(t, "test", issues[0].Project)
		assert.Equal(t, "Item one", issues[0].Title)
		assert.Equal(t, "Item two", issues[1].Title)
		assert.Equal(t, "Item three", issues[2].Title)
	})

	t.Run("no project heading", func(t *testing.T) {
		md := `# Issues

1. First issue
2. Second issue
`
		issues := parseMarkdownIssues(md)
		require.Len(t, issues, 2)
		assert.Equal(t, "", issues[0].Project)
		assert.Equal(t, "First issue", issues[0].Title)
	})

	t.Run("body preserved from raw line", func(t *testing.T) {
		md := `## Project test

1. Fix critical crash on startup
- Add dark mode support
`
		issues := parseMarkdownIssues(md)
		require.Len(t, issues, 2)

		assert.Equal(t, "1. Fix critical crash on startup", issues[0].Body)
		assert.Equal(t, "- Add dark mode support", issues[1].Body)
	})

	t.Run("sub-issues include parent text in body", func(t *testing.T) {
		md := `## Project test

1. Authentication system
1.1 Add login form
1.2 Add password reset

2. Database improvements
2.1 Add connection pooling
`
		issues := parseMarkdownIssues(md)
		require.Len(t, issues, 5)

		// Parent issues have their own text as body
		assert.Equal(t, "Authentication system", issues[0].Title)
		assert.Equal(t, "1. Authentication system", issues[0].Body)

		// Sub-issues include parent text + own text in body
		assert.Equal(t, "Add login form", issues[1].Title)
		assert.Equal(t, "1. Authentication system\n1.1 Add login form", issues[1].Body)

		assert.Equal(t, "Add password reset", issues[2].Title)
		assert.Equal(t, "1. Authentication system\n1.2 Add password reset", issues[2].Body)

		// Second parent
		assert.Equal(t, "Database improvements", issues[3].Title)
		assert.Equal(t, "2. Database improvements", issues[3].Body)

		// Sub-issue of second parent
		assert.Equal(t, "Add connection pooling", issues[4].Title)
		assert.Equal(t, "2. Database improvements\n2.1 Add connection pooling", issues[4].Body)
	})

	t.Run("sub-issues with dot suffix", func(t *testing.T) {
		md := `## Project test

1. Main feature
1.1. Sub-feature with trailing dot
`
		issues := parseMarkdownIssues(md)
		require.Len(t, issues, 2)

		assert.Equal(t, "Sub-feature with trailing dot", issues[1].Title)
		assert.Equal(t, "1. Main feature\n1.1. Sub-feature with trailing dot", issues[1].Body)
	})

	t.Run("sub-issues inherit parent project", func(t *testing.T) {
		md := `## Project pm

1. Parent feature
1.1 Child task
`
		issues := parseMarkdownIssues(md)
		require.Len(t, issues, 2)

		assert.Equal(t, "pm", issues[0].Project)
		assert.Equal(t, "pm", issues[1].Project)
	})

	t.Run("sub-issue without parent is standalone", func(t *testing.T) {
		// Edge case: sub-issue numbering without a preceding parent
		md := `## Project test

1.1 Orphan sub-issue
`
		issues := parseMarkdownIssues(md)
		require.Len(t, issues, 1)

		assert.Equal(t, "Orphan sub-issue", issues[0].Title)
		// No parent to prepend, so body is just the sub-issue line
		assert.Equal(t, "1.1 Orphan sub-issue", issues[0].Body)
	})

	t.Run("empty file", func(t *testing.T) {
		issues := parseMarkdownIssues("")
		assert.Empty(t, issues)
	})

	t.Run("no list items", func(t *testing.T) {
		md := `# Just a heading

Some paragraph text without any list items.
`
		issues := parseMarkdownIssues(md)
		assert.Empty(t, issues)
	})
}

// setupTestStore creates a temp SQLite store with a project for import tests.
func setupTestStore(t *testing.T) (store.Store, *models.Project) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.NewSQLiteStore(filepath.Join(dir, "test.db"))
	require.NoError(t, err)
	require.NoError(t, s.Migrate(context.Background()))
	t.Cleanup(func() { _ = s.Close() })

	// Initialize the cmd-level ui so createExtractedIssues can use it
	ui = output.New()

	proj := &models.Project{Name: "testproj", Path: dir}
	require.NoError(t, s.CreateProject(context.Background(), proj))
	return s, proj
}

func TestCreateExtractedIssues_Idempotent(t *testing.T) {
	t.Run("first import creates all issues", func(t *testing.T) {
		s, proj := setupTestStore(t)
		ctx := context.Background()

		extracted := []llm.ExtractedIssue{
			{Project: proj.Name, Title: "Issue A", Type: "feature", Priority: "medium"},
			{Project: proj.Name, Title: "Issue B", Type: "bug", Priority: "high"},
			{Project: proj.Name, Title: "Issue C", Type: "chore", Priority: "low"},
		}

		err := createExtractedIssues(ctx, s, extracted)
		require.NoError(t, err)

		issues, err := s.ListIssues(ctx, store.IssueListFilter{ProjectID: proj.ID})
		require.NoError(t, err)
		assert.Len(t, issues, 3)
	})

	t.Run("second import creates no duplicates", func(t *testing.T) {
		s, proj := setupTestStore(t)
		ctx := context.Background()

		extracted := []llm.ExtractedIssue{
			{Project: proj.Name, Title: "Issue A", Type: "feature", Priority: "medium"},
			{Project: proj.Name, Title: "Issue B", Type: "bug", Priority: "high"},
		}

		// First import
		err := createExtractedIssues(ctx, s, extracted)
		require.NoError(t, err)

		// Second import — same issues
		err = createExtractedIssues(ctx, s, extracted)
		require.NoError(t, err)

		issues, err := s.ListIssues(ctx, store.IssueListFilter{ProjectID: proj.ID})
		require.NoError(t, err)
		assert.Len(t, issues, 2, "should not create duplicates")
	})

	t.Run("mixed import creates only new issues", func(t *testing.T) {
		s, proj := setupTestStore(t)
		ctx := context.Background()

		// First batch
		batch1 := []llm.ExtractedIssue{
			{Project: proj.Name, Title: "Existing A", Type: "feature", Priority: "medium"},
			{Project: proj.Name, Title: "Existing B", Type: "bug", Priority: "high"},
		}
		err := createExtractedIssues(ctx, s, batch1)
		require.NoError(t, err)

		// Second batch — mix of existing and new
		batch2 := []llm.ExtractedIssue{
			{Project: proj.Name, Title: "Existing A", Type: "feature", Priority: "medium"},
			{Project: proj.Name, Title: "New C", Type: "chore", Priority: "low"},
			{Project: proj.Name, Title: "Existing B", Type: "bug", Priority: "high"},
			{Project: proj.Name, Title: "New D", Type: "feature", Priority: "medium"},
		}
		err = createExtractedIssues(ctx, s, batch2)
		require.NoError(t, err)

		issues, err := s.ListIssues(ctx, store.IssueListFilter{ProjectID: proj.ID})
		require.NoError(t, err)
		assert.Len(t, issues, 4, "should have 2 original + 2 new")

		titles := make(map[string]bool)
		for _, issue := range issues {
			titles[issue.Title] = true
		}
		assert.True(t, titles["Existing A"])
		assert.True(t, titles["Existing B"])
		assert.True(t, titles["New C"])
		assert.True(t, titles["New D"])
	})

	t.Run("duplicates within same batch are caught", func(t *testing.T) {
		s, proj := setupTestStore(t)
		ctx := context.Background()

		extracted := []llm.ExtractedIssue{
			{Project: proj.Name, Title: "Dup Issue", Type: "feature", Priority: "medium"},
			{Project: proj.Name, Title: "Dup Issue", Type: "feature", Priority: "medium"},
			{Project: proj.Name, Title: "Unique Issue", Type: "bug", Priority: "high"},
		}

		err := createExtractedIssues(ctx, s, extracted)
		require.NoError(t, err)

		issues, err := s.ListIssues(ctx, store.IssueListFilter{ProjectID: proj.ID})
		require.NoError(t, err)
		assert.Len(t, issues, 2, "should deduplicate within the same batch")
	})
}
