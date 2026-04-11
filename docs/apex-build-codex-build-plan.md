# Apex Build — Codex Build Plan for the 95-Point Path

## Purpose

This plan tells Codex exactly which subsystems are:

- partially built and should be finished in place
- new and should be added as new subsystems
- intentionally deferred until later

This is optimized for the current Apex Build architecture, not for a greenfield rewrite.

## Core rule

Codex must not treat Apex Build as unfinished from scratch.

Several high-value pieces already exist and should be extended rather than replaced:

- heuristic context slicing
- context-diet prompt packing
- Hydra-style parallel compile repair
- contract-first orchestration artifacts
- provider verification and judging scaffolding
- Git UI primitives
- collaboration and event infrastructure
- preview verification and repair loops

The goal is to finish the right partial systems, not to destroy them and start over.

## Delivery order

1. Finish the Context Diet with real AST pruning
2. Build the compute waterfall and triage router
3. Strengthen deterministic verification before AI critique
4. Make Hydra repairs patch-first and merge-safe
5. Turn contract-first scaffolding into a real precompute War Room
6. Refactor LivePreview and move toward event-driven preview state
7. Expose Glass Box execution UX
8. Add repair memory, scorecard persistence, and semantic repair caching
9. Only then evaluate hybrid WebContainer preview
10. Only later add prompt-evolution and self-improving prompt proposals

---

# Phase 1 — Finish the Context Diet with AST-aware pruning

**Status:** Partial  
**Priority:** P0  
**Expected impact:** Very high  
**Type:** Finish existing work in place

## Existing anchors

### Already in repo
- `backend/internal/agents/context_selector.go`
  - file relevance scoring
  - error-path heuristic
  - recency heuristic
  - simple dependency and import heuristic
  - token budget and truncation behavior

- `backend/internal/agents/context_diet.go`
  - import extraction
  - signature extraction
  - focused source windows

- `backend/internal/agents/compile_validator.go`
  - already uses `buildContextDietSection(...)` in the compile-repair prompt path

## What is missing
The current system is still mostly:
- regex-based
- text-structure based
- not symbol-accurate
- not AST-aware

It does not yet provide:
- exact function boundaries
- exact class boundaries
- exact interface and type boundaries
- exact symbol ownership context
- proper target function full body plus everything else collapsed to signatures

## What to add

### New files to add
- `backend/internal/agents/ast_context.go`
- `backend/internal/agents/ast_typescript.go`
- `backend/internal/agents/ast_go.go` (optional later)
- `backend/internal/agents/symbol_index.go`

### What those files should do
1. Parse TS, TSX, JS, and JSX into symbol-aware structures.
2. Extract imports, exported symbols, declarations, and line spans.
3. Build a context packer that can:
   - include full bodies for target symbols
   - collapse non-target symbols to signatures
   - keep imports
   - include compact neighboring references
4. Expose a single API such as `BuildPrunedSymbolContext(path, content, targetSymbols, focusLines, options)`.

## How to integrate
- Keep `context_selector.go` as the file-level chooser.
- Upgrade `context_diet.go` or replace its internals so it can use AST-aware extraction.
- Update `compile_validator.go` so repair prompts use AST-pruned symbol context when parsing succeeds and the current regex/signature fallback when parsing fails.

## Do not do
- Do not replace the whole context selector.
- Do not attempt full repository graph-RAG here.
- Do not make AST parsing required for all languages on day one.
- Do not block if parsing fails. Always fall back gracefully.

## Done when
- compile repair prompts no longer ship full files unnecessarily
- target symbol bodies are exact
- non-target file context is collapsed to imports, signatures, and focused windows
- token usage for compile repair drops materially without hurting pass rate

---

# Phase 2 — Build the compute waterfall and triage router

**Status:** Partial  
**Priority:** P0  
**Expected impact:** Very high  
**Type:** Finish and restructure existing routing work

## Existing anchors

### Already in repo
- `backend/internal/agents/ai_adapter.go`
  - static `modelsByPowerMode`
  - static `selectModelForPowerMode(...)`

- `backend/internal/ai/router.go`
  - capability and provider routing
  - health checks
  - fallbacks
  - rate limiting

- `backend/internal/ai/enhanced_router.go`
  - conceptual scaffolding for advanced routing
  - much of it is still stub-level and should not be treated as production-complete

