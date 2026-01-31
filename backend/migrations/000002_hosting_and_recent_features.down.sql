-- Rollback: Remove Native Hosting System and Recent Features
-- WARNING: This will delete all hosting data!

-- Drop performance indexes first
DROP INDEX CONCURRENTLY IF EXISTS idx_subdomains_name_status;
DROP INDEX CONCURRENTLY IF EXISTS idx_deployment_env_vars_deployment;
DROP INDEX CONCURRENTLY IF EXISTS idx_deployment_logs_deployment_time;
DROP INDEX CONCURRENTLY IF EXISTS idx_native_deployments_always_on;
DROP INDEX CONCURRENTLY IF EXISTS idx_native_deployments_subdomain_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_native_deployments_user_status;
DROP INDEX CONCURRENTLY IF EXISTS idx_native_deployments_project;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS ssl_certificates CASCADE;
DROP TABLE IF EXISTS deployment_events CASCADE;
DROP TABLE IF EXISTS custom_domains CASCADE;
DROP TABLE IF EXISTS subdomains CASCADE;
DROP TABLE IF EXISTS deployment_history CASCADE;
DROP TABLE IF EXISTS deployment_env_vars CASCADE;
DROP TABLE IF EXISTS deployment_logs CASCADE;
DROP TABLE IF EXISTS native_deployments CASCADE;
