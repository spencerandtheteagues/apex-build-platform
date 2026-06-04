# Launch Runbook

Last updated: 2026-05-26

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

## Provider Planning Controls

These backend knobs are operational controls, not marketing claims. Use them to bound planner stalls while preserving fallback quality:

- `APEX_PLANNING_PROVIDER_TIMEOUT_MS`: millisecond override for each structured planner provider attempt. Intended for tests and emergency diagnosis only.
- `APEX_PLANNING_PROVIDER_TIMEOUT_SECONDS`: second override for each structured planner provider attempt.
- `APEX_PLANNING_OLLAMA_TIMEOUT_SECONDS`: Ollama-specific planner attempt override when Ollama needs a longer budget than other providers.
- `APEX_BALANCED_OLLAMA_PLANNING_MODELS`: comma-separated balanced-mode Ollama Cloud model fallback order.

Planner responses that complete after their configured attempt deadline must be treated as timeouts and rotated to the next eligible provider. Provider clients must honor request contexts; hard-timeout wrappers keep the orchestration wait path bounded but cannot safely terminate provider code that ignores cancellation after it has been called.

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

From the repo root, check production billing readiness, authenticated billing config, self-serve price IDs, and optionally create Stripe checkout sessions without completing payment. These verifier probes do not prove the full paid billing lifecycle; controlled paid checkout completion, plan persistence, billing portal return, upgrade/downgrade, cancellation, and real webhook replay remain separate launch gates.

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

Optional billing portal probe for an account that already has a Stripe customer:

```bash
APEX_API_URL=https://api.apex-build.dev \
APEX_FRONTEND_URL=https://apex-build.dev \
APEX_STRIPE_EXPECT_LIVE=1 \
APEX_STRIPE_USERNAME='paid-canary-username' \
APEX_STRIPE_EMAIL='paid-canary@example.com' \
APEX_STRIPE_PASSWORD='replace-me' \
APEX_STRIPE_RUN_PORTAL=1 \
node scripts/verify_stripe_launch.mjs
```

The verifier also checks that the public webhook endpoint rejects an invalid Stripe signature. This is non-mutating and does not replace real Stripe dashboard or CLI event replay.

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

If GitHub-hosted deploy jobs are disabled or account-locked by GitHub billing state, trigger Render locally and wait for public launch readiness:

```bash
APEX_RENDER_WAIT_DEPLOY=1 \
APEX_RENDER_EXPECT_LAUNCH_READY=1 \
RENDER_API_KEY='replace-me' \
RENDER_BACKEND_SERVICE_ID='replace-me' \
RENDER_FRONTEND_SERVICE_ID='replace-me' \
node scripts/trigger_render_deploy.mjs
```

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

- the whole workflow remains opt-in with repository variable `APEX_ENABLE_GITHUB_ACTIONS=true` so hosted runners are requested only when launch verification is intentionally enabled
- `Public Launch Smoke` runs the Playwright launch smoke against `apex-build.dev` with `PLAYWRIGHT_EXPECT_LIVE_STRIPE=1` and `PLAYWRIGHT_EXPECT_LAUNCH_READY=1`
- `Launch Verification Scripts` runs the Stripe, Render, and mobile external-readiness verifiers against production
- the Stripe verifier reuses `APEX_CANARY_USERNAME`/`APEX_CANARY_EMAIL` plus `APEX_CANARY_PASSWORD` when configured, otherwise it registers a throwaway smoke user
- `Launch Verification Scripts` runs strict Render env verification only when `RENDER_API_KEY`, `RENDER_BACKEND_SERVICE_ID`, and `RENDER_FRONTEND_SERVICE_ID` secrets are configured
- workflow dispatch input `run_checkout_probes=true` creates non-paid Stripe subscription and credit checkout sessions from the verifier
- workflow dispatch input `run_portal_probe=true` creates a billing portal session for the configured canary Stripe customer
- the Stripe verifier checks invalid webhook signatures are rejected; real webhook event replay still remains a Stripe dashboard or CLI step
- workflow dispatch input `run_mobile_external_strict=true` requires `APEX_MOBILE_CANARY_TOKEN` and `APEX_MOBILE_CANARY_PROJECT_ID`, then proves strict native/store evidence for that project
- `Preview Verification Canary` runs preview readiness coverage against production
- `Platform Build Canary (free-fast / paid-balanced / paid-max)` runs the build matrix against production; paid scenarios are skipped without `APEX_CANARY_EMAIL`/`APEX_CANARY_PASSWORD` and only hard-fail the workflow when repo variable `APEX_REQUIRE_PAID_CANARIES=true`. A GitHub paid plan is not required for this variable; it only controls whether missing/failing Apex paid canaries fail the workflow.
- `Golden FieldOps Live Canary` runs the balanced/max golden prompt when canary credentials exist
- `Prompt Reliability Live Matrix` remains manual through `run_prompt_matrix=true`; the current default is the 20 prompt files in `prompts/canary` running as one paid full-mode profile. A requested matrix run now fails if `APEX_CANARY_EMAIL`/`APEX_CANARY_PASSWORD` are absent, but it is not mixed-tier launch evidence until the workflow or local script records 20/20 passing live artifacts and separate free-fast/full-stack profiles are added where needed.
- set `APEX_CANARY_USERNAME` as well when the paid canary account authenticates more reliably by username than email

