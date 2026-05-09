ALTER TABLE IF EXISTS projects
    ADD COLUMN IF NOT EXISTS target_platform VARCHAR(32) DEFAULT 'web',
    ADD COLUMN IF NOT EXISTS mobile_platforms JSONB,
    ADD COLUMN IF NOT EXISTS mobile_framework VARCHAR(64),
    ADD COLUMN IF NOT EXISTS mobile_release_level VARCHAR(64),
    ADD COLUMN IF NOT EXISTS mobile_capabilities JSONB,
    ADD COLUMN IF NOT EXISTS mobile_dependency_policy VARCHAR(64),
    ADD COLUMN IF NOT EXISTS mobile_preview_status VARCHAR(64),
    ADD COLUMN IF NOT EXISTS mobile_build_status VARCHAR(64),
    ADD COLUMN IF NOT EXISTS mobile_store_readiness_status VARCHAR(64),
    ADD COLUMN IF NOT EXISTS generated_backend_url VARCHAR(512),
    ADD COLUMN IF NOT EXISTS generated_mobile_client_path VARCHAR(512),
    ADD COLUMN IF NOT EXISTS eas_project_id VARCHAR(128),
    ADD COLUMN IF NOT EXISTS android_package VARCHAR(255),
    ADD COLUMN IF NOT EXISTS ios_bundle_identifier VARCHAR(255),
    ADD COLUMN IF NOT EXISTS app_display_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS app_version VARCHAR(64),
    ADD COLUMN IF NOT EXISTS build_number VARCHAR(64),
    ADD COLUMN IF NOT EXISTS version_code INTEGER,
    ADD COLUMN IF NOT EXISTS icon_asset_ref VARCHAR(512),
    ADD COLUMN IF NOT EXISTS splash_asset_ref VARCHAR(512),
    ADD COLUMN IF NOT EXISTS permission_manifest JSONB,
    ADD COLUMN IF NOT EXISTS store_metadata_draft_ref VARCHAR(512),
    ADD COLUMN IF NOT EXISTS mobile_metadata JSONB;

ALTER TABLE IF EXISTS completed_builds
    ADD COLUMN IF NOT EXISTS target_platform VARCHAR(32) DEFAULT 'web',
    ADD COLUMN IF NOT EXISTS mobile_platforms JSONB,
    ADD COLUMN IF NOT EXISTS mobile_framework VARCHAR(64),
    ADD COLUMN IF NOT EXISTS mobile_release_level VARCHAR(64),
    ADD COLUMN IF NOT EXISTS mobile_capabilities JSONB,
    ADD COLUMN IF NOT EXISTS android_package VARCHAR(255),
    ADD COLUMN IF NOT EXISTS ios_bundle_identifier VARCHAR(255),
    ADD COLUMN IF NOT EXISTS app_display_name VARCHAR(255),
    ADD COLUMN IF NOT EXISTS app_version VARCHAR(64),
    ADD COLUMN IF NOT EXISTS build_number VARCHAR(64),
    ADD COLUMN IF NOT EXISTS version_code INTEGER,
    ADD COLUMN IF NOT EXISTS mobile_spec_json TEXT,
    ADD COLUMN IF NOT EXISTS mobile_metadata JSONB;

CREATE INDEX IF NOT EXISTS idx_projects_target_platform ON projects(target_platform);
CREATE INDEX IF NOT EXISTS idx_projects_android_package ON projects(android_package);
CREATE INDEX IF NOT EXISTS idx_projects_ios_bundle_identifier ON projects(ios_bundle_identifier);
CREATE INDEX IF NOT EXISTS idx_completed_builds_target_platform ON completed_builds(target_platform);

ALTER TABLE IF EXISTS projects
    ALTER COLUMN target_platform SET DEFAULT 'web';

ALTER TABLE IF EXISTS completed_builds
    ALTER COLUMN target_platform SET DEFAULT 'web';

UPDATE projects
SET target_platform = 'web'
WHERE target_platform IS NULL OR BTRIM(target_platform) = '';

UPDATE completed_builds
SET target_platform = 'web'
WHERE target_platform IS NULL OR BTRIM(target_platform) = '';

CREATE TABLE IF NOT EXISTS mobile_submission_jobs (
    id VARCHAR(64) PRIMARY KEY,
    project_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    build_id VARCHAR(64) NOT NULL,
    platform VARCHAR(32) NOT NULL,
    status VARCHAR(64) NOT NULL,
    provider VARCHAR(64),
    provider_submission_id VARCHAR(128),
    track VARCHAR(64),
    artifact_url VARCHAR(1024),
    failure_type VARCHAR(64),
    failure_message TEXT,
    logs JSONB,
    created_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_mobile_submission_jobs_project_id ON mobile_submission_jobs(project_id);
CREATE INDEX IF NOT EXISTS idx_mobile_submission_jobs_user_id ON mobile_submission_jobs(user_id);
CREATE INDEX IF NOT EXISTS idx_mobile_submission_jobs_build_id ON mobile_submission_jobs(build_id);
CREATE INDEX IF NOT EXISTS idx_mobile_submission_jobs_platform ON mobile_submission_jobs(platform);
CREATE INDEX IF NOT EXISTS idx_mobile_submission_jobs_status ON mobile_submission_jobs(status);
CREATE INDEX IF NOT EXISTS idx_mobile_submission_jobs_provider_submission_id ON mobile_submission_jobs(provider_submission_id);
