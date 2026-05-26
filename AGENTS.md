# Apex Build Agent Contract

## Documentation First

Documentation is part of the Apex Build runtime contract. Treat every `AGENTS.md` file as binding instruction for agentic work, not as optional contributor notes. Bad documentation creates agent drift, architecture drift, duplicated fixes, and production behavior that no one can safely operate.

Always read the closest owning `AGENTS.md` before editing code in that subtree. Always update the relevant docs in the same session as the code change when a stable contract, workflow, ownership boundary, verification gate, or agent-facing behavior changes.

The current documentation hierarchy is:

- `/AGENTS.md`: repo-wide mission, launch bar, documentation policy, architecture boundaries, and cross-cutting engineering rules.
- `/backend/AGENTS.md`: Go API, orchestration, billing, execution, preview, deploy, auth, persistence, provider, and WebSocket contracts.
- `/frontend/AGENTS.md`: React/Vite application, builder, IDE, preview, onboarding, billing/spend, state, and UI quality contracts.
- `/frontend/src/components/builder/AGENTS.md`: prompt-to-app builder, first-run onboarding, build progress, quality gates, and generated-app truth surfaces.
- `/scripts/AGENTS.md`: local, production, canary, Render, Stripe, mobile, and reliability verification script contracts.
- `/tests/AGENTS.md`: repo-level, backend, frontend, and Playwright test harness contracts.
- `/docs/AGENTS.md`: public docs, launch trackers, architecture docs, roadmap docs, and narrative documentation contracts.
- `/.github/AGENTS.md`: CI, deploy, production canary, and nightly workflow contracts.

If a subtree gains enough stable behavior that root or core docs become too broad, add a deeper `AGENTS.md` before or alongside the code change. The closer a doc is to code, the more concrete it should be. The higher a doc is in the tree, the more it should focus on ownership, stable contracts, boundaries, and policies.

### AGENTS File Index

Keep this index exhaustive. Update it whenever an `AGENTS.md` file is added, removed, moved, or renamed.

- `/AGENTS.md`
- `/.github/AGENTS.md`
- `/backend/AGENTS.md`
- `/docs/AGENTS.md`
- `/frontend/AGENTS.md`
- `/frontend/src/components/builder/AGENTS.md`
- `/scripts/AGENTS.md`
- `/tests/AGENTS.md`

### Documentation Update Rules

When changing code, update docs as follows:

- Update the closest owning `AGENTS.md` for stable architecture, workflow, API, state, style, prompt, provider, deployment, or verification behavior changes.
- Update parent docs when the higher-level contract, ownership boundary, launch bar, or workflow changed.
- Update `README.md` only for public product pitch, quick starts, links, public feature maps, and high-level documentation discovery.
- Do not let `README.md` become a competing implementation contract. Durable architecture and workflow rules belong in `AGENTS.md`; broader narrative belongs under `docs/`.
- Update `docs/launch-readiness-tracker.md`, `LAUNCH_WAR_PLAN.md`, or relevant launch docs when evidence, blockers, canary status, rollback status, provider status, or production readiness changes.
- Remove stale or contradictory documentation immediately. Do not leave two sources claiming different launch, pricing, provider, preview, or billing behavior.
- Never store secrets, provider transcripts, customer data, paid checkout details, API keys, tokens, cookies, or private production payloads in docs.

### Documentation Depth Model

- Level 0 repo doc: mission, launch quality bar, cross-cutting rules, top-level architecture, and documentation policy.
- Level 1 domain docs: `backend/`, `frontend/`, `scripts/`, `tests/`, `docs/`, and `.github/` architecture plus child-doc guidance.
- Level 2 subsystem docs: one major surface such as builder, preview, billing, execution, provider routing, or E2E harness.
- Level 3 leaf docs: one concrete feature, service, prompt family, handler group, workflow, or UI surface with exact implementation guidance.
- If a level 2 or level 3 doc later gains child docs, add a `Documentation Hierarchy` section before those child docs land.

