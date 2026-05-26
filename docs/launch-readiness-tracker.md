# Apex Build Launch Readiness Tracker

Date: 2026-05-26

This tracker reconciles the master launch plan with the current repository state. Code, tests, production config, and live canary evidence remain authoritative.

## Current Branch State

- Branch: `main`
- Local status must be checked with `git status`; this tracker records launch evidence and must not be used as a cleanliness claim.
- 2026-05-26 orchestrator note: the local tree contains launch-readiness doc, builder onboarding, preview verification surfacing, and placeholder-gate hardening changes that are under local verification before push.
- Push dependency: do not store GitHub, Render, Stripe, provider, or customer secrets in repo files, docs, remotes, or logs.

## Closed In This Batch

- No-network prompt matrix fixture breadth guardrail added: `scripts/test/prompt_matrix_fixture_breadth_test.sh` enforces that the 20-prompt `prompts/canary` set stays launch-diverse (6 simple, 8 medium, 6 complex; 8 required capabilities covered) via `prompts/canary/matrix-manifest.json`. The guardrail is wired into `scripts/test/canary_matrix_guardrail_test.sh` and `.github/workflows/production-canary.yml`. Live matrix run evidence remains open; this only prevents the repo from claiming breadth from a narrow frontend-only prompt set.
- TASK-010 load harness added: `scripts/loadtest.js` is a k6 load harness covering 200 concurrent unauthenticated landing+health traffic, optional 50-VU authenticated API load, and optional 10-concurrent build-start polling with build poll tokens. Authenticated and mutating scenarios are fully opt-in, and `scripts/test/loadtest_guardrail_test.sh` validates script shape, thresholds, credential hygiene, login fields, poll-token handling, and k6 syntax without secrets or network traffic. Live authenticated API and build-start evidence remain open.
- TASK-008 backend gate evidence recorded: after timing-test harness stabilization, `cd backend && go test -p 1 -parallel 4 ./... -timeout 20m` passed on `kali` on 2026-05-26 with evidence log `/tmp/backend_full_p1_after_tmp_cleanup_20260526T1650Z.txt`; `go build ./...` also passed. The focused timing-test fixes were pushed in commit `c7df8ec`.
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
- Stripe launch verification now includes a non-mutating live webhook invalid-signature rejection check and an opt-in billing portal probe for existing Stripe customer accounts.
- Production canary workflow now has a manual `run_portal_probe` input for the configured canary Stripe customer.
- Provider cost-threshold skips are classified as provider-level failures so build orchestration can immediately try a cheaper available provider instead of failing the build.
- FSM WebSocket events now hydrate the shared frontend store from both build-stream and collaboration-stream envelopes without creating empty build IDs.
- Preview browser verification now rejects placeholder-only generated dashboards such as generic `KPI 1 / Value` cards and loading-skeleton-only sections before a build can claim preview success.
- Live golden canary tooling now fails on placeholder-only previews and bounds HTTP request hangs with retryable request timeouts.
- Backend browser verification and live golden preview checks now share stricter app-shell/recovery-shell placeholder semantics, and the backend browser proof analyzes full visible text rather than a short DOM-text snippet.
- The live prompt matrix runner now has no-network false-green guardrails for expected prompt count, non-empty/allowed power modes, and exact run counts; the production canary matrix job requires 20 prompt files and depends on script guardrail tests.
- Manually requested production prompt-matrix evidence now fails if canary credentials are absent instead of finishing as a skipped green workflow.
- Builder blank-workspace navigation now preserves the explicit `/ide` no-project intent instead of falling back to the active project, and the main build screen surfaces preview-relevant blockers even when no verification report is attached.
- The post-placeholder live golden planning stall was classified from Render logs: the planner produced a 12-step plan quickly, then orchestration did not advance. Planning tasks now register cancellation, enforce an outer deadline around planner post-processing, stop heartbeats before result handoff, and nil AI-router paths fail the task instead of panicking.
- Live golden Tailwind proof now uses a computed-style probe based on classes already present in the generated app, avoiding false failures when a valid Tailwind build emits fewer than 100 accessible CSS rules.

## Launch Blockers Still Open

