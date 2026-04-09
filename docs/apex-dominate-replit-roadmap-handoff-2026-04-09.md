# Apex Reliability and Product-Dominance Roadmap

Date: 2026-04-09

This document captures the current implementation plan and the slices already landed in the local workspace so work can resume cleanly if the session is interrupted.

## Core Goal

Make Apex win on:

- first-pass frontend success
- truthful preview quality
- visually polished default output
- faster, better-coordinated builds
- stronger iteration and deployment paths over time

Non-negotiables:

- Frontend-first is mandatory.
- Free users always get a truthful, previewable frontend.
- Full-stack continuation should happen only after frontend approval.
- Major orchestration changes must be feature-flagged.
- Do not touch `handlers.go`, `handlers_test.go`, `frontend/e2e/`, or `tests/e2e/`.

## Best-Order Execution Plan

### Phase 0: Reliability Control Plane

- Failure taxonomy across planning, generation, compile, preview boot, visual, interaction, contract, runtime, deployment.
- Canary corpus for `free-fast`, `paid-balanced`, and `paid-max`.
- Per-build quality summaries and metrics.
- Promotion canaries must reject builds that technically complete but remain `degraded` or `blocked` in orchestration-derived reliability state.
- Rollback flags for every major feature.

### Phase 1: Guaranteed Frontend-First Preview Path

- Deterministic frontend shell repair.
- Compile-loop stabilization and normalization.
- Truthful frontend preview even when backend/runtime work is incomplete.

### Phase 2: Forced Design System Baseline

- Deterministic shadcn/ui scaffold.
- Tailwind token baseline and reusable primitives.
- Better frontend prompt guidance to avoid generic UIs.

### Phase 3: Interaction-Truthful Preview Validation

- Post-load canary interaction checks.
- Repair hints for “loaded but unusable” previews.

### Phase 4: Vision-Loop Validation

- Screenshot capture during browser verification.
- Optional vision analysis for visual breakage hints.
- Non-blocking, advisory-only repair guidance.

### Phase 5: Adaptive Routing and QA Inputs

- Sample-aware scorecard routing.
- Role-aware capability mapping.
- Better test-generation contracts and testing-agent inputs.

### Phase 6: Parallel Orchestration

- Architecture phase first.
- Parallel core phase for frontend + database + backend behind a feature flag.
- Integration and review phases remain serialized.

### Later Phases

- Test generation as first-class delivery output.
- Multi-modal input from screenshots/wireframes.
- Semantic diff-based iteration.
- One-click deployment orchestration.

## Slices Already Implemented In Local Workspace

### 1. Preview truthfulness and visual/advisory validation

Implemented:

- screenshot capture in browser verification
- advisory canary interaction tester
- optional Claude-vision screenshot reviewer via `APEX_CLAUDE_VISION_KEY`
- propagation of screenshot/canary/repair-hint metadata through preview verification
- preview gate preservation of advisory warnings on passing previews
- normalized preview `failure_class:*` tagging

Key files:

- `backend/internal/preview/browser_verifier.go`
- `backend/internal/preview/canary_tester.go`
- `backend/internal/preview/vision_verifier.go`
- `backend/internal/preview/runtime_verifier.go`
- `backend/internal/preview/verifier.go`
- `backend/internal/agents/preview_gate.go`
- `backend/internal/ai/claude.go`
- `backend/cmd/main.go`

### 2. Design-system floor via deterministic shadcn scaffold

Implemented:

- deterministic creation of:
  - `components.json`
  - `src/lib/utils.ts`
  - `src/components/ui/button.tsx`
  - `src/components/ui/card.tsx`
  - `src/components/ui/input.tsx`
  - `src/components/ui/badge.tsx`
  - `src/components/ui/dialog.tsx`
- synthetic frontend shell upgraded to use the scaffolded primitives
- Tailwind config upgraded to semantic tokens + `tailwindcss-animate`
- package/dependency generation updated for shadcn-compatible baseline
- frontend prompts updated to prefer scaffolded UI primitives

Key files:

- `backend/internal/agents/shadcn_scaffold.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/autonomous/executor.go`

### 3. Scorecard routing and role-aware capability mapping

Implemented:

- sample-aware live scorecard detection
- cost-sensitive scorecard routing by power mode
- baseline policy preserved when scorecards only contain priors or too few live samples
- reliability-aware provider fallback when live scorecards are weak:
  - recurring/current `visual_layout` and `interaction_canary` bias frontend/reviewer/testing/solver roles toward stronger UI-verification providers
  - recurring/current `compile_failure` biases frontend/backend/database/solver roles toward stronger compile-repair providers
  - recurring/current `contract_violation` and `coordination_violation` bias planner/architect/reviewer roles toward stronger contract/planning providers
- reliability bias now survives the later static role-policy fallback in provider assignment instead of being overwritten by default role preferences
- explicit `RoleHint` added to `GenerateOptions`
- role-aware capability mapping for planner/architect/reviewer/testing/solver/code roles
- fallback prompt heuristics kept in place

Key files:

- `backend/internal/agents/scorecard_router.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/ai_adapter.go`
- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/compile_validator.go`
- `backend/internal/agents/manager_task_routing.go`
- `backend/internal/agents/manager_optimized.go`
- `backend/internal/agents/error_analyzer.go`

### 4. QA/test-generation input contract improvements

Implemented:

- `QAWorkOrder` contract for testing tasks
- derived testing frameworks and owned paths from the build plan
- injection of `test_frameworks`, `owned_test_paths`, and `test_contract` into testing task input
- testing-agent prompt updated to prefer Vitest, Testing Library, and Playwright smoke coverage
- kept tests as delivery artifacts rather than a reason to block the frontend-first preview path

Key files:

- `backend/internal/agents/qa_agent.go`
- `backend/internal/agents/manager.go`

### 5. Parallel core orchestration behind a feature flag

Implemented:

- `buildExecutionPhasesParallel(...)`
- feature flag: `APEX_PARALLEL_MID_PHASE`
- when enabled:
  - `architecture`
  - `parallel_core`
  - `integration`
  - `review`
- integration preflight recovery now also runs after `parallel_core`
- progress-window support for `parallel_core`

Key files:

- `backend/internal/agents/parallel_phase.go`
- `backend/internal/agents/manager.go`

Current default:

- `APEX_PARALLEL_MID_PHASE` defaults to `false` for safety.

### 6. Failure taxonomy in persisted snapshot state

Implemented:

- `failure_taxonomy` persisted on `BuildSnapshotState`
- categories tracked across:
  - planning
  - generation
  - compile
  - preview boot
  - visual
  - interaction
  - contract
  - runtime
  - deployment
  - verification
- taxonomy is fed from:
  - failed verification reports
  - build error / fatal activity messages
- duplicate suppression prevents preview failures from being double-counted when both report and build-error paths fire
- successful verification/completion clears the current failure while preserving historical counts and the last failure

Key files:

- `backend/internal/agents/failure_taxonomy.go`
- `backend/internal/agents/types.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/orchestration_contracts.go`

### 7. Preview verification excludes generated test artifacts

Implemented:

- preview/runtime proof now skips:
  - `src/__tests__/...`
  - `tests/...`
  - `e2e/...`
  - `*.test.*`
  - `*.spec.*`
- this keeps generated tests in delivery/history while preventing them from polluting preview boot verification

Key files:

- `backend/internal/agents/preview_gate.go`

### 8. Derived reliability summary and canary enforcement

Implemented:

- `BuildReliabilitySummary` derived from:
  - current failure taxonomy
  - latest verification reports
  - validated build spec surfaces/user flows
  - historical failure fingerprints
- per-build summary now classifies results as:
  - `clean`
  - `advisory`
  - `degraded`
  - `blocked`
- summary carries:
  - current failure category/class
  - advisory classes
  - recurring failure classes
  - top issues
  - recommended focus
  - acceptance surfaces
  - primary user flows
- passing preview advisories are preserved in history without reactivating current failure state
- platform smoke runner now rejects “completed” builds whose orchestration summary is still `degraded` or `blocked`
- smoke runner also requires the summary to include acceptance surfaces and primary user flows so promotion checks verify preserved build intent, not just terminal status

Key files:

- `backend/internal/agents/reliability_summary.go`
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/orchestration_semantics.go`
- `scripts/run_platform_build_smoke.sh`