### Documentation Shape Rules

Sibling docs at the same depth should use the same section order unless a domain-specific contract truly needs one extra section. Every `AGENTS.md` should answer, in this order when practical: what this scope owns, which files or surfaces it owns, which stable contracts it enforces, how child docs divide remaining detail, and what changes require doc updates.

Parent docs explain boundaries, ownership maps, and stable seams. Child docs explain concrete file-level behavior, state, styles, assets, API usage, prompts, tests, and failure handling.

## Product Mission And Launch Bar

Apex Build is an AI app-building platform for shipping real, working web and mobile products from prompts. The product target is to be at least 10% better than Replit across the categories customers notice: onboarding, prompt-to-app success, build reliability, preview fidelity, IDE capability, deployment/export, cost transparency, BYOK/provider choice, collaboration/community, billing trust, and supportability.

The operational launch bar is intentionally strict:

- Every customer-facing build flow must either complete to a real usable result or fail clearly with recoverable guidance and no false success state.
- App-building prompts must strive for first-pass completion, functional generated code, and a perfect interactive preview. If that cannot be proven, document the gap as a launch blocker instead of weakening the claim.
- Generated previews must be real, styled, interactive, and representative of the requested app. Placeholder-only dashboards, generic KPI cards, skeleton-only screens, fake backend claims, or empty app shells must fail verification.
- Billing, credits, provider spend, and BYOK behavior must be exact, transparent, and safe before paid traffic scales.
- Launch-readiness evidence beats optimism. Treat canary output, tests, Render health, Stripe verification, preview screenshots, and logs as the source of truth.

Do not fake readiness. Do not hide provider, preview, billing, or build-quality failures behind successful UI states. Do not claim a feature is public-launch ready unless the relevant verification has passed and the evidence is recorded in the owning launch doc.

## Top-Level Structure

- `backend/`: Go API and infrastructure runtime. Owns auth, billing, orchestration, AI provider routing, build lifecycle, preview/execution, WebSockets, deploy/hosting, persistence, migrations, startup checks, and operational health.
- `frontend/`: Vite + React + TypeScript client. Owns onboarding, builder UX, IDE, preview surfaces, file/project UI, billing/spend/BYOK settings, admin surfaces, and customer-facing product quality.
- `scripts/`: verification and operations automation for local checks, production canaries, Render, Stripe, mobile external readiness, live golden builds, and reliability matrices.
- `tests/`: repo-level and Playwright harnesses, generated app specs, SLO thresholds, smoke tests, and launch/preview verification.
- `docs/`: architecture, launch readiness, deployment, API, pricing, roadmap, handoff, and evidence documentation.
- `.github/workflows/`: opt-in CI, deploy, production canary, and reliability-nightly workflows.
- `render.yaml`, `docker-compose*.yml`, `deploy.sh`, `start.sh`: deployment and runtime entry surfaces.
- `prompts/`: golden and canary prompt fixtures used for launch-quality evidence.

## Stable Runtime Contracts

- `backend/api/openapi.yaml` is the detailed API contract. Keep it aligned with handler behavior.
- `API_CONTRACT.md` and `docs/api.md` are human-facing API maps. Update them when external behavior changes.
- Build lifecycle state must reach honest terminal states. A build must not report `completed` or `quality_gate_status=passed` until generated files, preview startup, and quality gates are actually satisfied.
- Preview verification is a launch gate, not decoration. Keep placeholder detection, HTTP timeout bounds, screenshot capture, console checks, and quality gate state aligned.
- Provider routing must prefer reliable completion and bounded cost. Never route to a cheaper provider in a way that increases hang risk, silent underbuilding, or fake success.
- Cost transparency, spend ledgers, credits, and billing plans must derive from shared pricing/payment contracts, not duplicated frontend literals.
- Production launch configuration must be verifiable without printing secret values.
- Runtime config for the frontend must remain environment-adaptable through `/config.js` and documented `VITE_API_URL` / `VITE_WS_URL` behavior.

