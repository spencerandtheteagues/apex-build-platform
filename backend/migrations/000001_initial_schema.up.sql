-- APEX.BUILD Initial Schema Migration
-- This migration creates all tables from the current GORM models
-- Generated for golang-migrate

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- ============================================================================
-- USERS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Basic user information
    username VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255),
    avatar_url TEXT,

    -- Account status
    is_active BOOLEAN DEFAULT true,
    is_verified BOOLEAN DEFAULT false,

    -- Admin and special privileges
    is_admin BOOLEAN DEFAULT false,
    is_super_admin BOOLEAN DEFAULT false,
    has_unlimited_credits BOOLEAN DEFAULT false,
    bypass_billing BOOLEAN DEFAULT false,
    bypass_rate_limits BOOLEAN DEFAULT false,

    -- Subscription and billing (Stripe integration)
    stripe_customer_id VARCHAR(255),
    subscription_id VARCHAR(255),
    subscription_status VARCHAR(50) DEFAULT 'inactive',
    subscription_type VARCHAR(50) DEFAULT 'free',
    subscription_end TIMESTAMP WITH TIME ZONE,
    billing_cycle_start TIMESTAMP WITH TIME ZONE,

    -- Usage tracking for AI services
    monthly_ai_requests INTEGER DEFAULT 0,
    monthly_ai_cost DECIMAL(12, 4) DEFAULT 0.0,
    credit_balance DECIMAL(12, 4) DEFAULT 0.0,

    -- Preferences
    preferred_theme VARCHAR(50) DEFAULT 'cyberpunk',
    preferred_ai VARCHAR(50) DEFAULT 'auto'
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
CREATE INDEX IF NOT EXISTS idx_users_stripe_customer_id ON users(stripe_customer_id);
CREATE INDEX IF NOT EXISTS idx_users_subscription_id ON users(subscription_id);

-- ============================================================================
-- PROJECTS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS projects (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Basic project information
    name VARCHAR(255) NOT NULL,
    description TEXT,
    language VARCHAR(100) NOT NULL,
    framework VARCHAR(100),

    -- Project ownership and access
    owner_id INTEGER NOT NULL REFERENCES users(id),
    is_public BOOLEAN DEFAULT false,
    is_archived BOOLEAN DEFAULT false,

    -- Project structure
    root_directory VARCHAR(1024) DEFAULT '/',
    entry_point VARCHAR(512),

    -- Runtime configuration (JSON columns)
    environment JSONB DEFAULT '{}',
    dependencies JSONB DEFAULT '{}',
    build_config JSONB DEFAULT '{}',

    -- Nix-like Environment Configuration
    environment_config TEXT,

    -- Auto-provisioned database
    provisioned_database_id INTEGER,

    -- Collaboration
    collab_room_id INTEGER
);

CREATE INDEX IF NOT EXISTS idx_projects_deleted_at ON projects(deleted_at);
CREATE INDEX IF NOT EXISTS idx_projects_owner_id ON projects(owner_id);
CREATE INDEX IF NOT EXISTS idx_projects_provisioned_database_id ON projects(provisioned_database_id);

