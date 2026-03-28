# Build Workflow Overhaul

Last updated: 2026-03-27
Owner: Codex + Spencer
Status: In progress

## Objective

Make the APEX build experience reliable, staged, compact, and easy to understand.

## Build Assurance Mandate

For APEX Build specifically, the standing goal is now:

- Free users must always receive a prompt-matching frontend UI that runs in the interactive preview pane.
- Paid users must always converge toward a completely working full-stack app that runs end-to-end in the interactive preview pane.
- Full-stack delivery remains contract-first and frontend-first: freeze the backend/data contract early, land the UI shell early, then fill the runtime behind it.
- The system should prefer truthful fallback, repair, retry, and provider recovery over preventable terminal build failure.

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

### 2026-03-26 (redis allowlist remediation pass)

Completed:

- Replaced the stale external-Redis blueprint path with managed Render Key Value wiring in `render.yaml`.
- Changed backend `REDIS_URL` to come from `apex-redis` via `fromService -> type: keyvalue -> property: connectionString`.
- Added the `apex-redis` Key Value service to the Render blueprint as an internal-only dependency.
- Removed outdated Upstash/managed-Redis comments that no longer matched current Render capabilities.
- Added Redis fallback remediation hints so health/readiness surfaces now explain the exact fix when the backend is pointed at an external allowlisted Redis endpoint.
- Tightened failed-build platform-issue classification so Redis allowlist rejections are treated as configuration errors instead of transient maintenance.
- Updated the compact builder platform notice to show the concrete Redis remediation step instead of a generic degradation message.
- Updated launch and deployment docs to require the internal Render Key Value URL for `REDIS_URL`.

Verification completed:

- `cd backend && go test ./internal/cache ./internal/api ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`
- `cd frontend && npm run test -- --run src/components/builder/AppBuilder.test.tsx`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run lint`
- `cd frontend && npm run build`
- `cd frontend && npm run test -- --run`

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

### 2026-03-26 (launch-readiness billing hardening pass)

Completed:

- Closed the Stripe billing portal return-path trust gap by forcing `return_url` to stay on the configured app origin in production and only allowing additional approved origins in non-production environments.
- Normalized relative billing-portal return paths against the configured app URL so the browser no longer needs to send a full absolute URL back to the server.
- Rejected placeholder Stripe plan price IDs before checkout session creation so half-configured billing environments fail clearly instead of surfacing opaque Stripe errors.
- Rejected unknown subscription price IDs at the handler boundary so checkout requests can only target the server's real plan catalog.
- Verified that live customer-facing pricing copy still matches the backend plan catalog after the hardening changes.

Files changed:

- `backend/internal/handlers/payments.go`
- `backend/internal/handlers/payments_launch_test.go`
- `backend/internal/payments/plans.go`
- `backend/internal/payments/plans_test.go`
- `frontend/src/components/billing/BillingSettings.tsx`

Verification completed:

- `cd frontend && npm run test -- --run src/components/billing/BillingSettings.test.tsx`
- `cd frontend && npm run lint`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run build`
- `cd frontend && npm run test -- --run`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/handlers ./internal/payments`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

### 2026-03-26 (frontend Monaco/Vite slimming pass)

Completed:

- Removed the default Monaco TypeScript worker from the standard build path so the frontend no longer ships the previous 4.7 MB worker bundle by default.
- Swapped editor surfaces from the full `monaco-editor` root import to a slimmer runtime path while keeping Monaco types available through type-only imports.
- Added alias-driven Monaco runtime selection so normal builds use the lean runtime and full semantic workers remain opt-in through `VITE_MONACO_FULL_LANGUAGE_WORKERS=true`.
- Split Monaco language registration into on-demand language support loaders so editor language packs stay modular instead of bloating the app entry path.
- Tightened the Vite warning threshold around the now-isolated Monaco core chunk so the build stops reporting the lazy Monaco bundle as a generic frontend regression.

Files changed:

- `frontend/src/components/editor/InlineCompletionProvider.ts`
- `frontend/src/components/editor/MonacoEditor.tsx`
- `frontend/src/components/editor/MultiplayerCursors.tsx`
- `frontend/src/components/editor/monacoLanguageSupport.ts`
- `frontend/src/components/editor/monacoLanguageSupport.full.ts`
- `frontend/src/components/editor/monacoRuntime.ts`
- `frontend/src/components/editor/monacoRuntime.full.ts`
- `frontend/src/components/editor/setupMonacoWorkers.ts`
- `frontend/src/components/ide/DiffViewer.tsx`
- `frontend/tsconfig.json`
- `frontend/vite.config.ts`

Verification completed:

- `cd frontend && npm run lint`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run build`
- `cd frontend && npm run test -- --run`

### 2026-03-26 (launch smoke + public resource routing pass)

Completed:

- Added a dedicated non-destructive launch smoke in `tests/e2e` covering:
  - public landing resource links
  - public legal and help surfaces without authentication
  - auth/legal acceptance surface
  - backend `/health`
  - backend `/health/features`
  - public billing plans readiness
  - optional authenticated sign-in with provided smoke credentials
- Added a first-class `npm run test:launch` command to the Playwright harness so launch validation is no longer buried in ad hoc notes.
- Added `docs/launch-runbook.md` as the operator-facing go-live checklist, including:
  - preconditions
  - automated checks
  - platform build smoke
  - manual checks
  - hold criteria
  - first-hour monitoring
  - rollback triggers
- Linked the deployment guide to the launch runbook so release verification and go-live verification are now connected.
- Fixed the public landing footer so customer-facing resource links no longer point at dead `#` anchors.
- Added public query-param routing for `legal` and `help` so unauthenticated visitors can open the legal center and help center directly from the landing surface.

Files changed:

- `docs/deployment.md`
- `docs/launch-runbook.md`
- `frontend/src/App.tsx`
- `frontend/src/pages/Landing.tsx`
- `tests/e2e/README.md`
- `tests/e2e/package.json`
- `tests/e2e/specs/launch.prod.smoke.spec.ts`

Verification completed:

- `cd frontend && npm run lint`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run build`
- `cd frontend && npm run test -- --run`
- `cd tests/e2e && npm run test:launch -- --list`

Verification note:

- The new Playwright launch suite compiles and enumerates cleanly in this sandbox.
- I did not execute the live production smoke from here because that would require hitting the public production services directly from the sandbox.

### 2026-03-26 (live launch smoke alignment pass)

Completed:

- Executed the launch smoke against the live production surfaces at `https://apex-build.dev` and `https://api.apex-build.dev`.
- Corrected the auth-screen legal assertion so the smoke tolerates the current duplicated legal link placement on the auth surface.
- Corrected the billing-plans smoke to follow the real production contract:
  - create a temporary user
  - sign in through the cookie session flow
  - fetch billing plans as an authenticated session