### 9. Stronger precompute security/performance advisories

Implemented:

- precomputed validated specs now merge intent-brief capabilities instead of relying only on raw prompt re-detection
- security advisories expanded for:
  - role boundary enforcement
  - billing webhook verification
  - tenant isolation
  - AI prompt/data boundary hardening
- performance advisories expanded for:
  - progressive dashboard loading
  - feed windowing/bounded rendering
  - upstream latency budgets for AI/external-provider features
- this makes the “war room” spec materially more useful before planning and codegen begin

Key files:

- `backend/internal/agents/validated_build_spec.go`
- `backend/internal/agents/validated_build_spec_test.go`

### 10. Reliability summary now steers prompts and repairs

Implemented:

- task prompts now include the derived `reliability_summary` context when present
- compile-repair prompts now include the same summary before error windows/context-diet source excerpts
- recurring failure classes, recommended focus, acceptance surfaces, and primary user flows now actively shape solver behavior
- the summary is also copied into per-task input for downstream tooling/debugging

Key files:

- `backend/internal/agents/reliability_summary.go`
- `backend/internal/agents/reliability_summary_test.go`
- `backend/internal/agents/compile_validator.go`
- `backend/internal/agents/compile_validator_test.go`
- `backend/internal/agents/manager.go`

### 11. Reliability summary now biases retry strategy selection

Implemented:

- `determineRetryStrategyWithHistory(...)` now consults orchestration reliability state before falling back to pure provider-history heuristics
- recurring/current `visual_layout` or `interaction_canary` signals now escalate repeated verification-style failures to solver recovery instead of wasting provider switches
- recurring/current `compile_failure` now upgrades generic retries into `fix_and_retry`, and escalates to solver sooner once fix-path history shows the problem is repeating
- this keeps the retry path aligned with the known build degradation class instead of reacting only to the latest error string

Key files:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/provider_failure_matrix_test.go`

### 12. Plan work orders now absorb reliability and frontend-first intent

Implemented:

- plan-level work orders are now enriched before task assignment with:
  - frontend-first preview acceptance checks
  - preserved primary user-flow checks
  - compile/visual/interaction recurring-risk checks
  - reviewer/testing reliability focus items
- the enrichment runs both:
  - after plan completion
  - after frontend approval when full-stack continuation rebuilds work orders
- this pushes reliability guidance upstream into frontend/testing/reviewer/backend tasks instead of waiting for solver recovery

Key files:

- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/build_spec_test.go`
- `backend/internal/agents/manager.go`

### 13. Autonomous package manifests are now more runnable by default

Implemented:

- React/Tailwind manifests now include:
  - `tsx`
  - `@testing-library/react`
  - `@testing-library/user-event`
  - `@testing-library/jest-dom`
  - `@playwright/test`
  - `@vitest/coverage-v8`
  - `jsdom`
- generated scripts now include:
  - `server` via `tsx server/index.ts`
  - `test`
  - `test:coverage`
  - `test:e2e` for React builds
- fixed the previous mismatch where `server` used `ts-node` without adding `ts-node`

Key files:

- `backend/internal/agents/autonomous/executor.go`

### 9. Advisory preview signals are now typed as visual vs interaction

Implemented:

- screenshot-review repair hints now emit `visual:...`
- canary errors and repair hints now emit `interaction:...`
- preview gate remains backward-compatible with older `vision:`-prefixed hints when deciding whether screenshot context should be forwarded to repair tasks

Key files:

- `backend/internal/preview/runtime_verifier.go`
- `backend/internal/agents/preview_gate.go`

### 10. Multi-modal wireframe/screenshot intake at build creation

Implemented:

- `BuildRequest` now accepts:
  - `wireframe_image`
  - `wireframe_description`
- optional `VisionIntakeProcessor` added to the agent manager
- when `APEX_CLAUDE_VISION_KEY` is present and a build request includes `wireframe_image`:
  - the image is decoded
  - a structured `ComponentSpec` is extracted
  - the spec is merged into the effective build prompt before planning
- `CreateBuild(...)` now uses the richer `prompt` when present instead of silently anchoring on `description` alone

Key files:

- `backend/internal/agents/vision_intake.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/types.go`

