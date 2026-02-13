-- Rename running -> active, failed -> abandoned in existing rows.
UPDATE agent_sessions SET status = 'active' WHERE status = 'running';
UPDATE agent_sessions SET status = 'abandoned' WHERE status = 'failed';
