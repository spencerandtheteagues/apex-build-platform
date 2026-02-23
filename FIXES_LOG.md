
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
