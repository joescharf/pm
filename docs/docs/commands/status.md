# pm status

Show a cross-project status dashboard or detailed status for a single project.

```bash
pm status [project] [flags]
```

## Overview Mode (no arguments)

Without arguments, shows a summary table across all tracked projects.

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--stale` | bool | `false` | Show only stale projects (no activity in 7+ days) |
| `--group` | string | `""` | Filter by project group |

**Output columns:** Project, Branch, Status (dirty/clean), Issues (open/in-progress), Health (0-100, colored), Activity (relative time)

**Examples:**

```bash
# All projects
pm status

# Only stale projects
pm status --stale

# Filter by group
pm status --group backend
```

## Single-Project Mode

With a project name argument, shows detailed information for that project (equivalent to `pm project show`).

```bash
pm status my-api
```

## Health Score

The health score is a composite 0-100 score computed from 5 components. Higher is better.

### Components

| Component | Max Points | Scoring |
|-----------|-----------|---------|
| **Git Cleanliness** | 15 | Clean repo = 15 pts; dirty repo = 5 pts |
| **Activity Recency** | 25 | Based on days since last commit (see recency curve below) |
| **Issue Health** | 20 | No issues = 20 pts; otherwise penalized by open/total ratio |
| **Release Freshness** | 20 | Based on days since last release; no releases = 5 pts |
| **Branch Hygiene** | 20 | Fewer branches = more points (see branch scoring below) |

### Activity Recency Curve

| Days Since Last Commit | Percentage of Max Points |
|------------------------|--------------------------|
| 0-1 | 100% |
| 2-3 | 90% |
| 4-7 | 75% |
| 8-14 | 60% |
| 15-30 | 40% |
| 31-90 | 20% |
| 90+ | 10% |

### Branch Hygiene Scoring

| Branch Count | Percentage of Max Points |
|-------------|--------------------------|
| 1-3 | 100% |
| 4-5 | 80% |
| 6-10 | 60% |
| 11-20 | 40% |
| 20+ | 20% |

### Issue Health Scoring

If there are no issues, the project gets full points (20). Otherwise:

```
points = maxPoints * (1 - openRatio * 0.8)
```

Where `openRatio = (open + in_progress) / total_issues`. A project with all issues closed scores 20 pts; one with all issues open scores 4 pts.

### Health Color Coding

| Score Range | Color |
|-------------|-------|
| 80-100 | Green |
| 50-79 | Yellow |
| 0-49 | Red |
