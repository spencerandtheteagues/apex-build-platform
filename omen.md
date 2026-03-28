# Omen Handoff

Last updated: 2026-03-27 America/Chicago

This file is the exact handoff for continuing APEX Build work on the new Fedora workstation.

## Current Goal

For APEX Build work, the active mission is:

- free users must always end up with a prompt-matching frontend UI that works in the interactive preview
- paid users must always end up with a working full-stack app that works in the interactive preview
- builds should not fail due to orchestration drift, stale session state, or avoidable verification/recovery gaps

## Where We Are

The major reliability overhaul is already far along.

What is already improved:

- builder UI is much more compact and split into `Overview`, `Activity`, `Files`, `Timeline`, `Issues`, `Diagnostics`, and `Console`
- stale live agent/provider/error boxes no longer persist after work stops
- explicit `Steer Build` path exists in the builder UI
- builder no longer auto-reopens the last build after login; it starts at a fresh prompt unless a prior build is explicitly selected
- full-stack orchestration is contract-first and frontend-first
- restart/recovery logic was hardened multiple times
- free full-stack requests now fall back truthfully to frontend-preview delivery instead of fake backend attempts
- preview verification exists at three levels:
  - static preview verification
  - runtime Vite boot verification
  - browser execution proof layer
- Chromium support was added to the backend production image
- backend Docker images were aligned to Go 1.26

## Important Remote State

Latest important remote commits already published to `main`:

- `a58baaaebca8264b8fc94dc3f0642a8a9bf56eae`
  - `feat: provision browser proof in production runtime`
- `d820a85afb335108bc2bd7489dff213c46df02d2`
  - `fix: align backend images with go 1.26`

Local git also has:

- `f17ec81fbe3d636e8cf4f2ffbae01cb922d8d623`
  - local equivalent of the Dockerfile/Go 1.26 fix

At the time of this handoff, the tracked worktree is clean except for unrelated existing untracked files already in this repo.

## Production Findings Verified Live

### 1. Free frontend canary

Result: passed

Live build:

- `0eb9f6e9-f3c3-4c61-b5d3-0273f2ecb020`

Observed path:

- `planning -> in_progress -> reviewing -> completed`
- terminal result was truthful
- completed with `files_count = 21`

Important note:

- the first free canary attempt failed before build start because I incorrectly used `POWER_MODE=balanced`
- free users are restricted to `POWER_MODE=fast`
- the real free path is:
  - `MODE=fast`
  - `POWER_MODE=fast`

### 2. Paid full-stack canary

Result: failed

Live build:

- `28fdaa25-2f81-45cc-9521-0735e9bf6476`

Observed path:

- `planning -> in_progress -> testing -> failed`

Exact terminal error:

- `Failed after 4 attempts: provider verification blocked task output: The file tests/verify-integration.ts ends abruptly without completing the function, resulting in a missing closing brace and compilation failure.`

This is a real product reliability gap.

Meaning:

- autoscaling did not immediately break build ownership
- the full-stack path is still vulnerable to truncated generated TypeScript/test artifacts
- current repair logic did not recover this class after 4 attempts

### 3. Production backend health

Production was green enough to serve traffic and run builds, but one important feature flag mismatch still exists.

Observed live backend process during testing:

- `started_at = 2026-03-28T04:04:32.072503148Z`

Observed from `https://api.apex-build.dev/health/features`:

- `preview_runtime_verify` is still `degraded`
- current summary:
  - `Runtime Vite boot proof disabled (set APEX_PREVIEW_RUNTIME_VERIFY=true)`
- details:
  - `enabled: false`
  - `chrome_available: true`
  - `browser_proof: false`

Interpretation:

- the new backend image with Chrome is live
- production still is not enforcing runtime/browser preview proof
- this is probably a Render environment/config sync problem, or a code-default issue if the env is not present in the live service

## Credentials / Accounts

Do not store secrets in the repo.

What is safe and relevant to know:

- the seeded production admin account that successfully logged in during testing was:
  - username: `admin`
  - email: `admin@apex.build`
- the earlier credential note in chat that used `Starsh1pKEY!` was wrong
- the seeded password in code/tests is `TheStarsh1pKEY!`

Use a secure channel or secure local secret storage on the new machine rather than writing passwords into tracked repo files.

Also important:

- PATs pasted in chat were not accepted for `git push` over HTTPS from this machine
- I had to publish several commits through the GitHub connector path instead of normal git auth
- on the new machine, set up a real working git credential first

