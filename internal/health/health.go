package health

import (
	"time"

	"github.com/joescharf/pm/internal/models"
)

// ProjectMetadata holds live metadata used for health scoring.
type ProjectMetadata struct {
	IsDirty        bool
	LastCommitDate time.Time
	BranchCount    int
	WorktreeCount  int
	LatestRelease  string
	ReleaseDate    time.Time
}

// HealthScore represents the computed health of a project.
type HealthScore struct {
	Total            int
	GitCleanliness   int // 0-15
	ActivityRecency  int // 0-25
	IssueHealth      int // 0-20
	ReleaseFreshness int // 0-20
	BranchHygiene    int // 0-20
}

// Scorer computes health scores for projects.
type Scorer struct{}

// NewScorer returns a new health Scorer.
func NewScorer() *Scorer {
	return &Scorer{}
}

// Score computes a health score (0-100) for a project.
func (s *Scorer) Score(project *models.Project, meta *ProjectMetadata, issues []*models.Issue) *HealthScore {
	h := &HealthScore{}

	// Git cleanliness (15 pts) - clean repo = full points
	if !meta.IsDirty {
		h.GitCleanliness = 15
	} else {
		h.GitCleanliness = 5
	}

	// Activity recency (25 pts) - more recent = more points
	h.ActivityRecency = scoreRecency(meta.LastCommitDate, 25)

	// Issue health (20 pts) - fewer open issues relative to total = better
	h.IssueHealth = scoreIssues(issues, 20)

	// Release freshness (20 pts) - recent release = more points
	if !meta.ReleaseDate.IsZero() {
		h.ReleaseFreshness = scoreRecency(meta.ReleaseDate, 20)
	} else if meta.LatestRelease != "" {
		h.ReleaseFreshness = 10 // has releases but date unknown
	} else {
		h.ReleaseFreshness = 5 // no releases
	}

	// Branch hygiene (20 pts) - fewer branches = cleaner
	h.BranchHygiene = scoreBranches(meta.BranchCount, 20)

	h.Total = h.GitCleanliness + h.ActivityRecency + h.IssueHealth + h.ReleaseFreshness + h.BranchHygiene
	return h
}

// scoreRecency converts time since last activity to points.
func scoreRecency(t time.Time, maxPoints int) int {
	if t.IsZero() {
		return 0
	}
	days := int(time.Since(t).Hours() / 24)
	switch {
	case days <= 1:
		return maxPoints
	case days <= 3:
		return int(float64(maxPoints) * 0.9)
	case days <= 7:
		return int(float64(maxPoints) * 0.75)
	case days <= 14:
		return int(float64(maxPoints) * 0.6)
	case days <= 30:
		return int(float64(maxPoints) * 0.4)
	case days <= 90:
		return int(float64(maxPoints) * 0.2)
	default:
		return int(float64(maxPoints) * 0.1)
	}
}

// scoreIssues computes issue health based on backlog.
func scoreIssues(issues []*models.Issue, maxPoints int) int {
	if len(issues) == 0 {
		return maxPoints // no issues = healthy
	}

	open := 0
	for _, i := range issues {
		if i.Status == models.IssueStatusOpen || i.Status == models.IssueStatusInProgress {
			open++
		}
	}

	ratio := float64(open) / float64(len(issues))
	// Lower open ratio = better health
	return int(float64(maxPoints) * (1 - ratio*0.8))
}

// scoreBranches penalizes having too many branches.
func scoreBranches(count, maxPoints int) int {
	switch {
	case count <= 3:
		return maxPoints
	case count <= 5:
		return int(float64(maxPoints) * 0.8)
	case count <= 10:
		return int(float64(maxPoints) * 0.6)
	case count <= 20:
		return int(float64(maxPoints) * 0.4)
	default:
		return int(float64(maxPoints) * 0.2)
	}
}