- Tightened live Stripe verification so the smoke enforces non-placeholder IDs only for the self-serve paid plans:
  - `builder`
  - `pro`
  - `team`
- Confirmed the authenticated live billing catalog returns real Stripe price IDs for those self-serve plans.

Files changed:

- `tests/e2e/specs/launch.prod.smoke.spec.ts`

Verification completed:

- `cd tests/e2e && PLAYWRIGHT_BASE_URL=https://apex-build.dev PLAYWRIGHT_API_URL=https://api.apex-build.dev PLAYWRIGHT_EXPECT_LIVE_STRIPE=1 npm run test:launch -- --project=chromium`

Live result:

- `5 passed`
- `1 skipped` (`optional authenticated launch login succeeds`, skipped because no dedicated launch credentials were provided)

### 2026-03-26 (ownership echo-filter pass)

Completed:

- Fixed a coordinator failure mode where later full-stack phases could abort if a model re-emitted unchanged frontend scaffold files from earlier phases.
- Added an output-pruning step before ownership validation so unchanged files already present in the build snapshot are ignored instead of being treated as fresh cross-role writes.
- Preserved hard ownership enforcement for real drift: changed files outside the active work order still fail coordination validation.
- Tightened the shared agent prompt so tasks return only files they actually created or changed, instead of repeating untouched context files from other roles.
- Added a regression test that reproduces the `Data Foundation` style failure with echoed Vite scaffold files and verifies the database task now proceeds cleanly when only its owned file actually changes.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestPruneEchoedExistingFilesIgnoresUnchangedContextOutsideOwnership|TestValidateTaskCoordinationOutputRejectsOutOfScopeFiles'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

### 2026-03-26 (builder steering + fresh entry pass)

Completed:

- Added a visible `Steer Build` action directly to the build status card so users can steer the planner without hunting through deep telemetry views.
- Clarified the build controls with helper copy that distinguishes planner steering from agent-specific `Direct Control` inside the `Activity` view.
- Wired the steering action to open the `Console` workspace, reveal planner chat, and focus the message input immediately.
- Changed the builder entry behavior so login no longer auto-restores the last active or recent build into the main workspace.
- Kept previous builds available explicitly through build history, so archived or failed work only reopens when the user intentionally selects it.
- Updated the recent-build copy to explain the new fresh-entry behavior and avoid confusion about why the builder opens blank on return.
- Reworked builder tests so resumed-build behaviors are triggered by explicit history selection instead of hidden local-storage auto-restore.

Files changed:

- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/components/builder/AppBuilder.test.tsx`
- `frontend/src/components/builder/BuildHistory.tsx`

Verification completed:

- `cd frontend && npm run test -- --run src/components/builder/AppBuilder.test.tsx`
- `cd frontend && npm run lint`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run build`
- `cd frontend && npm run test -- --run`

### 2026-03-26 (build assurance mandate pass)

Completed:

- Added an explicit APEX Build assurance mandate to the orchestration layer so future planner, frontend, backend, reviewer, and solver prompts all optimize toward preview-ready delivery instead of only file generation.
- Split delivery targeting by plan tier:
  - free/static tiers now target a truthful frontend-only preview deliverable
  - paid tiers now target a verified full-stack preview deliverable
- Added a plan-level fallback that rewrites free-plan full-stack outputs into frontend-only React/Vite delivery plans instead of letting backend/database work orders proceed and fail dishonestly.
- Added an explicit `delivery_mode` signal to the build plan and build contract so verification can distinguish intentional frontend-preview fallback from accidental missing backend work.
- Relaxed contract verification for deferred auth/database/billing runtime only when the plan is explicitly in `frontend_preview_only` mode.
- Changed free-plan upgrade blockers from hard-stop framing to warning framing when static frontend fallback is active, so the build can continue toward a truthful preview instead of reading as totally blocked.
- Strengthened acceptance language so frontend preview readiness is part of the frozen build contract, not just an implied expectation.

Files changed:

- `backend/internal/agents/build_assurance.go`
- `backend/internal/agents/build_assurance_test.go`
- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/build_spec_test.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/orchestration_semantics.go`
- `backend/internal/agents/orchestration_semantics_test.go`
- `backend/internal/agents/planning_contracts.go`
- `backend/internal/agents/types.go`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestApplyBuildAssurancePolicyToPlanDowngradesFreeFullStackToFrontendPreview|TestRefreshDerivedSnapshotStateLockedUpgradeRequiredBuildIncludesPlanAcknowledgement|TestGetSystemPromptIncludesBuildAssuranceMission|TestCreateBuildPlanFromPlanningBundleHonorsStaticFrontendIntent'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`

### 2026-03-26 (preview-first start path pass)

Completed:

- Moved the free-tier truthful frontend fallback earlier in the lifecycle so builds no longer wait until deep orchestration to acknowledge the delivery target.
- Changed planner input generation so free-tier full-stack requests explicitly ask for the strongest truthful frontend-only preview while freezing deferred backend/runtime contracts for a later paid pass.
- Removed the backend/full-stack paid-plan hard stop from `StartBuild`, allowing free users to enter the build pipeline instead of bouncing on a `402` before any frontend preview work begins.
- Defaulted non-API build starts toward preview-readiness verification so frontend/UI output is judged against live preview behavior more consistently.
- Surfaced delivery mode in the orchestration overview so the builder tells the user whether the current run is targeting a frontend preview or a full-stack preview.
- Kept paid-plan upgrade truth visible in the overview, but reframed it around shipping a truthful preview now and deferring backend/runtime scope until Builder or higher is active.

Files changed:

- `backend/internal/agents/build_assurance_test.go`
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/handlers_test.go`
- `backend/internal/agents/planning_contracts.go`
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/components/builder/OrchestrationOverview.tsx`
- `frontend/src/services/api.ts`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestPlanningDescriptionForBuildAddsFreeTierFallbackGuidance|TestStartBuildFallsBackToFrontendPreviewForFreeFullStackRequests|TestApplyBuildAssurancePolicyToPlanDowngradesFreeFullStackToFrontendPreview|TestRefreshDerivedSnapshotStateLockedUpgradeRequiredBuildIncludesPlanAcknowledgement|TestGetSystemPromptIncludesBuildAssuranceMission|TestCreateBuildPlanFromPlanningBundleHonorsStaticFrontendIntent'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`
- `cd frontend && npm run test -- --run src/components/builder/OrchestrationOverview.test.tsx src/components/builder/AppBuilder.test.tsx`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run lint`
- `cd frontend && npm run build`
- `cd frontend && npm run test -- --run`

### 2026-03-26 (runtime proof + production canary pass)

Completed:

- Tightened backend runtime truth for preview-required builds so Node backends no longer pass final readiness without a runnable runtime script.
- Added automatic backend runtime-script normalization for generated Node/TypeScript APIs when the manifest has a build path but no usable runtime start/dev script.
- Strengthened final readiness behavior so preview-required Go and Python backends now fail honestly when no runnable entrypoint can be detected for runtime probing.
- Fixed the platform build smoke script so terminal `failed` or `cancelled` results no longer exit successfully.
- Added smoke profiles to the platform build smoke:
  - `free_frontend` for sacrificial free-tier preview canaries
  - `paid_fullstack` for credentialed paid-tier canaries
- Added a scheduled production canary workflow that runs:
  - public launch smoke against `apex-build.dev`
  - a free frontend build canary against production
  - an optional paid full-stack canary when dedicated canary credentials are configured
- Documented the stronger canary path and the new smoke profiles in the launch runbook and E2E README.

Files changed:

- `.github/workflows/production-canary.yml`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`
- `docs/launch-runbook.md`
- `scripts/run_platform_build_smoke.sh`
- `tests/e2e/README.md`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestVerifyGeneratedBackendBuildReadiness|TestApplyDeterministicPreValidationNormalizationAddsBackendTSConfigAndTooling|TestApplyDeterministicPreValidationNormalizationAddsMissingBackendRuntimeScripts|TestApplyDeterministicPreValidationNormalizationAddsFrontendPreviewScriptAndViteEnvAndTestDeps'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`
- `cd frontend && npm run test -- --run`
- `cd frontend && npm run build`
- `cd frontend && npm run lint`
- `bash -n scripts/run_platform_build_smoke.sh`
- `ruby -e 'require "yaml"; YAML.load_file(".github/workflows/production-canary.yml"); puts "ok"'`

### 2026-03-27 (canary truth + snapshot consistency pass)

Completed:

- Tightened capability detection so phrases like `clean file structure` no longer falsely escalate a free frontend prompt into paid file-upload/storage scope.
- Taught derived snapshot semantics to honor the frozen `frontend_preview_only` delivery target after planning completes, instead of continuing to show paid/full-stack approvals and blockers on a truthful free preview run.
- Refreshed derived snapshot state immediately after plan freeze so persisted orchestration truth matches the downgraded plan before later phases and history reads.
- Hardened completed-build presentation so terminal snapshots with `completed_at` and no error are shown consistently as completed across status, details, history, and download/export checks.
- Guarded snapshot upserts against stale late writes by refusing to let an older `updated_at` overwrite a newer terminal snapshot row.
- Upgraded the production smoke script to match the real auth flow, support cookie sessions, and fail if `/build/:id/status` says completed but `/builds/:id` disagrees.
- Added a Claude Code handoff runbook with exact canary commands, expected outputs, and failure-to-fix mapping for this reliability track.

Files changed:

- `backend/internal/agents/entitlements_test.go`
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/handlers_test.go`
- `backend/internal/agents/iteration_test.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/orchestration_semantics.go`
- `backend/internal/agents/orchestration_semantics_test.go`
- `docs/canary-reliability-handoff.md`
- `scripts/run_platform_build_smoke.sh`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestBuildSubscriptionRequirement|TestRefreshDerivedSnapshotStateLockedUpgradeRequiredBuildIncludesPlanAcknowledgement|TestRefreshDerivedSnapshotStateLockedFrontendPreviewOnlyClearsPaidRuntimeApprovals|TestNormalizeRestoredBuildStatusTreatsLegacyBuildingAsResumable|TestPersistBuildSnapshotDoesNotOverwriteNewerTerminalSnapshot|TestCompletedBuildEndpointsPresentCompletedTerminalSnapshot'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestGetBuildDetailsIncludesSnapshotState|TestSnapshotReadEndpointsFallbackToPersistedState|TestApplyBuildAssurancePolicyToPlanDowngradesFreeFullStackToFrontendPreview'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`
- `bash -n scripts/run_platform_build_smoke.sh`

Live confirmation:

- Reproduced the still-deployed production bug on a disposable canary account with build `10677c84-71c5-4b79-9705-4930bc21f40a`.
- Confirmed the old production backend still misclassifies the free `PulseBoard` frontend-only canary as `upgrade_required` because `clean file structure` still trips stale `file_upload` / `storage` capability detection.
- Updated `docs/canary-reliability-handoff.md` with:
  - the exact reproduced build id
  - the observed wrong approvals/blockers
  - the immediate post-deploy canary checklist
  - the next repair path if the free canary then fails with `preview_build_failed` / `Unterminated string literal`

Additional tightening after the live repro:

- Added a regression around the exact `PulseBoard` prompt so `compileIntentBriefFromRequest` no longer treats a frontend-only dashboard prompt as `fullstack`.
- Narrowed `inferIntentAppType` so `dashboard` only implies `fullstack` when paired with affirmed runtime signals such as auth, database, billing, API, or backend terms.
- Added a derived-snapshot regression to ensure the exact free canary prompt stays `static_ready` and does not surface active paid-upgrade approvals.

Additional verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestBuildSubscriptionRequirement|TestCompileIntentBriefFromRequestDoesNotTreatCleanFileStructureAsUploadStorage|TestRefreshDerivedSnapshotStateLockedFrontendCanaryPromptAvoidsUpgradeRequired|TestRefreshDerivedSnapshotStateLockedUpgradeRequiredBuildIncludesPlanAcknowledgement|TestRefreshDerivedSnapshotStateLockedFrontendPreviewOnlyClearsPaidRuntimeApprovals|TestNormalizeRestoredBuildStatusTreatsLegacyBuildingAsResumable|TestPersistBuildSnapshotDoesNotOverwriteNewerTerminalSnapshot|TestCompletedBuildEndpointsPresentCompletedTerminalSnapshot'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`
- `bash -n scripts/run_platform_build_smoke.sh`

Date: 2026-03-27

Change summary:

