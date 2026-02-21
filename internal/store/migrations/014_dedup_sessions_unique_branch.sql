-- Deduplicate: for each (project_id, branch) with multiple active/idle sessions,
-- keep the one with the latest started_at, abandon the rest.
UPDATE agent_sessions
SET status = 'abandoned',
    ended_at = datetime('now')
WHERE id IN (
    SELECT a.id
    FROM agent_sessions a
    INNER JOIN (
        SELECT project_id, branch, MAX(started_at) AS max_started
        FROM agent_sessions
        WHERE status IN ('active', 'idle')
        GROUP BY project_id, branch
        HAVING COUNT(*) > 1
    ) dups ON a.project_id = dups.project_id
         AND a.branch = dups.branch
         AND a.status IN ('active', 'idle')
         AND a.started_at < dups.max_started
);

-- Safety net: prevent future duplicates for same project+branch when active/idle
CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_sessions_active_branch
ON agent_sessions(project_id, branch)
WHERE status IN ('active', 'idle');
