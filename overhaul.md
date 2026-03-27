# Build Workflow Overhaul

Last updated: 2026-03-26
Owner: Codex + Spencer
Status: In progress

## Objective

Make the APEX build experience reliable, staged, compact, and easy to understand.

The current workflow exposes too much internal orchestration detail, does not communicate the build path clearly, and does not recover failed builds reliably enough. The target state is a build system that:

- builds in clear sections from `0%` to `100%`
- leads with scaffold and frontend/UI first for full-stack builds
- fills backend, data, and integration work in after the UI shell exists
- narrates progress in plain English at each stage
- keeps the default build screen compact
- moves deep diagnostics into dedicated views instead of one long scroll
- preserves all current detail, but no longer forces it into the main viewport

## Product Direction

The desired UX is closer to the best parts of Replit/Bolt/Lovable, but tighter:

- A compact default build cockpit
- A visible staged workflow
- Fast user comprehension of `what is happening now`
- User-facing updates at each section boundary
- Heavy telemetry available on demand, not always expanded

## Success Criteria

The overhaul is successful when all of the following are true:

- A full-stack build visibly progresses through discrete sections:
  1. `Scaffold`
  2. `Frontend UI`
  3. `Backend/Data`
  4. `Integration`
  5. `Verification`
  6. `Preview/Ship`
- Full-stack builds create the frontend/UI shell before backend completion work.
- The default build screen fits the most important status into one compact view without excessive scrolling.
- Failed and terminal builds do not appear falsely live.
- Restart and recovery flows reliably resume the build and reconnect the live session.
- The user can still access all orchestration details, but through focused views such as `Overview`, `Activity`, `Issues`, `Files`, `Timeline`, and `Diagnostics`.
- Progress messages are written for humans, not internal model diagnostics.

## Master Plan

### 1. Workflow Sequencing Overhaul

Goal: make the build pipeline stage work in the order a user expects.

Planned work:

- Audit current task dependency ordering and phase transitions.
- Change full-stack sequencing so scaffold and frontend/UI are always generated first.
- Move backend/data generation behind the initial UI shell unless hard dependencies require otherwise.
- Make integration and verification explicit later sections instead of blending them into general generation.
- Add section-level progress state so the UI can render a clear staged flow.

Acceptance:

- Full-stack builds no longer feel like a backend-first or verification-first system.
- The UI shell appears early and can be discussed, repaired, and iterated while backend work continues.

### 2. User-Facing Progress Model

Goal: replace internal-heavy status with clear progress narration.

Planned work:

- Define a user-facing stage model separate from low-level orchestration internals.
- Emit plain-English updates whenever a section starts, advances, blocks, completes, or retries.
- Distinguish clearly between:
  - current step
  - next step
  - blocker
  - recovery action
- Reduce exposure of low-signal internal labels in the default view.

Acceptance:

- A user can understand the state of a build in a few seconds without reading diagnostics.

### 3. Compact Builder UI Redesign

Goal: eliminate the long-scroll default screen.

Planned work:

- Replace the current vertical stack with a compact build cockpit.
- Promote the highest-value items into the top view:
  - build status
  - section progress
  - current task
  - next action
  - restart / resume / preview controls
- Collapse secondary detail behind tabs or linked panels.
- Ensure terminal builds show concise summaries instead of stale activity boxes.

Acceptance:

- The default page is compact and high-signal.
- The user does not have to scroll through diagnostics to manage a build.

### 4. Detail Views And Information Architecture

Goal: preserve all detail while moving it out of the main flow.

Planned work:

- Introduce focused views such as:
  - `Overview`
  - `Activity`
  - `Issues`
  - `Files`
  - `Timeline`
  - `Diagnostics`
- Move long-form orchestration panels into `Diagnostics`.
- Group checkpoints, blockers, work orders, verification reports, and journals by intent.
- Make each view readable on desktop without competing for the same space.

Acceptance:

- All current information still exists.
- The default view is no longer overloaded.

### 5. Restart And Recovery Reliability

Goal: make failed-build recovery dependable and visible.

Planned work:

