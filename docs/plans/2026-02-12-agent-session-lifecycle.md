# Agent Session Lifecycle & Worktree Management

## Problem

Agent sessions created by `pm agent launch` have no way to be closed, paused, or cleaned up. Sessions stay as `running` forever. There's no visibility into which worktrees are associated with which issues, and no way to manage the lifecycle from the CLI, MCP, or web UI.

## Design

### Data Model

**Session statuses** change from `running/completed/failed/abandoned` to:

| Status | Meaning |
|---|---|
| `active` | Claude window is open, actively working |
| `idle` | Worktree exists, no active Claude session |
| `completed` | Work done, worktree may or may not exist |
| `abandoned` | Gave up, worktree may or may not exist |

**Migration**: Rename `running` → `active` in existing rows. Drop `failed` (merge into `abandoned`).

**Issue status cascading**:

| Session transition | Issue status change |
|---|---|
| launch → `active` | `open` → `in_progress` (already exists) |
| `active` → `idle` | no change (stays `in_progress`) |
| `idle` → `active` (resume) | no change (stays `in_progress`) |
| → `completed` | `in_progress` → `done` |
| → `abandoned` | `in_progress` → `open` |

### CLI Commands

#### New: `pm agent close`

```
pm agent close [session_id]            # → idle (default)
pm agent close [session_id] --done     # → completed, issues → done
pm agent close [session_id] --abandon  # → abandoned, issues → open
pm agent close --project <name>        # list sessions for project, pick one
```

**Context detection** (when no session_id given):

1. Check if cwd is a worktree → match `WorktreePath` → close that session
2. Check if cwd is a main project dir → list active/idle sessions → prompt user to pick
3. Fall back to `--project` flag

#### Enhanced: `pm agent launch`

When launching with a branch that already has an `idle` session:

- Reuse the existing session, set status back to `active`
- Skip worktree creation (it already exists)
- Output the `cd && claude "..."` command as usual

No new `pm agent resume` command needed — launch handles it.

### MCP Tool

**New: `pm_close_agent`**

- Params: `session_id` (required), `status` (optional, default `idle`, one of `idle`/`completed`/`abandoned`)
- Returns: updated session JSON with issue status changes noted
- Handles issue status cascading server-side

### API Endpoint

**New: `POST /api/v1/agent/close`**

Request:
```json
{
  "session_id": "...",
  "status": "idle"
}
```

Response: updated session plus any issue status changes.

### Worktree Sync (Reconciliation)

On `pm agent list`, `pm status`, and API calls that return sessions, lazily check whether worktree directories still exist on disk.

- For each `active` or `idle` session with a `WorktreePath`: check if directory exists (`os.Stat`)
- If gone: set status to `abandoned`, linked issues `in_progress` → `open`
- Log a warning when orphaned sessions are cleaned up

This keeps data clean without a manual cleanup step.

### Web UI

**Project detail page** — new "Worktrees" section:

- Table columns: Branch, Issues, Status (`active`/`idle`), Started, Actions
- Issues shown as linked badges (short IDs + titles)
- Actions per row:
  - `idle` → Resume, Done, Abandon buttons
  - `active` → Close (→ idle), Done, Abandon buttons
- Resume reuses the existing `AgentLaunchDialog` to show the copy-paste command

**Issue detail/list** — worktree association:

- If an issue has an active/idle session, show branch name with status badge
- Clicking navigates to the project worktrees section

**Agent sessions list** — updated status colors:

- Green = active, Yellow = idle, Gray = completed/abandoned

### Launch Command Prompt

When launching (or resuming) a session with an issue, the generated command includes:

```
cd <worktree_path> && claude "Use pm MCP tools to look up issue <short_id> and implement it. Update the issue status when complete."
```

This tells Claude to use MCP tools to fetch issue details rather than requiring the user to paste context manually. (Already implemented.)

## Implementation Order

1. Data model: migration to rename `running` → `active`, update status constants
2. Store/models: update `SessionStatus` constants, add helper for issue cascading
3. CLI: `pm agent close` with context detection
4. MCP: `pm_close_agent` tool
5. API: `POST /api/v1/agent/close` endpoint
6. Launch enhancement: detect idle sessions for reuse
7. Worktree sync: lazy reconciliation on list/status calls
8. Web UI: worktrees section on project detail, issue association display
