package review

import (
	"fmt"
	"strings"

	"github.com/joescharf/pm/internal/models"
)

// BuildReviewPrompt generates the system prompt appended to Claude for autonomous review.
func BuildReviewPrompt(issue *models.Issue, session *models.AgentSession, project *models.Project, cfg Config) string {
	shortIssueID := issue.ID
	if len(shortIssueID) > 12 {
		shortIssueID = shortIssueID[:12]
	}

	sessionID := ""
	if session != nil {
		sessionID = session.ID
	}

	var b strings.Builder

	b.WriteString("You are an autonomous code review agent. Your job is to review the implementation of an issue, fix problems, run tests, and submit a review verdict.\n\n")

	b.WriteString("## Issue Context\n")
	fmt.Fprintf(&b, "- Issue ID: %s\n", shortIssueID)
	fmt.Fprintf(&b, "- Title: %s\n", issue.Title)
	if issue.Description != "" {
		fmt.Fprintf(&b, "- Description: %s\n", issue.Description)
	}
	if project != nil {
		fmt.Fprintf(&b, "- Project: %s (%s)\n", project.Name, project.Language)
	}
	if sessionID != "" {
		fmt.Fprintf(&b, "- Session ID: %s\n", sessionID)
	}
	b.WriteString("\n")

	b.WriteString("## Review Process\n\n")
	fmt.Fprintf(&b, "You have up to %d attempts to get this review to pass.\n\n", cfg.MaxAttempts)

	b.WriteString("For each attempt:\n\n")
	b.WriteString("1. **Gather context**: Call `pm_prepare_review` with issue_id to get the diff, requirements, and review history.\n\n")
	b.WriteString("2. **Review the diff**: Check that:\n")
	b.WriteString("   - The implementation matches the issue requirements (title, description, body, ai_prompt)\n")
	b.WriteString("   - Code quality is acceptable (no obvious bugs, follows project patterns)\n")
	b.WriteString("   - No security issues or regressions\n\n")
	b.WriteString("3. **Run tests**: Execute `make test` or `go test ./...` to verify all tests pass.\n\n")
	b.WriteString("4. **If issues found**: Fix them directly:\n")
	b.WriteString("   - Edit code to fix bugs, add missing tests, resolve lint issues\n")
	b.WriteString("   - Commit fixes with descriptive messages\n")
	b.WriteString("   - Re-run tests to verify fixes\n\n")
	b.WriteString("5. **Sync with main**: Call `pm_sync_session` with the session_id to ensure the branch is up to date.\n\n")
	b.WriteString("6. **Submit verdict**: Call `pm_save_review` with:\n")
	b.WriteString("   - `verdict`: \"pass\" if everything looks good, \"fail\" if there are unresolvable issues\n")
	b.WriteString("   - `summary`: A narrative summary of what you found\n")
	b.WriteString("   - `code_quality`: \"pass\" or \"fail\"\n")
	b.WriteString("   - `requirements_match`: \"pass\" or \"fail\"\n")
	b.WriteString("   - `test_coverage`: \"pass\" or \"fail\"\n")
	b.WriteString("   - `failure_reasons`: newline-separated list if failing\n\n")

	b.WriteString("## Decision Criteria\n\n")
	b.WriteString("**Pass** when:\n")
	b.WriteString("- Implementation matches requirements\n")
	b.WriteString("- All tests pass\n")
	b.WriteString("- No obvious security issues\n")
	b.WriteString("- Code follows existing project patterns\n\n")
	b.WriteString("**Fail** when:\n")
	b.WriteString("- Requirements are not met and cannot be fixed\n")
	b.WriteString("- Tests fail and you cannot fix them within your attempts\n")
	b.WriteString("- Fundamental architectural problems\n\n")

	b.WriteString("## Rules\n\n")
	b.WriteString("- Always start by calling `pm_prepare_review` — do NOT skip this step\n")
	b.WriteString("- Fix issues yourself rather than just reporting them when possible\n")
	b.WriteString("- If you fix code, always re-run tests before submitting verdict\n")
	b.WriteString("- Be thorough but practical — focus on correctness, not style nitpicks\n")
	b.WriteString("- Always submit a verdict before finishing, even if you run out of attempts\n")

	return b.String()
}

// BuildKickoffPrompt generates the short initial prompt passed to claude as the positional argument.
func BuildKickoffPrompt(issue *models.Issue) string {
	shortIssueID := issue.ID
	if len(shortIssueID) > 12 {
		shortIssueID = shortIssueID[:12]
	}
	return fmt.Sprintf("Review issue %s: %s. Call pm_prepare_review to begin.", shortIssueID, issue.Title)
}
