# Canary Reliability Handoff

This file is the exact handoff for continuing APEX Build canary reliability work from Claude Code or any other coding session if this chat hits limits.

## Goal

The current reliability target is:

- Free users always get a prompt-matching frontend UI that works in the preview pane.
- Paid users always get a truthful, working full-stack app in the preview pane.
- Build history, build status, and completed-build details must agree on the terminal result.
- Free preview-only builds must not leak paid/full-stack blockers, approvals, or work orders after the plan has been downgraded to `frontend_preview_only`.

## Current Bug Cluster

The live free canary exposed four concrete issues:

1. The production smoke script was stale for the real auth flow.
2. Free preview-only builds could still inherit paid/full-stack approvals and capability flags after planning completed.
3. The phrase `clean file structure` could falsely trigger file-upload/storage paid gating because capability detection matched the word `file`.
4. A build could finish successfully in live status polling but still persist or present a stale failed snapshot through `/api/v1/builds/:buildId`.

## Live Confirmation Before This Patch Set Is Deployed

The currently deployed production backend still reproduces the old capability-detection bug.

Confirmed on `2026-03-27` with a disposable canary account:

- build id: `10677c84-71c5-4b79-9705-4930bc21f40a`
- prompt: frontend-only `PulseBoard` canary
- actual bad live behavior:
  - `required_capabilities` included `file_upload` and `storage`
  - approvals still included `full_stack_upgrade` and `file_storage`
  - blocker `plan-upgrade-required` still appeared
  - build classification was `upgrade_required`
- why this is wrong:
  - the prompt only asked for a frontend-only preview
  - the phrase `clean file structure` was enough to trigger the stale `file` keyword heuristic in production

This means:

- the local capability-detection and preview-only cleanup changes are real fixes for a currently live production issue
- you must deploy this patch set before trusting any new free-tier canary result

## Exact Next Step After Deploy

Immediately after deploying this patch set:

1. Re-run the free canary.
2. Confirm the build is no longer classified as `upgrade_required`.
3. Confirm `required_capabilities` no longer include `file_upload` or `storage` for the free frontend prompt.
4. Confirm approvals no longer show `full_stack_upgrade`, `file_storage`, or `plan_upgrade_acknowledgement` as active blockers on the downgraded preview path.
5. Only after that should you judge the next failure class.

## Files Already Being Changed

- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/orchestration_semantics.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/entitlements_test.go`
- `backend/internal/agents/orchestration_semantics_test.go`
- `backend/internal/agents/iteration_test.go`
- `backend/internal/agents/handlers_test.go`
- `scripts/run_platform_build_smoke.sh`

## What Each Change Is Supposed To Do

### 1. Smoke script

File:
- `scripts/run_platform_build_smoke.sh`

Required behavior:
- register with `accept_legal_terms:true`
- support cookie-session auth instead of assuming bearer-only auth
- fail fast on registration/login errors
- after a `completed` live status, also fetch `/api/v1/builds/:buildId`
- fail if the completed-build endpoint does not also report `status=completed`

Expected signal:
- The canary should now catch status/detail inconsistency instead of silently passing.

### 2. Capability detection

File:
- `backend/internal/agents/orchestration_contracts.go`

Required behavior:
- `file` by itself must no longer imply upload/storage capability
- phrases like `clean file structure` or `clear folder structure` must stay free/frontend-eligible
- actual upload/storage phrases like `file upload`, `uploads`, `attachment`, `file storage`, `s3`, `blob`, `bucket` should still trigger the right capability

### 3. Preview-only policy cleanup

File:
- `backend/internal/agents/orchestration_semantics.go`

Required behavior:
- once the build delivery mode is `frontend_preview_only`, derived capability state should stop claiming:
  - auth
  - database
  - storage
  - jobs
  - billing
  - realtime
  - backend runtime
  - publish
- pending paid approvals such as `plan_upgrade_acknowledgement` and `full_stack_upgrade` should stop appearing once the actual delivery target is the free preview fallback
- the `plan-upgrade-required` blocker should not remain on a build that is already truthfully proceeding in preview-only mode

Important nuance:
- It is acceptable for the original request to have been full-stack.
- It is not acceptable for the user-facing live build state to keep acting like the build is blocked after the system has already committed to the preview-only fallback.

### 4. Snapshot persistence and presentation

Files:
- `backend/internal/agents/manager.go`
- `backend/internal/agents/handlers.go`

Required behavior:
- after plan completion, derived snapshot state must be refreshed immediately so persisted state reflects the frozen plan and contract
- snapshot persistence must not let an older write overwrite a newer terminal snapshot
- completed history endpoints must present a terminal snapshot with:
  - `status=completed`
  - `progress=100`
  when `completed_at` is set and `error` is empty
- `/api/v1/build/:id/status`
- `/api/v1/build/:id`
- `/api/v1/builds`
- `/api/v1/builds/:buildId`
- `/api/v1/builds/:buildId/download`
  should all agree on presented terminal state

## Required Tests

Run these first from `backend/`:

```bash
TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp \
go test ./internal/agents -run 'TestBuildSubscriptionRequirement|TestRefreshDerivedSnapshotStateLockedUpgradeRequiredBuildIncludesPlanAcknowledgement|TestRefreshDerivedSnapshotStateLockedFrontendPreviewOnlyClearsPaidRuntimeApprovals|TestNormalizeRestoredBuildStatusTreatsLegacyBuildingAsResumable|TestPersistBuildSnapshotDoesNotOverwriteNewerTerminalSnapshot|TestCompletedBuildEndpointsPresentCompletedTerminalSnapshot'
```

Then run these adjacent regression checks:

```bash
TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp \
go test ./internal/agents -run 'TestGetBuildDetailsIncludesSnapshotState|TestSnapshotReadEndpointsFallbackToPersistedState|TestApplyBuildAssurancePolicyToPlanDowngradesFreeFullStackToFrontendPreview'
```

Then run the full backend agent package:

```bash
TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp \
go test ./internal/agents
```

Then run the full backend suite:

```bash
TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp \
go build ./...

TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp \
go test ./... -timeout=120s
```

Also validate the smoke script syntax:

```bash
bash -n scripts/run_platform_build_smoke.sh
```

## Live Free Canary

Run this against production:

```bash
BASE_URL=https://api.apex-build.dev/api/v1 \
MODE=fast \
POWER_MODE=fast \
SMOKE_PROFILE=free_frontend \
EXPECT_STATUS=completed \
POLL_SECONDS=10 \
MAX_POLLS=120 \
./scripts/run_platform_build_smoke.sh
```

Expected result:
- the script exits `0`
- it prints `FINAL_DETAIL_SUMMARY`
- it prints `COMPLETED_BUILD_SUMMARY`
- both live status and completed-build detail agree on `status=completed`

If the script exits with `COMPLETED_BUILD_STATUS_MISMATCH`, the status/detail split is still broken.

If the script exits with `BUILD_TERMINATED_WITH_UNEXPECTED_STATUS=failed`, immediately fetch the terminal build detail and look at:

- `.error`
- `.files`
- `.build_contract.delivery_mode`
- `.capability_state`
- `.approvals`
- `.blockers`

## Live Full-Stack Canary

This requires a paid canary account.

Environment variables needed:
- `LOGIN_EMAIL`
- `LOGIN_PASSWORD`

Run:

```bash
BASE_URL=https://api.apex-build.dev/api/v1 \
MODE=fast \
POWER_MODE=balanced \
SMOKE_PROFILE=paid_fullstack \
EXPECT_STATUS=completed \
POLL_SECONDS=10 \
MAX_POLLS=180 \
LOGIN_EMAIL='YOUR_PAID_CANARY_EMAIL' \
LOGIN_PASSWORD='YOUR_PAID_CANARY_PASSWORD' \
./scripts/run_platform_build_smoke.sh
```

Expected result:
- the build starts without power-mode upgrade refusal
- it reaches `completed`
- the final detail and completed-build endpoints agree

## If The Free Canary Still Fails

### Case A: build start fails with `POWER_MODE_UPGRADE_REQUIRED`

Fix:
- use `POWER_MODE=fast` for free canaries
- do not treat this as a platform bug

### Case B: build starts but free preview still shows paid blockers/approvals

Inspect:
- `backend/internal/agents/orchestration_semantics.go`
- `backend/internal/agents/manager.go`

Likely cause:
- `build.Plan.DeliveryMode` or `BuildContract.DeliveryMode` is not being consulted when derived state is recomputed

### Case C: live polling says `completed` but `/builds/:buildId` says `failed`

Inspect:
- `backend/internal/agents/manager.go`
- `backend/internal/agents/handlers.go`

Likely causes:
- stale later snapshot write overwrote the completed snapshot
- completed-build presentation trusts raw stored status instead of terminal truth

### Case D: free preview still spawns backend/database work orders

Inspect:
- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/orchestration_contracts.go`

