# Inline Review Creation UI

*2026-02-21T00:32:23Z*

Added an inline review creation form to the issue detail page in the web UI. Previously, reviews could only be submitted via MCP or CLI. The form includes verdict selection (pass/fail), summary text, four category ratings (code quality, requirements, test coverage, UI/UX), and failure reasons input. The REST API was also enhanced to transition issue status on review creation (pass->closed, fail->in_progress), matching the existing MCP pm_save_review behavior.

```bash
go test -count=1 ./internal/api/ ./internal/store/ ./internal/mcp/ 2>&1
```

```output
ok  	github.com/joescharf/pm/internal/api	0.581s
ok  	github.com/joescharf/pm/internal/store	0.401s
ok  	github.com/joescharf/pm/internal/mcp	0.642s
```