### 11. Validated build spec locked before generation

Implemented:

- new persisted `validated_build_spec` on orchestration state
- request-time precompute spec with:
  - normalized request
  - app type / delivery mode
  - primary user flows
  - state domains
  - security advisories
  - performance advisories
- planning-time finalization that locks:
  - route plan
  - API paths
  - acceptance surfaces
- every task now receives `validated_build_spec`
- every agent prompt now includes a `<validated_build_spec>` block so generation stays anchored to the frozen product contract

Key files:

- `backend/internal/agents/validated_build_spec.go`
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/manager.go`

### 12. Context-diet repair prompts + Hydra compile repair

Implemented:

- new context-diet helper that replaces raw full-file dumps with:
  - imports
  - public signatures
  - focused source windows around the failing lines
- compile repair prompt now uses that pruned context instead of large whole-file payloads
- feature-flagged Hydra compile repair:
  - `APEX_COMPILE_HYDRA_REPAIR`
  - enabled for `balanced` and `max`
  - skipped for `fast`
  - runs three speculative repair strategies in parallel
  - validates each candidate in an isolated workspace before applying the winning patch to the live build
- existing deterministic repair ladder still runs first; Hydra is an acceleration layer, not a replacement

Key files:

- `backend/internal/agents/context_diet.go`
- `backend/internal/agents/compile_validator.go`

## Focused Verification Already Run

These slices were verified locally with focused build/test runs during implementation:

- `go build ./cmd/main.go`
- targeted `./internal/preview` tests for screenshot/vision/canary behavior
- targeted `./internal/agents` tests for:
  - preview gate warning preservation and screenshot propagation
  - shadcn scaffold repair
  - executor package/tailwind generation
  - scorecard routing and low-sample fallback
  - role-aware capability mapping
  - QA contract injection
  - parallel-phase flag behavior and progress windows
  - failure taxonomy capture and dedupe behavior
  - preview verification filtering of generated test files
  - autonomous manifest test/coverage/e2e scripts
  - multi-modal wireframe-image intake during `CreateBuild(...)`
  - validated build spec precompute/finalization/prompt injection
  - context-diet compile repair prompts
  - Hydra strategy gating by power mode
  - reliability summary derivation and prompt injection
  - retry-path bias from recurring reliability classes
  - reliability-aware work-order enrichment for frontend-first acceptance surfaces
  - reliability-aware provider fallback when scorecards are insufficient

Broad verification checkpoint reached:

- `cd backend && GOCACHE=/tmp/apex-go-cache go test ./internal/agents -count=1 -parallel=4 -timeout=900s`

## Recommended Next Slices

In order:

1. Tie advisory `visual:` / `interaction:` preview findings and the new `validated_build_spec` into the failure-fingerprint pipeline so recurring UI-quality and contract-drift issues become searchable, not just visible in warnings.
2. Expand the canary/reliability control plane using the new `failure_taxonomy` and `validated_build_spec` instead of inventing a second failure model.
3. Promote `APEX_PARALLEL_MID_PHASE` and `APEX_COMPILE_HYDRA_REPAIR` through broader production/staging canary runs before enabling either behavior unconditionally.
4. Extend tester output beyond prompt inputs:
   - generate stronger default test file shapes
   - verify generated package scripts on more stacks
   - keep tests out of preview-proof gating.
5. Run an exact-branch-tip canary verification pass and only then promote the verified SHA to `main`.
6. Decide whether to expose `wireframe_image` in the frontend request path now that backend support exists.

## Important Repository Context

- There are unrelated local modifications in the repo that should not be reverted or staged automatically.
- User explicitly requested that these surfaces remain untouched:
  - `handlers.go`
  - `handlers_test.go`
  - `frontend/e2e/`
  - `tests/e2e/`

## Suggested Commit Grouping

When ready to commit, keep the work scoped:

1. preview truthfulness + vision/canary
2. shadcn deterministic scaffold
3. scorecard routing + role hints
4. QA/test contract
5. parallel-core flag
6. failure taxonomy + preview-test filtering
7. autonomous manifest runnable test scripts
8. multi-modal wireframe intake
9. validated build spec + context-diet repair prompts + Hydra compile repair
10. reliability summary + retry/work-order/provider bias
