package golang

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Analyzer provides Go project analysis by parsing go.mod.
type Analyzer interface {
	GoVersion(path string) (string, error)
	ModulePath(path string) (string, error)
}

// RealAnalyzer implements Analyzer by parsing go.mod files.
type RealAnalyzer struct{}

// NewAnalyzer returns a new RealAnalyzer.
func NewAnalyzer() *RealAnalyzer {
	return &RealAnalyzer{}
}

func (a *RealAnalyzer) GoVersion(path string) (string, error) {
	goMod := filepath.Join(path, "go.mod")
	return parseGoModField(goMod, "go ")
}

func (a *RealAnalyzer) ModulePath(path string) (string, error) {
	goMod := filepath.Join(path, "go.mod")
	return parseGoModField(goMod, "module ")
}

// parseGoModField reads go.mod and returns the value for a given prefix line.
func parseGoModField(goModPath, prefix string) (string, error) {
	f, err := os.Open(goModPath)
	if err != nil {
		return "", fmt.Errorf("open go.mod: %w", err)
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix)), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}
	return "", fmt.Errorf("field %q not found in %s", strings.TrimSpace(prefix), goModPath)
}

// IsGoProject returns true if the path contains a go.mod file.
func IsGoProject(path string) bool {
	_, err := os.Stat(filepath.Join(path, "go.mod"))
	return err == nil
}

// DetectLanguage attempts to detect the primary language of a project.
func DetectLanguage(path string) string {
	if IsGoProject(path) {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(path, "package.json")); err == nil {
		return "javascript"
	}
	if _, err := os.Stat(filepath.Join(path, "Cargo.toml")); err == nil {
		return "rust"
	}
	if _, err := os.Stat(filepath.Join(path, "pyproject.toml")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(path, "requirements.txt")); err == nil {
		return "python"
	}
	return ""
}
