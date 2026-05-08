# Apex Build Launch Readiness Tracker

Date: 2026-05-08

This tracker reconciles the master launch plan with the current repository state. Code, tests, production config, and live canary evidence remain authoritative.

## Current Branch State

- Branch: `feat/mobile-app-builder-end-to-end`
- Local status before push: ahead of origin with mobile lifecycle work plus this launch-alignment batch.
- Push dependency: GitHub auth must succeed without storing the token in repo files, remotes, shell history, or docs.

## Closed In This Batch

- Pricing truth aligned around Builder `$24/mo`, Pro `$59/mo`, Team `$149/mo`.
- Pro annual price aligned to `$566.40/yr`.
- Frontend launch-special `$49/$79` copy removed.
- Credit top-up fallback surfaces aligned to `$25`, `$50`, `$100`, `$250`.
- `/api/v1/usage/limits` pricing now derives from `payments.GetAllPlans()`.
- Usage plan payload includes owner limits for internal/admin consistency.
- GitHub CI, production canary, and nightly reliability workflows now use Node `20`.
- Production API/WebSocket docs and fallback WebSocket URL point at `api.apex-build.dev`.
- Billing audit now marks old findings as reconciled where current code has closed them.
- Billing launch readiness now reports missing Stripe secret, webhook secret, and self-serve plan price ID configuration through startup health and `/billing/config-status`.
- Execution and preview startup readiness now add `launch_ready`, safe runtime-config booleans, missing-env hints, issue codes, and recommended fixes to `/health/features`.
- Production preview sandbox fallback now degrades `preview_service` instead of being treated as launch-ready.
- Launch and preview Playwright smoke checks now assert runtime launch readiness when `PLAYWRIGHT_EXPECT_LAUNCH_READY=1`.
- Local Stripe webhook replay coverage now proves duplicate subscription checkout, credit purchase, invoice paid, invoice failed, plan change, and subscription deletion delivery does not double-credit or corrupt billing state.
- Stripe launch verification is now scripted through `scripts/verify_stripe_launch.mjs` for production payments readiness, authenticated billing config, paid price IDs, and opt-in checkout-session probes.
- Render launch environment verification is now scripted through `scripts/verify_render_launch_env.mjs` for blueprint checks, optional Render API env-var presence, and strict live health/readiness checks.

## Launch Blockers Still Open

- Configure real Stripe live/test price IDs matching the current launch contract and confirm `/health/features` no longer reports `payments` degraded.
- Replay real Stripe test webhooks through the configured webhook endpoint for subscription checkout, credit purchase, invoice paid, invoice failed, plan change, and duplicate event delivery.
- Run a controlled live checkout and billing portal flow before enabling broad public signup.
- Run strict Render launch env verification and confirm production has `DATABASE_URL`, `REDIS_URL`, JWT secrets, `SECRETS_MASTER_KEY`, Stripe secrets, provider keys, remote Docker/preview env, and current frontend/backend URLs.
- Redeploy production and confirm `/health/features` shows `code_execution.details.launch_ready=true`, `preview_service.details.launch_ready=true`, and runtime browser proof ready.
- Run production canary matrix against `https://apex-build.dev` and `https://api.apex-build.dev`.
- Verify free frontend build, paid full-stack build, preview proof, export/deploy handoff, billing upgrade/downgrade, credit top-up, and failed-build restart in production.
- Live-validate EAS Build/Submit, Apple, and Google Play credentials before making native mobile build/store claims public.

## Evidence Required For Public Launch

- `cd backend && go test ./... -timeout 12m`
- `cd frontend && npm run typecheck && npm run test -- --run && npm run lint && npm run build`
- `cd tests/e2e && PLAYWRIGHT_EXPECT_LIVE_STRIPE=1 PLAYWRIGHT_EXPECT_LAUNCH_READY=1 npm run test:launch -- --project=chromium`
- `cd tests/e2e && npm run test:preview-verify -- --project=chromium`
- `APEX_RENDER_EXPECT_PRODUCTION=1 RENDER_API_KEY=... RENDER_BACKEND_SERVICE_ID=... RENDER_FRONTEND_SERVICE_ID=... node scripts/verify_render_launch_env.mjs`
- Production platform build canary matrix: free-fast, paid-balanced, paid-max.
- Stripe webhook replay and controlled live checkout evidence.
- Screenshot/console evidence for generated preview readiness.
- Rollback drill and support/incident checklist reviewed.

## Mobile Launch Position

- Public launch position: source/export and Expo Web preview can be shown truthfully when enabled.
- Native builds, store upload, listing metadata, screenshots, review submission, and store approval remain gated beta until live external-provider evidence exists.
