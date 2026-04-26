DROP INDEX IF EXISTS idx_users_verification_code_expires_at;
DROP INDEX IF EXISTS idx_users_email_verified_at;

ALTER TABLE users
    DROP COLUMN IF EXISTS verification_code_expires_at,
    DROP COLUMN IF EXISTS verification_code,
    DROP COLUMN IF EXISTS email_verified_at;
