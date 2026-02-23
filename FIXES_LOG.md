
## Session 2026-02-22 - Comprehensive Bug Fix Session

### Bugs Fixed
1. **JWT missing admin flags** - JWTClaims now includes IsAdmin, BypassBilling, HasUnlimitedCredits, BypassRateLimits, IsSuperAdmin
2. **AuthMiddleware (old)** - Updated server.AuthMiddleware() to use ValidateToken and set all bypass flags in context
3. **AuthMiddleware (new)** - Updated RequireAuth middleware to set bypass flags from claims
4. **Quota bypass** - bypassesBilling() now works correctly since context keys are set
5. **File path annotations** - sanitizeFilePath() strips " (root)", " (entry)" etc. from AI-generated paths
6. **Build validation too strict** - Only runs frontend checks for React/Next apps (not backend APIs)
7. **Build validation monorepo** - Accepts frontend/package.json, frontend/index.html, frontend/src/ paths
8. **Build validation Next.js** - Accepts src/app/page.tsx, src/pages/index.tsx as valid entry points
9. **Build validation index.html** - Finds index.html anywhere in tree; skips if vite/webpack config present
10. **Agent prompts** - Frontend agent now always generates index.html, vite.config.ts, package.json, tsconfig.json
11. **Agent prompts** - baseRules now enforces clean file paths (no annotations)
12. **Login** - useStore.ts always sends in username field (backend handles @ detection)
13. **Seed password** - SPENCER_SEED_PASSWORD env var updated to TheStarshipKey

### Result
- React/Vite builds: ✅ Complete with 35 files, no duplicates
- Backend API builds: ✅ No false validation failures  
- Login: ✅ Works with email or username

## Session 2026-02-23 - Frontend Production Realtime/Build Status Fixes

### Bugs Fixed
1. **Collaboration Socket.IO 404 spam** - Stopped auto-connecting optional collaboration socket on login/register, corrected Socket.IO path config (`/ws/socket.io`), and disabled noisy retry loops on unsupported endpoints so Render deployments no longer spam `/socket.io` 404s.
2. **False build failures near completion** - Hardened AppBuilder status normalization and terminal-state precedence so missing/alias statuses (e.g. `running`/`building`) and late progress/state messages cannot overwrite a completed/failed terminal state.
3. **Missing failure detail in Build Activity** - AppBuilder now preserves/extracts backend failure reason (`details`/`error_detail`/`error`) and shows it in the Build Activity panel + terminal messages when a build truly fails.
4. **Regression coverage** - Added builder status helper tests and websocket unsupported-endpoint detection tests.

### Why
- Production frontend was attempting a legacy Socket.IO handshake against a backend realtime endpoint that may not support Socket.IO on that path, causing repeated 404 console noise.
- Build UI could misclassify status due to websocket/payload mismatch and default unknown statuses to `failed`, producing "Failed at 90%" without a corresponding backend error reason.

## Session 2026-02-23 - BYOK Ollama Relay Guidance + Preview UX + Build Quality Guardrails

### Bugs Fixed
1. **Ollama BYOK localhost confusion (frontend UX)** - Added `isLocalOllamaUrl()` detection in `APIKeySettings` and an inline warning banner when users enter `localhost`/`127.0.0.1`/`0.0.0.0`/`::1`, including a collapsible Quick Setup guide for `ngrok` and `cloudflared`.
2. **Ollama BYOK validation error messaging (backend)** - Normalized unreachable/403 Ollama validation errors to return actionable guidance: local Ollama must be exposed via a public URL for cloud-hosted builds.
3. **Preview pane loading/running feedback** - `LivePreview` now tracks iframe loading/error state, shows a loading overlay while the app frame initializes, and shows a clearer frame-load error hint instead of a silent blank pane.
4. **Preview pane layout robustness in IDE split view** - Added `min-h-0`/flex containment to the IDE preview pane wrapper and preview root to prevent flexbox height collapse/hidden preview content in the IDE layout.
5. **Build quality readiness validation** - Fixed `package.json` script validation to require actual runnable frontend scripts (`dev` + `build`, plus `start` for Next.js) instead of passing when any single script exists.
6. **Build quality placeholder detection** - Expanded generated-code validation to catch common placeholder markers (generic `TODO`/`FIXME`, not-implemented variants, and bracketed template placeholders like `[complete file content here]`).
7. **Agent prompt guardrail tightening** - Added an explicit self-check requirement to core agent system rules for runnable entry points, valid package scripts, and zero placeholder text before completion.

