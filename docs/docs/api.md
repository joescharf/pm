# REST API

The pm REST API is served alongside the web UI when running `pm serve`. All endpoints are under `/api/v1/`.

## Base URL

```
http://localhost:8080/api/v1/
```

Change the port with `pm serve --port <port>`.

## CORS

All responses include permissive CORS headers:

- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type`

## Response Format

All responses are JSON. Successful responses return the resource or array directly. Errors return:

```json
{
  "error": "error message describing what went wrong"
}
```

## Endpoints

### Projects

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/projects` | List all projects |
| `POST` | `/api/v1/projects` | Create a project |
| `GET` | `/api/v1/projects/{id}` | Get a project by ID |
| `PUT` | `/api/v1/projects/{id}` | Update a project |
| `DELETE` | `/api/v1/projects/{id}` | Delete a project |

**Query parameters for `GET /api/v1/projects`:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `group` | string | Filter by project group |

### Issues

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/issues` | List all issues |
| `GET` | `/api/v1/issues/{id}` | Get an issue by ID |
| `PUT` | `/api/v1/issues/{id}` | Update an issue |
| `DELETE` | `/api/v1/issues/{id}` | Delete an issue |
| `GET` | `/api/v1/projects/{id}/issues` | List issues for a project |
| `POST` | `/api/v1/projects/{id}/issues` | Create an issue under a project |

**Query parameters for `GET /api/v1/issues`:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | Filter by status (`open`, `in_progress`, `done`, `closed`) |
| `priority` | string | Filter by priority (`low`, `medium`, `high`) |
| `tag` | string | Filter by tag name |

**Defaults for `POST /api/v1/projects/{id}/issues`:**

When creating an issue, unspecified fields default to: `status: "open"`, `priority: "medium"`, `type: "feature"`.

### Status & Health

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/status` | Status overview for all projects |
| `GET` | `/api/v1/status/{id}` | Status for a single project |
| `GET` | `/api/v1/health/{id}` | Health score breakdown for a project |

**Status response shape:**

```json
{
  "project": { "id": "...", "name": "...", "path": "...", ... },
  "branch": "main",
  "isDirty": false,
  "openIssues": 3,
  "inProgressIssues": 1,
  "health": 82,
  "lastActivity": "2025-01-15T10:30:00Z"
}
```

**Health response shape:**

```json
{
  "Total": 82,
  "GitCleanliness": 15,
  "ActivityRecency": 22,
  "IssueHealth": 16,
  "ReleaseFreshness": 15,
  "BranchHygiene": 14
}
```

### Sessions

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/sessions` | List agent sessions |

**Query parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| `project_id` | string | Filter by project ID |

### Tags

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/v1/tags` | List all tags |

## Examples

### List all projects

```bash
curl http://localhost:8080/api/v1/projects
```

### Create an issue

```bash
curl -X POST http://localhost:8080/api/v1/projects/01J5ABCD1234EFGH5678IJKL/issues \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Add rate limiting",
    "priority": "high",
    "type": "feature"
  }'
```

### Get health score for a project

```bash
curl http://localhost:8080/api/v1/health/01J5ABCD1234EFGH5678IJKL
```

### Filter issues by status

```bash
curl "http://localhost:8080/api/v1/issues?status=open&priority=high"
```