-- ============================================================================
-- FILES TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS files (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- File identification
    project_id INTEGER NOT NULL REFERENCES projects(id),
    path VARCHAR(2048) NOT NULL,
    name VARCHAR(512) NOT NULL,
    type VARCHAR(50) NOT NULL,
    mime_type VARCHAR(255),

    -- File content
    content TEXT,
    size BIGINT DEFAULT 0,
    hash VARCHAR(64),

    -- Versioning
    version INTEGER DEFAULT 1,
    last_edit_by INTEGER REFERENCES users(id),

    -- File status
    is_locked BOOLEAN DEFAULT false,
    locked_by INTEGER REFERENCES users(id),
    locked_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_files_deleted_at ON files(deleted_at);
CREATE INDEX IF NOT EXISTS idx_files_project_id ON files(project_id);

-- ============================================================================
-- SESSIONS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS sessions (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Session identification
    user_id INTEGER NOT NULL REFERENCES users(id),
    session_id VARCHAR(512) NOT NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,

    -- Session state
    is_active BOOLEAN DEFAULT true,
    expires_at TIMESTAMP WITH TIME ZONE,
    last_seen TIMESTAMP WITH TIME ZONE,

    -- Current context
    current_project_id INTEGER REFERENCES projects(id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_sessions_session_id ON sessions(session_id);
CREATE INDEX IF NOT EXISTS idx_sessions_deleted_at ON sessions(deleted_at);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);

-- ============================================================================
-- AI_REQUESTS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS ai_requests (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Request identification
    request_id VARCHAR(36) NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(id),
    project_id INTEGER REFERENCES projects(id),

    -- AI request details
    provider VARCHAR(50) NOT NULL,
    capability VARCHAR(100) NOT NULL,
    prompt TEXT,
    code TEXT,
    language VARCHAR(100),
    context JSONB DEFAULT '{}',

    -- AI response
    response TEXT,
    tokens_used INTEGER DEFAULT 0,
    cost DECIMAL(12, 6) DEFAULT 0.0,
    duration BIGINT DEFAULT 0,

    -- Request status
    status VARCHAR(50) DEFAULT 'pending',
    error_msg TEXT,

    -- Quality feedback
    user_rating INTEGER,
    user_feedback TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ai_requests_request_id ON ai_requests(request_id);
CREATE INDEX IF NOT EXISTS idx_ai_requests_deleted_at ON ai_requests(deleted_at);
CREATE INDEX IF NOT EXISTS idx_ai_requests_user_id ON ai_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_requests_project_id ON ai_requests(project_id);

-- ============================================================================
-- EXECUTIONS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS executions (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Execution identification
    execution_id VARCHAR(36) NOT NULL,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    user_id INTEGER NOT NULL REFERENCES users(id),

    -- Execution context
    command TEXT NOT NULL,
    language VARCHAR(100) NOT NULL,
    environment JSONB DEFAULT '{}',
    input TEXT,

    -- Execution results
    output TEXT,
    error_out TEXT,
    exit_code INTEGER DEFAULT 0,
    duration BIGINT DEFAULT 0,

    -- Execution state
    status VARCHAR(50) DEFAULT 'running',
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,

    -- Resource usage
    memory_used BIGINT DEFAULT 0,
    cpu_time BIGINT DEFAULT 0
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_executions_execution_id ON executions(execution_id);
CREATE INDEX IF NOT EXISTS idx_executions_deleted_at ON executions(deleted_at);
CREATE INDEX IF NOT EXISTS idx_executions_project_id ON executions(project_id);
CREATE INDEX IF NOT EXISTS idx_executions_user_id ON executions(user_id);

-- ============================================================================
-- COLLAB_ROOMS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS collab_rooms (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Room identification
    room_id VARCHAR(36) NOT NULL,
    project_id INTEGER NOT NULL REFERENCES projects(id),

    -- Room state
    is_active BOOLEAN DEFAULT true,
    max_users INTEGER DEFAULT 10,
    current_users INTEGER DEFAULT 0,

    -- Collaboration settings
    allow_anonymous BOOLEAN DEFAULT false,
    require_auth BOOLEAN DEFAULT true,
    password VARCHAR(255)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_collab_rooms_room_id ON collab_rooms(room_id);
CREATE INDEX IF NOT EXISTS idx_collab_rooms_deleted_at ON collab_rooms(deleted_at);
CREATE INDEX IF NOT EXISTS idx_collab_rooms_project_id ON collab_rooms(project_id);

-- ============================================================================
-- CURSOR_POSITIONS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS cursor_positions (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Position tracking
    room_id INTEGER NOT NULL REFERENCES collab_rooms(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    file_id INTEGER NOT NULL REFERENCES files(id),

    -- Cursor coordinates
    line INTEGER NOT NULL,
    "column" INTEGER NOT NULL,

    -- Selection range
    selection_start_line INTEGER,
    selection_start_column INTEGER,
    selection_end_line INTEGER,
    selection_end_column INTEGER,

    -- Status
    is_active BOOLEAN DEFAULT true,
    last_active TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_cursor_positions_room_id ON cursor_positions(room_id);
CREATE INDEX IF NOT EXISTS idx_cursor_positions_user_id ON cursor_positions(user_id);
CREATE INDEX IF NOT EXISTS idx_cursor_positions_file_id ON cursor_positions(file_id);

-- ============================================================================
-- CHAT_MESSAGES TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS chat_messages (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Message details
    room_id INTEGER NOT NULL REFERENCES collab_rooms(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    message TEXT NOT NULL,
    type VARCHAR(50) DEFAULT 'text',

    -- Message status
    is_edited BOOLEAN DEFAULT false,
    edited_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_chat_messages_deleted_at ON chat_messages(deleted_at);
CREATE INDEX IF NOT EXISTS idx_chat_messages_room_id ON chat_messages(room_id);
CREATE INDEX IF NOT EXISTS idx_chat_messages_user_id ON chat_messages(user_id);

-- ============================================================================
-- USER_COLLAB_ROOMS TABLE (Many-to-Many)
-- ============================================================================
CREATE TABLE IF NOT EXISTS user_collab_rooms (
    user_id INTEGER NOT NULL REFERENCES users(id),
    collab_room_id INTEGER NOT NULL REFERENCES collab_rooms(id),
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    role VARCHAR(50) DEFAULT 'member',
    is_active BOOLEAN DEFAULT true,
    PRIMARY KEY (user_id, collab_room_id)
);

-- ============================================================================
-- FILE_VERSIONS TABLE (Version History - Replit Parity)
-- ============================================================================
CREATE TABLE IF NOT EXISTS file_versions (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- File relationship
    file_id INTEGER NOT NULL REFERENCES files(id),
    project_id INTEGER NOT NULL,

    -- Version identification
    version INTEGER NOT NULL,
    version_hash VARCHAR(64),

    -- Content snapshot
    content TEXT,
    size BIGINT DEFAULT 0,
    line_count INTEGER DEFAULT 0,

    -- Change metadata
    change_type VARCHAR(50) DEFAULT 'edit',
    change_summary TEXT,
    lines_added INTEGER DEFAULT 0,
    lines_removed INTEGER DEFAULT 0,

    -- Author information
    author_id INTEGER NOT NULL REFERENCES users(id),
    author_name VARCHAR(255),

    -- File path at this version
    file_path VARCHAR(2048) NOT NULL,
    file_name VARCHAR(512) NOT NULL,

    -- Retention flags
    is_pinned BOOLEAN DEFAULT false,
    is_auto_save BOOLEAN DEFAULT false
);

CREATE INDEX IF NOT EXISTS idx_file_versions_deleted_at ON file_versions(deleted_at);
CREATE INDEX IF NOT EXISTS idx_file_versions_file_id ON file_versions(file_id);
CREATE INDEX IF NOT EXISTS idx_file_versions_project_id ON file_versions(project_id);
CREATE INDEX IF NOT EXISTS idx_file_versions_version_hash ON file_versions(version_hash);
CREATE INDEX IF NOT EXISTS idx_file_versions_author_id ON file_versions(author_id);

-- ============================================================================
-- CODE_COMMENTS TABLE (Inline Comments - Replit Parity)
-- ============================================================================
CREATE TABLE IF NOT EXISTS code_comments (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- File relationship
    file_id INTEGER NOT NULL REFERENCES files(id),
    project_id INTEGER NOT NULL,

    -- Position in file
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    start_column INTEGER DEFAULT 0,
    end_column INTEGER DEFAULT 0,

    -- Comment content
    content TEXT NOT NULL,

    -- Thread management
    parent_id INTEGER REFERENCES code_comments(id),
    thread_id VARCHAR(36),

    -- Author
    author_id INTEGER NOT NULL REFERENCES users(id),
    author_name VARCHAR(255),

    -- Status
    is_resolved BOOLEAN DEFAULT false,
    resolved_at TIMESTAMP WITH TIME ZONE,
    resolved_by_id INTEGER REFERENCES users(id),

    -- Reactions (emoji -> user IDs)
    reactions JSONB DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_code_comments_deleted_at ON code_comments(deleted_at);
CREATE INDEX IF NOT EXISTS idx_code_comments_file_id ON code_comments(file_id);
CREATE INDEX IF NOT EXISTS idx_code_comments_project_id ON code_comments(project_id);
CREATE INDEX IF NOT EXISTS idx_code_comments_parent_id ON code_comments(parent_id);
CREATE INDEX IF NOT EXISTS idx_code_comments_thread_id ON code_comments(thread_id);
CREATE INDEX IF NOT EXISTS idx_code_comments_author_id ON code_comments(author_id);

-- ============================================================================
-- REFRESH_TOKENS TABLE (Secure Token Rotation)
-- ============================================================================
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Token identification
    token VARCHAR(512) NOT NULL,
    token_hash VARCHAR(64) NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(id),

    -- Token metadata
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    issued_at TIMESTAMP WITH TIME ZONE NOT NULL,

    -- Token state for rotation
    used BOOLEAN DEFAULT false,
    used_at TIMESTAMP WITH TIME ZONE,
    revoked BOOLEAN DEFAULT false,
    revoked_at TIMESTAMP WITH TIME ZONE,

    -- Security tracking
    ip_address VARCHAR(45),
    user_agent TEXT,
    device_id VARCHAR(255),

    -- Token family for reuse detection
    family_id VARCHAR(36) NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_token ON refresh_tokens(token);
CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_deleted_at ON refresh_tokens(deleted_at);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_used ON refresh_tokens(used);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_revoked ON refresh_tokens(revoked);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_device_id ON refresh_tokens(device_id);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON refresh_tokens(family_id);

-- ============================================================================
-- USER_API_KEYS TABLE (BYOK - Bring Your Own Key)
-- ============================================================================
CREATE TABLE IF NOT EXISTS user_api_keys (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Ownership
    user_id INTEGER NOT NULL REFERENCES users(id),
    provider VARCHAR(20) NOT NULL,

    -- Encrypted key storage
    encrypted_key TEXT NOT NULL,
    key_salt VARCHAR(255) NOT NULL,
    key_fingerprint VARCHAR(255) NOT NULL,

    -- User preferences
    model_preference VARCHAR(100),

    -- Status and tracking
    is_active BOOLEAN DEFAULT true,
    is_valid BOOLEAN DEFAULT false,
    last_used TIMESTAMP WITH TIME ZONE,
    usage_count BIGINT DEFAULT 0,
    total_cost DECIMAL(12, 4) DEFAULT 0.0
);

CREATE INDEX IF NOT EXISTS idx_user_api_keys_deleted_at ON user_api_keys(deleted_at);
CREATE UNIQUE INDEX IF NOT EXISTS idx_user_api_keys_user_provider ON user_api_keys(user_id, provider);

-- ============================================================================
-- AI_USAGE_LOGS TABLE (Cost Tracking)
-- ============================================================================
CREATE TABLE IF NOT EXISTS ai_usage_logs (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Request context
    user_id INTEGER NOT NULL REFERENCES users(id),
    project_id INTEGER REFERENCES projects(id),
    session_id VARCHAR(255),

    -- Provider and model details
    provider VARCHAR(20) NOT NULL,
    model VARCHAR(100) NOT NULL,
    is_byok BOOLEAN DEFAULT false,

    -- Token usage
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,

    -- Cost tracking (USD)
    cost DECIMAL(12, 6) DEFAULT 0.0,

    -- Request metadata
    capability VARCHAR(50),
    duration BIGINT DEFAULT 0,
    status VARCHAR(20),

    -- Monthly aggregation key
    month_key VARCHAR(7)
);

CREATE INDEX IF NOT EXISTS idx_ai_usage_logs_created_at ON ai_usage_logs(created_at);
CREATE INDEX IF NOT EXISTS idx_ai_usage_logs_user_id ON ai_usage_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_ai_usage_logs_project_id ON ai_usage_logs(project_id);
CREATE INDEX IF NOT EXISTS idx_ai_usage_logs_session_id ON ai_usage_logs(session_id);
CREATE INDEX IF NOT EXISTS idx_ai_usage_logs_provider ON ai_usage_logs(provider);
CREATE INDEX IF NOT EXISTS idx_ai_usage_logs_month_key ON ai_usage_logs(month_key);

-- ============================================================================
-- SECRETS TABLE (Encrypted Secrets Management)
-- ============================================================================
CREATE TABLE IF NOT EXISTS secrets (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    project_id INTEGER REFERENCES projects(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) DEFAULT 'generic',
    encrypted_value TEXT NOT NULL,
    key_fingerprint VARCHAR(255) NOT NULL,
    salt VARCHAR(255) NOT NULL,
    last_accessed TIMESTAMP WITH TIME ZONE,
    rotation_due TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_secrets_user_id ON secrets(user_id);
CREATE INDEX IF NOT EXISTS idx_secrets_project_id ON secrets(project_id);

-- ============================================================================
-- SECRET_AUDIT_LOGS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS secret_audit_logs (
    id SERIAL PRIMARY KEY,
    secret_id INTEGER NOT NULL REFERENCES secrets(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    action VARCHAR(50),
    ip_address VARCHAR(45),
    user_agent TEXT,
    success BOOLEAN,
    error_msg TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_secret_audit_logs_secret_id ON secret_audit_logs(secret_id);
CREATE INDEX IF NOT EXISTS idx_secret_audit_logs_user_id ON secret_audit_logs(user_id);

-- ============================================================================
-- EXTERNAL_MCP_SERVERS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS external_mcp_servers (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    project_id INTEGER REFERENCES projects(id),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    url TEXT NOT NULL,
    auth_type VARCHAR(50),
    auth_header VARCHAR(255),
    credential_secret_id INTEGER REFERENCES secrets(id),
    enabled BOOLEAN DEFAULT true,
    last_status VARCHAR(50),
    last_error TEXT,
    last_connected TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_external_mcp_servers_user_id ON external_mcp_servers(user_id);
CREATE INDEX IF NOT EXISTS idx_external_mcp_servers_project_id ON external_mcp_servers(project_id);

-- ============================================================================
-- REPOSITORIES TABLE (Git Integration)
-- ============================================================================
CREATE TABLE IF NOT EXISTS repositories (
    id SERIAL PRIMARY KEY,
    project_id INTEGER REFERENCES projects(id),
    remote_url TEXT,
    provider VARCHAR(50),
    repo_owner VARCHAR(255),
    repo_name VARCHAR(255),
    branch VARCHAR(255),
    last_sync TIMESTAMP WITH TIME ZONE,
    is_connected BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_repositories_project_id ON repositories(project_id);

-- ============================================================================
-- MANAGED_DATABASES TABLE (Auto-Provisioned PostgreSQL per Project)
-- ============================================================================
CREATE TABLE IF NOT EXISTS managed_databases (
    id SERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(id),
    type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    host VARCHAR(255),
    port INTEGER,
    username VARCHAR(255),
    password TEXT NOT NULL,
    salt VARCHAR(255) NOT NULL,
    database_name VARCHAR(255),
    status VARCHAR(50) DEFAULT 'provisioning',
    file_path TEXT,

    -- Auto-provisioning flag
    is_auto_provisioned BOOLEAN DEFAULT false,

    -- Usage metrics
    storage_used_mb DECIMAL(12, 4) DEFAULT 0,
    connection_count INTEGER DEFAULT 0,
    query_count BIGINT DEFAULT 0,
    last_queried TIMESTAMP WITH TIME ZONE,

    -- Backup configuration
    backup_enabled BOOLEAN DEFAULT true,
    backup_schedule VARCHAR(100),
    last_backup TIMESTAMP WITH TIME ZONE,
    next_backup TIMESTAMP WITH TIME ZONE,

    -- Plan limits
    max_storage_mb INTEGER DEFAULT 100,
    max_connections INTEGER DEFAULT 5,

    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_managed_databases_project_id ON managed_databases(project_id);
CREATE INDEX IF NOT EXISTS idx_managed_databases_user_id ON managed_databases(user_id);

-- ============================================================================
-- PERFORMANCE INDEXES (from current database.go)
-- ============================================================================
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_username_active ON users(username) WHERE is_active = true;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email_verified ON users(email) WHERE is_verified = true;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_projects_owner_active ON projects(owner_id) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_projects_public ON projects(is_public) WHERE is_public = true AND deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_projects_language ON projects(language) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_files_project_path ON files(project_id, path) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_files_hash ON files(hash) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ai_requests_user_date ON ai_requests(user_id, created_at DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ai_requests_provider_status ON ai_requests(provider, status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ai_requests_capability ON ai_requests(capability);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_project_date ON executions(project_id, created_at DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_executions_user_status ON executions(user_id, status);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_user_active ON sessions(user_id) WHERE is_active = true;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_sessions_expires ON sessions(expires_at) WHERE is_active = true;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_collab_rooms_project ON collab_rooms(project_id) WHERE is_active = true;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_cursor_positions_room_active ON cursor_positions(room_id) WHERE is_active = true;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chat_messages_room_date ON chat_messages(room_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_secrets_user_name ON secrets(user_id, name);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_secrets_project ON secrets(project_id) WHERE project_id IS NOT NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_secret_audit_logs_secret ON secret_audit_logs(secret_id, created_at DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_external_mcp_servers_user ON external_mcp_servers(user_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_external_mcp_servers_project ON external_mcp_servers(project_id) WHERE project_id IS NOT NULL;
