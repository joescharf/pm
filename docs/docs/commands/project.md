# pm project

Manage tracked projects.

```
pm project add <path>           Add a project to tracking
pm project remove <name-or-path>  Remove a project (alias: rm)
pm project list                 List tracked projects (alias: ls)
pm project show <name>          Show detailed project information
pm project scan <directory>     Auto-discover git repos in a directory
```

## project add

Add a project directory to pm tracking. Use `.` for the current directory.

```bash
pm project add <path> [flags]
```

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | string | `""` | Override project name (default: directory basename) |
| `--group` | string | `""` | Project group name for organization |

**Behavior:**

- Resolves the path to an absolute path
- Auto-detects the programming language
- Picks up the git remote URL if available
- The project name defaults to the directory basename unless `--name` is specified

**Examples:**

```bash
# Add current directory
pm project add .

# Add with a custom name and group
pm project add ~/code/my-api --name api-service --group backend

# Dry-run to preview
pm project add ~/code/my-api --dry-run
```

## project remove

Remove a project from tracking. This does not delete any files on disk.

```bash
pm project remove <name-or-path>
```

Aliases: `rm`

The argument can be either the project name or its absolute path.

**Examples:**

```bash
pm project remove my-api
pm project rm ~/code/my-api
```

## project list

List all tracked projects.

```bash
pm project list [flags]
```

Aliases: `ls`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--group` | string | `""` | Filter by group name |

**Output columns:** Name, Path, Language, Group

**Examples:**

```bash
# List all projects
pm project list

# Filter by group
pm project ls --group backend
```

## project show

Show detailed information about a project.

```bash
pm project show <name>
```

Displays:

- **Header:** name, path, description, group, language, remote URL
- **Git info:** current branch, dirty/clean status, last commit hash and message, activity age
- **Worktree count** (excluding the main worktree)
- **Go-specific info** (if applicable): Go version, module path
- **Issue counts:** open and in-progress
- **GitHub release info** (if remote URL is set): latest release tag and date

**Example:**

```bash
pm project show my-api
```

## project scan

Auto-discover git repositories in a directory and add them to tracking.

```bash
pm project scan <directory>
```

Scans the top-level entries in the given directory. For each subdirectory that is a git repository root and not already tracked, it adds the project automatically.

Dotfile directories (starting with `.`) are skipped.

**Examples:**

```bash
# Scan ~/code for all git repos
pm project scan ~/code

# Preview what would be added
pm project scan ~/code --dry-run
```
