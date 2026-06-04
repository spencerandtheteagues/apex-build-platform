# Apex Build Launch Readiness Tracker

Date: 2026-05-26

This tracker reconciles the master launch plan with the current repository state. Code, tests, production config, and live canary evidence remain authoritative.

## Current Branch State

- Branch: `main`
- Local status must be checked with `git status`; this tracker records launch evidence and must not be used as a cleanliness claim.
- 2026-05-26 orchestrator note: the local tree contains backend test-harness reliability changes that made the required serialized backend suite pass under VPS contention. These changes are verified locally but not yet pushed.
- Push dependency: do not store GitHub, Render, Stripe, provider, or customer secrets in repo files, docs, remotes, or logs.

## Closed In This Batch

- No-network prompt matrix fixture breadth guardrail added: `scripts/test/prompt_matrix_fixture_breadth_test.sh` enforces that the 20-prompt `prompts/canary` set stays launch-diverse (6 simple, 8 medium, 6 complex; 8 required capabilities covered) via `prompts/canary/matrix-manifest.json`. The guardrail is wired into `scripts/test/canary_matrix_guardrail_test.sh` and `.github/workflows/production-canary.yml`. Live matrix run evidence remains open; this only prevents the repo from claiming breadth from a narrow frontend-only prompt set.
- TASK-010 load harness added: `scripts/loadtest.js` is a k6 load harness covering 200 concurrent unauthenticated landing+health traffic, optional 50-VU authenticated API load, and optional 10-concurrent build-start polling with build poll tokens. Authenticated and mutating scenarios are fully opt-in, and `scripts/test/loadtest_guardrail_test.sh` validates script shape, thresholds, credential hygiene, login fields, poll-token handling, and k6 syntax without secrets or network traffic. Live authenticated API and build-start evidence remain open.
- TASK-008 backend gate evidence recorded: after timing-test harness stabilization, `cd backend && go test -p 1 -parallel 4 ./... -timeout 20m` passed on `kali` on 2026-05-26 with evidence log `/tmp/backend_full_p1_after_tmp_cleanup_20260526T1650Z.txt`; `go build ./...` also passed. The focused timing-test fixes were pushed in commit `c7df8ec`.
- TASK-008 frontend gate evidence recorded: `cd frontend && npm run typecheck && npm run test -- --run && npm run lint && npm run build` passed locally on 2026-05-26 against current `main` (`38` Vitest files / `267` tests).
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
- Enable `APEX_ENABLE_GITHUB_ACTIONS=true`, configure required GitHub/Render/Stripe/canary secrets, set `APEX_REQUIRE_PAID_CANARIES=true` only when using paid canaries as a hard workflow gate, and run the production canary workflow against `https://apex-build.dev` and `https://api.apex-build.dev`. As of 2026-06-04, `APEX_ENABLE_GITHUB_ACTIONS=true` is configured, Render and canary email/password secrets are present, and `APEX_REQUIRE_PAID_CANARIES=false`. Hermes `GITHUB_TOKEN` confirms the account is GitHub Free with admin access to this public repo and Actions enabled; recent scheduled runs fail before steps because GitHub reports the account is locked due to a billing issue, not because GitHub Pro is required.
- Verify paid max build, export/deploy handoff, billing upgrade/downgrade, and failed-build restart in production. Paid balanced full-stack passed on 2026-05-25 with build `69d3582e`.
- Provision and verify reusable launch canary credentials. Disposable production registrations can reach auth/billing checks, but build start now rejects unverified email accounts; live build canaries require a pre-verified free canary account and a paid canary account.
- Keep the hardened public k6 gate in launch evidence. The lightweight `/ready` readiness-probe fix was deployed in commit `f01dfac` and the post-deploy public 200-VU run passed with zero public 5xx responses; authenticated API and build-start load evidence remain open.
- Run the 20-prompt diverse matrix and launch-concurrency load test; record completion, preview, quality, p95 latency, and error-rate evidence. The `prompts/canary` set now contains 20 topic fixtures with `prompts/canary/matrix-manifest.json` mapping simple/medium/complex categories and required capabilities (`backend_api`, `persistence`, `auth_roles`, `realtime_or_collab`, `file_or_media`, `commerce_or_billing_sim`, `admin_reporting`, `ai_simulation`), and the no-network breadth guardrail enforces the fixture split. `scripts/loadtest.js` now covers public, authenticated API, and build-start load modes, with public-only 200-VU evidence recorded in the runbook. Launch readiness still requires a live 20/20 matrix run plus authenticated API and 10-concurrent build-start load evidence with recorded artifacts/results.
- Run strict mobile external-provider readiness verification with a real mobile project, EAS Build/Submit history, Apple credentials, Google Play credentials, and store-readiness evidence before making native mobile build/store claims public.
- Decide whether Gemini and Grok degraded provider health are acceptable for public launch, or fix provider billing/permissions before launch. Current 2026-06-04 posture is launch-capable only with explicit caveat: core readiness is healthy and Ollama routing is available, but do not market full provider coverage while Gemini/Grok remain degraded.

