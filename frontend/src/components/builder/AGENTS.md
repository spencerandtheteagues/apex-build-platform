# Builder Agent Contract

## Purpose

`frontend/src/components/builder/` owns the prompt-to-app customer workflow: first-run onboarding, starter prompts, build submission, build progress, agent activity, quality-gate state, recovery controls, and the handoff into preview, IDE, export, and deployment surfaces.

This is the highest-leverage frontend surface for launch. It must help a new user create a successful first app without hiding build, provider, preview, billing, or verification risk.

## Owned Files And Surfaces

- `AppBuilder.tsx`: builder shell, prompt state, build start, WebSocket/status hydration, recovery, and top-level build UI composition.
- `BuildScreen.tsx`: active build display, agent activity, progress, blockers, quality-gate state, and build interaction controls.
- `OnboardingTour.tsx` and `onboardingStarters.ts`: first-run tour, starter prompts, starter selection, and blank-workspace escape hatch.
- `TemplateGallery.tsx`: reusable prompts and build starting points beyond first-run onboarding.
- `OrchestrationOverview.tsx`, `LiveActivityFeed.tsx`, and related tests: orchestration truth, verification reports, failure fingerprints, repair signals, and readiness promotion evidence.
- `buildRestore.ts`: persisted/rehydrated build state normalization and terminal-status precedence.

## Stable Contracts

- Starter prompts must be specific enough to produce useful, verifiable apps on the first pass. Avoid generic requests that naturally produce placeholder dashboards.
- Frontend-only starters must say so clearly. Do not imply backend, auth, database, payments, or server runtime when the starter is intended for fast frontend builds.
- Builder UI must not invent success. Terminal state, progress, `quality_gate_status`, `quality_gate_passed`, blockers, and verification reports must reflect backend truth.
- Completed builds must preserve quality-gate and verification evidence when restored from status/detail endpoints.
- Preview and placeholder failures are launch-blocking signals. Surface them as repair/verification issues instead of letting a green progress state hide them.
- Preview-specific warning banners must be tied to preview-relevant quality-gate stages or verification reports. Do not label backend-only or billing-only failures as preview launch blockers.
- Onboarding may guide users toward starter prompts, but it must preserve a blank-workspace escape hatch for users who want the IDE directly.
- Blank-workspace actions must pass an explicit `projectId: null` intent through the app shell and land on `/ide`; they must not silently reopen the active or last project.
- Dismissing onboarding should be deterministic and test-covered. `forceShow` must override stored completion for support, demos, and regression tests.
- Onboarding actions should describe what they actually do. Starter cards may prefill a build prompt; blank-workspace actions may open the IDE; a generic continue action must not imply that a build has already started.
- Build controls must remain usable across mobile and desktop viewports. Text must fit inside controls and progress/status elements must not shift layout unexpectedly.

## Development Guidance

- Keep starter prompt data separate from modal rendering so tests can assert prompt semantics without brittle DOM traversal.
- Prefer small helper functions for status normalization and restoration behavior; keep them covered in colocated tests.
- When adding build events or WebSocket fields, update service types, `AppBuilder.tsx`, `buildRestore.ts`, and tests together.
- When changing copy that affects launch promises, align it with backend verification reality and pricing/billing contracts.
- Do not refactor the whole builder during a narrow onboarding or quality-gate fix. The file is large; scoped patches are safer.

## Verification

Run focused tests for the touched builder surface first:

```bash
cd frontend
npm run test -- --run src/components/builder/OnboardingTour.test.tsx
npm run test -- --run src/components/builder/buildRestore.test.ts
npm run test -- --run src/components/builder/BuildScreen.test.tsx
npm run test -- --run src/components/builder/OrchestrationOverview.test.tsx
```

Then run broader gates when changes affect shared build state or launch-critical UX:

```bash
cd frontend
npm run typecheck
npm run lint
npm run build
```

## Documentation Updates

Update this file when builder onboarding, starter prompts, build state semantics, quality-gate display, repair/recovery behavior, or generated-app truth surfaces change. Update `frontend/AGENTS.md` when ownership moves outside this folder, and root `AGENTS.md` when the launch bar or cross-domain contract changes.
