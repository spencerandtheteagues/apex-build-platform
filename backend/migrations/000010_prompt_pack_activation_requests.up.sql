CREATE TABLE IF NOT EXISTS prompt_pack_activation_requests (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    request_id VARCHAR(96) NOT NULL,
    build_id VARCHAR(64) NOT NULL,
    draft_id VARCHAR(96) NOT NULL,
    draft_version VARCHAR(64) NOT NULL,
    scope VARCHAR(255),
    status VARCHAR(64) NOT NULL,
    requested_by_id INTEGER NOT NULL REFERENCES users(id),
    reason TEXT,
    feature_flag VARCHAR(96) NOT NULL,
    source_candidate_ids_json TEXT,
    changes_json TEXT,
    prompt_mutated BOOLEAN NOT NULL DEFAULT false
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_pack_activation_requests_request_id
    ON prompt_pack_activation_requests(request_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_requests_build_id
    ON prompt_pack_activation_requests(build_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_requests_draft_id
    ON prompt_pack_activation_requests(draft_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_requests_status
    ON prompt_pack_activation_requests(status);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_requests_requested_by_id
    ON prompt_pack_activation_requests(requested_by_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_requests_created_at
    ON prompt_pack_activation_requests(created_at);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_requests_deleted_at
    ON prompt_pack_activation_requests(deleted_at);

CREATE TABLE IF NOT EXISTS prompt_pack_versions (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    version_id VARCHAR(96) NOT NULL,
    scope VARCHAR(255) NOT NULL,
    version VARCHAR(64) NOT NULL,
    status VARCHAR(64) NOT NULL,
    source_build_id VARCHAR(64) NOT NULL,
    source_draft_id VARCHAR(96) NOT NULL,
    source_request_id VARCHAR(96) NOT NULL,
    source_candidate_ids_json TEXT,
    changes_json TEXT,
    activated_by_id INTEGER NOT NULL REFERENCES users(id),
    activated_at TIMESTAMP WITH TIME ZONE,
    rollback_of_version_id VARCHAR(96),
    prompt_mutated BOOLEAN NOT NULL DEFAULT false,
    live_prompt_read_enabled BOOLEAN NOT NULL DEFAULT false
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_pack_versions_version_id
    ON prompt_pack_versions(version_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_versions_scope
    ON prompt_pack_versions(scope);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_versions_status
    ON prompt_pack_versions(status);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_versions_source_build_id
    ON prompt_pack_versions(source_build_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_versions_source_draft_id
    ON prompt_pack_versions(source_draft_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_versions_source_request_id
    ON prompt_pack_versions(source_request_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_versions_activated_by_id
    ON prompt_pack_versions(activated_by_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_versions_rollback_of_version_id
    ON prompt_pack_versions(rollback_of_version_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_versions_created_at
    ON prompt_pack_versions(created_at);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_versions_deleted_at
    ON prompt_pack_versions(deleted_at);

CREATE TABLE IF NOT EXISTS prompt_pack_activation_events (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    event_id VARCHAR(96) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    prompt_pack_version_id VARCHAR(96) NOT NULL,
    activation_request_id VARCHAR(96) NOT NULL,
    build_id VARCHAR(64) NOT NULL,
    actor_id INTEGER NOT NULL REFERENCES users(id),
    reason TEXT,
    rollback_of_version_id VARCHAR(96),
    prompt_mutated BOOLEAN NOT NULL DEFAULT false,
    live_prompt_read_enabled BOOLEAN NOT NULL DEFAULT false
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_prompt_pack_activation_events_event_id
    ON prompt_pack_activation_events(event_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_events_event_type
    ON prompt_pack_activation_events(event_type);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_events_prompt_pack_version_id
    ON prompt_pack_activation_events(prompt_pack_version_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_events_activation_request_id
    ON prompt_pack_activation_events(activation_request_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_events_build_id
    ON prompt_pack_activation_events(build_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_events_actor_id
    ON prompt_pack_activation_events(actor_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_events_rollback_of_version_id
    ON prompt_pack_activation_events(rollback_of_version_id);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_events_created_at
    ON prompt_pack_activation_events(created_at);
CREATE INDEX IF NOT EXISTS idx_prompt_pack_activation_events_deleted_at
    ON prompt_pack_activation_events(deleted_at);