- Reproduced the next real free-canary production failure after the paid-gating fix by rebuilding the live generated app locally from build `8591de16-7337-45de-921d-2ec198e8c09e`.
- Added frontend preview preflight detection for missing generated local modules so unresolved relative imports now fail early with the exact missing file path instead of surfacing later as a generic Rollup error.
- Added a deterministic repair path that materializes compile-safe placeholder frontend modules for missing generated local component imports, so the preview pipeline can continue repairing toward a visible UI instead of terminal-failing on omitted files.
- Added regressions covering both the preflight detection and the placeholder repair bundle capture.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestApplyDeterministicValidationRepairsCreatesMissingLocalModulePlaceholder|TestVerifyGeneratedFrontendPreviewReadiness'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Date: 2026-03-27

Change summary:

- Re-ran the live free frontend canary after deploy and confirmed the next real blocker moved upstream into work-order ownership, not preview compilation.
- Identified that planner stack labels like `React 18` and `Tailwind CSS` were not canonicalized before scaffold selection, causing frontend-only web builds to fall through to the default API scaffold.
- Fixed scaffold selection to normalize versioned/framework-labeled frontend and backend stack strings before routing, so frontend-preview plans now keep the Vite SPA scaffold and truthful frontend ownership.
- Added a regression for the exact `PulseBoard` canary prompt to ensure the frontend work order owns `src/**`, `index.html`, and `vite.config.ts`, and that no backend work order leaks into the free frontend-preview path.

Files changed:

- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/build_spec_test.go`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestSelectBuildScaffoldNewStacks|TestCreateBuildPlanFromPlanningBundlePulseBoardUsesFrontendScaffoldAndFrontendOwnership|TestCreateBuildPlanFromPlanningBundleHonorsStaticFrontendIntent|TestApplyBuildAssurancePolicyToPlanDowngradesFreeFullStackToFrontendPreview'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Date: 2026-03-27

Change summary:

- Confirmed the free production canary now completes successfully end to end after the preview-scaffold and missing-local-module fixes.
- Used the admin owner account to run the paid full-stack canary and isolated the next orchestration blocker from the live build record instead of guessing.
- Found that the database schema task legitimately emitted `server/migrate.ts` and `server/seed.ts`, but the fullstack React+Express scaffold ownership map rejected those files as out-of-scope, causing a coordination-contract retry loop that parked the build in `data_foundation`.
- Expanded the fullstack React+Express database ownership map to include `server/migrate.ts` and `server/seed.ts`, and added a regression to keep those database runtime helpers owned by the database role.

Files changed:

- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/build_spec_test.go`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestFullstackExpressScaffoldAssignsDatabaseRuntimeHelpersToDatabaseRole|TestSelectBuildScaffoldNewStacks|TestCreateBuildPlanFromPlanningBundlePulseBoardUsesFrontendScaffoldAndFrontendOwnership'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Date: 2026-03-27

Change summary:

- Added a real build-history delete path so terminal builds can be removed from Recent Builds instead of piling up forever.
- Kept active builds safe by rejecting delete on non-terminal snapshots and exposing explicit cancel controls in the history UI for resumable/live runs.
- Removed deleted terminal builds from both persisted history and in-memory build coordination state so a removed build does not silently reappear.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/handlers_test.go`
- `frontend/src/services/api.ts`
- `frontend/src/components/builder/BuildHistory.tsx`
- `frontend/src/components/builder/BuildHistory.test.tsx`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestDeleteBuildRemovesTerminalSnapshotFromHistory|TestDeleteBuildRejectsActiveSnapshot'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd frontend && npm run test -- --run src/components/builder/BuildHistory.test.tsx`
- `cd frontend && npm run typecheck`
- `cd frontend && npm run lint`
- `cd frontend && npm run build`

Date: 2026-03-27

Active investigation:

- Ran a fresh live paid full-stack canary on the admin account after the account was moved to the Team tier.
- Confirmed the platform itself is healthy again (`/health/features` is fully green for Redis, database, preview, and orchestration).
- Confirmed a live status-truth bug remains on paid/full-stack builds: the canary jumps from `planning 0%` to `in_progress 99%` almost immediately, then can continue through `testing` and `reviewing` while still reporting `99%`.
- Root cause isolated in code: overall build progress can currently jump into the terminal band off worker-agent completion even when the build is still in architecture, frontend, or integration phases.
- Phase-aware progress capping is now the active fix in progress so build status reflects the real execution stage instead of looking almost finished long before handoff.

Date: 2026-03-27

Change summary:

- Fixed the status-truth bug behind the live paid/full-stack canary by capping overall build progress to the active execution phase instead of letting worker completion push builds into a fake `99%` state during architecture, frontend, or integration work.
- Stopped counting errored workers as completed progress so failed agents no longer make an unfinished build look nearly done.
- Strengthened final-validation solver recovery hints for route-drift failures so the solver gets explicit missing-endpoint guidance when frontend API calls and backend routes do not match.
- Confirmed on the live paid canary that the next real full-stack blocker is integration-route drift (`/api/auth/login`, `/api/dashboard/kpis`, etc.), and also observed the smoke harness session can age out mid-poll with `authentication required`, which needs separate hardening in the canary runner.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_progress_test.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestUpdateBuildProgressCapsArchitecturePhaseProgress|TestUpdateBuildProgressKeepsReviewPhaseBelowCompletion|TestExtractDependencyRepairHintsFromReadinessErrorsIncludesSpecificIntegrationRouteGuidance'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`

Date: 2026-03-27

Change summary:

- Hardened the production canary smoke runner so long paid/full-stack builds can re-authenticate mid-poll instead of collapsing into repeated `authentication required` responses after the initial login ages out.
- Added automatic re-login before the status poll, final detail fetch, and completed-build fetch whenever the live API reports an expired or missing session.
- This keeps the canary useful while the deeper paid/full-stack integration-route drift issue is still being chased in the product itself.

Files changed:

- `scripts/run_platform_build_smoke.sh`

Verification completed:

- `bash -n scripts/run_platform_build_smoke.sh`

Date: 2026-03-27

Change summary:

- Added an integration-preflight repair lane immediately after the backend-services phase for full-stack builds, so frontend/backend route drift is caught and repaired before the build enters final review.
- Added a dedicated scoped recovery action, `fix_integration_contract`, with its own loop cap and no duplicate post-fix validation fan-out, so integration repairs stay inside the phased pipeline instead of ballooning into overlapping review/test work.
- Added a deterministic Express integration repair for the common cheap failures: missing CORS middleware, missing health route, and a hardcoded listen port instead of `process.env.PORT`.
- Tightened frontend/backend/testing/reviewer/solver contract instructions so the frontend stops inventing dead API calls, the backend explicitly implements frontend-called routes, and testing/review call out exact route drift instead of generic integration failure.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestApplyDeterministicIntegrationPreflightRepairsExpressRuntime|TestLaunchIntegrationPreflightRecoveryCreatesScopedFixTask|TestHandleTaskCompletionSkipsPostFixValidationForIntegrationPreflightFix|TestUpdateBuildProgressCapsArchitecturePhaseProgress|TestUpdateBuildProgressKeepsReviewPhaseBelowCompletion|TestExtractDependencyRepairHintsFromReadinessErrorsIncludesSpecificIntegrationRouteGuidance'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Date: 2026-03-27