## Engineering Rules

- Keep implementations lean. Prefer simplification and refactoring over adding bloat.
- Do not repeat code unnecessarily. Extract shared implementations only when reuse is real and the abstraction clarifies behavior.
- Prefer explicit contracts, stable module boundaries, and deterministic discovery over ad hoc cross-dependencies.
- Keep launch-critical behavior boring, observable, and testable.
- Use Go modules and `gofmt` in `backend/`; use ES modules, React, TypeScript, and existing Vite patterns in `frontend/`.
- Do not introduce compatibility shims, mirrored code paths, or wrapper layers unless the user explicitly asks for compatibility support.
- Do not create new tracked scratch, temporary, generated, or hidden work directories. Keep local verification artifacts in ignored or external paths such as `/tmp` unless a durable fixture is intentionally required.
- Do not commit generated binaries, release outputs, provider logs, screenshots, or large artifacts into ad hoc repo paths.
- Treat auth, billing, provider keys, sandbox execution, deployment, file mutation, and customer project data as high-risk areas. Changes there need focused tests and launch-doc updates.
- Never commit real secrets, `.env` files with values, API keys, provider tokens, Stripe secrets, Render secrets, cookies, customer prompts, or private customer app contents.

## Working With Existing Changes

The worktree may contain user or helper-agent changes. Do not revert changes you did not make unless the user explicitly asks. If existing changes touch your target files, read them first and preserve their intent. If a helper agent left partial work, verify it through tests and code review before building on it.

Use Hermes, opencode, Claude, Codex, or other helpers as reviewers and parallel investigators when they are available, but the final repo state must be verified locally. Helper output is not a substitute for code reads, tests, and launch evidence.

## Launch Evidence Sources

Before non-trivial launch work, read the relevant current evidence:

- `LAUNCH_WAR_PLAN.md`: current launch blockers, priorities, agent assignments, and canary status.
- `docs/launch-readiness-tracker.md`: reconciled readiness evidence and remaining blockers.
- `docs/launch-runbook.md`: operational launch, rollback, incident, and support procedures.
- `docs/builder-hardening-plan.md`: builder reliability and quality-gate work.
- `docs/replit-overtake-roadmap.md`: category strategy against Replit.
- `docs/canary-reliability-handoff.md`: canary and reliability handoff notes.
- `FUTURE.md`: forward-looking pathway notes. Keep it evidence-based.

## Standard Verification Gates

Use the smallest verification that covers the change, then broaden for shared or launch-critical behavior.

Backend:

```bash
cd backend
go test -p 1 -parallel 4 ./... -timeout 20m
go build ./...
```

Frontend:

```bash
cd frontend
npm run typecheck
npm run test -- --run
npm run lint
npm run build
```

E2E and launch:

```bash
cd tests/e2e
npm run test:launch -- --project=chromium
npm run test:preview-verify -- --project=chromium
```

Production-facing verifiers must be run only with the appropriate environment and secrets already configured outside the repo:

```bash
node scripts/verify_render_launch_env.mjs
node scripts/verify_stripe_launch.mjs
node scripts/verify_mobile_external_readiness.mjs
node scripts/run_live_golden_build.mjs
```

If a verification command cannot be run in the current environment, state that clearly in the final response and record any launch-relevant gap in the appropriate doc.

## Commit And PR Conventions

Use short imperative commit subjects. Optional conventional prefixes such as `fix:`, `feat:`, `refactor:`, `docs:`, and `test:` are encouraged when they clarify scope.

PRs should include affected areas, launch-risk assessment, verification commands, screenshots or canary evidence for UI/preview changes, and doc updates. For billing, auth, provider routing, execution, or deployment changes, include rollback notes and operational impact.
