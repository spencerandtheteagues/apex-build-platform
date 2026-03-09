# Architecture Guide

## High-level shape

APEX.BUILD is split into two primary applications:

- `backend/`: Go API, orchestration, execution, preview, auth, billing, and deploy logic
- `frontend/`: React/Vite client for the builder, IDE, preview, admin, and account surfaces

## Backend

Key backend areas:

- `cmd/main.go`: application entry point and route wiring
- `internal/agents/`: build orchestration, task routing, and build WebSocket flow
- `internal/ai/`: provider clients and routing across Claude, GPT-4, Gemini, Grok, and Ollama
- `internal/handlers/`: HTTP handlers for projects, preview, deploy, billing, packages, search, and more
- `internal/preview/` and `internal/execution/`: live preview and execution systems
- `internal/auth/` and `internal/middleware/`: auth and request protection
- `internal/db/`, `internal/database/`, `migrations/`: persistence and schema changes

## Frontend

Key frontend areas:

- `src/App.tsx`: top-level shell and view switching
- `src/components/builder/`: app-builder workflow and build progress
- `src/components/ide/`: IDE layout, panels, review surfaces, preview integration
- `src/components/preview/`: preview iframe, backend server status, console/network capture
- `src/services/`: API client, AI client, WebSocket client
- `src/hooks/useStore.ts`: Zustand store for auth, projects, files, UI, spend, budget, and collaboration state

## Runtime flow

1. The frontend authenticates against `/api/v1/auth/*`.
2. Project, file, build, preview, and deployment actions use the backend REST API.
3. Long-running flows stream updates over WebSocket endpoints under `/ws/*`.
4. The backend coordinates AI providers, persistence, preview/execution, and deployment integrations.

## Deployment model

- Local multi-service development is driven by Docker Compose.
- Production deployment is described in `render.yaml`.
- The frontend can consume runtime API/WebSocket endpoints from `/config.js`, allowing the same image to adapt across environments.
