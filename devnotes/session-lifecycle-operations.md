# Session Lifecycle Operations

*2026-02-20T03:12:54Z*

Add sync, merge, delete-worktree, and discover operations to session management. Integrates wt v0.6.0 library for git worktree operations via a repo-bound adapter. Backend adds new DB fields (conflict_state, last_sync_at, last_error, discovered), REST API endpoints, MCP tools, and CLI commands. Frontend adds status filter tabs, project filter, discover button, sync/merge/delete dialogs with confirmation, conflict and discovered badges, and 30s auto-refresh polling.

```bash
go test -race -count=1 ./... 2>&1
```

```output
?   	github.com/joescharf/pm	[no test files]
ok  	github.com/joescharf/pm/cmd	2.255s
ok  	github.com/joescharf/pm/internal/agent	1.927s
ok  	github.com/joescharf/pm/internal/api	3.909s
ok  	github.com/joescharf/pm/internal/daemon	3.205s
ok  	github.com/joescharf/pm/internal/git	1.955s
ok  	github.com/joescharf/pm/internal/golang	1.737s
ok  	github.com/joescharf/pm/internal/health	2.782s
ok  	github.com/joescharf/pm/internal/llm	2.614s
ok  	github.com/joescharf/pm/internal/mcp	2.448s
?   	github.com/joescharf/pm/internal/models	[no test files]
ok  	github.com/joescharf/pm/internal/output	2.133s
?   	github.com/joescharf/pm/internal/refresh	[no test files]
?   	github.com/joescharf/pm/internal/sessions	[no test files]
ok  	github.com/joescharf/pm/internal/standards	1.793s
ok  	github.com/joescharf/pm/internal/store	3.141s
?   	github.com/joescharf/pm/internal/ui	[no test files]
?   	github.com/joescharf/pm/internal/wt	[no test files]
```
