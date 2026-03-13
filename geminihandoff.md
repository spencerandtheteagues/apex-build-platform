# Gemini Handoff For Apex.Build

Read this entire file before changing anything.

This repository already contains substantial orchestration work that is correct and verified. Your job is to continue the next narrow backend slices without undoing any of it.

## 1. Non-Negotiable Rules

1. Do not rewrite the orchestration system.
2. Do not remove the phased build manager, router, context selector, error analyzer, deterministic repairs, final readiness validation, preview verification, or backend readiness verification.
3. Do not reintroduce Gemini/OpenClaw junk. Before every commit and before every push, run `git status --short`. If any of these appear, delete them and do not stage them:
   - `BOOTSTRAP.md`
   - `HEARTBEAT.md`
   - `IDENTITY.md`
   - `SOUL.md`
   - `TOOLS.md`
   - `USER.md`
   - `.openclaw/`
4. Do not enable Ollama for platform-hosted builds. Ollama remains BYOK/local-only.
5. Do not weaken verification to get green tests.
6. Do not touch unrelated frontend files unless a failing test forces it.
7. Do not remove the raw `TaskOutput.Files` compatibility path yet.
8. Do not push if any required verification command fails.
9. Prefer small helpers and compatibility seams over deep refactors.
10. If a change would require broad file replacement, stop and take the narrower patch-oriented path instead.

## 2. Current Baseline

Everything below is already implemented. Do not redo it.

### 2.1 Artifact Layer Is Live

Implemented in:
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/build_snapshot.go`
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/types.go`

Already done:
- `IntentBrief`
- `BuildContract`
- `WorkOrder`
- `PatchBundle`
- `VerificationReport`
- `PromotionDecision`
- `FailureFingerprint`
- `ProviderScorecard`
- contract compilation from build request / frozen plan
- deterministic contract verification before fan-out
- hosted-provider filtering for platform builds
- orchestration state persisted in build snapshots and exposed through APIs

### 2.2 WorkOrders Drive Task Dispatch

Implemented in:
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/planning_contracts.go`
- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/build_spec_test.go`

Already done:
- artifact-backed work orders are hydrated into assigned tasks
- prompts include contract slice, routing mode, risk, readable files, required exports, truth tags, and max context budget
- legacy `BuildWorkOrder` fallback still exists and must remain for compatibility

### 2.3 Deterministic Repairs Are Patch-First

Implemented in:
- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Already done:
- deterministic repair helpers build `PatchBundle`s first
- `applyPatchBundleToBuild()` is the manager-owned mutation seam
- snapshot-backed repair is supported
- final readiness deterministic repair stores bundles in orchestration state

### 2.4 Successful AI Code Tasks Already Emit Artifacts

Implemented in:
- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`
- `backend/internal/agents/build_spec_test.go`

Already done:
- assignment captures task-scoped patch baselines
- successful AI code-generation tasks emit first-class `PatchBundle`s
- successful AI code-generation tasks emit task-local `VerificationReport`s
- raw `TaskOutput.Files` remains in place for compatibility

### 2.5 Provider Learning Is Live

Implemented in:
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/orchestration_contracts_test.go`
- `backend/internal/agents/manager_provider_assignment_test.go`
- `backend/internal/agents/manager_readiness_test.go`
- `backend/internal/agents/provider_failure_matrix_test.go`
- `backend/internal/agents/manager.go`

Already done:
- task outcomes update live `ProviderScorecard`s
- task outcomes record `FailureFingerprint`s
- runtime provider assignment and `switch_provider` fallback read live scorecards
- retry strategy now uses recent failure history
- repeated same-provider verification failures can escalate to `switch_provider`
- repeated cross-provider failures can escalate to `spawn_solver`
- truncation-heavy failures bias toward `reduce_context`
- incident consensus prompt includes recent fingerprint summary

### 2.6 Keep These User-Control Features Stable

These are already working. Do not destabilize them while working on orchestration:
- per-agent messaging
- planner broadcast / send-to-all
- failed-build restart
- restored telemetry persistence
- restored build state recovery

Relevant files:
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/interaction.go`
- `backend/internal/agents/websocket.go`
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/services/api.ts`

### 2.7 Use The Project Gemini Agents

Project subagents now live in `.gemini/agents/`. Use them deliberately instead of relying on vague implicit routing.

For the current remaining work:
- section `3.1`:
  - primary `apex-repair-ladder-marshal`
  - support `apex-provider-economics-analyst`
  - support `apex-regression-sentinel`
- section `3.2`:
  - primary `apex-patch-promotion-foreman`
  - support `apex-surface-verification-judge`
  - support `apex-regression-sentinel`
- section `3.3`:
  - primary `apex-surface-verification-judge`
  - support `apex-repair-ladder-marshal`
  - support `apex-regression-sentinel`
- section `3.4`:
  - primary `apex-surface-verification-judge`
  - support `apex-orchestration-architect`

Rules:
1. Use one primary owner and at most two support agents.
2. Keep write scope narrow and explicit.
3. Do not send broad "fix everything" prompts to the generalist path.
4. Read `GEMINI.md` before starting.

## 3. Remaining Work Only

Do not repeat finished slices. Start here.

### 3.1 Make Repair-Work-Order Selection Use Fingerprint History

Goal:
- when the manager chooses a repair path after repeated failures, use the recorded `FailureFingerprint`s and live scorecards to choose the narrowest effective repair mode

