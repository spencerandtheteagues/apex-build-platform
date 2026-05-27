-- 000015_expand_mode_power_mode_columns.down.sql
-- Reverts mode and power_mode columns back to VARCHAR(10)

-- Note: if any rows contain values longer than 10 chars, this will fail.
-- For a safe rollback, truncate data first or skip this migration.

-- completed_builds table
ALTER TABLE IF EXISTS completed_builds
    ALTER COLUMN mode TYPE VARCHAR(10),
    ALTER COLUMN power_mode TYPE VARCHAR(10);

-- spend_events table
ALTER TABLE IF EXISTS spend_events
    ALTER COLUMN power_mode TYPE VARCHAR(10);
