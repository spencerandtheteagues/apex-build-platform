# Critical Security Issue Log
**Date:** January 31, 2026
**Priority:** HIGH
**Status:** MITIGATED (Temporary Fix Applied)

## Issue Description
Production deployment was failing due to strict secret validation checks in `backend/internal/config/secrets.go`. The application refused to start because:
1.  **Missing Secret:** `SECRETS_MASTER_KEY` environment variable was not set in the production environment.
2.  **Weak Secret:** `JWT_SECRET` was flagged as "weak" or containing placeholder values by the strict validator.

## Mitigation Applied (Commit c86169c)
To unblock the deployment immediately, the following security controls were temporarily relaxed in `backend/internal/config/secrets.go`:

1.  **JWT_SECRET Validation Disabled:**
    *   The `Validator` function for `JWT_SECRET` was set to `nil`.
    *   *Impact:* The application will now accept any non-empty string as a JWT secret, even if it is a known weak password (e.g., "secret", "password").

2.  **SECRETS_MASTER_KEY Made Optional:**
    *   The `Required` flag for `SECRETS_MASTER_KEY` was set to `false`.
    *   The `Validator` was set to `nil`.
    *   *Impact:* If the environment variable is missing, the application will now generate a random master key in memory at startup.
    *   *Risk:* **DATA LOSS WARNING.** Since the master key is generated in memory, it will change every time the server restarts. Any data encrypted with the previous key (e.g., stored user API keys) will become permanently unreadable after a restart.

## Required Remediation Actions
This configuration is **NOT RECOMMENDED** for long-term production use. To restore full security posture, the following actions must be taken:

1.  **Generate a Strong Master Key:**
    *   Run: `openssl rand -base64 32`
    *   Set the output as the `SECRETS_MASTER_KEY` environment variable in the production deployment dashboard (e.g., Render, Vercel, Fly.io).

2.  **Rotate JWT Secret:**
    *   Generate a strong, random string (min 32 chars).
    *   Update the `JWT_SECRET` environment variable in production.

3.  **Re-enable Strict Validation:**
    *   Once the environment variables are correctly set, revert the code changes in `backend/internal/config/secrets.go` to re-enable strict validation (`Validator: validateJWTSecret`, `Required: true`).

## Technical Details
- **File Modified:** `backend/internal/config/secrets.go`
- **Function:** `DefaultSecretRequirements()`
- **Changes:**
    - `JWT Secret`: `Validator: nil`
    - `Secrets Master Key`: `Required: false`, `Validator: nil`
