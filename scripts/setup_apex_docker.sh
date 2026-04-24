#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/setup_apex_docker.sh [--context NAME] [--env-file PATH]

Verifies Docker CLI + daemon availability and prints (or writes) Apex preview
environment variables that align Apex with the active Docker context.

Options:
  --context NAME     Docker context to use (defaults to current context)
  --env-file PATH    Write exports to PATH in KEY=VALUE format
  -h, --help         Show help
EOF
}

context=""
env_file=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --context)
      if [[ $# -lt 2 ]]; then
        echo "error: --context requires a value" >&2
        exit 1
      fi
      context="$2"
      shift 2
      ;;
    --env-file)
      if [[ $# -lt 2 ]]; then
        echo "error: --env-file requires a value" >&2
        exit 1
      fi
      env_file="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if ! command -v docker >/dev/null 2>&1; then
  echo "error: docker CLI not found on PATH" >&2
  exit 1
fi

if [[ -z "$context" ]]; then
  context="$(docker context show 2>/dev/null || true)"
fi

if [[ -z "$context" ]]; then
  echo "error: no active Docker context found" >&2
  exit 1
fi

if ! docker context inspect "$context" >/dev/null 2>&1; then
  echo "error: Docker context '$context' does not exist" >&2
  echo "available contexts:" >&2
  docker context ls >&2 || true
  exit 1
fi

current_context="$(docker context show)"
if [[ "$current_context" != "$context" ]]; then
  echo "Switching Docker context: $current_context -> $context"
  docker context use "$context" >/dev/null
fi

endpoint="$(docker context inspect "$context" --format '{{(index .Endpoints "docker").Host}}' 2>/dev/null || true)"
server_version="$(docker info --format '{{.ServerVersion}}' 2>/dev/null || true)"

if [[ -z "$server_version" ]]; then
  echo "error: docker daemon is not reachable for context '$context'" >&2
  echo "run: open -a Docker" >&2
  echo "then rerun this script" >&2
  exit 1
fi

echo "Docker daemon reachable"
echo "  context: $context"
if [[ -n "$endpoint" ]]; then
  echo "  endpoint: $endpoint"
fi
echo "  server version: $server_version"

apex_host="$endpoint"
apex_context="$context"

if [[ -n "$env_file" ]]; then
  mkdir -p "$(dirname "$env_file")"
  {
    echo "APEX_PREVIEW_DOCKER_CONTEXT=$apex_context"
    if [[ -n "$apex_host" ]]; then
      echo "APEX_PREVIEW_DOCKER_HOST=$apex_host"
    fi
  } > "$env_file"
  echo "Wrote Apex Docker env to $env_file"
else
  echo
  echo "Export these for Apex preview:"
  echo "  export APEX_PREVIEW_DOCKER_CONTEXT=$apex_context"
  if [[ -n "$apex_host" ]]; then
    echo "  export APEX_PREVIEW_DOCKER_HOST=$apex_host"
  fi
fi

