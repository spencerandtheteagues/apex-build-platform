-- APEX.BUILD Initial Schema Rollback
-- This migration drops all tables in reverse order of creation
-- WARNING: This will delete ALL data!

-- Drop performance indexes first
DROP INDEX CONCURRENTLY IF EXISTS idx_external_mcp_servers_project;
DROP INDEX CONCURRENTLY IF EXISTS idx_external_mcp_servers_user;
DROP INDEX CONCURRENTLY IF EXISTS idx_secret_audit_logs_secret;
DROP INDEX CONCURRENTLY IF EXISTS idx_secrets_project;
DROP INDEX CONCURRENTLY IF EXISTS idx_secrets_user_name;
DROP INDEX CONCURRENTLY IF EXISTS idx_chat_messages_room_date;
DROP INDEX CONCURRENTLY IF EXISTS idx_cursor_positions_room_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_collab_rooms_project;
DROP INDEX CONCURRENTLY IF EXISTS idx_sessions_expires;
DROP INDEX CONCURRENTLY IF EXISTS idx_sessions_user_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_executions_user_status;
DROP INDEX CONCURRENTLY IF EXISTS idx_executions_project_date;
DROP INDEX CONCURRENTLY IF EXISTS idx_ai_requests_capability;
DROP INDEX CONCURRENTLY IF EXISTS idx_ai_requests_provider_status;
DROP INDEX CONCURRENTLY IF EXISTS idx_ai_requests_user_date;
DROP INDEX CONCURRENTLY IF EXISTS idx_files_hash;
DROP INDEX CONCURRENTLY IF EXISTS idx_files_project_path;
DROP INDEX CONCURRENTLY IF EXISTS idx_projects_language;
DROP INDEX CONCURRENTLY IF EXISTS idx_projects_public;
DROP INDEX CONCURRENTLY IF EXISTS idx_projects_owner_active;
DROP INDEX CONCURRENTLY IF EXISTS idx_users_email_verified;
DROP INDEX CONCURRENTLY IF EXISTS idx_users_username_active;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS managed_databases CASCADE;
DROP TABLE IF EXISTS repositories CASCADE;
DROP TABLE IF EXISTS external_mcp_servers CASCADE;
DROP TABLE IF EXISTS secret_audit_logs CASCADE;
DROP TABLE IF EXISTS secrets CASCADE;
DROP TABLE IF EXISTS ai_usage_logs CASCADE;
DROP TABLE IF EXISTS user_api_keys CASCADE;
DROP TABLE IF EXISTS refresh_tokens CASCADE;
DROP TABLE IF EXISTS code_comments CASCADE;
DROP TABLE IF EXISTS file_versions CASCADE;
DROP TABLE IF EXISTS user_collab_rooms CASCADE;
DROP TABLE IF EXISTS chat_messages CASCADE;
DROP TABLE IF EXISTS cursor_positions CASCADE;
DROP TABLE IF EXISTS collab_rooms CASCADE;
DROP TABLE IF EXISTS executions CASCADE;
DROP TABLE IF EXISTS ai_requests CASCADE;
DROP TABLE IF EXISTS sessions CASCADE;
DROP TABLE IF EXISTS files CASCADE;
DROP TABLE IF EXISTS projects CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- Drop extensions
DROP EXTENSION IF EXISTS "uuid-ossp";
