# Launch Runbook

Last updated: 2026-03-26

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

Runs a sacrificial end-to-end app build with preview readiness enforced.

```bash
BASE_URL=https://api.apex-build.dev/api/v1 \
MODE=fast \
POWER_MODE=balanced \
./scripts/run_platform_build_smoke.sh
```

Expected result:

- the script registers a new temporary account
- starts a build
- polls until terminal state
- prints the final build summary

Treat any `failed`, `cancelled`, or `BUILD_DID_NOT_TERMINATE_WITHIN_POLL_WINDOW` result as a launch blocker until explained.

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