## Latest Live Read

- 2026-05-30 19:45 UTC: public `/health` reports `ready=true`, `status=healthy`, `feature_readiness_status=healthy`, and 5/7 providers healthy. Claude, DeepSeek, GLM, GPT-4, and Ollama report `ok`; Gemini remains `error` from depleted credits/rate limit; Grok remains `auth_error` from permission/spend limit. Do not market 7/7 provider readiness.
- 2026-05-30 19:45 UTC: public `/health/features` reports `phase=ready`, `critical 6/6 ready`, `optional 40/40 ready`, Redis connected, Stripe payment integration launch-ready, `code_execution.details.launch_ready=true` through E2B, `preview_service.details.launch_ready=true` through reachable preview Docker, and `preview_runtime_verify` browser proof ready.
- 2026-05-30: strict Render launch verification passed locally with Render API credentials and service IDs (`apex-backend` / `apex-frontend`) without printing secret values. Static blueprint checks, Render env-var presence, `/ready`, `/health`, Redis, code execution, preview service, and preview runtime proof all passed.
- 2026-05-30: safe Stripe launch verifier passed against production: `/health/features` payments ready, invalid webhook signatures rejected, throwaway smoke user registered, `/billing/config-status` ready, and self-serve plan ladder returned. Subscription checkout creation, billing portal creation, credit checkout creation, real paid checkout, cancellation, and real webhook replay remain open.
- 2026-05-30: Ollama Cloud model discovery confirmed `kimi-k2.6`, `glm-5.1`, and `deepseek-v4-pro` are available in the account tag list, but generation for all three is blocked by Ollama account weekly usage limit (`HTTP 429`). Ollama-pinned live canaries are blocked until the account is upgraded or extra usage is added.
- 2026-05-30 20:13 UTC: aggregate local launch gate passed with `bash scripts/verify_all.sh`: backend `go build`, `go vet`, full `go test ./... -timeout 12m`, frontend typecheck, Vitest (`38` files / `267` tests), lint, and production build all green.
- 2026-05-30: full serialized backend gate also passed with `cd backend && go build ./... && go vet ./... && go test -p 1 -parallel 4 ./... -timeout 20m`; `internal/preview` remained the slow package at about `376s`, but completed successfully.
- 2026-05-30: safe production Playwright launch smoke passed in Chromium with `5 passed / 1 skipped` using `PLAYWRIGHT_EXPECT_LIVE_STRIPE=1` and `PLAYWRIGHT_EXPECT_LAUNCH_READY=1`. The skipped test is authenticated launch login because `PLAYWRIGHT_LAUNCH_USERNAME/PASSWORD` were not supplied.
- 2026-05-30: safe production preview verification Playwright check passed in Chromium with `3 passed / 4 skipped`. Public preview health/gate/runtime wiring passed; build-generation checks were skipped because `PLAYWRIGHT_PV_USERNAME/PASSWORD` were not supplied.
- 2026-05-30: attempted non-Ollama free frontend live build canary through `scripts/run_platform_build_smoke.sh` with platform routing. Build start was blocked by production email verification for the disposable account (`email_not_verified`), with artifact directory `/tmp/apex-free-canary-20260530T200521Z`. No pre-verified canary credentials were present in the shell or checked env files.
- 2026-05-30: public 200-VU k6 load test against production exposed intermittent `/ready` 503s. The original broad-threshold run exited green with 12 health 503s out of 31,620 requests; after hardening `scripts/loadtest.js` to require `public_5xx_errors count == 0`, the rerun failed correctly with 3 health 503s out of 32,234 requests, landing p95 `32.44ms`, health p95 `634.19ms`, and public error rate `0.00%`. Post-load `/ready` recovered to healthy. Treat zero-5xx public load as open until the local `/ready` fix is deployed and rerun.
- 2026-05-30 21:23 UTC: deployed readiness fix commit `f01dfac` to `main`; Render is configured for backend `autoDeploy: true` and healthCheckPath `/ready`. `/ready` stayed healthy during a 12-poll settling window. Hardened public 200-VU k6 gate passed after deploy: `37,266` total requests, `93,165` checks, `0` failed checks, `public_5xx_errors count=0`, public error rate `0.00%`, landing p95 `32.07ms`, health p95 `99ms`. Post-load `/ready`, `/health`, and `/health/features` all reported healthy.
- 2026-05-30 21:03 UTC: post-`/ready`-fix aggregate local gate passed with `bash scripts/verify_all.sh`: backend build/vet/test and frontend typecheck/Vitest (`38` files / `267` tests)/lint/build all green. Public generated Playwright smoke also passed against production in Chromium (`specs/generated/apex-build-web.smoke.spec.ts`: `4 passed`). Static Render verifier passed without strict credentials; mobile external readiness default-safe verifier passed with strict native/store evidence skipped.
- 2026-06-04: Rollback -> roll-forward drill completed on production. Rollback deploys `dep-d8gbuiv7f7vs73forpq0` (2026-06-03T23:57:01Z) and `dep-d8gbur8g4nts739fvbd0` (2026-06-03T23:57:34Z) were created and later deactivated. Roll-forward deploy `dep-d8gc0r7lk1mc73enkjb0` (2026-06-04T00:01:50Z) is current live. Post-roll-forward `/health/features`: `phase=ready`, `status=healthy`, `critical 6/6`, `optional 40/40`. Commit on the rollback/roll-forward deploys: `0850a89e5b6749a8edd291f9abd75df6c25fbd92`. No further live rollback drill should be run without explicit operator approval.
- 2026-06-04 00:27 UTC: strict Render launch verifier passed locally using Hermes profile env without printing secret values: blueprint checks passed, 94 direct backend Render env vars and 4 frontend env vars loaded, required Stripe/provider/runtime env names were present, `/ready` and `/health` were healthy, Redis was not degraded, `code_execution.details.launch_ready=true`, `preview_service.details.launch_ready=true`, and `preview_runtime_verify` was browser-proof ready.
- 2026-06-04 00:27 UTC: safe Stripe live verifier passed against production with no completed payment: `/health/features` payments ready, invalid webhook signature rejected, throwaway smoke user registered, `/billing/config-status` ready, self-serve plans returned, builder/monthly subscription checkout session created, and `$25` credit checkout session created. Billing portal remained intentionally skipped because it requires an existing Stripe customer account. Real Stripe event replay and controlled paid billing lifecycle remain open.
- 2026-06-04 00:26 UTC: no-network launch guardrails passed locally: `canary_report_test.sh` (31 assertions), `canary_matrix_guardrail_test.sh` (10 top-level assertions plus 18 prompt breadth assertions), `loadtest_guardrail_test.sh` (25 assertions), `render_launch_guardrail_test.sh`, `production_canary_workflow_guardrail_test.sh`, `prompt_matrix_fixture_breadth_test.sh`, and `ollama_credit_saver_env_test.sh`.
- 2026-06-04 00:28 UTC: GitHub production-canary status remains externally blocked by GitHub account state, not by repo config. Latest run `26896313666` from 2026-06-03T15:50:17Z failed before job steps. Check-run annotations state: "The job was not started because your account is locked due to a billing issue."
- 2026-06-04 00:42 UTC: Hermes dashboard `GITHUB_TOKEN` check confirmed `spencerandtheteagues` is on GitHub Free, this repo is public, token has admin repo access, GitHub Actions is enabled, repo secrets exist (`APEX_CANARY_EMAIL`, `APEX_CANARY_PASSWORD`, Render IDs/API key), and repo variables are readable with `APEX_ENABLE_GITHUB_ACTIONS=true`, `APEX_REQUIRE_PAID_CANARIES=false`. Current GitHub billing usage APIs report no current-month Actions charge and 2026 repo Actions usage of 2 Linux minutes at `$0.00`. A paid GitHub account is not the blocker; the account billing lock must be cleared in GitHub account settings or the production canary evidence must be collected via local/Hermes/Render execution.
- 2026-06-04 00:31 UTC: provider launch decision recorded as acceptable only with caveat. `/health` is healthy/ready and reports 5/8 providers healthy: OpenRouter, Ollama, GPT-4, GLM, and DeepSeek are `ok`; Claude is `no_credits`; Gemini is credit/rate-limited; Grok is permission/spend-limited. Fallback order keeps OpenRouter/Ollama/GPT/DeepSeek/GLM available around Gemini/Grok failures. Do not market full provider coverage until Claude/Gemini/Grok billing/permissions are fixed.
- 2026-05-30: no-network script guardrails passed after portability fixes: `bash scripts/test/canary_matrix_guardrail_test.sh`, `bash scripts/test/prompt_matrix_fixture_breadth_test.sh`, `bash scripts/test/loadtest_guardrail_test.sh`, and shell syntax checks for the touched scripts.
- 2026-05-30: focused local validation for current uncommitted launch/onboarding/script changes passed: backend agents recovery tests, `go test ./internal/config ./internal/storage ./cmd -count=1`, frontend `npm run typecheck`, and `npm run test -- --run src/components/builder/OnboardingTour.test.tsx src/App.test.tsx`.
- 2026-05-30: TASK-011 concurrent duplicate invoice-paid regression exists and passed locally with `cd backend && go test ./internal/handlers -run TestHandleInvoicePaidConcurrentDedup -count=1`.
- 2026-05-26 12:30 UTC: public `/health` reports `ready=true`, `feature_readiness_status=healthy`, and 5/7 providers healthy. Claude, DeepSeek, GLM, GPT-4, and Ollama report `ok`; Gemini reports `error` from depleted credits/rate limit; Grok reports `auth_error` from permissions or spend limit.
- 2026-05-26 12:30 UTC: public `/health/features` reports `status=healthy`, critical services `6/6 ready`, optional services `40/40 ready`, `code_execution.details.launch_ready=true` through E2B, `preview_service.details.launch_ready=true`, and `preview_runtime_verify` ready with browser proof enabled.
- 2026-05-26 17:16 UTC: backend TASK-008 gate passed locally on the Kali VPS with `cd backend && go test -p 1 -parallel 4 ./... -timeout 20m` after clearing stale `/tmp` build artifacts; evidence log is `/tmp/backend_full_p1_after_tmp_cleanup_20260526T1650Z.txt`. `cd backend && go build ./...` also passed. The earlier `internal/agents` false-reds were test timing/parallelism issues under VPS contention, not runtime regressions.
- 2026-05-25: TASK-004 paid balanced full-stack canary functionally passed with build `69d3582e`, `status=completed progress=100 quality_gate_status=passed`, and a live interactive preview. Screenshot/console artifact location is not attached in the current tracker and remains required before treating this as complete launch evidence.
- Historical 2026-05-10 evidence remains useful for prior verifier behavior: strict Render launch verification passed, mobile launch-safe default verification passed, Stripe invalid-signature/non-paid checkout probes passed, Playwright production launch smoke passed `5 passed / 1 skipped`, and fast live golden proof exited `GOLDEN_BUILD_PASSED`. Real Stripe event replay, controlled billing lifecycle, paid max, rollback, restart recovery, load test, and diverse matrix evidence remain open.

