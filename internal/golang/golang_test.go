package golang

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGoVersion(t *testing.T) {
	dir := t.TempDir()
	goMod := filepath.Join(dir, "go.mod")
	require.NoError(t, os.WriteFile(goMod, []byte("module example.com/test\n\ngo 1.25.0\n"), 0644))

	a := NewAnalyzer()
	ver, err := a.GoVersion(dir)
	require.NoError(t, err)
	assert.Equal(t, "1.25.0", ver)
}

func TestModulePath(t *testing.T) {
	dir := t.TempDir()
	goMod := filepath.Join(dir, "go.mod")
	require.NoError(t, os.WriteFile(goMod, []byte("module example.com/test\n\ngo 1.25.0\n"), 0644))

	a := NewAnalyzer()
	mod, err := a.ModulePath(dir)
	require.NoError(t, err)
	assert.Equal(t, "example.com/test", mod)
}

func TestGoVersion_NoFile(t *testing.T) {
	dir := t.TempDir()
	a := NewAnalyzer()
	_, err := a.GoVersion(dir)
	assert.Error(t, err)
}

func TestIsGoProject(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, IsGoProject(dir))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644))
	assert.True(t, IsGoProject(dir))
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		file     string
		expected string
	}{
		{"go.mod", "go"},
		{"package.json", "javascript"},
		{"Cargo.toml", "rust"},
		{"pyproject.toml", "python"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, tt.file), []byte(""), 0644))
			assert.Equal(t, tt.expected, DetectLanguage(dir))
		})
	}

	t.Run("unknown", func(t *testing.T) {
		dir := t.TempDir()
		assert.Equal(t, "", DetectLanguage(dir))
	})
}
