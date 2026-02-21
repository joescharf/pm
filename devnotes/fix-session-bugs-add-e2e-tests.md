# Fix session bugs and add comprehensive E2E tests

*2026-02-21T00:16:31Z*

Fixed three bugs in the session/agent system: (1) launchAgent computed worktree paths incorrectly (project-branch instead of project.worktrees/branch), causing sessions to disappear after reconciliation; (2) listSessions returned stale data after ReconcileSessions changed session statuses mid-request; (3) UpdateAgentSession SQL was missing worktree_path column, so DeleteWorktree could never clear the path in the database. Added 27 non-mocked E2E tests using real git repos and worktrees covering the full agent/session lifecycle.

```bash
go test ./internal/api/... -count=1 -timeout=120s 2>&1 | tail -5
```

```output
ok  	github.com/joescharf/pm/internal/api	2.899s
```

```bash
go test ./internal/api/... -v -count=1 -timeout=120s 2>&1 | grep -E '(^=== RUN   Test[A-Z]|^--- (PASS|FAIL))' | head -40
```

```output
=== RUN   TestListProjects_Empty
--- PASS: TestListProjects_Empty (0.01s)
=== RUN   TestProjectCRUD_API
--- PASS: TestProjectCRUD_API (0.01s)
=== RUN   TestIssuesCRUD_API
--- PASS: TestIssuesCRUD_API (0.01s)
=== RUN   TestTags_API
--- PASS: TestTags_API (0.01s)
=== RUN   TestSessions_API
--- PASS: TestSessions_API (0.01s)
=== RUN   TestCORS
--- PASS: TestCORS (0.01s)
=== RUN   TestStatusOverview_HasVersionFields
--- PASS: TestStatusOverview_HasVersionFields (0.05s)
=== RUN   TestGetProject_NotFound
--- PASS: TestGetProject_NotFound (0.01s)
=== RUN   TestUpdateProject_PartialUpdate_PreservesOmittedFields
--- PASS: TestUpdateProject_PartialUpdate_PreservesOmittedFields (0.01s)
=== RUN   TestUpdateProject_EmptyStringsShouldNotOverwrite
--- PASS: TestUpdateProject_EmptyStringsShouldNotOverwrite (0.01s)
=== RUN   TestCloseAgent
--- PASS: TestCloseAgent (0.03s)
=== RUN   TestCleanupSessions_API
--- PASS: TestCleanupSessions_API (0.01s)
=== RUN   TestSessionLifecycle_E2E
--- PASS: TestSessionLifecycle_E2E (0.17s)
=== RUN   TestLaunchAgent_ResumesIdleSession
--- PASS: TestLaunchAgent_ResumesIdleSession (0.09s)
=== RUN   TestLaunchAgent_WorktreePathMatchesConvention
--- PASS: TestLaunchAgent_WorktreePathMatchesConvention (0.09s)
=== RUN   TestLaunchAgent_Validation
=== RUN   TestLaunchAgent_Validation/missing_project_id
=== RUN   TestLaunchAgent_Validation/missing_issue_ids
=== RUN   TestLaunchAgent_Validation/nonexistent_project
=== RUN   TestLaunchAgent_Validation/nonexistent_issue
--- PASS: TestLaunchAgent_Validation (0.07s)
=== RUN   TestLaunchAgent_IssueFromDifferentProject
--- PASS: TestLaunchAgent_IssueFromDifferentProject (0.07s)
=== RUN   TestLaunchAgent_MultipleIssues
--- PASS: TestLaunchAgent_MultipleIssues (0.09s)
```
