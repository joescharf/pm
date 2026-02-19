# Release v0.3.0

*2026-02-19T15:30:29Z*

## Release v0.3.0

### Features
- Add issue review system: table, model, store methods, MCP tools (pm_prepare_review, pm_save_review), REST API endpoints, CLI command, and review history UI
- Add build_cmd, serve_cmd, serve_port to project model with pm_update_project MCP tool
- Add Diff, DiffStat, DiffNameOnly to git client
- Promote import to top-level command with dedup and multi-project selection

### Fixes
- Handle unchecked json.Encode return values in review API handlers (CI lint fix)
- Filter placeholder titles during issue import

### Docs
- Update CLAUDE.md with review MCP tools and lifecycle
- Add issue review feature design and implementation plan
