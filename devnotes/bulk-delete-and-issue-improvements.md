# Bulk Issue Delete, Import Body Fix, and Build Ordering

*2026-02-18T17:08:10Z by Showboat 0.6.0*
<!-- showboat-id: e784d3ca-5807-4b60-9ab4-1af40eb9bd25 -->

Added bulk issue deletion to the web UI issues page with confirmation dialog, using the existing checkbox selection and floating action bar pattern. Fixed SQLite 'database is locked' errors during bulk operations by adding PRAGMA busy_timeout=5000 and making deletes sequential. Fixed issue import to have the LLM return per-issue body text instead of dumping the entire file into every issue's Body field. Added Body/Raw section to the issue detail page. Fixed Makefile build ordering so ui-build -> ui-embed -> go build runs sequentially. Enhanced MCP pm_list_issues tool description to clarify description vs body fields for AI consumers.
