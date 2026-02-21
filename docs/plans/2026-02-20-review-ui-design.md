# Review UI Design - Inline Review Form

## Overview

Add a review creation form inline on the issue detail page, enabling users to submit reviews through the web UI. Currently reviews can only be created via MCP/CLI.

## What Exists

- Backend: `GET/POST /api/v1/issues/{id}/reviews` endpoints
- Database: `issue_reviews` table with verdict, summary, 4 categories, failure_reasons
- Frontend: Read-only `ReviewHistory` component on issue detail page
- Types/hooks: `IssueReview` type, `useIssueReviews` hook

## What's Needed

### 1. Backend: Issue Status Transition on Review

The REST API `POST /api/v1/issues/{id}/reviews` currently creates a review but does NOT transition the issue status. The MCP `pm_save_review` does. Add status transition to the REST endpoint:
- Pass verdict -> issue status `closed`, set `ClosedAt`
- Fail verdict -> issue status `in_progress`, clear `ClosedAt`

### 2. Frontend: `useCreateReview` Mutation Hook

New hook in `ui/src/hooks/use-reviews.ts`:
- POST to `/api/v1/issues/{issueId}/reviews`
- Invalidates `["issue-reviews", issueId]` and `["issue", issueId]` query caches on success

### 3. Frontend: `ReviewForm` Component

New component `ui/src/components/issues/review-form.tsx`:
- Verdict: Two toggle buttons (Pass/Fail) with green/red colors
- Summary: Textarea (required)
- Categories: 4 button groups (Code Quality, Requirements, Tests, UI/UX) each with Pass/Fail/Skip/N/A
- Failure reasons (visible when verdict=fail): Add/remove string list
- Submit button with loading state
- Form resets on success

### 4. Integration in `IssueDetail`

- When issue status is `done`: Show ReviewForm card expanded above ReviewHistory
- Other non-closed statuses: Show collapsible "Add Review" button that expands the form
- Closed issues: Don't show the form

## Architecture Decision

**Inline form** over dialog/modal because:
- Reviews are the primary action on a "done" issue
- Single-page context keeps issue details visible while reviewing
- Matches the existing Card-based layout pattern
