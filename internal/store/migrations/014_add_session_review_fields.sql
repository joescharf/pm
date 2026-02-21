ALTER TABLE agent_sessions ADD COLUMN session_type TEXT NOT NULL DEFAULT 'implementation';
ALTER TABLE agent_sessions ADD COLUMN review_attempt INTEGER NOT NULL DEFAULT 0;
