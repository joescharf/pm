CREATE TABLE IF NOT EXISTS issue_reviews (
    id TEXT PRIMARY KEY,
    issue_id TEXT NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    session_id TEXT DEFAULT '',
    verdict TEXT NOT NULL,
    summary TEXT NOT NULL,
    code_quality TEXT DEFAULT '',
    requirements_match TEXT DEFAULT '',
    test_coverage TEXT DEFAULT '',
    ui_ux TEXT DEFAULT '',
    failure_reasons TEXT DEFAULT '[]',
    diff_stats TEXT DEFAULT '',
    reviewed_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_issue_reviews_issue_id ON issue_reviews(issue_id);
