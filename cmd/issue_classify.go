package cmd

import "strings"

// classifyIssueType infers the issue type from the title using keyword heuristics.
// Bug keywords are checked before chore keywords (e.g., "fix the migration" = bug).
// Defaults to "feature" if no keywords match.
func classifyIssueType(title string) string {
	lower := strings.ToLower(title)

	// Multi-word phrases checked first, then single words with common variants.
	bugPhrases := []string{
		"issue with", "not working",
	}
	for _, kw := range bugPhrases {
		if strings.Contains(lower, kw) {
			return "bug"
		}
	}

	bugWords := []string{
		"fix ", "fix:", "fixed", "fixes", "fixing",
		"bug", "broken", "crash", "error",
		"regression", "fail", "fault", "defect",
	}
	for _, kw := range bugWords {
		if strings.Contains(lower, kw) {
			return "bug"
		}
	}
	// "fix" at end of string
	if strings.HasSuffix(lower, "fix") {
		return "bug"
	}

	choreKeywords := []string{
		"refactor", "cleanup", "clean up", "update dep", "migrate",
		"upgrade", "rename", "reorganize", "chore", "lint",
	}
	for _, kw := range choreKeywords {
		if strings.Contains(lower, kw) {
			return "chore"
		}
	}

	return "feature"
}

// classifyIssuePriority infers the issue priority from the title using keyword heuristics.
// High keywords are checked before low keywords. Defaults to "medium".
func classifyIssuePriority(title string) string {
	lower := strings.ToLower(title)

	highKeywords := []string{
		"critical", "urgent", "blocker", "crash", "security",
		"data loss", "production down", "p0", "p1",
	}
	for _, kw := range highKeywords {
		if strings.Contains(lower, kw) {
			return "high"
		}
	}

	lowKeywords := []string{
		"minor", "nice to have", "cosmetic", "trivial",
		"low priority", "cleanup", "clean up",
	}
	for _, kw := range lowKeywords {
		if strings.Contains(lower, kw) {
			return "low"
		}
	}

	return "medium"
}