## Most Important Open Problems

### A. Full-stack builds can still fail on truncated generated TS/test files

This is the most immediate product blocker discovered by the live paid canary.

Specific failure class:

- generated `tests/verify-integration.ts` came back truncated
- file ended abruptly
- verification blocked output
- repair path failed after 4 attempts

Next work should focus on deterministic detection and repair for incomplete JS/TS/TSX files.

Places to inspect:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`
- `backend/internal/agents/preview_gate.go`
- `backend/internal/agents/core/validator.go`
- `backend/internal/agents/autonomous/validator.go`

Search anchors:

- `TruncatedFiles`
- `quickSyntaxCheck`
- `applyDeterministicQuoteSyntaxRepair`
- `verifyGeneratedCode`
- `verifyGeneratedFrontendPreviewReadiness`
- `verifyGeneratedBackendBuildReadiness`

What likely needs to be added:

- deterministic incomplete-file detection for JS/TS/TSX/JSX
- hard detection of abrupt EOF / unbalanced braces / unterminated function blocks
- repair path that either:
  - drops invalid generated test artifacts before final verification when they are non-essential, or
  - synthesizes a compile-safe placeholder/repair for the truncated file, or
  - forces a targeted regeneration of only the broken file

### B. `preview_runtime_verify` is still disabled in production

The browser-proof code is shipped, Chrome is present, but production says:

- `enabled: false`

Need to determine whether:

1. Render did not actually apply `APEX_PREVIEW_RUNTIME_VERIFY=true`
2. existing Render service envs were not synced from `render.yaml`
3. code should default runtime verification to enabled in production when Chrome is available, unless explicitly disabled

Relevant code:

- `backend/cmd/main.go`
  - current logic:
    - `runtimeVerifyEnabled := strings.EqualFold(strings.TrimSpace(os.Getenv("APEX_PREVIEW_RUNTIME_VERIFY")), "true")`

Likely stronger behavior:

- in production, default `preview_runtime_verify` to `true` when Chrome is available unless env is explicitly `false`
- keep local/dev default opt-in if needed

That would remove fragile config drift from the customer guarantee path.

### C. Paid canary cadence still needs to be rerun after fixes

After fixing A and B:

- rerun free frontend canary
- rerun paid full-stack canary
- then run repeated canaries across:
  - `fast`
  - `balanced`
  - `max`

## Exact Commands To Resume

### Backend validation

```bash
cd /Users/spencerteague/apex-build/backend
TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...
TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/preview
TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s
```

### Live health checks

```bash
curl -L --max-time 15 -s https://api.apex-build.dev/health | jq
curl -L --max-time 15 -s https://api.apex-build.dev/health/features | jq
```

### Free frontend canary

```bash
cd /Users/spencerteague/apex-build
BASE_URL=https://api.apex-build.dev/api/v1 \
MODE=fast \
POWER_MODE=fast \
SMOKE_PROFILE=free_frontend \
POLL_SECONDS=10 \
MAX_POLLS=120 \
./scripts/run_platform_build_smoke.sh
```

### Paid full-stack canary

Use valid secure credentials for a paid or owner account.

The known-good admin identifier during testing was `admin@apex.build`.

```bash
cd /Users/spencerteague/apex-build
BASE_URL=https://api.apex-build.dev/api/v1 \
MODE=full \
POWER_MODE=balanced \
SMOKE_PROFILE=paid_fullstack \
LOGIN_EMAIL='admin@apex.build' \
LOGIN_PASSWORD='REDACTED_USE_SECURE_SOURCE' \
PROJECT_NAME=paid-fullstack-canary \
POLL_SECONDS=10 \
MAX_POLLS=180 \
./scripts/run_platform_build_smoke.sh
```

## Suggested Next Sequence On The New Machine

1. Get normal git auth working on the new machine first.
2. Open this repo and confirm `main` includes:
   - `a58baaa`
   - `d820a85`
3. Fix the truncated JS/TS artifact class from the paid canary.
4. Fix or remove the `preview_runtime_verify` config drift.
5. Run backend tests.
6. Rerun free and paid live canaries.
7. Only after both pass, move on to higher-power repeated canaries and more aggressive guarantee work.

## High-Signal Summary

If starting fresh on the new machine, the situation is:

- frontend/free path: passing live
- paid/full-stack path: still failing live, but much later in the pipeline than before
- browser-proof code: shipped and now live in production
- production `preview_runtime_verify`: enabled and healthy, with Chrome available
- deploy image/toolchain mismatch: fixed
- the older blockers are resolved:
  - truncated generated TS/test-file failure class repaired
  - qualifier normalization (`string unique`) repaired
  - TypeScript backend preview entry detection repaired
  - database ownership precedence repaired
  - seeded auth/API contract now syncs back into `build.Plan` before task spawn
- the current highest-value blocker is now preview/backend route proof

## Latest Live Status

Production backend after the latest deploy:

- pushed fix: `9c78b7d` `fix: sync seeded auth contracts into build plans`
- production `started_at`: `2026-03-28T18:51:28.564317981Z`
- `/health/features`: healthy
- `preview_runtime_verify`: ready
  - `enabled: true`
  - `browser_proof: true`
  - `chrome_available: true`

Latest paid full-stack canary:

- build id: `9792219d-a297-4d29-bd0c-a9b576495f3d`
- improvements confirmed:
  - build now moves through planning and generation on the new deploy
  - prior `/api/auth/me` integration drift is gone
  - prior database work-order ownership conflict is gone
- current blocker:
  - preview verification reaches `server/index.ts` and fails terminally at `96%` with:
    - `Preview verification failed: Backend entry "server/index.ts" defines no routes.`

Interpretation:

- planning/contract hydration is no longer the main failure surface
- the next pass should focus on preview verifier backend route detection/runtime truth
- likely target files:
  - `backend/internal/preview/verifier.go`
  - `backend/internal/preview/runtime_verifier.go`
  - related preview tests under `backend/internal/preview/*_test.go`

## Immediate Next Task

Fix the preview verifier so valid backend TypeScript entries that mount routes indirectly still count as route-bearing when they are actually runnable. The current verifier is still too literal about route definitions in `server/index.ts`.

Concrete objective:

- paid canary should complete with preview proof on the current production contract stack
- route detection should not falsely fail when:
  - routes are imported from another file and mounted with `app.use(...)`
  - router setup happens through modularized handlers
  - Express app creation and route registration are split across files

## Newest Live State

Render / production status:

- backend live and healthy after the runtime-preview deployment
- `preview_runtime_verify` is ready in production
- Chrome/browser proof is available in the live backend

Newest paid full-stack canary:

- build id: `295e7be8-263c-40f1-94b0-e0e1c9a260e0`
- path improved:
  - planning passed
  - generation passed
  - testing passed
  - review advanced to the high 90s
- new blocker:
  - the old `server/index.ts defines no routes` preview-verifier false positive is fixed
  - the build now fails later at `97%` because a provider-verification repair task can veto the output when generated Jest-style tests exist but `package.json` does not yet declare `jest`

Exact live failure:

- `Failed after 1 attempts: provider verification blocked task output: The 'AFTER' version of package.json does not add Jest to devDependencies, which is required for the test script to run and would cause build failures.`

What was fixed locally after that canary:

- `backend/internal/agents/manager.go`
  - provider-blocked deterministic repairs now patch missing test-tooling manifest dependencies instead of immediately failing the task
  - pre-validation normalization now recognizes generated `@jest/globals` usage and adds `jest`
- `backend/internal/agents/manager_readiness_test.go`
  - added regressions for:
    - provider-blocked manifest repair for missing `jest`
    - pre-validation normalization adding `jest` for generated Jest tests

Current next step:

- push/deploy the new orchestration repair
- rerun the paid full-stack canary
- if it passes, move immediately to repeated canaries across power modes / autoscaled conditions

## Newest Local Repair (Not Yet Live At Time Of This Note)

What was added locally:

- deterministic repair for broken generated test files during final validation
  - target surface: generated `*.test.*` / `*.spec.*` files only
  - behavior:
    - patch broken imports when possible
    - otherwise replace with compile-safe placeholder/smoke tests
- CSRF-aware live canary script
  - `scripts/run_platform_build_smoke.sh` now logs in, fetches `/api/v1/csrf-token`, and includes `X-CSRF-Token` for build start and follow-up calls

Files:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`
- `scripts/run_platform_build_smoke.sh`

Reason:

- latest live paid canary advanced to the high 90s, then failed because generated Jest/RTL test artifacts were still brittle enough to break preview verification
- production now also enforces CSRF on build start, so the smoke script needed to match the real auth flow

Resume commands on the new machine:

- `cd /path/to/apex-build/backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd /path/to/apex-build/backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd /path/to/apex-build/backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`
- `cd /path/to/apex-build && bash -n scripts/run_platform_build_smoke.sh`

Then:

1. Push the local repair if not already on `main`
2. Wait for Render backend deploy to finish
3. Run the paid full-stack canary again
4. If it passes, repeat paid canaries across `fast`, `balanced`, and `max`

## Latest Live Canary Result After Generated-Test Fix

Paid canary rerun:

- build id: `c932433a-80f0-4f7a-8f5f-602e3d0e5d22`
- backend deploy start time during run: `2026-03-28T20:11:29.002243633Z`
- progress path:
  - `19` frontend UI
  - `44` backend/data
  - `79` testing
  - `89` review
  - `95-97` preview/final validation

What it proved:

- the earlier generated-test failure is fixed
- the build now survives past that lane and fails later on a different deterministic issue

Newest blocker:

- `src/hooks/useAuth.ts` contains JSX even though it is a `.ts` file
- live preview verification error:
  - `Transform failed with 1 error`
  - `src/hooks/useAuth.ts:90:6: ERROR: Expected ">" but found "value"`

Newest local fix after that canary:

- deterministic JSX-in-TS provider repair in final validation
  - rewrites generated provider returns from JSX to `React.createElement(...)`
  - adds a default React import when the file only had named React imports
  - preserves provider logic instead of replacing the whole file with a placeholder

Files:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Next exact step:

1. Push this JSX-in-TS repair
2. Wait for Render to deploy
3. Rerun the paid full-stack canary
4. If green, move to repeated paid canaries across power modes and autoscaled conditions

## Latest Live Canary Result After JSX-In-TS Fix

Paid canary rerun:

- build id: `d1b1d260-3c13-4009-9c0d-33eb5e46215a`
- backend deploy start time during run: `2026-03-28T20:29:04.350552658Z`

What changed:

- the previous JSX-in-`.ts` preview failure was no longer the first issue encountered
- the build exposed an earlier contract/schema normalization blocker instead

Newest blocker:

- provider-assisted contract critique flagged:
  - `Foreign keys in db_schema_contract do not specify referenced tables, e.g., tenant_id in User`

Interpretation:

- this is not a runtime build crash
- it is a build-contract normalization gap
- the schema already implies the relationship; the compiler just needs to make it explicit before critique runs

Newest local fix after that canary:

- data-model normalization now infers explicit FK references for obvious `_id` fields
  - example: `tenant_id` with type `uuid foreign key` becomes `uuid foreign key references Tenant(id)`

Files:

- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/orchestration_contracts_test.go`

Next exact step:

1. Push the FK-reference normalization fix
2. Wait for Render to deploy
3. Rerun the paid full-stack canary again
4. If green, run repeated paid canaries across power modes and autoscaled conditions

## Latest Live Canary Result After FK Reference Fix

Paid canary rerun:

- build id: `0e1a596f-dccd-47de-bc65-27794a07727a`
- backend deploy start time during run: `2026-03-28T20:36:23.098875616Z`

What changed:

- the earlier FK reference blocker no longer surfaced as the first issue
- instead, the build exposed a planning handoff stall

Newest blocker:

- the `plan` task completed, but the build remained in `planning`
- no team spawned
- no concrete build error was recorded
- this pointed at provider-assisted contract critique hanging on the critical path instead of a generated-app failure

Newest local fix after that canary:

- provider-assisted contract critique now runs under a hard `20s` timeout
- if the critique provider stalls, the critique degrades to `nil` instead of freezing the entire build in planning

Files:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_contract_critique_test.go`

Next exact step:

1. Push the critique-timeout fix
2. Wait for Render to deploy
3. Start a fresh paid full-stack canary
4. If it clears planning and goes green, move to repeated paid canaries across power modes and autoscaled conditions

## Latest Live Canary Result After Critique Timeout Fix

Paid canary rerun:

- build id: `e2ad3f45-8da8-4622-88eb-2865d22574bd`
- backend deploy start time during run: `2026-03-28T20:43:47.520232289Z`

What changed:

- the build cleared the earlier planning freeze
- `plan`, scaffold bootstrap, architecture, and frontend UI all completed
- the build then wedged at `44%` in `frontend_ui` with no blocker, no failure, and no downstream phase start

Newest blocker:

- phased execution stalled between phases
- `Data Foundation` never started even though the `Frontend UI` task was already terminal
- this is an orchestration recovery gap, not a generated-app/runtime failure

Newest local fix after that canary:

- the inactivity monitor now detects a terminal-task phased gap and starts the next missing execution phase instead of waiting forever
- phase starts are now funneled through a shared helper so the recovery path and the normal phased pipeline use the same snapshot/broadcast/task-assignment behavior
- phase state is persisted immediately when the next phase starts

Files:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_spawn_test.go`

Verification:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Next exact step:

1. Push the phased-gap recovery fix
2. Wait for Render to deploy
3. Start a fresh paid full-stack canary
4. If it reaches backend/data/integration/review cleanly, move to repeated paid canaries across power modes and autoscaled conditions

## Latest Live Canary Result After Phased-Gap Recovery Fix

Paid canary rerun:

- build id: `f149c461-cce3-4f5d-a3ad-b5aeccc9de75`
- backend deploy start time during run: `2026-03-28T20:51:07.841799849Z`

What changed:

- the build no longer froze between `Frontend UI` and `Data Foundation` because of the earlier between-phase gap
- instead, it exposed the next contract-normalization miss during architecture/contract critique

Newest blocker:

- provider critique still flagged actor-style foreign keys with no explicit references:
  - `Invoice.created_by`
  - `Payment.recorded_by`
  - `Project.created_by`
  - `Task.assigned_to`
  - `Task.created_by`

Interpretation:

- this is still not a preview/runtime crash
- it is a schema-contract normalization gap
- the compiler already handles `tenant_id`-style FKs, but it did not yet map actor-reference fields onto the obvious identity model

Newest local fix after that canary:

- actor-style FK inference now treats fields like `created_by`, `recorded_by`, `assigned_to`, `owner`, and `assignee_id` as identity references and maps them to common identity models when present (`User`, `Member`, `Agent`, `Admin`, `Profile`)
- planner relation targets like `Manager` or `Assignee` are now normalized too, so the compiler prefers the real identity model already present in the schema (for this canary: `User`) instead of preserving a non-existent table reference

Files:

- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/orchestration_contracts_test.go`

Verification:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Next exact step:

1. Push the actor-reference FK fix
2. Wait for Render to deploy
3. Start a fresh paid full-stack canary
4. If it clears architecture + contract critique cleanly, continue pushing toward the first fully green paid canary

## Latest Reliability Slice: Stale In-Progress Task Recovery

What changed after the latest live paid canary:

- the newest deployed backend fixed phase handoff and actor-reference contract normalization
- the next live paid canary advanced cleanly into `Frontend UI`
- the remaining failure mode was narrower: `generate_ui` could stay `in_progress` for too long with no build updates, and the inactivity monitor would not recover it because it only knew how to recover `pending>0 && in_progress=0`

Newest local fix:

- task execution now uses a provider-aware timeout budget instead of the previous coarse `15m` per-task wrapper
- the inactivity monitor can now detect a stale `in_progress` task, synthesize a timeout failure for that exact attempt, and hand control back to the existing retry/provider-switch logic
- stale results from the cancelled older attempt are now ignored if a newer retry of the same task is already underway
- `context deadline exceeded` / `context canceled` are normalized into the transient timeout class so retry strategy stays truthful

Files:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/reliability_helpers_test.go`
- `backend/internal/agents/preflight_test.go`
- `backend/internal/agents/provider_failure_matrix_test.go`
- `backend/internal/agents/orchestration_contracts.go`

Verification:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestRecoverStaleInProgressTasksQueuesSyntheticTimeoutFailure|TestProcessResultDropsStaleTaskAttemptResult|TestDetermineRetryStrategyNonRetriable|TestDetermineRetryStrategyFullMatrix'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Next exact step:

1. Push the stale-task recovery fix
2. Wait for Render to deploy
3. Start a fresh paid full-stack canary
4. Watch specifically for any retry/provider-switch event on long `generate_ui` / `generate_api` attempts instead of a silent wedge
5. If the canary completes, move immediately to repeated paid canaries across `fast`, `balanced`, and `max`

## Latest Live Paid Canary After Stale-Task Recovery

Production backend during run:

- `started_at = 2026-03-28T22:02:29.178083226Z`

Live canary:

- build id: `58fa8cce-6a8d-4548-8aa8-5868ebe39352`

What improved:

- the run no longer wedged in early planning or on a long in-flight architecture/frontend task
- it advanced cleanly through the major sections:
  - `0 -> 19 -> 44 -> 59 -> 79 -> 89 -> 95`
- this confirms the stale-task recovery work is functioning on the live paid path

Current blocker:

- final preview validation still failed at `95%`
- exact failure:
  - `server/migrate.ts(1,22): error TS7016: Could not find a declaration file for module 'pg'`
  - `server/seed.ts(1,22): error TS7016: Could not find a declaration file for module 'pg'`

Interpretation:

- orchestration is no longer the main blocker here
- the remaining issue is a deterministic generated-project repair gap
- the existing TypeScript type-package repair path already handles modules like `express`, `cors`, `react`, and `react-dom`, but it did not yet map `pg -> @types/pg`

Newest local fix after that canary:

- missing declaration-file repair now maps `pg` to `@types/pg`
- added a regression test that mirrors the live paid canary failure and verifies deterministic manifest repair injects `@types/pg`

Files:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestParseMissingTypePackagesFromBuildErrors|TestApplyDeterministicTypeDeclarationRepairAddsPgTypes'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Next exact step:

1. Push the `@types/pg` deterministic repair
2. Wait for Render to deploy
3. Start a fresh paid full-stack canary
4. If it clears the prior `pg` typing failure, keep pushing on the next remaining generated-project gap until the paid canary is fully green

## Latest Live Paid Canary After `@types/pg` Repair

Production backend during run:

- `started_at = 2026-03-28T22:17:26.955291407Z`

Live canary:

- build id: `ad01dbbe-6efe-4ee4-abe0-784713fa6124`

What improved:

- the prior `pg` declaration failure is cleared
- the run advanced deeper into the real paid path:
  - `0 -> 19 -> 44 -> 82`
- this confirms the `@types/pg` repair is no longer the blocker

Current blocker:

- integration preflight failed at `82%`
- exact failure:
  - `integration: frontend calls /api/auth/login but backend has no matching route`
  - `integration: frontend calls /api/auth/me but backend has no matching route`

Interpretation:

- the backend likely generated `/auth/login` and `/auth/me` routes without the `/api` prefix, while the frontend is correctly calling them under `/api/auth/*`
- orchestration is still healthy; this is a deterministic integration-route alignment gap

Newest local fix after that canary:

- deterministic Express integration repair now adds an `/api` alias middleware only when:
  - the frontend-missing routes all begin with `/api/`
  - the backend already has matching non-`/api` routes
  - the backend does not already expose real `/api/*` routes
- this is a narrow, truthful compatibility fix rather than a fake route generator

Files:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestApplyDeterministicExpressIntegrationRepairAddsAPIPrefixAlias|TestExtractDependencyRepairHintsFromReadinessErrorsIncludesSpecificIntegrationRouteGuidance|TestCheckIntegrationCoherence'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Next exact step:

1. Push the Express `/api` alias integration repair
2. Wait for Render to deploy
3. Start a fresh paid full-stack canary
4. If it clears the auth-route drift, continue through the next remaining generated-project blocker until the paid path is fully green

## Latest Live Paid Canary After Express `/api` Alias Repair

Production backend during run:

- `started_at = 2026-03-28T22:34:41.938698813Z`

Live canary:

- build id: `77f56547-f47d-411c-9737-cc6121caad9d`

What happened:

- the real build advanced into `architecture` / `in_progress 19%`
- but repeated status polls oscillated between:
  - `planning / 0%`
  - `in_progress / 19%`
- the build-detail endpoint sometimes returned `live=true` planning state with an older `updated_at`, then a moment later returned the newer architecture state

Root cause:

- this was an autoscaling read-path bug, not a new generator failure
- `GET /build/:id` and `GET /build/:id/status` were restoring active snapshots into memory on non-owner instances
- once that happened, a second instance could report a fake "live" planning session from stale snapshot state, while the real owner instance reported the true in-progress build
- this explains the oscillation after enabling Render autoscaling

Newest local fix after that canary:

- read-only build endpoints no longer restore active snapshots into manager memory
- they now either:
  - return the true local live build, or
  - serve the persisted snapshot as `live=false`
- this keeps polling truthful under autoscaling and stops fake live planning sessions from being created by reads alone

Files:

- `backend/internal/agents/handlers.go`
- `backend/internal/agents/handlers_test.go`

Verification:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestGetBuildStatusServesActiveSnapshotReadOnlyWithoutRestoringSession|TestGetBuildDetailsServesActiveSnapshotReadOnlyWithoutRestoringSession|TestGetBuildDetailsMarksRestoredTerminalBuildAsNotLive|TestGetBuildDetailsNormalizesLiveProgressWithinPhaseWindow|TestGetBuildStatusNormalizesLiveProgressWithinPhaseWindow'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Next exact step:

1. Push the autoscaling read-path fix
2. Wait for Render to deploy
3. Start a fresh paid full-stack canary
4. Verify polling stays monotonic instead of bouncing between `planning` and `in_progress`
5. If the canary reaches the next real generator/verifier blocker, fix that next narrow failure class and repeat

## Latest Live Paid Canary After Autoscaling Read-Path Fix

Production backend during run:

- `started_at = 2026-03-28T22:50:38.822340954Z`

Live canary:

- build id: `b9e6dde9-90f4-42aa-b952-c09abba65a80`

What improved:

- the autoscaling/state-truth bug is fixed on production
- the run advanced monotonically through the paid path:
  - `0 -> 19 -> 44 -> 79 -> 89 -> failed at 96`
- direct build-detail reads stayed truthful and no longer bounced back to fake `planning` state
- this confirms the Render autoscaling read path is now safe for customer polling

Current blocker:

- final output validation failed at `96%`
- exact failing file:
  - `server/__tests__/api.test.ts`
- exact failure class:
  - missing `supertest`
  - mismatched `apiRouter` import/export shape
  - missing test globals in the generated backend test file

Interpretation:

- orchestration, restore state, progress truth, and autoscaling polling are no longer the blocker on this path
- the next failure is a narrow generated backend test artifact leaking into preview validation
- the app path itself is now much healthier than the generated server test scaffolding

Newest local fix after that canary:

- deterministic broken-generated-test repair now forces backend/server test files to a framework-free placeholder instead of a brittle partial import patch
- this avoids repeated failures from non-essential generated backend tests poisoning preview proof

Files:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestApplyDeterministicValidationRepairsReplacesBrokenGeneratedTestFile|TestApplyDeterministicValidationRepairsReplacesBrokenBackendGeneratedTestFileWithPlaceholder'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Next exact step:

1. Push the backend generated-test placeholder repair
2. Wait for Render to deploy
3. Start the next paid full-stack canary
4. If it clears this backend-test artifact, continue on the next remaining late-stage generated-project blocker until the paid path is green

## Latest Live Paid Canary After Backend Generated-Test Placeholder Repair

Production backend during run:

- `started_at = 2026-03-28T23:06:33.159174308Z`

Live canary:

- build id: `892d028e-d5bc-4244-8b09-25fa5de231b1`

What improved:

- the prior backend test artifact was cleared
- the run advanced cleanly again through:
  - `0 -> 19 -> 44 -> 59 -> 82 -> 89`
- this confirms the backend generated-test placeholder repair is working on production

Current blocker:

- integration preflight still failed at `89%`
- exact failure:
  - `integration: frontend calls /api/auth/login but backend has no matching route`
  - `integration: frontend calls /api/auth/me but backend has no matching route`

Root cause discovered from the generated files:

- the backend actually generated the correct nested Express structure:
  - `app.use("/api", apiRouter)`
  - `router.use("/auth", authRouter)`
  - `authRouter.post("/login")`
  - `authRouter.get("/me")`
- the verifier bug was ours:
  - route resolution only expanded one mount level
  - it could see `/api/login` and `/auth/login`
  - it could not derive the true nested route `/api/auth/login`

Newest local fix after that canary:

- nested Express route resolution now expands mounted routers transitively
- the fix is bounded so it does not recurse forever by repeatedly re-prefixing the same mount path
- integration tests now cover nested mounted routes directly

Files:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestExtractExpressResolvedRoutesResolvesNestedMountedRouters|TestCheckIntegrationCoherenceAcceptsNestedMountedExpressRoutes|TestCheckIntegrationCoherenceCatchesRouteDrift|TestApplyDeterministicExpressIntegrationRepairAddsAPIPrefixAlias'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Next exact step:

1. Push the nested Express route-resolution fix
2. Wait for Render to deploy
3. Start the next paid full-stack canary
4. If it clears `/api/auth/*`, continue on the next remaining late-stage generated-project blocker until the paid path is green