- `backend/internal/agents/manager_task_routing.go`
  - provider-assisted verification
  - judging between candidates
  - routing-mode logic
  - deterministic scoring hooks

- `backend/internal/agents/orchestration_contracts.go`
  - `ProviderScorecard`
  - `WorkOrder`
  - routing modes
  - task shapes
  - provider preference helpers

## What is missing
Apex still routes too much through:
- static model selection by power mode
- coarse provider choice by capability
- incomplete scorecard-driven escalation

It does not yet have a real compute waterfall:
- cheap model frames the task
- deterministic layer compiles a work order if possible
- medium-cost model handles bounded planning or verification
- expensive model only wakes up for hard code generation or high-risk cross-surface edits

## What to add

### New files to add
- `backend/internal/agents/task_triage.go`
- `backend/internal/agents/workorder_compiler.go`
- `backend/internal/agents/routing_waterfall.go`

### What those files should do
1. Task triage:
   - classify task shape
   - classify risk level
   - estimate scope
   - detect local repair, local refactor, cross-surface feature, verification, or architecture planning
2. Work-order compilation:
   - deterministic where possible
   - otherwise use the cheapest viable model to derive owned files, readable files, forbidden files, acceptance checks, and max context budget
3. Provider and model escalation:
   - cheap model first for intent parsing, triage, work-order shaping, critique, or judging where appropriate
   - expensive models only for high-risk architecture, cross-surface generation, hard rewrites, or repeated repair failure

## Integration changes
- Keep `router.go` as the low-level provider, health, and fallback layer.
- Do not build on `enhanced_router.go` blindly. Either replace its stub internals with real code or leave it experimental and build the actual routing waterfall in `agents/`.
- Update `ai_adapter.go` so static power-mode mapping becomes a default fallback, not the main policy.

## Explicit Codex instruction
Treat `enhanced_router.go` as partially conceptual. Do not assume it is the right production base. Reuse only what is real and verified.

## Done when
- small local fixes stop waking up expensive models by default
- work-order shaping is cheap and bounded
- provider and model escalation is tied to risk and scope
- routing choices are explainable and reproducible

---

# Phase 3 — Strengthen deterministic verification before AI critique

**Status:** Partial  
**Priority:** P0  
**Expected impact:** High  
**Type:** Extend existing validation pipeline

## Existing anchors

### Already in repo
- `backend/internal/agents/compile_validator.go`
  - `npm install`
  - `tsc --noEmit`
  - `npm run build`
  - parsed errors
  - deterministic validation before repair
  - Hydra candidate revalidation

- `frontend/package.json`
  - scripts for `build`, `typecheck`, `lint`, and tests

- `backend/internal/agents/manager_task_routing.go`
  - provider-assisted task verification already receives deterministic verification status and errors

## What is missing
There is not yet a stronger deterministic-first policy such as:
- run lint, typecheck, and build first on relevant surfaces
- only invoke provider-assisted critique if deterministic checks pass or if critique is truly needed
- skip critique when deterministic truth already says the candidate is broken

There is also no strong surface-local verifier pipeline yet for:
- frontend patch quality
- integration invariants
- route, schema, and auth consistency checks before model critique

## What to add

### New files to add
- `backend/internal/agents/surface_verifier.go`
- `backend/internal/agents/deterministic_checks.go`

### Concrete work
1. Add explicit deterministic gates by task shape:
   - frontend patch: lint, typecheck, targeted build
   - backend patch: compile and testable command where applicable
   - config and deploy patch: config validation and manifest sanity
2. Update `providerAssistedTaskVerification(...)` call sites so the verifier is:
   - skipped when deterministic checks clearly fail
   - used only as a second opinion when deterministic truth passes or is inconclusive
3. Emit structured verification reasons:
   - `deterministic_failed`
   - `deterministic_passed`
   - `provider_critique_needed`
   - `provider_critique_skipped`

## Do not do
- Do not add TypeDoc or documentation gates now.
- Do not run huge full-suite validation for tiny local edits.
- Do not make all critique provider-based.

## Done when
- provider-assisted critique becomes narrower and cheaper
- deterministic gates catch obvious failures earlier
- token burn from models critiquing clearly broken model output drops significantly

