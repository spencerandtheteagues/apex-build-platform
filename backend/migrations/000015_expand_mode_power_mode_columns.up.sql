-- 000015_expand_mode_power_mode_columns.up.sql
-- Expands mode and power_mode columns from VARCHAR(10) to VARCHAR(20)
-- to support "frontend_preview_only" delivery mode and future values.

-- completed_builds table
ALTER TABLE IF EXISTS completed_builds
    ALTER COLUMN mode TYPE VARCHAR(20),
    ALTER COLUMN power_mode TYPE VARCHAR(20);

-- spend_events table
ALTER TABLE IF EXISTS spend_events
    ALTER COLUMN power_mode TYPE VARCHAR(20);
