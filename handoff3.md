# Handoff 3

## Current Status

- Branch: `main`
- Last pushed commit on GitHub before this local work: `21e9509` (`docs: add handoff3 status`)
- This handoff file has now been updated to reflect the latest completed local work
- The current interaction-control / user-steering slice is now materially implemented and verified locally
- The current changes are **local only** unless explicitly pushed later

## Render Deploy Follow-Up

The backend Render deployment path was investigated directly with a production-like
container boot against PostgreSQL. That uncovered multiple real startup blockers,
not just one:

1. `backend/cmd/main.go`
   - startup treated missing Docker as too close to fatal when `EXECUTION_FORCE_CONTAINER=true`
   - this is wrong for Render, because web services do not provide a Docker daemon
   - fixed so code execution degrades cleanly while the core API still reaches readiness

2. `backend/internal/config/secrets.go`
   and `backend/internal/secrets/secrets.go`
   - production secret validation previously required `SECRETS_MASTER_KEY` to be base64-encoded 32 bytes
   - Render-style generated secrets are typically strong raw strings, not base64 material
   - fixed so strong raw secrets are accepted and deterministically derived to 32-byte key material, while existing base64 keys still work unchanged

3. `backend/Dockerfile`
   and `backend/Dockerfile.production`
   - production images previously copied only the binary, not the SQL migrations
   - startup therefore failed in production with `migrations directory not found`
   - fixed by copying `migrations/` into the runtime images

4. `backend/migrations/000001_*`
   and `backend/migrations/000002_*`
   - the baseline SQL migrations used `CREATE INDEX CONCURRENTLY` / `DROP INDEX CONCURRENTLY`
   - golang-migrate runs those files inside a transaction, so fresh PostgreSQL boots failed immediately
   - fixed by making those index statements transaction-safe for first-run production deployments

5. `frontend/src/components/ide/IDELayout.tsx`
   and `frontend/src/components/preview/LivePreview.tsx`
   - secure preview refresh now falls back from hot-reload to full refresh
   - iframe sandbox no longer grants `allow-same-origin`, matching the hardened preview model

## What Was Completed In This Pass

### Backend: build interaction, websocket, and permission control

- Build websocket auth/origin handling is now aligned with the shared websocket auth/origin helpers instead of using a separate ad hoc path.
- Autonomous websocket control no longer skips auth in development-style fashion.
- Build websocket no longer accepts `build:start` as an active orchestration path. Deprecated commands are ignored instead of duplicating build launches.
- Build websocket forwarder leak was fixed by stopping the forwarding goroutine when the connection closes.
- Build websocket now accepts a wider set of inbound control/message shapes for compatibility:
  - `user:message`
  - `user_message`
  - `pause`
  - `resume`
  - `build:pause`
  - `build:resume`
  - `command` with `data.command`
- Initial `build:state` websocket payload now includes durable conversation `messages` and `interaction`.
- Build interaction permission rules were hardened:
  - deny rules are now surfaced into prompt context
  - allow-once rules are consumed instead of silently persisting forever
  - deny rules can suppress repeated asks
  - permission resolutions now distinguish `once` vs `build`
- Snapshot-backed `/build/:id/permissions` now works instead of only live-build lookup.
- `awaiting_review` is now treated as an active/resumable build status in the backend.

### Backend: autonomous agent correctness

- Autonomous pause/resume now restores the task to the paused-from state instead of always resuming to `executing`.
- Autonomous websocket initial state now exposes `paused_from`.
- Autonomous logging was hardened to avoid panicking on short task IDs.

### Frontend: user steering, visibility, and review flow

- `AppBuilder` now hydrates persisted build `messages` and `interaction` instead of only relying on local ephemeral state.
- Stale chat/activity is cleared on restore instead of carrying forward unrelated local UI residue.
- User build messages are now sent through the API with `client_token` support instead of depending on an open websocket.
- Optimistic user messages now reconcile correctly and show `pending` / `failed` states instead of failing silently.
- Pause / resume controls were added to the builder UI.
- Pending permission requests now render in the builder UI with actions:
  - allow once
  - allow for build
  - deny
- Pre-approval controls were added for common local resources:
  - Docker
  - Git
  - localhost network access
- The builder now shows a task timeline so users can see what the AI team is doing in real time.
- `awaiting_review` flow is now integrated with `DiffReviewPanel`, including reload and reopen behavior.
- Follow-up iteration after a completed/failed build now works correctly in the UI:
  - if the user asks for another pass, the frontend will leave terminal state and show the resumed build instead of staying visually stuck at `completed`
