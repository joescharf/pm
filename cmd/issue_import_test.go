package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
