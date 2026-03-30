# Architecture Guide

## Canonical entrypoints

- `backend/cmd/main.go`: canonical backend server entrypoint, route wiring, startup checks, and service initialization
- `frontend/src/App.tsx`: top-level frontend shell and view switching
- `backend/main.go`: build-ignored legacy snapshot kept only for reference

Non-runtime directories:

- `investor-demo/`: static investor collateral, not part of the product runtime
- `my-new-project/`: scratch/demo artifact, not part of the product runtime

## High-level shape

APEX.BUILD is split into two primary applications:

- `backend/`: Go API, orchestration, execution, preview, auth, billing, deployment, and collaboration services
- `frontend/`: React/Vite client for the builder, IDE, preview, admin, billing, and account surfaces

## Backend map

Key backend areas:

- `cmd/main.go`: startup wiring, health/readiness, route registration, and background systems
- `internal/agents/`: build orchestration, task routing, retry/recovery loops, diff review, and agent WebSocket state
- `internal/ai/`: provider clients and routing across Claude, OpenAI, Gemini, Grok, and Ollama
- `internal/handlers/`: HTTP handlers for auth, projects, preview, deploy, billing, packages, search, and admin
- `internal/preview/` and `internal/execution/`: preview verification, runner abstractions, and sandbox execution
- `internal/auth/` and `internal/middleware/`: auth, cookies, JWT, CSRF/session protections, and request guards
- `internal/payments/`, `internal/spend/`, `internal/budget/`: Stripe, ledgering, spend events, and budget controls
- `internal/collaboration/` and `internal/websocket/`: room state, presence, cursors, OT, and realtime transport
- `internal/deploy/`, `internal/hosting/`: deployment providers and hosting/runtime metadata
- `internal/db/`, `internal/database/`, `migrations/`: persistence setup, models, and schema changes

## Frontend map

Key frontend areas:

- `src/App.tsx`: shell composition, authentication gating, and top-level feature routing
- `src/components/builder/`: app-builder workflow, build progress, review, issues, and recovery surfaces
- `src/components/ide/`: IDE layout, editor panels, file tree, Git, terminals, and preview embedding
- `src/components/preview/`: preview iframe, server state, console/network capture, and runtime diagnostics
- `src/components/billing/`, `src/components/settings/`, `src/components/admin/`: account, usage, spend, secrets, and admin tools
- `src/services/`: API client, WebSocket client, provider integrations, and shared stateful services
- `src/hooks/useStore.ts`: Zustand store for auth, projects, files, UI, spend, budget, and collaboration state

## Request flow

1. The frontend authenticates against `/api/v1/auth/*`.
2. The frontend calls REST endpoints under `/api/v1/*` for projects, files, builds, preview, billing, and deploy actions.
3. The backend handlers validate auth/session state, load persistence state, and dispatch to domain services.
4. Long-running operations expose progress over WebSocket endpoints under `/ws/*`.
5. Health/readiness state is surfaced through `/health`, `/ready`, and feature status endpoints for deploy/runtime truth.

## Agent orchestration flow

1. A build request enters the backend through the build handlers and is normalized into build/task state.
2. The planner and architect establish the build plan, contract, and task ordering.
3. Specialized agents execute frontend, backend, database, testing, review, and solver work.
4. The AI router selects the appropriate provider/model for each task and records usage/spend metadata.
5. Validation, preview readiness, and recovery loops run before the build is marked complete.
6. Proposed edits and diff review surfaces allow the user to inspect and approve generated changes.

## Billing flow

1. Usage and spend events are recorded per provider, model, and build.
2. Budget enforcement checks run before expensive AI calls.
3. Stripe checkout and webhook handlers manage top-ups and plan-related billing flows.
4. Ledger, spend, and usage surfaces are exposed to the frontend for account history and budget truth.

## Preview and execution flow

1. Generated apps are prepared for preview through `internal/preview/` and `internal/execution/`.
2. Static readiness checks run first, then runtime/browser verification runs when enabled.
3. Preview runners and sandbox/container layers execute project code and surface logs/status.
4. The frontend preview panel streams render state, server status, console output, and network activity.

## Collaboration flow

1. Clients join project/room channels over WebSockets.
2. Presence, cursor, and selection state is tracked server-side.
3. Operational transformation and file-sync events reconcile concurrent edits.
4. The frontend IDE renders shared presence, cursors, comments, and file updates in realtime.

## Deployment flow

1. Project deploy requests are normalized through deploy handlers and services.
2. Provider-specific implementations generate configs and deployment requests for Vercel, Netlify, or Render.
3. Hosting/deployment metadata is persisted for status tracking and follow-up operations.
4. Production deployment of the platform itself is described by `render.yaml`, while CI/CD lives in `.github/workflows/`.

## Deployment model

- Local multi-service development is driven by Docker Compose.
- Production deployment is described in `render.yaml`.
- The frontend consumes runtime API/WebSocket endpoints from `/config.js`, allowing the same image to adapt across environments.
