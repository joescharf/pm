# Tools available for AI development use:

## pm - Project Manager

- Has a binary `pm` that can be used to manage projects, sessions, and issues.
- Has a mcp server that can be used to manage projects, sessions, and issues via API.

- [Repo](https://github.com/joescharf/pm)
- [Docs](https://joescharf.github.io/pm/)

## wt - Worktree manager for parallel agentic development

- Has a binary `wt` that can be used to manage worktrees for parallel agentic development.
- Should be run in the pwd of a project or the project's worktree to manage worktrees for that project.

- [Repo](https://github.com/joescharf/wt)
- [Docs](https://joescharf.github.io/wt/)

## gsi - Go Superinit

- Has a binary `gsi` that can be used to quickly initialize new Go projects with a standard structure and configuration.

- [Repo](https://github.com/joescharf/gsi)

## Showboat - AI agent tool to create markdown documents describing features built

- Has a binary `showboat` if installed directly or `uvx showboat` if not installed on the local system.
- [Repo](https://github.com/simonw/showboat)

### Pre-release agent prompt:

Run "uvx showboat --help" and then use showboat to create a document describing the release you just built and output it to a new markdown file in the `devnotes/` directory named after the release and a short slug describing the release

### Pre feature(s) commit agent prompt:

Run "uvx showboat --help" and then use showboat to create a document describing the feature(s) you just built and output it to a new markdown file in the `devnotes/` directory named with a short slug describing the feature.

## Rodney - AI agent CLI tool to interact with the web from the command line via Chrome automation

- Has a binary `rodney` if installed directly or `uvx rodney` if not installed on the local system.
- [Repo](https://github.com/simonw/rodney)

Run `uvx rodney --help` to see the available commands and options for interacting with the web via Chrome automation. You can use rodney to perform tasks such as validating web pages, taking screenshots, and more, all from the command line.

### Use case:

Use rodney to validate web pages you just built to ensure they have the expected content, structure, and functionality. For example, you could use rodney to check that a new feature you just built is present in a build of the web app, and if not, you can iterate on the feature until it is present and correct.

## shot-scraper - A command-line utility for taking automated screenshots of websites

- Invoke with `uvx shot-scraper`

Use shot-scraper to take automated screenshots of websites, which can be useful for visual regression testing, monitoring website changes, or ensuring that documentation has the latest screenshots.

- [Repo](https://github.com/simonw/shot-scraper)
- [Docs](https://shot-scraper.datasette.io/)

## When finishing a feature or release:

### Agent Prompt: 

Update readme and user facing documentation, Run "uvx showboat --help" and then use showboat to create a document describing the feature(s) you just built and output it to a new markdown file in the `devnotes/` directory named with a short slug describing the feature. Commit the changes, close any related issues in the `pm` tracker
