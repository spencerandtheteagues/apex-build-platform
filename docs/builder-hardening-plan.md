# Builder Hardening Plan

This document defines how to stop multi-agent builds from drifting, truncating, hallucinating ownership, or producing half-apps.

It is grounded in the current builder on `main`, especially these failure points:

- Planning output is not converted into a real structured plan. `handlePlanCompletion` only creates an empty `BuildPlan` shell and immediately moves on.
- Downstream tasks are assigned from the raw user description, not from a frozen shared artifact.
- Team coordination is a short summary of recent completed tasks, not a binding contract.
- Live intervention messages can create freeform steering notes, but they do not produce validated engine-level provider/model/config changes.
- Code generation still relies on large freeform multi-file responses, which is fragile under token limits and easy to truncate.

The TranscriptVault failure is the exact shape this design permits:

1. The system accepted a plan without freezing a real shared spec.
2. Agents generated partial backend/database artifacts without a canonical file manifest.
3. Mid-build intervention churn changed narrative direction faster than the system changed execution state.
4. A large multi-file response truncated mid-file.
5. Recovery logic appended missing content, but the orchestration still treated stale parser warnings as fatal.
6. Even the recovered artifact snapshot showed structural drift: backend-only output for a full-stack request, random folder naming, and conflicting persistence/runtime assumptions.

The fix is not "use better models." The fix is to make the builder harder to misuse.

## Design Principles

1. The system, not the model, owns architecture truth.
2. Every build must have one frozen shared spec before code generation starts.
3. Every task must have explicit file ownership and acceptance criteria.
4. Fast and balanced mode must reduce ambiguity, not just reduce token cost.
5. Large freeform multi-file output is an anti-pattern and must be phased out.
6. Verification should happen after every meaningful integration boundary, not only at the end.
7. User interventions must be translated into validated engine actions, not conversational guesses.
8. Scaffold consistency should stabilize infrastructure, while style and product behavior remain flexible.

## Root Causes In Current Code

### 1. No real plan freeze

Current code path:

- `backend/internal/agents/manager.go:2272`
- `backend/internal/agents/manager.go:2275`
- `backend/internal/agents/manager.go:2326`

The planning phase does not populate `BuildPlan.TechStack`, `Files`, `APIEndpoints`, `Components`, `DataModels`, or `Features`. It stores only `ID`, `BuildID`, and `CreatedAt`, then immediately queues implementation tasks.

Result:

- the planner is advisory only
- the architect is advisory only
- implementation agents are not bound to one canonical app definition

### 2. Every downstream agent still gets the raw prompt

Current code path:

- `backend/internal/agents/manager.go:5546`
- `backend/internal/agents/manager.go:5558`

Each task input contains only:

- `app_description`
- `agent_role`

There is no enforced:

- file ownership map
- required manifest
- acceptance checklist
- contract hash
- allowed dependency list
- explicit scope boundary

Result:

- agents can all decide they are building different versions of the app
- one agent can silently switch architecture
- package manifests and root structure are easy to conflict on

### 3. Team coordination is weak and non-binding

Current code path:

- `backend/internal/agents/manager.go:7155`
- `backend/internal/agents/manager.go:7209`

The coordination brief is just the last few task summaries. It is useful context, but it is not an executable source of truth.

Result:

- "coordinate with the team" is only a prompt suggestion
- no agent has to prove it is aligned to a common spec
- the lead does not approve a concrete task plan before work begins

### 4. Interventions are parsed as steering, not validated actions

Current code path:

- `backend/internal/agents/manager.go:5978`
- `backend/internal/agents/manager.go:6119`
- `backend/internal/agents/manager.go:6142`
- `backend/internal/agents/manager.go:6188`

The lead can respond with JSON-like steering, but the result is mostly:

- reply text
- queued revision notes
- pause/resume
- permission requests

There is no strict engine-level action model for:

- provider change
- model tier change
- scaffold change
- architecture reset
- scope reduction

Result:

- the user can hear "switching to X" even when the engine has not atomically changed the build protocol
- the lead can confidently narrate stale or wrong model facts

### 5. Multi-file freeform generation is still the default

