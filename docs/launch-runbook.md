# Launch Runbook

Last updated: 2026-05-08

This is the minimum go-live runbook for opening `apex-build.dev` to real customers.

## Preconditions

- Do not launch during a known Render maintenance window for `apex-db` or `apex-redis`.
- Confirm these production environment variables are set and correct in Render:
  - `DATABASE_URL`
  - `REDIS_URL`
  - `FRONTEND_URL`
  - `JWT_SECRET`
  - `JWT_REFRESH_SECRET`
  - `SECRETS_MASTER_KEY`
  - `STRIPE_SECRET_KEY`
  - `STRIPE_WEBHOOK_SECRET`
  - every `STRIPE_PRICE_*` value used by the app
- Confirm `https://apex-build.dev` and `https://api.apex-build.dev` are serving the latest deployed commit.
- Confirm backend `REDIS_URL` resolves to the internal `apex-redis` Render Key Value connection string, not an external allowlisted Redis URL.
- Confirm `/health/features` reports `code_execution.details.launch_ready=true` and `preview_service.details.launch_ready=true`.
- Confirm production has one reachable isolated runtime path for code execution: `E2B_API_KEY` or a remote Docker configuration such as `APEX_EXECUTION_DOCKER_HOST`.
- Confirm production preview has one reachable isolated runtime path for preview containers/backend preview: `APEX_PREVIEW_DOCKER_HOST` plus `APEX_PREVIEW_CONNECT_HOST` when needed, or a validated E2B preview runtime.

## Render Workspace Setup

Set these before public launch:

- `Notifications`: enable workspace notifications for failed deploys and unhealthy services. Connect Slack and/or email, and make sure `apex-api` and `apex-frontend` are covered.
- `Service metrics` and `logs`: use Render's built-in dashboards for `apex-api`, `apex-frontend`, `apex-db`, and `apex-redis` during launch.

Do not treat these as launch blockers unless you already depend on them operationally:

- `Webhooks`: optional. Only set this up if you want custom automation when deploys fail or services change state.
- `Private Links`: not required for the current APEX.BUILD production path. Render-managed Postgres and Key Value already use internal wiring, and E2B currently works over the normal outbound API-key path.
- External observability sinks: optional. Add external log or metrics streaming only if you already have a destination such as Datadog, Grafana, or another monitoring stack.

## Automated Checks

### 1. Public launch smoke

Runs the non-destructive production smoke against the customer-facing surfaces.

```bash
cd tests/e2e
PLAYWRIGHT_BASE_URL=https://apex-build.dev \
PLAYWRIGHT_API_URL=https://api.apex-build.dev \
PLAYWRIGHT_EXPECT_LIVE_STRIPE=1 \
npm run test:launch
```

Optional authenticated step:

```bash
cd tests/e2e
PLAYWRIGHT_BASE_URL=https://apex-build.dev \
PLAYWRIGHT_API_URL=https://api.apex-build.dev \
PLAYWRIGHT_EXPECT_LIVE_STRIPE=1 \
PLAYWRIGHT_LAUNCH_USERNAME='launch-smoke-user' \
PLAYWRIGHT_LAUNCH_PASSWORD='replace-me' \
npm run test:launch
```

### 2. Platform build smoke

Runs a sacrificial end-to-end app build with preview readiness enforced and asserts the completed-build detail agrees with the live build status.

```bash
BASE_URL=https://api.apex-build.dev/api/v1 \
MODE=fast \
POWER_MODE=balanced \
./scripts/run_platform_build_smoke.sh
```

Expected result:

- the script registers a new temporary account
- starts a frontend-preview build by default (`SMOKE_PROFILE=free_frontend`)
- polls until terminal state
- exits non-zero unless the build reaches `completed`
- asserts `quality_gate_passed=true`
- asserts completed-build history agrees with the terminal build status
- prints the final build summary

Treat any `failed`, `cancelled`, or `BUILD_DID_NOT_TERMINATE_WITHIN_POLL_WINDOW` result as a launch blocker until explained.

Optional paid full-stack canary:

```bash
BASE_URL=https://api.apex-build.dev/api/v1 \
MODE=full \
POWER_MODE=balanced \
SMOKE_PROFILE=paid_fullstack \
LOGIN_USERNAME='paid-canary-username' \
LOGIN_EMAIL='paid-canary@example.com' \
LOGIN_PASSWORD='replace-me' \
./scripts/run_platform_build_smoke.sh
```

### 3. Platform canary matrix

Runs the production-critical matrix instead of a single build:

- free fast frontend-preview canary
- paid balanced full-stack canary
- paid max full-stack canary

```bash
BASE_URL=https://api.apex-build.dev/api/v1 \
LOGIN_USERNAME='paid-canary-username' \
LOGIN_EMAIL='paid-canary@example.com' \
LOGIN_PASSWORD='replace-me' \
./scripts/run_platform_canary_matrix.sh
```

## Scheduled Production Canary

GitHub Actions now includes `.github/workflows/production-canary.yml`:

- `Public Launch Smoke` runs the Playwright launch smoke against `apex-build.dev`
- `Free Frontend Build Canary` runs the sacrificial free-tier preview build against production
- `Platform Build Canary (free-fast / paid-balanced / paid-max)` runs the build matrix against production
- set `APEX_CANARY_USERNAME` as well when the paid canary account authenticates more reliably by username than email

Treat any failure in that workflow as a customer-facing reliability regression until explained.

## Manual Checks

Run these after the automated checks pass:

1. Open the public landing page and confirm the footer `Privacy`, `Terms`, `Docs`, and `Status` links all work.
2. Create a real user account and confirm legal acceptance is visible during signup.
3. Confirm Stripe checkout opens for the intended paid plan and returns to the app correctly.
4. Confirm Stripe billing portal opens and returns to the app correctly.
5. Complete one free frontend-only build and confirm it finishes cleanly.
6. Complete one paid full-stack build and confirm:
   - staged progress is visible
   - frontend/UI appears before backend completion work
   - the build reaches a truthful terminal state
7. Force one recoverable failed build and confirm `Restart Failed Build` creates visible new work.

## Hold Criteria

Do not open the product to customers if any of these are true:

- `/health` is not `200`
- `/health/features` is not `200`
- `/health/features` reports `redis_cache` degraded because of an allowlist or external Redis connection error
- `/health/features` reports `payments` degraded because Stripe secrets, webhook secret, or self-serve plan price IDs are missing/placeholders
- `/health/features` reports `code_execution.details.launch_ready=false`
- `/health/features` reports `preview_service.details.launch_ready=false`
- `/health/features` reports `preview_runtime_verify` degraded in production because runtime browser proof is enabled but Chrome/Chromium is missing
- billing plans return placeholder Stripe price IDs
- the platform build smoke does not reach a clean terminal result
- restart recovery acknowledges the action but does not create new execution
- Render database or Redis maintenance is actively in progress

## First-Hour Monitoring

Keep these visible during launch:

- `https://api.apex-build.dev/health`
- `https://api.apex-build.dev/health/features`
- backend Render logs
- Stripe webhook logs
- build failure and restart telemetry
- Redis and Postgres service status in Render

## Rollback Trigger

Rollback or close launch traffic immediately if:

- signup or login breaks for new users
- checkout or billing portal fails for paying users
- full-stack builds fail broadly
- health endpoints go unhealthy
- Redis or Postgres instability produces repeated customer-facing build failures
