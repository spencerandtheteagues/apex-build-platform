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
PLAYWRIGHT_EXPECT_LAUNCH_READY=1 \
npm run test:launch
```

### 2. Stripe launch verification

From the repo root, check production billing readiness, authenticated billing config, self-serve price IDs, and optionally create Stripe checkout sessions without completing payment.

```bash
APEX_API_URL=https://api.apex-build.dev \
APEX_FRONTEND_URL=https://apex-build.dev \
APEX_STRIPE_EXPECT_LIVE=1 \
APEX_STRIPE_REGISTER_SMOKE_USER=1 \
node scripts/verify_stripe_launch.mjs
```

Optional checkout-session probe:

```bash
APEX_API_URL=https://api.apex-build.dev \
APEX_FRONTEND_URL=https://apex-build.dev \
APEX_STRIPE_EXPECT_LIVE=1 \
APEX_STRIPE_REGISTER_SMOKE_USER=1 \
APEX_STRIPE_RUN_CHECKOUT=1 \
APEX_STRIPE_CHECKOUT_PLAN=builder \
APEX_STRIPE_RUN_CREDIT_CHECKOUT=1 \
node scripts/verify_stripe_launch.mjs
```

Optional authenticated step:

```bash
cd tests/e2e
PLAYWRIGHT_BASE_URL=https://apex-build.dev \
PLAYWRIGHT_API_URL=https://api.apex-build.dev \
PLAYWRIGHT_EXPECT_LIVE_STRIPE=1 \
PLAYWRIGHT_EXPECT_LAUNCH_READY=1 \
PLAYWRIGHT_LAUNCH_USERNAME='launch-smoke-user' \
PLAYWRIGHT_LAUNCH_PASSWORD='replace-me' \
npm run test:launch
```

### 3. Render environment verification

From the repo root, validate the Render blueprint and, when Render API credentials are available, verify production env-var presence without printing secret values.

```bash
node scripts/verify_render_launch_env.mjs
```

Strict production check:

```bash
APEX_RENDER_EXPECT_PRODUCTION=1 \
RENDER_API_KEY='replace-me' \
RENDER_BACKEND_SERVICE_ID='replace-me' \
RENDER_FRONTEND_SERVICE_ID='replace-me' \
node scripts/verify_render_launch_env.mjs
```

The strict check also calls production `/health` and `/health/features`, and fails if Redis, code execution, preview runtime, or browser proof readiness is not launch-ready.

### 4. Platform build smoke

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

### 5. Platform canary matrix

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

### 6. Mobile external-provider evidence

From the repo root, verify the public launch posture still treats native mobile build and store-upload paths as gated:

```bash
node scripts/verify_mobile_external_readiness.mjs
```

Strict native/store evidence check after a real mobile project has EAS, Apple, Google Play, and signing credentials plus provider history:

```bash
APEX_API_URL=https://api.apex-build.dev \
APEX_MOBILE_EXPECT_NATIVE_READY=1 \
APEX_AUTH_TOKEN='replace-me' \
APEX_MOBILE_PROJECT_ID='replace-me' \
node scripts/verify_mobile_external_readiness.mjs
```

Do not make native build, TestFlight, Google Play, or store-approval claims public unless the strict check has real project evidence and manual store-console review remains accurately separated.

## Scheduled Production Canary

GitHub Actions now includes `.github/workflows/production-canary.yml`:

- the whole workflow remains opt-in with repository variable `APEX_ENABLE_GITHUB_ACTIONS=true` so hosted runners are not requested on the free/no-billing account by default
- `Public Launch Smoke` runs the Playwright launch smoke against `apex-build.dev` with `PLAYWRIGHT_EXPECT_LIVE_STRIPE=1` and `PLAYWRIGHT_EXPECT_LAUNCH_READY=1`
- `Launch Verification Scripts` runs the Stripe, Render, and mobile external-readiness verifiers against production
- the Stripe verifier reuses `APEX_CANARY_USERNAME`/`APEX_CANARY_EMAIL` plus `APEX_CANARY_PASSWORD` when configured, otherwise it registers a throwaway smoke user
- `Launch Verification Scripts` runs strict Render env verification only when `RENDER_API_KEY`, `RENDER_BACKEND_SERVICE_ID`, and `RENDER_FRONTEND_SERVICE_ID` secrets are configured
- workflow dispatch input `run_checkout_probes=true` creates non-paid Stripe subscription and credit checkout sessions from the verifier
- workflow dispatch input `run_mobile_external_strict=true` requires `APEX_MOBILE_CANARY_TOKEN` and `APEX_MOBILE_CANARY_PROJECT_ID`, then proves strict native/store evidence for that project
- `Preview Verification Canary` runs preview readiness coverage against production
- `Platform Build Canary (free-fast / paid-balanced / paid-max)` runs the build matrix against production
- `Golden FieldOps Live Canary` runs the balanced/max golden prompt when canary credentials exist
- `Prompt Reliability Live Matrix` remains manual through `run_prompt_matrix=true`
- set `APEX_CANARY_USERNAME` as well when the paid canary account authenticates more reliably by username than email

Treat any failure in that workflow as a customer-facing reliability regression until explained.

## Manual Checks

Run these after the automated checks pass:

1. Open the public landing page and confirm the footer `Privacy`, `Terms`, `Docs`, and `Status` links all work.
2. Create a real user account and confirm legal acceptance is visible during signup.
3. Confirm Stripe checkout opens for the intended paid plan and returns to the app correctly.
4. Confirm Stripe billing portal opens and returns to the app correctly.
5. Replay real Stripe test webhook events for subscription checkout, credit purchase, invoice paid, invoice failed, plan change, subscription deletion, and duplicate delivery of credit-granting event IDs.
6. Complete one free frontend-only build and confirm it finishes cleanly.
7. Complete one paid full-stack build and confirm:
   - staged progress is visible
   - frontend/UI appears before backend completion work
   - the build reaches a truthful terminal state
8. Force one recoverable failed build and confirm `Restart Failed Build` creates visible new work.

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
