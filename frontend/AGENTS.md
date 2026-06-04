# Frontend Agent Contract

## Purpose

`frontend/` owns the Apex Build customer experience: onboarding, prompt-to-app builder, IDE, file/project management, preview surfaces, billing/spend/BYOK settings, deployment UI, mobile surfaces, admin views, and visual polish.

The frontend must make Apex Build feel more reliable, transparent, and capable than Replit. UI must not hide uncertainty: build state, provider state, preview state, billing state, and recovery actions must be clear and honest.

## Documentation Hierarchy

This file is the level 1 frontend contract. Add child `AGENTS.md` files when a feature folder gains stable rules that should not live in this broad doc. Likely future child docs include:

- `frontend/src/components/builder/AGENTS.md` for prompt entry, onboarding, build progress, power modes, and first-run flows.
- `frontend/src/components/ide/AGENTS.md` for editor, panels, terminal, review, and preview integration.
- `frontend/src/components/preview/AGENTS.md` for iframe behavior, console/network capture, runtime verification, and placeholder handling.
- `frontend/src/components/billing/AGENTS.md` for plans, credits, checkout, billing portal, and trust UX.
- `frontend/src/store/AGENTS.md` for Zustand state ownership and cross-surface updates.
- `frontend/src/services/AGENTS.md` for API/WebSocket clients and runtime config.

When child docs exist, list them here and keep parent/child contracts synchronized.

## Owned Files And Surfaces

- `src/App.tsx`: top-level application shell and route/view composition.
- `src/components/builder/`: prompt input, onboarding, template/gallery starters, build progress, power modes, and generated app entry flow.
- `src/components/ide/`: editor workspace, panels, terminal, diff/review, project controls, and preview integration.
- `src/components/preview/`: preview iframe, runtime status, backend preview server state, console/network evidence, and recovery UI.
- `src/components/billing/`, `src/components/spend/`, `src/components/budget/`, `src/components/secrets/`, `src/components/usage/`: paid plan, credits, BYOK, usage, and transparency surfaces.
- `src/services/`: API client, WebSocket client, provider/client utilities, and runtime config consumption.
- `src/hooks/`, `src/store/`, `src/types/`, `src/lib/`: shared state, hooks, types, and utilities.
- `src/components/ui/` and `src/styles/`: reusable visual primitives and global styling.

## Stable Contracts

- Use React + TypeScript + Vite patterns already present in the repo. Prefer colocated `*.test.tsx` tests for component behavior.
- Runtime API and WebSocket URLs must remain configurable through `/config.js` and documented Vite env fallbacks.
- Build UI must reflect the backend source of truth. Do not invent completed, passed, or deployable states in the client.
- First-run onboarding must push users toward a successful first build with realistic starter prompts and a clear blank-workspace escape hatch.
- Generated app previews must be inspectable, interactive, and visually complete. Frontend UI should surface preview verification failures instead of presenting broken iframes as success.
- Collaboration and presence surfaces are Team/Enterprise/Owner plan features. Lower-tier sessions must not join collaboration rooms or send cursor/edit operations just because an editor component is mounted.
- Billing and usage UI must derive plans, limits, credits, and cost status from backend contracts. Avoid hardcoded pricing or stale launch-special copy.
- Shared state updates must avoid empty IDs, duplicate build records, stale progress, or cross-user leakage.
- UI must be responsive, accessible enough for keyboard/screen-reader basics, and text must fit within controls across supported viewport sizes.
- Do not add marketing-only surfaces where a working product surface is expected. Public pages can market; in-app builder/IDE screens should prioritize action, evidence, and recovery.

## UI Quality Bar

- The first viewport after login should make it obvious how to build, inspect, preview, deploy/export, and manage costs.
- Buttons should use clear iconography from existing icon libraries when available. Do not invent one-off visual systems when shared UI primitives already cover the need.
- Avoid placeholder content in production UI unless clearly labeled as sample data or an intentional empty state.
- Use compact, work-focused layouts for builder, IDE, admin, billing, and operational surfaces.
- Keep loading, empty, error, retry, and degraded states complete for launch-critical flows.
- Do not let status badges, progress bars, toasts, or modals contradict backend state.

## Development Guidance

- Keep components focused. Split only when it improves readability, reuse, or testability.
- Prefer explicit props and typed service responses over implicit global coupling.
- Keep API call behavior in services or existing hooks rather than scattering fetch logic across UI components.
- When WebSocket payloads change, update the backend producer, frontend consumer, types, and tests together.
- When adding prompts or starters, make them specific enough to generate useful, verifiable apps and honest about frontend-only vs full-stack scope.
- Avoid broad visual rewrites unless the task is explicitly a redesign or launch-polish pass with verification screenshots.

## Verification

For frontend changes, run focused tests first, then broaden based on risk:

```bash
cd frontend
npm run typecheck
npm run test -- --run
npm run lint
npm run build
```

For builder, preview, onboarding, or IDE changes, also run or update relevant Vitest coverage and consider Playwright preview verification under `tests/e2e/`.

## Documentation Updates

Update this file when frontend architecture, state ownership, build/preview UX, onboarding rules, billing/spend UI contracts, service boundaries, visual primitives, or verification expectations change. Update root `AGENTS.md` when the change affects the overall launch bar or cross-domain behavior.

## Child Docs

- `src/components/builder/AGENTS.md`: builder workflow, first-run onboarding, build progress, quality gates, and generated-app truth surfaces.