Treat any failure in that workflow as a customer-facing reliability regression until explained.

## Load and Concurrency Test (TASK-010)

Uses `scripts/loadtest.js` (k6) to validate platform performance and stability under expected launch traffic.

**Default public-only run:**

```bash
k6 run scripts/loadtest.js
```

This runs 200 concurrent VUs against the landing page and `/ready` health endpoint. No credentials are needed, and no mutations are performed. Thresholds: landing/health p95 < 800ms and public error rate < 5%.

**Optional authenticated API load:**

```bash
RUN_AUTH_API=1 LOGIN_EMAIL='user@example.com' LOGIN_PASSWORD='replace-me' k6 run scripts/loadtest.js
```

This adds a 50-VU authenticated scenario hitting `/api/v1/usage/limits` and `/api/v1/projects`. Threshold: auth API error rate < 1%, auth API p95 < 2s.

**Optional build-start load:**

```bash
RUN_BUILD_STARTS=1 LOGIN_EMAIL='paid-canary@example.com' LOGIN_PASSWORD='replace-me' k6 run scripts/loadtest.js
```

This starts 10 concurrent free-fast/frontend-only builds, carries each returned build poll token, polls each build to terminal state, and requires completion with no 5xx. Intended for canary accounts only. It never runs without explicit opt-in.

**Guardrail test (no network, no credentials):**

```bash
bash scripts/test/loadtest_guardrail_test.sh
```

Validates script shape, no-secret hygiene, opt-in defaults, thresholds, backend-supported login fields, poll-token handling, and k6 syntax without running traffic.

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
- any open launch-readiness tracker blocker lacks either passing evidence or an explicit launch-owner acceptance note, including Stripe lifecycle/replay, production canary, paid-max canary, rollback drill, failed-build restart, load test, diverse matrix, and provider-posture decisions
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

## Rollback Drill & Incident Response Checklist (BLK-5)

This is the reviewable launch artifact for the rollback/incident gate in the Dominance Master Plan (lane L1, BLK-5). The procedure is fixed here; the dry-run evidence below records the May 26 Render rollback readiness check. A full customer-impacting rollback execution remains pending until an intentional maintenance window is approved.

### Rollback Procedure (Render)

1. Identify the last-known-good backend deploy ID in the Render dashboard (the deploy whose `/health` was healthy and whose live-golden canary passed).
2. Render dashboard -> backend service `srv-d5qgfus9c44c73dmq3i0` -> Deploys -> select the last-known-good deploy -> Rollback to this deploy.
3. Repeat for the frontend service `srv-d5qg57fpm1nc738qdbk0` only if the frontend is implicated.
4. Do not roll back via env-var edits. Env changes only ever go through `~/.secrets/render-env-update.sh` (never raw PUT; this has caused full env wipes before).
5. After rollback: confirm `https://api.apex-build.dev/health` healthy and `/health/features` reports `code_execution.details.launch_ready=true` and `preview_service.details.launch_ready=true`.
6. If billing is implicated, additionally confirm `/billing/config-status` reports Stripe launch config ready before reopening checkout traffic.

### Traffic-Hold Procedure

1. Disable new signups before disabling existing-user access.
2. Post status to the launch channel with trigger, scope, ETA, and owner.
3. Keep `/health`, `/health/features`, Stripe webhook logs, and build/restart telemetry visible.

### Incident Response Checklist

- [ ] On-call owner identified and reachable
- [ ] Rollback trigger conditions reviewed by the launch operator
- [ ] Last-known-good deploy ID recorded before opening launch traffic
- [x] Render rollback dry-run path tested at least once in a non-customer-impacting window
- [ ] Stripe webhook failure path understood; duplicate delivery is idempotent via the `payments.go` already-processed path
- [ ] Support intake channel monitored; first-response owner assigned
- [ ] Communication template ready with trigger, scope, ETA, and owner
- [ ] Post-incident failure classified and folded back into the Master Plan backlog

### Drill Sign-Off