Current code path:

- `backend/internal/agents/ai_adapter.go:129`
- `backend/internal/agents/manager.go:6881`

The builder still asks models to emit large batches of source files inside a conversational output envelope.

Result:

- truncation risk scales with ambition
- mixed prose and code become parser hazards
- one cut-off can invalidate a whole batch

### 6. Fast and balanced modes are under-scaffolded

Current code already has local strict delivery hints, but they are still prompt hints, not hard execution rules:

- `backend/internal/agents/manager.go:6854`

Result:

- weaker models get the same open-ended degree of freedom as stronger models
- the hardest part of the task remains architectural convergence rather than feature completion

## Target Architecture

The builder should move from "prompted collaboration" to "contracted execution."

### Phase 0: Requirement Extraction

Create a machine-validated `BuildIntent` from the user prompt.

Required fields:

- `app_type`
- `primary_goal`
- `must_have_acceptance_criteria`
- `should_have_acceptance_criteria`
- `non_goals`
- `tech_stack_constraints`
- `external_tool_requirements`
- `delivery_mode`

For TranscriptVault, this would have captured:

- full-stack app
- React + Tailwind + Vite frontend
- Node + Express backend
- local filesystem transcripts root
- yt-dlp subprocess
- Whisper API with whisper.cpp fallback
- folder named after the video
- supported URL sources

No code generation starts until `must_have_acceptance_criteria` is non-empty and normalized.

### Phase 1: Spec Freeze

Replace the empty-shell `BuildPlan` flow with a real frozen `BuildSpec`.

`BuildSpec` should include:

- `spec_id`
- `spec_hash`
- `build_intent`
- `scaffold_id`
- `repo_layout`
- `tech_stack`
- `runtime_ports`
- `env_schema`
- `dependency_manifest`
- `data_model`
- `api_contract`
- `ui_routes`
- `file_manifest`
- `ownership_map`
- `verification_gates`
- `acceptance_checklist`

The freeze process should be:

1. Planner outputs strict JSON only.
2. Architect refines that JSON only.
3. Lead validates consistency and emits `BuildSpec v1`.
4. Reviewer runs a spec sanity pass before any code task starts.

If parsing fails, the build does not continue. It replans. No "best effort" freeform markdown fallback.

### Phase 2: Deterministic Scaffold Selection

Fast and balanced mode should never begin from a blank page.

Add a scaffold registry with archetypes such as:

- `single-repo-react-vite-express-localfs`
- `single-repo-react-vite-express-postgres`
- `single-repo-next-go-postgres`
- `api-only-express-typescript`
- `worker-pipeline-node-localfs`

Each scaffold provides deterministic infrastructure:

- root manifests
- TypeScript config
- Vite config
- Express entrypoint
- error middleware
- health route
- env example
- README skeleton
- base folder layout
- test/lint/build scripts
- local dev proxy conventions

The scaffold should not determine the product. It should determine the boring parts.

Variation should come from:

- domain model
- route set
- UI composition
- visual design spec
- naming
- workflow behavior

That keeps apps from feeling identical while eliminating preventable infra mistakes.

### Phase 3: Ownership Handshake

Every implementation task must be generated from a `WorkOrder`, not from the raw prompt.

`WorkOrder` fields:

- `task_id`
- `spec_hash`
- `agent_role`
- `goal`
- `owned_paths`
- `allowed_new_paths`
- `must_not_touch_paths`
- `dependencies_required`
- `acceptance_checks`
- `max_files`
- `max_chars`
- `requires_lead_approval_before_write`

Before writing code, the agent must return a machine-parseable `TaskStartAck`:

- `spec_hash`
- `task_id`
- `agent_role`
- `files_to_create`
- `files_to_modify`
- `assumptions`
- `dependency_additions`
- `risk_flags`

The lead must validate:

- file ownership does not collide
- dependencies are legal
- assumptions do not violate the spec

No ACK, no execution.

### Phase 4: Completion Handshake

Every task must end with a `TaskCompletionReport`:

- `task_id`
- `spec_hash`
- `files_produced`
- `dependencies_added`
- `checks_run`
- `known_risks`
- `acceptance_checks_passed`
- `handoff_notes`

