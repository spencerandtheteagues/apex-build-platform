#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NETWORK_NAME="${DOCKER_NETWORK:-apex-network}"
POSTGRES_CONTAINER="${POSTGRES_CONTAINER_NAME:-apex-postgres}"
REDIS_CONTAINER="${REDIS_CONTAINER_NAME:-apex-redis}"
BACKEND_CONTAINER="${BACKEND_CONTAINER_NAME:-apex-backend}"
POSTGRES_USER="${POSTGRES_USER:-postgres}"
POSTGRES_DB="${POSTGRES_DB:-apex_build}"
APP_PORT="${APP_PORT:-8080}"

require_env() {
    local name="$1"
    if [[ -z "${!name:-}" ]]; then
        echo "ERROR: ${name} must be set before running deploy.sh"
        exit 1
    fi
}

append_optional_env() {
    local name="$1"
    if [[ -n "${!name:-}" ]]; then
        docker_env_args+=("-e" "${name}=${!name}")
    fi
}

container_exists() {
    docker container inspect "$1" >/dev/null 2>&1
}

container_running() {
    [[ "$(docker inspect -f '{{.State.Running}}' "$1" 2>/dev/null || true)" == "true" ]]
}

echo "Starting APEX.BUILD production deployment"
echo "========================================="

require_env POSTGRES_PASSWORD
require_env REDIS_PASSWORD
require_env JWT_SECRET
require_env SECRETS_MASTER_KEY

export BUILD_DATE="${BUILD_DATE:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}"
export VERSION="${VERSION:-$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo production)}"
export COMMIT_HASH="${COMMIT_HASH:-$(git -C "$ROOT_DIR" rev-parse HEAD 2>/dev/null || echo production)}"

DATABASE_URL="${DATABASE_URL:-postgresql://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_CONTAINER}:5432/${POSTGRES_DB}?sslmode=disable}"
REDIS_URL="${REDIS_URL:-redis://:${REDIS_PASSWORD}@${REDIS_CONTAINER}:6379/0}"

echo "Checking Docker network and volumes..."
docker network inspect "$NETWORK_NAME" >/dev/null 2>&1 || docker network create "$NETWORK_NAME" >/dev/null
docker volume create postgres_data >/dev/null
docker volume create redis_data >/dev/null
mkdir -p "$ROOT_DIR/uploads" "$ROOT_DIR/logs"

echo "Ensuring PostgreSQL is available..."
if ! container_exists "$POSTGRES_CONTAINER"; then
    docker run -d \
        --name "$POSTGRES_CONTAINER" \
        --network "$NETWORK_NAME" \
        -e POSTGRES_DB="$POSTGRES_DB" \
        -e POSTGRES_USER="$POSTGRES_USER" \
        -e POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
        -p 5432:5432 \
        -v postgres_data:/var/lib/postgresql/data \
        postgres:16-alpine >/dev/null
elif ! container_running "$POSTGRES_CONTAINER"; then
    docker start "$POSTGRES_CONTAINER" >/dev/null
fi

echo "Ensuring Redis is available..."
if ! container_exists "$REDIS_CONTAINER"; then
    docker run -d \
        --name "$REDIS_CONTAINER" \
        --network "$NETWORK_NAME" \
        -p 6379:6379 \
        -v redis_data:/data \
        redis:7-alpine \
        redis-server --appendonly yes --requirepass "$REDIS_PASSWORD" >/dev/null
elif ! container_running "$REDIS_CONTAINER"; then
    docker start "$REDIS_CONTAINER" >/dev/null
fi

echo "Building backend image..."
docker build -f "$ROOT_DIR/backend/Dockerfile.production" -t apex-backend:production "$ROOT_DIR/backend"

echo "Replacing backend container..."
docker rm -f "$BACKEND_CONTAINER" >/dev/null 2>&1 || true

docker_env_args=(
    "-e" "ENVIRONMENT=production"
    "-e" "GIN_MODE=release"
    "-e" "PORT=8080"
    "-e" "DATABASE_URL=${DATABASE_URL}"
    "-e" "REDIS_URL=${REDIS_URL}"
    "-e" "REDIS_PASSWORD=${REDIS_PASSWORD}"
    "-e" "JWT_SECRET=${JWT_SECRET}"
    "-e" "SECRETS_MASTER_KEY=${SECRETS_MASTER_KEY}"
    "-e" "EXECUTION_FORCE_CONTAINER=true"
    "-e" "ENABLE_METRICS=${ENABLE_METRICS:-true}"
    "-e" "ENABLE_TRACING=${ENABLE_TRACING:-false}"
    "-e" "LOG_LEVEL=${LOG_LEVEL:-info}"
    "-e" "BUILD_DATE=${BUILD_DATE}"
    "-e" "VERSION=${VERSION}"
    "-e" "GIT_COMMIT=${COMMIT_HASH}"
)

for optional_env in ANTHROPIC_API_KEY OPENAI_API_KEY GOOGLE_AI_API_KEY GEMINI_API_KEY XAI_API_KEY STRIPE_SECRET_KEY STRIPE_WEBHOOK_SECRET VERCEL_TOKEN NETLIFY_TOKEN RENDER_TOKEN; do
    append_optional_env "$optional_env"
done

docker run -d \
    --name "$BACKEND_CONTAINER" \
    --network "$NETWORK_NAME" \
    -p "${APP_PORT}:8080" \
    "${docker_env_args[@]}" \
    -v "$ROOT_DIR/uploads:/app/uploads" \
    -v "$ROOT_DIR/logs:/app/logs" \
    apex-backend:production >/dev/null

echo "Waiting for backend readiness..."
for attempt in $(seq 1 30); do
    if curl -fsS "http://localhost:${APP_PORT}/ready" >/dev/null 2>&1; then
        echo "Backend is ready."
        break
    fi

    if [[ "$attempt" -eq 30 ]]; then
        echo "Backend readiness check failed after 30 attempts."
        docker logs "$BACKEND_CONTAINER" --tail 50
        exit 1
    fi

    sleep 2
done

echo ""
echo "Deployment complete"
echo "==================="
echo "Backend: http://localhost:${APP_PORT}"
echo "Health:  http://localhost:${APP_PORT}/health"
echo "Ready:   http://localhost:${APP_PORT}/ready"
echo ""
docker ps --filter "name=apex-"
