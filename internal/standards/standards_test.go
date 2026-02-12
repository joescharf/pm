package standards

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChecker_EmptyProject(t *testing.T) {
	dir := t.TempDir()
	c := NewChecker()
	checks := c.Run(dir)

	// All checks should fail for empty dir
	for _, check := range checks {
		assert.False(t, check.Passed, "check %s should fail in empty dir", check.Name)
	}
}

func TestChecker_FullProject(t *testing.T) {
	dir := t.TempDir()

	// Create all expected files
	files := []string{".goreleaser.yml", "Makefile", "CLAUDE.md", ".mockery.yml", "LICENSE", "README.md", "go.mod"}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f), []byte(""), 0644))
	}

	// Create internal/ dir
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "internal"), 0755))

	// Create a test file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main_test.go"), []byte("package main"), 0644))

	c := NewChecker()
	checks := c.Run(dir)

	for _, check := range checks {
		assert.True(t, check.Passed, "check %s should pass: %s", check.Name, check.Detail)
	}
}

func TestChecker_PartialProject(t *testing.T) {
	dir := t.TempDir()

	// Only some files
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Makefile"), []byte(""), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644))

	c := NewChecker()
	checks := c.Run(dir)

	passed := 0
	for _, check := range checks {
		if check.Passed {
			passed++
		}
	}
	assert.Equal(t, 2, passed, "should pass exactly 2 checks (Makefile and go.mod)")
}