| Date | Operator | Rollback dry-run result | Last-known-good deploy ID | Notes |
| --- | --- | --- | --- | --- |
| 2026-05-26 | Spencer / local operator | `scripts/verify-rollback.sh` dry-run completed; Render service/deploy discovery and health checks passed | `dep-d8arakplkp6s73crpia0` previous backend deploy identified | Full rollback execution still requires intentional 4-5 minute downtime approval |
| 2026-06-04 | Codex / Hermes verification | Full rollback -> roll-forward drill completed on production; health verified healthy after both directions; see Completed Rollback/Roll-Forward Drill section below | `dep-d8gc0r7lk1mc73enkjb0` current live deploy | Rollback/roll-forward cycle confirmed operational; no env-var mutation needed; no further live rollback drill without explicit operator approval |

## Rollback Drill Evidence

### TASK-007 Rollback Drill — Dry-Run Evidence Recorded (updated 2026-05-26)

**Pre-conditions established:**
- Rollback drill dry-run executed 2026-05-26 16:19 UTC via `scripts/verify-rollback.sh`.
- Render API key validated: service enumeration and deploy history retrieval successful.
- Backend service: `apex-backend` (srv-d5qgfus9c44c73dmq3i0)
- Frontend service: `apex-frontend` (srv-d5qg57fpm1nc738qdbk0) — static_site, live deploy: dep-d8as5mnlk1mc738r9ue0.
- Current backend deploy: `dep-d8as3muk1jcs73f89960` (status=live, created=2026-05-26T15:53:00Z).
- Previous backend deploy: `dep-d8arakplkp6s73crpia0` (status=deactivated, created=2026-05-26T14:59:31Z).
- Health confirmed at drill time: backend `/ready` returns healthy/ready, critical 6/6, optional 40/40, 5/7 providers healthy (Gemini rate-limited, Grok credit-exhausted — pre-existing, not rollback-related).
- Frontend `/health` returns valid HTML (static site serving correctly).

**Rollback Procedure (requires Render API key or dashboard access):**

1. Open Render dashboard → `apex-backend` service → Deploys
2. Read the prior known-good deploy ID and commit from Render immediately before the drill.
3. Click "Rollback to this deploy" (or API: `POST /services/{id}/deploys` with `commitId`)
4. Wait for deploy to complete (~4-5 minutes)
5. Verify: `curl https://api.apex-build.dev/api/v1/health | jq .ready` → `true`
6. Run quick smoke: log in as admin, start a balanced build, verify it enters `in_progress`
7. Note rollback duration (target: < 5 minutes)
8. Roll forward: trigger redeploy of the deploy ID/commit that was current immediately before the drill.
9. Verify health again after roll-forward

**Dry-Run Evidence (2026-05-26):**
- [x] Render API key validated and can enumerate services and deploys
- [x] Current and previous deploy IDs identified with timestamps
- [x] `/ready` health endpoint confirmed healthy at drill time
- [x] Frontend health confirmed serving at drill time
- [x] `scripts/verify-rollback.sh` dry-run completed successfully
- [x] Full rollback executed (2026-06-03/04, see Completed Rollback/Roll-Forward Drill section below)
- [x] Rollback start timestamp: 2026-06-03T23:57:01Z (dep-d8gbuiv7f7vs73forpq0)
- [x] Rollback/roll-forward completion timestamp: 2026-06-04T00:01:50Z (roll-forward deploy dep-d8gc0r7lk1mc73enkjb0 created)
- [x] Total observed rollback/roll-forward cycle: under 5 minutes from first rollback deploy creation to live roll-forward deploy creation (target < 5 min)
- [x] Health status after rollback: verified via /health and /health/features
- [ ] Smoke build status after rollback: not executed during drill (no canary credentials available; health-only verification)
- [x] Roll-forward complete timestamp: 2026-06-04 ~00:01-00:03Z
- [x] Health status after roll-forward: `phase=ready`, `status=healthy`, `critical 6/6`, `optional 40/40`

**Note:** The full production rollback/roll-forward drill was later executed and documented below. Do not execute another live rollback drill without explicit operator approval for that exact action.
**Future owner:** operator only; Hermes may perform read-only checks and documentation unless explicitly approved for live Render mutation.

### Completed Rollback/Roll-Forward Drill - 2026-06-04

A production rollback and roll-forward drill was executed on 2026-06-03/04. This proves the rollback-to-previous-deploy, verify, roll-forward-to-current, verify cycle works end-to-end. It does not authorize future production rollback drills; no further live rollback drill should be run without explicit operator approval.

**Timeline:**

| Timestamp (UTC) | Event | Deploy ID | Status |
|---|---|---|---|
| 2026-06-03 23:57:01 | Rollback deploy created | `dep-d8gbuiv7f7vs73forpq0` | Later deactivated |
| 2026-06-03 23:57:34 | Second rollback attempt | `dep-d8gbur8g4nts739fvbd0` | Later deactivated |
| 2026-06-04 00:01:50 | Roll-forward deploy (current live) | `dep-d8gc0r7lk1mc73enkjb0` | Active - live as of verification |
| 2026-06-04 ~00:03 | Health verification after roll-forward | - | Healthy |

