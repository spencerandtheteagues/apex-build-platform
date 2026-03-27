# tests/e2e

Root Playwright infrastructure for APEX.BUILD Phase 4.

## What this adds
- App-manifest driven smoke spec generation (`apps/*.json` -> `specs/generated/*.spec.ts`)
- CI-friendly root Playwright config independent of frontend-local E2E setup
- A place to add cross-app production verification journeys

## Usage
- `cd tests/e2e`
- `npm install`
- `npm run generate`
- `npm test`
- `npm run test:launch`

Environment overrides:
- `PLAYWRIGHT_BASE_URL` (default `http://localhost:5180`)
- `PLAYWRIGHT_API_URL` (default `http://localhost:8080`)
- `PLAYWRIGHT_INCLUDE_FIREFOX=true` to include Firefox locally
- `PLAYWRIGHT_EXPECT_LIVE_STRIPE=1` to require non-placeholder paid Stripe price IDs during the launch smoke
- `PLAYWRIGHT_LAUNCH_USERNAME` / `PLAYWRIGHT_LAUNCH_PASSWORD` to enable the optional authenticated launch smoke step

## Launch smoke

`npm run test:launch` is a non-destructive production-readiness smoke for the customer-facing surfaces:

- landing page and footer resources
- public legal and help routes
- auth/legal acceptance screen
- backend `/health` and `/health/features`
- public billing plans endpoint
- optional authenticated sign-in if launch test credentials are provided

Example production run:

```bash
cd tests/e2e
PLAYWRIGHT_BASE_URL=https://apex-build.dev \
PLAYWRIGHT_API_URL=https://api.apex-build.dev \
PLAYWRIGHT_EXPECT_LIVE_STRIPE=1 \
npm run test:launch
```

## Platform build smoke

`../scripts/run_platform_build_smoke.sh` runs a sacrificial build against a target API and fails unless the build reaches `completed`.

Default profile:

- `SMOKE_PROFILE=free_frontend`
- registers a temporary free user
- verifies the free-tier frontend preview path reaches a truthful completed state

Optional profile:

- `SMOKE_PROFILE=paid_fullstack`
- use `LOGIN_EMAIL` and `LOGIN_PASSWORD` to exercise a paid full-stack canary account
