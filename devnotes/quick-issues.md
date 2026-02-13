# Quick Issues

## Project pm

1. Sessions page doesn't have a column for the session's project, which makes it hard to find sessions for a specific project.
2. Sessions page doens't allow clicking on the session to get more details about the session, such as the worktree and state of the branch (# commits, last commit message, ahead/behind main, etc.)
3. There should be status somewhere that indicates whether github pages is configured for hosting the user facing documentation, and allowing a user to click on the status to get to the documentation for the project.
4. When we refresh our projects, we should check the origin repo for any updated About information and update the project information in pm accordingly.
5. When we refresh our projects, we should check the origin repo for # of branches and store this in the metadata for the project and display it in project details section of the UI.
6. Whe we restart the pm application and have a session that was active before the restart, we should check the worktree to see if it is still available, and if so we should set the session to paused, instead of abandoned which is the current behavior. If the worktree is not available, then we can set the session to abandoned.
7. We should add a "last active" timestamp to sessions, and update this timestamp whenever the session is active (e.g. when we check the status of the session, or when we interact with the session in any way). This will allow us to sort sessions by last active and identify which sessions are active and which are stale.
8. We should add a "last commit