### Validation
- `frontend`: `npm run typecheck` ✅
- `frontend`: `npm run lint` ✅
- `frontend`: `npm run test -- --run` ✅ (includes `LivePreview` tests)
- `frontend`: `npm run build` ✅ (Vite chunk-size warnings only)

## Session 2026-02-23 - New User Credits + Clear INSUFFICIENT_CREDITS Build Failures

### Bugs Fixed
1. **New registrations blocked from builds** - `AuthService.CreateUser()` now grants `has_unlimited_credits=true` by default for newly registered users so they can run builds immediately instead of failing with zero credits.
2. **INSUFFICIENT_CREDITS retry loop** - Agent manager now treats `INSUFFICIENT_CREDITS` as non-retriable and prevents recovery/retry loops for the same task.
3. **Build failure UX for credit errors** - Agent task failures now surface a clear user-facing message: `Build paused: Your account has insufficient credits. Please add credits in Settings or contact support.`
4. **AI adapter error propagation** - Agent AI adapter now preserves and propagates a clear `INSUFFICIENT_CREDITS` message through the build error chain to frontend build status consumers.

## Session 2026-02-23 - Build→Project Auto-Linking + IDE/Preview Post-Completion Flow

### Bugs Fixed
1. **Completed builds now auto-link to a project (backend)** - Agent manager now creates a project from generated build files on successful build completion, stores file records, and persists `completed_builds.project_id`.
2. **Duplicate project creation on repeated build opens** - Auto-link helper is idempotent and reuses an existing `completed_builds.project_id` when present instead of creating another project.
3. **Post-build IDE CTA clarity (frontend)** - Completion card now highlights `Open in IDE` as the primary next step with a stronger visual CTA and auto-open status messaging.
4. **Auto-open IDE after build completion (frontend)** - On live `build:completed` events, the builder automatically opens the completed build in IDE (when the tab is still visible) to reduce friction.
5. **Preview auto-start on project switch (frontend)** - `LivePreview` auto-start now checks preview activity for the current project ID so a stale previous-project status does not block preview startup.

### Validation
- `frontend`: `npm run typecheck` ✅
- `frontend`: `npm run lint` ✅
- `frontend`: `npm run build` ✅ (existing Vite warnings only)
- `backend`: `GOCACHE=/tmp/gocache go build -p 1 ./... 2>&1 | head -20` ✅ (no output)

## Session 2026-02-23 - Backend Startup Crash + Render Health Timing + Agent Crash Guards

### Bugs Fixed
1. **Render health check startup grace too short (backend Dockerfile)** - Increased container health check `start-period` to `120s`, `timeout` to `10s`, and `retries` to `5` to avoid premature container kills during cold start.
2. **Backend port bind delayed until after slow init (backend main)** - Added an early bootstrap HTTP listener that binds `PORT` immediately and serves `/health` during startup, then atomically swaps to the full Gin router once initialization completes.
3. **Unsafe auth context type assertion (agent handlers)** - Replaced `userID.(uint)` in build start handler with a safe assertion and `500` response on type mismatch.
4. **Over-aggressive inactivity warnings (agent manager)** - Increased inactivity threshold from `45s` to `120s` and warning count from `3` to `5` to better tolerate slow AI provider responses.
5. **AI generation hang protection (agent adapter)** - Added a default `90s` context timeout around `targetRouter.Generate(...)` when callers do not provide a deadline.
6. **WebSocket subscriber broadcast race (agent manager)** - `broadcast()` now copies subscriber channels under `RLock` before iterating.
7. **Nil task result/output crash guards (agent manager)** - Added guards for nil `TaskResult` and unexpected successful results with nil output.
8. **WebSocket send-channel double close (agent websocket hub)** - Added `sync.Once` protected channel close helper and replaced all direct `close(conn.send)` calls.
9. **Stale builds stuck after restart (agent manager/main)** - Added startup recovery that marks persisted `completed_builds` rows in `in_progress`/`planning`/`building`/`testing`/`reviewing` as `failed` with a restart interruption message.

### Validation
- `backend`: `cd backend && GOCACHE=/tmp/gocache go build -p 1 ./... 2>&1 | head -20` ✅ (no output)
