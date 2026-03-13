# Contract-First Build Orchestration Plan

## Purpose

This document defines the migration of APEX.BUILD from broad generation with late validation into a contract-first, patch-oriented, gated build system that preserves the existing phased orchestration backbone and deterministic verification assets.

The plan is evolutionary:

- preserve the current manager/orchestrator, router, context selector, error analyzer, deterministic repairs, readiness validation, and preview/backend verification
- introduce new artifact types as compatibility layers first
- move failure discovery earlier
- reduce broad regeneration in favor of narrow repair work orders
- keep hosted orchestration on Claude, GPT, Gemini, and Grok only
- keep Ollama on the BYOK/local path only

## Current Codebase Anchors

### Existing systems to preserve

- `backend/internal/agents/manager.go`
  - primary phased build lifecycle
  - lead agent spawn, plan freeze, task queueing, readiness finalization, recovery
- `backend/internal/agents/build_spec.go`
  - deterministic scaffold selection
  - frozen build spec generation
  - ownership and work-order synthesis
- `backend/internal/agents/error_analyzer.go`
  - failure classification and repair hints
- `backend/internal/agents/context_selector.go`
  - narrow context slicing for local repairs and focused prompts
- `backend/internal/agents/manager.go`
  - deterministic validation repairs
  - preview/build verification
  - integration coherence checks
- `backend/internal/ai/router.go`
  - provider routing, health checks, fallback, rate limiting
- `backend/internal/ai/enhanced_router.go`
  - existing strategy scaffolding for more task-aware routing

### New compatibility layer added in this migration slice

- `backend/internal/agents/orchestration_contracts.go`
  - new artifact/domain models
  - compatibility compilers from request and plan state
  - baseline provider scorecards
  - hosted-provider policy filter
- `backend/internal/agents/build_snapshot.go`
  - orchestration artifacts now survive snapshot persistence/restore through `state_json`
- `backend/internal/agents/manager.go`
  - intent brief creation at build creation
  - build contract compilation and deterministic contract verification at plan freeze
  - promotion decision + failure fingerprinting at terminal promotion
- `backend/internal/agents/handlers.go`
  - orchestration artifacts exposed via build status/detail/history APIs

## Canonical Artifact Model

### 1. IntentBrief

Purpose:
- normalize the raw request once
- compress downstream prompt payloads
- capture required capabilities, risks, deployment intent, and acceptance seeds

Fields:
- normalized request
- app type
- required features
- non-goals
- complexity class
- required capabilities
- deployment target
- risk flags
- cost sensitivity
- acceptance summary seed

Current implementation:
- compiled from `BuildRequest` in `CreateBuild`
- persisted in orchestration snapshot state

### 2. BuildContract

Purpose:
- canonical machine-checkable truth before generation
- explicit surface ownership and acceptance targets

Fields:
- route/page map
- backend resource map
- API contract
- DB schema contract
- auth contract
- env var contract
- dependency skeleton
- file ownership plan
- runtime/build command contract
- acceptance by surface
- verification gates by surface
- truth tags by surface

Current implementation:
- compiled from the frozen `BuildPlan`
- verified before downstream agent spawning

### 3. WorkOrder

Purpose:
- narrow the task boundary
- limit context
- prevent cross-file drift

Fields:
- category
- task shape
- owned/readable/forbidden files
- contract slice
- required outputs / exports
- surface-local checks
- max context budget
- risk level
- routing mode
- preferred provider

Current implementation:
- compatibility layer compiled from existing `BuildWorkOrder`
- provider preference seeded from scorecards instead of only static role defaults

### 4. PatchBundle

Purpose:
- patch-first mutation format for future narrow repair and apply flows

Operations:
- `create_file`
- `replace_symbol`
- `replace_function`
- `insert_after_symbol`
- `patch_json_key`
- `patch_env_var`
- `patch_route_registration`
- `patch_dependency`
- `patch_schema_entity`
- `delete_block`
- `rename_symbol`

Current implementation:
- domain model only in this migration slice
- intended next step is to route deterministic and model repairs through `PatchBundle` apply semantics

### 5. VerificationReport

Purpose:
- unify surface-local and global verification outputs
- give promotion logic structured evidence

Fields:
- phase
- surface
- status
- deterministic/provider source
- checks run
- warnings/errors/blockers
- truth tags
- confidence

Current implementation:
- contract verification report
- final readiness/promotion report

### 6. PromotionDecision

Purpose:
- truthful readiness summary instead of implicit “build completed”