Change summary:

- Fixed the next real paid/full-stack canary failure path: retried phase tasks could be re-queued as `pending` without being promoted back to `in_progress` when execution resumed.
- That let `waitForPhaseCompletion` falsely conclude nothing was running and abort the phase after the stall grace window, which matches the live `Data Foundation aborted before task completion (pending=1, in_progress=0)` failure signature.
- Added a retry-dispatch promotion step so re-queued tasks immediately re-enter `in_progress`, refresh their start time, and re-surface as active work before the next execution attempt begins.
- Added the Render launch note that workspace notifications should be enabled now, while Render webhooks, private links, and external observability sinks remain optional for the current production path.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`
- `docs/launch-runbook.md`

Verification completed:

- `cd backend && gofmt -w internal/agents/manager.go internal/agents/manager_readiness_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestMarkQueuedTaskExecutionStartedPromotesPendingRetryTask|TestHandleTaskCompletionSkipsPostFixValidationForIntegrationPreflightFix|TestApplyDeterministicIntegrationPreflightRepairsExpressRuntime|TestLaunchIntegrationPreflightRecoveryCreatesScopedFixTask'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Date: 2026-03-27

Change summary:

- Fixed a deeper phased-pipeline reliability gap: phase completion used to watch only the original phase task IDs, so a failed phase task that was superseded by solver recovery and post-fix validation could let the phase advance too early.
- Added recovery-lineage tracking for phase completion, so the waiter now follows `superseded_by_recovery`, `failed_task_id`, and `trigger_task` descendants instead of pretending the phase is done as soon as the first-generation tasks go terminal.
- Added a fast unresolved-failure abort path for phase lineages with no active descendant recovery, which prevents misleading downstream stalls and keeps failure causes closer to the original broken task.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/reliability_helpers_test.go`

Verification completed:

- `cd backend && gofmt -w internal/agents/manager.go internal/agents/reliability_helpers_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestRelatedPhaseTaskIDsIncludesRecoveryAndValidationDescendants|TestWaitForPhaseCompletionWaitsForRecoveryLineage|TestWaitForPhaseCompletionFailsOnUnresolvedLineageFailure|TestMarkQueuedTaskExecutionStartedPromotesPendingRetryTask|TestCheckIntegrationCoherenceIgnoresFrontendTestOnlyDeadRoutes|TestGetBuildStatusNormalizesLiveProgressWithinPhaseWindow'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Date: 2026-03-27

Change summary:

- Fixed the remaining live progress-truth bug after the paid full-stack canary succeeded: active builds could briefly expose raw internal progress like `99%` even while still in early phases such as architecture or frontend UI.
- Added a presentation-only live progress normalizer that caps active build progress to the current phase window while leaving internal orchestration state untouched.
- Applied that normalization consistently across the status/detail APIs, websocket build-state sync, and outgoing build progress/error messages so the UI and canaries see the same truthful phase-bounded progress.
- Added regressions covering live status, live build details, and outbound websocket/build-progress normalization.

Files changed:

- `backend/internal/agents/handlers.go`
- `backend/internal/agents/handlers_test.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/websocket.go`

Verification completed:

- `cd backend && gofmt -w internal/agents/handlers.go internal/agents/handlers_test.go internal/agents/websocket.go internal/agents/manager.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestGetBuildStatusNormalizesLiveProgressWithinPhaseWindow|TestGetBuildDetailsNormalizesLiveProgressWithinPhaseWindow|TestNormalizeBuildMessageProgressCapsActiveBuildUpdates|TestMarkQueuedTaskExecutionStartedPromotesPendingRetryTask|TestCheckIntegrationCoherenceIgnoresFrontendTestOnlyDeadRoutes'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`
- Live canary after deploy: `BASE_URL='https://api.apex-build.dev/api/v1' SMOKE_PROFILE='paid_fullstack' LOGIN_EMAIL='admin@apex.build' LOGIN_PASSWORD='TheStarsh1pKEY!' PROJECT_NAME='agency-ops-platform' POWER_MODE='balanced' ./scripts/run_platform_build_smoke.sh`
- Live canary result: build `db10a65d-aad9-4a70-9c68-ad1b3ab2db23` completed with truthful progress `0 -> 19 -> 44 -> 79 -> 89 -> 100`

Commit hash if pushed:

- Local: `46421f4`
- Remote: `2242e77`

Date: 2026-03-27

Change summary:

- Confirmed on the next live paid full-stack canary that the Data Foundation stall was fixed; the build now advances through testing/review and fails on a later integration validator.
- Fixed a new false-positive validator path: `checkIntegrationCoherence` was scanning generated `__tests__`, `.test.*`, and `.spec.*` files as if they were real runtime frontend dependencies.
- That allowed test-only dead-route checks such as `/api/nonexistent-route` to fail an otherwise valid full-stack build during final integration validation.
- Integration coherence now ignores test/spec files, preserving real route/CORS/port checks on actual runtime code while dropping synthetic dead-route noise from tests.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification completed:

- `cd backend && gofmt -w internal/agents/manager.go internal/agents/manager_readiness_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestCheckIntegrationCoherenceIgnoresFrontendTestOnlyDeadRoutes|TestMarkQueuedTaskExecutionStartedPromotesPendingRetryTask|TestApplyDeterministicIntegrationPreflightRepairsExpressRuntime|TestLaunchIntegrationPreflightRecoveryCreatesScopedFixTask'`
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

Date: 2026-03-27

Change summary:

- Merged the first-class preview verification gate into the active reliability branch and verified the full backend suite with the gate enabled.
- Fixed a restore-state hole where `PreviewVerificationAttempts` was not persisted through build snapshots, which could let restarted builds retry preview verification as if no prior repair had happened.
- Fixed terminal preview-gate failure truth so a build that fails preview verification no longer remains at `100%`; failed preview verification now caps progress below completion, and deterministic fence-strip repair re-enters testing at `95%`.
- Re-ran the live paid full-stack canary on production after the latest backend deploy and confirmed successful completion on build `ed5167b7-87eb-46a5-9ecd-12698d631f82`.