---

# Phase 4 — Make Hydra repairs patch-first and merge-safe

**Status:** Partial  
**Priority:** P0  
**Expected impact:** Very high  
**Type:** Finish the existing repair system instead of replacing it

## Existing anchors

### Already in repo
- `backend/internal/agents/compile_validator.go`
  - Hydra race exists
  - strategy diversity exists
  - local sibling workspaces exist
  - validation of candidates exists

- `backend/internal/agents/orchestration_contracts.go`
  - `PatchBundle`
  - patch operation types
  - patch bundle append logic
  - truth updates for patch bundles

- `frontend/src/hooks/useGitIntegration.ts`
  - commit, push, pull, branch switching, branch creation

- `frontend/src/components/ide/GitPanel.tsx`
  - Git UI already exists

## What is missing
Hydra winners are not yet treated as first-class:
- reviewable atomic patch artifacts
- risk-classified merge actions
- proposal branches for risky changes
- accept, reject, and revert UX

The current Hydra pipeline is still more repair-and-apply than repair, prove, diff, classify, and merge safely.

## What to add

### New files to add
- `backend/internal/agents/repair_patch_classifier.go`
- `backend/internal/agents/repair_commit_flow.go`
- `frontend/src/components/ide/AIRepairReviewPanel.tsx`

### Concrete work
1. Convert Hydra winner application into:
   - explicit patch artifact
   - patch risk classification
2. Risk classifier must tag repairs as:
   - `auto_merge_safe`
   - `review_required`
3. Safe auto-merge conditions:
   - tiny patch
   - no control-flow rewrite
   - no schema, auth, billing, or deploy changes
   - localized compile fix
4. Review-required conditions:
   - large patch
   - multi-file rewrite
   - control-flow changes
   - data, auth, billing, or config changes
5. Create review flow:
   - branch for risky AI repair
   - side-by-side diff
   - Accept & Merge
   - Reject
   - Revert later

## Explicit Codex instruction
Do not replace Hydra. Hydra already exists and is valuable. Finish it by making the winning output a safe product-grade patch workflow.

## Done when
- every accepted Hydra repair is attributable
- risky repairs are reviewable before merge
- safe repairs feel instant
- users can revert AI fixes without fear

---

# Phase 5 — Turn contract-first scaffolding into a real War Room

**Status:** Partial  
**Priority:** P1  
**Expected impact:** Very high  
**Type:** Finish an architectural direction already underway

## Existing anchors

### Already in repo
- `docs/contract-first-orchestration-plan.md`
  - clear contract-first direction
  - intent brief, build contract, work order, patch bundle, verification report, promotion decision

- `backend/internal/agents/orchestration_contracts.go`
  - `IntentBrief`
  - `BuildContract`
  - `WorkOrder`
  - `PatchBundle`
  - `VerificationReport`
  - `PromotionDecision`
  - `ProviderScorecard`
  - orchestration flags including `EnableValidatedBuildSpec`

- `backend/internal/agents/manager_task_routing.go`
  - verifier and judge concepts
  - routing modes
  - deterministic candidate scoring

## What is missing
The real War Room leap is not implemented yet.

Missing pieces:
- `DraftBuildSpec`
- structured critique passes before code generation
- blackboard mutation loop
- immutable `ValidatedBuildSpec`
- coding agents constrained to implementation rather than ad hoc architecture invention

## What to add

### New files to add
- `backend/internal/agents/validated_build_spec.go`
- `backend/internal/agents/war_room.go`
- `backend/internal/agents/war_room_critique.go`

### Concrete work
1. Compile a `DraftBuildSpec` from:
   - intent brief
   - build contract
   - frozen plan
2. Run cheap critique passes before generation:
   - architect critique
   - security critique
   - performance critique
3. Use structured critique, not freeform chat:
   - JSON patch or structured issue objects
4. Resolve critiques into a `ValidatedBuildSpec`.
5. Pass the `ValidatedBuildSpec` into coding agents as the execution contract.
6. Mark deviations as failures or verifier warnings.

## Important rule
First implementation should be narrow and structured.
Do not build theatrical multi-agent chatter first.
Do not build the fancy UI before the structured backend exists.