States:
- `prototype_ready`
- `preview_ready`
- `integration_ready`
- `test_ready`
- `production_candidate`
- `blocked`

Fields:
- readiness state
- unresolved blockers
- confidence score
- truth by surface
- full-build results
- preview readiness
- integration readiness
- production-candidate flag

Current implementation:
- compiled at terminal finalization
- persisted and exposed to API clients

### 7. FailureFingerprint

Purpose:
- track repeated failure shapes and recovery cost

Fields:
- stack combination
- task shape
- provider/model
- failure class
- files involved
- repair path chosen
- repair success
- token cost to recovery

Current implementation:
- recorded for contract-verification blocks
- recorded for terminal promotion failures

### 8. ProviderScorecard

Purpose:
- task-shape-aware provider priors
- bridge from static role mapping to measurable routing

Metrics:
- compile-pass rate
- first-pass verification-pass rate
- repair success rate
- truncation rate
- average accepted tokens per success
- average cost per success
- latency
- failure-class recurrence
- promotion rate

Current implementation:
- hosted priors in code
- used to seed preferred provider selection for work orders and role assignment

## Phase Mapping to Current Apex Pipeline

### Phase 0: Request Intake / Intent Normalization

Current anchor:
- `AgentManager.CreateBuild`

Target behavior:
- compile `IntentBrief`
- store a compact request artifact immediately
- derive risk/cost/capability shape before model calls

Status:
- implemented in initial compatibility form

### Phase 1: Contract Compilation

Current anchors:
- `build_spec.go`
- `handlePlanCompletion`

Target behavior:
- compile `BuildContract` from frozen `BuildPlan`
- retain existing plan generation and deterministic scaffold logic

Status:
- implemented in compatibility form

### Phase 2: Contract Verification

Current anchors:
- `handlePlanCompletion`
- existing API contract and acceptance primitives in `build_spec.go`

Target behavior:
- block impossible builds before agent fan-out
- auto-correct obvious gaps such as missing runtime defaults
- fail early on missing auth/schema/billing/runtime coverage

Status:
- deterministic verification implemented
- provider-assisted contract critique remains a follow-up phase

### Phase 3: Deterministic Scaffold Synthesis

Current anchors:
- `build_spec.go`
- `bootstrapBuildScaffold`

Target behavior:
- keep current scaffold logic
- extend it into the default source of manifests, env templates, route shells, entrypoints, schema shells

Migration direction:
- preserve current scaffold selection
- attach scaffold coverage metrics to verification reports and promotion decisions

### Phase 4: WorkOrder Slicing

Current anchors:
- `BuildWorkOrder`
- `buildWorkOrders`
- task queueing in `manager.go`

Target behavior:
- compile strict `WorkOrder` artifacts with explicit owned/readable/forbidden files and routing mode

Status:
- compatibility work orders implemented
- next step is to feed them directly into task dispatch and prompt construction

### Phase 5: Patch Generation

Current anchors:
- `chunked_edit.go`
- existing file generation paths
- deterministic repair helpers in `manager.go`

Target behavior:
- patch-first generation and repair
- whole-file rewrite only as explicit fallback

Migration direction:
- route deterministic repairs first through `PatchBundle`
- later adapt model repair prompts to emit structured patch operations

### Phase 6: Surface-Local Verification

Current anchors:
- preview/frontend/backend readiness verification in `manager.go`
- integration coherence validation
- deterministic manifest/tooling checks

Target behavior:
- run cheap local checks per work order before global readiness
- emit `VerificationReport` per surface

Status:
- final global report implemented
- surface-local report emission is the next incremental step

### Phase 7: Selective Multi-Provider Escalation

Current anchors:
- `assignProvidersToRoles`
- `selectLeadProvider`
- `router.go`
- `enhanced_router.go`

Target behavior:
- mode A: single provider + deterministic gate
- mode B: single provider + verifier
- mode C: dual candidate + judge
- mode D: diagnosis + repair + verifier + optional critique

Status:
- scorecard and task-shape scaffolding implemented
- full escalation-mode routing is still to be integrated into dispatch

### Phase 8: Repair Ladder

Current anchors:
- deterministic validation repairs
- solver recovery
- error analyzer

Target behavior:
- deterministic repair first
- localized provider repair next
- no broad regeneration unless contract corruption is proven

Status:
- existing repair ladder primitives remain the live path
- next step is to express each repair as a repair `WorkOrder` + `PatchBundle`

