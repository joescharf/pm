CREATE TABLE IF NOT EXISTS agent_sessions (
    id            TEXT PRIMARY KEY,
    project_id    TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    issue_id      TEXT NOT NULL DEFAULT '',
    branch        TEXT NOT NULL DEFAULT '',
    worktree_path TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'running',
    outcome       TEXT NOT NULL DEFAULT '',
    commit_count  INTEGER NOT NULL DEFAULT 0,
    started_at    DATETIME NOT NULL DEFAULT (datetime('now')),
    ended_at      DATETIME
);

CREATE INDEX IF NOT EXISTS idx_agent_sessions_project_id ON agent_sessions(project_id);
