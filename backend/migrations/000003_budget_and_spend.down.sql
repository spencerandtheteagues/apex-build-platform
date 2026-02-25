-- 000003_budget_and_spend.down.sql
-- Rollback spend tracking, budget caps, BYOK project scoping, and protected paths

ALTER TABLE projects DROP COLUMN IF EXISTS protected_paths;
ALTER TABLE user_api_keys DROP COLUMN IF EXISTS rotation_reminder_days;
ALTER TABLE user_api_keys DROP COLUMN IF EXISTS last_rotated_at;
ALTER TABLE user_api_keys DROP COLUMN IF EXISTS project_id;

DROP TABLE IF EXISTS budget_caps;
DROP TABLE IF EXISTS spend_events;