## Evidence Required For Public Launch

- Backend/frontend aggregate gate last passed on 2026-05-30 with `bash scripts/verify_all.sh`: backend build/vet/test and frontend typecheck/test/lint/build all green.
- Serialized backend gate last passed on 2026-05-30 with `cd backend && go build ./... && go vet ./... && go test -p 1 -parallel 4 ./... -timeout 20m`.
- `cd tests/e2e && PLAYWRIGHT_EXPECT_LIVE_STRIPE=1 PLAYWRIGHT_EXPECT_LAUNCH_READY=1 npm run test:launch -- --project=chromium`
- `cd tests/e2e && npm run test:preview-verify -- --project=chromium`
- `APEX_RENDER_EXPECT_PRODUCTION=1 RENDER_API_KEY=... RENDER_BACKEND_SERVICE_ID=... RENDER_FRONTEND_SERVICE_ID=... node scripts/verify_render_launch_env.mjs`
- Production canary `Launch Verification Scripts` job passing with strict Render secrets configured and canary credentials present. Set `APEX_REQUIRE_PAID_CANARIES=true` only when paid canaries are intentionally a hard workflow gate. Current blocker: GitHub's account billing lock prevents hosted workflow execution even though the account is GitHub Free, the repo is public, Actions is enabled, and secrets/variables are present.
- Production platform build canary matrix: free-fast passed on 2026-05-09, fast frontend-only live golden passed on 2026-05-10, paid-balanced passed on 2026-05-25; a fresh free frontend canary was blocked on 2026-05-30 by email verification for disposable accounts, so verified free and paid canary credentials are required before refreshing build evidence. Paid-max remains open.
- Hardened public k6 load test last passed on 2026-05-30 after deploying `f01dfac`: `k6 run scripts/loadtest.js` with `public_5xx_errors count == 0`, landing p95 `32.07ms`, and health p95 `99ms`.
- Stripe webhook invalid-signature rejection check and non-payment checkout-session probes in `scripts/verify_stripe_launch.mjs`; real Stripe event replay, portal for an existing Stripe customer, and controlled live checkout lifecycle evidence still required.
- Strict mobile external-provider evidence with `APEX_MOBILE_EXPECT_NATIVE_READY=1`.
- Screenshot/console evidence for generated preview readiness, including an archived path or artifact reference for each launch-critical canary.
- Rollback drill and support/incident checklist reviewed. Rollback drill completed 2026-06-04; see `docs/launch-runbook.md` "Completed Rollback/Roll-Forward Drill" section for deploy IDs, timestamps, commit, and health results.

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