- Continue hardening restart orchestration, snapshot continuity, and websocket reconnect behavior.
- Prevent terminal builds from being represented as active/live.
- Ensure restart actions always spawn visible recovery work or produce an explicit conflict/error.
- Preserve valid work and surface what was retained during recovery.

Acceptance:

- Failed builds either restart correctly or fail with a precise user-facing explanation.

### 6. Verification And Rollout

Goal: keep the overhaul shippable and measurable.

Planned work:

- Add backend tests for stage sequencing, restart behavior, and status truth.
- Add frontend tests for compact UI, terminal-state rendering, and recovery interactions.
- Verify build, lint, typecheck, and critical test suites before each push.
- Keep rollout changes incremental and logged here.

Acceptance:

- Each shipped slice has passing verification and a clear log entry.

## Work Log

### 2026-03-26

Completed:

- Diagnosed the failed-build restart issue from the live build screen and traced it to a mismatch between terminal build state and live websocket/session handling.
- Fixed terminal in-memory builds reporting `live: true` in the build details response.
- Fixed restart recovery reconnection so it uses actual websocket state instead of stale `liveSession` UI state.
- Fixed stale live agent/task panels so terminal builds do not continue showing active work boxes.
- Added regression coverage for restored terminal builds and failed-build restart reconnect behavior.
- Pushed the restart/live-session fix to `main` in commit `8ceb036`.
- Reviewed the screen recording and confirmed the larger UX problem:
  - too much vertical scroll
  - diagnostics dominating the default view
  - user-facing build flow obscured by internal telemetry
  - current progress model too internal and not section-oriented

Verification completed:

- `cd backend && go test ./internal/agents`
- `cd frontend && npm run test -- --run src/components/builder/AppBuilder.test.tsx src/services/api.test.ts`
- `cd frontend && npm run lint`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run build`

Open findings:

- The overall build workflow still needs a structural overhaul.
- Full-stack task sequencing currently does not strongly enforce a frontend-first/UI-first delivery model.
- The default build UI still contains too many long-form orchestration panels in the primary scroll path.

### 2026-03-26 (staged workflow + compact workspace pass)

Completed:

- Reworked backend phased execution so the default full-stack flow now runs in user-facing sections:
  - `Architecture`
  - `Frontend UI`
  - `Data Foundation`
  - `Backend Services`
  - `Integration`
  - `Review`
- Changed legacy fallback role ordering and build work-order ordering so frontend work is scheduled before database/backend work when a full-stack plan is generated.
- Added explicit user-facing phase start and completion messages so the planner can narrate section progress instead of only exposing internal phase names.
- Reworked the default builder progress experience into focused workspace views:
  - `Overview`
  - `Activity`
  - `Issues`
  - `Diagnostics`
  - `Console`
- Made `Overview` the compact default with:
  - staged build-flow cards
  - current update
  - recent planner/system updates
  - phase / quality / next-focus summary
- Kept only live providers, live agents, and in-progress tasks inside `Activity`.
- Added a dedicated `Issues` surface for blockers, approvals, checkpoints, diff review, and recovery controls.
- Moved the heavy orchestration report behind the `Diagnostics` view instead of leaving it in the default scroll path.
- Added empty states so `Activity` and `Issues` stay intentionally compact when nothing needs attention.
- Updated builder tests to match the new tabbed workflow and added coverage that deep panels stay hidden until explicitly opened.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/manager_spawn_test.go`
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/components/builder/AppBuilder.test.tsx`

Verification completed:

- `cd frontend && npm run test -- --run src/components/builder/AppBuilder.test.tsx`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run lint`
- `cd frontend && npm run build`
- `cd frontend && npm run test -- --run`
- `cd backend && gofmt -w internal/agents/manager.go internal/agents/build_spec.go internal/agents/manager_spawn_test.go`
- `cd backend && go build ./...`
- `cd backend && go test ./internal/agents`
- `cd backend && go test ./... -timeout=120s`

### 2026-03-26 (contract-first frontend-first reliability pass)

Completed:

- Tightened the planner and specialist contract so full-stack builds now think through backend shape deeply before codegen while still shipping the frontend shell first.
- Strengthened architect required outputs to freeze:
  - screen map
  - API contract
  - data expectations
  - environment contract
