# Resolve Open PM Issues — Batch 2

*2026-02-13T00:12:57Z*

This batch resolves 3 open issues from the PM issue tracker plus commits prior in-flight work. The work spans the Go CLI, the React web UI, LLM integration, and project documentation.

## Commits

Five commits were created on `main`, each covering a distinct task:

```bash
git log --oneline -5
```

```output
4ebdea6 build: rebuild embedded UI with issues grouped by project
3b8e1c0 docs: update README with import command, MCP status, web dashboard section
c58a463 feat: add issue import from markdown with LLM extraction
c7d0787 feat: group issues page by project in web UI
2cad5f6 feat: detect project from cwd with git root fallback
```

## Commit 1: `2cad5f6` — Detect project from cwd with git root fallback

**In-flight work committed.** When `pm` is run with no subcommand from inside a tracked project directory (or any subdirectory of its git repo), it now auto-detects the project, refreshes metadata, and displays project info. Added `resolveProjectFromCwd()` git-root fallback in `cmd/issue.go` and `rootRun()` in `cmd/root.go`. Includes tests for `refreshProject()` logic.

```bash
git show --stat 2cad5f6 | tail -5
```

```output

 cmd/issue.go                |  13 +++
 cmd/project_refresh_test.go | 205 ++++++++++++++++++++++++++++++++++++++++++++
 cmd/root.go                 |  27 ++++++
 3 files changed, 245 insertions(+)
```

## Commit 2: `c7d0787` — Group issues page by project in web UI

**Closes issue `01KHA41ZXESM`.** Rewrote `ui/src/components/issues/issues-page.tsx` to fetch projects alongside issues, group them by `ProjectID` with `useMemo`, and render collapsible sections per project. Each section has a linked project name header, count badge, and chevron toggle. Existing filters (status, priority, tag) apply across all groups; empty groups auto-hide.

```bash
git show --stat c7d0787 | tail -3
```

```output

 ui/src/components/issues/issues-page.tsx | 163 +++++++++++++++++++++++--------
 1 file changed, 124 insertions(+), 39 deletions(-)
```

## Commit 3: `c58a463` — Add issue import from markdown with LLM extraction

**Closes issue `01KHA1VGY4K2`.** New `pm issue import <file>` command with two paths:

- **`--project <name>`**: Simple markdown parser extracts numbered/bulleted list items under `## Project <name>` headings and assigns them to the specified project. No LLM needed.
- **Without `--project`**: Sends the markdown content to Claude (via `anthropic-sdk-go`) to extract structured issues with project assignment, type, and priority inference.

Both paths support `--dry-run` for preview. New `internal/llm/` package wraps the Anthropic API. Viper config defaults added for `anthropic.api_key` and `anthropic.model`.

```bash
git show --stat c58a463 | tail -10
```

```output

 cmd/issue_import.go      | 275 +++++++++++++++++++++++++++++++++++++++++++++++
 cmd/issue_import_test.go |  85 +++++++++++++++
 cmd/root.go              |   2 +
 devnotes/quick-issues.md |  13 +++
 go.mod                   |   5 +
 go.sum                   |  12 +++
 internal/llm/llm.go      | 119 ++++++++++++++++++++
 internal/llm/llm_test.go |  48 +++++++++
 8 files changed, 559 insertions(+)
```

## Commit 4: `3b8e1c0` — Update README with import command, MCP status, web dashboard section

**Closes issue `01KHA1VKH9X8`.** Updated README.md to reflect current state: added issue import and auto-detect project to features list, changed MCP from "coming soon" to implemented, added `anthropic.*` config keys to the settings table, added Web Dashboard section, and updated command reference to include `project refresh` and `issue import`.

## Commit 5: `4ebdea6` — Rebuild embedded UI with issues grouped by project

Rebuilt the React UI (`bun run build`), embedded into `internal/ui/dist/`, and rebuilt the Go binary. Verified with rodney that the issues page displays grouped-by-project layout correctly.

## Verification

Full test suite passes with race detector:

```bash
go test -race -count=1 ./... 2>&1
```