**Frontend (2026-05-26):**
- TypeScript typecheck: PASS (0 errors)
- ESLint: PASS (0 errors)
- Vitest unit tests: 267/267 PASS (38 test files)
- Production build: PASS

**Backend (2026-05-26):**
- Required serialized backend suite: PASS on `kali`.
- Command: `cd backend && go test -p 1 -parallel 4 ./... -timeout 20m`
- Evidence log: `/tmp/backend_full_p1_after_tmp_cleanup_20260526T1650Z.txt`
- Build gate: `go build ./...` PASS.
- Note: timing-sensitive backend agent tests were stabilized in commit `c7df8ec`; use the serialized command above for the full backend gate to avoid local Chrome/package contention.

### Updated Launch Blockers

- [ ] **TASK-005**: Max power canary has no recorded pass yet; last documented build was `f360affa` in progress on 2026-05-25
- [ ] Production canary workflow pass: `APEX_ENABLE_GITHUB_ACTIONS=true`, Render secrets, and canary email/password secrets are configured, but a passing workflow run is not recorded. Current external blocker is GitHub's account billing lock; this is not a missing GitHub Pro plan and not a repo config/secrets issue. Use local/Hermes/Render canary execution as the bypass until GitHub-hosted runners are unlocked.
- [x] Rollback drill execution evidence: production rollback -> roll-forward drill completed on 2026-06-04. Rollback deploy `dep-d8gbuiv7f7vs73forpq0` created 2026-06-03T23:57:01Z; roll-forward deploy `dep-d8gc0r7lk1mc73enkjb0` created 2026-06-04T00:01:50Z and live at verification time. Post-roll-forward `/health/features` confirmed `phase=ready`, `status=healthy`, `critical 6/6`, `optional 40/40`. No further live rollback drill should be run without explicit operator approval.
- [ ] Stripe webhook replay for all critical events (real events, not just invalid-signature check)
- [ ] Controlled paid checkout, billing portal for an existing customer, upgrade/downgrade, and cancellation pass in production. Safe non-payment checkout-session probes passed on 2026-06-04.
- [ ] Failed-build restart/recovery, export/deploy handoff, diverse prompt matrix, and launch load test evidence
- [x] Provider posture decision recorded: launch may proceed only with explicit caveat; do not market full provider coverage while Claude/Gemini/Grok are degraded.

### Latest Server State (2026-05-26 12:30 UTC)
- Public health: healthy and ready, 5/7 providers healthy.
- Runtime readiness: E2B execution launch-ready, preview service launch-ready, browser runtime proof ready.
- Provider caveat: Gemini and Grok remain degraded from credit/spend/permission posture; do not market 7/7 provider readiness until fixed or explicitly accepted as non-critical.
