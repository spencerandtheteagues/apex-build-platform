# API Guide

This repo ships an OpenAPI spec at [`backend/api/openapi.yaml`](../backend/api/openapi.yaml). Treat that file as the detailed contract. This guide is the human-readable map of the API surface that matters most for development and release verification.

## Base URLs

- Local API base: `http://localhost:8080/api/v1`
- Production API base: `https://api.apex.build/api/v1`

Non-versioned health endpoints:

- `GET /health`
- `GET /health/deep`
- `GET /ready`
- `GET /health/features`

`/health/features` returns the startup/readiness summary for critical and optional backend services. It is the operational endpoint to inspect degraded-but-running features such as cache fallbacks, disabled payment providers, or optional subsystems that failed to initialize cleanly.

## Authentication

Primary auth endpoints:

- `POST /auth/register`
- `POST /auth/login`
- `POST /auth/refresh`
- `POST /auth/logout`
- `GET /user/profile`
- `PUT /user/profile`

The frontend stores access and refresh tokens and will attempt token refresh on `401` responses. If refresh fails, the current app session is cleared and the client reloads into a fresh unauthenticated state.

## Core resource areas

### Projects and files

- Project CRUD and listing live under `/projects`
- File CRUD and upload endpoints live under `/projects/:projectId/files` and related upload/import handlers
- Git operations are exposed under `/git/*`

### AI and builds

- AI generation endpoints live under `/ai/*`
- Build lifecycle endpoints live under `/build/*`
- Agent and build status streaming is exposed over WebSockets under `/ws/build/:buildId`

### Preview and execution

- Preview endpoints live under `/preview/*`
- Full-stack preview startup is exposed at `/preview/fullstack/start`
- Execution and terminal features live under `/execution/*` and `/terminal/*`

### Spend, budget, billing, and BYOK

- Spend dashboard endpoints live under `/spend/*`
- Budget caps and enforcement endpoints live under `/budget/*`
- Billing and payments live under `/billing/*` and `/payments/*`
- User-managed provider keys live under `/byok/*`

### Deployment and hosting

- Provider deployment endpoints live under `/deploy/*`
- Native hosting and domain management live under `/hosting/*` and `/domains/*`

## WebSocket surfaces

- `/ws/build/:buildId`: build progress and completion updates
- `/ws/terminal/:sessionId`: interactive terminal sessions
- `/ws/collab`: collaboration and presence
- `/ws/debug/:sessionId`: debugging sessions
- `/ws/deploy/:deploymentId`: deploy progress/logs
- `/mcp/ws`: MCP protocol transport

## Release verification checklist

Before shipping API changes:

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/main.go
```

If the API contract changed, update both:

- [`backend/api/openapi.yaml`](../backend/api/openapi.yaml)
- this guide when the change affects how humans integrate with the system