### Phase 9: Final Readiness Promotion

Current anchors:
- `runBuildFinalization`
- final readiness validation

Target behavior:
- emit `PromotionDecision`
- aggregate truth tags + verification reports
- distinguish prototype/preview/integration/test/prod-candidate honestly

Status:
- implemented in compatibility form

## Hosted Provider Policy

### Hosted path

Allowed:
- Claude
- GPT
- Gemini
- Grok

Disallowed:
- Ollama

Current implementation:
- platform build provider discovery now filters to hosted providers only
- role/provider assignment is scorecard-informed on top of current fallback logic

### BYOK / local path

Allowed:
- existing provider set, including Ollama

Rationale:
- local/BYOK is a separate orchestration lane with different cost and latency assumptions

## Why Preserved Legacy Subsystems Stay

### AI router

Keep:
- provider availability
- fallback
- health
- rate limiting

Adaptation:
- scorecards and task shape become a policy layer above the router, not a replacement for it

### Context selector

Keep:
- file selection and narrow context windows

Adaptation:
- use it as the context budget enforcer for `WorkOrder` execution and repair work orders

### Error analyzer

Keep:
- failure classification and repair hints

Adaptation:
- make it the diagnosis stage in the repair ladder and failure fingerprint recorder

### Deterministic repairs

Keep:
- manifest repair
- syntax repair
- type-package repair
- mixed-code cleanup
- missing-manifest / deliverable repair

Adaptation:
- move them earlier into surface-local verification
- emit reports and patch artifacts instead of opaque retries

### Final readiness validation

Keep:
- final truth gate

Adaptation:
- make it promotion aggregation, not the first time basic structural defects are discovered

### Existing manager/orchestrator backbone

Keep:
- phased build execution
- task queueing
- spawn lifecycle
- checkpointing
- resume/restore

Adaptation:
- drive phases with the new artifacts rather than loose prompt state

## Migration Flags

Implemented flags:
- `APEX_ENABLE_INTENT_BRIEF`
- `APEX_ENABLE_BUILD_CONTRACT`
- `APEX_ENABLE_CONTRACT_VERIFICATION`
- `APEX_ENABLE_PATCH_BUNDLES`
- `APEX_ENABLE_SURFACE_LOCAL_VERIFICATION`
- `APEX_ENABLE_SELECTIVE_ESCALATION`
- `APEX_ENABLE_REPAIR_LADDER`
- `APEX_ENABLE_PROMOTION_DECISION`
- `APEX_ENABLE_FAILURE_FINGERPRINTING`
- `APEX_ENABLE_PROVIDER_SCORECARDS`
- `APEX_HOSTED_PROVIDERS_ONLY`

Default posture:
- enabled for intent, contract, verification, promotion, scorecards, fingerprinting
- patch bundle capture is enabled for deterministic repairs; patch-bundle-driven apply semantics are still the next step

## Metrics and Instrumentation Plan

### Quality

- first-pass runnable rate
- first-pass verification-pass rate
- integration pass rate
- preview-ready rate
- successful repair rate
- blocker recurrence rate

### Cost

- tokens per successful build
- cost per successful build
- tokens per promoted patch
- cost per recovered failure
- retries per successful build

### Reliability

- patch promotion rate
- contract violation rate
- late-stage failure rate
- average repair depth
- provider-specific failure recurrence

### Business

- runnable output per dollar

### Where to instrument next

- emit verification report counters from `manager.go`
- attach provider/model/cost to work-order completion
- persist token-to-recovery on failure fingerprints
- aggregate provider scorecards from real build outcomes instead of priors

## Backward Compatibility Strategy

- keep `BuildPlan` as the compatibility input to `BuildContract`
- keep existing `BuildWorkOrder` while new `WorkOrder` artifacts are compiled beside it
- keep final readiness validator and current deterministic repairs
- keep current WebSocket/status flows; add orchestration artifacts to snapshot state and API responses
- keep rollback path by gating new behavior with env flags

## Next Implementation Steps

1. feed compiled `WorkOrder` artifacts directly into task prompt context and dispatch
2. emit `VerificationReport` per surface-local verification stage, not just contract/promotion
3. convert deterministic repair helpers to produce `PatchBundle` operations
4. add contract-aware repair work orders using the error analyzer
5. replace static provider role bias with scorecard-weighted task-shape routing in dispatch
6. persist real provider scorecard outcomes and recovery costs
7. add explicit truth-tag updates during scaffold, patch promotion, and verifier passes
