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

## Logging Rules

For every completed work item during this overhaul, append:

- date
- short change summary
- files changed
- verification run
- commit hash if pushed

Do not remove old entries. Keep this file as the running implementation log until the overhaul is complete.
