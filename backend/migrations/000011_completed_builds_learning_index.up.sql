CREATE INDEX IF NOT EXISTS idx_completed_builds_user_updated
    ON completed_builds(user_id, updated_at DESC);