```output
?   	github.com/joescharf/pm	[no test files]
ok  	github.com/joescharf/pm/cmd	2.664s
ok  	github.com/joescharf/pm/internal/api	1.911s
ok  	github.com/joescharf/pm/internal/git	1.656s
ok  	github.com/joescharf/pm/internal/golang	1.341s
ok  	github.com/joescharf/pm/internal/health	2.870s
ok  	github.com/joescharf/pm/internal/llm	2.104s
?   	github.com/joescharf/pm/internal/models	[no test files]
ok  	github.com/joescharf/pm/internal/output	2.281s
ok  	github.com/joescharf/pm/internal/standards	2.691s
ok  	github.com/joescharf/pm/internal/store	2.170s
?   	github.com/joescharf/pm/internal/ui	[no test files]
?   	github.com/joescharf/pm/internal/wt	[no test files]
```

Go vet clean:

```bash
go vet ./... 2>&1 && echo 'OK: no issues'
```

```output
OK: no issues
```

Issue import dry-run with `--project`:

```bash
bin/pm issue import devnotes/quick-issues.md --project pm --dry-run 2>&1
```

```output
#  PROJECT  TITLE                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        TYPE     PRIORITY  
1  pm       Dashboard: click on project name should view the project. Click on Branch should go to the github repo.                                                                                                                                                                                                                                                                                                                                                                                                                                                                      feature  medium    
2  pm       There should be a way to import issues from a markdown file. This is for quickly adding a backlog of issues to a project. Probably should use the LLM to determine which project the issues belong to based on the heading above the list of issues and/or the content of the issues.                                                                                                                                                                                                                                                                                        feature  medium    
3  pm       Please review the PROMPT.md to determine if there are any additional features or capabilities that should be added to the project. The README.md should be updated to reflect the capabilities and features of the project as outlined in the PROMPT.md. The README.md should also include a quick start guide and installation instructions for users to get up and running with the project quickly. The user-facing documentation in `docs/`should be clear and concise, providing users with the information they need to effectively use the project and its features.  feature  medium    
4  pm       There doesn't seem to be much capability for the agent feature in the web ui at least. I would think I should be able to select some issues and then click a button to launch a claude code session for the batch of issues. Claude could use the MCP server to get the project and issue information as well as keep the issues updated as it works on them.                                                                                                                                                                                                                feature  medium    
5  pm       There should be an easy way to install the MCP server for Claude Code. Perhaps this is tied to the command that starts the mcp server - it should do a quick check to see if the mcp server is installed in Claude code properly. Also the MCP server should probably start when the `pm serve` command is run, so that it's always running when the web dashboard is being used.                                                                                                                                                                                            feature  medium    
6  pm       refreshing the web ui, returns a blank page                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  feature  medium    
7  pm       Health score breakdown on the project info page isn't as useful as some of the other sections - i.e. issues. maybe we can move this or show it less prominently, or just move it to the end.                                                                                                                                                                                                                                                                                                                                                                                 feature  medium    
8  pm       Issues should be sorted by status and then priority. So open issues should be at the top, and then sorted by priority within the open issues. Closed issues should be at the bottom by date (or maybe just hidden by default).                                                                                                                                                                                                                                                                                                                                               feature  medium    
9  pm       It would be nice to have a way to quickly see which projects have open issues and how many open issues they have. Maybe this could be a column in the project list view, or maybe it could be a filter to show only projects with open issues. This would help users quickly identify which projects need attention and which ones are in good shape.                                                                                                                                                                                                                        feature  medium    
```

## Closed Issues

Three issues resolved and closed in this batch:

```bash
bin/pm issue list --all --status closed 2>&1 | head -5
```

```output
ID            PROJECT  TITLE                                                                           STATUS  PRIORITY  TYPE     GH #  
01KHA41ZXESM  pm       Issues page should break issues up by project                                   closed  medium    feature        
01KHA06QDMVV  pm       Dashboard: project name should view project, branch should link to github repo  closed  medium    feature        
01KHA031HEYB  pm       Click on version in dashboard should go to the release on github                closed  medium    feature        
01KHA01T5F2F  pm       Should be able to add issue from the projects show screen                       closed  medium    feature        
```

- **`01KHA41ZXESM`** — Issues page should break issues up by project *(commit c7d0787)*
- **`01KHA1VGY4K2`** — Import issues from a markdown file *(commit c58a463)*
- **`01KHA1VKH9X8`** — Review PROMPT.md and update README and docs *(commit 3b8e1c0)*
