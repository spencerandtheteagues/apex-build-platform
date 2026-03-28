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