- Replay real Stripe test/live-mode webhooks through the configured webhook endpoint for subscription checkout, credit purchase, invoice paid, invoice failed, plan change, subscription deletion, and duplicate event delivery.
- Run a controlled paid live checkout, billing portal, upgrade/downgrade, and cancellation pass before enabling broad public signup. A real customer payment was observed on 2026-05-25, but that is not a substitute for controlled billing-lifecycle evidence.
- Enable `APEX_ENABLE_GITHUB_ACTIONS=true`, configure required GitHub/Render/Stripe/canary secrets, set `APEX_REQUIRE_PAID_CANARIES=true` when using the workflow as launch evidence, and run the production canary workflow against `https://apex-build.dev` and `https://api.apex-build.dev`.
- Verify paid max build, export/deploy handoff, billing upgrade/downgrade, and failed-build restart in production. Paid balanced full-stack passed on 2026-05-25 with build `69d3582e`.
- Run the 20-prompt diverse matrix and launch-concurrency load test; record completion, preview, quality, p95 latency, and error-rate evidence. The `prompts/canary` set now contains 20 topic fixtures with `prompts/canary/matrix-manifest.json` mapping simple/medium/complex categories and required capabilities (`backend_api`, `persistence`, `auth_roles`, `realtime_or_collab`, `file_or_media`, `commerce_or_billing_sim`, `admin_reporting`, `ai_simulation`), and the no-network breadth guardrail enforces the fixture split. `scripts/loadtest.js` now covers public, authenticated API, and build-start load modes, with public-only 200-VU evidence recorded in the runbook. Launch readiness still requires a live 20/20 matrix run plus authenticated API and 10-concurrent build-start load evidence with recorded artifacts/results.
- Run strict mobile external-provider readiness verification with a real mobile project, EAS Build/Submit history, Apple credentials, Google Play credentials, and store-readiness evidence before making native mobile build/store claims public.
- Decide whether Gemini and Grok degraded provider health are acceptable for public launch with 5/7 providers healthy, or fix provider billing/permissions before launch.

## Latest Live Read

- 2026-05-26 12:30 UTC: public `/health` reports `ready=true`, `feature_readiness_status=healthy`, and 5/7 providers healthy. Claude, DeepSeek, GLM, GPT-4, and Ollama report `ok`; Gemini reports `error` from depleted credits/rate limit; Grok reports `auth_error` from permissions or spend limit.
- 2026-05-26 12:30 UTC: public `/health/features` reports `status=healthy`, critical services `6/6 ready`, optional services `40/40 ready`, `code_execution.details.launch_ready=true` through E2B, `preview_service.details.launch_ready=true`, and `preview_runtime_verify` ready with browser proof enabled.
- 2026-05-25: TASK-004 paid balanced full-stack canary functionally passed with build `69d3582e`, `status=completed progress=100 quality_gate_status=passed`, and a live interactive preview. Screenshot/console artifact location is not attached in the current tracker and remains required before treating this as complete launch evidence.
- Historical 2026-05-10 evidence remains useful for prior verifier behavior: strict Render launch verification passed, mobile launch-safe default verification passed, Stripe invalid-signature/non-paid checkout probes passed, Playwright production launch smoke passed `5 passed / 1 skipped`, and fast live golden proof exited `GOLDEN_BUILD_PASSED`. Real Stripe event replay, controlled billing lifecycle, paid max, rollback, restart recovery, load test, and diverse matrix evidence remain open.

## Evidence Required For Public Launch

- Backend gate last passed on 2026-05-26: `cd backend && go test -p 1 -parallel 4 ./... -timeout 20m` on `kali`, evidence log `/tmp/backend_full_p1_after_tmp_cleanup_20260526T1650Z.txt`; `go build ./...` also passed.
- `cd frontend && npm run typecheck && npm run test -- --run && npm run lint && npm run build`
- `cd tests/e2e && PLAYWRIGHT_EXPECT_LIVE_STRIPE=1 PLAYWRIGHT_EXPECT_LAUNCH_READY=1 npm run test:launch -- --project=chromium`
- `cd tests/e2e && npm run test:preview-verify -- --project=chromium`
- `APEX_RENDER_EXPECT_PRODUCTION=1 RENDER_API_KEY=... RENDER_BACKEND_SERVICE_ID=... RENDER_FRONTEND_SERVICE_ID=... node scripts/verify_render_launch_env.mjs`
- Production canary `Launch Verification Scripts` job passing with strict Render secrets configured, paid canary credentials present, and `APEX_REQUIRE_PAID_CANARIES=true` when the workflow is used as launch evidence.
- Production platform build canary matrix: free-fast passed on 2026-05-09, fast frontend-only live golden passed on 2026-05-10, paid-balanced passed on 2026-05-25; paid-max remains open.
- Stripe webhook invalid-signature rejection check in `scripts/verify_stripe_launch.mjs`; real Stripe event replay and controlled live checkout evidence still required.
- Strict mobile external-provider evidence with `APEX_MOBILE_EXPECT_NATIVE_READY=1`.
- Screenshot/console evidence for generated preview readiness, including an archived path or artifact reference for each launch-critical canary.
- Rollback drill and support/incident checklist reviewed.

