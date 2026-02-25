-- 000003_budget_and_spend.up.sql
-- Adds spend tracking, budget caps, BYOK project scoping, and protected paths

-- S2: Immutable spend event log
CREATE TABLE IF NOT EXISTS spend_events (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id INTEGER NOT NULL REFERENCES users(id),
    project_id INTEGER REFERENCES projects(id),
    build_id VARCHAR(64),
    agent_id VARCHAR(64),
    agent_role VARCHAR(20),
    provider VARCHAR(20) NOT NULL,
    model VARCHAR(100) NOT NULL,
    capability VARCHAR(50),
    is_byok BOOLEAN DEFAULT false,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    raw_cost NUMERIC(12,6) NOT NULL DEFAULT 0,
    billed_cost NUMERIC(12,6) NOT NULL DEFAULT 0,
    power_mode VARCHAR(10),
    duration_ms INTEGER DEFAULT 0,
    status VARCHAR(20) DEFAULT 'success',
    target_file VARCHAR(500),
    day_key DATE NOT NULL,
    month_key VARCHAR(7) NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_spend_user_day ON spend_events(user_id, day_key);
CREATE INDEX IF NOT EXISTS idx_spend_user_month ON spend_events(user_id, month_key);
CREATE INDEX IF NOT EXISTS idx_spend_build ON spend_events(build_id) WHERE build_id IS NOT NULL;

-- S1: Budget caps
CREATE TABLE IF NOT EXISTS budget_caps (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id INTEGER NOT NULL REFERENCES users(id),
    cap_type VARCHAR(20) NOT NULL,
    project_id INTEGER REFERENCES projects(id),
    limit_usd NUMERIC(12,6) NOT NULL,
    action VARCHAR(10) NOT NULL DEFAULT 'stop',
    is_active BOOLEAN DEFAULT true,
    UNIQUE(user_id, cap_type, project_id)
);

-- A2: BYOK project scoping + rotation
ALTER TABLE user_api_keys ADD COLUMN IF NOT EXISTS project_id INTEGER REFERENCES projects(id);
ALTER TABLE user_api_keys ADD COLUMN IF NOT EXISTS last_rotated_at TIMESTAMPTZ;
ALTER TABLE user_api_keys ADD COLUMN IF NOT EXISTS rotation_reminder_days INTEGER DEFAULT 90;

-- A3: Protected paths
ALTER TABLE projects ADD COLUMN IF NOT EXISTS protected_paths TEXT DEFAULT '[]';