The build system should reject completion if:

- files fall outside ownership
- required files are missing
- claimed checks do not match actual verification
- spec hash is stale

### Phase 5: Per-Task Output Budgeting

Stop asking models for huge file batches.

Rules:

- `fast`: max 2 files or 6 KB of code per generation response
- `balanced`: max 4 files or 12 KB of code per response
- `max`: max 6 files or 24 KB of code per response
- any file over threshold uses chunked generation by default
- any manifest or lockfile is generated deterministically, not by freeform code output when possible

The current chunked continuation logic is useful, but it should be a fallback, not the main path.

Preferred protocol:

1. emit manifest only
2. emit one owned file batch
3. verify
4. continue

### Phase 6: Mandatory Gate Sequence

The build should use hard gates:

#### Gate 0: Intent validity

- prompt parsed into `BuildIntent`
- acceptance checklist present

#### Gate 1: Spec freeze

- `BuildSpec` validated
- ownership map conflict-free
- scaffold selected

#### Gate 2: Scaffold boot

- deterministic scaffold materialized
- `npm install` and boot/build smoke pass for scaffold alone

#### Gate 3: Phase integration

After each major phase:

- database/schema verification
- backend build verification
- frontend build verification
- API contract match verification

#### Gate 4: Acceptance audit

Reviewer validates every `must_have_acceptance_criteria`.

A build cannot complete if any `must_have` remains unproven.

### Phase 7: Failure-Class Recovery

Recovery should depend on failure class, not generic retries.

Add a classifier:

- `artifact_truncation`
- `spec_drift`
- `ownership_collision`
- `missing_required_file`
- `manifest_invalid`
- `dependency_hallucination`
- `compile_failure`
- `provider_config_error`
- `integration_contract_mismatch`

Recovery policy:

- `artifact_truncation`: split output size immediately; do not switch provider first
- `spec_drift`: stop implementation, reopen spec, reissue work orders
- `ownership_collision`: reject both writes and reassign ownership
- `missing_required_file`: issue deterministic follow-up task targeted to manifest gap
- `dependency_hallucination`: remove/replace package and rerun verification
- `compile_failure`: targeted solver fix using exact failing file set
- `provider_config_error`: provider/model fallback is allowed

The current system often treats failures too uniformly.

### Phase 8: Intervention Control Plane

User interventions should not be interpreted as prose that may or may not map to engine state.

Replace freeform intervention handling with a validated action set:

- `request_scope_change`
- `request_provider_override`
- `request_power_mode_change`
- `request_pause`
- `request_resume`
- `request_replan`
- `request_use_scaffold`
- `request_force_vertical_slice`

Rules:

1. The lead may only propose provider/model changes from a system-supplied allowlist.
2. The UI should display actual engine state, not model claims.
3. If the user asks for a provider/model that is unavailable, the system should say so deterministically.
4. Mid-build provider changes should create a new `BuildSpec version` or explicit execution note, not hidden steering text.

### Phase 9: Diversity Without Sameness

Infrastructure should be scaffolded. Product expression should not.

Split generation into two layers:

#### Stable infrastructure layer

- repo shape
- manifests
- configs
- boot scripts
- health routes
- env handling
- test harness

#### Variable experience layer

- visual design brief
- domain language
- information architecture
- workflows
- naming
- feature composition

Add a `DesignSpec` artifact:

- palette
- typography direction
- spacing density
- component personality
- motion style
- content tone

This allows consistent build success without cloned-feeling apps.

## Strict Rules By Mode

### Fast Mode

Fast mode should optimize for "runnable first" above all else.

Rules:

1. Scaffold is mandatory.
2. Single repo only.
3. One persistence stack only.
4. One vertical slice first.
5. No custom architecture unless explicitly requested by the user.
6. No more than 2 files per generation response.
7. No role may create root manifests except scaffold/bootstrap tasks.
8. Every code task must pass compile verification before the next peer task begins.
9. Reviewer checks only build blockers and acceptance blockers, not polish.
10. If the build is at risk, auto-scope down secondary features before retrying.