What must be true:
- `applyBuildAssurancePolicyToPlan` must reduce the plan to frontend-only
- the persisted plan and persisted orchestration work orders must reflect that reduced plan
- the user-facing snapshot fields must show the reduced plan, not the original intent

### Case E: free preview fails during final verification with `preview_build_failed`

Example symptom:

- `Final output validation failed after repeated recovery (preview_build_failed): Preview verification build failed: ... Unterminated string literal`

What to inspect first:

- `backend/internal/agents/manager.go`

Exact functions to inspect:

- `parsePreviewSyntaxErrorTargetFiles`
- `repairDoubleSingleQuoteCorruption`
- `applyDeterministicQuoteSyntaxRepair`
- `applyDeterministicValidationRepairs`
- `validateFinalBuildReadiness`

Why this matters:

- the current deterministic syntax repair only catches TypeScript compiler errors like `TS1002` and `TS1005`
- Vite/esbuild failures such as `Transform failed ... ERROR: Unterminated string literal` can bypass that repair path entirely

Required fix direction:

1. Extend `parsePreviewSyntaxErrorTargetFiles` so it can extract target file paths from esbuild/Vite error lines, not only `TS1002/TS1005` compiler output.
2. Extend deterministic syntax repair so it can run for:
   - `Unterminated string literal`
   - `Transform failed`
   - equivalent esbuild syntax failures on `.ts`, `.tsx`, `.js`, `.jsx`
3. Add regression tests in `backend/internal/agents/manager_readiness_test.go` that use the actual esbuild/Vite-style error format.
4. Re-run:
   - focused syntax-repair tests
   - `go test ./internal/agents`
   - the free canary against production after deploy

Minimum regression tests to add if this case appears:

```bash
cd backend
TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp \
go test ./internal/agents -run 'TestParsePreviewSyntaxErrorTargetFiles|TestRepairDoubleSingleQuoteCorruption'
```

Then re-run the full agent package:

```bash
cd backend
TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp \
go test ./internal/agents
```

## If You Need To Push From Claude Code

1. Run the tests above.
2. Run the free canary.
3. If the canary still fails, classify the failure into Case A-E above before changing code.
4. Implement the narrowest deterministic fix that addresses the actual failure class.
5. Re-run the focused tests plus `go test ./internal/agents`.
6. Re-run the free canary.
7. If green, commit only the tracked reliability files.
8. Push to `main`.
9. Re-run the free canary after deploy.

Do not touch unrelated untracked files in the repo root or `investor-demo/`.

## What To Say If Asked For Truth

These statements are accurate:

- A reliable free frontend canary proves the core preview path is sound.
- It does not, by itself, prove all higher power modes or full-stack builds are reliable.
- Higher power modes and paid full-stack flows need their own canaries.

These statements are not accurate:

- `all builds can no longer fail for any reason`
- `if the free canary is green then all paid full-stack builds are definitely better`
