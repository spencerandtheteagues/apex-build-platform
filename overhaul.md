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

## Logging Rules

For every completed work item during this overhaul, append:

- date
- short change summary
- files changed
- verification run
- commit hash if pushed

Do not remove old entries. Keep this file as the running implementation log until the overhaul is complete.