### Balanced Mode

Balanced mode should allow breadth, but still remain scaffold-first.

Rules:

1. Scaffold is mandatory unless planner, architect, and reviewer all approve a custom path.
2. Ownership map is mandatory.
3. Max 4 files per generation response.
4. Frontend and backend may run in parallel only after API contract freeze.
5. No root manifest edits from implementation agents without lead approval.
6. Any dependency addition must be declared in `TaskStartAck`.
7. Any file outside owned paths is rejected.
8. Testing and review must validate the acceptance checklist, not just compile success.

### Max Mode

Max mode can be more ambitious, but it must keep the same control plane.

Rules:

1. A custom architecture may replace the scaffold only after spec freeze.
2. Ownership map still applies.
3. Large files use chunk protocol by default.
4. Solver gets exact failure-class context, not just a general retry prompt.
5. Completion requires acceptance audit, compile verification, and integration verification.

## Specific Rules Every Agent Must Follow

These should become system-enforced rules, not only prompt text.

1. Do not invent architecture after spec freeze.
2. Do not create files outside your owned path set.
3. Do not edit shared root files unless explicitly assigned.
4. Do not add dependencies without declaring them in `TaskStartAck`.
5. Do not output more files than the `WorkOrder` budget allows.
6. Do not mix prose and code in one output channel.
7. Do not continue after a stale `spec_hash`.
8. Do not change stack, ports, auth strategy, persistence layer, or repo layout without opening a spec revision.
9. Do not mark a task complete without a completion report.
10. Do not claim provider/model changes that were not engine-validated.

## Existing Code To Reuse

Do not rebuild everything from zero. Promote what already exists.

### Reuse immediately

- `backend/internal/agents/types.go` already defines `BuildPlan`, `TechStack`, `Feature`, `DataModel`, `APIEndpoint`, and `PlannedFile`.
- `backend/internal/agents/autonomous/planner.go` already has a JSON-first planning flow that is stricter than the main build path.
- `backend/internal/agents/manager.go` already has phased execution and useful verification hooks.
- `backend/internal/agents/manager.go` already has chunked repair primitives that can be reused for large-file generation.

### Replace or harden

- Replace freeform planning with parsed `BuildSpec`.
- Replace summary-only team coordination with ownership-based work orders.
- Replace giant multi-file outputs with bounded batches.
- Replace intervention prose with validated engine actions.

## Implementation Roadmap

### Phase 1: Hardest immediate wins

1. Parse planner output into a real `BuildPlan` instead of an empty shell.
2. Introduce `BuildSpec` and `spec_hash`.
3. Add `ownership_map` and `WorkOrder`.
4. Enforce per-task file-count and character budgets.
5. Make fast and balanced mode scaffold-first.
6. Reject completions outside owned paths.

### Phase 2: Protocol enforcement

1. Add `TaskStartAck` validation.
2. Add `TaskCompletionReport` validation.
3. Add intervention action schema with engine validation.
4. Add failure classifier and policy-based recovery.

### Phase 3: Build quality ratchet

1. Add scaffold boot verification.
2. Add phase-level compile gates.
3. Add acceptance checklist audit.
4. Add telemetry for drift, truncation, and ownership conflicts.

### Phase 4: Diversity layer

1. Add `DesignSpec`.
2. Add scaffold families by archetype.
3. Add variation generators for naming, IA, design, and workflow structure.

## Success Metrics

Track these per power mode:

- first-pass compile success rate
- build completion rate
- acceptance checklist pass rate
- truncation incidence
- ownership conflict incidence
- provider-change incidence
- spec revision count
- solver invocation count
- build time to first runnable preview

The target is not just "fewer failures." The target is:

- fewer architectural divergences
- smaller retry surfaces
- deterministic recovery
- consistent output quality across weak and strong models

## Non-Negotiable Outcome

After this hardening, the builder should behave like this:

1. one app definition
2. one shared contract
3. one ownership map
4. one scaffold or approved custom architecture
5. bounded outputs
6. verified handoffs
7. deterministic interventions
8. deterministic recovery

That is how dumb models and smart models alike become more reliable without making every app feel the same.
