CREATE TABLE IF NOT EXISTS mobile_build_jobs (
    id VARCHAR(64) PRIMARY KEY,
    project_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    platform VARCHAR(32) NOT NULL,
    profile VARCHAR(32) NOT NULL,
    release_level VARCHAR(64) NOT NULL,
    status VARCHAR(64) NOT NULL,
    provider VARCHAR(64),
    provider_build_id VARCHAR(128),
    artifact_url VARCHAR(1024),
    app_version VARCHAR(64),
    build_number VARCHAR(64),
    version_code BIGINT,
    commit_ref VARCHAR(255),
    failure_type VARCHAR(64),
    failure_message TEXT,
    logs JSONB,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_mobile_build_jobs_project_id ON mobile_build_jobs(project_id);
CREATE INDEX IF NOT EXISTS idx_mobile_build_jobs_user_id ON mobile_build_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_mobile_build_jobs_platform ON mobile_build_jobs(platform);
CREATE INDEX IF NOT EXISTS idx_mobile_build_jobs_status ON mobile_build_jobs(status);
CREATE INDEX IF NOT EXISTS idx_mobile_build_jobs_provider_build_id ON mobile_build_jobs(provider_build_id);