- Reworded frontend, backend, and database work orders so downstream work explicitly plugs into a frozen interface instead of re-deciding the product shape mid-build.
- Added a full-stack architect acceptance check requiring contract freeze before implementation begins.
- Hardened phased execution state by persisting the current phase and quality-gate state into build snapshot state as each phase starts, reducing reliance on websocket timing for restore/build-details truth.
- Added dedicated compact builder views for:
  - `Files`
  - `Timeline`
- Kept the default `Overview` compact while moving generated artifacts and stage history into their own focused screens.
- Added an explicit overview strategy card explaining the new contract-first, frontend-first execution model so the user can understand why backend work comes later without feeling unplanned.
- Added regression coverage for:
  - architect contract-first plan outputs
  - snapshot phase persistence for restores
  - focused builder views for files and timeline

Files changed:

- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/build_spec_test.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_spawn_test.go`
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/components/builder/AppBuilder.test.tsx`

Verification completed:

- `cd frontend && npm run test -- --run src/components/builder/AppBuilder.test.tsx`
- `cd frontend && npm run lint`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run build`
- `cd frontend && npm run test -- --run`
- `cd backend && gofmt -w internal/agents/build_spec.go internal/agents/build_spec_test.go internal/agents/manager.go internal/agents/manager_spawn_test.go`
- `cd backend && go test ./internal/agents -run 'TestCreateBuildPlanFromPlanningBundle|TestBuildExecutionPhasesPrefersFrontendBeforeBackendAndData|TestSetBuildPhaseSnapshotPersistsCurrentPhaseForRestores'`
- `cd backend && go test ./internal/agents`
- `cd backend && go build ./...`
- `cd backend && go test ./... -timeout=120s`

### 2026-03-26 (planner + timeout tightening pass)

Completed:

- Removed the stale backend-first execution guidance from the planner prompt and replaced it with an explicit frontend-first, contract-first sequencing rule.
- Tightened the lead-agent prompt so user-facing progress updates stay section-oriented and plain English instead of drifting back toward generic orchestration chatter.
- Tightened the frontend-agent prompt so the UI shell is built first against the frozen contract, with realistic loading, empty, and error states while backend work catches up.
- Increased default timeout headroom for full-stack full builds unless operators explicitly override it, reducing premature failure on longer real app generations.
- Made the global build timeout more activity-aware by extending the deadline when recent progress is still arriving, instead of hard-failing an active build at the first threshold edge.
- Increased per-phase stall grace before aborting a pending-but-not-yet-running phase, preferring queue recovery over premature failure.
- Compacted the left status rail in the builder:
  - reduced the always-visible metric set to section, live work, attention, and files
  - collapsed power mode/provider telemetry into a smaller operations summary
  - kept deeper provider detail in `Activity` / `Diagnostics`
- Preserved all prior detail views while shrinking the default surface even further.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_spawn_test.go`
- `frontend/src/components/builder/AppBuilder.tsx`
- `overhaul.md`

Verification completed:

- `cd frontend && npm run test -- --run src/components/builder/AppBuilder.test.tsx`
- `cd frontend && npm run lint`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run build`
- `cd frontend && npm run test -- --run`
- `cd backend && gofmt -w internal/agents/manager.go internal/agents/manager_spawn_test.go`
- `cd backend && go test ./internal/agents -run 'TestBuildExecutionPhasesPrefersFrontendBeforeBackendAndData|TestSetBuildPhaseSnapshotPersistsCurrentPhaseForRestores|TestBuildTimeoutForBuildGivesFullstackBuildsMoreHeadroomByDefault|TestBuildTimeoutForBuildHonorsExplicitEnvOverride'`
- `cd backend && go test ./internal/agents`
- `cd backend && go build ./...`
- `cd backend && go test ./... -timeout=120s`

Commit hash:

- local `83d8719`
- published remote `cbf3379` via GitHub connector

### 2026-03-26 (infra-maintenance resilience + builder platform status pass)

Completed:

- Hardened primary database startup by adding a bounded ping-with-retry loop before the backend declares the database ready.
- Added context-aware database health checks so runtime readiness and deep health probes reflect live database availability instead of only startup success.
- Made Redis cache status truthful after startup:
  - runtime health now pings Redis instead of assuming a non-nil client means healthy
  - cache status falls back to `memory` with a concrete reason during Redis maintenance
  - cache status recovers back to `redis` automatically once Redis responds again
- Added runtime readiness overlays so `/health`, `/health/features`, and `/ready` can reflect current Redis/database state instead of stale startup-only state.
- Tightened managed PostgreSQL provisioning fallback so temporary Postgres unavailability falls back to SQLite at ping time, not only when `sql.Open` fails immediately.
- Added a compact builder `Platform Status` notice in `Overview` so users can tell when the platform itself is degraded and jump directly to `Issues` or `Diagnostics`.
- Added regression coverage for:
  - Redis cache runtime fallback / recovery status
  - database ping retry behavior
  - runtime readiness overlays for Redis fallback
  - deep health reporting when the primary database becomes unavailable
  - compact builder platform-status rendering

Files changed:

- `backend/cmd/main.go`
- `backend/internal/api/handlers.go`
- `backend/internal/api/health_test.go`
- `backend/internal/cache/redis.go`
- `backend/internal/cache/redis_adapter.go`
- `backend/internal/cache/redis_status_test.go`
- `backend/internal/database/manager.go`
- `backend/internal/db/database.go`
- `backend/internal/db/database_health_test.go`
- `backend/internal/startup/registry.go`
- `backend/internal/startup/registry_test.go`
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/components/builder/AppBuilder.test.tsx`
- `frontend/src/services/api.ts`

Verification completed:

- `cd frontend && npm run test -- --run src/components/builder/AppBuilder.test.tsx`
- `cd frontend && npm run lint`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run build`
- `cd frontend && npm run test -- --run`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/cache ./internal/db ./internal/startup ./internal/api`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash:

- local `7aabc26`
- published remote `e14ca56` via GitHub connector

### 2026-03-26 (maintenance-aware build failure attribution + build-history retry pass)

Completed:

- Normalized build-session and build-history failures so primary database interruptions no longer masquerade as `404 build not found` or generic unclassified `503` responses.
- Added structured build error metadata for platform-originated outages:
  - `platform_issue`
  - `platform_service`
  - `platform_issue_type`
  - `platform_issue_summary`
  - `retryable`
  - `maintenance_window`
- Added bounded retry/backoff for build-history reads so transient database reconnect windows have a chance to recover before the UI shows an outage.
- Hardened these build surfaces against temporary database interruptions:
  - build details / status restore reads
  - restart / message restore path
  - build history list
  - completed build detail
  - completed build download
  - artifact reads / apply path
  - snapshot-backed messages, permissions, checkpoints, tasks, agents, and files
- Tightened the builder failure UX so failed builds can be framed as platform-related when runtime health is degraded instead of always reading like app-code failure.
- Added compact `Failure Context` cards in `Overview`, `Issues`, and `Console` so the user can quickly tell:
  - whether the failure is likely platform-related
  - which platform service is implicated
  - whether the failure is retryable
  - what the captured build error was

Files changed:

- `backend/internal/agents/handlers.go`
- `backend/internal/agents/handlers_test.go`
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/components/builder/AppBuilder.test.tsx`

Verification completed:

- `cd frontend && npm run test -- --run src/components/builder/AppBuilder.test.tsx`
- `cd frontend && npm run lint`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run build`
- `cd frontend && npm run test -- --run`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestListBuildsReturnsPlatformIssueWhenDatabaseUnavailable|TestGetCompletedBuildReturnsPlatformIssueWhenDatabaseUnavailable|TestSendMessageReturnsPlatformIssueWhenSnapshotLookupFails|TestGetBuildDetailsMarksRestoredTerminalBuildAsNotLive|TestRestartFailedBuildRestoresSnapshotAndQueuesRevision'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

## Logging Rules

For every completed work item during this overhaul, append:

- date
- short change summary
- files changed
- verification run
- commit hash if pushed

Do not remove old entries. Keep this file as the running implementation log until the overhaul is complete.
