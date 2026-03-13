# Gemini Handoff For Apex.Build

Read this entire file before making any change.

This repository already contains valuable work. Do not rewrite it. Do not "simplify" it by removing the new orchestration artifacts, repair gates, or user-control features. Your job is to finish the next safe slices without regressing the current system.

## 1. Non-Negotiable Rules

1. Do not delete or replace the existing phased build manager, provider router, context selector, error analyzer, deterministic repairs, final readiness validation, or preview/backend verification.
2. Do not do a broad rewrite. Extend and refactor in place.
3. Do not reintroduce Gemini/OpenClaw junk files. Before every commit and before every push, run:
   - `git status --short`
   - if any of these appear, delete them immediately and do not stage them:
     - `BOOTSTRAP.md`
     - `HEARTBEAT.md`
     - `IDENTITY.md`
     - `SOUL.md`
     - `TOOLS.md`
     - `USER.md`
     - `.openclaw/`
4. Do not touch files outside the exact scope listed in this handoff unless a failing test forces it.
5. Do not weaken verification to get green tests.
6. Do not enable Ollama for platform-hosted builds. Ollama must remain BYOK/local-only.
7. Do not commit secrets, tokens, `.gemini` files, or home-directory junk.
8. Do not push if any required verification command fails.
9. Prefer small helpers and compatibility layers over deep refactors.
10. Preserve backward compatibility with the current raw `TaskOutput.Files` path until the new patch path is fully proven.

## 2. Current Branch State

- Branch: `main`
- Remote: `origin https://github.com/spencerandtheteagues/apex-build-platform.git`
- Existing local commits before this handoff:
  - `229e3fb` `Harden build restore and preview workflows`
  - `d895327` `Fix orchestration pipeline for consistent high-quality builds`

There are important uncommitted changes in the working tree. They are intentional and should be committed. Do not discard them.

## 3. What Is Already Implemented

These pieces are already done. Do not regress them.

### 3.1 Contract-First Artifact Layer

Implemented in:
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/build_snapshot.go`
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/types.go`

Added artifact/domain models:
- `IntentBrief`
- `BuildContract`
- `WorkOrder`
- `PatchBundle`
- `VerificationReport`
- `PromotionDecision`
- `FailureFingerprint`
- `ProviderScorecard`

Behavior already implemented:
- intent compilation from build request
- contract compilation from frozen build plan
- deterministic contract verification before agent fan-out
- hosted-provider filtering for platform builds
- scorecard-aware provider preference as a layer above current defaults
- persisted orchestration state in build snapshots
- orchestration state exposed through API responses

### 3.2 WorkOrder Artifacts Are Live In Task Dispatch

Implemented in:
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/planning_contracts.go`
- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/build_spec_test.go`

Behavior already implemented:
- compiled `WorkOrder` artifacts now include:
  - `Summary`
  - `RequiredFiles`
  - readable files
  - contract slice
  - routing mode
  - risk
  - max context budget
  - preferred provider
- `AssignTask()` hydrates tasks from artifact-backed work orders
- legacy `BuildWorkOrder` still exists as a fallback compatibility layer
- prompts now include artifact-specific context via `workOrderArtifactPromptContext()`

Do not remove the legacy fallback. Keep artifact-backed work orders authoritative, but keep the old path working until every call site is migrated.

### 3.3 Surface Verification + Truth Model

Implemented in:
- `backend/internal/agents/manager.go`
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/manager_readiness_test.go`

Behavior already implemented:
- final readiness emits surface-local `VerificationReport`s
- reports cover at least:
  - `global`
  - `deployment`
  - `frontend`
  - `backend`
  - `integration`
- truth tags are upgraded from `scaffolded`/`partially_wired` to `verified` when appropriate

### 3.4 Deterministic Repairs Now Use Real Patch Application

Implemented in:
- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Behavior already implemented:
- deterministic repair helpers now return `PatchBundle` instead of mutating first and diffing later
- `applyPatchBundleToBuild()` applies patch bundles through a single manager-owned mutation seam
- snapshot-backed builds are supported
- deterministic repair patch bundles are stored in orchestration state

Important current helpers:
- `generatedFilePatchPlan`
- `applyPatchBundleToBuild()`
- `applyDeterministicValidationRepairs()`
- deterministic repair helpers returning `(*PatchBundle, string)`

### 3.5 Builder Control / User Control Features

Implemented in these existing working-tree files:
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/handlers_test.go`
- `backend/internal/agents/interaction.go`
- `backend/internal/agents/interaction_test.go`
- `backend/internal/agents/websocket.go`
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/components/builder/AppBuilder.test.tsx`
- `frontend/src/services/api.ts`
- `frontend/src/services/api.test.ts`

Behavior already implemented:
- planner message box
- direct per-agent messaging
- broadcast to all agents
- restart failed build
- restored telemetry persistence
- build state recovery improvements

Do not destabilize this UI/backend control path while working on the orchestration backend.

## 4. The Exact Next Work To Do

Your next job is not "more architecture."

Your next job is:

1. make AI-generated code/fix tasks emit first-class `PatchBundle`s
2. make those tasks emit per-task / per-surface `VerificationReport`s
3. do it with compatibility preserved
4. do not convert the whole system to patch-only in one leap

### 4.1 Scope You Are Allowed To Touch

Primary files:
- `backend/internal/agents/manager.go`
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/planning_contracts.go`
- `backend/internal/agents/build_snapshot.go`
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/manager_readiness_test.go`
- add a new focused backend test file if needed

Files you should avoid touching unless strictly necessary:
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/services/api.ts`
- `backend/internal/agents/interaction.go`
- `backend/internal/agents/websocket.go`
- `backend/internal/agents/handlers_test.go`

