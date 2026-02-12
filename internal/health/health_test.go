package health

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/joescharf/pm/internal/models"
)

func TestScore_HealthyProject(t *testing.T) {
	s := NewScorer()

	project := &models.Project{Name: "test"}
	meta := &ProjectMetadata{
		IsDirty:        false,
		LastCommitDate: time.Now().Add(-1 * time.Hour),
		BranchCount:    2,
	}
	issues := []*models.Issue{
		{Status: models.IssueStatusClosed},
		{Status: models.IssueStatusDone},
	}

	h := s.Score(project, meta, issues)

	assert.Equal(t, 15, h.GitCleanliness, "clean repo should get full git points")
	assert.Equal(t, 25, h.ActivityRecency, "recent activity should get full points")
	assert.Equal(t, 20, h.IssueHealth, "all closed issues = full points")
	assert.Equal(t, 20, h.BranchHygiene, "few branches = full points")
	assert.True(t, h.Total >= 80, "healthy project should score 80+")
}

func TestScore_UnhealthyProject(t *testing.T) {
	s := NewScorer()

	project := &models.Project{Name: "test"}
	meta := &ProjectMetadata{
		IsDirty:        true,
		LastCommitDate: time.Now().Add(-120 * 24 * time.Hour),
		BranchCount:    25,
	}
	issues := []*models.Issue{
		{Status: models.IssueStatusOpen},
		{Status: models.IssueStatusOpen},
		{Status: models.IssueStatusOpen},
	}

	h := s.Score(project, meta, issues)

	assert.Equal(t, 5, h.GitCleanliness, "dirty repo should get reduced git points")
	assert.True(t, h.ActivityRecency < 10, "old activity should get few points")
	assert.True(t, h.IssueHealth < 10, "all open issues = low health")
	assert.True(t, h.Total < 50, "unhealthy project should score below 50")
}

func TestScore_NoIssues(t *testing.T) {
	s := NewScorer()

	project := &models.Project{Name: "test"}
	meta := &ProjectMetadata{
		LastCommitDate: time.Now(),
		BranchCount:    1,
	}

	h := s.Score(project, meta, nil)
	assert.Equal(t, 20, h.IssueHealth, "no issues = full issue health")
}

func TestScore_WithRelease(t *testing.T) {
	s := NewScorer()

	project := &models.Project{Name: "test"}
	meta := &ProjectMetadata{
		LastCommitDate: time.Now(),
		BranchCount:    1,
		LatestRelease:  "v1.0.0",
		ReleaseDate:    time.Now().Add(-2 * 24 * time.Hour),
	}

	h := s.Score(project, meta, nil)
	assert.True(t, h.ReleaseFreshness > 10, "recent release should score well")
}

func TestScoreRecency(t *testing.T) {
	tests := []struct {
		name     string
		daysAgo  int
		minScore int
	}{
		{"today", 0, 20},
		{"this week", 5, 10},
		{"this month", 20, 5},
		{"old", 100, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := time.Now().Add(-time.Duration(tt.daysAgo) * 24 * time.Hour)
			score := scoreRecency(ts, 25)
			assert.True(t, score >= tt.minScore, "daysAgo=%d should score >= %d, got %d", tt.daysAgo, tt.minScore, score)
		})
	}
}

func TestScoreRecency_Zero(t *testing.T) {
	assert.Equal(t, 0, scoreRecency(time.Time{}, 25))
}

func TestScoreBranches(t *testing.T) {
	assert.Equal(t, 20, scoreBranches(1, 20))
	assert.Equal(t, 20, scoreBranches(3, 20))
	assert.Equal(t, 16, scoreBranches(5, 20))
	assert.Equal(t, 12, scoreBranches(10, 20))
	assert.Equal(t, 4, scoreBranches(30, 20))
}
