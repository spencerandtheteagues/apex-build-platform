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

## Launch Blockers Still Open

- Configure real Stripe live/test price IDs matching the current launch contract and confirm `/health/features` no longer reports `payments` degraded.
- Replay Stripe test webhooks for subscription checkout, credit purchase, invoice paid, invoice failed, plan change, and duplicate event delivery.
- Run a controlled live checkout and billing portal flow before enabling broad public signup.
- Confirm Render production has `DATABASE_URL`, `REDIS_URL`, JWT secrets, `SECRETS_MASTER_KEY`, Stripe secrets, provider keys, remote Docker/preview env, and current frontend/backend URLs.
- Run production canary matrix against `https://apex-build.dev` and `https://api.apex-build.dev`.
- Verify free frontend build, paid full-stack build, preview proof, export/deploy handoff, billing upgrade/downgrade, credit top-up, and failed-build restart in production.
- Live-validate EAS Build/Submit, Apple, and Google Play credentials before making native mobile build/store claims public.

## Evidence Required For Public Launch

- `cd backend && go test ./... -timeout 12m`
- `cd frontend && npm run typecheck && npm run test -- --run && npm run lint && npm run build`
- `cd tests/e2e && npm run test:launch -- --project=chromium`
- `cd tests/e2e && npm run test:preview-verify -- --project=chromium`
- Production platform build canary matrix: free-fast, paid-balanced, paid-max.
- Stripe webhook replay and controlled live checkout evidence.
- Screenshot/console evidence for generated preview readiness.
- Rollback drill and support/incident checklist reviewed.

## Mobile Launch Position

- Public launch position: source/export and Expo Web preview can be shown truthfully when enabled.
- Native builds, store upload, listing metadata, screenshots, review submission, and store approval remain gated beta until live external-provider evidence exists.
