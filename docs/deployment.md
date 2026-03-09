# Deployment Guide

## Supported deployment paths

- Render via [`render.yaml`](../render.yaml)
- Docker Compose via [`docker-compose.yml`](../docker-compose.yml)
- Backend and frontend standalone Docker images via [`backend/Dockerfile`](../backend/Dockerfile) and [`frontend/Dockerfile`](../frontend/Dockerfile)

## Render

The current production blueprint defines:

- `apex-api` as a Docker web service
- `apex-frontend` as a Docker web service
- `apex-db` as a PostgreSQL database

Backend health configuration:

- bootstrap liveness: `/health`
- readiness/deep health: `/ready`
- feature readiness summary: `/health/features`

Frontend health configuration:

- `/health`

### Frontend runtime endpoint configuration

Container deployments use runtime configuration generated into `/config.js`. The frontend reads:

- `VITE_API_URL`
- `VITE_WS_URL`

from that runtime file first, then falls back to build-time values. This keeps Render and Docker deployments from baking in stale localhost URLs.

## Docker Compose

Bring up the local stack:

```bash
docker compose up --build
```

Default ports:

- frontend: `3000`
- backend: `8080`
- postgres: `5432`
- redis: `6379`

Optional local tooling:

- Adminer on `8081`
- Redis Commander on `8082`
- Prometheus on `9090`

Local compose defaults are for local use only. Seed accounts are disabled unless you provide `ADMIN_SEED_PASSWORD` / `SPENCER_SEED_PASSWORD` or explicitly set `ALLOW_DEFAULT_SEED_PASSWORDS=true`.

## Standalone Docker deploy script

[`deploy.sh`](../deploy.sh) is now a guarded backend deployment path. It requires explicit values for:

- `POSTGRES_PASSWORD`
- `REDIS_PASSWORD`
- `JWT_SECRET`
- `SECRETS_MASTER_KEY`

Optional provider tokens such as AI keys, Stripe, and deploy-provider credentials are passed through only when set. The script also enforces `EXECUTION_FORCE_CONTAINER=true` and checks `/ready` before reporting success.

## Release checklist

Run before cutting a release or pushing a deploy commit:

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/main.go
```

```bash
cd frontend
npm run typecheck
npm run lint
npm test -- --run
npm run build
```

Optional but recommended:

```bash
cd tests/e2e
npm ci
npm run generate
npm test
```

## Deployment-specific notes

- The backend starts a bootstrap HTTP listener early so load balancer health checks succeed while deeper initialization continues.
- The frontend Nginx container serves `/config.js` and health checks from the same image used in production.
- Keep [`backend/api/openapi.yaml`](../backend/api/openapi.yaml) updated when externally consumed endpoints change.
