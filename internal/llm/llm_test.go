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
