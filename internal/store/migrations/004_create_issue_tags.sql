CREATE TABLE IF NOT EXISTS issue_tags (
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    tag_id   TEXT NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, tag_id)
);
