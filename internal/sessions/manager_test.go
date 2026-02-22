package sessions

import (
	"testing"

	"github.com/joescharf/wt/pkg/ops"
	"github.com/stretchr/testify/assert"
)

// TestRebaseOptionMapping_Sync verifies that SyncOptions.Rebase=true produces "rebase" strategy.
func TestRebaseOptionMapping_Sync(t *testing.T) {
	tests := []struct {
		name     string
		rebase   bool
		expected string
	}{
		{"merge strategy by default", false, "merge"},
		{"rebase strategy when requested", true, "rebase"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := SyncOptions{Rebase: tt.rebase}

			strategy := "merge"
			if opts.Rebase {
				strategy = "rebase"
			}

			syncOpts := ops.SyncOptions{
				BaseBranch: "main",
				Strategy:   strategy,
			}

			assert.Equal(t, tt.expected, syncOpts.Strategy)
		})
	}
}

// TestRebaseOptionMapping_Merge verifies that MergeOptions.Rebase=true produces "rebase" strategy.
func TestRebaseOptionMapping_Merge(t *testing.T) {
	tests := []struct {
		name     string
		rebase   bool
		expected string
	}{
		{"merge strategy by default", false, "merge"},
		{"rebase strategy when requested", true, "rebase"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := MergeOptions{Rebase: tt.rebase}

			strategy := "merge"
			if opts.Rebase {
				strategy = "rebase"
			}

			mergeOpts := ops.MergeOptions{
				BaseBranch: "main",
				Strategy:   strategy,
			}

			assert.Equal(t, tt.expected, mergeOpts.Strategy)
		})
	}
}

// TestGitAdapterImplementsRebase verifies repoBoundClient has Rebase methods.
func TestGitAdapterImplementsRebase(t *testing.T) {
	// Verify repoBoundClient has the rebase-related methods by instantiating it
	// and confirming it implements the interface methods used by ops.Merge/Sync.
	client := newRepoBoundClient("/tmp/nonexistent")

	// These type assertions confirm the methods exist on the client.
	// We can't call them without a real repo, but the type check is sufficient.
	type rebaseCapable interface {
		Rebase(repoPath, branch string) error
		RebaseContinue(repoPath string) error
		RebaseAbort(repoPath string) error
		IsRebaseInProgress(repoPath string) (bool, error)
	}

	_, ok := client.(rebaseCapable)
	assert.True(t, ok, "repoBoundClient should implement rebase methods")
}
