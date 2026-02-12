# PM Prompt

`pm` or `Program Manager` is a command-line tool that keeps track of AI-based app development projects, their status and progress, issues that need to be resolved, and aids in launching ai agents and worktrees to implement features and fix bugs. Overall goal is to help the user keep track of multiple projects, parallel ai development and not get overwhelmed by the complexity of managing multiple projects and their associated tasks. The key is that this `pm` app is essential for AI development workflows, so it should be tailored to the specific needs of AI developers and AI development and integrate with LLMs to provide insights and recommendations based on project metadata and issue tracking where appropriate. It should also support some level of autonomous behavior, such as automatically launching agents to work on issues or features. It should also be able to help enforce a degree of standardization between projects and repos. It will likely use some of the other tools I've built such as `wt - Worktree Manager`, `gsi - Go-Superinit` and others that have yet to be built.

## Capabilities (initial list, will likely evolve over time)

1. Track a list of projects and their directoeries on the local machine.
2. Capture a user-defined set of feature requests or issues that need to be resolved for each project. store these in a local database or file.
3. Retrieve metadata about the project from github, local directory, or other sources to help developers keep track of project status and progress. Such as:
  1. Latest release version, last commit date, and other relevant information about the project from github or local git repository.
  2. local branch status and number of open worktrees + their status
  3. Infer the latest commit message and its content to understand the current state of the project and what features or bugs are being worked on. Will want to integrate this with LLM to determine.
  4. Go version in go.mod and dependencies that might be outdated.
4. Facilitate creating worktrees and launching claude agents to implement or fix issues in the backlog. 
5. Provide a user-friendly interface to manage projects, view their status, and track progress on feature requests and issues.
