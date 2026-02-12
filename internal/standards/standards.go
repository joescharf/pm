package standards

import (
	"os"
	"path/filepath"
)

// Check represents a single standardization check.
type Check struct {
	Name   string
	Passed bool
	Detail string
}

// Checker evaluates project standardization.
type Checker struct{}

// NewChecker returns a new Checker.
func NewChecker() *Checker {
	return &Checker{}
}

// Run evaluates all standard checks for a project at the given path.
func (c *Checker) Run(path string) []Check {
	var checks []Check

	checks = append(checks, checkFile(path, ".goreleaser.yml", "GoReleaser config"))
	checks = append(checks, checkFile(path, "Makefile", "Makefile"))
	checks = append(checks, checkFile(path, "CLAUDE.md", "CLAUDE.md"))
	checks = append(checks, checkFile(path, ".mockery.yml", "Mockery config"))
	checks = append(checks, checkFile(path, "LICENSE", "LICENSE file"))
	checks = append(checks, checkFile(path, "README.md", "README"))
	checks = append(checks, checkFile(path, "go.mod", "Go module"))
	checks = append(checks, checkDir(path, "internal", "internal/ directory"))
	checks = append(checks, checkHasTests(path))

	return checks
}

func checkFile(base, name, label string) Check {
	_, err := os.Stat(filepath.Join(base, name))
	if err == nil {
		return Check{Name: label, Passed: true, Detail: name + " found"}
	}
	return Check{Name: label, Passed: false, Detail: name + " missing"}
}

func checkDir(base, name, label string) Check {
	info, err := os.Stat(filepath.Join(base, name))
	if err == nil && info.IsDir() {
		return Check{Name: label, Passed: true, Detail: name + "/ found"}
	}
	return Check{Name: label, Passed: false, Detail: name + "/ missing"}
}

func checkHasTests(path string) Check {
	// Check for any _test.go files recursively
	found := false
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && filepath.Ext(p) == ".go" {
			base := filepath.Base(p)
			if len(base) > 8 && base[len(base)-8:] == "_test.go" {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})

	if found {
		return Check{Name: "Tests", Passed: true, Detail: "_test.go files found"}
	}
	return Check{Name: "Tests", Passed: false, Detail: "no _test.go files found"}
}
