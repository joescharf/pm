package llm

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildPrompt(t *testing.T) {
	t.Run("with projects", func(t *testing.T) {
		system, user := buildPrompt("# Issues\n1. Fix bug", []string{"pm", "gsi"})

		assert.Contains(t, system, "JSON array")
		assert.Contains(t, system, `"project"`)
		assert.Contains(t, system, `"title"`)
		assert.Contains(t, system, `"type"`)
		assert.Contains(t, system, `"priority"`)

		assert.Contains(t, user, "Known projects: pm, gsi")
		assert.Contains(t, user, "Fix bug")
	})

	t.Run("without projects", func(t *testing.T) {
		system, user := buildPrompt("some content", nil)

		assert.Contains(t, system, "JSON array")
		assert.NotContains(t, user, "Known projects")
		assert.Contains(t, user, "some content")
	})

	t.Run("system prompt specifies valid types and priorities", func(t *testing.T) {
		system, _ := buildPrompt("content", nil)

		assert.Contains(t, system, `"feature"`)
		assert.Contains(t, system, `"bug"`)
		assert.Contains(t, system, `"chore"`)
		assert.Contains(t, system, `"low"`)
		assert.Contains(t, system, `"medium"`)
		assert.Contains(t, system, `"high"`)
	})
}

func TestBuildPromptContent(t *testing.T) {
	content := strings.Repeat("x", 10000)
	_, user := buildPrompt(content, []string{"a"})
	assert.Contains(t, user, content)
}

func TestBuildEnrichPrompt(t *testing.T) {
	t.Run("with all fields", func(t *testing.T) {
		system, user := buildEnrichPrompt("Fix login bug", "When user clicks login, page crashes", "Login page crashes on submit")

		assert.Contains(t, system, "description")
		assert.Contains(t, system, "ai_prompt")
		assert.Contains(t, system, "JSON")

		assert.Contains(t, user, "Fix login bug")
		assert.Contains(t, user, "When user clicks login, page crashes")
		assert.Contains(t, user, "Login page crashes on submit")
	})

	t.Run("with only title", func(t *testing.T) {
		system, user := buildEnrichPrompt("Add dark mode", "", "")

		assert.Contains(t, system, "description")
		assert.Contains(t, system, "ai_prompt")
		assert.Contains(t, user, "Add dark mode")
	})

	t.Run("with title and body no description", func(t *testing.T) {
		system, user := buildEnrichPrompt("Refactor auth", "The authentication module needs refactoring to use JWT tokens instead of session cookies", "")

		assert.Contains(t, system, "JSON")
		assert.Contains(t, user, "Refactor auth")
		assert.Contains(t, user, "JWT tokens")
	})

	t.Run("system prompt specifies JSON format", func(t *testing.T) {
		system, _ := buildEnrichPrompt("Test issue", "", "")

		assert.Contains(t, system, `"description"`)
		assert.Contains(t, system, `"ai_prompt"`)
	})
}