- Websocket message handling now uses a ref-backed live handler instead of a stale closure-prone path.

### Frontend: bundle hygiene

- `DiffReviewPanel` now lazy-loads `DiffViewer` so the new review flow does not statically pin that IDE diff module into the builder path.

## Tests Added In This Pass

- `backend/internal/agents/interaction_test.go`
  - verifies allow-once permission rules are consumed
  - verifies denied permissions appear in prompt context
- `backend/internal/agents/autonomous/agent_test.go`
  - verifies autonomous pause/resume restores the paused-from state

## Key Additional Bug Found And Fixed During This Pass

While adding regression coverage, one unrelated but real bug was discovered and fixed:

- autonomous task logging could panic on short task IDs because it sliced `task.ID[:8]` unconditionally

That is now fixed.

## Verified Locally After The Latest Changes

### Backend

- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- Render-like cold boot:
  - backend Docker image built from `backend/Dockerfile`
  - PostgreSQL container started
  - backend container started with:
    - `ENVIRONMENT=production`
    - `EXECUTION_FORCE_CONTAINER=true`
    - raw Render-style `SECRETS_MASTER_KEY`
    - no Docker socket available
  - `/ready` reached HTTP 200 with `ready=true`
  - startup correctly reported optional degraded features like `code_execution`, `preview_service`, `payments`, and `redis_cache` instead of crashing

### Frontend

- `npm run typecheck`
- `npm run lint`
- `npm test -- --run`
- `npm run build`
- `docker build -f frontend/Dockerfile.prod -t apex-build-frontend-prod-test frontend`

### Backend Containers

- `docker build -f backend/Dockerfile.production -t apex-build-backend-prod-test backend`

## Important Behavioral Improvement

One subtle orchestration bug was fixed late in the pass:

- when a build was waiting for user input, a user reply previously cleared the waiting state **before** the lead agent had processed the reply
- that could let work resume one cycle too early and miss the new instruction
- now the build stays blocked until the lead agent processes the reply and resolves the interaction state

## Remaining Work Worth Tracking

These are the notable follow-ups that still remain after this pass:

1. Push strategy
   - The current interaction-control changes are local only unless explicitly committed and pushed.

2. Stale agent UI code
   - `frontend/src/services/agentApi.ts`
   - `frontend/src/components/agent/AgentPanel.tsx`
   - These still look stale / unused relative to the now-active builder interaction path.
   - They are not currently the primary user path, but should either be updated to match the live contract or retired.

3. Frontend bundle size
   - Monaco and `ts.worker` remain the dominant production bundle warnings.
   - This is still a real optimization target, but not a correctness blocker for the interaction-control feature set.

4. Broader launch audit
   - The current pass focused heavily on the unfinished builder interaction / permission / visibility slice.
   - If another deep launch-readiness pass happens, the next likely targets are:
     - stale/unused frontend surfaces
     - remaining CSP/perf hardening
     - any additional public-execution abuse controls beyond the already-completed execution hardening

## Current Files Touched In This Pass

- `backend/internal/handlers/websocket_auth.go`
- `backend/internal/agents/interaction.go`
- `backend/internal/agents/manager.go`
- `backend/internal/agents/websocket.go`
- `backend/internal/agents/handlers.go`
- `backend/internal/agents/autonomous/agent.go`
- `backend/internal/agents/autonomous/handlers.go`
- `backend/internal/agents/interaction_test.go`
- `backend/internal/agents/autonomous/agent_test.go`
- `frontend/src/services/api.ts`
- `frontend/src/components/builder/AppBuilder.tsx`
- `frontend/src/components/diff/DiffReviewPanel.tsx`
- `backend/cmd/main.go`
- `backend/Dockerfile`
- `backend/Dockerfile.production`
- `backend/internal/config/secrets.go`
- `backend/internal/config/secrets_test.go`
- `backend/internal/secrets/secrets.go`
- `backend/internal/secrets/secrets_test.go`
- `backend/migrations/000001_initial_schema.up.sql`
- `backend/migrations/000001_initial_schema.down.sql`
- `backend/migrations/000002_hosting_and_recent_features.up.sql`
- `backend/migrations/000002_hosting_and_recent_features.down.sql`
- `frontend/src/components/ide/IDELayout.tsx`
- `frontend/src/components/preview/LivePreview.tsx`

## Important Context For The Next AI

- The earlier version of this handoff described the interaction-control work as unfinished WIP.
- That is no longer accurate.
- The builder interaction / permission / pause-resume / diff-review path is now substantially wired through and verified locally.
- The next AI should treat this handoff as the current truth, not the older “unfinished” snapshot.
