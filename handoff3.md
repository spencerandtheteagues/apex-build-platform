# Handoff 3

## Purpose

This handoff captures the exact verified state of the repo at the time this file was written, what was already completed and verified before the latest unfinished interaction work began, what new issues were found in the latest audit, what partial implementation work was started but not finished, and what still needs to be completed next.

## Repo State At Handoff

- Branch: `main`
- HEAD when this handoff was prepared: `de15e08`
- The worktree is dirty with many modified files and several untracked files.
- The current uncommitted changes include unfinished backend and frontend interaction-control work and must **not** be pushed as part of any ship commit until they are completed and re-verified.

## What Was Already Completed And Verified Before The Latest Unfinished Work

These items were completed in earlier passes and were already verified locally before the current unfinished interaction/permission work started:

- Runtime API and WebSocket config fixes were shipped so runtime overrides are consumed consistently in deployed environments.
- Expired-auth flow no longer redirects to a dead `/login` path.
- Free signups no longer receive unlimited credits by default.
- Refresh tokens are no longer stored raw; only hashed persistence remains, with migration support.
- Execution records now allow missing `project_id` for snippet execution.
- Project execution was hardened to run the actual workspace command in the container sandbox instead of pretending to run the project while only executing an entry file.
- Go runtime/version inconsistencies were previously aligned in the verified slice that had already been shipped.
- Startup/runtime hardening was improved:
  - stricter production/staging secret validation
  - seed-account fallback made opt-in instead of silent
  - deploy script secret handling improved
  - `/health/features` expanded
  - frontend CSP tightened and bootstrap assets moved out of inline HTML
  - help UI lazy-loaded
- GitHub-facing docs were refreshed in earlier completed passes.

### Previously Verified Green Checks

These passed before the latest unfinished interaction work started:

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `frontend`: `npm run typecheck`
- `frontend`: `npm run lint`
- `frontend`: `npm test -- --run`
- `frontend`: `npm run build`

## New Audit Findings From The Latest Deep Pass

These were the most important additional findings from the latest audit:

1. Main builder chat is mostly advisory and not truly actionable.
   - Backend `SendMessage` / `processUserMessage` produces a lead response, but does not provide robust steering, waiting, permission, or durable conversation semantics.
   - Relevant files:
     - `backend/internal/agents/manager.go`
     - `backend/internal/agents/handlers.go`
     - `frontend/src/components/builder/AppBuilder.tsx`

2. Build activity and chat are not durable enough.
   - `chatMessages` and `aiThoughts` are mostly local UI state.
   - Build restore/hydration does not reliably repopulate interaction state.

3. User messages can fail silently in the builder.
   - The frontend appends local chat immediately, but only sends through the websocket if the socket is open.
   - There is no proper queue, ack, or failure state.

4. Review-required flow is incomplete.
   - The builder can enter an `awaiting_review` style state, but the diff review UI is not properly wired into that flow.
   - `DiffReviewPanel.tsx` exists, but the main builder loop does not fully integrate it.

5. Main build websocket leaks a goroutine per connection.
   - The forwarder goroutine reading `updateChan` can remain stuck after unsubscribe because the channel is not closed and there is no forwarder stop signal.
   - Relevant file:
     - `backend/internal/agents/websocket.go`

6. Main build websocket exposes `build:start`, which can duplicate orchestration.
   - `StartBuild` already launches builds asynchronously.
   - Websocket-triggered `build:start` is a duplicate path and should be removed or made safely idempotent.

7. Autonomous websocket control path is too permissive.
   - The autonomous agent websocket path was identified as too trusting.
   - It needs real user/auth checks and origin validation.

8. Autonomous pause/resume loses the prior state.
   - Resume behavior returns to a generic executing state instead of restoring the paused-from state like validating/planning/executing.

## Frontend Build UX Audit Summary

The frontend build experience still needs real-time steering and visibility improvements:

- stale or empty activity after build restore
- silent message failure when websocket is unavailable
- weak or under-modeled waiting/action-required states
- `awaiting_review` dead end in the builder flow
- task timeline is not exposed clearly enough to the user
- old `AgentPanel` / `agentApi` code appears stale relative to the live backend contract

## Partial Implementation Started But Not Finished

The following work was started in this session, but it was **not** fully completed, verified, or made safe to push.

### 1. Backend interaction state types

File:

- `backend/internal/agents/types.go`

Started additions included:

- `BuildConversationRole`
- `BuildConversationKind`
- `BuildPermissionScope`
- `BuildPermissionDecision`
- `BuildPermissionMode`
- `BuildPermissionRequestStatus`
- `BuildConversationMessage`
- `BuildPermissionRule`
- `BuildPermissionRequest`
- `BuildInteractionState`

Started model extension:

- `Build.Interaction BuildInteractionState`

Started websocket message types:

- `build:interaction`
- `build:user-input-required`
- `build:user-input-resolved`
- `build:permission-request`
- `build:permission-update`

### 2. New interaction helper file

File:

- `backend/internal/agents/interaction.go` (untracked)

This file was started to hold:

- interaction-state copying helpers
- conversation append/persist helpers
- steering notes
- permission rule/request normalization
- wait-state resolution
- lead JSON-plan parsing
- prompt-context building for interaction-aware task prompting
- manager helpers such as:
  - `GetBuildInteraction`
  - `GetBuildMessages`
  - `broadcastInteractionUpdate`
  - `PauseBuild`
  - `ResumeBuild`
  - `SetPermissionRule`
  - `ResolvePermissionRequest`
  - `enqueueUserRevisionTask`
  - `waitForBuildInteractionClear`

This file exists locally but is unfinished and unverified.

### 3. Persisting interaction state in completed builds

File:

- `backend/pkg/models/models.go`

Started addition:

- `InteractionJSON string` on `CompletedBuild`

Migration files started:

- `backend/migrations/000008_build_interactions.up.sql`
- `backend/migrations/000008_build_interactions.down.sql`

These were intended to add/drop `interaction_json` on `completed_builds`.

### 4. Agent manager changes

File:

- `backend/internal/agents/manager.go`

Started work included:

- refactoring `SendMessage` to delegate to `SendMessageWithClientToken`
- appending and persisting interaction messages
- clearing wait state when the user replies
- broadcasting interaction updates
- structured JSON prompting for lead responses
- interpreting lead output into:
  - steering notes
  - wait-state questions
  - permission requests
  - revision task enqueueing for terminal builds
- making `executeTask` wait on interaction state before running AI work
- injecting interaction context into task prompts
- making completion/wait logic aware of paused and waiting builds
- persisting `interaction_json` in build snapshots

This file is mid-edit and must be treated as unverified.

### 5. Agent handlers changes

File:

- `backend/internal/agents/handlers.go`

Started work included:

- adding `interaction` to live and snapshot build responses
- adding `messages` to details/completed-build responses
- allowing `client_token` on `SendMessage`
- adding new handlers:
  - `GetMessages`
  - `GetPermissions`
  - `SetPermissionRule`
  - `ResolvePermissionRequest`
  - `PauseBuild`
  - `ResumeBuild`
- started route registration for:
  - `GET /build/:id/messages`
  - `GET /build/:id/permissions`
  - `POST /build/:id/permissions/rules`
  - `POST /build/:id/permissions/requests/:requestId/resolve`
  - `POST /build/:id/pause`
  - `POST /build/:id/resume`
- added `parseBuildInteraction(...)`

This file is mid-edit and must be re-read carefully before continuing.

### 6. Build websocket changes

File:

- `backend/internal/agents/websocket.go`

This work was started but not finished. Intended changes included:

- adding a forwarder stop signal to avoid the leaked goroutine
- changing `readPump(updateChan)` to accept a stop signal
- removing `build:start`
- accepting both legacy and current inbound message shapes:
  - `user:message`
  - `user_message`
  - `command` with `data.command`
  - `pause`
  - `resume`
  - `build:pause`
  - `build:resume`
- including `messages` and `interaction` in initial `build:state`
- possibly adding helper parsing utilities

This file is modified locally but incomplete.

## Still Needs To Be Completed

### Highest Priority

1. Do not push the current unfinished interaction-control code as a ship commit.
2. Finish and stabilize the backend interaction implementation.
3. Finish and stabilize the frontend builder interaction/visibility implementation.
4. Re-run the full verification suite only after the code is made coherent again.

### Backend Work Still Needed

- Complete `backend/internal/agents/websocket.go`
  - fix the goroutine leak
  - remove or harden duplicate `build:start`
  - make message parsing backward-compatible and explicit
  - include durable interaction state in initial websocket state
- Patch autonomous websocket auth/origin handling
  - use real websocket auth helpers
  - reject unauthorized control connections
  - enforce origin policy
- Fix autonomous pause/resume to restore the prior state instead of always returning to executing
- Re-read and finish:
  - `backend/internal/agents/types.go`
  - `backend/internal/agents/interaction.go`
  - `backend/internal/agents/manager.go`
  - `backend/internal/agents/handlers.go`
  - `backend/internal/agents/websocket.go`
- Ensure interaction persistence is coherent and migration-safe

### Frontend Work Still Needed

- Patch `frontend/src/services/api.ts`
  - add build-message retrieval
  - add permission retrieval and mutation
  - add pause/resume endpoints
  - add reliable build-message send with client token support
- Patch `frontend/src/components/builder/AppBuilder.tsx`
  - clear stale local-only chat/thought state on hydrate
  - hydrate server-provided `messages` and `interaction`
  - render real paused/waiting/action-required states
  - add pause/resume controls
  - add pending permission request UI
  - model local-resource permission options like a CLI:
    - allow once
    - allow for build
    - deny
  - make user message delivery reliable with pending/sent/failed state
  - reconnect live builds in review/pause/wait states
  - wire in `DiffReviewPanel` for review-required flows
  - expose tasks as a clearer real-time timeline
- Evaluate whether stale `agentApi` / `AgentPanel` code should be updated or retired

### Test Work Still Needed

After the unfinished implementation is completed, run:

- `gofmt` on all changed Go files
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `frontend`: `npm run typecheck`
- `frontend`: `npm run lint`
- `frontend`: `npm test -- --run`
- `frontend`: `npm run build`

Recommended additional tests:

- backend handler tests for:
  - `SendMessage` with `client_token`
  - build message retrieval
  - permission rule set/resolve
  - pause/resume endpoints
- websocket parsing tests if helper extraction is added
- autonomous websocket auth tests
- frontend tests for build interaction normalization and restore behavior

## Current Dirty Worktree Snapshot

At the time of this handoff, the worktree includes many modified files and these notable untracked files:

- `OLLAMA_PIPELINE_TEST_INSTRUCTIONS.md`
- `backend/internal/agents/interaction.go`
- `backend/internal/collaboration/project_access.go`
- `backend/internal/collaboration/project_access_test.go`
- `backend/internal/handlers/collaboration.go`
- `backend/internal/handlers/collaboration_test.go`
- `backend/internal/handlers/websocket_auth.go`
- `backend/migrations/000008_build_interactions.down.sql`
- `backend/migrations/000008_build_interactions.up.sql`

These were left intentionally uncommitted at the time this handoff was created.

## Safe Next Step

The next AI should first read the current modified files carefully, decide whether to salvage or back out the unfinished interaction work, and only then continue implementation. Nothing in the current unfinished interaction-control slice should be treated as verified.
