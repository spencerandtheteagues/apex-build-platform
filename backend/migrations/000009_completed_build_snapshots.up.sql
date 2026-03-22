CREATE TABLE IF NOT EXISTS completed_builds (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    build_id VARCHAR(64) NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(id),
    project_id INTEGER REFERENCES projects(id),
    project_name VARCHAR(255),
    description TEXT,
    status VARCHAR(20),
    mode VARCHAR(10),
    power_mode VARCHAR(10),
    tech_stack TEXT,
    files_json TEXT,
    agents_json TEXT,
    tasks_json TEXT,
    checkpoints_json TEXT,
    state_json TEXT,
    activity_json TEXT,
    interaction_json TEXT NOT NULL DEFAULT '',
    files_count INTEGER DEFAULT 0,
    total_cost DECIMAL(12, 6) DEFAULT 0.0,
    progress INTEGER DEFAULT 100,
    duration_ms BIGINT DEFAULT 0,
    error TEXT,
    completed_at TIMESTAMP WITH TIME ZONE
);

ALTER TABLE IF EXISTS completed_builds
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE,
    ADD COLUMN IF NOT EXISTS build_id VARCHAR(64),
    ADD COLUMN IF NOT EXISTS user_id INTEGER REFERENCES users(id),
    ADD COLUMN IF NOT EXISTS project_id INTEGER REFERENCES projects(id),
    ADD COLUMN IF NOT EXISTS project_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS status VARCHAR(20),
    ADD COLUMN IF NOT EXISTS mode VARCHAR(10),
    ADD COLUMN IF NOT EXISTS power_mode VARCHAR(10),
    ADD COLUMN IF NOT EXISTS tech_stack TEXT,
    ADD COLUMN IF NOT EXISTS files_json TEXT,
    ADD COLUMN IF NOT EXISTS agents_json TEXT,
    ADD COLUMN IF NOT EXISTS tasks_json TEXT,
    ADD COLUMN IF NOT EXISTS checkpoints_json TEXT,
    ADD COLUMN IF NOT EXISTS state_json TEXT,
    ADD COLUMN IF NOT EXISTS activity_json TEXT,
    ADD COLUMN IF NOT EXISTS interaction_json TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS files_count INTEGER DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_cost DECIMAL(12, 6) DEFAULT 0.0,
    ADD COLUMN IF NOT EXISTS progress INTEGER DEFAULT 100,
    ADD COLUMN IF NOT EXISTS duration_ms BIGINT DEFAULT 0,
    ADD COLUMN IF NOT EXISTS error TEXT,
    ADD COLUMN IF NOT EXISTS completed_at TIMESTAMP WITH TIME ZONE;

ALTER TABLE IF EXISTS completed_builds
    ALTER COLUMN build_id SET NOT NULL,
    ALTER COLUMN user_id SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_completed_builds_build_id ON completed_builds(build_id);
CREATE INDEX IF NOT EXISTS idx_completed_builds_user_id ON completed_builds(user_id);
CREATE INDEX IF NOT EXISTS idx_completed_builds_project_id ON completed_builds(project_id);
CREATE INDEX IF NOT EXISTS idx_completed_builds_status ON completed_builds(status);
CREATE INDEX IF NOT EXISTS idx_completed_builds_created_at ON completed_builds(created_at);
CREATE INDEX IF NOT EXISTS idx_completed_builds_deleted_at ON completed_builds(deleted_at);
