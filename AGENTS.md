# Repository Guidelines

## Project Structure & Module Organization
`backend/` contains the Go API, agent orchestration, billing, deployment, and websocket services; the main entrypoint is `backend/cmd/main.go`, with most code under `backend/internal/`. `frontend/` is the Vite + React + TypeScript client; UI lives in `frontend/src/components/`, views in `frontend/src/pages/`, and shared logic in `frontend/src/services/` and `frontend/src/hooks/`. End-to-end coverage lives in `tests/e2e/`. Root files such as `docker-compose.yml`, `render.yaml`, and `deploy.sh` cover local and hosted deployment.

## Build, Test, and Development Commands
- `cd backend && go run ./cmd/main.go`: run the backend locally.
- `cd backend && go build ./... && go test ./...`: compile and run backend tests.
- `cd frontend && npm install && npm run dev`: start the frontend dev server.
- `cd frontend && npm run build`: produce the production frontend bundle.
- `cd frontend && npm run lint && npm run typecheck && npm run test`: run frontend linting, type checks, and Vitest.
- `cd tests/e2e && npm install && npm run test:smoke`: run generated Playwright smoke coverage.
- `docker compose up --build`: boot the local full-stack environment.

## Coding Style & Naming Conventions
Use `gofmt` formatting for Go and keep package names short, lowercase, and underscore-free. In React/TypeScript, use 2-space indentation, PascalCase for components (`AppBuilder.tsx`), camelCase for functions/hooks, and colocated `*.test.tsx` files. Prefer small modules inside existing feature folders instead of new top-level directories. Run `npm run lint` before submitting frontend changes.

## Testing Guidelines
Backend tests use Go’s standard test runner with `testify`; name files `*_test.go`. Frontend tests use Vitest and Testing Library; name files `*.test.ts` or `*.test.tsx`. E2E coverage uses Playwright in `tests/e2e/specs`. Add or update tests for behavioral changes, especially around build orchestration, billing, websocket flows, and UI recovery.

## Commit & Pull Request Guidelines
Recent history favors short, imperative subjects such as `fix: tighten recovery storage behavior` or `Fix build WebSocket connecting to wrong host in production`. Keep commits scoped and readable; use an optional conventional prefix (`fix:`, `feat:`, `refactor:`) when helpful. PRs should include a concise summary, affected areas, verification steps, linked issues, and screenshots or recordings for UI changes.

## Security & Configuration Tips
Never commit real API keys or populated `.env` files. Start from `.env.example`, keep provider keys local, and verify CORS, JWT, and Stripe settings before running shared environments. Treat billing, auth, and deployment code as high-risk surfaces and mention config changes explicitly in reviews.
