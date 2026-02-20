-- Add fields to support session operations (sync, merge, conflict tracking, discovery)
ALTER TABLE agent_sessions ADD COLUMN last_error TEXT NOT NULL DEFAULT '';
ALTER TABLE agent_sessions ADD COLUMN last_sync_at DATETIME;
ALTER TABLE agent_sessions ADD COLUMN conflict_state TEXT NOT NULL DEFAULT 'none';
ALTER TABLE agent_sessions ADD COLUMN conflict_files TEXT NOT NULL DEFAULT '[]';
ALTER TABLE agent_sessions ADD COLUMN discovered BOOLEAN NOT NULL DEFAULT 0;