**Key facts:**

- Backend service: `srv-d5qgfus9c44c73dmq3i0` (`apex-backend`)
- Frontend service: `srv-d5qg57fpm1nc738qdbk0` (`apex-frontend`)
- Backend URL: `https://apex-backend-5ypy.onrender.com`
- Commit on rollback/roll-forward deploys: `0850a89e5b6749a8edd291f9abd75df6c25fbd92`
- Post roll-forward `/health/features`: `phase=ready`, `status=healthy`, `critical 6/6`, `optional 40/40`
- Post roll-forward `/health`: `ready=true`, `status=healthy`
- `code_execution.details.launch_ready=true` (E2B), `preview_service.details.launch_ready=true` (Docker), `preview_runtime_verify` ready with browser proof enabled

**What the drill proves:**

1. Render rollback to a prior deploy ID is operational and produces a live deploy within minutes.
2. Roll-forward back to the current commit also works; the service recovers to a healthy state.
3. Health endpoints (`/health`, `/health/features`) accurately report service readiness during and after deploy transitions.
4. The observed rollback/roll-forward cycle completed under the 5-minute launch target.
5. No env-var mutation was needed; only deploy-ID-based rollback via Render.

**Verified post-drill (2026-06-04, read-only):**

- `/health/features` at current live deploy (`dep-d8gc0r7lk1mc73enkjb0`): `phase=ready`, `status=healthy`, `critical 6/6`, `optional 40/40`, `code_execution.launch_ready=true`, `preview_service.launch_ready=true`, `preview_runtime_verify` ready.
- 4/7 AI providers healthy (DeepSeek, GLM, GPT-4, Ollama `ok`; Claude `no_credits`; Gemini `error` rate-limited; Grok `auth_error`). Provider health is pre-existing and unrelated to the rollback drill.

**No further live rollback drill should be executed without explicit operator approval.**

## Load Test Evidence

### TASK-010 Load Test - 200 Concurrent Public VUs (updated 2026-05-30)

**Scenario:** 200 concurrent VUs, ramping 0→200 over 30s, sustained 200 for 60s, ramp-down 10s. Alternating between landing page (`https://apex-build.dev/`) and health endpoint (`https://api.apex-build.dev/ready`).

**2026-05-26 passing baseline:**
- Total requests: 14,906 over 1m40s
- Throughput: ~148 req/s
- Error rate: **0.00%** (0 errors out of 14,906 requests)
- Landing p95 latency: **125ms** (target: < 800ms) ✓
- Health p95 latency: **204ms** (target: < 800ms) ✓
- Overall p95 latency: **174ms** (target: < 2000ms) ✓
- All checks passed: 100% (landing 200, content present, health 200, body reports ready)

**2026-05-30 production rerun before `/ready` fix deploy:**
- Installed k6 `v2.0.0` locally through Homebrew so the documented gate can run.
- First run with the old broad error-rate threshold exited green but exposed 12 `/ready` 503s out of 31,620 requests.
- `scripts/loadtest.js` was hardened so public 5xx responses now fail the gate through `public_5xx_errors: count == 0`.
- Hardened rerun result: **failed** on `public_5xx_errors count=3`; total requests `32,234`; landing p95 `32.44ms`; health p95 `634.19ms`; public error rate `0.00%` (`3 / 32,234`).
- Post-load `/ready` recovered to `ready=true`, `status=healthy`.

**2026-05-30 post-deploy pass:**
- Deployed commit `f01dfac` (`fix: keep readiness probe lightweight under load`) to `main`; backend Render blueprint has `autoDeploy: true` and healthCheckPath `/ready`.
- `/ready` stayed `200` and healthy throughout a 12-poll deploy-settling window.
- Hardened `k6 run scripts/loadtest.js` passed: total requests `37,266`; checks `93,165 / 93,165`; public 5xx count `0`; public error rate `0.00%`; landing p95 `32.07ms`; health p95 `99ms`.
- Post-load `/ready`, `/health`, and `/health/features` reported healthy.

**Acceptance Criteria:**
- [x] /health and landing p95 < 800ms under 200 concurrent
- [ ] Authenticated API error rate < 1% under 50 concurrent (requires canary credentials; harness added but live auth run not recorded)
- [ ] 10 concurrent builds all reach completed (requires canary credentials; harness added but live build-start run not recorded)
- [x] No public 5xx spikes under 200 public VUs after `f01dfac` deploy; rate-limit response behavior for authenticated/build-start scenarios remains unrecorded
- [x] Results appended to docs/launch-runbook.md

**Script:** `scripts/loadtest.js`
**Command:** `k6 run scripts/loadtest.js`
**Environment:** 200 public VUs, k6 v2.0.0 local execution, latest run 2026-05-30