## Done when
- architecture flaws are caught before heavy code generation
- coding agents are constrained by validated spec
- first-shot quality improves
- fix-and-break loops caused by bad early architecture choices drop

---

# Phase 6 — Refactor LivePreview and move preview state toward event-driven control

**Status:** Partial  
**Priority:** P1  
**Expected impact:** High  
**Type:** Finish and split an overloaded existing component

## Existing anchors

### Already in repo
- `frontend/src/components/preview/LivePreview.tsx`
  - huge monolithic component
  - many state hooks
  - multiple polling loops
  - iframe preview
  - preview, server, and log polling

- `frontend/src/services/collaboration.ts`
  - real-time collaboration and event infrastructure
  - CRDT-inspired service model
  - websocket-based update patterns

## What is missing
The preview system still has:
- too much state in one component
- multiple polling-based loops
- too much coupling between toolbar controls, runtime status, backend server state, logs, iframe state, and devtools panes

It does not yet have a clean event-driven preview model.

## What to add

### New files to add
- `frontend/src/hooks/usePreviewRuntime.ts`
- `frontend/src/hooks/usePreviewServer.ts`
- `frontend/src/hooks/usePreviewDevtools.ts`
- `frontend/src/components/preview/PreviewToolbar.tsx`
- `frontend/src/components/preview/PreviewRuntimePane.tsx`
- `frontend/src/components/preview/PreviewStatusCards.tsx`

### Concrete work
1. Split `LivePreview.tsx` into:
   - runtime state hook
   - backend server hook
   - logs and devtools hook
   - pure UI subcomponents
2. Replace polling where possible with:
   - websocket and event-driven updates
   - pushed status from backend
3. Keep polling only as fallback safety, not as the primary sync path.
4. Standardize state transitions:
   - starting
   - running
   - degraded
   - backend_down
   - failed
   - stopped

## Explicit Codex instruction
Do not jump to WebContainers yet.
Do not rewrite collaboration into a full CRDT workspace yet.
First make the current preview architecture legible and event-driven.

## Done when
- `LivePreview.tsx` is no longer a giant state blob
- preview feels more responsive
- backend, runtime, and devtools state is easier to reason about
- race-condition risk is reduced

---

# Phase 7 — Expose Glass Box execution UX

**Status:** Mostly new, with partial telemetry foundations  
**Priority:** P1  
**Expected impact:** High on trust and product feel  
**Type:** New product layer on top of real backend events

## Existing anchors

### Already in repo
- build progress events already exist
- compile validation broadcasts already exist
- provider verification and judging concepts exist
- Git and diff foundations exist

## What is missing
The user still cannot clearly see:
- which repair strategy ran
- what the AI is currently doing
- what files it is reading
- why a model and provider was chosen
- what patch won and why

## What to add

### New files to add
- `frontend/src/components/ide/AITelemetryOverlay.tsx`
- `frontend/src/components/ide/HydraRacePanel.tsx`
- `frontend/src/components/ide/BuildActivityTimeline.tsx`

### Backend work
Add websocket events for:
- Hydra candidate started
- Hydra candidate passed or failed
- Hydra winner selected
- work-order compiled
- war room critique started or resolved
- patch review required
- deterministic gate passed or failed

## Important rule
Only visualize real execution artifacts.
Do not invent fake AI-thinking theater.

## Done when
- users can see the repair race
- users understand what the system is doing
- build flow feels disciplined rather than magical

---

# Phase 8 — Add repair memory, real scorecard persistence, and semantic repair caching

**Status:** Partial  
**Priority:** P2  
**Expected impact:** High over time  
**Type:** Finish existing telemetry and memory concepts, then add semantic caching

## Existing anchors

### Already in repo
- `backend/internal/agents/orchestration_contracts.go`
  - `FailureFingerprint`
  - `ProviderScorecard`
  - `recordProviderTaskOutcome(...)`
  - provider ranking helpers

- `backend/internal/agents/manager_task_routing.go`
  - provider judging and verification logic

### Important current caveat
The repository direction clearly expects adaptive scorecard behavior, but scorecard learning should be treated as unfinished until runtime call sites are verified and connected end to end.

## What is missing
- persistent scorecard learning from real outcomes
- repair strategy win-rate learning
- repair fingerprint reuse
- semantic repair cache