## 5. Step-By-Step Implementation Plan

Follow these steps in order. Do not skip ahead.

### Step 1: Capture A Baseline For AI Task Patch Generation

Goal:
- when an AI code task starts, preserve a scoped "before" baseline so later you can generate a `PatchBundle` for that task

Do this in:
- `backend/internal/agents/manager.go`

Recommended approach:
1. Add a helper near `hydrateTaskContractInputs()` that captures a scoped baseline of generated files for the task.
2. Store that baseline in `task.Input`.
3. Do not store the whole repo indiscriminately.

Strict rules for the baseline:
- if a `WorkOrder` artifact exists:
  - baseline should prefer files in `OwnedFiles`
  - also include `RequiredFiles`
  - optionally include files in `ReadableFiles` only if they are directly emitted by the current task later
- if the task is a fix/recovery task with repair hints:
  - include only files referenced by repair hints or directly targeted by the task
- if no artifact exists:
  - fall back to current task output compatibility and do not crash

Implementation detail:
- store the baseline in `task.Input["patch_baseline_files"]`
- store it as a plain serializable slice of `GeneratedFile`

Do not:
- capture the entire codebase for every task
- include giant irrelevant context blobs

### Step 2: Add A Helper To Build AI Task Patch Bundles

Goal:
- convert AI task output into a real `PatchBundle`

Do this in:
- `backend/internal/agents/manager.go`

Add a helper with behavior like:
- inputs:
  - `build *Build`
  - `task *Task`
  - `agent *Agent`
  - `output *TaskOutput`
- output:
  - `*PatchBundle`

Rules:
1. Read `patch_baseline_files` from `task.Input`.
2. Compare baseline files to `output.Files`.
3. Use existing `buildPatchBundleFromFileDiff()` rather than inventing a second diff system.
4. Populate:
   - `BuildID`
   - `WorkOrderID` if `taskArtifactWorkOrderFromInput(task)` exists
   - `Provider` from `agent.Provider`
   - `Justification`
5. If the task output contains files but the diff helper returns `nil`, do not panic. Return `nil` cleanly.

Strict compatibility rule:
- this helper only captures a bundle
- it must not yet replace the raw `task.Output.Files` path globally

### Step 3: Append AI Task Patch Bundles On Successful Task Completion

Goal:
- persist patch bundles for AI tasks, not just deterministic repairs

Do this in:
- `backend/internal/agents/manager.go`
- inside `processResult()` success path

Precise insertion point:
- after verification passes for a successful code generation task
- before broadcasting final success / before `handleTaskCompletion()`

Rules:
1. Only attempt this for `am.isCodeGenerationTask(task.Type)`.
2. Use the new helper from Step 2.
3. If a bundle is produced:
   - append it with `appendPatchBundle(build, *bundle)`
4. Do not change current success/failure semantics in this step.
5. Do not remove `task.Output.Files`.

Important:
- this step is observability + artifact correctness first
- not a destructive behavior change

### Step 4: Emit Task-Local Verification Reports

Goal:
- produce `VerificationReport`s for individual AI tasks, not just final readiness

Do this in:
- `backend/internal/agents/manager.go`

Add a helper with behavior like:
- `emitTaskLocalVerificationReport(build, task, agent, output) *VerificationReport`