## Mobile Launch Position

- Public launch position: source/export and Expo Web preview can be shown truthfully when enabled.
- Native builds, store upload, listing metadata, screenshots, review submission, and store approval remain gated beta until live external-provider evidence exists.

---

## 2026-05-25 Launch Readiness Update

### Bugs Fixed Today (4 commits, all on main)

**Root Cause: Ollama/Kimi K2 Cloud forced routing blocked all balanced+max builds**

Prior code forced balanced+platform builds to use `ollama/kimi-k2.6:cloud` for Lead/Planner/Architect roles. Kimi K2 Cloud hangs indefinitely on complex planning tasks with no per-task timeout. Three separate code paths enforced this. Additionally, per-request cost thresholds caused fallback to Ollama (cost=0) when Claude/GPT4 exceeded threshold on large context tasks.

- `513e190`: Remove forced Ollama lead provider for balanced+platform builds
- `cee6b50`: Raise cost thresholds (initial attempt, insufficient)
- `627cb2a`: Raise Claude threshold to $2.00 (also insufficient)
- `5f70ca2`: Remove per-request cost thresholds entirely (correct fix)
- `fc3d81f`: Add rollback drill evidence template to launch-runbook.md

### Canary Evidence

**TASK-004 FUNCTIONAL PASS — Balanced Full-Stack Canary (2026-05-25)**
- Build: `69d3582e` (DocuLens AI SaaS, balanced, 33 files)
- Result: `status=completed progress=100 quality_gate_status=passed`
- Duration: ~13 minutes (17:26 UTC → 17:39 UTC)
- Provider routing: Claude Sonnet 4.6 → planning/lead; DeepSeek/GPT4 → parallel_core
- Preview: live and interactive
- Missing evidence artifact: screenshot/console path is not attached in this tracker yet.

**TASK-005 IN PROGRESS — Max Power Canary (2026-05-25)**
- Build: `f360affa` (Ops Command Center, max, claude-opus-4-7 lead)
- Status: in_progress, architecture phase, actively generating with claude-opus-4-7
- No Ollama fallback observed (cost threshold fix working)

**Prior Canaries (from earlier session)**
- free-fast build: PASSED
- fast-frontend-only build: PASSED

### Test Suite Status

**Frontend (2026-05-25):**
- TypeScript typecheck: PASS (0 errors)
- ESLint: PASS (0 errors)
- Vitest unit tests: 225/225 PASS (34 test files)

**Backend (2026-05-26):**
- Required serialized backend suite: PASS on `kali`.
- Command: `cd backend && go test -p 1 -parallel 4 ./... -timeout 20m`
- Evidence log: `/tmp/backend_full_p1_after_tmp_cleanup_20260526T1650Z.txt`
- Build gate: `go build ./...` PASS.
- Note: timing-sensitive backend agent tests were stabilized in commit `c7df8ec`; use the serialized command above for the full backend gate to avoid local Chrome/package contention.

### Updated Launch Blockers

- [ ] **TASK-005**: Max power canary has no recorded pass yet; last documented build was `f360affa` in progress on 2026-05-25
- [ ] **TASK-006**: Enable `APEX_ENABLE_GITHUB_ACTIONS=true` in GitHub repo variables (requires GitHub admin)
- [ ] **TASK-007**: Rollback drill execution (requires Render dashboard/API key)
- [ ] Stripe webhook replay for all critical events (real events, not just invalid-signature check)
- [ ] Controlled paid checkout, billing portal, upgrade/downgrade, and cancellation pass in production
- [ ] Failed-build restart/recovery, export/deploy handoff, diverse prompt matrix, and launch load test evidence
- [ ] Fix Gemini provider error (`gemini: status=error` in `/health`) or explicitly approve 5/7 provider launch posture
- [ ] Fix Grok auth error (`grok: status=auth_error` in `/health`) or explicitly approve 5/7 provider launch posture

### Latest Server State (2026-05-26 12:30 UTC)
- Public health: healthy and ready, 5/7 providers healthy.
- Runtime readiness: E2B execution launch-ready, preview service launch-ready, browser runtime proof ready.
- Provider caveat: Gemini and Grok remain degraded from credit/spend/permission posture; do not market 7/7 provider readiness until fixed or explicitly accepted as non-critical.