There is also no real vector or embedding-based cache yet.

## What to add

### New files to add
- `backend/internal/agents/repair_memory.go`
- `backend/internal/agents/repair_fingerprint_cache.go`
- `backend/internal/agents/semantic_repair_cache.go`

### Concrete work
1. Finish real outcome wiring:
   - compile pass and fail
   - first-pass verification
   - repair success
   - promotion success
   - latency, cost, and tokens
2. Store repair fingerprints:
   - failure class
   - files involved
   - strategy winner
   - patch class
3. Add semantic repair cache for narrow repair classes first:
   - compile errors
   - import and export mismatches
   - prop and type mismatch classes
4. Only after that consider broader user-intent semantic caching.

## Important rule
Do not start with a huge general vector cache.
Start with repair memory for repeated technical failure classes.

## Done when
- provider and strategy choice improves with real outcomes
- common repair classes reuse good prior work
- repeated token burn on identical bug classes drops

---

# Phase 9 — Evaluate hybrid frontend preview via WebContainers

**Status:** New  
**Priority:** P3  
**Expected impact:** Potentially high, but risky  
**Type:** New subsystem, explicitly deferred

## Existing anchors
- none in production form
- no real WebContainer path currently exists

## Why this is deferred
This is a big architectural move.
It should only happen after:
- context discipline is stronger
- patch safety exists
- LivePreview has been refactored
- preview state semantics are clearer

## What to add later
- browser-local frontend execution path
- keep cloud, E2B, or backend runtime for APIs, databases, and heavy execution
- route pure frontend preview tasks to browser-local runtime

## Explicit Codex instruction
Do not start this phase until Phases 1 through 8 are in strong shape.

---

# Phase 10 — Prompt evolution and self-improving prompt proposals

**Status:** New  
**Priority:** P3  
**Expected impact:** Long-term  
**Type:** New subsystem, explicitly deferred

## Existing anchors
- failure fingerprints
- scorecards
- critique and report structures

## What is missing
There is no disciplined self-improvement loop yet.

## What to add later
- failure cluster analysis
- prompt-improvement proposal generator
- human admin approval workflow
- benchmark-gated prompt upgrades

## Important rule
Do not let prompts mutate themselves automatically early.
Generate proposals, not autonomous rewrites.

## Done when
- prompt changes are evidence-backed
- regressions are benchmarked
- self-improvement is controlled, not chaotic

---

# Cross-phase implementation rules for Codex

## Rule 1
Prefer finishing partial systems over greenfield replacement.

## Rule 2
Do not rip out:
- Hydra compile repair
- context selector
- orchestration contracts
- Git integration
- collaboration infrastructure

Upgrade them.

## Rule 3
Treat these files as the main already-built anchors:
- `backend/internal/agents/context_selector.go`
- `backend/internal/agents/context_diet.go`
- `backend/internal/agents/compile_validator.go`
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/manager_task_routing.go`
- `backend/internal/agents/ai_adapter.go`
- `backend/internal/ai/router.go`
- `frontend/src/components/preview/LivePreview.tsx`
- `frontend/src/hooks/useGitIntegration.ts`
- `frontend/src/components/ide/GitPanel.tsx`
- `frontend/src/services/collaboration.ts`

## Rule 4
Treat these as not production-complete yet:
- `backend/internal/ai/enhanced_router.go`
- any future-direction docs that are not fully wired
- any adaptive scorecard logic not proven in runtime hot paths

## Rule 5
Do not build WebContainers, full CRDT runtime synchronization, or prompt self-evolution before the earlier phases are complete.

---

# Recommended milestone grouping

## Milestone 1
- Phase 1
- Phase 2
- Phase 3

## Milestone 2
- Phase 4
- Phase 5

## Milestone 3
- Phase 6
- Phase 7

## Milestone 4
- Phase 8

## Milestone 5
- Phase 9
- Phase 10

---

# Final guidance to Codex

Apex Build already contains the beginnings of a serious software-factory control plane.

The correct strategy is:
1. make context smaller and smarter
2. make routing cheaper and more disciplined
3. make deterministic truth stronger
4. make repairs patch-safe and user-trustworthy
5. move architecture verification earlier
6. then improve preview UX and latency
7. only later pursue deeper runtime and platform rewrites
