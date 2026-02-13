# Quick Issues

## Project pm

1. Dashboard: click on project name should view the project. Click on Branch should go to the github repo.
2. There should be a way to import issues from a markdown file. This is for quickly adding a backlog of issues to a project. Probably should use the LLM to determine which project the issues belong to based on the heading above the list of issues and/or the content of the issues.
3. Please review the PROMPT.md to determine if there are any additional features or capabilities that should be added to the project. The README.md should be updated to reflect the capabilities and features of the project as outlined in the PROMPT.md. The README.md should also include a quick start guide and installation instructions for users to get up and running with the project quickly. The user-facing documentation in `docs/`should be clear and concise, providing users with the information they need to effectively use the project and its features.
4. There doesn't seem to be much capability for the agent feature in the web ui at least. I would think I should be able to select some issues and then click a button to launch a claude code session for the batch of issues. Claude could use the MCP server to get the project and issue information as well as keep the issues updated as it works on them.
5. There should be an easy way to install the MCP server for Claude Code. Perhaps this is tied to the command that starts the mcp server - it should do a quick check to see if the mcp server is installed in Claude code properly. Also the MCP server should probably start when the `pm serve` command is run, so that it's always running when the web dashboard is being used.
6. refreshing the web ui, returns a blank page
7. Health score breakdown on the project info page isn't as useful as some of the other sections - i.e. issues. maybe we can move this or show it less prominently, or just move it to the end.
8. Issues should be sorted by status and then priority. So open issues should be at the top, and then sorted by priority within the open issues. Closed issues should be at the bottom by date (or maybe just hidden by default).
9. It would be nice to have a way to quickly see which projects have open issues and how many open issues they have. Maybe this could be a column in the project list view, or maybe it could be a filter to show only projects with open issues. This would help users quickly identify which projects need attention and which ones are in good shape.
