ALTER TABLE agent_sessions ADD COLUMN last_commit_hash TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_sessions ADD COLUMN last_commit_message TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_sessions ADD COLUMN last_active_at DATETIME;
