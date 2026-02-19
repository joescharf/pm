package models

import "time"

// ReviewVerdict is the outcome of an issue review.
type ReviewVerdict string

const (
	ReviewVerdictPass ReviewVerdict = "pass"
	ReviewVerdictFail ReviewVerdict = "fail"
)

// ReviewCategory is a per-aspect rating.
type ReviewCategory string

// IssueReview records a single AI review of an issue's implementation.
type IssueReview struct {
	ID                string
	IssueID           string
	SessionID         string
	Verdict           ReviewVerdict
	Summary           string
	CodeQuality       ReviewCategory
	RequirementsMatch ReviewCategory
	TestCoverage      ReviewCategory
	UIUX              ReviewCategory
	FailureReasons    []string
	DiffStats         string
	ReviewedAt        time.Time
	CreatedAt         time.Time
}
