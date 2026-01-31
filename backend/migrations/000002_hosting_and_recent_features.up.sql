-- Migration: Add Native Hosting System and Recent Features
-- This migration adds tables for the .apex.app native hosting system (Replit parity feature)
-- and any other recent schema additions not in initial migration

-- ============================================================================
-- NATIVE_DEPLOYMENTS TABLE (Core hosting model)
-- ============================================================================
CREATE TABLE IF NOT EXISTS native_deployments (
    id VARCHAR(36) PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Project and user association
    project_id INTEGER NOT NULL REFERENCES projects(id),
    user_id INTEGER NOT NULL REFERENCES users(id),

    -- Subdomain configuration
    subdomain VARCHAR(63) NOT NULL,
    custom_subdomain VARCHAR(63),
    subdomain_status VARCHAR(50) DEFAULT 'pending',

    -- Full URLs
    url TEXT,
    preview_url TEXT,

    -- Deployment status
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    container_status VARCHAR(50) DEFAULT 'stopped',
    error_message TEXT,

    -- Container configuration
    container_id VARCHAR(64),
    container_port INTEGER DEFAULT 3000,
    external_port INTEGER,

    -- Build configuration
    build_command VARCHAR(500),
    start_command VARCHAR(500),
    install_command VARCHAR(500),
    framework VARCHAR(50),
    node_version VARCHAR(20) DEFAULT '18',
    python_version VARCHAR(20) DEFAULT '3.11',
    go_version VARCHAR(20) DEFAULT '1.21',

    -- Resource limits
    memory_limit BIGINT DEFAULT 512,
    cpu_limit BIGINT DEFAULT 500,
    storage_limit BIGINT DEFAULT 1024,
    bandwidth_used BIGINT DEFAULT 0,

    -- Scaling configuration
    auto_scale BOOLEAN DEFAULT false,
    min_instances INTEGER DEFAULT 1,
    max_instances INTEGER DEFAULT 3,
    current_instances INTEGER DEFAULT 0,

    -- Health check configuration
    health_check_path VARCHAR(255) DEFAULT '/health',
    health_check_interval INTEGER DEFAULT 30,
    health_check_timeout INTEGER DEFAULT 5,
    restart_on_failure BOOLEAN DEFAULT true,
    max_restarts INTEGER DEFAULT 3,
    restart_count INTEGER DEFAULT 0,

    -- Always-On configuration (Replit parity feature)
    always_on BOOLEAN DEFAULT false,
    always_on_enabled_at TIMESTAMP WITH TIME ZONE,
    last_keep_alive TIMESTAMP WITH TIME ZONE,
    keep_alive_interval INTEGER DEFAULT 60,
    sleep_after_minutes INTEGER DEFAULT 0,

    -- DNS configuration
    dns_record_id VARCHAR(50),
    dns_zone_id VARCHAR(50),
    ssl_certificate_id VARCHAR(50),
    ssl_status VARCHAR(50) DEFAULT 'pending',

    -- Metrics
    total_requests BIGINT DEFAULT 0,
    avg_response_time BIGINT DEFAULT 0,
    last_request_at TIMESTAMP WITH TIME ZONE,
    uptime_seconds BIGINT DEFAULT 0,

    -- Timestamps
    build_started_at TIMESTAMP WITH TIME ZONE,
    build_completed_at TIMESTAMP WITH TIME ZONE,
    deployed_at TIMESTAMP WITH TIME ZONE,
    last_health_check TIMESTAMP WITH TIME ZONE,

    -- Build timing
    build_duration BIGINT,
    deploy_duration BIGINT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_native_deployments_subdomain ON native_deployments(subdomain);
CREATE INDEX IF NOT EXISTS idx_native_deployments_deleted_at ON native_deployments(deleted_at);
CREATE INDEX IF NOT EXISTS idx_native_deployments_project_id ON native_deployments(project_id);
CREATE INDEX IF NOT EXISTS idx_native_deployments_user_id ON native_deployments(user_id);
CREATE INDEX IF NOT EXISTS idx_native_deployments_status ON native_deployments(status);

-- ============================================================================
-- DEPLOYMENT_LOGS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS deployment_logs (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deployment_id VARCHAR(36) NOT NULL REFERENCES native_deployments(id),

    -- Log details
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    level VARCHAR(20) NOT NULL,
    source VARCHAR(50),
    message TEXT,
    metadata TEXT
);

CREATE INDEX IF NOT EXISTS idx_deployment_logs_deployment_id ON deployment_logs(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_logs_timestamp ON deployment_logs(timestamp);

-- ============================================================================
-- DEPLOYMENT_ENV_VARS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS deployment_env_vars (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    deployment_id VARCHAR(36) NOT NULL REFERENCES native_deployments(id),
    project_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,

    -- Variable details
    key VARCHAR(255) NOT NULL,
    value TEXT,
    description VARCHAR(500),
    is_secret BOOLEAN DEFAULT false,
    is_system BOOLEAN DEFAULT false,

    -- Scope
    environment VARCHAR(50) DEFAULT 'production'
);

CREATE INDEX IF NOT EXISTS idx_deployment_env_vars_deleted_at ON deployment_env_vars(deleted_at);
CREATE INDEX IF NOT EXISTS idx_deployment_env_vars_deployment_id ON deployment_env_vars(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_env_vars_project_id ON deployment_env_vars(project_id);

-- ============================================================================
-- DEPLOYMENT_HISTORY TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS deployment_history (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    deployment_id VARCHAR(36) NOT NULL REFERENCES native_deployments(id),
    project_id INTEGER NOT NULL,

    -- Deployment snapshot
    version INTEGER NOT NULL,
    commit_sha VARCHAR(40),
    commit_message VARCHAR(500),
    image_tag VARCHAR(100),

    -- Status at time of snapshot
    status VARCHAR(50),
    build_logs TEXT,

    -- Metrics at deployment time
    build_duration BIGINT,
    deploy_duration BIGINT
);

CREATE INDEX IF NOT EXISTS idx_deployment_history_deleted_at ON deployment_history(deleted_at);
CREATE INDEX IF NOT EXISTS idx_deployment_history_deployment_id ON deployment_history(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_history_project_id ON deployment_history(project_id);

-- ============================================================================
-- SUBDOMAINS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS subdomains (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Subdomain info
    name VARCHAR(63) NOT NULL,
    project_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,

    -- Status
    status VARCHAR(50) DEFAULT 'active',
    reserved_until TIMESTAMP WITH TIME ZONE,

    -- DNS
    dns_configured BOOLEAN DEFAULT false,
    ssl_configured BOOLEAN DEFAULT false
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_subdomains_name ON subdomains(name);
CREATE INDEX IF NOT EXISTS idx_subdomains_deleted_at ON subdomains(deleted_at);
CREATE INDEX IF NOT EXISTS idx_subdomains_project_id ON subdomains(project_id);
CREATE INDEX IF NOT EXISTS idx_subdomains_user_id ON subdomains(user_id);

-- ============================================================================
-- CUSTOM_DOMAINS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS custom_domains (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Domain info
    domain VARCHAR(255) NOT NULL,
    project_id INTEGER NOT NULL,
    user_id INTEGER NOT NULL,

    -- Verification
    verification_status VARCHAR(50) DEFAULT 'pending',
    verification_token VARCHAR(255),
    verified_at TIMESTAMP WITH TIME ZONE,

    -- DNS configuration
    dns_type VARCHAR(20) DEFAULT 'CNAME',
    dns_target VARCHAR(255),
    dns_verified BOOLEAN DEFAULT false,
    dns_checked_at TIMESTAMP WITH TIME ZONE,

    -- SSL configuration
    ssl_status VARCHAR(50) DEFAULT 'pending',
    ssl_provider VARCHAR(50) DEFAULT 'cloudflare',
    ssl_certificate_id VARCHAR(100),
    ssl_expires_at TIMESTAMP WITH TIME ZONE,
    ssl_auto_renew BOOLEAN DEFAULT true,

    -- Link to active deployment
    deployment_id VARCHAR(36),

    -- Status flags
    is_active BOOLEAN DEFAULT false,
    is_primary BOOLEAN DEFAULT false
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_custom_domains_domain ON custom_domains(domain);
CREATE INDEX IF NOT EXISTS idx_custom_domains_deleted_at ON custom_domains(deleted_at);
CREATE INDEX IF NOT EXISTS idx_custom_domains_project_id ON custom_domains(project_id);
CREATE INDEX IF NOT EXISTS idx_custom_domains_user_id ON custom_domains(user_id);
CREATE INDEX IF NOT EXISTS idx_custom_domains_deployment_id ON custom_domains(deployment_id);

-- ============================================================================
-- DEPLOYMENT_EVENTS TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS deployment_events (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Event info
    deployment_id VARCHAR(36) NOT NULL REFERENCES native_deployments(id),
    event_type VARCHAR(50) NOT NULL,
    event_status VARCHAR(20),
    message TEXT,
    metadata TEXT,

    -- Timing
    duration BIGINT
);

CREATE INDEX IF NOT EXISTS idx_deployment_events_deployment_id ON deployment_events(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_events_event_type ON deployment_events(event_type);

-- ============================================================================
-- SSL_CERTIFICATES TABLE
-- ============================================================================
CREATE TABLE IF NOT EXISTS ssl_certificates (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,

    -- Certificate info
    certificate_id VARCHAR(100) NOT NULL,
    domain VARCHAR(255) NOT NULL,
    issuer VARCHAR(100),

    -- Certificate data (encrypted)
    certificate_pem TEXT,
    private_key_pem TEXT,
    certificate_chain TEXT,

    -- Validity
    issued_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    is_valid BOOLEAN DEFAULT true,

    -- Renewal
    auto_renew BOOLEAN DEFAULT true,
    renewal_attempts INTEGER DEFAULT 0,
    last_renewal_at TIMESTAMP WITH TIME ZONE,
    next_renewal_at TIMESTAMP WITH TIME ZONE,

    -- Linking
    custom_domain_id INTEGER,
    subdomain_id INTEGER
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_ssl_certificates_certificate_id ON ssl_certificates(certificate_id);
CREATE INDEX IF NOT EXISTS idx_ssl_certificates_deleted_at ON ssl_certificates(deleted_at);
CREATE INDEX IF NOT EXISTS idx_ssl_certificates_domain ON ssl_certificates(domain);
CREATE INDEX IF NOT EXISTS idx_ssl_certificates_expires_at ON ssl_certificates(expires_at);
CREATE INDEX IF NOT EXISTS idx_ssl_certificates_custom_domain_id ON ssl_certificates(custom_domain_id);
CREATE INDEX IF NOT EXISTS idx_ssl_certificates_subdomain_id ON ssl_certificates(subdomain_id);

-- ============================================================================
-- PERFORMANCE INDEXES FOR HOSTING SYSTEM
-- ============================================================================
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_native_deployments_project ON native_deployments(project_id) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_native_deployments_user_status ON native_deployments(user_id, status) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_native_deployments_subdomain_active ON native_deployments(subdomain) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_native_deployments_always_on ON native_deployments(always_on) WHERE always_on = true AND deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_deployment_logs_deployment_time ON deployment_logs(deployment_id, timestamp DESC);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_deployment_env_vars_deployment ON deployment_env_vars(deployment_id) WHERE deleted_at IS NULL;
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_subdomains_name_status ON subdomains(name) WHERE status = 'active';
