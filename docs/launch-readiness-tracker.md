# Apex Build Launch Readiness Tracker

Date: 2026-05-09

This tracker reconciles the master launch plan with the current repository state. Code, tests, production config, and live canary evidence remain authoritative.

## Current Branch State

- Branch: `main`
- Local status after push: clean and even with `origin/main` at `1b5286e`.
- Push dependency: do not store GitHub, Render, Stripe, provider, or customer secrets in repo files, docs, remotes, or logs.

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
- Mobile external-provider readiness is now scripted through `scripts/verify_mobile_external_readiness.mjs` to keep native build/store-upload claims gated until real project evidence exists.
- Production canary now runs the Stripe, Render, and mobile launch verifier scripts when `APEX_ENABLE_GITHUB_ACTIONS=true`; public launch smoke also enforces runtime launch readiness.
- Render backend Docker builds now compile the full `backend/cmd` package so deployment includes startup launch-readiness and admin-promotion files.
- Production file migration `000014_mobile_project_snapshot_metadata` adds the mobile project/snapshot columns and `mobile_submission_jobs` table that production file migrations were missing.
- Stripe launch verification now supports deployed cookie-session auth and CSRF-protected checkout probes.
- Provider cost-threshold skips are classified as provider-level failures so build orchestration can immediately try a cheaper available provider instead of failing the build.

## Launch Blockers Still Open

- Replay real Stripe test webhooks through the configured webhook endpoint for subscription checkout, credit purchase, invoice paid, invoice failed, plan change, and duplicate event delivery.
- Run a controlled paid live checkout, billing portal, upgrade/downgrade, and cancellation pass before enabling broad public signup.
- Enable `APEX_ENABLE_GITHUB_ACTIONS=true` and run the production canary workflow against `https://apex-build.dev` and `https://api.apex-build.dev`.
- Verify paid full-stack build, paid max build, export/deploy handoff, billing upgrade/downgrade, and failed-build restart in production.
- Run strict mobile external-provider readiness verification with a real mobile project, EAS Build/Submit history, Apple credentials, Google Play credentials, and store-readiness evidence before making native mobile build/store claims public.

## Latest Live Read

- 2026-05-09 03:22 UTC: Render backend deploy `dep-d7vadhlbbn2s73bi4dc0` is live on code commit `2358a30`; repo docs were then updated at `1b5286e`.
- Public `/health` is healthy and ready with startup `2026-05-09T03:25:29.548635607Z` after the final deploy.
- Strict Render launch verification passed: Render env-var presence was verified without printing secret values, Redis was ready, `code_execution.details.launch_ready=true`, `preview_service.details.launch_ready=true`, and `preview_runtime_verify` was browser-proof ready.
- Mobile external readiness verifier passed its launch-safe default gate: native EAS builds and store submission remain gated until real project/provider/store evidence exists.
- Stripe launch verifier passed strict production readiness and created non-paid checkout sessions for Builder monthly and `$25` credits. Live webhook replay and controlled paid checkout remain external evidence gaps.
- Playwright production launch smoke passed `5 passed / 1 skipped` and preview verification smoke passed `3 passed / 4 skipped`.
- Production free frontend platform smoke completed build `a04e49ec-d18e-4202-b8ce-56a5ed85b88a` with `ASSERTIONS_PASSED profile=free_frontend power_mode=fast`.

## Evidence Required For Public Launch

- `cd backend && go test ./... -timeout 12m`
- `cd frontend && npm run typecheck && npm run test -- --run && npm run lint && npm run build`
- `cd tests/e2e && PLAYWRIGHT_EXPECT_LIVE_STRIPE=1 PLAYWRIGHT_EXPECT_LAUNCH_READY=1 npm run test:launch -- --project=chromium`
- `cd tests/e2e && npm run test:preview-verify -- --project=chromium`
- `APEX_RENDER_EXPECT_PRODUCTION=1 RENDER_API_KEY=... RENDER_BACKEND_SERVICE_ID=... RENDER_FRONTEND_SERVICE_ID=... node scripts/verify_render_launch_env.mjs`
- Production canary `Launch Verification Scripts` job passing with strict Render secrets configured.
- Production platform build canary matrix: free-fast passed on 2026-05-09; paid-balanced and paid-max remain.
- Stripe webhook replay and controlled live checkout evidence.
- Strict mobile external-provider evidence with `APEX_MOBILE_EXPECT_NATIVE_READY=1`.
- Screenshot/console evidence for generated preview readiness.
- Rollback drill and support/incident checklist reviewed.

## Mobile Launch Position

- Public launch position: source/export and Expo Web preview can be shown truthfully when enabled.
- Native builds, store upload, listing metadata, screenshots, review submission, and store approval remain gated beta until live external-provider evidence exists.
