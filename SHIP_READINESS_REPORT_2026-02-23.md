# APEX.BUILD Ship-Readiness Report (2026-02-23)

## Scope of this pass
Hardening/validation pass focused on production-grade parity priorities, with emphasis on preview reliability and session robustness under this sandboxed local environment.

## Fixed issues (by area)

### 1) Preview pane reliability
Files:
- `frontend/src/components/preview/LivePreview.tsx`
- `frontend/src/components/preview/LivePreview.test.tsx`

Fixes:
- Prevented active preview polling from switching to the *settings* sandbox toggle and incorrectly dropping a running preview.
  - Polling now uses the active preview sandbox mode when a preview is running.
- Prevented transient `/preview/status/:id` polling failures from clearing an active preview UI state.
  - Active preview remains visible; connection status degrades to Offline unless backend reports preview missing (404/410).
- Added regression tests for both cases.

### 2) Auth/session robustness
Files:
- `frontend/src/services/api.ts`
- `frontend/src/services/api.test.ts`

Fixes:
- Hardened the 401 response interceptor to skip token-refresh logic for refresh endpoints themselves (`/auth/refresh`, `/auth/token/refresh`).
- This avoids refresh-loop / self-await deadlock scenarios when the refresh endpoint returns 401.
- Added URL-guard regression tests.

## Validation results (this environment)

### Frontend (validated)
Working directory: `frontend/`

- `npm run typecheck` ✅ pass
- `npm run lint` ✅ pass
- `npm run test -- --run` ✅ pass (3 files, 7 tests)
- `npm run build` ✅ pass
  - Warnings only: Vite chunk-size warnings and `"use client"` directive warning from `react-resizable-panels`

### Backend (blocked by environment)
Working directory: `backend/`

- `go test ./...` ❌ blocked (sandbox/network)
  - Cannot fetch modules from `proxy.golang.org` (DNS/UDP denied)
- `go build ./...` ❌ not validated in this pass
  - Initial attempts also hit sandbox cache permissions and process limits; after cache relocation, module fetches were blocked by network policy

### App startup / runtime smoke (blocked by environment)
- `npm run dev -- --host 127.0.0.1 --port 4173` ❌ blocked (`listen EPERM`)
- `npm run preview -- --host 127.0.0.1 --port 4174` ❌ blocked (`listen EPERM`)

## Remaining blockers / gaps (severity + exact files)

### Critical (environment blocker)
1. Backend dependencies cannot be downloaded in sandbox, preventing backend compile/test/runtime validation.
- Impact: blocks end-to-end verification of preview server loop, auth refresh endpoints, project file APIs, deployment integrations.
- Files impacted: `backend/go.mod`, `backend/go.sum` (dependency resolution), plus all backend packages transitively.

2. Local port binding is denied (`listen EPERM`), preventing browser/runtime startup smoke checks.
- Impact: cannot validate frontend runtime boot, iframe preview flows, websocket connectivity, or UI refresh/error states in a running app.
- Files impacted (startup commands): `frontend/package.json` (`dev`, `preview` scripts).

### High (functional validation still pending end-to-end)
3. AI build/code-generation websocket flow not revalidated live in this pass.
- Reason: requires backend/websocket runtime.
- Primary files: `frontend/src/components/builder/AppBuilder.tsx`, backend build/ws handlers under `backend/internal/`.

4. Project create/open/edit/save flows not revalidated against live API in this pass.
- Reason: backend API unavailable in sandbox runtime.
- Primary files: `frontend/src/components/project/ProjectList.tsx`, `frontend/src/components/project/ProjectDashboard.tsx`, `frontend/src/services/api.ts`.

5. Deployment integration points (Render/Vercel/Netlify) not locally testable in this pass.
- Reason: backend runtime + external service credentials/API access required.
- Primary files: `frontend/src/services/api.ts`, backend deploy providers under `backend/internal/deploy/` and `backend/internal/deploy/providers/`.

### Medium
6. Preview pane websocket/browser instrumentation (`postMessage` console/network capture) remains unit-tested only indirectly.
- Primary file: `frontend/src/components/preview/LivePreview.tsx`
- Recommendation: add jsdom tests for `message` events and origin filtering, then validate in browser once port binding is available.

## Replit parity delta (current state after this pass)

Improved:
- Preview resilience in the UI state machine (less likely to drop active preview due to transient status failures or sandbox-setting toggles)
- Auth/session interceptor safety around refresh endpoint failures
- Regression coverage for both fix clusters

Still behind practical Replit parity (blocked/unvalidated here):
- Full preview run/refresh websocket loop in a live browser session
- End-to-end project CRUD + editor save persistence against backend
- Live AI build orchestration and streaming status validation
- Deployment provider execution paths

## Next actions (ordered)
1. Run backend validation in an environment with network access (or pre-warmed module cache): `go test ./...` and `go build ./...`.
2. Run local app startup with port binding enabled and manually validate preview pane flows (start/stop/refresh, URL path, error states, reconnect UX).
3. Add automated browser E2E coverage for preview + project save flows (Playwright/Cypress) once runtime is available.
4. Revalidate AI build websocket events and auth refresh behavior against real backend responses.
5. Exercise deployment provider integration endpoints with mocked providers or sandbox credentials.