Primary files:
- `backend/internal/agents/manager.go`
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/provider_failure_matrix_test.go`
- `backend/internal/agents/manager_readiness_test.go`

Requirements:
1. Keep `determineRetryStrategy()` unchanged as the raw classifier.
2. Keep using `determineRetryStrategyWithHistory()` as the live wrapper.
3. Extend the next-step decision after retry exhaustion so repair-work-order selection considers:
   - task shape
   - failure class recurrence
   - same-provider repeated failures
   - cross-provider repeated failures
   - prior failed `switch_provider`
   - prior failed `fix_and_retry`
   - prior successful recoveries
4. Prefer the narrowest escalation that is justified:
   - localized repair work order
   - alternate provider repair work order
   - diagnosis + repair work order
5. Do not restart broad generation unless contract corruption is clearly proven.

Strict rule:
- this is a routing/selection change, not a rewrite of the repair subsystem

### 3.2 Make Model-Driven Repair Tasks Patch-First

Goal:
- repair / solver tasks should produce and promote `PatchBundle`s as first-class outputs, not just broad replacement files

Primary files:
- `backend/internal/agents/manager.go`
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/planning_contracts.go`
- `backend/internal/agents/build_snapshot.go`

Requirements:
1. Keep `TaskOutput.Files` as compatibility.
2. For repair-oriented tasks like:
   - `solve_build_failure`
   - `fix_tests`
   - `fix_review_issues`
3. Ensure their successful outputs:
   - emit a `PatchBundle`
   - emit a task-local or surface-local `VerificationReport`
   - record the selected repair path in orchestration state
4. Reuse the existing task baseline / diff helper path instead of inventing a second patch system.
5. Do not force the proposed-edits approval system onto every repair path yet.

### 3.3 Add Repair Promotion Boundaries

Goal:
- avoid silently accepting bad repair output

Primary files:
- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Requirements:
1. A repair task should only be treated as promoted if:
   - its patch bundle exists or the fallback compatibility path is valid
   - its verification report is not failed
2. If a repair output is syntactically present but contract-invalid, do not mark it as a successful recovery.
3. Feed that outcome back into:
   - `FailureFingerprint`
   - `ProviderScorecard`
   - repair-path selection

### 3.4 Preserve Truthfulness

Goal:
- final promotion and intermediate repair success must remain truthful

Requirements:
1. Do not mark a surface `verified` if the task-local verification actually failed.
2. Do not label a repair successful just because files were produced.
3. Keep truth tags derived from real verification state.

## 4. Exact Files You Are Allowed To Touch

Primary scope:
- `backend/internal/agents/manager.go`
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/planning_contracts.go`
- `backend/internal/agents/build_snapshot.go`
- `backend/internal/agents/manager_readiness_test.go`
- `backend/internal/agents/provider_failure_matrix_test.go`
- `backend/internal/agents/orchestration_contracts_test.go`
- add one new focused backend test file if needed

Avoid unless a failing test forces it:
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/services/api.ts`
- `backend/internal/agents/interaction.go`
- `backend/internal/agents/websocket.go`
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/handlers_test.go`

## 5. Tests You Must Keep Or Extend

You must keep these green:
- `backend/internal/agents/build_spec_test.go`
- `backend/internal/agents/manager_readiness_test.go`
- `backend/internal/agents/manager_provider_assignment_test.go`
- `backend/internal/agents/orchestration_contracts_test.go`
- `backend/internal/agents/provider_failure_matrix_test.go`

Add focused tests for any new repair-path selection behavior. Do not rely on manual reasoning only.

Minimum new coverage for the next slice:
1. repeated fingerprint history changes repair-work-order selection
2. successful repair task emits `PatchBundle`
3. successful repair task emits `VerificationReport`
4. failed verification on a repair task does not count as promoted recovery
5. fallback compatibility path still works when artifact context is missing

## 6. Exact Verification Commands

Run these after your changes:

1. `cd /home/s/projects/apex-build-platform/backend && go test ./internal/agents -run 'TestDetermineRetryStrategyWithHistory_|TestStrategyToDecisionFullMatrix|TestRecordProviderTaskOutcomeUpdatesLiveScorecard|TestAssignProvidersToRolesForBuild_UsesLiveScorecards|TestGetNextFallbackProviderForTask_UsesLiveScorecards|TestProcessResultSuccessfulCodeTaskCapturesPatchBundleAndVerificationReport|TestProcessResultVerificationFailureRecordsFailureFingerprintAndScorecard' -count=1`
2. `cd /home/s/projects/apex-build-platform/backend && go test ./...`
3. `cd /home/s/projects/apex-build-platform/backend && go test -race ./internal/agents/...`
4. `cd /home/s/projects/apex-build-platform/frontend && npm run typecheck`
5. `cd /home/s/projects/apex-build-platform/frontend && npm run build`
6. `cd /home/s/projects/apex-build-platform && git diff --check`
7. `cd /home/s/projects/apex-build-platform && git status --short`

Do not skip step 7. That is how you catch reappearing Gemini junk before push.

## 7. Push Discipline

Before commit:
1. Confirm no Gemini/OpenClaw junk exists in repo status.
2. Confirm only intended files are staged.
3. Confirm all commands in Section 6 are green.

Suggested commit style:
- one commit for one backend orchestration slice
- do not bundle speculative frontend changes

Before push:
1. run `git status --short`
2. confirm there is no:
   - `BOOTSTRAP.md`
   - `HEARTBEAT.md`
   - `IDENTITY.md`
   - `SOUL.md`
   - `TOOLS.md`
   - `USER.md`
   - `.openclaw/`
3. then push

## 8. If You Get Stuck

If a proposed change would require:
- deleting the raw file-output compatibility path
- redesigning the proposed-edit / approval subsystem
- touching many frontend files
- changing provider policy
- weakening verification
- broad regeneration instead of patch-local repair

stop and do the narrower thing:

1. add the smallest helper
2. add the smallest focused test
3. preserve the current compatibility path
4. keep behavior stable and truthful

That is the correct strategy for this repository.