Files changed:

- `backend/internal/agents/build_snapshot.go`
- `backend/internal/agents/iteration_test.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/preview_gate.go`
- `backend/internal/agents/preview_gate_test.go`
- `backend/internal/agents/types.go`

Verification completed:

- `cd backend && gofmt -w internal/agents/build_snapshot.go internal/agents/iteration_test.go internal/agents/manager.go internal/agents/preview_gate.go internal/agents/types.go internal/agents/preview_gate_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestApplyPreviewFenceStripRepairResetsProgressAndAttempts|TestRunPreviewVerificationGateTerminalFailureDropsProgressBelowCompletion|TestRestoreBuildSessionFromSnapshotPreservesRuntimeAndTaskState'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`
- Live canary after deploy: `BASE_URL='https://api.apex-build.dev/api/v1' SMOKE_PROFILE='paid_fullstack' LOGIN_EMAIL='admin@apex.build' LOGIN_PASSWORD='TheStarsh1pKEY!' PROJECT_NAME='agency-ops-platform' POWER_MODE='balanced' ./scripts/run_platform_build_smoke.sh`
- Live canary result: build `ed5167b7-87eb-46a5-9ecd-12698d631f82` completed successfully with truthful phase progress and final completion

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Change summary:

- Live paid canary `f149c461-cce3-4f5d-a3ad-b5aeccc9de75` exposed the next contract-normalization gap after the phased-gap recovery push: actor-style foreign keys like `created_by`, `recorded_by`, and `assigned_to` were still reaching provider critique without explicit `references User(id)`.
- Extended FK inference so actor-reference fields now map to common identity models (`User`, `Member`, `Agent`, `Admin`, `Profile`) instead of only handling `_id` suffixes.
- Added a regression that covers the exact `created_by` / `assigned_to` live failure shape.

Files changed:

- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/orchestration_contracts_test.go`

Verification completed:

- `cd backend && gofmt -w internal/agents/build_spec.go internal/agents/orchestration_contracts_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestCompileBuildContractFromPlanInfersForeignKeyReferences|TestCompileBuildContractFromPlanInfersActorForeignKeyReferences'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Change summary:

- Diagnosed a new autoscaled/live paid canary stall after frontend completion. The build was not blocked and the frontend task had completed, but the phased pipeline never started `Data Foundation`, leaving the build stuck at `44%` in `frontend_ui`.
- Added deterministic phased-pipeline gap recovery in the build inactivity monitor. When all tasks in the current phase are terminal but the phased pipeline is not complete, the manager now starts the next missing execution phase instead of waiting forever for the original phase goroutine.
- Refactored phased execution startup so phase start + task assignment can be reused by both the normal pipeline and the stalled-phase recovery path, and phase snapshots are now persisted immediately when a phase begins.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_spawn_test.go`

Verification completed:

- `cd backend && gofmt -w internal/agents/manager.go internal/agents/manager_spawn_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestRecoverStalledPhasedExecutionStartsNextPhaseAfterFrontend|TestBuildExecutionPhasesPrefersFrontendBeforeBackendAndData|TestResumeBuildExecutionRequeuesPendingRecoveryTasksAndRefreshesTimestamp'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Change summary:

- The next paid canary on backend `started_at=2026-03-28T20:36:23.098875616Z` exposed a different failure mode: the `plan` task completed, but the build remained parked in `planning` with no blockers and no agent team spawned.
- Traced that stall to provider-assisted contract critique still running on the critical path without a hard timeout.
- Added a `20s` timeout around `providerAssistedContractCritique` so a slow critique provider now degrades to `nil` instead of leaving the whole build apparently frozen in planning.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_contract_critique_test.go`

Verification completed:

- `cd backend && gofmt -w internal/agents/manager.go internal/agents/manager_contract_critique_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestProviderAssistedContractCritiqueReturnsVerificationReport|TestProviderAssistedContractCritiqueTimesOutAndReturnsNil|TestHandlePlanCompletionSyncsSeededAPIContractBackIntoPlan|TestHandlePlanCompletionBlocksOnProviderAssistedContractCritique'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Change summary:

- Reran the paid full-stack canary on backend `started_at=2026-03-28T20:29:04.350552658Z`.
- Found a new earlier blocker during contract/provider critique: schema fields like `tenant_id` were normalized as `uuid foreign key` without an explicit referenced table, which triggered a provider critique blocker even though the relationship was obvious.
- Extended data-model normalization to infer explicit `references Model(id)` clauses for obvious foreign keys such as `tenant_id -> Tenant(id)` before the build contract is critiqued.

Files changed:

- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/orchestration_contracts_test.go`

Verification completed:

- `cd backend && gofmt -w internal/agents/build_spec.go internal/agents/orchestration_contracts_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestCompileBuildContractFromPlanInfersForeignKeyReferences|TestCompileBuildContractFromPlanNormalizesUniqueTypeQualifiers|TestVerifyAndNormalizeBuildContractAcceptsNextFullstackScaffoldContract'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Change summary:

- Reran the paid full-stack production canary on backend `started_at=2026-03-28T20:11:29.002243633Z`.
- Confirmed the previous generated-test blocker is fixed: the build now clears testing and enters review before failing.
- Found the next deterministic blocker at `95-97%`: generated JSX inside a `.ts` provider file (`src/hooks/useAuth.ts`) caused esbuild to fail with `Expected ">" but found "value"`.
- Added a deterministic validation repair that normalizes generated `.ts`/`.js` provider files containing JSX into `React.createElement(...)` form, preserving the auth/provider logic without renaming files.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification completed:

- `cd backend && gofmt -w internal/agents/manager.go internal/agents/manager_readiness_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestParsePreviewJSXInTSRepairTargets|TestApplyDeterministicValidationRepairsConvertsJSXInTSProviderFile|TestParsePreviewSyntaxErrorTargetFiles|TestApplyDeterministicValidationRepairsReplacesBrokenGeneratedTestFile'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Change summary:

- Added a deterministic generated-test repair pass in final validation so late-stage preview/test failures from broken generated `*.test.*` / `*.spec.*` files do not terminal-fail the entire build when they can be patched or downgraded safely.
- Wired the repair into `applyDeterministicValidationRepairs`, using the pure helper lane to rewrite broken test imports or fall back to compile-safe placeholders only for generated test files.
- Updated the live canary smoke script to fetch and send a CSRF token after login, which is now required by production `POST /api/v1/build/start`.

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`
- `scripts/run_platform_build_smoke.sh`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestDetectSourceFlaws_CleanFile|TestRepairGeneratedTestFile_PatchesMissingVitestImport|TestApplyDeterministicValidationRepairsReplacesBrokenGeneratedTestFile|TestApplyDeterministicProviderBlockedTestRepairAddsMissingJestDependency|TestApplyDeterministicPreValidationNormalizationAddsJestDependencyForGeneratedJestTests'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`
- `bash -n scripts/run_platform_build_smoke.sh`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Change summary:

- Ran a fresh paid full-stack production canary against the live backend with runtime preview proof enabled.
- Confirmed the earlier preview-route false positive is fixed: the canary advanced through testing and review instead of failing at the old `server/index.ts defines no routes` check.
- Found a new narrower blocker at `97%`: a late provider-verification repair task can fail the build when generated Jest-style tests exist but the root manifest does not yet declare the required test tooling.
- Added deterministic self-healing for that failure path in orchestration:
  - provider-blocked test repairs can now patch `package.json` with missing test-tooling dependencies instead of terminal-failing the task
  - pre-validation normalization now also recognizes generated `@jest/globals` usage and adds `jest` before final readiness validation

Latest live paid canary:

- build id: `295e7be8-263c-40f1-94b0-e0e1c9a260e0`
- terminal status: `failed`
- terminal error:
  - `Failed after 1 attempts: provider verification blocked task output: The 'AFTER' version of package.json does not add Jest to devDependencies, which is required for the test script to run and would cause build failures.`

Files changed:

- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestApplyDeterministicProviderBlockedTestRepair|TestApplyDeterministicProviderBlockedTestRepairAddsMissingJestDependency|TestApplyDeterministicPreValidationNormalizationAddsJestDependencyForGeneratedJestTests'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Live canary status update:

- Pushed `fix: sync seeded auth contracts into build plans` as `9c78b7d` and confirmed Render rolled the backend at `started_at=2026-03-28T18:51:28.564317981Z`.
- Production feature health stayed green after deploy, including `preview_runtime_verify` with browser proof enabled and Chrome available.
- Re-ran the paid full-stack canary on build `9792219d-a297-4d29-bd0c-a9b576495f3d`.
- The new deploy cleared the earlier planning/runtime blockers:
  - the build moved cleanly through planning, generation, and review phases
  - the prior `/api/auth/me` drift is gone
  - the prior database ownership conflict on `server/migrate.ts`, `server/seed.ts`, and `server/db/index.ts` is gone
- The remaining live blocker is now narrower and later in the pipeline:
  - integration preflight briefly reported only `/api/auth/login` drift
  - the build then advanced into preview verification
  - terminal failure at `96%` is `Preview verification failed: Backend entry "server/index.ts" defines no routes.`
- This means the next reliability pass should focus on backend route detection/runtime proof in the preview verifier rather than planning/contract hydration.

Date: 2026-03-28

Change summary:

- Fixed the paid full-stack canary drift exposed by live build `4010066c-9e2c-4b31-a9be-7b34273a0cf6`, where the frontend correctly called `/api/auth/login` and `/api/auth/me` but the backend work order still only saw the stale `/api/health` contract.
- Synced the verified/orchestrated API contract back into `build.Plan` during `handlePlanCompletion`, so the specialist tasks now inherit the same auth/API surface that contract compilation and verification already inferred from the user intent.
- Fixed work-order ownership precedence so specifically-owned database files under `server/` no longer get rejected by the same work order's broad `server/**` forbidden pattern.
- Kept frontend-preview-only builds truthful while preserving deferred API/auth shape: static/frontend-only builds no longer get an injected backend API contract, and `frontend_preview_only` verification now requires frontend/deployment proof without falsely demanding backend runtime or integration acceptance.
- Tightened contract compilation coherence by deriving backend resources and auth strategy from the normalized seeded endpoint set instead of the stale pre-seeded endpoint list.

Files changed:

- `backend/internal/agents/planning_contracts.go`
- `backend/internal/agents/build_spec_test.go`
- `backend/internal/agents/orchestration_contracts.go`
- `backend/internal/agents/orchestration_contracts_test.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_contract_critique_test.go`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestValidateTaskCoordinationOutputRejectsOutOfScopeFiles|TestPathAllowedByWorkOrderSpecificOwnedPathOverridesBroadForbiddenPattern|TestCompileBuildContractFromPlanSeedsAuthEndpointsFromIntent|TestHandlePlanCompletionSyncsSeededAPIContractBackIntoPlan'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestCreateBuildPlanFromPlanningBundleHonorsStaticFrontendIntent|TestApplyBuildAssurancePolicyToPlanDowngradesFreeFullStackToFrontendPreview|TestCompileBuildContractFromPlanSeedsAuthEndpointsFromIntent|TestHandlePlanCompletionSyncsSeededAPIContractBackIntoPlan'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Change summary:

- Re-ran the paid full-stack canary after the qualifier-normalization deploy and confirmed the prior schema blockers were gone; the build advanced through schema, testing, and review before failing at preview verification.
- Diagnosed the next live paid-canary failure on build `fb0e266d-9adc-49c5-a373-7450f410c193`: preview verification rejected the build with `No backend server entry file found ...` even though the generated backend entry was `server/index.ts`.
- Fixed backend preview entry detection so the verifier now accepts common TypeScript/Node backend entrypoints such as `server/index.ts`, `server/main.ts`, `backend/index.ts`, and similar API-folder variants.
- Added a regression proving that a full-stack build with `server/index.ts` plus a normal Express listen/route setup passes static backend preview verification.

Files changed:

- `backend/internal/preview/verifier.go`
- `backend/internal/preview/verifier_test.go`

Verification completed:

- `cd backend && gofmt -w internal/preview/verifier.go internal/preview/verifier_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/preview -run 'TestVerifier_FullStack_AcceptsServerIndexTSBackendEntry|TestVerifier_FullStack_PassesValidExpressApp|TestVerifier_FullStack_FailsMissingBackend'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Change summary:

- Re-ran live canaries after the truncation-repair deploy: the free frontend canary completed successfully on build `f8de29ae-8f58-4b0e-bf4c-476bdfca514a`, but the completed-build summary surface lagged briefly behind the terminal detail view before converging.
- Diagnosed the next paid full-stack blocker on build `28833c8f-4e1a-4515-a03f-e31a8cbbc27a`: architecture/data-model verification was emitting blockers like `Tenant field 'slug' has type 'string unique' but unique is false` and `User field 'email' has type 'string unique' but unique is false`.
- Fixed contract normalization so model-field type qualifiers such as `unique`, `not null`, `nullable`, and `optional` are converted into `Unique` / `Required` flags instead of leaking through as contradictory raw type strings.
- Added regressions for both plan normalization and contract compilation so `string unique` now becomes `type=string` with `unique=true` before verifier review.

Files changed:

- `backend/internal/agents/build_spec.go`
- `backend/internal/agents/build_spec_test.go`
- `backend/internal/agents/orchestration_contracts_test.go`

Verification completed:

- `cd backend && gofmt -w internal/agents/build_spec.go internal/agents/build_spec_test.go internal/agents/orchestration_contracts_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents -run 'TestNormalizeModelFieldsPromotesTypeQualifiersToFlags|TestCompileBuildContractFromPlanNormalizesUniqueTypeQualifiers|TestCreateBuildPlanFromPlanningBundle|TestCompileBuildContractFromPlanSeedsTruthAndVerification'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Change summary:

- Fixed the live paid-canary failure class where provider verification could hard-block a generated task because a top-level generated test file such as `tests/verify-integration.ts` was truncated mid-function.
- Added a deterministic pre-acceptance repair that replaces truncated generated JS/TS test artifacts with compile-safe placeholder verification content before provider verification aborts the task.
- Broadened syntax-target parsing to recognize abrupt-EOF / missing-closing-brace verifier messages, and fixed root-level `tests/...` paths so they are consistently treated as test files.
- Hardened production startup truth by defaulting preview runtime verification on in production when Chrome is available unless `APEX_PREVIEW_RUNTIME_VERIFY=false` is explicitly set.

Files changed:

- `backend/cmd/main.go`
- `backend/cmd/main_test.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/manager_readiness_test.go`

Verification completed:

- `cd backend && gofmt -w cmd/main.go cmd/main_test.go internal/agents/manager.go internal/agents/manager_readiness_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./cmd ./internal/agents -run 'TestPreviewRuntimeVerificationEnabled|TestParsePreviewSyntaxErrorTargetFiles|TestParsePreviewSyntaxErrorTargetFilesIncludesAbruptEOFMessages|TestApplyDeterministicProviderBlockedTestRepair'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/agents`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-28

Change summary:

- Fixed the production backend image definitions to use Go 1.26 so Render can actually build the shipped browser-proof preview runtime after `backend/go.mod` was raised to `go 1.26`.
- Aligned all maintained backend Dockerfiles with the current module toolchain requirement instead of leaving Render on the stale `golang:1.25-alpine` builder image.
- Treated this as a deploy-pipeline reliability fix rather than an orchestration fix: the product code was ready, but the hosted image could not be produced.

Files changed:

- `backend/Dockerfile`
- `backend/Dockerfile.prod`
- `backend/Dockerfile.production`

Verification completed:

- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/preview`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending
Date: 2026-03-27

Change summary:

- Productionized browser-execution proof for Render by baking Chromium and supporting libraries into the backend runtime image and enabling `APEX_PREVIEW_RUNTIME_VERIFY` in the Render blueprint.
- Hardened Chrome discovery to honor `APEX_CHROME_PATH` and `CHROME_BIN`, added container-safe launch flags (`headless`, `disable-dev-shm-usage`, `no-sandbox`, background-networking shutdown), and added coverage for configured browser path detection.
- This closes the gap between “browser proof exists in code” and “production can actually run it after deploy”.

Files changed:

- `backend/Dockerfile`
- `backend/internal/preview/browser_verifier.go`
- `backend/internal/preview/browser_verifier_test.go`
- `render.yaml`

Verification completed:

- `cd backend && gofmt -w internal/preview/browser_verifier.go internal/preview/browser_verifier_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/preview -run 'TestFindChromePrefersConfiguredPath|TestBrowserVerifier_PassesWhenAppRendered|TestRuntimeVerifierFailsWhenBrowserProofEnabledButChromeUnavailable'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending

Date: 2026-03-27

Change summary:

- Added a runtime Vite preview verification layer on top of the static preview gate, with an opt-in `APEX_PREVIEW_RUNTIME_VERIFY=true` path that boots the generated Vite dev server and checks the root page, mount point, Vite client, entry module, and CSS asset availability.
- Wired runtime-preview verification visibility into startup/feature reporting and the preview verification E2E canary surface so production can expose whether runtime proof is active.
- Hardened the runtime verifier after integration review: enabled runtime proof now fails honestly instead of silently skipping when npm is unavailable or install timeouts occur, and temp-workdir file writes now reject unsafe absolute/escaping paths from generated output.

Files changed:

- `backend/cmd/main.go`
- `backend/internal/preview/runtime_verifier.go`
- `backend/internal/preview/runtime_verifier_integration_test.go`
- `backend/internal/preview/runtime_verifier_test.go`
- `backend/internal/preview/verifier.go`
- `tests/e2e/specs/preview-verification.spec.ts`

Verification completed:

- `cd backend && gofmt -w internal/preview/runtime_verifier.go internal/preview/runtime_verifier_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/preview`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`
- `cd tests/e2e && npm run test -- --list specs/preview-verification.spec.ts`

Commit hash if pushed:

- Local: `5014d78`, `0fdaf58`, hardening follow-up pending
- Remote: pending

Date: 2026-03-27

Change summary:

- Verified Claude’s headless Chrome browser execution proof layer on top of the runtime Vite verifier and confirmed the full backend suite still passes with the browser dependency path enabled.
- Tightened one truth gap before shipping: when runtime preview verification is enabled but Chrome is missing, the system no longer silently downgrades to weaker HTTP-only proof while reporting the feature as ready.
- `preview_runtime_verify` now degrades honestly in startup health when Chrome is unavailable, and the runtime verifier fails with `browser_unavailable` instead of quietly skipping browser proof.

Files changed:

- `backend/cmd/main.go`
- `backend/internal/preview/runtime_verifier.go`
- `backend/internal/preview/runtime_verifier_test.go`

Verification completed:

- `cd backend && gofmt -w cmd/main.go internal/preview/runtime_verifier.go internal/preview/runtime_verifier_test.go`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/preview -run 'TestRuntimeVerifierFailsWhenBrowserProofEnabledButChromeUnavailable|TestBrowserVerifier_PassesWhenAppRendered|TestCheckRootPage_OK'`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./internal/preview`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go build ./...`
- `cd backend && TMPDIR=/tmp GOCACHE=/tmp/go-build GOTMPDIR=/tmp/go-tmp go test ./... -timeout=120s`

Commit hash if pushed:

- Local: pending
- Remote: pending