Rules:
1. Use the `WorkOrder` artifact if present.
2. Map the report surface from the work order contract slice.
3. Use cheap checks only. Reuse existing validation logic where possible.
4. Report status based on what actually happened:
   - passed
   - failed
   - blocked
5. Include:
   - `WorkOrderID`
   - `Provider`
   - `ChecksRun`
   - `Errors`
   - `TruthTags`

Cheap checks you are allowed to use:
- quick syntax checks already used by `verifyGeneratedCode()`
- required output / required export presence if easily checkable
- basic path/route presence for backend/frontend files when explicit in the work order

Do not:
- run full preview readiness here
- duplicate final readiness logic
- add expensive networked verification

### Step 5: Append Task Verification Reports In The Same Success Path

Goal:
- each successful code task leaves behind a task-level verification artifact

Do this in:
- `backend/internal/agents/manager.go`
- same general success path as Step 3

Rules:
1. Build the report after the task output is verified.
2. Append it with `appendVerificationReport(build, report)`.
3. Keep final readiness reports untouched.

### Step 6: Optional Second Slice Only If Everything Above Is Green

Only do this if Steps 1-5 are fully green.

Goal:
- convert `TaskFix` recovery tasks to use patch-bundle promotion more explicitly

Allowed target actions:
- `solve_build_failure`
- `fix_tests`
- `fix_review_issues`

Safe approach:
1. Keep `task.Output.Files` as compatibility.
2. If the task is a fix task and a patch bundle exists:
   - ensure it is appended to orchestration state
   - emit a verification report for the affected surface
3. Stop there unless tests prove a stricter promotion path is safe.

Do not in this slice:
- rip out `applyApprovedEdits()`
- redesign proposed edit storage
- force user approval flows onto every patch

That larger promotion refactor can be a later step.

## 6. Tests You Must Add

Minimum required new tests:

1. A test that an AI code-generation task produces a `PatchBundle`
   - best location: new backend test or `manager_readiness_test.go`
   - assert:
     - bundle appended
     - `WorkOrderID` populated when artifact exists
     - `Provider` populated

2. A test that a successful AI task emits a task-level `VerificationReport`
   - assert surface matches work order contract slice
   - assert status is `passed` for a valid output

3. A test that absence of `work_order_artifact` does not crash patch/report emission
   - fallback behavior must remain safe

4. Keep all existing tests green:
   - deterministic repair tests
   - artifact/work-order tests
   - frontend builder tests

## 7. Exact Verification Commands

Run these after your changes:

1. `cd /home/s/projects/apex-build-platform/backend && go test ./internal/agents -run 'TestAssignPhaseAgentsUsesFrozenWorkOrder|TestAssignTaskHydratesArtifactWorkOrderForAdHocTask|TestTaskWorkOrderFromInputUsesArtifactFallback|TestWorkOrderArtifactPromptContextIncludesContractSlice|TestApplyDeterministicValidationRepairsCapturesPatchBundle|TestApplyPatchBundleToBuildUsesSnapshotFileFallback|TestApplyDeterministicValidationRepairsAppliesBundleToSnapshotFiles'`
2. `cd /home/s/projects/apex-build-platform/backend && go test ./...`
3. `cd /home/s/projects/apex-build-platform/backend && go test -race ./internal/agents/...`
4. `cd /home/s/projects/apex-build-platform/frontend && npm run typecheck`
5. `cd /home/s/projects/apex-build-platform/frontend && npm run build`
6. `cd /home/s/projects/apex-build-platform && git diff --check`
7. `cd /home/s/projects/apex-build-platform && git status --short`

Do not skip step 7. It is how you catch reappearing junk before push.

## 8. Push Discipline

Before commit:
1. Confirm no Gemini/OpenClaw junk exists in repo status.
2. Confirm only intended files are staged.
3. Confirm all commands in Section 7 are green.

Suggested commit style:
- one commit for the backend patch/report slice
- do not bundle unrelated speculative refactors

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

## 9. Files That Already Contain Important Current Work

Read these before editing:
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/planning_contracts.go`
- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`
- `docs/contract-first-orchestration-plan.md`

Also do not regress these existing user-control changes:
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/interaction.go`
- `backend/internal/agents/websocket.go`
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/services/api.ts`

## 10. If You Get Stuck

If a proposed change would require:
- deleting the legacy raw file-output path
- redesigning the entire review/proposed-edit subsystem
- touching a large number of frontend files
- changing provider policy
- changing final readiness semantics

stop and do not improvise.

Instead:
1. keep the current compatibility path
2. add the narrow helper
3. add the narrow test
4. keep the behavior stable

That is the correct strategy for this repository.
